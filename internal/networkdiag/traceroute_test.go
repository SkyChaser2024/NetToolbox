package networkdiag

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

func TestNormalizeTraceOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    TraceOptions
		target   string
		protocol string
		wantErr  bool
	}{
		{name: "domain", input: TraceOptions{Target: " WWW.Example.COM. ", Protocol: "auto", MaxHops: 30, Timeout: time.Second}, target: "www.example.com", protocol: "auto"},
		{name: "url", input: TraceOptions{Target: "https://www.example.com/path?q=1", Protocol: "IPv4", MaxHops: 30, Timeout: time.Second}, target: "www.example.com", protocol: "ipv4"},
		{name: "ipv6", input: TraceOptions{Target: "[2001:db8::1]", Protocol: "ipv6", MaxHops: 64, Timeout: 5 * time.Second}, target: "2001:db8::1", protocol: "ipv6"},
		{name: "empty", input: TraceOptions{Protocol: "auto", MaxHops: 30, Timeout: time.Second}, wantErr: true},
		{name: "bad protocol", input: TraceOptions{Target: "example.com", Protocol: "udp", MaxHops: 30, Timeout: time.Second}, wantErr: true},
		{name: "bad hops", input: TraceOptions{Target: "example.com", Protocol: "auto", MaxHops: 65, Timeout: time.Second}, wantErr: true},
		{name: "bad timeout", input: TraceOptions{Target: "example.com", Protocol: "auto", MaxHops: 30, Timeout: 100 * time.Millisecond}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := NormalizeTraceOptions(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeTraceOptions() error = %v", err)
			}
			if actual.Target != test.target || actual.Protocol != test.protocol {
				t.Fatalf("normalized target/protocol = %q/%q, want %q/%q", actual.Target, actual.Protocol, test.target, test.protocol)
			}
		})
	}
}

func TestSelectTraceIP(t *testing.T) {
	addresses := []net.IPAddr{{IP: net.ParseIP("2001:db8::2")}, {IP: net.ParseIP("192.0.2.10")}}
	address, family, err := selectTraceIP(addresses, "auto")
	if err != nil || family != "ipv4" || address.String() != "192.0.2.10" {
		t.Fatalf("auto selection = %s/%s/%v, want IPv4 first", address, family, err)
	}
	address, family, err = selectTraceIP(addresses, "ipv6")
	if err != nil || family != "ipv6" || address.String() != "2001:db8::2" {
		t.Fatalf("IPv6 selection = %s/%s/%v", address, family, err)
	}
	if _, _, err = selectTraceIP(addresses[1:], "ipv6"); err == nil {
		t.Fatal("expected missing IPv6 error")
	}
}

type fakeTraceProber struct {
	hops  map[int][]TraceProbe
	calls []int
}

func (fake *fakeTraceProber) ProbeHop(ctx context.Context, hop int, _ time.Duration, count int) ([]TraceProbe, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fake.calls = append(fake.calls, hop)
	probes, exists := fake.hops[hop]
	if !exists {
		probes = make([]TraceProbe, count)
		for index := range probes {
			probes[index].Status = "timeout"
		}
	}
	return append([]TraceProbe(nil), probes...), nil
}

func (*fakeTraceProber) Close() error { return nil }

func TestRunTraceHopsStreamsUntilDestination(t *testing.T) {
	prober := &fakeTraceProber{hops: map[int][]TraceProbe{
		1: {
			{Status: "reply", Address: "192.168.1.1", LatencyMs: 1.2},
			{Status: "reply", Address: "192.168.1.1", LatencyMs: 1.4},
			{Status: "reply", Address: "192.168.1.1", LatencyMs: 1.6},
		},
		3: {
			{Status: "reply", Address: "203.0.113.8", LatencyMs: 18.1, Reached: true},
			{Status: "reply", Address: "203.0.113.8", LatencyMs: 17.9, Reached: true},
			{Status: "reply", Address: "203.0.113.8", LatencyMs: 18.3, Reached: true},
		},
	}}
	var hops []TraceHop
	lookupCalls := 0
	summary, err := runTraceHops(context.Background(), TraceOptions{MaxHops: 8, Timeout: time.Second, ResolveHostnames: false}, prober, func(context.Context, string) ([]string, error) {
		lookupCalls++
		return nil, nil
	}, func(hop TraceHop) { hops = append(hops, hop) })
	if err != nil {
		t.Fatalf("runTraceHops() error = %v", err)
	}
	if !summary.Reached || summary.Status != "reached" || summary.HopCount != 3 {
		t.Fatalf("summary = %+v", summary)
	}
	if len(hops) != 3 || hops[1].Status != "timeout" || hops[2].Status != "destination" {
		t.Fatalf("hops = %+v", hops)
	}
	if lookupCalls != 0 {
		t.Fatalf("DNS lookup called %d times while disabled", lookupCalls)
	}
}

func TestSummarizeTraceHopHandlesMultipleAddresses(t *testing.T) {
	hop := summarizeTraceHop(4, []TraceProbe{
		{Status: "reply", Address: "192.0.2.1", LatencyMs: 10},
		{Status: "reply", Address: "192.0.2.2", LatencyMs: 20},
		{Status: "timeout"},
	})
	if hop.Status != "reply" || hop.AverageMs != 15 || len(hop.Probes) != 3 {
		t.Fatalf("hop = %+v", hop)
	}
}

func TestRunTraceHopsStopsOnUnreachable(t *testing.T) {
	prober := &fakeTraceProber{hops: map[int][]TraceProbe{
		1: {{Status: "unreachable", Address: "192.0.2.1", LatencyMs: 2.5}},
	}}
	summary, err := runTraceHops(context.Background(), TraceOptions{MaxHops: 30, Timeout: time.Second}, prober, nil, nil)
	if err != nil || summary.Status != "unreachable" || summary.HopCount != 1 || len(prober.calls) != 1 {
		t.Fatalf("summary/calls/error = %+v/%v/%v", summary, prober.calls, err)
	}
}

func TestRunTraceHopsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	prober := &fakeTraceProber{hops: map[int][]TraceProbe{
		1: {{Status: "reply", Address: "192.0.2.1", LatencyMs: 1}},
	}}
	_, err := runTraceHops(ctx, TraceOptions{MaxHops: 30, Timeout: time.Second}, prober, nil, func(TraceHop) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if len(prober.calls) != 1 {
		t.Fatalf("probe calls = %v, want one hop", prober.calls)
	}
}

type cancellingTraceProber struct {
	cancel context.CancelFunc
}

func (prober *cancellingTraceProber) ProbeHop(context.Context, int, time.Duration, int) ([]TraceProbe, error) {
	prober.cancel()
	return []TraceProbe{{Status: "reply", Address: "192.0.2.1", LatencyMs: 1}}, nil
}

func (*cancellingTraceProber) Close() error { return nil }

func TestRunTraceHopsDoesNotEmitAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	prober := &cancellingTraceProber{cancel: cancel}
	emitted := false
	_, err := runTraceHops(ctx, TraceOptions{MaxHops: 30, Timeout: time.Second}, prober, nil, func(TraceHop) { emitted = true })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if emitted {
		t.Fatal("hop emitted after cancellation")
	}
}

func TestTraceProbeFromIPStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      uint32
		probeStatus string
		reached     bool
	}{
		{name: "destination", status: ipSuccess, probeStatus: "reply", reached: true},
		{name: "router", status: ipTTLExpiredTransit, probeStatus: "reply"},
		{name: "unreachable", status: ipDestHostUnreachable, probeStatus: "unreachable"},
		{name: "timeout", status: ipRequestTimedOut, probeStatus: "timeout"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			probe, err := traceProbeFromIPStatus(test.status, "192.0.2.1", 12)
			if err != nil || probe.Status != test.probeStatus || probe.Reached != test.reached {
				t.Fatalf("probe/error = %+v/%v", probe, err)
			}
		})
	}
	if _, err := traceProbeFromIPStatus(11999, "192.0.2.1", 1); err == nil {
		t.Fatal("unknown ICMP status must return an error")
	}
}

func TestResolveTraceHostnamesDeduplicatesAddresses(t *testing.T) {
	probes := []TraceProbe{{Status: "reply", Address: "192.0.2.1"}, {Status: "reply", Address: "192.0.2.1"}, {Status: "timeout"}}
	lookupCalls := 0
	err := resolveTraceHostnames(context.Background(), probes, func(_ context.Context, address string) ([]string, error) {
		lookupCalls++
		return []string{"router.example."}, nil
	})
	if err != nil || lookupCalls != 1 || probes[0].Hostname != "router.example" || probes[1].Hostname != "router.example" {
		t.Fatalf("probes/calls/error = %+v/%d/%v", probes, lookupCalls, err)
	}
}

func TestRunTracerouteLoopbackIntegration(t *testing.T) {
	if os.Getenv("NETTOOLBOX_TRACE_INTEGRATION") != "1" {
		t.Skip("set NETTOOLBOX_TRACE_INTEGRATION=1 to run native ICMP integration tests")
	}
	for _, test := range []struct {
		name     string
		target   string
		protocol string
	}{
		{name: "IPv4", target: "127.0.0.1", protocol: "ipv4"},
		{name: "IPv6", target: "::1", protocol: "ipv6"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			var hops []TraceHop
			summary, err := RunTraceroute(ctx, TraceOptions{Target: test.target, Protocol: test.protocol, MaxHops: 3, Timeout: 500 * time.Millisecond}, nil, func(hop TraceHop) {
				hops = append(hops, hop)
			})
			if err != nil {
				t.Fatalf("RunTraceroute() error = %v", err)
			}
			if !summary.Reached || summary.HopCount != 1 || len(hops) != 1 || hops[0].Status != "destination" {
				t.Fatalf("summary/hops = %+v/%+v", summary, hops)
			}
		})
	}
}

func TestRunTraceroutePublicIntegration(t *testing.T) {
	target := os.Getenv("NETTOOLBOX_TRACE_TARGET")
	if target == "" {
		t.Skip("set NETTOOLBOX_TRACE_TARGET to run a public route integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var hops []TraceHop
	summary, err := RunTraceroute(ctx, TraceOptions{Target: target, Protocol: "ipv4", MaxHops: 5, Timeout: time.Second}, nil, func(hop TraceHop) {
		hops = append(hops, hop)
		t.Logf("hop %d: %+v", hop.Number, hop)
	})
	if err != nil {
		t.Fatalf("RunTraceroute() error = %v", err)
	}
	if len(hops) == 0 {
		t.Fatal("expected at least one hop")
	}
	responded := false
	for _, hop := range hops {
		if hop.Status != "timeout" {
			responded = true
			break
		}
	}
	if !responded {
		t.Fatalf("all hops timed out; native ICMP route replies were not returned: %+v", summary)
	}
}
