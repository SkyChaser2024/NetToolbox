package desktop

import (
	"errors"
	"fmt"
	"os"

	"campusnet-toolbox/internal/autostart"
	"campusnet-toolbox/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type cleanupEffects struct {
	cancelDiagnostics func()
	stopAuth          func() error
	stopBackground    func() error
	removeTask        func(string) error
	launchWorker      func(int) error
	quit              func()
}

func prepareCleanup(e cleanupEffects, executable string, parentPID int) error {
	e.cancelDiagnostics()
	if err := e.stopAuth(); err != nil {
		return fmt.Errorf("停止当前认证失败: %w", err)
	}
	if err := e.stopBackground(); err != nil {
		return fmt.Errorf("停止自动认证后台失败: %w", err)
	}
	if err := e.removeTask(executable); err != nil {
		return fmt.Errorf("取消登录自启动失败: %w", err)
	}
	if err := e.launchWorker(parentPID); err != nil {
		return fmt.Errorf("自动认证已停止，自启动已取消，但清理进程未能启动: %w", err)
	}
	e.quit()
	return nil
}

func (a *App) ClearLocalData() error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if a.clearing.Load() {
		return errors.New("本机数据正在清理")
	}
	if a.ctx == nil || a.auth == nil {
		return errors.New("程序尚未准备好清理本机数据")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位当前程序: %w", err)
	}
	a.clearing.Store(true)
	effects := cleanupEffects{
		cancelDiagnostics: func() {
			a.CancelLatencyChecks()
			a.CancelTraceroute("")
			a.CancelPing("")
		},
		stopAuth:       func() error { return a.auth.Stop(false) },
		stopBackground: tray.StopExisting,
		removeTask:     autostart.RemoveCurrentUserTask,
		launchWorker:   func(pid int) error { return launchLocalDataWorker(executable, pid) },
		quit:           func() { runtime.Quit(a.ctx) },
	}
	if err := prepareCleanup(effects, executable, os.Getpid()); err != nil {
		a.clearing.Store(false)
		return err
	}
	return nil
}
