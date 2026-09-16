package main

import (
	"context"
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
	system, err := store.LoadSystemPreferences()
	if err != nil {
		return err
	}
	if !system.AutoAuthenticate && !system.CloseToTray {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime.LockOSThread()
	go func() {
		if waitBackground(ctx, 2*time.Second) {
			trimBackgroundWorkingSet()
		}
	}()
	go monitorAuthentication(ctx, store, func(string) {}, func() {
		cancel()
		tray.Quit()
	})
	return tray.Run(icon, launchMainWindow, cancel)
}

func monitorAuthentication(ctx context.Context, store *settings.Store, setStatus func(string), stop func()) {
	const monitorInterval = 30 * time.Second
	var attemptedForOutage bool
	var linkWasUp bool
	for {
		profile, _, system, err := store.LoadConfiguration()
		if err != nil {
			setStatus("配置读取失败")
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		if !system.AutoAuthenticate && !system.CloseToTray {
			stop()
			return
		}
		if !system.AutoAuthenticate {
			setStatus("托盘驻留")
			attemptedForOutage = false
			linkWasUp = false
			if !waitBackground(ctx, 15*time.Second) {
				return
			}
			continue
		}

		if profile.Username == "" || !profile.PasswordSet {
			setStatus("等待有效账号配置")
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		interfaces, interfaceErr := systemnet.List()
		selectedInterface := selectSystemEthernet(interfaces, profile.DeviceName, profile.LocalMAC)
		if interfaceErr != nil {
			setStatus("读取有线网卡失败")
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		if selectedInterface == nil || !selectedInterface.Up {
			setStatus("等待有线网卡连接")
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
			setStatus("有线网络已认证")
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
			setStatus("本次断线已停止重试")
			if !waitBackground(ctx, monitorInterval) {
				return
			}
			continue
		}
		setStatus("确认断线状态")
		if !waitBackground(ctx, 3*time.Second) {
			return
		}
		confirmContext, cancelConfirm := context.WithTimeout(ctx, 4*time.Second)
		online = networkdiag.InterfaceOnline(confirmContext, interfaceAddresses(selectedInterface))
		cancelConfirm()
		if online {
			setStatus("有线网络已认证")
			continue
		}

		password, err := store.Password()
		if err != nil || password == "" {
			setStatus("无法读取保存的密码")
			attemptedForOutage = true
			continue
		}
		attemptedForOutage = true
		setStatus("断线，正在自动认证（最多 3 次）")
		adapterResult := adapters.List()
		selected := selectEthernetAdapter(adapterResult.Adapters, profile.DeviceName)
		if selected == nil || !selected.Up {
			setStatus("未找到可认证的有线网卡")
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
			setStatus("主界面正在认证")
		} else if err != nil {
			setStatus("自动认证失败，已停止重试")
		} else {
			setStatus("认证成功，等待网络就绪")
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
