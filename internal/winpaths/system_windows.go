package winpaths

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// SystemExecutable resolves a fixed executable below the Windows system
// directory without trusting environment variables or PATH search order.
func SystemExecutable(parts ...string) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("系统程序路径为空")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || filepath.Base(part) != part || strings.ContainsAny(part, `/\\`) {
			return "", errors.New("系统程序路径包含无效片段")
		}
	}
	directory, err := windows.GetSystemDirectory()
	if err != nil {
		return "", err
	}
	path := filepath.Join(append([]string{directory}, parts...)...)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("系统程序路径指向目录")
	}
	return path, nil
}
