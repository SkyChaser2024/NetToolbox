package desktop

import (
	"campusnet-toolbox/internal/adapters"
	"campusnet-toolbox/internal/auth"
	"campusnet-toolbox/internal/networkdiag"
	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/systemnet"
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
	BackgroundWarning string                       `json:"backgroundWarning,omitempty"`
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
	Warning           string                       `json:"warning,omitempty"`
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

type TraceRequest struct {
	SessionID        string `json:"sessionId"`
	Target           string `json:"target"`
	Protocol         string `json:"protocol"`
	MaxHops          int    `json:"maxHops"`
	TimeoutMs        int    `json:"timeoutMs"`
	ResolveHostnames bool   `json:"resolveHostnames"`
}

type TraceEvent struct {
	SessionID  string                `json:"sessionId"`
	Type       string                `json:"type"`
	Target     string                `json:"target,omitempty"`
	Address    string                `json:"address,omitempty"`
	Protocol   string                `json:"protocol,omitempty"`
	Hop        *networkdiag.TraceHop `json:"hop,omitempty"`
	Status     string                `json:"status,omitempty"`
	Reached    bool                  `json:"reached,omitempty"`
	HopCount   int                   `json:"hopCount,omitempty"`
	DurationMs int64                 `json:"durationMs,omitempty"`
	Error      string                `json:"error,omitempty"`
}

type PingRequest struct {
	SessionID  string `json:"sessionId"`
	Target     string `json:"target"`
	Protocol   string `json:"protocol"`
	Count      int    `json:"count"`
	TimeoutMs  int    `json:"timeoutMs"`
	IntervalMs int    `json:"intervalMs"`
}

type PingEvent struct {
	SessionID string                   `json:"sessionId"`
	Type      string                   `json:"type"`
	Target    string                   `json:"target,omitempty"`
	Address   string                   `json:"address,omitempty"`
	Protocol  string                   `json:"protocol,omitempty"`
	Reply     *networkdiag.PingReply   `json:"reply,omitempty"`
	Summary   *networkdiag.PingSummary `json:"summary,omitempty"`
	Error     string                   `json:"error,omitempty"`
}
