//go:build windows

package sys

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ---------------------------------------------------------------- autostart

// NextBox starts with Windows through the Run key of the current user, or,
// when TUN mode needs administrator rights, through a scheduled task that
// runs at logon with the highest privileges (a Run entry cannot start an
// elevated program without a UAC prompt, so it would silently fail).

const (
	runKey          = `Software\Microsoft\Windows\CurrentVersion\Run`
	runOnceKey      = `Software\Microsoft\Windows\CurrentVersion\RunOnce`
	startupApproved = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
	autoStartName   = "NextBox"
	cleanupName     = "NextBoxProxyCleanup"
)

// AutoStart methods.
const (
	AutoStartRun  = "run"
	AutoStartTask = "task"
)

// AutoStartInfo describes how NextBox is registered to start with Windows.
type AutoStartInfo struct {
	Method   string `json:"method"`   // AutoStartRun, AutoStartTask or "" (not registered)
	Path     string `json:"path"`     // the program the entry starts
	Disabled bool   `json:"disabled"` // switched off in Task Manager / Settings → Startup apps
}

func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return exe, nil
}

// SetAutoStart registers (method AutoStartRun or AutoStartTask) or removes
// (method "") the autostart entry. Creating the task needs administrator
// rights.
func SetAutoStart(method string) error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	switch method {
	case AutoStartRun:
		deleteTask() // best effort: deleting an elevated task may need admin rights
		k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		enableStartupEntry()
		return k.SetStringValue(autoStartName, `"`+exe+`" --minimized`)
	case AutoStartTask:
		if !IsElevated() {
			return errors.New("administrator rights required")
		}
		if err := createTask(exe); err != nil {
			return err
		}
		removeRunValue()
		return nil
	}
	removeRunValue()
	deleteTask()
	return nil
}

func removeRunValue() {
	if k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE); err == nil {
		k.DeleteValue(autoStartName)
		k.Close()
	}
}

// enableStartupEntry clears a "disabled" mark left by Task Manager or by
// Settings → Apps → Startup for the Run entry.
func enableStartupEntry() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, startupApproved, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	if err := k.DeleteValue(autoStartName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}

// GetAutoStart reports the current registration.
func GetAutoStart() AutoStartInfo {
	if cmd, args, enabled, ok := queryTask(); ok {
		_ = args
		return AutoStartInfo{Method: AutoStartTask, Path: cmd, Disabled: !enabled}
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return AutoStartInfo{}
	}
	defer k.Close()
	v, _, err := k.GetStringValue(autoStartName)
	if err != nil {
		return AutoStartInfo{}
	}
	return AutoStartInfo{Method: AutoStartRun, Path: commandPath(v), Disabled: startupDisabled()}
}

// SameExe reports whether path is this program.
func SameExe(path string) bool {
	exe, err := exePath()
	if err != nil || path == "" {
		return false
	}
	if p, err := filepath.EvalSymlinks(path); err == nil {
		path = p
	}
	return strings.EqualFold(filepath.Clean(path), filepath.Clean(exe))
}

// commandPath extracts the program from a command line.
func commandPath(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if strings.HasPrefix(cmd, `"`) {
		if end := strings.Index(cmd[1:], `"`); end >= 0 {
			return cmd[1 : end+1]
		}
	}
	if i := strings.Index(strings.ToLower(cmd), ".exe"); i >= 0 {
		return cmd[:i+4]
	}
	return cmd
}

// startupDisabled reads the StartupApproved flag: the first byte is odd when
// the user switched the entry off.
func startupDisabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, startupApproved, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	b, _, err := k.GetBinaryValue(autoStartName)
	return err == nil && len(b) > 0 && b[0]&1 == 1
}

// RepairAutoStart re-enables an entry that was switched off in Windows.
func RepairAutoStart() error { return enableStartupEntry() }

func schtasks(args ...string) ([]byte, error) {
	cmd := exec.Command("schtasks.exe", args...)
	PrepareCmd(cmd)
	return cmd.CombinedOutput()
}

func deleteTask() { schtasks("/Delete", "/TN", autoStartName, "/F") }

type taskXML struct {
	Settings struct {
		Enabled *bool `xml:"Enabled"`
	} `xml:"Settings"`
	Triggers struct {
		Logon []struct {
			Enabled *bool `xml:"Enabled"`
		} `xml:"LogonTrigger"`
	} `xml:"Triggers"`
	Actions struct {
		Exec []struct {
			Command   string `xml:"Command"`
			Arguments string `xml:"Arguments"`
		} `xml:"Exec"`
	} `xml:"Actions"`
}

func queryTask() (command, args string, enabled, ok bool) {
	out, err := schtasks("/Query", "/TN", autoStartName, "/XML")
	if err != nil {
		return "", "", false, false
	}
	// schtasks prints UTF-16 declared XML in the console code page; the
	// declaration only gets in the way of the decoder
	text := string(out)
	if i := strings.Index(text, "?>"); i >= 0 {
		text = text[i+2:]
	}
	var t taskXML
	if xml.Unmarshal([]byte(text), &t) != nil || len(t.Actions.Exec) == 0 {
		return "", "", false, false
	}
	enabled = t.Settings.Enabled == nil || *t.Settings.Enabled
	for _, l := range t.Triggers.Logon {
		if l.Enabled != nil && !*l.Enabled {
			enabled = false
		}
	}
	return strings.Trim(t.Actions.Exec[0].Command, `"`), t.Actions.Exec[0].Arguments, enabled, true
}

func createTask(exe string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	esc := func(s string) string {
		var b bytes.Buffer
		xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	doc := `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Description>NextBox</Description></RegistrationInfo>
  <Triggers>
    <LogonTrigger><Enabled>true</Enabled><UserId>` + esc(u.Username) + `</UserId><Delay>PT3S</Delay></LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author"><UserId>` + esc(u.Username) + `</UserId><LogonType>InteractiveToken</LogonType><RunLevel>HighestAvailable</RunLevel></Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <IdleSettings><StopOnIdleEnd>false</StopOnIdleEnd><RestartOnIdle>false</RestartOnIdle></IdleSettings>
    <Enabled>true</Enabled>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>4</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec><Command>"` + esc(exe) + `"</Command><Arguments>--minimized</Arguments><WorkingDirectory>` + esc(filepath.Dir(exe)) + `</WorkingDirectory></Exec>
  </Actions>
</Task>`
	f, err := os.CreateTemp("", "nextbox-task-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	// UTF-16 LE with a byte order mark, as the declaration says
	buf := []byte{0xFF, 0xFE}
	for _, c := range utf16.Encode([]rune(doc)) {
		buf = binary.LittleEndian.AppendUint16(buf, c)
	}
	_, err = f.Write(buf)
	f.Close()
	if err != nil {
		return err
	}
	if out, err := schtasks("/Create", "/TN", autoStartName, "/XML", f.Name(), "/F"); err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

// ---------------------------------------------------------------- proxy cleanup

// SetProxyCleanup arranges for the system proxy pointing at addr to be
// removed at the next logon, in case NextBox is killed before it can do so
// itself (a forced shutdown, a crash, a power cut). Windows runs RunOnce
// entries once and deletes them. addr "" cancels it.
func SetProxyCleanup(addr string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runOnceKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if addr == "" {
		if err := k.DeleteValue(cleanupName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	exe, err := exePath()
	if err != nil {
		return err
	}
	return k.SetStringValue(cleanupName, `"`+exe+`" --restore-proxy=`+addr)
}

// ---------------------------------------------------------------- session end

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassEx   = user32.NewProc("RegisterClassExW")
	procCreateWindowEx    = user32.NewProc("CreateWindowExW")
	procDefWindowProc     = user32.NewProc("DefWindowProcW")
	procGetMessage        = user32.NewProc("GetMessageW")
	procDispatchMessage   = user32.NewProc("DispatchMessageW")
	procSetShutdownParams = kernel32.NewProc("SetProcessShutdownParameters")
	sessionEndFn          func()
	sessionEndCalled      bool
)

type wndClassEx struct {
	size, style                        uint32
	wndProc                            uintptr
	clsExtra, wndExtra                 int32
	instance, icon, cursor, background windows.Handle
	menuName, className                *uint16
	iconSm                             windows.Handle
}

type msg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// OnSessionEnd calls fn as soon as Windows asks whether it may log off or
// shut down (WM_QUERYENDSESSION), before the other windows of this process
// are torn down. It runs on a hidden window of its own; fn must be quick.
// NextBox also asks to be among the first programs notified.
func OnSessionEnd(fn func()) {
	sessionEndFn = fn
	procSetShutdownParams.Call(0x3FF, 0)
	go func() {
		runtime.LockOSThread()
		const (
			wmQueryEndSession = 0x0011
			wmEndSession      = 0x0016
		)
		name, _ := windows.UTF16PtrFromString("NextBoxSessionWatcher")
		proc := windows.NewCallback(func(hwnd windows.Handle, m uint32, w, l uintptr) uintptr {
			if (m == wmQueryEndSession || (m == wmEndSession && w != 0)) && !sessionEndCalled {
				sessionEndCalled = true
				sessionEndFn()
			}
			if m == wmQueryEndSession {
				return 1 // allow the shutdown
			}
			r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(m), w, l)
			return r
		})
		wc := wndClassEx{wndProc: proc, className: name}
		wc.size = uint32(unsafe.Sizeof(wc))
		if r, _, _ := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			return
		}
		// a hidden top-level window: message-only windows get no broadcasts
		hwnd, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, 0, 0, 0)
		if hwnd == 0 {
			return
		}
		var m msg
		for {
			r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
}
