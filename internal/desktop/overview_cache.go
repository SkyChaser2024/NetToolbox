package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"campusnet-toolbox/internal/networkdiag"
	"campusnet-toolbox/internal/privatefile"
)

const (
	overviewCacheVersion  = 1
	maxOverviewCacheBytes = 256 * 1024
)

var overviewCacheMu sync.Mutex

type overviewCacheDocument struct {
	Version int                        `json:"version"`
	Result  networkdiag.OverviewResult `json:"result"`
}

func overviewCachePath() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "CampusNetToolbox", "overview-cache.json"), nil
}

func saveOverviewCache(result networkdiag.OverviewResult) error {
	path, err := overviewCachePath()
	if err != nil {
		return err
	}
	overviewCacheMu.Lock()
	defer overviewCacheMu.Unlock()
	return saveOverviewCacheAt(path, result)
}

func saveOverviewCacheAt(path string, result networkdiag.OverviewResult) error {
	result.Probes = nil
	data, err := json.Marshal(overviewCacheDocument{Version: overviewCacheVersion, Result: result})
	if err != nil {
		return err
	}
	return privatefile.Write(path, data)
}

func loadOverviewCache() (networkdiag.OverviewResult, error) {
	path, err := overviewCachePath()
	if err != nil {
		return networkdiag.OverviewResult{}, err
	}
	overviewCacheMu.Lock()
	defer overviewCacheMu.Unlock()
	return loadOverviewCacheAt(path)
}

func updateOverviewCacheProtocol(version string, info networkdiag.PublicNetworkInfo) error {
	path, err := overviewCachePath()
	if err != nil {
		return err
	}
	overviewCacheMu.Lock()
	defer overviewCacheMu.Unlock()
	result, err := loadOverviewCacheAt(path)
	if err != nil {
		result = networkdiag.OverviewResult{}
	}
	if version == "ipv6" {
		result.IPv6 = info
	} else {
		result.IPv4 = info
	}
	result.CheckedAt = time.Now().Format("15:04:05")
	return saveOverviewCacheAt(path, result)
}

func loadOverviewCacheAt(path string) (networkdiag.OverviewResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return networkdiag.OverviewResult{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxOverviewCacheBytes+1))
	if err != nil {
		return networkdiag.OverviewResult{}, err
	}
	if len(data) > maxOverviewCacheBytes {
		return networkdiag.OverviewResult{}, errors.New("网络概览缓存过大")
	}
	var document overviewCacheDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return networkdiag.OverviewResult{}, err
	}
	if document.Version != overviewCacheVersion || document.Result.CheckedAt == "" {
		return networkdiag.OverviewResult{}, errors.New("网络概览缓存版本无效")
	}
	// Latency samples are live UI state and should not survive a hidden window.
	document.Result.Probes = make([]networkdiag.LatencyProbe, 0)
	return document.Result, nil
}
