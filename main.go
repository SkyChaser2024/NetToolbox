package main

import (
	"embed"
	"fmt"
	"os"

	"campusnet-toolbox/internal/tray"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	winoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	if isBackgroundMode(os.Args[1:]) {
		if err := runBackground(trayIcon); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "后台服务启动失败:", err)
			os.Exit(1)
		}
		return
	}
	configureForegroundRuntime()
	releaseInstance, alreadyRunning, instanceErr := acquireMainInstance()
	if instanceErr != nil {
		_, _ = fmt.Fprintln(os.Stderr, "单实例检查失败:", instanceErr)
	} else if alreadyRunning {
		return
	} else {
		defer releaseInstance()
	}
	app := NewApp(hasArgument(os.Args[1:], "--restore-overview") || tray.IsRunning())

	// Create application with options
	err := wails.Run(&options.App{
		Title:            "网络工具箱",
		Width:            1180,
		Height:           780,
		MinWidth:         980,
		MinHeight:        680,
		WindowStartState: options.Normal,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 241, G: 245, B: 249, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "network-toolbox-8021x-6f19c10d",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				if app.ctx != nil {
					runtime.WindowShow(app.ctx)
					runtime.WindowUnminimise(app.ctx)
				}
			},
		},
		DragAndDrop: &options.DragAndDrop{DisableWebViewDrop: true},
		Windows: &winoptions.Options{
			Theme:                winoptions.SystemDefault,
			WindowIsTranslucent:  false,
			WebviewIsTransparent: false,
			BackdropType:         winoptions.None,
			ResizeDebounceMS:     8,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "启动应用失败:", err)
		os.Exit(1)
	}
}

func isBackgroundMode(arguments []string) bool {
	return hasArgument(arguments, "--background") || hasArgument(arguments, "--auto-auth")
}

func hasArgument(arguments []string, expected string) bool {
	for _, argument := range arguments {
		if argument == expected {
			return true
		}
	}
	return false
}
