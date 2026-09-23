package desktop

import (
	"runtime"
	"runtime/debug"

	"golang.org/x/sys/windows"
)

var procEmptyWorkingSet = windows.NewLazySystemDLL("psapi.dll").NewProc("EmptyWorkingSet")

func configureForegroundRuntime() {
	runtime.MemProfileRate = 0
	// The UI spends most of its time idle; trading a little collection work
	// during network bursts for a smaller retained heap is a good fit here.
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(24 << 20)
}

// releaseUnusedForegroundMemory returns temporary startup buffers (adapter
// enumeration, HTTP/TLS setup) after bootstrap without trimming live WebView
// pages or forcing a periodic foreground working-set purge.
func releaseUnusedForegroundMemory() {
	debug.FreeOSMemory()
}

// configureBackgroundRuntime keeps the headless tray helper intentionally
// small. Its workload is serial and infrequent, so one scheduler thread and a
// tighter heap growth target avoid reserving memory that only benefits the GUI.
func configureBackgroundRuntime() {
	runtime.GOMAXPROCS(1)
	runtime.MemProfileRate = 0
	debug.SetGCPercent(35)
	debug.SetMemoryLimit(16 << 20)
}

// trimBackgroundWorkingSet is used only while the lightweight helper is
// sleeping. It returns unused Go heap pages and executable pages to Windows;
// pages needed by a later authentication attempt are loaded normally again.
func trimBackgroundWorkingSet() {
	debug.FreeOSMemory()
	procEmptyWorkingSet.Call(uintptr(windows.CurrentProcess()))
}
