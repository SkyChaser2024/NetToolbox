package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"campusnet-toolbox/internal/appdata"
	"golang.org/x/sys/windows"
)

const localDataWorkerFlag = "--clear-local-data"

func parseLocalDataWorker(arguments []string) (int, bool, error) {
	if len(arguments) == 0 || arguments[0] != localDataWorkerFlag {
		return 0, false, nil
	}
	if len(arguments) != 2 {
		return 0, true, errors.New("清理进程参数数量无效")
	}
	pid, err := strconv.Atoi(arguments[1])
	if err != nil || pid <= 0 {
		return 0, true, errors.New("清理进程的父进程编号无效")
	}
	return pid, true, nil
}

func launchLocalDataWorker(executable string, parentPID int) error {
	if !filepath.IsAbs(executable) || parentPID <= 0 {
		return errors.New("无法启动本机数据清理进程")
	}
	command := exec.Command(executable, localDataWorkerFlag, strconv.Itoa(parentPID))
	command.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("启动本机数据清理进程失败: %w", err)
	}
	_ = command.Process.Release()
	return nil
}

func runLocalDataWorker(parentPID int) error {
	err := runLocalDataWorkerWith(parentPID, waitForParentExit, os.RemoveAll)
	if err == nil {
		return nil
	}
	message, _ := windows.UTF16PtrFromString("本机数据未能全部清除：\n\n" + err.Error())
	caption, _ := windows.UTF16PtrFromString("NetToolbox 清理失败")
	_, _ = windows.MessageBox(0, message, caption, windows.MB_OK|windows.MB_ICONERROR)
	return err
}

func runLocalDataWorkerWith(parentPID int, wait func(int, time.Duration) error, remove func(string) error) error {
	paths, err := appdata.Current()
	if err != nil {
		return fmt.Errorf("无法定位当前用户数据目录: %w", err)
	}
	if err := wait(parentPID, 30*time.Second); err != nil {
		return fmt.Errorf("等待主程序退出失败: %w", err)
	}
	failures := clearDataDirectoriesWithRetry(paths, remove, 120, 250*time.Millisecond, time.Sleep)
	return errors.Join(failures...)
}

func waitForParentExit(parentPID int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(parentPID))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	if err != nil {
		return err
	}
	if status == uint32(windows.WAIT_TIMEOUT) {
		return errors.New("主程序仍在运行")
	}
	if status != uint32(windows.WAIT_OBJECT_0) {
		return fmt.Errorf("等待主程序退出返回异常状态 %d", status)
	}
	return nil
}

func clearDataDirectoriesWithRetry(paths appdata.Paths, remove func(string) error, attempts int, interval time.Duration, sleep func(time.Duration)) []error {
	if attempts < 1 {
		return []error{errors.New("清理尝试次数无效")}
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		failures := clearDataDirectories(paths, remove)
		if len(failures) == 0 || attempt == attempts {
			return failures
		}
		sleep(interval)
	}
	return nil
}

func clearDataDirectories(paths appdata.Paths, remove func(string) error) []error {
	configRoot := filepath.Dir(paths.ConfigDir)
	cacheRoot := filepath.Dir(paths.CacheDir)
	expected, err := appdata.Resolve(configRoot, cacheRoot)
	if err != nil || expected != paths {
		return []error{errors.New("本机数据清理目标路径无效")}
	}
	targets := []string{
		paths.ConfigDir,
		paths.LegacyConfigDir,
		paths.CacheDir,
		paths.LegacyCacheDir,
		paths.LegacyWebViewDir,
	}
	var failures []error
	for _, path := range targets {
		if err := remove(path); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", path, err))
		}
	}
	return failures
}
