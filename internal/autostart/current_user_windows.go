package autostart

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"campusnet-toolbox/internal/winpaths"
	"golang.org/x/sys/windows"
)

type taskIdentity struct {
	Principal string `xml:"Principals>Principal>UserId"`
	Command   string `xml:"Actions>Exec>Command"`
}

func RemoveCurrentUserTask(executable string) error {
	if !filepath.IsAbs(executable) {
		return errors.New("无法确认当前程序路径")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("读取当前 Windows 账户失败: %w", err)
	}
	return removeTaskIfOwned(user.User.Sid.String(), executable, queryCurrentTaskXML, resolveTaskPrincipalSID, removeVerifiedCurrentTask)
}

func removeTaskIfOwned(currentSID, executable string, query func() ([]byte, bool, error), resolve func(string) (string, error), remove func() error) error {
	data, exists, err := query()
	if err != nil {
		return fmt.Errorf("查询登录启动任务失败: %w", err)
	}
	if !exists {
		return nil
	}
	task, err := parseTaskIdentity(data)
	if err != nil {
		return fmt.Errorf("读取登录启动任务内容失败: %w", err)
	}
	taskSID, err := resolve(task.Principal)
	if err != nil {
		return fmt.Errorf("识别登录启动任务账户失败: %w", err)
	}
	if !taskBelongsToCurrentUser(taskSID, currentSID, task.Command, executable) {
		return nil
	}
	if err := remove(); err != nil {
		return fmt.Errorf("移除当前账户的登录启动任务失败: %w", err)
	}
	return nil
}

func taskBelongsToCurrentUser(taskSID, currentSID, taskExecutable, executable string) bool {
	command := strings.Trim(strings.TrimSpace(taskExecutable), `"`)
	return strings.EqualFold(taskSID, currentSID) &&
		filepath.IsAbs(command) &&
		strings.EqualFold(filepath.Clean(command), filepath.Clean(executable))
}

func parseTaskIdentity(data []byte) (taskIdentity, error) {
	decoded, err := decodeTaskXML(data)
	if err != nil {
		return taskIdentity{}, err
	}
	if start := bytes.Index(decoded, []byte("<?xml")); start >= 0 {
		if end := bytes.Index(decoded[start:], []byte("?>")); end >= 0 {
			decoded = decoded[start+end+2:]
		}
	}
	var task taskIdentity
	if err := xml.Unmarshal(decoded, &task); err != nil {
		return taskIdentity{}, err
	}
	task.Principal = strings.TrimSpace(task.Principal)
	task.Command = strings.TrimSpace(task.Command)
	if task.Principal == "" || task.Command == "" {
		return taskIdentity{}, errors.New("计划任务缺少账户或程序路径")
	}
	return task, nil
}

func decodeTaskXML(data []byte) ([]byte, error) {
	if len(data) < 2 || !(bytes.Equal(data[:2], []byte{0xff, 0xfe}) || bytes.Equal(data[:2], []byte{0xfe, 0xff})) {
		return data, nil
	}
	if len(data)%2 != 0 {
		return nil, errors.New("计划任务 XML 编码长度无效")
	}
	littleEndian := data[0] == 0xff
	units := make([]uint16, 0, (len(data)-2)/2)
	for index := 2; index < len(data); index += 2 {
		if littleEndian {
			units = append(units, uint16(data[index])|uint16(data[index+1])<<8)
		} else {
			units = append(units, uint16(data[index])<<8|uint16(data[index+1]))
		}
	}
	return []byte(string(utf16.Decode(units))), nil
}

func resolveTaskPrincipalSID(value string) (string, error) {
	if strings.HasPrefix(strings.ToUpper(value), "S-1-") {
		sid, err := windows.StringToSid(value)
		if err != nil {
			return "", err
		}
		return sid.String(), nil
	}
	sid, _, _, err := windows.LookupSID("", value)
	if err != nil {
		return "", err
	}
	return sid.String(), nil
}

func queryCurrentTaskXML() ([]byte, bool, error) {
	exists, err := currentTaskExists()
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	tool, err := winpaths.SystemExecutable("schtasks.exe")
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, tool, "/Query", "/TN", taskName, "/XML")
	command.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, false, fmt.Errorf("schtasks 查询失败: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return output, true, nil
}

func currentTaskExists() (bool, error) {
	windowsRoot, err := windows.GetWindowsDirectory()
	if err != nil {
		return false, err
	}
	taskFile := filepath.Join(windowsRoot, "System32", "Tasks", taskName)
	if _, err := os.Stat(taskFile); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func removeVerifiedCurrentTask() error {
	return deleteVerifiedTask(func() error {
		tool, err := winpaths.SystemExecutable("schtasks.exe")
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, tool, "/Delete", "/TN", taskName, "/F")
		command.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("schtasks 删除失败: %s: %w", strings.TrimSpace(string(output)), err)
		}
		return nil
	}, currentTaskExists)
}

func deleteVerifiedTask(remove func() error, exists func() (bool, error)) error {
	err := remove()
	if err == nil {
		return nil
	}
	stillExists, checkErr := exists()
	if checkErr != nil {
		return errors.Join(err, checkErr)
	}
	if stillExists {
		return err
	}
	return nil
}
