//go:build windows

package networkdiag

import (
	"context"
	"net"
	"time"

	"golang.org/x/sys/windows"
)

var pingRequestData = []byte("nettoolbox-ping-0123456789abcdef")

type windowsICMPPingProber struct {
	session *windowsICMPSession
	handle  windows.Handle
}

func newICMPPingProber(target net.IP, protocol string) (*windowsICMPPingProber, error) {
	session, err := newWindowsICMPSession(target, protocol)
	if err != nil {
		return nil, err
	}
	handle, err := session.openHandle()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	return &windowsICMPPingProber{session: session, handle: handle}, nil
}

func (prober *windowsICMPPingProber) Probe(ctx context.Context, timeout time.Duration) (icmpProbeResult, error) {
	return prober.session.probeWithHandle(ctx, prober.handle, 0, timeout, pingRequestData)
}

func (prober *windowsICMPPingProber) Close() error {
	return prober.session.Close()
}
