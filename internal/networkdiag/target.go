package networkdiag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const diagnosticResolutionTimeout = 5 * time.Second

func normalizeDiagnosticTarget(value, emptyMessage, tooLongMessage string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New(emptyMessage)
	}
	if len(value) > 2048 {
		return "", errors.New(tooLongMessage)
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil || parsed == nil {
			return "", errors.New("仅支持有效的 HTTP/HTTPS URL、域名或 IP 地址")
		}
		scheme := strings.ToLower(parsed.Scheme)
		if (scheme != "http" && scheme != "https") || parsed.Hostname() == "" {
			return "", errors.New("仅支持有效的 HTTP/HTTPS URL、域名或 IP 地址")
		}
		value = parsed.Hostname()
	}
	value = strings.TrimSpace(strings.Trim(value, "[]"))
	if ip := net.ParseIP(value); ip != nil {
		return ip.String(), nil
	}
	value = strings.TrimSuffix(value, ".")
	if len(value) == 0 || len(value) > 253 || strings.ContainsAny(value, " /\\?#@\t\r\n") {
		return "", errors.New("域名格式无效")
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("域名格式无效")
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
				return "", errors.New("域名格式无效")
			}
		}
	}
	return strings.ToLower(value), nil
}

func normalizeDiagnosticProtocol(value string) (string, error) {
	protocol := strings.ToLower(strings.TrimSpace(value))
	if protocol == "" {
		protocol = "auto"
	}
	if protocol != "auto" && protocol != "ipv4" && protocol != "ipv6" {
		return "", errors.New("协议必须是自动、IPv4 或 IPv6")
	}
	return protocol, nil
}

func resolveDiagnosticTarget(ctx context.Context, target, protocol string) (net.IP, string, error) {
	if parsed := net.ParseIP(target); parsed != nil {
		return selectDiagnosticIP([]net.IPAddr{{IP: parsed}}, protocol)
	}
	lookupCtx, cancel := context.WithTimeout(ctx, diagnosticResolutionTimeout)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(lookupCtx, target)
	if err != nil {
		return nil, "", fmt.Errorf("无法解析目标 %s: %w", target, err)
	}
	return selectDiagnosticIP(addresses, protocol)
}

func selectDiagnosticIP(addresses []net.IPAddr, protocol string) (net.IP, string, error) {
	var firstIPv4 net.IP
	var firstIPv6 net.IP
	for _, address := range addresses {
		if ipv4Address := address.IP.To4(); ipv4Address != nil {
			if firstIPv4 == nil {
				firstIPv4 = append(net.IP(nil), ipv4Address...)
			}
			continue
		}
		if ipv6Address := address.IP.To16(); ipv6Address != nil && firstIPv6 == nil {
			firstIPv6 = append(net.IP(nil), ipv6Address...)
		}
	}
	switch protocol {
	case "ipv4":
		if firstIPv4 != nil {
			return firstIPv4, "ipv4", nil
		}
		return nil, "", errors.New("目标没有可用的 IPv4 地址")
	case "ipv6":
		if firstIPv6 != nil {
			return firstIPv6, "ipv6", nil
		}
		return nil, "", errors.New("目标没有可用的 IPv6 地址")
	default:
		if firstIPv4 != nil {
			return firstIPv4, "ipv4", nil
		}
		if firstIPv6 != nil {
			return firstIPv6, "ipv6", nil
		}
		return nil, "", errors.New("目标没有可用的 IP 地址")
	}
}

// Keep the traceroute-specific helpers available to existing package tests and
// callers while sharing the implementation with ping.
func normalizeTraceTarget(value string) (string, error) {
	return normalizeDiagnosticTarget(value, "请输入要追踪的域名或 IP 地址", "追踪目标过长")
}

func resolveTraceTarget(ctx context.Context, target, protocol string) (net.IP, string, error) {
	return resolveDiagnosticTarget(ctx, target, protocol)
}

func selectTraceIP(addresses []net.IPAddr, protocol string) (net.IP, string, error) {
	return selectDiagnosticIP(addresses, protocol)
}
