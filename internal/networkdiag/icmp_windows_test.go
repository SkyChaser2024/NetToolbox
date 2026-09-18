//go:build windows

package networkdiag

import (
	"context"
	"errors"
	"testing"
	"time"
	"unsafe"
)

func TestICMPv6ReplyBufferSizeIncludesIOStatusBlock(t *testing.T) {
	requestSize := 32
	want := int(unsafe.Sizeof(icmp6EchoReply{})) + requestSize + 8 + 2*int(unsafe.Sizeof(uintptr(0)))
	if got := icmpV6ReplyBufferSize(requestSize); got != want {
		t.Fatalf("icmpV6ReplyBufferSize(%d) = %d, want %d", requestSize, got, want)
	}
}

func TestRunCancelableICMPProbeReturnsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	probeStarted := make(chan struct{})
	releaseProbe := make(chan struct{})
	finished := make(chan error, 1)
	defer close(releaseProbe)

	go func() {
		_, err := runCancelableICMPProbe(ctx, func() (icmpProbeResult, error) {
			close(probeStarted)
			<-releaseProbe
			return icmpProbeResult{Status: ipSuccess}, nil
		})
		finished <- err
	}()

	select {
	case <-probeStarted:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	cancel()

	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("cancelled probe did not return promptly")
	}
}
