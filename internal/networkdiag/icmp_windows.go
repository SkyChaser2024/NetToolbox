//go:build windows

package networkdiag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ipHelperDLL     = windows.NewLazySystemDLL("iphlpapi.dll")
	icmpCreateFile  = ipHelperDLL.NewProc("IcmpCreateFile")
	icmp6CreateFile = ipHelperDLL.NewProc("Icmp6CreateFile")
	icmpCloseHandle = ipHelperDLL.NewProc("IcmpCloseHandle")
	icmpSendEcho    = ipHelperDLL.NewProc("IcmpSendEcho")
	icmp6SendEcho2  = ipHelperDLL.NewProc("Icmp6SendEcho2")
)

type ipOptionInformation struct {
	TTL         uint8
	TOS         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uintptr
}

type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

// IPV6_ADDRESS_EX uses four-byte packing in the Windows SDK. Keeping the
// padding explicit makes the layout correct on both 32-bit and 64-bit builds.
type ipv6AddressEx struct {
	Port     uint16
	Padding  uint16
	FlowInfo uint32
	Address  [16]byte
	ScopeID  uint32
}

type icmp6EchoReply struct {
	Address       ipv6AddressEx
	Status        uint32
	RoundTripTime uint32
}

// IO_STATUS_BLOCK is two pointer-sized fields. Icmp6SendEcho2 requires room
// for one at the end of every reply buffer, even for synchronous calls.
type ioStatusBlock struct {
	Status      uintptr
	Information uintptr
}

func icmpV6ReplyBufferSize(requestSize int) int {
	return int(unsafe.Sizeof(icmp6EchoReply{})) + requestSize + 8 + int(unsafe.Sizeof(ioStatusBlock{}))
}

// windowsICMPSession owns every native handle used by one diagnostic run.
// Closing it interrupts outstanding synchronous Echo calls and is idempotent.
type windowsICMPSession struct {
	target   net.IP
	protocol string
	mu       sync.Mutex
	handles  map[windows.Handle]struct{}
	closed   bool
}

type icmpProbeOutcome struct {
	result icmpProbeResult
	err    error
}

func runCancelableICMPProbe(ctx context.Context, probe func() (icmpProbeResult, error)) (icmpProbeResult, error) {
	finished := make(chan icmpProbeOutcome, 1)
	go func() {
		result, err := probe()
		finished <- icmpProbeOutcome{result: result, err: err}
	}()

	select {
	case <-ctx.Done():
		return icmpProbeResult{}, ctx.Err()
	case outcome := <-finished:
		return outcome.result, outcome.err
	}
}

func newWindowsICMPSession(target net.IP, protocol string) (*windowsICMPSession, error) {
	procedures := []*windows.LazyProc{icmpCloseHandle}
	if protocol == "ipv6" {
		procedures = append(procedures, icmp6CreateFile, icmp6SendEcho2)
	} else {
		procedures = append(procedures, icmpCreateFile, icmpSendEcho)
	}
	for _, procedure := range procedures {
		if err := procedure.Find(); err != nil {
			return nil, err
		}
	}
	return &windowsICMPSession{
		target:   append(net.IP(nil), target...),
		protocol: protocol,
		handles:  make(map[windows.Handle]struct{}),
	}, nil
}

// Probe uses a separate native handle so callers may safely issue concurrent
// requests, as traceroute does for the three probes in each hop.
func (session *windowsICMPSession) Probe(ctx context.Context, ttl int, timeout time.Duration, request []byte) (icmpProbeResult, error) {
	if err := ctx.Err(); err != nil {
		return icmpProbeResult{}, err
	}
	handle, err := session.openHandle()
	if err != nil {
		return icmpProbeResult{}, err
	}
	defer session.releaseHandle(handle)
	return session.probeWithHandle(ctx, handle, ttl, timeout, request)
}

// probeWithHandle permits sequential users such as ping to reuse one handle.
func (session *windowsICMPSession) probeWithHandle(ctx context.Context, handle windows.Handle, ttl int, timeout time.Duration, request []byte) (icmpProbeResult, error) {
	if err := ctx.Err(); err != nil {
		return icmpProbeResult{}, err
	}
	return runCancelableICMPProbe(ctx, func() (icmpProbeResult, error) {
		if session.protocol == "ipv6" {
			return session.probeIPv6(ctx, handle, ttl, timeout, request)
		}
		return session.probeIPv4(ctx, handle, ttl, timeout, request)
	})
}

func (session *windowsICMPSession) probeIPv4(ctx context.Context, handle windows.Handle, ttl int, timeout time.Duration, request []byte) (icmpProbeResult, error) {
	target := session.target.To4()
	if target == nil {
		return icmpProbeResult{}, errors.New("无效的 IPv4 目标地址")
	}
	var options ipOptionInformation
	var optionsPointer uintptr
	if ttl > 0 {
		options.TTL = uint8(ttl)
		optionsPointer = uintptr(unsafe.Pointer(&options))
	}
	var requestPointer uintptr
	if len(request) > 0 {
		requestPointer = uintptr(unsafe.Pointer(&request[0]))
	}
	replyBuffer := make([]byte, int(unsafe.Sizeof(icmpEchoReply{}))+len(request)+8)
	destination := uintptr(uint32(target[0]) | uint32(target[1])<<8 | uint32(target[2])<<16 | uint32(target[3])<<24)
	replyCount, _, callErr := icmpSendEcho.Call(
		uintptr(handle),
		destination,
		requestPointer,
		uintptr(len(request)),
		optionsPointer,
		uintptr(unsafe.Pointer(&replyBuffer[0])),
		uintptr(len(replyBuffer)),
		uintptr(timeout.Milliseconds()),
	)
	runtime.KeepAlive(request)
	runtime.KeepAlive(options)
	runtime.KeepAlive(replyBuffer)
	if err := ctx.Err(); err != nil {
		return icmpProbeResult{}, err
	}
	if replyCount == 0 {
		if isICMPTimeoutError(callErr) {
			return icmpProbeResult{Status: ipRequestTimedOut}, nil
		}
		return icmpProbeResult{}, fmt.Errorf("IPv4 ICMP 探测失败: %w", callErr)
	}
	reply := (*icmpEchoReply)(unsafe.Pointer(&replyBuffer[0]))
	address := net.IPv4(byte(reply.Address), byte(reply.Address>>8), byte(reply.Address>>16), byte(reply.Address>>24)).String()
	return icmpProbeResult{Status: reply.Status, Address: address, RoundTripTime: reply.RoundTripTime}, nil
}

func (session *windowsICMPSession) probeIPv6(ctx context.Context, handle windows.Handle, ttl int, timeout time.Duration, request []byte) (icmpProbeResult, error) {
	target := session.target.To16()
	if target == nil || session.target.To4() != nil {
		return icmpProbeResult{}, errors.New("无效的 IPv6 目标地址")
	}
	var options ipOptionInformation
	var optionsPointer uintptr
	if ttl > 0 {
		options.TTL = uint8(ttl)
		optionsPointer = uintptr(unsafe.Pointer(&options))
	}
	var requestPointer uintptr
	if len(request) > 0 {
		requestPointer = uintptr(unsafe.Pointer(&request[0]))
	}
	replyBuffer := make([]byte, icmpV6ReplyBufferSize(len(request)))
	sourceAddress := windows.RawSockaddrInet6{Family: windows.AF_INET6}
	destinationAddress := windows.RawSockaddrInet6{Family: windows.AF_INET6}
	copy(destinationAddress.Addr[:], target)
	replyCount, _, callErr := icmp6SendEcho2.Call(
		uintptr(handle),
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&sourceAddress)),
		uintptr(unsafe.Pointer(&destinationAddress)),
		requestPointer,
		uintptr(len(request)),
		optionsPointer,
		uintptr(unsafe.Pointer(&replyBuffer[0])),
		uintptr(len(replyBuffer)),
		uintptr(timeout.Milliseconds()),
	)
	runtime.KeepAlive(request)
	runtime.KeepAlive(options)
	runtime.KeepAlive(sourceAddress)
	runtime.KeepAlive(destinationAddress)
	runtime.KeepAlive(replyBuffer)
	if err := ctx.Err(); err != nil {
		return icmpProbeResult{}, err
	}
	if replyCount == 0 {
		if isICMPTimeoutError(callErr) {
			return icmpProbeResult{Status: ipRequestTimedOut}, nil
		}
		return icmpProbeResult{}, fmt.Errorf("IPv6 ICMP 探测失败: %w", callErr)
	}
	reply := (*icmp6EchoReply)(unsafe.Pointer(&replyBuffer[0]))
	return icmpProbeResult{Status: reply.Status, Address: net.IP(reply.Address.Address[:]).String(), RoundTripTime: reply.RoundTripTime}, nil
}

func (session *windowsICMPSession) openHandle() (windows.Handle, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return 0, context.Canceled
	}
	procedure := icmpCreateFile
	if session.protocol == "ipv6" {
		procedure = icmp6CreateFile
	}
	value, _, callErr := procedure.Call()
	handle := windows.Handle(value)
	if handle == windows.InvalidHandle || handle == 0 {
		return 0, fmt.Errorf("无法创建 ICMP 句柄: %w", callErr)
	}
	session.handles[handle] = struct{}{}
	return handle, nil
}

func (session *windowsICMPSession) releaseHandle(handle windows.Handle) {
	session.mu.Lock()
	_, exists := session.handles[handle]
	if exists {
		delete(session.handles, handle)
	}
	session.mu.Unlock()
	if exists {
		icmpCloseHandle.Call(uintptr(handle))
	}
}

func (session *windowsICMPSession) Close() error {
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return nil
	}
	session.closed = true
	handles := make([]windows.Handle, 0, len(session.handles))
	for handle := range session.handles {
		handles = append(handles, handle)
		delete(session.handles, handle)
	}
	session.mu.Unlock()
	for _, handle := range handles {
		icmpCloseHandle.Call(uintptr(handle))
	}
	return nil
}

func isICMPTimeoutError(err error) bool {
	if err == nil {
		return true
	}
	errno, ok := err.(syscall.Errno)
	return ok && (errno == 0 || uint32(errno) == ipRequestTimedOut || errno == windows.ERROR_TIMEOUT)
}
