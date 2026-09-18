package networkdiag

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNormalizePingOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    PingOptions
		target   string
		protocol string
		wantErr  bool
	}{
		{name: "domain", input: PingOptions{Target: " WWW.Example.COM. ", Protocol: "auto", Count: 4, Interval: time.Second, Timeout: time.Second}, target: "www.example.com", protocol: "auto"},
		{name: "url", input: PingOptions{Target: "HTTPS://www.example.com/path", Protocol: "IPv6", Count: 1, Interval: 100 * time.Millisecond, Timeout: 200 * time.Millisecond}, target: "www.example.com", protocol: "ipv6"},
		{name: "empty", input: PingOptions{Protocol: "auto", Count: 4, Interval: time.Second, Timeout: time.Second}, wantErr: true},
		{name: "malformed url", input: PingOptions{Target: "http://a\nb", Protocol: "auto", Count: 4, Interval: time.Second, Timeout: time.Second}, wantErr: true},
		{name: "bad protocol", input: PingOptions{Target: "example.com", Protocol: "udp", Count: 4, Interval: time.Second, Timeout: time.Second}, wantErr: true},
		{name: "bad count", input: PingOptions{Target: "example.com", Protocol: "auto", Count: 101, Interval: time.Second, Timeout: time.Second}, wantErr: true},
		{name: "bad interval", input: PingOptions{Target: "example.com", Protocol: "auto", Count: 4, Interval: 50 * time.Millisecond, Timeout: time.Second}, wantErr: true},
		{name: "bad timeout", input: PingOptions{Target: "example.com", Protocol: "auto", Count: 4, Interval: time.Second, Timeout: 100 * time.Millisecond}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := NormalizePingOptions(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePingOptions() error = %v", err)
			}
			if actual.Target != test.target || actual.Protocol != test.protocol {
				t.Fatalf("normalized target/protocol = %q/%q, want %q/%q", actual.Target, actual.Protocol, test.target, test.protocol)
			}
		})
	}
}

type fakePingProber struct {
	replies []icmpProbeResult
	errors  []error
	calls   int
	active  int
	maxSeen int
}

func (fake *fakePingProber) Probe(ctx context.Context, _ time.Duration) (icmpProbeResult, error) {
	if err := ctx.Err(); err != nil {
		return icmpProbeResult{}, err
	}
	fake.active++
	if fake.active > fake.maxSeen {
		fake.maxSeen = fake.active
	}
	index := fake.calls
	fake.calls++
	defer func() { fake.active-- }()
	if index < len(fake.errors) && fake.errors[index] != nil {
		return icmpProbeResult{}, fake.errors[index]
	}
	return fake.replies[index], nil
}

func (*fakePingProber) Close() error { return nil }

func TestRunPingProbesStreamsSequentialRepliesAndStats(t *testing.T) {
	prober := &fakePingProber{replies: []icmpProbeResult{
		{Status: ipSuccess, Address: "192.0.2.8", RoundTripTime: 0},
		{Status: ipRequestTimedOut},
		{Status: ipSuccess, Address: "192.0.2.8", RoundTripTime: 11},
		{Status: ipDestHostUnreachable, Address: "192.0.2.1", RoundTripTime: 2},
	}}
	var replies []PingReply
	summary, err := runPingProbes(context.Background(), PingOptions{Count: 4, Timeout: time.Second}, prober, func(reply PingReply) {
		replies = append(replies, reply)
	})
	if err != nil {
		t.Fatalf("runPingProbes() error = %v", err)
	}
	if summary.Status != "completed" || summary.Sent != 4 || summary.Received != 2 || summary.Lost != 2 || summary.LossPercent != 50 {
		t.Fatalf("summary counts = %+v", summary)
	}
	if summary.MinMs != 0 || summary.AverageMs != 5.5 || summary.MaxMs != 11 {
		t.Fatalf("summary latency = %+v", summary)
	}
	if len(replies) != 4 || replies[0].Sequence != 1 || replies[1].Status != "timeout" || replies[3].Status != "unreachable" {
		t.Fatalf("replies = %+v", replies)
	}
	if prober.maxSeen != 1 {
		t.Fatalf("maximum concurrent probes = %d, want 1", prober.maxSeen)
	}
}

func TestRunPingProbesCancellationStopsBeforeNextProbe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	prober := &fakePingProber{replies: []icmpProbeResult{
		{Status: ipSuccess, Address: "127.0.0.1"},
		{Status: ipSuccess, Address: "127.0.0.1"},
	}}
	summary, err := runPingProbes(ctx, PingOptions{Count: 2, Timeout: time.Second}, prober, func(PingReply) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if summary.Status != "cancelled" || summary.Sent != 1 || summary.Received != 1 || prober.calls != 1 {
		t.Fatalf("summary/calls = %+v/%d", summary, prober.calls)
	}
}

func TestRunPingProbesReturnsNativeErrorWithPartialStats(t *testing.T) {
	nativeError := errors.New("native failure")
	prober := &fakePingProber{
		replies: []icmpProbeResult{{Status: ipSuccess, Address: "127.0.0.1"}},
		errors:  []error{nil, nativeError},
	}
	summary, err := runPingProbes(context.Background(), PingOptions{Count: 2, Timeout: time.Second}, prober, nil)
	if !errors.Is(err, nativeError) || summary.Status != "error" || summary.Sent != 2 || summary.Received != 1 || summary.Lost != 1 || summary.LossPercent != 50 {
		t.Fatalf("summary/error = %+v/%v", summary, err)
	}
}

func TestPingReplyFromICMPResult(t *testing.T) {
	tests := []struct {
		status uint32
		want   string
	}{
		{status: ipSuccess, want: "reply"},
		{status: ipRequestTimedOut, want: "timeout"},
		{status: ipTTLExpiredTransit, want: "unreachable"},
		{status: ipDestHostUnreachable, want: "unreachable"},
	}
	for _, test := range tests {
		reply, err := pingReplyFromICMPResult(3, icmpProbeResult{Status: test.status, Address: "192.0.2.1", RoundTripTime: 4})
		if err != nil || reply.Sequence != 3 || reply.Status != test.want {
			t.Fatalf("status %d: reply/error = %+v/%v", test.status, reply, err)
		}
	}
	if _, err := pingReplyFromICMPResult(1, icmpProbeResult{Status: 11999}); err == nil {
		t.Fatal("unknown ICMP status must return an error")
	}
}

func TestPingSummaryKeepsZeroLatencyFieldsInJSON(t *testing.T) {
	encoded, err := json.Marshal(PingSummary{})
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	for _, field := range []string{`"minMs":0`, `"averageMs":0`, `"maxMs":0`} {
		if !strings.Contains(value, field) {
			t.Fatalf("JSON %s does not contain %s", value, field)
		}
	}
}

func TestRunPingLoopbackIntegration(t *testing.T) {
	if os.Getenv("NETTOOLBOX_PING_INTEGRATION") != "1" {
		t.Skip("set NETTOOLBOX_PING_INTEGRATION=1 to run native ICMP integration tests")
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
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var replies []PingReply
			summary, err := RunPing(ctx, PingOptions{Target: test.target, Protocol: test.protocol, Count: 2, Interval: 100 * time.Millisecond, Timeout: 500 * time.Millisecond}, nil, func(reply PingReply) {
				replies = append(replies, reply)
			})
			if err != nil {
				t.Fatalf("RunPing() error = %v", err)
			}
			if summary.Status != "completed" || summary.Sent != 2 || summary.Received != 2 || summary.Lost != 0 || len(replies) != 2 {
				t.Fatalf("summary/replies = %+v/%+v", summary, replies)
			}
		})
	}
}
