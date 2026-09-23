package desktop

import (
	"errors"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	mainWindowTitle = "网络工具箱"
	mainMutexName   = `Local\NetworkToolbox.MainWindow.6f19c10d`
	swRestore       = 9
)

var (
	mainUser32               = windows.NewLazySystemDLL("user32.dll")
	procFindWindow           = mainUser32.NewProc("FindWindowW")
	procShowWindow           = mainUser32.NewProc("ShowWindow")
	procSetMainForeground    = mainUser32.NewProc("SetForegroundWindow")
	procBringMainWindowToTop = mainUser32.NewProc("BringWindowToTop")
)

func acquireMainInstance() (release func(), alreadyRunning bool, err error) {
	name, _ := windows.UTF16PtrFromString(mainMutexName)
	handle, createErr := windows.CreateMutex(nil, false, name)
	if handle == 0 {
		return nil, false, createErr
	}
	if errors.Is(createErr, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(handle)
		focusExistingMainWindow()
		return func() {}, true, nil
	}
	if createErr != nil {
		_ = windows.CloseHandle(handle)
		return nil, false, createErr
	}
	var once sync.Once
	return func() { once.Do(func() { _ = windows.CloseHandle(handle) }) }, false, nil
}

func isMainInstanceRunning() bool {
	name, _ := windows.UTF16PtrFromString(mainMutexName)
	handle, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(handle)
	return true
}

func focusExistingMainWindow() bool {
	title, _ := windows.UTF16PtrFromString(mainWindowTitle)
	window, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(title)))
	if window == 0 {
		return false
	}
	procShowWindow.Call(window, swRestore)
	procBringMainWindowToTop.Call(window)
	procSetMainForeground.Call(window)
	return true
}
