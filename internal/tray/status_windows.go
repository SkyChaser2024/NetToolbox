package tray

import (
	"unsafe"

	"campusnet-toolbox/internal/automonitor"
)

const (
	wmMonitorChanged  = 0x8003
	wmAuthChanged     = 0x8004
	menuAuthStatus    = 1003
	menuMonitorStatus = 1004
	mfDisabled        = 0x0002
)

type authStatus uintptr

const (
	authUnknown authStatus = iota
	authIdle
	authWorking
	authOnline
	authFailed
	authError
	authStopping
)

func (s authStatus) label() string {
	switch s {
	case authIdle:
		return "未认证"
	case authWorking:
		return "正在认证"
	case authOnline:
		return "已认证"
	case authFailed:
		return "认证失败"
	case authError:
		return "认证异常"
	case authStopping:
		return "正在停止认证"
	default:
		return "尚未检测"
	}
}

func (s authStatus) badge() int {
	switch s {
	case authWorking, authStopping:
		return 1
	case authOnline:
		return 2
	case authFailed, authError:
		return 3
	default:
		return 0
	}
}

// SetAuthState sends only a small state code across processes, never text or pointers.
func SetAuthState(value string) {
	values := map[string]authStatus{
		"idle": authIdle, "starting": authWorking, "waiting_identity": authWorking,
		"waiting_challenge": authWorking, "authenticated": authOnline,
		"failed": authFailed, "error": authError, "stopping": authStopping,
	}
	status, ok := values[value]
	if !ok {
		return
	}
	if window := existingWindow(); window != 0 {
		procPostMessage.Call(uintptr(window), wmAuthChanged, uintptr(status), 0)
	}
}

// Marshal all native UI changes onto the tray's message-loop thread.
func SetMonitorState(value automonitor.State) {
	stateMu.Lock()
	defer stateMu.Unlock()
	if state != nil && state.window != 0 {
		procPostMessage.Call(uintptr(state.window), wmMonitorChanged, uintptr(value), 0)
	}
}

func (t *trayState) updateMonitor(value automonitor.State) {
	t.setMenuText(menuMonitorStatus, value.String())
	switch value {
	case automonitor.Online, automonitor.WaitingNetwork:
		t.updateAuth(authOnline)
	case automonitor.Authenticating, automonitor.ForegroundBusy:
		t.updateAuth(authWorking)
	case automonitor.Exhausted:
		t.updateAuth(authFailed)
	case automonitor.AdapterError, automonitor.PasswordError, automonitor.ConfigurationError:
		t.updateAuth(authError)
	case automonitor.WaitingLink, automonitor.Paused:
		t.updateAuth(authIdle)
	}
}

func (t *trayState) setMenuText(id uintptr, value string) {
	text := mustUTF16Ptr(value)
	procModifyMenu.Call(uintptr(t.menu), id, mfString|mfDisabled, id, uintptr(unsafe.Pointer(text)))
}

func (t *trayState) updateAuth(value authStatus) {
	if value > authStopping {
		return
	}
	t.setMenuText(menuAuthStatus, "认证状态："+value.label())
	icon := t.statusIcons[value.badge()]
	if icon == 0 {
		icon = t.icon
	}
	if t.nid.Icon == icon {
		return
	}
	t.nid.Icon = icon
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&t.nid)))
}
