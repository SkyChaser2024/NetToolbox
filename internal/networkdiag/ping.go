package networkdiag

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

type PingOptions struct {
	Target   string
	Protocol string
	Count    int
	Interval time.Duration
	Timeout  time.Duration
}

type PingStarted struct {
	Target   string `json:"target"`
	Address  string `json:"address"`
	Protocol string `json:"protocol"`
}

type PingReply struct {
	Sequence  int     `json:"sequence"`
	Status    string  `json:"status"`
	Address   string  `json:"address,omitempty"`
	LatencyMs float64 `json:"latencyMs"`
}

type PingSummary struct {
	Target      string  `json:"target"`
	Address     string  `json:"address"`
	Protocol    string  `json:"protocol"`
	Status      string  `json:"status"`
	Sent        int     `json:"sent"`
	Received    int     `json:"received"`
	Lost        int     `json:"lost"`
	LossPercent float64 `json:"lossPercent"`
	MinMs       float64 `json:"minMs"`
	AverageMs   float64 `json:"averageMs"`
	MaxMs       float64 `json:"maxMs"`
	DurationMs  int64   `json:"durationMs"`
}

type pingProber interface {
	Probe(context.Context, time.Duration) (icmpProbeResult, error)
	Close() error
}

// NormalizePingOptions validates user-controlled ping settings without
// performing network I/O.
func NormalizePingOptions(options PingOptions) (PingOptions, error) {
	target, err := normalizeDiagnosticTarget(options.Target, "请输入要 Ping 的域名或 IP 地址", "Ping 目标过长")
	if err != nil {
		return PingOptions{}, err
	}
	protocol, err := normalizeDiagnosticProtocol(options.Protocol)
	if err != nil {
		return PingOptions{}, err
	}
	if options.Count < 1 || options.Count > 100 {
		return PingOptions{}, errors.New("探测次数必须在 1 到 100 之间")
	}
	if options.Interval < 100*time.Millisecond || options.Interval > 10*time.Second {
		return PingOptions{}, errors.New("探测间隔必须在 100 到 10000 毫秒之间")
	}
	if options.Timeout < 200*time.Millisecond || options.Timeout > 5*time.Second {
		return PingOptions{}, errors.New("探测超时必须在 200 到 5000 毫秒之间")
	}
	options.Target = target
	options.Protocol = protocol
	return options, nil
}

// RunPing resolves the target, opens one reusable native ICMP handle, and
// emits replies in sequence order. No more than one Echo call is in flight.
func RunPing(ctx context.Context, options PingOptions, onStarted func(PingStarted), onReply func(PingReply)) (PingSummary, error) {
	startedAt := time.Now()
	normalized, err := NormalizePingOptions(options)
	if err != nil {
		return PingSummary{}, err
	}
	targetIP, protocol, err := resolveDiagnosticTarget(ctx, normalized.Target, normalized.Protocol)
	if err != nil {
		return PingSummary{}, err
	}
	if err := ctx.Err(); err != nil {
		return PingSummary{}, err
	}
	prober, err := newICMPPingProber(targetIP, protocol)
	if err != nil {
		return PingSummary{}, fmt.Errorf("无法创建 ICMP 探测会话: %w", err)
	}
	defer prober.Close()

	closed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = prober.Close()
		case <-closed:
		}
	}()
	defer close(closed)

	start := PingStarted{Target: normalized.Target, Address: targetIP.String(), Protocol: protocol}
	if onStarted != nil {
		onStarted(start)
	}
	summary, err := runPingProbes(ctx, normalized, prober, onReply)
	summary.Target = start.Target
	summary.Address = start.Address
	summary.Protocol = start.Protocol
	summary.DurationMs = time.Since(startedAt).Milliseconds()
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled):
		summary.Status = "cancelled"
	case err != nil:
		summary.Status = "error"
	default:
		summary.Status = "completed"
	}
	return summary, err
}

func runPingProbes(ctx context.Context, options PingOptions, prober pingProber, onReply func(PingReply)) (PingSummary, error) {
	summary := PingSummary{Status: "running"}
	var latencyTotal float64
	var nextProbeAt time.Time
	for sequence := 1; sequence <= options.Count; sequence++ {
		if err := waitForNextPing(ctx, nextProbeAt); err != nil {
			summary.Status = "cancelled"
			return summary, err
		}
		probeStartedAt := time.Now()
		nextProbeAt = probeStartedAt.Add(options.Interval)
		summary.Sent++
		nativeReply, err := prober.Probe(ctx, options.Timeout)
		if err != nil {
			summary.Lost = summary.Sent - summary.Received
			updatePingLoss(&summary)
			if ctx.Err() != nil {
				summary.Status = "cancelled"
				return summary, ctx.Err()
			}
			summary.Status = "error"
			return summary, err
		}
		reply, err := pingReplyFromICMPResult(sequence, nativeReply)
		if err != nil {
			summary.Lost = summary.Sent - summary.Received
			updatePingLoss(&summary)
			summary.Status = "error"
			return summary, err
		}
		if reply.Status == "reply" {
			summary.Received++
			latencyTotal += reply.LatencyMs
			if summary.Received == 1 || reply.LatencyMs < summary.MinMs {
				summary.MinMs = reply.LatencyMs
			}
			if summary.Received == 1 || reply.LatencyMs > summary.MaxMs {
				summary.MaxMs = reply.LatencyMs
			}
			summary.AverageMs = roundTraceLatency(latencyTotal / float64(summary.Received))
		}
		summary.Lost = summary.Sent - summary.Received
		updatePingLoss(&summary)
		if err := ctx.Err(); err != nil {
			summary.Status = "cancelled"
			return summary, err
		}
		if onReply != nil {
			onReply(reply)
		}
		if err := ctx.Err(); err != nil {
			summary.Status = "cancelled"
			return summary, err
		}
	}
	summary.Status = "completed"
	return summary, nil
}

func waitForNextPing(ctx context.Context, scheduled time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	delay := time.Until(scheduled)
	if scheduled.IsZero() || delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func pingReplyFromICMPResult(sequence int, result icmpProbeResult) (PingReply, error) {
	reply := PingReply{Sequence: sequence, Address: result.Address, LatencyMs: roundTraceLatency(float64(result.RoundTripTime))}
	switch result.Status {
	case ipSuccess:
		reply.Status = "reply"
	case ipRequestTimedOut:
		reply.Status = "timeout"
		reply.Address = ""
		reply.LatencyMs = 0
	case ipTTLExpiredTransit, ipDestNetUnreachable, ipDestHostUnreachable, ipDestProtUnreachable, ipDestPortUnreachable, ipPacketTooBig:
		reply.Status = "unreachable"
	default:
		return PingReply{}, fmt.Errorf("ICMP 返回状态 %d", result.Status)
	}
	return reply, nil
}

func updatePingLoss(summary *PingSummary) {
	if summary.Sent == 0 {
		summary.LossPercent = 0
		return
	}
	summary.LossPercent = math.Round(float64(summary.Lost)*1000/float64(summary.Sent)) / 10
}
