package desktop

import (
	"errors"

	"campusnet-toolbox/internal/automonitor"
	"campusnet-toolbox/internal/autostart"
	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/tray"
)

type AutomaticAuthenticationStatus struct {
	Running bool   `json:"running"`
	Paused  bool   `json:"paused"`
	Message string `json:"message"`
}

func (a *App) GetAutomaticAuthenticationStatus() (AutomaticAuthenticationStatus, error) {
	state, err := tray.MonitorState()
	if err != nil {
		return AutomaticAuthenticationStatus{}, err
	}
	if state == automonitor.Stopped && a.settings != nil {
		prefs, err := a.settings.LoadSystemPreferences()
		if err != nil {
			return AutomaticAuthenticationStatus{}, err
		}
		if !prefs.AutoAuthenticate {
			state = automonitor.Disabled
		}
	}
	return AutomaticAuthenticationStatus{
		Running: state != automonitor.Stopped && state != automonitor.Disabled,
		Paused:  state == automonitor.Paused, Message: state.String(),
	}, nil
}

func (a *App) ResumeAutomaticAuthentication() error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if a.settings == nil {
		return errors.New("无法读取自动认证设置")
	}
	prefs, err := a.settings.LoadSystemPreferences()
	if err != nil {
		return err
	}
	if !prefs.AutoAuthenticate {
		return errors.New("请先开启自动认证与断线重连")
	}
	if err := autostart.LaunchBackground(); err != nil {
		return err
	}
	return tray.SendCommand(tray.CommandResume)
}

func applyClosePreference(prefs settings.SystemPreferences, keep, stop func() error) error {
	if prefs.CloseToTray {
		return keep()
	}
	return stop()
}

func pauseThenLogout(pause, logout func() error) error {
	if err := pause(); err != nil {
		return err
	}
	return logout()
}

// A credential save is already committed when these operations run. Even if
// foreground authentication cannot acquire the lock, the resident worker must
// cancel its old attempts and reload the saved credentials.
func startWithBackgroundRefresh(start, refresh func() error) (error, error) {
	startErr := start()
	return startErr, refresh()
}
