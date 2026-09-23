package desktop

import (
	"context"
	"errors"
	"strings"
	"time"

	"campusnet-toolbox/internal/adapters"
	"campusnet-toolbox/internal/auth"
	"campusnet-toolbox/internal/autostart"
	"campusnet-toolbox/internal/networkdiag"
	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/systemnet"
	"campusnet-toolbox/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) Connect(request AuthRequest) error {
	return a.startAuthentication(request, true)
}

func (a *App) startAuthentication(request AuthRequest, saveProfile bool) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if saveProfile && a.settings != nil {
		if err := migrateLoginStartup(a.settings, autostart.Configure); err != nil {
			return err
		}
	}
	if a.auth.State() == auth.StateAuthenticated {
		return nil
	}
	if a.auth.Running() {
		return errors.New("认证任务已在运行")
	}
	profile := settings.Profile{
		DeviceName: request.DeviceName, AdapterLabel: request.AdapterLabel,
		LocalMAC: request.LocalMAC, Username: strings.TrimSpace(request.Username),
		Identity: request.Identity, IdentitySuffix: request.IdentitySuffix,
		StartDelayMs: request.StartDelayMs, RetryDelayMs: request.RetryDelayMs,
		Debug: request.Debug, RememberPassword: request.RememberPassword,
	}
	adapterResult := adapters.List()
	for index := range adapterResult.Adapters {
		item := &adapterResult.Adapters[index]
		if item.DeviceName != request.DeviceName || item.Kind != systemnet.PriorityEthernet || !item.Up {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		online := networkdiag.InterfaceOnline(ctx, adapterAddresses(item))
		cancel()
		if online {
			if saveProfile && a.settings != nil {
				if err := a.settings.Save(profile, request.Password); err != nil {
					return err
				}
				if err := tray.SendCommand(tray.CommandCredentialsChanged); err != nil {
					return err
				}
			}
			a.auth.MarkAuthenticated(auth.Config{DeviceName: item.DeviceName, LocalMAC: item.MAC})
			a.emitAuthEvent(auth.Event{State: auth.StateAuthenticated, Level: "success", Message: "检测到该有线网卡已经认证，无需重复认证", Timestamp: time.Now().Format("15:04:05")})
			return nil
		}
		break
	}
	password := request.Password
	if password == "" && request.RememberPassword && a.settings != nil {
		stored, err := a.settings.Password()
		if err != nil {
			return err
		}
		password = stored
	}
	identity := strings.TrimSpace(request.Identity)
	if identity == "" {
		identity = strings.TrimSpace(request.Username)
	}
	cfg := auth.Config{
		DeviceName: request.DeviceName, LocalMAC: request.LocalMAC,
		Username: strings.TrimSpace(request.Username), Password: password,
		Identity: identity, IdentitySuffix: request.IdentitySuffix,
		StartDelay:  time.Duration(request.StartDelayMs) * time.Millisecond,
		RetryDelay:  time.Duration(request.RetryDelayMs) * time.Millisecond,
		MaxAttempts: auth.DefaultMaxAttempts,
		Debug:       request.Debug,
	}
	if err := auth.Validate(cfg); err != nil {
		return err
	}
	if saveProfile && a.settings != nil {
		if err := a.settings.Save(profile, request.Password); err != nil {
			return err
		}
	}
	startErr, refreshErr := startWithBackgroundRefresh(func() error {
		return a.auth.Start(cfg, a.emitAuthEvent)
	}, func() error {
		if saveProfile {
			return tray.SendCommand(tray.CommandCredentialsChanged)
		}
		return nil
	})
	if startErr != nil {
		return errors.Join(startErr, refreshErr)
	}
	if refreshErr != nil {
		a.emitAuthEvent(auth.Event{State: a.auth.State(), Level: "warning", Message: "认证已启动，但后台配置更新失败：" + refreshErr.Error(), Timestamp: time.Now().Format("15:04:05")})
	}
	return nil
}

func (a *App) Logout() error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	loggedOut := false
	err := pauseThenLogout(func() error { return tray.SendCommand(tray.CommandPause) }, func() error {
		loggedOut = true
		return a.auth.Stop(true)
	})
	if !loggedOut {
		return err
	}
	if a.ctx != nil {
		level, message := "info", "已注销，自动认证已暂停；可在设置中恢复"
		if err != nil {
			level, message = "warning", "自动认证已暂停，但发送注销报文失败："+err.Error()
		}
		runtime.EventsEmit(a.ctx, "auth:event", auth.Event{
			State: auth.StateIdle, Level: level, Message: message, Timestamp: time.Now().Format("15:04:05"),
		})
	}
	return err
}

func (a *App) CancelAuthentication() error {
	err := a.auth.Stop(false)
	a.emitAuthEvent(auth.Event{State: auth.StateIdle, Level: "info", Message: "已取消本次认证", Timestamp: time.Now().Format("15:04:05")})
	return err
}

func (a *App) emitAuthEvent(event auth.Event) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "auth:event", event)
	}
}
