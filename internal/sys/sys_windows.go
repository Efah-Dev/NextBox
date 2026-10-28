//go:build windows

package sys

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const internetSettings = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

var defaultBypass = strings.Join([]string{
	"localhost", "127.*", "10.*", "172.16.*", "172.17.*", "172.18.*", "172.19.*",
	"172.20.*", "172.21.*", "172.22.*", "172.23.*", "172.24.*", "172.25.*",
	"172.26.*", "172.27.*", "172.28.*", "172.29.*", "172.30.*", "172.31.*",
	"192.168.*", "<local>",
}, ";")

// SetSystemProxy points the WinINet (IE / Edge / Chrome) proxy at addr ("host:port").
func SetSystemProxy(addr string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, internetSettings, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue("ProxyServer", addr); err != nil {
		return err
	}
	if err := k.SetStringValue("ProxyOverride", defaultBypass); err != nil {
		return err
	}
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	notifyProxyChanged()
	return nil
}

// ClearSystemProxy disables the system proxy, but only if it still points to
// addr, so a proxy configured by another program is left alone.
func ClearSystemProxy(addr string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if current, _, err := k.GetStringValue("ProxyServer"); err == nil && addr != "" && current != addr {
		return nil
	}
	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return err
	}
	notifyProxyChanged()
	return nil
}

func notifyProxyChanged() {
	const (
		optionSettingsChanged = 39
		optionRefresh         = 37
	)
	wininet := windows.NewLazySystemDLL("wininet.dll")
	set := wininet.NewProc("InternetSetOptionW")
	set.Call(0, optionSettingsChanged, 0, 0)
	set.Call(0, optionRefresh, 0, 0)
}

func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// RestartAsAdmin starts a new elevated instance (UAC prompt). The caller must
// quit afterwards; the new instance waits for this process to exit.
func RestartAsAdmin() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	args, _ := syscall.UTF16PtrFromString("--wait-parent=" + strconv.Itoa(os.Getpid()))
	cwd, _ := os.Getwd()
	dir, _ := syscall.UTF16PtrFromString(cwd)
	return windows.ShellExecute(0, verb, file, args, dir, windows.SW_SHOWNORMAL)
}

// WaitPid blocks until the process exits (at most 10 seconds).
func WaitPid(pid int) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	windows.WaitForSingleObject(h, 10_000)
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func SetAutoStart(name string, enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enable {
		if err := k.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(name, `"`+exe+`" --minimized`)
}

var killJob windows.Handle

func init() {
	// Every child process is put into a job object that is closed when we exit,
	// so the core never outlives the GUI, even after a crash.
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		windows.CloseHandle(job)
		return
	}
	killJob = job
}

// PrepareCmd hides the console window of a child process.
func PrepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}

// AttachCmd binds a started child process to the kill-on-exit job.
func AttachCmd(cmd *exec.Cmd) {
	if killJob == 0 || cmd.Process == nil {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	windows.AssignProcessToJobObject(killJob, h)
}

func CoreBinaryName() string { return "sing-box.exe" }
