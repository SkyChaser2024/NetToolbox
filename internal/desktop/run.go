package desktop

import (
	"fmt"
	"io/fs"
	"os"

	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/tray"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	winoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Run starts the foreground window or background worker for this process.
func Run(assets fs.FS, trayIcon []byte) error {
	if isBackgroundMode(os.Args[1:]) {
		if err := runBackground(trayIcon); err != nil {
			return fmt.Errorf("后台服务启动失败: %w", err)
		}
		return nil
	}
	if store, err := settings.NewStore(); err == nil {
		if prefs, err := store.LoadSystemPreferences(); err == nil {
			handled, silentErr := trySilentStartup(os.Args[1:], prefs, tray.IsRunning(), isMainInstanceRunning(), func() error {
				return runBackground(trayIcon)
			})
			if handled {
				return nil
			}
			if silentErr != nil {
				_, _ = fmt.Fprintln(os.Stderr, "静默启动失败，改为打开主窗口:", silentErr)
			}
		}
	}
	configureForegroundRuntime()
	releaseInstance, alreadyRunning, instanceErr := acquireMainInstance()
	if instanceErr != nil {
		_, _ = fmt.Fprintln(os.Stderr, "单实例检查失败:", instanceErr)
	} else if alreadyRunning {
		return nil
	} else {
		defer releaseInstance()
	}
	app := NewApp(hasArgument(os.Args[1:], "--restore-overview") || tray.IsRunning())

	return wails.Run(&options.App{
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
		Bind: []interface{}{app},
	})
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
