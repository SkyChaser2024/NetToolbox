package appdata

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func RemoveKnownLegacyCache(paths Paths) error {
	entries, err := os.ReadDir(paths.LegacyCacheDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "overview-cache.json" && !(strings.HasPrefix(name, "tray-") && strings.HasSuffix(name, ".ico")) {
			continue
		}
		if entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(paths.LegacyCacheDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	remaining, err := os.ReadDir(paths.LegacyCacheDir)
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		if err := os.Remove(paths.LegacyCacheDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
