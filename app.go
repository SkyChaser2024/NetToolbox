package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"campusnet-toolbox/internal/adapters"
	"campusnet-toolbox/internal/appmeta"
	"campusnet-toolbox/internal/auth"
	"campusnet-toolbox/internal/autostart"
	"campusnet-toolbox/internal/networkdiag"
	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/systemnet"
	"campusnet-toolbox/internal/tray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type BootstrapData struct {
	Profile           settings.Profile             `json:"profile"`
	Diagnostics       settings.DiagnosticSettings  `json:"diagnostics"`
	System            settings.SystemPreferences   `json:"system"`
	Adapters          []adapters.Adapter           `json:"adapters"`
	NetworkInterfaces []systemnet.NetworkInterface `json:"networkInterfaces"`
	NpcapAvailable    bool                         `json:"npcapAvailable"`
	AdapterError      string                       `json:"adapterError,omitempty"`
	ConfigurationErr  string                       `json:"configurationError,omitempty"`
	CachedOverview    *networkdiag.OverviewResult  `json:"cachedOverview,omitempty"`
	AuthState         auth.State                   `json:"authState"`
	Version           string                       `json:"version"`
}

type SettingsRequest struct {
	Profile     settings.Profile            `json:"profile"`
	Diagnostics settings.DiagnosticSettings `json:"diagnostics"`
	System      settings.SystemPreferences  `json:"system"`
}

type SettingsResult struct {
	Profile           settings.Profile             `json:"profile"`
	Diagnostics       settings.DiagnosticSettings  `json:"diagnostics"`
	System            settings.SystemPreferences   `json:"system"`
	NetworkInterfaces []systemnet.NetworkInterface `json:"networkInterfaces"`
}

type AuthRequest struct {
	DeviceName       string `json:"deviceName"`
	AdapterLabel     string `json:"adapterLabel"`
	LocalMAC         string `json:"localMac"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Identity         string `json:"identity"`
	IdentitySuffix   string `json:"identitySuffix"`
	StartDelayMs     int    `json:"startDelayMs"`
	RetryDelayMs     int    `json:"retryDelayMs"`
	Debug            bool   `json:"debug"`
	RememberPassword bool   `json:"rememberPassword"`
}

type App struct {
	ctx               context.Context
	auth              *auth.Manager
	settings          *settings.Store
	configurationErr  error
	restoreOverview   bool
	latencyMu         sync.Mutex
	latencySequence   uint64
	latencyChecks     map[string]latencyCheck
	latencyTargets    []networkdiag.LatencyTarget
	latencyConfigured bool
}

type latencyCheck struct {
	sequence uint64
	cancel   context.CancelFunc
}

func NewApp(restoreOverview bool) *App {
	store, err := settings.NewStore()
	return &App{auth: auth.NewManager(), settings: store, configurationErr: err, restoreOverview: restoreOverview}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(_ context.Context) {
	a.CancelLatencyChecks()
	_ = a.auth.Stop(false)
}

func (a *App) beforeClose(_ context.Context) bool {
	if a.settings == nil {
		return false
	}
	system, err := a.settings.LoadSystemPreferences()
	if err == nil && system.CloseToTray {
		_ = autostart.LaunchBackground()
	} else {
		tray.QuitExisting()
	}
	return false
}

func (a *App) Bootstrap() BootstrapData {
	defer releaseUnusedForegroundMemory()
	data := BootstrapData{
		Profile: settings.DefaultProfile(), Diagnostics: settings.DefaultDiagnostics(), System: settings.DefaultSystemPreferences(),
		AuthState: a.auth.State(), Version: appmeta.Version,
	}
	if a.settings == nil {
		data.ConfigurationErr = "无法定位当前用户的配置目录"
		if a.configurationErr != nil {
			data.ConfigurationErr += ": " + a.configurationErr.Error()
		}
	} else {
		profile, diagnostics, system, err := a.settings.LoadConfiguration()
		if err != nil {
			data.ConfigurationErr = err.Error()
		} else {
			data.Profile = profile
			data.Diagnostics = diagnostics
			data.System = system
		}
	}
	a.cacheLatencyTargets(data.Diagnostics.LatencyTargets)
	networkInterfaces, err := listNetworkInterfaces()
	data.NetworkInterfaces = networkInterfaces
	var adapterResult adapters.Result
	if err == nil {
		adapterResult = adapters.ListWithInterfaces(data.NetworkInterfaces)
	} else {
		adapterResult = adapters.List()
	}
	data.Adapters = adapterResult.Adapters
	data.NpcapAvailable = adapterResult.NpcapAvailable
	data.AdapterError = adapterResult.Error
	if a.restoreOverview {
		if cached, err := loadOverviewCache(); err == nil {
			data.CachedOverview = &cached
		}
	}
	if data.AuthState == auth.StateIdle {
		selected := selectEthernetAdapter(data.Adapters, data.Profile.DeviceName)
		if selected != nil && selected.Up {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			online := networkdiag.InterfaceOnline(ctx, adapterAddresses(selected))
			cancel()
			if online {
				a.auth.MarkAuthenticated(auth.Config{DeviceName: selected.DeviceName, LocalMAC: selected.MAC})
				data.AuthState = auth.StateAuthenticated
			}
		}
	}
	return data
}

func (a *App) RefreshAdapters() adapters.Result {
	return adapters.List()
}

func (a *App) Connect(request AuthRequest) error {
	return a.startAuthentication(request, true)
}

func (a *App) startAuthentication(request AuthRequest, saveProfile bool) error {
	if a.auth.State() == auth.StateAuthenticated {
		return nil
	}
	if a.auth.Running() {
		return errors.New("认证任务已在运行")
	}
	profile := settings.Profile{
		DeviceName: request.DeviceName, AdapterLabel: request.AdapterLabel,
		LocalMAC: request.LocalMAC, Username: strings.TrimSpace(request.Username),
		Identity: request.Identity, IdentitySuffix: request.IdentitySuffix,
		StartDelayMs: request.StartDelayMs, RetryDelayMs: request.RetryDelayMs,
		Debug: request.Debug, RememberPassword: request.RememberPassword,
	}
	adapterResult := adapters.List()
	for index := range adapterResult.Adapters {
		item := &adapterResult.Adapters[index]
		if item.DeviceName != request.DeviceName || item.Kind != systemnet.PriorityEthernet || !item.Up {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		online := networkdiag.InterfaceOnline(ctx, adapterAddresses(item))
		cancel()
		if online {
			if saveProfile && a.settings != nil {
				if err := a.settings.Save(profile, request.Password); err != nil {
					return err
				}
			}
			a.auth.MarkAuthenticated(auth.Config{DeviceName: item.DeviceName, LocalMAC: item.MAC})
			a.emitAuthEvent(auth.Event{State: auth.StateAuthenticated, Level: "success", Message: "检测到该有线网卡已经认证，无需重复认证", Timestamp: time.Now().Format("15:04:05")})
			return nil
		}
		break
	}
	password := request.Password
	if password == "" && request.RememberPassword && a.settings != nil {
		stored, err := a.settings.Password()
		if err != nil {
			return err
		}
		password = stored
	}
	identity := strings.TrimSpace(request.Identity)
	if identity == "" {
		identity = strings.TrimSpace(request.Username)
	}
	cfg := auth.Config{
		DeviceName: request.DeviceName, LocalMAC: request.LocalMAC,
		Username: strings.TrimSpace(request.Username), Password: password,
		Identity: identity, IdentitySuffix: request.IdentitySuffix,
		StartDelay:  time.Duration(request.StartDelayMs) * time.Millisecond,
		RetryDelay:  time.Duration(request.RetryDelayMs) * time.Millisecond,
		MaxAttempts: auth.DefaultMaxAttempts,
		Debug:       request.Debug,
	}
	if err := auth.Validate(cfg); err != nil {
		return err
	}
	if saveProfile && a.settings != nil {
		if err := a.settings.Save(profile, request.Password); err != nil {
			return err
		}
	}
	return a.auth.Start(cfg, a.emitAuthEvent)
}

func (a *App) SaveSettings(request SettingsRequest) (SettingsResult, error) {
	if a.settings == nil {
		return SettingsResult{}, errors.New("无法定位当前用户的配置目录")
	}
	if request.Profile.StartDelayMs < 0 || request.Profile.StartDelayMs > int(auth.MaxStartDelay/time.Millisecond) ||
		request.Profile.RetryDelayMs < 0 || request.Profile.RetryDelayMs > int(auth.MaxRetryDelay/time.Millisecond) {
		return SettingsResult{}, errors.New("认证延迟或重试参数超出允许范围")
	}
	if _, err := auth.BuildIdentity(request.Profile.Identity, request.Profile.IdentitySuffix); err != nil {
		return SettingsResult{}, fmt.Errorf("identity 扩展无效: %w", err)
	}
	normalizedDiagnostics, err := settings.NormalizeDiagnostics(request.Diagnostics)
	if err != nil {
		return SettingsResult{}, err
	}
	if request.System.PriorityMode != systemnet.PriorityAutomatic && request.System.PriorityMode != systemnet.PriorityEthernet && request.System.PriorityMode != systemnet.PriorityWiFi {
		return SettingsResult{}, errors.New("未知的网卡优先级模式")
	}
	// Automatic monitoring shares the resident tray process. Disabling tray
	// residence means the user explicitly requested a complete background exit.
	if !request.System.CloseToTray {
		request.System.AutoAuthenticate = false
	}
	previousSystem, err := a.settings.LoadSystemPreferences()
	if err != nil {
		return SettingsResult{}, err
	}
	if request.System.AutoAuthenticate {
		password, passwordErr := a.settings.Password()
		if passwordErr != nil {
			return SettingsResult{}, passwordErr
		}
		if strings.TrimSpace(request.Profile.Username) == "" || password == "" {
			return SettingsResult{}, errors.New("启用自动认证前，请先在锐捷认证页保存账号和密码")
		}
	}
	if request.System.PriorityMode != previousSystem.PriorityMode {
		if err := systemnet.ApplyPriority(request.System.PriorityMode); err != nil {
			return SettingsResult{}, err
		}
	}
	if request.System.AutoAuthenticate != previousSystem.AutoAuthenticate || (!request.System.CloseToTray && previousSystem.CloseToTray) {
		if err := autostart.Configure(request.System.AutoAuthenticate); err != nil {
			return SettingsResult{}, err
		}
	}
	profile, diagnostics, system, err := a.settings.SaveConfiguration(request.Profile, normalizedDiagnostics, request.System)
	if err != nil {
		return SettingsResult{}, err
	}
	a.cacheLatencyTargets(diagnostics.LatencyTargets)
	if request.System.AutoAuthenticate && request.System.AutoAuthenticate != previousSystem.AutoAuthenticate {
		if err := autostart.Start(); err != nil {
			return SettingsResult{}, err
		}
	}
	if !system.CloseToTray {
		tray.QuitExisting()
	}
	interfaces, _ := listNetworkInterfaces()
	return SettingsResult{Profile: profile, Diagnostics: diagnostics, System: system, NetworkInterfaces: interfaces}, nil
}

func listNetworkInterfaces() ([]systemnet.NetworkInterface, error) {
	interfaces, err := systemnet.List()
	if interfaces == nil {
		interfaces = make([]systemnet.NetworkInterface, 0)
	}
	return interfaces, err
}

func (a *App) Logout() error {
	err := a.auth.Stop(true)
	if a.ctx != nil {
		level, message := "info", "已发送 EAPOL-Logoff，当前认证状态已注销"
		if err != nil {
			level, message = "warning", "认证状态已清除，但发送 EAPOL-Logoff 失败: "+err.Error()
		}
		runtime.EventsEmit(a.ctx, "auth:event", auth.Event{
			State: auth.StateIdle, Level: level, Message: message, Timestamp: time.Now().Format("15:04:05"),
		})
	}
	return err
}

func (a *App) CancelAuthentication() error {
	err := a.auth.Stop(false)
	a.emitAuthEvent(auth.Event{State: auth.StateIdle, Level: "info", Message: "已取消本次认证", Timestamp: time.Now().Format("15:04:05")})
	return err
}

func (a *App) Disconnect() error {
	return a.Logout()
}

func (a *App) emitAuthEvent(event auth.Event) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "auth:event", event)
	}
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
	targets := a.currentLatencyTargets()
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
	return networkdiag.CheckLatency(ctx, id, targets)
}

func (a *App) cacheLatencyTargets(values []settings.LatencyTarget) {
	targets := latencyTargets(values)
	a.latencyMu.Lock()
	a.latencyTargets = targets
	a.latencyConfigured = true
	a.latencyMu.Unlock()
}

func (a *App) currentLatencyTargets() []networkdiag.LatencyTarget {
	a.latencyMu.Lock()
	if a.latencyConfigured {
		targets := append([]networkdiag.LatencyTarget(nil), a.latencyTargets...)
		a.latencyMu.Unlock()
		return targets
	}
	a.latencyMu.Unlock()

	diagnostics := a.loadDiagnostics()
	a.cacheLatencyTargets(diagnostics.LatencyTargets)

	a.latencyMu.Lock()
	targets := append([]networkdiag.LatencyTarget(nil), a.latencyTargets...)
	a.latencyMu.Unlock()
	return targets
}

func (a *App) CancelLatencyChecks() {
	a.latencyMu.Lock()
	checks := a.latencyChecks
	a.latencyChecks = nil
	for _, check := range checks {
		check.cancel()
	}
	a.latencyMu.Unlock()
	networkdiag.CloseLatencyConnections()
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

func latencyTargets(values []settings.LatencyTarget) []networkdiag.LatencyTarget {
	result := make([]networkdiag.LatencyTarget, len(values))
	for index, value := range values {
		result[index] = networkdiag.LatencyTarget{ID: value.ID, Name: value.Name, URL: value.URL, Region: value.Region}
	}
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

func (a *App) OpenLink(url string) {
	if a.ctx != nil && isAllowedLink(url) {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

func isAllowedLink(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "npcap.com", "natchecker.com":
		return true
	case "github.com":
		const repositoryPath = "/yinin6/ruijie-sysu-go"
		return parsed.Path == repositoryPath || strings.HasPrefix(parsed.Path, repositoryPath+"/")
	default:
		return false
	}
}
