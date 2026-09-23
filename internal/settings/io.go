package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"campusnet-toolbox/internal/privatefile"
)

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
		if err := ValidateProfile(doc.Profile); err != nil {
			return document{}, fmt.Errorf("配置文件包含无效的认证字段: %w", err)
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
