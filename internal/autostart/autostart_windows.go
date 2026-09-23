package autostart

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"campusnet-toolbox/internal/tray"
	"campusnet-toolbox/internal/winpaths"
	"golang.org/x/sys/windows"
)

const taskName = "Network Toolbox Auto Authentication"

func Configure(enabled bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	arguments := []string{"/Delete", "/TN", taskName, "/F"}
	tool, err := winpaths.SystemExecutable("schtasks.exe")
	if err != nil {
		return fmt.Errorf("无法定位 Windows 任务计划程序: %w", err)
	}
	if enabled {
		executable, err := executablePath()
		if err != nil {
			return err
		}
		taskCommand := fmt.Sprintf("\"%s\" --login", executable)
		arguments = []string{"/Create", "/TN", taskName, "/TR", taskCommand, "/SC", "ONLOGON", "/DELAY", "0000:10", "/RL", "HIGHEST", "/IT", "/F"}
	} else {
		query := exec.CommandContext(ctx, tool, "/Query", "/TN", taskName)
		query.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
		if err := query.Run(); err != nil {
			return nil
		}
	}
	command := exec.CommandContext(ctx, tool, arguments...)
	command.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("更新登录启动任务失败: %s", message)
	}
	return nil
}

func LaunchBackground() error {
	if tray.IsRunning() {
		return nil
	}
	executable, err := executablePath()
	if err != nil {
		return err
	}
	command := exec.Command(executable, "--background")
	command.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("启动托盘后台进程失败: %w", err)
	}
	_ = command.Process.Release()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tray.IsRunning() {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("后台进程未能就绪，请检查配置后重试")
}

func executablePath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法定位应用程序: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("无法解析应用程序路径: %w", err)
	}
	if strings.Contains(strings.ToLower(executable), "go-build") {
		return "", fmt.Errorf("开发模式下不能注册后台任务，请使用已构建的网络工具箱.exe")
	}
	return executable, nil
}
