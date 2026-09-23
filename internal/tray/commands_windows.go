package tray

import (
	"errors"
	"unsafe"

	"campusnet-toolbox/internal/automonitor"
)

type Command uintptr

const (
	CommandReload Command = iota + 1
	CommandPause
	CommandResume
	CommandCredentialsChanged
)

const (
	wmBackgroundCommand = 0x8001
	wmBackgroundStatus  = 0x8002
)

var procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")

// SendCommand uses an acknowledged message rather than terminating a scheduled
// task. No credential or pointer crosses the process boundary.
func SendCommand(command Command) error {
	window := existingWindow()
	if window == 0 {
		if command == CommandResume {
			return errors.New("自动认证后台未运行")
		}
		return nil
	}
	var result uintptr
	ok, _, _ := procSendMessageTimeout.Call(uintptr(window), wmBackgroundCommand, uintptr(command), 0,
		0x0002, 5000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 || result != 1 {
		return errors.New("后台未确认操作，请稍后重试或退出后台后重新打开程序")
	}
	return nil
}

func MonitorState() (automonitor.State, error) {
	window := existingWindow()
	if window == 0 {
		return automonitor.Stopped, nil
	}
	var result uintptr
	ok, _, _ := procSendMessageTimeout.Call(uintptr(window), wmBackgroundStatus, 0, 0,
		0x0002, 1000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 {
		return automonitor.Stopped, errors.New("暂时无法读取后台状态")
	}
	return automonitor.State(result), nil
}

func StopExisting() error {
	window := existingWindow()
	if window == 0 {
		return nil
	}
	var result uintptr
	ok, _, _ := procSendMessageTimeout.Call(uintptr(window), wmClose, 0, 0,
		0x0002, 5000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 {
		return errors.New("后台未能及时退出，请稍后重试")
	}
	return nil
}
