package appdata

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"
)

type Paths struct {
	ConfigDir        string
	LegacyConfigDir  string
	CacheDir         string
	LegacyCacheDir   string
	WebViewDir       string
	LegacyWebViewDir string
}

func Current() (Paths, error) {
	configRoot, err := windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return Paths{}, err
	}
	cacheRoot, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return Paths{}, err
	}
	return Resolve(configRoot, cacheRoot)
}

func Resolve(configRoot, cacheRoot string) (Paths, error) {
	if !validRoot(configRoot) || !validRoot(cacheRoot) {
		return Paths{}, errors.New("当前用户数据根目录无效")
	}
	configRoot = filepath.Clean(configRoot)
	cacheRoot = filepath.Clean(cacheRoot)
	return Paths{
		ConfigDir:        filepath.Join(configRoot, "NetToolbox"),
		LegacyConfigDir:  filepath.Join(configRoot, "CampusNetToolbox"),
		CacheDir:         filepath.Join(cacheRoot, "NetToolbox"),
		LegacyCacheDir:   filepath.Join(cacheRoot, "CampusNetToolbox"),
		WebViewDir:       filepath.Join(cacheRoot, "NetToolbox", "WebView2"),
		LegacyWebViewDir: filepath.Join(configRoot, "NetToolbox.exe"),
	}, nil
}

func validRoot(root string) bool {
	if !filepath.IsAbs(root) {
		return false
	}
	clean := filepath.Clean(root)
	return filepath.Dir(clean) != clean
}
