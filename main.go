package main

import (
	"embed"
	"fmt"
	"os"

	"campusnet-toolbox/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	if err := desktop.Run(assets, trayIcon); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "启动应用失败:", err)
		os.Exit(1)
	}
}
