package settings

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ValidateProfile checks storage bounds even for an incomplete profile that has
// not yet been used to authenticate. Authentication checks remain in auth.Validate.
func ValidateProfile(profile Profile) error {
	if len(profile.DeviceName) > maxProfileFieldBytes || len(profile.AdapterLabel) > maxProfileFieldBytes ||
		len(profile.LocalMAC) > maxProfileFieldBytes || len(profile.Username) > maxProfileFieldBytes ||
		len(profile.Identity) > maxProfileFieldBytes || len(profile.IdentitySuffix) > maxProfileFieldBytes {
		return errors.New("认证字段过长")
	}
	return nil
}

func NormalizeDiagnostics(value DiagnosticSettings) (DiagnosticSettings, error) {
	var err error
	value.LatencyTargets, err = normalizeLatencyTargets(value.LatencyTargets)
	if err != nil {
		return DiagnosticSettings{}, err
	}
	value.NATServers = cleanLines(value.NATServers)
	value.IPv4Endpoints = cleanLines(value.IPv4Endpoints)
	value.IPv6Endpoints = cleanLines(value.IPv6Endpoints)
	value.IPv6Sites = cleanLines(value.IPv6Sites)
	value.AAAADomain = strings.TrimSpace(strings.TrimSuffix(value.AAAADomain, "."))
	value.IPv6LargeURL = strings.TrimSpace(value.IPv6LargeURL)
	if len(value.NATServers) < 2 {
		return DiagnosticSettings{}, errors.New("NAT 检测至少需要两个 STUN 服务器")
	}
	if len(value.NATServers) > maxDiagnosticListLength {
		return DiagnosticSettings{}, fmt.Errorf("NAT 检测最多配置 %d 个 STUN 服务器", maxDiagnosticListLength)
	}
	for _, server := range value.NATServers {
		if len(server) > 512 {
			return DiagnosticSettings{}, errors.New("STUN 地址过长")
		}
		host, port, err := net.SplitHostPort(server)
		if err != nil || host == "" {
			return DiagnosticSettings{}, fmt.Errorf("STUN 地址格式无效: %s", server)
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return DiagnosticSettings{}, fmt.Errorf("STUN 端口无效: %s", server)
		}
	}
	if len(value.IPv4Endpoints) == 0 || len(value.IPv6Endpoints) == 0 {
		return DiagnosticSettings{}, errors.New("IPv4 和 IPv6 公网检测网站均不能留空")
	}
	if len(value.IPv4Endpoints) > maxDiagnosticListLength || len(value.IPv6Endpoints) > maxDiagnosticListLength {
		return DiagnosticSettings{}, fmt.Errorf("每种公网协议最多配置 %d 个检测网站", maxDiagnosticListLength)
	}
	for _, endpoint := range append(append([]string{}, value.IPv4Endpoints...), value.IPv6Endpoints...) {
		if err := validateHTTPURL(endpoint); err != nil {
			return DiagnosticSettings{}, err
		}
	}
	if len(value.IPv6Sites) == 0 {
		return DiagnosticSettings{}, errors.New("IPv6 主流网站列表不能留空")
	}
	if len(value.IPv6Sites) > maxDiagnosticListLength {
		return DiagnosticSettings{}, fmt.Errorf("IPv6 主流网站最多配置 %d 个", maxDiagnosticListLength)
	}
	for _, site := range value.IPv6Sites {
		if err := validateHTTPURL(site); err != nil {
			return DiagnosticSettings{}, fmt.Errorf("IPv6 网站地址无效: %w", err)
		}
	}
	if value.AAAADomain == "" || len(value.AAAADomain) > 253 || strings.ContainsAny(value.AAAADomain, " /\\") {
		return DiagnosticSettings{}, errors.New("AAAA 测试域名无效")
	}
	if value.IPv6LargeURL != "" {
		if err := validateHTTPURL(value.IPv6LargeURL); err != nil {
			return DiagnosticSettings{}, fmt.Errorf("IPv6 数据测试地址无效: %w", err)
		}
	}
	return value, nil
}

func normalizeLatencyTargets(values []LatencyTarget) ([]LatencyTarget, error) {
	if len(values) == 0 {
		return nil, errors.New("延迟测试网站不能留空")
	}
	if len(values) > 16 {
		return nil, errors.New("延迟测试网站最多配置 16 个")
	}
	result := make([]LatencyTarget, 0, len(values))
	seenURLs := make(map[string]bool, len(values))
	seenIDs := make(map[string]bool, len(values))
	for _, value := range values {
		value.Name = strings.TrimSpace(value.Name)
		value.URL = strings.TrimSpace(value.URL)
		value.Region = strings.TrimSpace(value.Region)
		if len(value.URL) > maxDiagnosticURLLength {
			return nil, fmt.Errorf("延迟测试网站地址过长: %s", value.Name)
		}
		if value.Name == "" || len([]rune(value.Name)) > 40 {
			return nil, fmt.Errorf("延迟测试网站名称无效: %s", value.Name)
		}
		parsed, err := url.ParseRequestURI(value.URL)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("延迟测试网站地址无效: %s", value.URL)
		}
		if value.Region != "国内" && value.Region != "国际" {
			return nil, fmt.Errorf("%s 的区域必须是“国内”或“国际”", value.Name)
		}
		normalizedURL := strings.ToLower(parsed.Scheme+"://"+parsed.Host) + parsed.EscapedPath()
		if parsed.RawQuery != "" {
			normalizedURL += "?" + parsed.RawQuery
		}
		if seenURLs[normalizedURL] {
			continue
		}
		value.ID = builtinLatencyID(parsed.Hostname())
		if value.ID == "" {
			hash := sha256.Sum256([]byte(normalizedURL))
			value.ID = fmt.Sprintf("custom-%x", hash[:6])
		}
		if seenIDs[value.ID] {
			return nil, fmt.Errorf("延迟测试网站标识冲突: %s", value.Name)
		}
		seenURLs[normalizedURL] = true
		seenIDs[value.ID] = true
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, errors.New("延迟测试网站不能留空")
	}
	return result, nil
}

func builtinLatencyID(host string) string {
	switch strings.ToLower(strings.TrimPrefix(host, "www.")) {
	case "douyin.com":
		return "douyin"
	case "bilibili.com":
		return "bilibili"
	case "weixin.qq.com":
		return "wechat"
	case "taobao.com":
		return "taobao"
	case "github.com":
		return "github"
	case "telegram.org":
		return "telegram"
	case "x.com":
		return "x"
	case "youtube.com":
		return "youtube"
	default:
		return ""
	}
}

func cleanLines(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result
}

func validateHTTPURL(value string) error {
	if len(value) > maxDiagnosticURLLength {
		return errors.New("HTTP/HTTPS 地址过长")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("HTTP/HTTPS 地址无效: %s", value)
	}
	return nil
}
