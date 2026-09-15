package main

import (
	"runtime/debug"

	"golang.org/x/sys/windows"
)

var procEmptyWorkingSet = windows.NewLazySystemDLL("psapi.dll").NewProc("EmptyWorkingSet")

// trimBackgroundWorkingSet is used only while the lightweight helper is
// sleeping. It returns unused Go heap pages and executable pages to Windows;
// pages needed by a later authentication attempt are loaded normally again.
func trimBackgroundWorkingSet() {
	debug.FreeOSMemory()
	procEmptyWorkingSet.Call(uintptr(windows.CurrentProcess()))
}
