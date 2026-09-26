package settings

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"campusnet-toolbox/internal/appdata"
)

const (
	currentVersion          = 4
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
	StartAtLogin     bool   `json:"startAtLogin"`
	SilentStart      bool   `json:"silentStart"`
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
	paths, err := appdata.Current()
	if err != nil {
		return nil, err
	}
	return newStoreWithPaths(paths)
}

func newStoreWithPaths(paths appdata.Paths) (*Store, error) {
	if err := migrateCurrentDirectory(paths); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(paths.ConfigDir, "config.json")}, nil
}

func (s *Store) NeedsStartupMigration() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadDocument()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil && doc.Version < 4, err
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
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
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
	if err := ValidateProfile(profile); err != nil {
		return err
	}
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
	if err := ValidateProfile(profile); err != nil {
		return Profile{}, DiagnosticSettings{}, SystemPreferences{}, err
	}
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
