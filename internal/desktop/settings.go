package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"campusnet-toolbox/internal/auth"
	"campusnet-toolbox/internal/autostart"
	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/systemnet"
	"campusnet-toolbox/internal/tray"
)

type configurationStore interface {
	LoadSystemPreferences() (settings.SystemPreferences, error)
	Password() (string, error)
	SaveConfiguration(settings.Profile, settings.DiagnosticSettings, settings.SystemPreferences) (settings.Profile, settings.DiagnosticSettings, settings.SystemPreferences, error)
}

type settingsEffects struct {
	applyPriority         func(string) error
	configureAutostart    func(bool) error
	startBackground       func() error
	refreshBackground     func() error
	cacheLatencyTargets   func([]settings.LatencyTarget)
	listNetworkInterfaces func() ([]systemnet.NetworkInterface, error)
}

func (a *App) SaveSettings(request SettingsRequest) (SettingsResult, error) {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if a.clearing.Load() {
		return SettingsResult{}, errors.New("本机数据正在清理")
	}
	if a.settings == nil {
		return SettingsResult{}, errors.New("无法定位当前用户的配置目录")
	}
	if err := migrateLoginStartup(a.settings, autostart.Configure); err != nil {
		return SettingsResult{}, fmt.Errorf("无法迁移登录启动设置: %w", err)
	}
	return saveSettings(request, a.settings, settingsEffects{
		applyPriority:         systemnet.ApplyPriority,
		configureAutostart:    autostart.Configure,
		startBackground:       autostart.LaunchBackground,
		refreshBackground:     func() error { return tray.SendCommand(tray.CommandReload) },
		cacheLatencyTargets:   a.cacheLatencyTargets,
		listNetworkInterfaces: listNetworkInterfaces,
	})
}

func saveSettings(request SettingsRequest, store configurationStore, effects settingsEffects) (SettingsResult, error) {
	if store == nil {
		return SettingsResult{}, errors.New("无法定位当前用户的配置目录")
	}
	if err := settings.ValidateProfile(request.Profile); err != nil {
		return SettingsResult{}, err
	}
	diagnostics, err := settings.NormalizeDiagnostics(request.Diagnostics)
	if err != nil {
		return SettingsResult{}, err
	}
	request.Diagnostics = diagnostics
	if request.Profile.StartDelayMs < 0 || request.Profile.StartDelayMs > int(auth.MaxStartDelay/time.Millisecond) ||
		request.Profile.RetryDelayMs < 0 || request.Profile.RetryDelayMs > int(auth.MaxRetryDelay/time.Millisecond) {
		return SettingsResult{}, errors.New("认证延迟或重试参数超出允许范围")
	}
	if _, err := auth.BuildIdentity(request.Profile.Identity, request.Profile.IdentitySuffix); err != nil {
		return SettingsResult{}, fmt.Errorf("identity 扩展无效: %w", err)
	}
	if request.System.PriorityMode != systemnet.PriorityAutomatic && request.System.PriorityMode != systemnet.PriorityEthernet && request.System.PriorityMode != systemnet.PriorityWiFi {
		return SettingsResult{}, errors.New("未知的网卡优先级模式")
	}
	previousSystem, err := store.LoadSystemPreferences()
	if err != nil {
		return SettingsResult{}, err
	}
	if request.System.AutoAuthenticate {
		password, passwordErr := store.Password()
		if passwordErr != nil {
			return SettingsResult{}, passwordErr
		}
		if strings.TrimSpace(request.Profile.Username) == "" || password == "" {
			return SettingsResult{}, errors.New("启用自动认证前，请先在锐捷认证页保存账号和密码")
		}
	}
	// Register compensation before each command: an OS command may change some
	// interfaces or task state before failing. The settings file is committed last.
	var rollback []func() error
	fail := func(cause error) (SettingsResult, error) {
		var rollbackErrors []error
		for index := len(rollback) - 1; index >= 0; index-- {
			if err := rollback[index](); err != nil {
				rollbackErrors = append(rollbackErrors, err)
			}
		}
		if len(rollbackErrors) > 0 {
			return SettingsResult{}, fmt.Errorf("配置未保存，部分系统设置恢复失败；请以管理员身份重新选择网卡优先级和登录启动选项并保存，或检查 Windows 网卡及任务计划设置: %w", errors.Join(append([]error{cause}, rollbackErrors...)...))
		}
		return SettingsResult{}, fmt.Errorf("配置未保存，已重新应用此前的系统设置: %w", cause)
	}
	if request.System.PriorityMode != previousSystem.PriorityMode {
		rollback = append(rollback, func() error {
			if err := effects.applyPriority(previousSystem.PriorityMode); err != nil {
				return fmt.Errorf("恢复网卡优先级失败: %w", err)
			}
			return nil
		})
		if err := effects.applyPriority(request.System.PriorityMode); err != nil {
			return fail(fmt.Errorf("应用网卡优先级失败: %w", err))
		}
	}
	if request.System.StartAtLogin != previousSystem.StartAtLogin {
		rollback = append(rollback, func() error {
			if err := effects.configureAutostart(previousSystem.StartAtLogin); err != nil {
				return fmt.Errorf("恢复登录启动任务失败: %w", err)
			}
			return nil
		})
		if err := effects.configureAutostart(request.System.StartAtLogin); err != nil {
			return fail(fmt.Errorf("配置登录启动任务失败: %w", err))
		}
	}
	profile, diagnostics, system, err := store.SaveConfiguration(request.Profile, request.Diagnostics, request.System)
	if err != nil {
		return fail(fmt.Errorf("写入配置失败: %w", err))
	}
	effects.cacheLatencyTargets(diagnostics.LatencyTargets)
	result := SettingsResult{Profile: profile, Diagnostics: diagnostics, System: system}
	if request.System.AutoAuthenticate && request.System.AutoAuthenticate != previousSystem.AutoAuthenticate {
		if err := effects.startBackground(); err != nil {
			result.Warning = fmt.Sprintf("设置已保存，但自动认证后台未启动；请重新启动程序重试: %v", err)
		}
	}
	if err := effects.refreshBackground(); err != nil {
		result.Warning = fmt.Sprintf("设置已保存，但后台尚未确认更新；请退出后台后重新打开程序: %v", err)
	}
	result.NetworkInterfaces, _ = effects.listNetworkInterfaces()
	return result, nil
}
