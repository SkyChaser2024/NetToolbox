package networkdiag

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"campusnet-toolbox/internal/appmeta"
)

type IPProbeResult struct {
	Available bool   `json:"available"`
	Address   string `json:"address,omitempty"`
	LatencyMs int64  `json:"latencyMs,omitempty"`
	Error     string `json:"error,omitempty"`
}

type URLProbeResult struct {
	Available bool   `json:"available"`
	LatencyMs int64  `json:"latencyMs,omitempty"`
	BytesRead int64  `json:"bytesRead,omitempty"`
	Error     string `json:"error,omitempty"`
}

type IPv6Config struct {
	IPv4Endpoints []string
	IPv6Endpoints []string
	IPv6Sites     []string
	AAAADomain    string
	LargeURL      string
}

type WebsiteProbeResult struct {
	URL        string `json:"url"`
	Host       string `json:"host"`
	Available  bool   `json:"available"`
	Address    string `json:"address,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	LatencyMs  int64  `json:"latencyMs,omitempty"`
	Error      string `json:"error,omitempty"`
}

type IPv6Result struct {
	ConnectionType string               `json:"connectionType"`
	Summary        string               `json:"summary"`
	IPv4           IPProbeResult        `json:"ipv4"`
	IPv6           IPProbeResult        `json:"ipv6"`
	LocalGlobal    []string             `json:"localGlobal"`
	LocalLink      []string             `json:"localLink"`
	DNSAAAA        bool                 `json:"dnsAAAA"`
	LargePacket    *URLProbeResult      `json:"largePacket,omitempty"`
	Sites          []WebsiteProbeResult `json:"sites"`
}

func CheckIPv6(ctx context.Context, cfg IPv6Config) IPv6Result {
	result := IPv6Result{}
	result.LocalGlobal, result.LocalLink = localIPv6Addresses()
	var wait sync.WaitGroup
	taskCount := 3
	if cfg.LargeURL != "" {
		taskCount++
	}
	if len(cfg.IPv6Sites) > 0 {
		taskCount++
	}
	wait.Add(taskCount)
	go func() {
		defer wait.Done()
		result.IPv4 = probeFirstIP(ctx, "tcp4", cfg.IPv4Endpoints)
	}()
	go func() {
		defer wait.Done()
		result.IPv6 = probeFirstIP(ctx, "tcp6", cfg.IPv6Endpoints)
	}()
	go func() {
		defer wait.Done()
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", cfg.AAAADomain)
		if err == nil {
			for _, ip := range ips {
				if ip.To4() == nil {
					result.DNSAAAA = true
					break
				}
			}
		}
	}()
	if cfg.LargeURL != "" {
		go func() {
			defer wait.Done()
			probe := probeIPv6Payload(ctx, cfg.LargeURL)
			result.LargePacket = &probe
		}()
	}
	if len(cfg.IPv6Sites) > 0 {
		go func() {
			defer wait.Done()
			result.Sites = probeIPv6Sites(ctx, cfg.IPv6Sites)
		}()
	}
	wait.Wait()
	switch {
	case result.IPv4.Available && result.IPv6.Available:
		result.ConnectionType = "双栈 Dual Stack"
		result.Summary = "IPv4 与 IPv6 公网连接均可用"
	case result.IPv6.Available:
		result.ConnectionType = "仅 IPv6"
		result.Summary = "已接入 IPv6，但 IPv4 公网探测失败"
	case result.IPv4.Available:
		result.ConnectionType = "仅 IPv4"
		if len(result.LocalGlobal) > 0 {
			result.Summary = "网卡已有全局 IPv6 地址，但公网 IPv6 连接未打通"
		} else {
			result.Summary = "未发现可用的公网 IPv6 连接"
		}
	default:
		result.ConnectionType = "无公网连接"
		result.Summary = "IPv4 与 IPv6 探测均失败，请检查认证、代理或防火墙"
	}
	return result
}

func probeIPv6Sites(ctx context.Context, sites []string) []WebsiteProbeResult {
	if len(sites) > maxConcurrentLatencyTargets {
		sites = sites[:maxConcurrentLatencyTargets]
	}
	results := make([]WebsiteProbeResult, len(sites))
	var wait sync.WaitGroup
	wait.Add(len(sites))
	for index, site := range sites {
		go func() {
			defer wait.Done()
			probeContext, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			results[index] = probeIPv6Website(probeContext, site)
		}()
	}
	wait.Wait()
	return results
}

func probeIPv6Website(ctx context.Context, endpoint string) WebsiteProbeResult {
	result := WebsiteProbeResult{URL: endpoint, Host: endpoint}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" {
		result.Error = "网站地址无效"
		return result
	}
	result.Host = parsed.Hostname()
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(dialCtx context.Context, _, address string) (net.Conn, error) {
			connection, dialErr := dialer.DialContext(dialCtx, "tcp6", address)
			if dialErr == nil {
				host, _, splitErr := net.SplitHostPort(connection.RemoteAddr().String())
				if splitErr == nil {
					result.Address = host
				}
			}
			return connection, dialErr
		},
		TLSHandshakeTimeout: 4 * time.Second,
		DisableKeepAlives:   true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 6 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	request.Header.Set("User-Agent", appmeta.UserAgent)
	started := time.Now()
	response, err := client.Do(request)
	result.LatencyMs = time.Since(started).Milliseconds()
	if err != nil {
		result.Error = compactNetworkError(err)
		return result
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode
	result.Available = true
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4*1024))
	return result
}

func probeFirstIP(ctx context.Context, network string, endpoints []string) IPProbeResult {
	if len(endpoints) == 0 {
		return IPProbeResult{Error: "未配置检测端点"}
	}
	var last IPProbeResult
	for _, endpoint := range endpoints {
		probeContext, cancel := context.WithTimeout(ctx, 4*time.Second)
		last = probeIP(probeContext, network, endpoint)
		cancel()
		if last.Available {
			return last
		}
		if ctx.Err() != nil {
			break
		}
	}
	return last
}

func probeIP(ctx context.Context, network, endpoint string) IPProbeResult {
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(dialCtx context.Context, _, address string) (net.Conn, error) {
			return dialer.DialContext(dialCtx, network, address)
		},
		TLSHandshakeTimeout: 4 * time.Second,
		DisableKeepAlives:   true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 6 * time.Second}
	started := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return IPProbeResult{Error: err.Error()}
	}
	response, err := client.Do(request)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return IPProbeResult{LatencyMs: latency, Error: compactNetworkError(err)}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return IPProbeResult{LatencyMs: latency, Error: fmt.Sprintf("HTTP %d", response.StatusCode)}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16*1024))
	if err != nil {
		return IPProbeResult{LatencyMs: latency, Error: err.Error()}
	}
	address := parseIPAddress(body)
	if err := validatePublicAddress(address, network); err != nil {
		return IPProbeResult{LatencyMs: latency, Error: err.Error()}
	}
	return IPProbeResult{Available: true, Address: address, LatencyMs: latency}
}

func probeIPv6Payload(ctx context.Context, endpoint string) URLProbeResult {
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(dialCtx context.Context, _, address string) (net.Conn, error) {
			return dialer.DialContext(dialCtx, "tcp6", address)
		},
		TLSHandshakeTimeout: 4 * time.Second,
		DisableKeepAlives:   true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return URLProbeResult{Error: err.Error()}
	}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return URLProbeResult{LatencyMs: time.Since(started).Milliseconds(), Error: compactNetworkError(err)}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return URLProbeResult{LatencyMs: time.Since(started).Milliseconds(), Error: fmt.Sprintf("HTTP %d", response.StatusCode)}
	}
	count, err := io.Copy(io.Discard, io.LimitReader(response.Body, 256*1024))
	if err != nil {
		return URLProbeResult{LatencyMs: time.Since(started).Milliseconds(), BytesRead: count, Error: err.Error()}
	}
	return URLProbeResult{Available: true, LatencyMs: time.Since(started).Milliseconds(), BytesRead: count}
}

func parseIPAddress(body []byte) string {
	var payload struct {
		IP string `json:"ip"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.IP != "" {
		return strings.TrimSpace(payload.IP)
	}
	text := strings.TrimSpace(string(body))
	if net.ParseIP(text) != nil {
		return text
	}
	for _, token := range strings.FieldsFunc(text, func(value rune) bool {
		return !unicode.IsDigit(value) && (value < 'a' || value > 'f') && (value < 'A' || value > 'F') && value != '.' && value != ':'
	}) {
		if net.ParseIP(token) != nil {
			return token
		}
	}
	return text
}

func compactNetworkError(err error) string {
	message := err.Error()
	if index := strings.LastIndex(message, ": "); index >= 0 && index+2 < len(message) {
		message = message[index+2:]
	}
	return message
}

func localIPv6Addresses() (global, link []string) {
	global = make([]string, 0)
	link = make([]string, 0)
	interfaces, _ := net.Interfaces()
	seen := map[string]bool{}
	for _, item := range interfaces {
		if item.Flags&net.FlagUp == 0 || item.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := item.Addrs()
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() != nil || seen[ip.String()] {
				continue
			}
			seen[ip.String()] = true
			if ip.IsLinkLocalUnicast() {
				link = append(link, ip.String())
			} else if ip.IsGlobalUnicast() && !ip.IsPrivate() {
				global = append(global, ip.String())
			}
		}
	}
	return global, link
}
