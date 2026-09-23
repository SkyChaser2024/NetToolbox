package desktop

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"campusnet-toolbox/internal/networkdiag"
	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/systemnet"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type latencyCheck struct {
	sequence uint64
	cancel   context.CancelFunc
}

type diagnosticSession struct {
	mu     sync.Mutex
	id     string
	cancel context.CancelFunc
}

func (session *diagnosticSession) replace(id string, cancel context.CancelFunc) {
	session.mu.Lock()
	previousCancel := session.cancel
	session.id = id
	session.cancel = cancel
	session.mu.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
}

func (session *diagnosticSession) cancelMatching(id string) {
	session.mu.Lock()
	if id != "" && id != session.id {
		session.mu.Unlock()
		return
	}
	cancel := session.cancel
	session.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (session *diagnosticSession) clear(id string) {
	session.mu.Lock()
	if session.id == id {
		session.id = ""
		session.cancel = nil
	}
	session.mu.Unlock()
}

func listNetworkInterfaces() ([]systemnet.NetworkInterface, error) {
	interfaces, err := systemnet.List()
	if interfaces == nil {
		interfaces = make([]systemnet.NetworkInterface, 0)
	}
	return interfaces, err
}

func (a *App) CheckNAT() networkdiag.NATResult {
	diagnostics := a.loadDiagnostics()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return networkdiag.CheckNAT(ctx, diagnostics.NATServers)
}

func (a *App) CheckOverview() networkdiag.OverviewResult {
	diagnostics := a.loadDiagnostics()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result := networkdiag.CheckOverview(ctx, diagnostics.IPv4Endpoints, diagnostics.IPv6Endpoints)
	_ = saveOverviewCache(result)
	return result
}

func (a *App) CheckLatency(id string) networkdiag.LatencyProbe {
	target, exists := a.currentLatencyTarget(id)
	if !exists {
		return networkdiag.LatencyProbe{ID: id, Status: "failed", Error: "未知的连接测试目标"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)

	a.latencyMu.Lock()
	if a.latencyChecks == nil {
		a.latencyChecks = make(map[string]latencyCheck)
	}
	if previous, exists := a.latencyChecks[id]; exists {
		previous.cancel()
	}
	a.latencySequence++
	sequence := a.latencySequence
	a.latencyChecks[id] = latencyCheck{sequence: sequence, cancel: cancel}
	a.latencyMu.Unlock()

	defer func() {
		cancel()
		a.latencyMu.Lock()
		if current, exists := a.latencyChecks[id]; exists && current.sequence == sequence {
			delete(a.latencyChecks, id)
		}
		a.latencyMu.Unlock()
	}()
	return networkdiag.CheckLatencyTarget(ctx, target)
}

func (a *App) cacheLatencyTargets(values []settings.LatencyTarget) {
	targets := make([]networkdiag.LatencyTarget, len(values))
	for index, value := range values {
		targets[index] = networkdiag.LatencyTarget{ID: value.ID, Name: value.Name, URL: value.URL, Region: value.Region}
	}
	a.latencyMu.Lock()
	a.latencyTargets = targets
	a.latencyConfigured = true
	a.latencyMu.Unlock()
}

func (a *App) currentLatencyTarget(id string) (networkdiag.LatencyTarget, bool) {
	a.latencyMu.Lock()
	if a.latencyConfigured {
		target, exists := findLatencyTarget(a.latencyTargets, id)
		a.latencyMu.Unlock()
		return target, exists
	}
	a.latencyMu.Unlock()

	diagnostics := a.loadDiagnostics()
	a.cacheLatencyTargets(diagnostics.LatencyTargets)

	a.latencyMu.Lock()
	target, exists := findLatencyTarget(a.latencyTargets, id)
	a.latencyMu.Unlock()
	return target, exists
}

func findLatencyTarget(targets []networkdiag.LatencyTarget, id string) (networkdiag.LatencyTarget, bool) {
	for _, target := range targets {
		if target.ID == id {
			return target, true
		}
	}
	return networkdiag.LatencyTarget{}, false
}

func (a *App) CancelLatencyChecks() {
	a.latencyMu.Lock()
	checks := a.latencyChecks
	hadPollingSession := checks != nil
	a.latencyChecks = nil
	for _, check := range checks {
		check.cancel()
	}
	a.latencyMu.Unlock()
	networkdiag.CloseLatencyConnections()
	if hadPollingSession {
		releaseUnusedForegroundMemory()
	}
}

func (a *App) StartTraceroute(request TraceRequest) error {
	request.SessionID = strings.TrimSpace(request.SessionID)
	if request.SessionID == "" || len(request.SessionID) > 128 {
		return errors.New("无效的路由追踪会话")
	}
	options, err := networkdiag.NormalizeTraceOptions(networkdiag.TraceOptions{
		Target:           request.Target,
		Protocol:         request.Protocol,
		MaxHops:          request.MaxHops,
		Timeout:          time.Duration(request.TimeoutMs) * time.Millisecond,
		ResolveHostnames: request.ResolveHostnames,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.traceSession.replace(request.SessionID, cancel)

	go a.runTraceroute(ctx, request.SessionID, options)
	return nil
}

func (a *App) CancelTraceroute(sessionID string) {
	a.traceSession.cancelMatching(sessionID)
}

func (a *App) runTraceroute(ctx context.Context, sessionID string, options networkdiag.TraceOptions) {
	summary, err := networkdiag.RunTraceroute(ctx, options, func(started networkdiag.TraceStarted) {
		if ctx.Err() != nil {
			return
		}
		a.emitTracerouteEvent(TraceEvent{SessionID: sessionID, Type: "started", Target: started.Target, Address: started.Address, Protocol: started.Protocol})
	}, func(hop networkdiag.TraceHop) {
		if ctx.Err() != nil {
			return
		}
		a.emitTracerouteEvent(TraceEvent{SessionID: sessionID, Type: "hop", Hop: &hop})
	})

	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		a.emitTracerouteEvent(TraceEvent{SessionID: sessionID, Type: "cancelled", Status: "cancelled", HopCount: summary.HopCount, DurationMs: summary.DurationMs})
	} else if err != nil {
		a.emitTracerouteEvent(TraceEvent{SessionID: sessionID, Type: "error", Status: "error", HopCount: summary.HopCount, DurationMs: summary.DurationMs, Error: err.Error()})
	} else {
		a.emitTracerouteEvent(TraceEvent{SessionID: sessionID, Type: "completed", Target: summary.Target, Address: summary.Address, Protocol: summary.Protocol, Status: summary.Status, Reached: summary.Reached, HopCount: summary.HopCount, DurationMs: summary.DurationMs})
	}

	a.traceSession.clear(sessionID)
}

func (a *App) emitTracerouteEvent(event TraceEvent) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "traceroute:event", event)
	}
}

func (a *App) StartPing(request PingRequest) error {
	request.SessionID = strings.TrimSpace(request.SessionID)
	if request.SessionID == "" || len(request.SessionID) > 128 {
		return errors.New("无效的 Ping 会话")
	}
	options, err := networkdiag.NormalizePingOptions(networkdiag.PingOptions{
		Target:   request.Target,
		Protocol: request.Protocol,
		Count:    request.Count,
		Timeout:  time.Duration(request.TimeoutMs) * time.Millisecond,
		Interval: time.Duration(request.IntervalMs) * time.Millisecond,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.pingSession.replace(request.SessionID, cancel)

	go a.runPing(ctx, request.SessionID, options)
	return nil
}

func (a *App) CancelPing(sessionID string) {
	a.pingSession.cancelMatching(sessionID)
}

func (a *App) runPing(ctx context.Context, sessionID string, options networkdiag.PingOptions) {
	summary, err := networkdiag.RunPing(ctx, options, func(started networkdiag.PingStarted) {
		if ctx.Err() != nil {
			return
		}
		a.emitPingEvent(PingEvent{SessionID: sessionID, Type: "started", Target: started.Target, Address: started.Address, Protocol: started.Protocol})
	}, func(reply networkdiag.PingReply) {
		if ctx.Err() != nil {
			return
		}
		a.emitPingEvent(PingEvent{SessionID: sessionID, Type: "reply", Reply: &reply})
	})

	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		summary.Status = "cancelled"
		a.emitPingEvent(PingEvent{SessionID: sessionID, Type: "cancelled", Summary: &summary})
	} else if err != nil {
		summary.Status = "error"
		a.emitPingEvent(PingEvent{SessionID: sessionID, Type: "error", Summary: &summary, Error: err.Error()})
	} else {
		a.emitPingEvent(PingEvent{SessionID: sessionID, Type: "completed", Target: summary.Target, Address: summary.Address, Protocol: summary.Protocol, Summary: &summary})
	}

	a.pingSession.clear(sessionID)
}

func (a *App) emitPingEvent(event PingEvent) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "ping:event", event)
	}
}

func (a *App) CheckPublicIPv4() networkdiag.PublicNetworkInfo {
	return a.checkPublicNetwork("tcp4", "ipv4")
}

func (a *App) CheckPublicIPv6() networkdiag.PublicNetworkInfo {
	return a.checkPublicNetwork("tcp6", "ipv6")
}

func (a *App) checkPublicNetwork(network, version string) networkdiag.PublicNetworkInfo {
	diagnostics := a.loadDiagnostics()
	endpoints := diagnostics.IPv4Endpoints
	if network == "tcp6" {
		endpoints = diagnostics.IPv6Endpoints
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	result := networkdiag.CheckPublicNetworkInfo(ctx, network, endpoints)
	_ = updateOverviewCacheProtocol(version, result)
	return result
}

func (a *App) CheckIPv6() networkdiag.IPv6Result {
	diagnostics := a.loadDiagnostics()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return networkdiag.CheckIPv6(ctx, networkdiag.IPv6Config{
		IPv4Endpoints: diagnostics.IPv4Endpoints,
		IPv6Endpoints: diagnostics.IPv6Endpoints,
		IPv6Sites:     diagnostics.IPv6Sites,
		AAAADomain:    diagnostics.AAAADomain,
		LargeURL:      diagnostics.IPv6LargeURL,
	})
}

func (a *App) loadDiagnostics() settings.DiagnosticSettings {
	if a.settings != nil {
		if stored, err := a.settings.LoadDiagnostics(); err == nil {
			return stored
		}
	}
	return settings.DefaultDiagnostics()
}
