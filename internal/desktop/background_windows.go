package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"campusnet-toolbox/internal/adapters"
	"campusnet-toolbox/internal/auth"
	"campusnet-toolbox/internal/automonitor"
	"campusnet-toolbox/internal/autostart"
	"campusnet-toolbox/internal/networkdiag"
	"campusnet-toolbox/internal/settings"
	"campusnet-toolbox/internal/systemnet"
	"campusnet-toolbox/internal/tray"
	"golang.org/x/sys/windows"
)

func runBackground(icon []byte) error {
	configureBackgroundRuntime()
	release, alreadyRunning, err := acquireBackgroundLock()
	if err != nil {
		return err
	}
	if alreadyRunning {
		return nil
	}
	defer release()
	store, err := settings.NewStore()
	if err != nil {
		return err
	}
	if err := migrateLoginStartup(store, autostart.Configure); err != nil {
		return err
	}
	system, err := store.LoadSystemPreferences()
	if err != nil {
		return err
	}
	if !needsBackground(system) {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	go func() {
		if waitBackground(ctx, 2*time.Second) {
			trimBackgroundWorkingSet()
		}
	}()
	controller := automonitor.New(func(ctx context.Context, status func(automonitor.State)) {
		monitorAuthentication(ctx, store, status)
	}, tray.SetMonitorState)
	defer controller.Stop()
	loadConfig := func() (automonitor.Config, error) {
		profile, _, prefs, err := store.LoadConfiguration()
		key, _ := json.Marshal(profile)
		return automonitor.Config{Enabled: prefs.AutoAuthenticate, Key: string(key)}, err
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	controller.Reload(config, false)
	return tray.Run(icon, launchMainWindow, func() {
		cancel()
		controller.Stop()
	}, func(command tray.Command) bool {
		if command == tray.CommandPause {
			controller.Pause()
			return true
		}
		config, err := loadConfig()
		if err != nil {
			return false
		}
		switch command {
		case tray.CommandReload, tray.CommandCredentialsChanged:
			controller.Reload(config, command == tray.CommandCredentialsChanged)
		case tray.CommandResume:
			controller.Resume(config)
		default:
			return false
		}
		return true
	}, func() uintptr { return uintptr(controller.State()) })
}

func monitorAuthentication(ctx context.Context, store *settings.Store, setStatus func(automonitor.State)) {
	const monitorInterval = 30 * time.Second
	var attemptedForOutage bool
	var linkWasUp bool
	for {
		if ctx.Err() != nil {
			return
		}
		profile, _, system, err := store.LoadConfiguration()
		if err != nil {
			setStatus(automonitor.ConfigurationError)
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		if !system.AutoAuthenticate {
			setStatus(automonitor.Disabled)
			return
		}

		if profile.Username == "" || !profile.PasswordSet {
			setStatus(automonitor.WaitingCredentials)
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		interfaces, interfaceErr := systemnet.List()
		selectedInterface := selectSystemEthernet(interfaces, profile.DeviceName, profile.LocalMAC)
		if interfaceErr != nil {
			setStatus(automonitor.AdapterError)
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		if selectedInterface == nil || !selectedInterface.Up {
			setStatus(automonitor.WaitingLink)
			attemptedForOutage = false
			linkWasUp = false
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}

		probeContext, cancelProbe := context.WithTimeout(ctx, 4*time.Second)
		online := networkdiag.InterfaceOnline(probeContext, interfaceAddresses(selectedInterface))
		cancelProbe()
		if online {
			setStatus(automonitor.Online)
			attemptedForOutage = false
			linkWasUp = true
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		if !linkWasUp {
			attemptedForOutage = false
		}
		linkWasUp = true
		if attemptedForOutage {
			setStatus(automonitor.Exhausted)
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		setStatus(automonitor.Confirming)
		if !waitBackground(ctx, 3*time.Second) {
			return
		}
		confirmContext, cancelConfirm := context.WithTimeout(ctx, 4*time.Second)
		online = networkdiag.InterfaceOnline(confirmContext, interfaceAddresses(selectedInterface))
		cancelConfirm()
		if online {
			setStatus(automonitor.Online)
			continue
		}

		password, err := store.Password()
		if err != nil || password == "" {
			setStatus(automonitor.PasswordError)
			attemptedForOutage = true
			continue
		}
		attemptedForOutage = true
		setStatus(automonitor.Authenticating)
		adapterResult := adapters.List()
		selected := selectEthernetAdapter(adapterResult.Adapters, profile.DeviceName)
		if selected == nil || !selected.Up {
			setStatus(automonitor.AdapterError)
			continue
		}
		cfg := auth.Config{
			DeviceName: selected.DeviceName, LocalMAC: selected.MAC,
			Username: profile.Username, Password: password,
			Identity: profile.Identity, IdentitySuffix: profile.IdentitySuffix,
			StartDelay:  time.Duration(profile.StartDelayMs) * time.Millisecond,
			RetryDelay:  time.Duration(profile.RetryDelayMs) * time.Millisecond,
			MaxAttempts: auth.DefaultMaxAttempts, Debug: profile.Debug,
		}
		err = auth.Run(ctx, cfg, nil)
		cfg.Password = ""
		password = ""
		trimBackgroundWorkingSet()
		if errors.Is(err, auth.ErrOperationBusy) {
			attemptedForOutage = false
			setStatus(automonitor.ForegroundBusy)
		} else if err != nil {
			setStatus(automonitor.Exhausted)
		} else {
			setStatus(automonitor.WaitingNetwork)
		}
		if !waitBackground(ctx, monitorInterval) {
			return
		}
	}
}

func waitBackground(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	if duration >= 30*time.Second {
		trimBackgroundWorkingSet()
	}
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func launchMainWindow() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	command := exec.Command(executable, "--restore-overview")
	if command.Start() == nil {
		_ = command.Process.Release()
	}
}

func acquireBackgroundLock() (release func(), alreadyRunning bool, err error) {
	path := filepath.Join(os.TempDir(), "network-toolbox-background.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	overlapped := &windows.Overlapped{}
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, overlapped,
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		_ = file.Close()
		return func() {}, true, nil
	}
	if err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf("创建后台进程锁失败: %w", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
			_ = file.Close()
		})
	}, false, nil
}
