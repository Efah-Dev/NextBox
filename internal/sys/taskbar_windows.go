package sys

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procFindWindow         = user32.NewProc("FindWindowW")
	procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
)

// TaskbarReady reports whether the taskbar of this session exists and
// answers messages. Right after logon Explorer runs before its taskbar does,
// and adding a notification icon then can fail or time out.
func TaskbarReady() bool {
	name, _ := windows.UTF16PtrFromString("Shell_TrayWnd")
	hwnd, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(name)), 0)
	if hwnd == 0 {
		return false
	}
	const smtoAbortIfHung = 0x0002
	var result uintptr
	ok, _, _ := procSendMessageTimeout.Call(hwnd, 0 /* WM_NULL */, 0, 0, smtoAbortIfHung, 1000, uintptr(unsafe.Pointer(&result)))
	return ok != 0
}

// WaitTaskbar waits up to max for TaskbarReady and reports whether it came.
func WaitTaskbar(max time.Duration) bool {
	deadline := time.Now().Add(max)
	for !TaskbarReady() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
	return true
}
