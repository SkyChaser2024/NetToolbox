//go:build windows

package networkdiag

import (
	"context"
	"fmt"
	"net"
	"time"
)

var traceRequestData = []byte("nettoolbox-trace")

type windowsICMPTraceProber struct {
	session *windowsICMPSession
}

func newICMPTraceProber(target net.IP, protocol string) (*windowsICMPTraceProber, error) {
	session, err := newWindowsICMPSession(target, protocol)
	if err != nil {
		return nil, err
	}
	return &windowsICMPTraceProber{session: session}, nil
}

func (prober *windowsICMPTraceProber) ProbeHop(ctx context.Context, hop int, timeout time.Duration, count int) ([]TraceProbe, error) {
	type result struct {
		index int
		probe TraceProbe
		err   error
	}
	results := make(chan result, count)
	for index := 0; index < count; index++ {
		go func(probeIndex int) {
			nativeReply, err := prober.session.Probe(ctx, hop, timeout, traceRequestData)
			probe := TraceProbe{}
			if err == nil {
				probe, err = traceProbeFromICMPResult(nativeReply)
			}
			results <- result{index: probeIndex, probe: probe, err: err}
		}(index)
	}

	probes := make([]TraceProbe, count)
	for index := range probes {
		probes[index].Status = "timeout"
	}
	var firstProbeError error
	successfulProbes := 0
	for range count {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case item := <-results:
			if item.err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if firstProbeError == nil {
					firstProbeError = item.err
				}
				continue
			}
			successfulProbes++
			probes[item.index] = item.probe
		}
	}
	if successfulProbes == 0 && firstProbeError != nil {
		return nil, firstProbeError
	}
	return probes, nil
}

func (prober *windowsICMPTraceProber) Close() error {
	return prober.session.Close()
}

func traceProbeFromICMPResult(result icmpProbeResult) (TraceProbe, error) {
	return traceProbeFromIPStatus(result.Status, result.Address, result.RoundTripTime)
}

func traceProbeFromIPStatus(status uint32, address string, roundTripTime uint32) (TraceProbe, error) {
	probe := TraceProbe{Address: address, LatencyMs: roundTraceLatency(float64(roundTripTime))}
	switch status {
	case ipSuccess:
		probe.Status = "reply"
		probe.Reached = true
	case ipTTLExpiredTransit:
		probe.Status = "reply"
	case ipDestNetUnreachable, ipDestHostUnreachable, ipDestProtUnreachable, ipDestPortUnreachable, ipPacketTooBig:
		probe.Status = "unreachable"
	case ipRequestTimedOut:
		probe.Status = "timeout"
		probe.Address = ""
		probe.LatencyMs = 0
	default:
		return TraceProbe{}, fmt.Errorf("ICMP 返回状态 %d", status)
	}
	return probe, nil
}
