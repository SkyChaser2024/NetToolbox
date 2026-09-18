package networkdiag

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"time"
)

const (
	traceProbeCount    = 3
	traceLookupTimeout = 700 * time.Millisecond
)

type TraceOptions struct {
	Target           string
	Protocol         string
	MaxHops          int
	Timeout          time.Duration
	ResolveHostnames bool
}

type TraceStarted struct {
	Target   string `json:"target"`
	Address  string `json:"address"`
	Protocol string `json:"protocol"`
}

type TraceProbe struct {
	Status    string  `json:"status"`
	Address   string  `json:"address,omitempty"`
	Hostname  string  `json:"hostname,omitempty"`
	LatencyMs float64 `json:"latencyMs,omitempty"`
	Reached   bool    `json:"reached,omitempty"`
}

type TraceHop struct {
	Number    int          `json:"number"`
	Status    string       `json:"status"`
	Probes    []TraceProbe `json:"probes"`
	AverageMs float64      `json:"averageMs,omitempty"`
	Reached   bool         `json:"reached,omitempty"`
}

type TraceSummary struct {
	Target     string `json:"target"`
	Address    string `json:"address"`
	Protocol   string `json:"protocol"`
	Status     string `json:"status"`
	Reached    bool   `json:"reached"`
	HopCount   int    `json:"hopCount"`
	DurationMs int64  `json:"durationMs"`
}

type traceProber interface {
	ProbeHop(context.Context, int, time.Duration, int) ([]TraceProbe, error)
	Close() error
}

type traceLookup func(context.Context, string) ([]string, error)

// NormalizeTraceOptions validates user-controlled traceroute settings without
// performing network I/O.
func NormalizeTraceOptions(options TraceOptions) (TraceOptions, error) {
	target, err := normalizeTraceTarget(options.Target)
	if err != nil {
		return TraceOptions{}, err
	}
	protocol, err := normalizeDiagnosticProtocol(options.Protocol)
	if err != nil {
		return TraceOptions{}, err
	}
	if options.MaxHops < 1 || options.MaxHops > 64 {
		return TraceOptions{}, errors.New("最大跳数必须在 1 到 64 之间")
	}
	if options.Timeout < 200*time.Millisecond || options.Timeout > 5*time.Second {
		return TraceOptions{}, errors.New("每跳超时必须在 200 到 5000 毫秒之间")
	}
	options.Target = target
	options.Protocol = protocol
	return options, nil
}

// RunTraceroute resolves the target, opens a native ICMP session, and emits
// complete hop rows as they become available.
func RunTraceroute(ctx context.Context, options TraceOptions, onStarted func(TraceStarted), onHop func(TraceHop)) (TraceSummary, error) {
	startedAt := time.Now()
	normalized, err := NormalizeTraceOptions(options)
	if err != nil {
		return TraceSummary{}, err
	}
	targetIP, protocol, err := resolveTraceTarget(ctx, normalized.Target, normalized.Protocol)
	if err != nil {
		return TraceSummary{}, err
	}
	prober, err := newICMPTraceProber(targetIP, protocol)
	if err != nil {
		return TraceSummary{}, fmt.Errorf("无法创建 ICMP 探测会话: %w", err)
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

	start := TraceStarted{Target: normalized.Target, Address: targetIP.String(), Protocol: protocol}
	if err := ctx.Err(); err != nil {
		return TraceSummary{}, err
	}
	if onStarted != nil {
		onStarted(start)
	}
	summary, err := runTraceHops(ctx, normalized, prober, net.DefaultResolver.LookupAddr, onHop)
	summary.Target = start.Target
	summary.Address = start.Address
	summary.Protocol = start.Protocol
	summary.DurationMs = time.Since(startedAt).Milliseconds()
	return summary, err
}

func runTraceHops(ctx context.Context, options TraceOptions, prober traceProber, lookup traceLookup, onHop func(TraceHop)) (TraceSummary, error) {
	summary := TraceSummary{Status: "max_hops"}
	for hopNumber := 1; hopNumber <= options.MaxHops; hopNumber++ {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		probes, err := prober.ProbeHop(ctx, hopNumber, options.Timeout, traceProbeCount)
		if err != nil {
			if ctx.Err() != nil {
				return summary, ctx.Err()
			}
			return summary, err
		}
		if options.ResolveHostnames {
			if err := resolveTraceHostnames(ctx, probes, lookup); err != nil && ctx.Err() != nil {
				return summary, ctx.Err()
			}
		}
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		hop := summarizeTraceHop(hopNumber, probes)
		summary.HopCount = hopNumber
		if onHop != nil {
			onHop(hop)
		}
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		switch hop.Status {
		case "destination":
			summary.Status = "reached"
			summary.Reached = true
			return summary, nil
		case "unreachable":
			summary.Status = "unreachable"
			return summary, nil
		}
	}
	return summary, nil
}

func summarizeTraceHop(number int, probes []TraceProbe) TraceHop {
	hop := TraceHop{Number: number, Status: "timeout", Probes: probes}
	var latencyTotal float64
	var latencyCount int
	for _, probe := range probes {
		if probe.Status != "timeout" {
			latencyTotal += probe.LatencyMs
			latencyCount++
		}
		if probe.Reached {
			hop.Status = "destination"
			hop.Reached = true
		} else if probe.Status == "unreachable" && hop.Status != "destination" {
			hop.Status = "unreachable"
		} else if probe.Status == "reply" && hop.Status == "timeout" {
			hop.Status = "reply"
		}
	}
	if latencyCount > 0 {
		hop.AverageMs = roundTraceLatency(latencyTotal / float64(latencyCount))
	}
	return hop
}

func resolveTraceHostnames(ctx context.Context, probes []TraceProbe, lookup traceLookup) error {
	addresses := make(map[string]struct{})
	for _, probe := range probes {
		if probe.Address != "" {
			addresses[probe.Address] = struct{}{}
		}
	}
	if len(addresses) == 0 {
		return nil
	}
	type result struct {
		address  string
		hostname string
	}
	results := make(chan result, len(addresses))
	for address := range addresses {
		go func(value string) {
			lookupCtx, cancel := context.WithTimeout(ctx, traceLookupTimeout)
			defer cancel()
			names, err := lookup(lookupCtx, value)
			hostname := ""
			if err == nil && len(names) > 0 {
				hostname = strings.TrimSuffix(names[0], ".")
			}
			results <- result{address: value, hostname: hostname}
		}(address)
	}
	names := make(map[string]string, len(addresses))
	for range addresses {
		select {
		case item := <-results:
			names[item.address] = item.hostname
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for index := range probes {
		probes[index].Hostname = names[probes[index].Address]
	}
	return nil
}

func roundTraceLatency(value float64) float64 {
	rounded := math.Round(value*10) / 10
	if value > 0 && rounded == 0 {
		return 0.1
	}
	return rounded
}
