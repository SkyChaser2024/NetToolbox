package tray

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"campusnet-toolbox/internal/privatefile"
	"golang.org/x/sys/windows"
)

const (
	wmClose        = 0x0010
	wmDestroy      = 0x0002
	wmContextMenu  = 0x007B
	wmLButtonUp    = 0x0202
	wmLButtonDbl   = 0x0203
	wmRButtonUp    = 0x0205
	ninSelect      = 0x0400
	ninKeySelect   = 0x0401
	wmTray         = 0x0401
	nimAdd         = 0x00000000
	nimModify      = 0x00000001
	nimDelete      = 0x00000002
	nimSetVersion  = 0x00000004
	notifyVersion4 = 4
	nifMessage     = 0x00000001
	nifIcon        = 0x00000002
	nifTip         = 0x00000004
	nifShowTip     = 0x00000080
	imageIcon      = 1
	lrLoadFromFile = 0x00000010
	lrDefaultSize  = 0x00000040
	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	menuOpen       = 1001
	menuQuit       = 1002
)

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	shell32                   = windows.NewLazySystemDLL("shell32.dll")
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterClassEx       = user32.NewProc("RegisterClassExW")
	procUnregisterClass       = user32.NewProc("UnregisterClassW")
	procCreateWindowEx        = user32.NewProc("CreateWindowExW")
	procDefWindowProc         = user32.NewProc("DefWindowProcW")
	procDestroyWindow         = user32.NewProc("DestroyWindow")
	procGetMessage            = user32.NewProc("GetMessageW")
	procTranslateMessage      = user32.NewProc("TranslateMessage")
	procDispatchMessage       = user32.NewProc("DispatchMessageW")
	procPostMessage           = user32.NewProc("PostMessageW")
	procPostQuitMessage       = user32.NewProc("PostQuitMessage")
	procLoadCursor            = user32.NewProc("LoadCursorW")
	procLoadImage             = user32.NewProc("LoadImageW")
	procDestroyIcon           = user32.NewProc("DestroyIcon")
	procCreatePopupMenu       = user32.NewProc("CreatePopupMenu")
	procDestroyMenu           = user32.NewProc("DestroyMenu")
	procAppendMenu            = user32.NewProc("AppendMenuW")
	procGetCursorPos          = user32.NewProc("GetCursorPos")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procFindWindow            = user32.NewProc("FindWindowW")
	procTrackPopupMenu        = user32.NewProc("TrackPopupMenu")
	procRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
	procShellNotifyIcon       = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandle       = kernel32.NewProc("GetModuleHandleW")

	stateMu sync.Mutex
	state   *trayState
)

type trayState struct {
	window         windows.Handle
	menu           windows.Handle
	icon           windows.Handle
	instance       windows.Handle
	className      *uint16
	nid            notifyIconData
	taskbarCreated uint32
	onOpen         func()
	onExit         func()
	onCommand      func(Command) bool
	onStatus       func() uintptr
	openMu         sync.Mutex
	lastOpen       time.Time
	exitOnce       sync.Once
}

type windowClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type notifyIconData struct {
	Size             uint32
	Window           windows.Handle
	ID               uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             windows.Handle
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	GUIDItem         windows.GUID
	BalloonIcon      windows.Handle
}

type point struct {
	X int32
	Y int32
}

type message struct {
	Window  windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   point
	Private uint32
}

// Run creates a Windows notification-area icon and blocks on its native
// message loop. It intentionally exposes only the two actions this app needs,
// keeping the resident background binary and allocations small.
func Run(iconData []byte, onOpen, onExit func(), onCommand func(Command) bool, onStatus func() uintptr) error {
	current, err := newTray(iconData, onOpen, onExit)
	if err != nil {
		return err
	}
	current.onCommand, current.onStatus = onCommand, onStatus
	stateMu.Lock()
	state = current
	stateMu.Unlock()
	defer current.destroy()

	var msg message
	for {
		result, _, callErr := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch result {
		case ^uintptr(0):
			return fmt.Errorf("读取托盘消息失败: %w", callErr)
		case 0:
			return nil
		default:
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func SetTooltip(value string) {
	stateMu.Lock()
	defer stateMu.Unlock()
	if state == nil || state.window == 0 {
		return
	}
	clear(state.nid.Tip[:])
	copy(state.nid.Tip[:], utf16(value, len(state.nid.Tip))[:])
	state.nid.Flags |= nifTip
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&state.nid)))
}

func Quit() {
	stateMu.Lock()
	current := state
	stateMu.Unlock()
	if current != nil && current.window != 0 {
		procPostMessage.Call(uintptr(current.window), wmClose, 0, 0)
	}
}

// QuitExisting asks the resident tray process, if any, to remove its icon and
// terminate immediately. It is safe to call from the GUI process.
func QuitExisting() bool {
	window := existingWindow()
	if window == 0 {
		return false
	}
	posted, _, _ := procPostMessage.Call(uintptr(window), wmClose, 0, 0)
	return posted != 0
}

// IsRunning reports whether this Windows session already has the lightweight
// tray process, allowing executable launches to behave like a tray restore.
func IsRunning() bool {
	return existingWindow() != 0
}

func existingWindow() windows.Handle {
	className, _ := windows.UTF16PtrFromString("NetworkToolboxTrayWindow")
	windowName, _ := windows.UTF16PtrFromString("Network Toolbox Background")
	window, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)))
	return windows.Handle(window)
}

func newTray(iconData []byte, onOpen, onExit func()) (*trayState, error) {
	className, _ := windows.UTF16PtrFromString("NetworkToolboxTrayWindow")
	windowName, _ := windows.UTF16PtrFromString("Network Toolbox Background")
	instance, _, err := procGetModuleHandle.Call(0)
	if instance == 0 {
		return nil, err
	}
	cursor, _, _ := procLoadCursor.Call(0, 32512)
	class := windowClassEx{
		Size: uint32(unsafe.Sizeof(windowClassEx{})), WndProc: windows.NewCallback(windowProc),
		Instance: windows.Handle(instance), Cursor: windows.Handle(cursor), ClassName: className,
	}
	if atom, _, registerErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		return nil, fmt.Errorf("注册托盘窗口失败: %w", registerErr)
	}
	window, _, createErr := procCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)), 0,
		0, 0, 0, 0, 0, 0, instance, 0,
	)
	if window == 0 {
		procUnregisterClass.Call(uintptr(unsafe.Pointer(className)), instance)
		return nil, fmt.Errorf("创建托盘窗口失败: %w", createErr)
	}
	iconPath, err := cacheIcon(iconData)
	if err != nil {
		procDestroyWindow.Call(window)
		return nil, err
	}
	path, _ := windows.UTF16PtrFromString(iconPath)
	icon, _, loadErr := procLoadImage.Call(0, uintptr(unsafe.Pointer(path)), imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
	if icon == 0 {
		procDestroyWindow.Call(window)
		return nil, fmt.Errorf("加载托盘图标失败: %w", loadErr)
	}
	menu, _, menuErr := procCreatePopupMenu.Call()
	if menu == 0 {
		procDestroyIcon.Call(icon)
		procDestroyWindow.Call(window)
		return nil, fmt.Errorf("创建托盘菜单失败: %w", menuErr)
	}
	openText, _ := windows.UTF16PtrFromString("打开网络工具箱")
	quitText, _ := windows.UTF16PtrFromString("退出后台")
	procAppendMenu.Call(menu, mfString, menuOpen, uintptr(unsafe.Pointer(openText)))
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	procAppendMenu.Call(menu, mfString, menuQuit, uintptr(unsafe.Pointer(quitText)))
	taskbarMessage, _, _ := procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(mustUTF16Ptr("TaskbarCreated"))))
	if taskbarMessage == 0 || taskbarMessage > uintptr(^uint32(0)) {
		procDestroyMenu.Call(menu)
		procDestroyIcon.Call(icon)
		procDestroyWindow.Call(window)
		return nil, errors.New("注册任务栏恢复消息失败")
	}
	result := &trayState{
		window: windows.Handle(window), menu: windows.Handle(menu), icon: windows.Handle(icon),
		instance: windows.Handle(instance), className: className, taskbarCreated: uint32(taskbarMessage),
		onOpen: onOpen, onExit: onExit,
	}
	result.nid = notifyIconData{
		Size: uint32(unsafe.Sizeof(notifyIconData{})), Window: result.window, ID: 1,
		Flags: nifMessage | nifIcon | nifTip | nifShowTip, CallbackMessage: wmTray, Icon: result.icon,
	}
	copy(result.nid.Tip[:], utf16("网络工具箱", len(result.nid.Tip)))
	if added, _, addErr := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&result.nid))); added == 0 {
		result.destroy()
		return nil, fmt.Errorf("添加托盘图标失败: %w", addErr)
	}
	result.nid.TimeoutOrVersion = notifyVersion4
	procShellNotifyIcon.Call(nimSetVersion, uintptr(unsafe.Pointer(&result.nid)))
	return result, nil
}

func windowProc(window windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	stateMu.Lock()
	current := state
	stateMu.Unlock()
	if current == nil {
		result, _, _ := procDefWindowProc.Call(uintptr(window), uintptr(msg), wParam, lParam)
		return result
	}
	switch msg {
	case wmClose:
		procDestroyWindow.Call(uintptr(window))
		return 0
	case wmBackgroundCommand:
		if current.onCommand != nil && current.onCommand(Command(wParam)) {
			return 1
		}
		return 0
	case wmBackgroundStatus:
		if current.onStatus != nil {
			return current.onStatus()
		}
		return 0
	case wmDestroy:
		current.exitOnce.Do(func() {
			procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&current.nid)))
			if current.onExit != nil {
				current.onExit()
			}
		})
		procPostQuitMessage.Call(0)
		return 0
	case wmTray:
		switch uint32(lParam & 0xffff) {
		case wmLButtonUp, wmLButtonDbl, ninSelect, ninKeySelect:
			current.requestOpen()
		case wmRButtonUp, wmContextMenu:
			current.showMenu()
		}
		return 0
	default:
		if msg == current.taskbarCreated {
			procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&current.nid)))
			procShellNotifyIcon.Call(nimSetVersion, uintptr(unsafe.Pointer(&current.nid)))
			return 0
		}
	}
	result, _, _ := procDefWindowProc.Call(uintptr(window), uintptr(msg), wParam, lParam)
	return result
}

func (t *trayState) showMenu() {
	var cursor point
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok == 0 {
		return
	}
	procSetForegroundWindow.Call(uintptr(t.window))
	// #nosec G115 -- POINT coordinates are signed Win32 ABI values passed through uintptr.
	command, _, _ := procTrackPopupMenu.Call(uintptr(t.menu), tpmRightButton|tpmReturnCmd, uintptr(cursor.X), uintptr(cursor.Y), 0, uintptr(t.window), 0)
	procPostMessage.Call(uintptr(t.window), 0, 0, 0)
	switch command {
	case menuOpen:
		t.requestOpen()
	case menuQuit:
		Quit()
	}
}

func (t *trayState) requestOpen() {
	t.openMu.Lock()
	if time.Since(t.lastOpen) < 1500*time.Millisecond {
		t.openMu.Unlock()
		return
	}
	t.lastOpen = time.Now()
	onOpen := t.onOpen
	t.openMu.Unlock()
	if onOpen != nil {
		go onOpen()
	}
}

func (t *trayState) destroy() {
	t.exitOnce.Do(func() {
		procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&t.nid)))
		if t.onExit != nil {
			t.onExit()
		}
	})
	if t.menu != 0 {
		procDestroyMenu.Call(uintptr(t.menu))
		t.menu = 0
	}
	if t.icon != 0 {
		procDestroyIcon.Call(uintptr(t.icon))
		t.icon = 0
	}
	if t.window != 0 {
		procDestroyWindow.Call(uintptr(t.window))
		t.window = 0
	}
	if t.className != nil && t.instance != 0 {
		procUnregisterClass.Call(uintptr(unsafe.Pointer(t.className)), uintptr(t.instance))
	}
	stateMu.Lock()
	if state == t {
		state = nil
	}
	stateMu.Unlock()
}

func cacheIcon(data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("托盘图标数据为空")
	}
	hash := sha256.Sum256(data)
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "CampusNetToolbox", fmt.Sprintf("tray-%x.ico", hash[:6]))
	if cachedIconMatches(path, data) {
		return path, nil
	}
	if err := privatefile.Write(path, data); err != nil {
		return "", err
	}
	return path, nil
}

func cachedIconMatches(path string, expected []byte) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	actual, err := io.ReadAll(io.LimitReader(file, int64(len(expected))+1))
	return err == nil && bytes.Equal(actual, expected)
}

func utf16(value string, limit int) []uint16 {
	encoded, _ := windows.UTF16FromString(value)
	if len(encoded) > limit {
		encoded = encoded[:limit]
		encoded[limit-1] = 0
	}
	return encoded
}

func mustUTF16Ptr(value string) *uint16 {
	result, _ := windows.UTF16PtrFromString(value)
	return result
}
