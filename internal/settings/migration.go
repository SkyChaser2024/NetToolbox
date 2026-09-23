package settings

import (
	"slices"
	"strings"
)

func withDiagnosticDefaults(value DiagnosticSettings) DiagnosticSettings {
	defaults := DefaultDiagnostics()
	if len(value.LatencyTargets) == 0 {
		value.LatencyTargets = defaults.LatencyTargets
	}
	value.NATServers = replaceEndpoint(value.NATServers, "stun.qq.com:3478", "stun.hitv.com:3478")
	value.IPv4Endpoints = removeEndpoint(value.IPv4Endpoints, "https://v4.17nas.com")
	value.IPv6Endpoints = removeEndpoint(value.IPv6Endpoints, "https://v6.17nas.com")
	if len(value.NATServers) == 0 {
		value.NATServers = defaults.NATServers
	}
	if len(value.IPv4Endpoints) == 0 {
		value.IPv4Endpoints = defaults.IPv4Endpoints
	}
	if len(value.IPv6Endpoints) == 0 {
		value.IPv6Endpoints = defaults.IPv6Endpoints
	}
	if len(value.IPv6Sites) == 0 {
		value.IPv6Sites = defaults.IPv6Sites
	}
	if strings.TrimSpace(value.AAAADomain) == "" {
		value.AAAADomain = defaults.AAAADomain
	}
	return value
}

func profileFromDocument(doc document) Profile {
	doc.Profile.PasswordSet = doc.ProtectedPassword != ""
	return doc.Profile
}

func diagnosticsFromDocument(doc document) DiagnosticSettings {
	if isLegacyDiagnostics(doc.Diagnostics) || isPreviousDefaultDiagnostics(doc.Diagnostics) {
		return DefaultDiagnostics()
	}
	return withDiagnosticDefaults(doc.Diagnostics)
}

func isPreviousDefaultDiagnostics(value DiagnosticSettings) bool {
	return slices.Equal(value.NATServers, []string{"stun.miwifi.com:3478", "stun.qq.com:3478", "stun.chat.bilibili.com:3478", "stun.cloudflare.com:3478"}) &&
		slices.Equal(value.IPv4Endpoints, []string{"https://myip.ipip.net", "https://v4.17nas.com", "https://ipv4.icanhazip.com"}) &&
		slices.Equal(value.IPv6Endpoints, []string{"https://v6.17nas.com", "https://api-ipv6.ip.sb/ip", "https://ipv6.icanhazip.com"}) &&
		value.AAAADomain == "www.qq.com" && value.IPv6LargeURL == ""
}

func isLegacyDiagnostics(value DiagnosticSettings) bool {
	return slices.Equal(value.NATServers, []string{"stun.voipgate.com:3478", "stun.cloudflare.com:3478", "stun.l.google.com:19302", "stun1.l.google.com:19302"}) &&
		slices.Equal(value.IPv4Endpoints, []string{"https://4.ipw.cn", "https://ipv4.icanhazip.com", "https://v4.17nas.com"}) &&
		slices.Equal(value.IPv6Endpoints, []string{"https://6.ipw.cn", "https://ipv6.icanhazip.com", "https://v6.17nas.com"}) &&
		value.AAAADomain == "www.cloudflare.com" && value.IPv6LargeURL == ""
}

func withSystemDefaults(value SystemPreferences) SystemPreferences {
	if value.PriorityMode == "" {
		value.PriorityMode = DefaultSystemPreferences().PriorityMode
	}
	return value
}

func systemFromDocument(doc document) SystemPreferences {
	value := doc.System
	if doc.Version < 3 {
		value.CloseToTray = true
	}
	if doc.Version < 4 {
		// Older versions combined login startup with automatic authentication.
		value.AutoAuthenticate = value.AutoAuthenticate && value.CloseToTray
		value.StartAtLogin = value.AutoAuthenticate
	}
	return withSystemDefaults(value)
}

func removeEndpoint(values []string, endpoint string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !strings.EqualFold(strings.TrimSpace(value), endpoint) {
			result = append(result, value)
		}
	}
	return result
}

func replaceEndpoint(values []string, oldEndpoint, newEndpoint string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), oldEndpoint) {
			result[index] = newEndpoint
		} else {
			result[index] = value
		}
	}
	return cleanLines(result)
}
