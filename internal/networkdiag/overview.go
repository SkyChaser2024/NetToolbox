package networkdiag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"

	"campusnet-toolbox/internal/appmeta"
)

type PublicNetworkInfo struct {
	Available       bool   `json:"available"`
	Address         string `json:"address,omitempty"`
	Source          string `json:"source,omitempty"`
	ISP             string `json:"isp,omitempty"`
	ASN             int    `json:"asn,omitempty"`
	ASNOrganization string `json:"asnOrganization,omitempty"`
	Country         string `json:"country,omitempty"`
	Region          string `json:"region,omitempty"`
	City            string `json:"city,omitempty"`
	Error           string `json:"error,omitempty"`
}

type LatencyProbe struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	URL        string `json:"url"`
	Region     string `json:"region"`
	Status     string `json:"status"`
	Address    string `json:"address,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	LatencyMs  int64  `json:"latencyMs,omitempty"`
	Error      string `json:"error,omitempty"`
}

type OverviewResult struct {
	IPv4      PublicNetworkInfo `json:"ipv4"`
	IPv6      PublicNetworkInfo `json:"ipv6"`
	Probes    []LatencyProbe    `json:"probes"`
	CheckedAt string            `json:"checkedAt"`
}

type LatencyTarget struct {
	ID     string
	Name   string
	URL    string
	Region string
}

type publicInfoProvider struct {
	url    string
	format string
}

const (
	publicInfoIPSB              = "ip-sb"
	publicInfoWhois             = "ip-whois"
	publicInfoPlain             = "plain"
	maxPublicInfoProviders      = 18
	maxPublicInfoResponseBytes  = 64 * 1024
	maxConcurrentLatencyTargets = 16
)

func CheckOverview(ctx context.Context, targets []LatencyTarget, ipv4Endpoints, ipv6Endpoints []string) OverviewResult {
	result := OverviewResult{CheckedAt: time.Now().Format("15:04:05")}
	var wait sync.WaitGroup
	wait.Add(3)
	go func() {
		defer wait.Done()
		result.IPv4 = fetchPublicNetworkInfo(ctx, "tcp4", ipv4Endpoints)
	}()
	go func() {
		defer wait.Done()
		result.IPv6 = fetchPublicNetworkInfo(ctx, "tcp6", ipv6Endpoints)
	}()
	go func() {
		defer wait.Done()
		result.Probes = probeLatencyTargets(ctx, targets)
	}()
	wait.Wait()
	return result
}

func CheckLatency(ctx context.Context, id string, targets []LatencyTarget) LatencyProbe {
	for _, target := range targets {
		if target.ID == id {
			return probeHTTP(ctx, target)
		}
	}
	return LatencyProbe{ID: id, Status: "failed", Error: "未知的连接测试目标"}
}

// CheckPublicNetworkInfo refreshes one address family without running the
// latency probes or querying the other protocol family.
func CheckPublicNetworkInfo(ctx context.Context, network string, endpoints []string) PublicNetworkInfo {
	return fetchPublicNetworkInfo(ctx, network, endpoints)
}

func fetchPublicNetworkInfo(ctx context.Context, network string, endpoints []string) PublicNetworkInfo {
	structured := []publicInfoProvider{
		{url: "https://api.ip.sb/geoip", format: publicInfoIPSB},
		{url: "https://ipwho.is/", format: publicInfoWhois},
	}
	return fetchPublicNetworkInfoUsing(ctx, network, endpoints, structured)
}

func fetchPublicNetworkInfoUsing(ctx context.Context, network string, endpoints []string, structured []publicInfoProvider) PublicNetworkInfo {
	lookupContext, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()

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
	client := &http.Client{Transport: transport, Timeout: 4 * time.Second}

	providers := append([]publicInfoProvider(nil), structured...)
	seen := make(map[string]bool, len(providers)+len(endpoints))
	for _, provider := range providers {
		seen[provider.url] = true
	}
	for _, endpoint := range endpoints {
		if len(providers) >= maxPublicInfoProviders {
			break
		}
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" || seen[endpoint] {
			continue
		}
		seen[endpoint] = true
		providers = append(providers, publicInfoProvider{url: endpoint, format: publicInfoPlain})
	}
	if info, err := fetchFirstPublicInfo(lookupContext, client, network, providers); err == nil {
		return withPublicInfoCompleteness(info)
	}
	return PublicNetworkInfo{Error: "公网信息服务暂时不可用，请稍后重试"}
}

func fetchFirstPublicInfo(ctx context.Context, client *http.Client, network string, providers []publicInfoProvider) (PublicNetworkInfo, error) {
	providerContext, cancel := context.WithCancel(ctx)
	defer cancel()
	type response struct {
		info     PublicNetworkInfo
		err      error
		detailed bool
	}
	responses := make(chan response, len(providers))
	for _, provider := range providers {
		go func() {
			info, err := fetchPublicInfoProvider(providerContext, client, network, provider)
			responses <- response{info: info, err: err, detailed: provider.format != publicInfoPlain}
		}()
	}
	var failures []string
	var fallback *PublicNetworkInfo
	var graceTimer *time.Timer
	var grace <-chan time.Time
	defer func() {
		if graceTimer != nil {
			graceTimer.Stop()
		}
	}()
	for range providers {
		select {
		case <-ctx.Done():
			if fallback != nil {
				return *fallback, nil
			}
			return PublicNetworkInfo{}, ctx.Err()
		case <-grace:
			return *fallback, nil
		case result := <-responses:
			if result.err == nil {
				if result.detailed {
					return result.info, nil
				}
				if fallback == nil {
					fallback = &result.info
					graceTimer = time.NewTimer(600 * time.Millisecond)
					grace = graceTimer.C
				}
				continue
			}
			failures = append(failures, result.err.Error())
		}
	}
	if fallback != nil {
		return *fallback, nil
	}
	return PublicNetworkInfo{}, errors.New(strings.Join(failures, "; "))
}

func withPublicInfoCompleteness(info PublicNetworkInfo) PublicNetworkInfo {
	if !info.Available {
		return info
	}
	missing := make([]string, 0, 4)
	if strings.TrimSpace(info.ISP) == "" {
		missing = append(missing, "ISP")
	}
	if info.ASN == 0 {
		missing = append(missing, "ASN")
	}
	if strings.TrimSpace(info.ASNOrganization) == "" {
		missing = append(missing, "网络")
	}
	if strings.TrimSpace(info.Country) == "" && strings.TrimSpace(info.Region) == "" && strings.TrimSpace(info.City) == "" {
		missing = append(missing, "位置")
	}
	if len(missing) > 0 {
		info.Error = "公网地址已获取，但详细信息暂时不完整：缺少 " + strings.Join(missing, "、")
	}
	return info
}

func fetchPublicInfoProvider(ctx context.Context, client *http.Client, network string, provider publicInfoProvider) (PublicNetworkInfo, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.url, nil)
	if err != nil {
		return PublicNetworkInfo{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", appmeta.UserAgent)
	response, err := client.Do(request)
	if err != nil {
		return PublicNetworkInfo{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return PublicNetworkInfo{}, fmt.Errorf("HTTP %d", response.StatusCode)
	}

	var info PublicNetworkInfo
	switch provider.format {
	case publicInfoIPSB:
		info, err = decodeIPSB(response.Body, network)
	case publicInfoWhois:
		info, err = decodeIPWhois(response.Body, network)
	default:
		payload, readErr := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		if readErr != nil {
			return PublicNetworkInfo{}, readErr
		}
		address := parseIPAddress(payload)
		if err := validatePublicAddress(address, network); err != nil {
			return PublicNetworkInfo{}, err
		}
		info = PublicNetworkInfo{Available: true, Address: address}
	}
	if err != nil {
		return PublicNetworkInfo{}, err
	}
	info.Source = provider.url
	return info, nil
}

func decodeIPSB(reader io.Reader, network string) (PublicNetworkInfo, error) {
	var payload struct {
		IP              string `json:"ip"`
		ISP             string `json:"isp"`
		ASN             int    `json:"asn"`
		ASNOrganization string `json:"asn_organization"`
		Country         string `json:"country"`
		Region          string `json:"region"`
		City            string `json:"city"`
	}
	if err := decodeJSONResponse(reader, &payload); err != nil {
		return PublicNetworkInfo{}, err
	}
	payload.IP = strings.TrimSpace(payload.IP)
	if err := validatePublicAddress(payload.IP, network); err != nil {
		return PublicNetworkInfo{}, err
	}
	return PublicNetworkInfo{
		Available: true, Address: payload.IP, ISP: payload.ISP,
		ASN: payload.ASN, ASNOrganization: payload.ASNOrganization,
		Country: payload.Country, Region: payload.Region, City: payload.City,
	}, nil
}

func decodeIPWhois(reader io.Reader, network string) (PublicNetworkInfo, error) {
	var payload struct {
		IP         string `json:"ip"`
		Success    bool   `json:"success"`
		Message    string `json:"message"`
		Country    string `json:"country"`
		Region     string `json:"region"`
		City       string `json:"city"`
		Connection struct {
			ASN int    `json:"asn"`
			Org string `json:"org"`
			ISP string `json:"isp"`
		} `json:"connection"`
	}
	if err := decodeJSONResponse(reader, &payload); err != nil {
		return PublicNetworkInfo{}, err
	}
	if !payload.Success {
		return PublicNetworkInfo{}, errors.New(strings.TrimSpace(payload.Message))
	}
	payload.IP = strings.TrimSpace(payload.IP)
	if err := validatePublicAddress(payload.IP, network); err != nil {
		return PublicNetworkInfo{}, err
	}
	return PublicNetworkInfo{
		Available: true, Address: payload.IP, ISP: payload.Connection.ISP,
		ASN: payload.Connection.ASN, ASNOrganization: payload.Connection.Org,
		Country: payload.Country, Region: payload.Region, City: payload.City,
	}, nil
}

func validatePublicAddress(address, network string) error {
	parsed := net.ParseIP(address)
	if parsed == nil {
		return errors.New("公网信息服务未返回有效地址")
	}
	if network == "tcp4" && parsed.To4() == nil {
		return errors.New("公网信息服务未返回 IPv4 地址")
	}
	if network == "tcp6" && parsed.To4() != nil {
		return errors.New("公网信息服务未返回 IPv6 地址")
	}
	return nil
}

func probeLatencyTargets(ctx context.Context, targets []LatencyTarget) []LatencyProbe {
	if len(targets) > maxConcurrentLatencyTargets {
		targets = targets[:maxConcurrentLatencyTargets]
	}
	results := make([]LatencyProbe, len(targets))
	var wait sync.WaitGroup
	wait.Add(len(targets))
	for index, target := range targets {
		go func() {
			defer wait.Done()
			results[index] = probeHTTP(ctx, target)
		}()
	}
	wait.Wait()
	return results
}

func decodeJSONResponse(reader io.Reader, destination any) error {
	payload, err := io.ReadAll(io.LimitReader(reader, maxPublicInfoResponseBytes+1))
	if err != nil {
		return err
	}
	if len(payload) > maxPublicInfoResponseBytes {
		return errors.New("公网信息服务响应过大")
	}
	return json.Unmarshal(payload, destination)
}

func probeHTTP(ctx context.Context, target LatencyTarget) LatencyProbe {
	parsed, err := url.Parse(target.URL)
	if err != nil || parsed.Hostname() == "" {
		return LatencyProbe{ID: target.ID, Name: target.Name, URL: target.URL, Region: target.Region, Status: "failed", Error: "网站地址无效"}
	}
	result := LatencyProbe{ID: target.ID, Name: target.Name, Host: parsed.Hostname(), URL: target.URL, Region: target.Region}
	probeContext, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	defer cancel()
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	transport := &http.Transport{
		Proxy:               nil,
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: 3 * time.Second,
		DisableKeepAlives:   true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   3500 * time.Millisecond,
	}
	request, err := http.NewRequestWithContext(probeContext, http.MethodGet, target.URL, nil)
	if err != nil {
		result.Status = "failed"
		result.Error = compactNetworkError(err)
		return result
	}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			host, _, splitErr := net.SplitHostPort(info.Conn.RemoteAddr().String())
			if splitErr == nil {
				result.Address = host
			}
		},
	}))
	request.Header.Set("User-Agent", appmeta.UserAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
	request.Header.Set("Cache-Control", "no-cache")
	started := time.Now()
	response, err := client.Do(request)
	result.LatencyMs = time.Since(started).Milliseconds()
	if err != nil {
		result.Status = "failed"
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			result.Status = "timeout"
		}
		result.Error = compactNetworkError(err)
		return result
	}
	defer response.Body.Close()
	result.Status = "ok"
	result.StatusCode = response.StatusCode
	return result
}

func isTimeoutError(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}
