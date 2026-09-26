package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"campusnet-toolbox/internal/appdata"
	"campusnet-toolbox/internal/privatefile"
)

func migrateDirectory(paths appdata.Paths, write func(string, []byte) error) error {
	newPath := filepath.Join(paths.ConfigDir, "config.json")
	if _, err := os.Stat(newPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("检查新配置失败: %w", err)
	}
	oldPath := filepath.Join(paths.LegacyConfigDir, "config.json")
	if _, err := os.Stat(oldPath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("检查旧配置失败: %w", err)
	}
	oldStore := &Store{path: oldPath}
	doc, err := oldStore.loadDocument()
	if err != nil {
		return fmt.Errorf("旧配置无法迁移: %w", err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化旧配置失败: %w", err)
	}
	if err := write(newPath, data); err != nil {
		return fmt.Errorf("写入新配置失败: %w", err)
	}
	if _, err := (&Store{path: newPath}).loadDocument(); err != nil {
		return fmt.Errorf("验证新配置失败: %w", err)
	}
	if err := os.Remove(oldPath); err != nil {
		return fmt.Errorf("移除旧配置失败: %w", err)
	}
	_ = os.Remove(paths.LegacyConfigDir)
	return nil
}

func migrateCurrentDirectory(paths appdata.Paths) error {
	return migrateDirectory(paths, privatefile.Write)
}
