package settings

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"campusnet-toolbox/internal/privatefile"
)

const (
	currentVersion          = 3
	maxConfigurationBytes   = 1 << 20
	maxDiagnosticListLength = 16
	maxDiagnosticURLLength  = 2048
	maxProfileFieldBytes    = 64 * 1024
)

type Profile struct {
	DeviceName       string `json:"deviceName"`
	AdapterLabel     string `json:"adapterLabel"`
	LocalMAC         string `json:"localMac"`
	Username         string `json:"username"`
	Identity         string `json:"identity"`
	IdentitySuffix   string `json:"identitySuffix"`
	StartDelayMs     int    `json:"startDelayMs"`
	RetryDelayMs     int    `json:"retryDelayMs"`
	Debug            bool   `json:"debug"`
	RememberPassword bool   `json:"rememberPassword"`
	PasswordSet      bool   `json:"passwordSet"`
}

type DiagnosticSettings struct {
	LatencyTargets []LatencyTarget `json:"latencyTargets"`
	NATServers     []string        `json:"natServers"`
	IPv4Endpoints  []string        `json:"ipv4Endpoints"`
	IPv6Endpoints  []string        `json:"ipv6Endpoints"`
	IPv6Sites      []string        `json:"ipv6Sites"`
	AAAADomain     string          `json:"aaaaDomain"`
	IPv6LargeURL   string          `json:"ipv6LargeUrl"`
}

type LatencyTarget struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Region string `json:"region"`
}

type SystemPreferences struct {
	PriorityMode     string `json:"priorityMode"`
	AutoAuthenticate bool   `json:"autoAuthenticate"`
	CloseToTray      bool   `json:"closeToTray"`
}

type document struct {
	Version           int                `json:"version"`
	Profile           Profile            `json:"profile"`
	Diagnostics       DiagnosticSettings `json:"diagnostics"`
	System            SystemPreferences  `json:"system"`
	ProtectedPassword string             `json:"protectedPassword,omitempty"`
}

type Store struct {
	mu   sync.Mutex
	path string
}

func NewStore() (*Store, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(root, "CampusNetToolbox", "config.json")}, nil
}

func DefaultProfile() Profile {
	return Profile{RetryDelayMs: 2000, RememberPassword: true}
}

func DefaultDiagnostics() DiagnosticSettings {
	return DiagnosticSettings{
		LatencyTargets: []LatencyTarget{
			{ID: "douyin", Name: "字节抖音", URL: "https://www.douyin.com/", Region: "国内"},
			{ID: "bilibili", Name: "Bilibili", URL: "https://www.bilibili.com/", Region: "国内"},
			{ID: "wechat", Name: "腾讯微信", URL: "https://weixin.qq.com/", Region: "国内"},
			{ID: "taobao", Name: "阿里淘宝", URL: "https://www.taobao.com/", Region: "国内"},
			{ID: "github", Name: "GitHub", URL: "https://github.com/", Region: "国际"},
			{ID: "telegram", Name: "Telegram", URL: "https://telegram.org/", Region: "国际"},
			{ID: "x", Name: "X.com", URL: "https://x.com/", Region: "国际"},
			{ID: "youtube", Name: "YouTube", URL: "https://www.youtube.com/", Region: "国际"},
		},
		NATServers: []string{
			"stun.miwifi.com:3478",
			"stun.hitv.com:3478",
			"stun.chat.bilibili.com:3478",
			"stun.cloudflare.com:3478",
		},
		IPv4Endpoints: []string{
			"https://4.ipw.cn",
			"https://api-ipv4.ip.sb/ip",
			"https://myip.ipip.net",
			"https://ipv4.icanhazip.com",
		},
		IPv6Endpoints: []string{
			"https://6.ipw.cn",
			"https://api-ipv6.ip.sb/ip",
			"https://ipv6.icanhazip.com",
		},
		IPv6Sites: []string{
			"https://www.qq.com",
			"https://www.baidu.com",
			"https://www.taobao.com",
			"https://www.jd.com",
		},
		AAAADomain: "www.qq.com",
	}
}

func DefaultSystemPreferences() SystemPreferences {
	return SystemPreferences{PriorityMode: "automatic", CloseToTray: true}
}

// LoadConfiguration reads the settings file once so callers receive a
// consistent snapshot of all related settings.
func (s *Store) LoadConfiguration() (Profile, DiagnosticSettings, SystemPreferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDocument()
	if errors.Is(err, os.ErrNotExist) {
		return DefaultProfile(), DefaultDiagnostics(), DefaultSystemPreferences(), nil
	}
	if err != nil {
		return Profile{}, DiagnosticSettings{}, SystemPreferences{}, err
	}
	return profileFromDocument(doc), diagnosticsFromDocument(doc), systemFromDocument(doc), nil
}

func (s *Store) Password() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDocument()
	if err != nil {
		return "", err
	}
	if doc.ProtectedPassword == "" {
		return "", nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(doc.ProtectedPassword)
	if err != nil {
		return "", errors.New("保存的密码数据已损坏")
	}
	plain, err := unprotect(ciphertext)
	if err != nil {
		return "", err
	}
	defer clear(plain)
	return string(plain), nil
}

func (s *Store) LoadDiagnostics() (DiagnosticSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDocument()
	if errors.Is(err, os.ErrNotExist) {
		return DefaultDiagnostics(), nil
	}
	if err != nil {
		return DiagnosticSettings{}, err
	}
	return diagnosticsFromDocument(doc), nil
}

func (s *Store) LoadSystemPreferences() (SystemPreferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDocument()
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSystemPreferences(), nil
	}
	if err != nil {
		return SystemPreferences{}, err
	}
	return systemFromDocument(doc), nil
}

func (s *Store) Save(profile Profile, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDocument()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	doc.System = systemFromDocument(doc)
	if doc.Version == 0 {
		doc.Diagnostics = DefaultDiagnostics()
	}
	doc.Version = currentVersion
	if !profile.RememberPassword {
		doc.ProtectedPassword = ""
	} else if newPassword != "" {
		plain := []byte(newPassword)
		ciphertext, protectErr := protect(plain)
		clear(plain)
		if protectErr != nil {
			return protectErr
		}
		doc.ProtectedPassword = base64.StdEncoding.EncodeToString(ciphertext)
	}
	profile.PasswordSet = doc.ProtectedPassword != ""
	doc.Profile = profile
	return s.writeDocument(doc)
}

func (s *Store) SaveConfiguration(profile Profile, diagnostics DiagnosticSettings, system SystemPreferences) (Profile, DiagnosticSettings, SystemPreferences, error) {
	normalized, err := NormalizeDiagnostics(diagnostics)
	if err != nil {
		return Profile{}, DiagnosticSettings{}, SystemPreferences{}, err
	}
	system = withSystemDefaults(system)
	if system.PriorityMode != "automatic" && system.PriorityMode != "ethernet" && system.PriorityMode != "wifi" {
		return Profile{}, DiagnosticSettings{}, SystemPreferences{}, errors.New("未知的网卡优先级模式")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDocument()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Profile{}, DiagnosticSettings{}, SystemPreferences{}, err
	}
	doc.Version = currentVersion
	profile.PasswordSet = doc.ProtectedPassword != ""
	doc.Profile = profile
	doc.Diagnostics = normalized
	doc.System = system
	if err := s.writeDocument(doc); err != nil {
		return Profile{}, DiagnosticSettings{}, SystemPreferences{}, err
	}
	return profile, normalized, system, nil
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
	if !value.CloseToTray {
		value.AutoAuthenticate = false
	}
	return value
}

func systemFromDocument(doc document) SystemPreferences {
	value := doc.System
	if doc.Version < 3 {
		value.CloseToTray = true
	}
	return withSystemDefaults(value)
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

func (s *Store) writeDocument(doc document) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > maxConfigurationBytes {
		return errors.New("配置内容过大")
	}
	return privatefile.Write(s.path, data)
}

func (s *Store) loadDocument() (document, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return document{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigurationBytes+1))
	if err != nil {
		return document{}, err
	}
	if len(data) > maxConfigurationBytes {
		return document{}, errors.New("配置文件过大，已拒绝读取")
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return document{}, errors.New("配置文件无法解析")
	}
	if doc.Version < 0 || doc.Version > currentVersion {
		return document{}, errors.New("配置文件版本不受支持")
	}
	if doc.Version > 0 {
		if len(doc.Profile.DeviceName) > maxProfileFieldBytes || len(doc.Profile.AdapterLabel) > maxProfileFieldBytes ||
			len(doc.Profile.LocalMAC) > maxProfileFieldBytes || len(doc.Profile.Username) > maxProfileFieldBytes ||
			len(doc.Profile.Identity) > maxProfileFieldBytes || len(doc.Profile.IdentitySuffix) > maxProfileFieldBytes {
			return document{}, errors.New("配置文件包含过长的认证字段")
		}
		diagnostics, normalizeErr := NormalizeDiagnostics(diagnosticsFromDocument(doc))
		if normalizeErr != nil {
			return document{}, fmt.Errorf("配置文件包含无效的网络检测设置: %w", normalizeErr)
		}
		doc.Diagnostics = diagnostics
		doc.System = systemFromDocument(doc)
		if doc.System.PriorityMode != "automatic" && doc.System.PriorityMode != "ethernet" && doc.System.PriorityMode != "wifi" {
			return document{}, errors.New("配置文件包含未知的网卡优先级模式")
		}
	}
	return doc, nil
}
