package desktop

import (
	"context"
	"net/url"
	"strings"
	"time"

	"campusnet-toolbox/internal/adapters"
	"campusnet-toolbox/internal/appmeta"
	"campusnet-toolbox/internal/auth"
	"campusnet-toolbox/internal/autostart"
	"campusnet-toolbox/internal/networkdiag"
	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.settings != nil {
		if err := migrateLoginStartup(a.settings, autostart.Configure); err != nil {
			a.backgroundWarning = "登录启动设置迁移失败，下次启动将重试：" + err.Error()
			return
		}
		prefs, err := a.settings.LoadSystemPreferences()
		if err == nil && prefs.AutoAuthenticate {
			if err := autostart.LaunchBackground(); err != nil {
				a.backgroundWarning = "自动认证后台未启动，可在设置中重试：" + err.Error()
			}
		}
	}
}

func (a *App) shutdown(_ context.Context) {
	a.CancelLatencyChecks()
	a.CancelTraceroute("")
	a.CancelPing("")
	_ = a.auth.Stop(false)
}

func (a *App) beforeClose(_ context.Context) bool {
	if a.settings == nil {
		return false
	}
	system, err := a.settings.LoadSystemPreferences()
	if err == nil {
		err = applyClosePreference(system, autostart.LaunchBackground, tray.StopExisting)
	}
	if err != nil && a.ctx != nil {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type: runtime.ErrorDialog, Title: "无法完成关闭操作",
			Message: err.Error() + "。窗口已保留，请稍后重试。",
		})
		return true
	}
	return false
}

func (a *App) Bootstrap() BootstrapData {
	defer releaseUnusedForegroundMemory()
	data := BootstrapData{
		Profile: settings.DefaultProfile(), Diagnostics: settings.DefaultDiagnostics(), System: settings.DefaultSystemPreferences(),
		AuthState: a.auth.State(), Version: appmeta.Version,
		BackgroundWarning: a.backgroundWarning,
	}
	if a.settings == nil {
		data.ConfigurationErr = "无法定位当前用户的配置目录"
		if a.configurationErr != nil {
			data.ConfigurationErr += ": " + a.configurationErr.Error()
		}
	} else {
		profile, diagnostics, system, err := a.settings.LoadConfiguration()
		if err != nil {
			data.ConfigurationErr = err.Error()
		} else {
			data.Profile = profile
			data.Diagnostics = diagnostics
			data.System = system
		}
	}
	a.cacheLatencyTargets(data.Diagnostics.LatencyTargets)
	networkInterfaces, err := listNetworkInterfaces()
	data.NetworkInterfaces = networkInterfaces
	var adapterResult adapters.Result
	if err == nil {
		adapterResult = adapters.ListWithInterfaces(data.NetworkInterfaces)
	} else {
		adapterResult = adapters.List()
	}
	data.Adapters = adapterResult.Adapters
	data.NpcapAvailable = adapterResult.NpcapAvailable
	data.AdapterError = adapterResult.Error
	if a.restoreOverview {
		if cached, err := loadOverviewCache(); err == nil {
			data.CachedOverview = &cached
		}
	}
	if data.AuthState == auth.StateIdle {
		selected := selectEthernetAdapter(data.Adapters, data.Profile.DeviceName)
		if selected != nil && selected.Up {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			online := networkdiag.InterfaceOnline(ctx, adapterAddresses(selected))
			cancel()
			if online {
				a.auth.MarkAuthenticated(auth.Config{DeviceName: selected.DeviceName, LocalMAC: selected.MAC})
				data.AuthState = auth.StateAuthenticated
			}
		}
	}
	return data
}

func (a *App) RefreshAdapters() adapters.Result {
	return adapters.List()
}

func (a *App) OpenLink(url string) {
	if a.ctx != nil && isAllowedLink(url) {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

func isAllowedLink(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "npcap.com", "natchecker.com":
		return true
	case "github.com":
		const repositoryPath = "/yinin6/ruijie-sysu-go"
		return parsed.Path == repositoryPath || strings.HasPrefix(parsed.Path, repositoryPath+"/")
	default:
		return false
	}
}
