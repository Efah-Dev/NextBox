//go:build windows

package sys

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	// Dial-up (PPPoE) and VPN connections each have proxy settings of their
	// own, which replace the LAN ones while such a connection is up. The LAN
	// connection goes last, see resyncDefault.
	for _, conn := range rasConnections() {
		setConnProxy(conn, true, addr, defaultBypass)
	}
	if err := setConnProxy(nil, true, addr, defaultBypass); err != nil {
		// WinINet refused: write the legacy values ourselves.
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
	}
	notifyProxyChanged()
	return nil
}

// ClearSystemProxy disables the system proxy, but only if it still points to
// addr, so a proxy configured by another program is left alone.
//
// The address is removed as well, not just switched off: programs that read
// the settings on their own (download managers such as IDM) otherwise keep
// using the stale "ProxyServer" and fail on a port nobody listens on.
func ClearSystemProxy(addr string) error {
	rasChanged := false
	for _, conn := range rasConnections() {
		if addr == "" || connServer(conn) == addr {
			rasChanged = setConnProxy(conn, false, "", "") == nil || rasChanged
		}
	}
	defer notifyProxyChanged()
	if addr != "" && connServer(nil) != addr {
		// someone else's proxy: keep it, but see resyncDefault
		if rasChanged {
			resyncDefault()
		}
		return nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := setConnProxy(nil, false, "", ""); err != nil {
		if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
			return err
		}
		for _, name := range []string{"ProxyServer", "ProxyOverride"} {
			if err := k.DeleteValue(name); err != nil && err != registry.ErrNotExist {
				return err
			}
		}
	}
	return nil
}

const (
	optionRefresh         = 37
	optionSettingsChanged = 39
	optionPerConnection   = 75

	perConnFlags       = 1
	perConnProxyServer = 2
	perConnProxyBypass = 3
	perConnFlagsUI     = 10

	proxyTypeDirect = 1
	proxyTypeProxy  = 2
)

var (
	wininet         = windows.NewLazySystemDLL("wininet.dll")
	procSetOption   = wininet.NewProc("InternetSetOptionW")
	procQueryOption = wininet.NewProc("InternetQueryOptionW")

	rasapi32           = windows.NewLazySystemDLL("rasapi32.dll")
	procRasEnumEntries = rasapi32.NewProc("RasEnumEntriesW")

	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalFree    = kernel32.NewProc("GlobalFree")
	procAttachConsole = kernel32.NewProc("AttachConsole")
	procFreeConsole   = kernel32.NewProc("FreeConsole")
)

// perConnOption is INTERNET_PER_CONN_OPTIONW; value is its DWORD / LPWSTR /
// FILETIME union (uint64 gives the right size and alignment on 32 and 64 bit).
type perConnOption struct {
	option uint32
	value  uint64
}

// perConnOptionList is INTERNET_PER_CONN_OPTION_LISTW. A nil connection means
// the default (LAN) connection, anything else names a dial-up / VPN entry.
type perConnOptionList struct {
	size       uint32
	connection *uint16
	count      uint32
	optErr     uint32
	options    *perConnOption
}

// setConnProxy changes the proxy of one connection through WinINet, which
// keeps the binary settings blob and (for the LAN connection) the legacy
// registry values in step. Other settings (auto detect, PAC) are left alone.
func setConnProxy(conn *uint16, enable bool, server, bypass string) error {
	if err := procSetOption.Find(); err != nil {
		return err
	}
	flags := connFlags(conn) | proxyTypeDirect
	if enable {
		flags |= proxyTypeProxy
	} else {
		flags &^= proxyTypeProxy
	}
	return setConnOptions(conn, flags, server, bypass)
}

// resyncDefault writes the LAN connection's settings back unchanged. WinINet
// mirrors whichever connection was written last into the legacy registry
// values (ProxyEnable / ProxyServer), so after changing a dial-up / VPN entry
// this makes them describe the LAN connection again.
func resyncDefault() error {
	return setConnOptions(nil, connFlags(nil)|proxyTypeDirect,
		connString(nil, perConnProxyServer), connString(nil, perConnProxyBypass))
}

func setConnOptions(conn *uint16, flags uint32, server, bypass string) error {
	var srv, byp *uint16
	var err error
	if server != "" {
		if srv, err = windows.UTF16PtrFromString(server); err != nil {
			return err
		}
	}
	if bypass != "" {
		if byp, err = windows.UTF16PtrFromString(bypass); err != nil {
			return err
		}
	}
	opts := [3]perConnOption{
		{perConnFlags, uint64(flags)},
		{perConnProxyServer, uint64(uintptr(unsafe.Pointer(srv)))},
		{perConnProxyBypass, uint64(uintptr(unsafe.Pointer(byp)))},
	}
	list := perConnOptionList{connection: conn, count: uint32(len(opts)), options: &opts[0]}
	list.size = uint32(unsafe.Sizeof(list))
	r, _, callErr := procSetOption.Call(0, optionPerConnection, uintptr(unsafe.Pointer(&list)), uintptr(list.size))
	runtime.KeepAlive(srv)
	runtime.KeepAlive(byp)
	if r == 0 {
		return callErr
	}
	return nil
}

// connFlags returns the PROXY_TYPE_* flags of a connection.
func connFlags(conn *uint16) uint32 {
	if procQueryOption.Find() != nil {
		return proxyTypeDirect
	}
	opt := perConnOption{option: perConnFlagsUI}
	if !queryConnOption(conn, &opt) {
		return proxyTypeDirect
	}
	return uint32(opt.value)
}

// connServer returns the proxy address stored for a connection, even when the
// proxy is switched off.
func connServer(conn *uint16) string { return connString(conn, perConnProxyServer) }

func connString(conn *uint16, option uint32) string {
	if procQueryOption.Find() != nil {
		return ""
	}
	opt := perConnOption{option: option}
	if !queryConnOption(conn, &opt) {
		return ""
	}
	// WinINet hands the string over in a buffer the caller has to free.
	p := *(**uint16)(unsafe.Pointer(&opt.value))
	if p == nil {
		return ""
	}
	defer procGlobalFree.Call(uintptr(unsafe.Pointer(p)))
	return windows.UTF16PtrToString(p)
}

func queryConnOption(conn *uint16, opt *perConnOption) bool {
	list := perConnOptionList{connection: conn, count: 1, options: opt}
	list.size = uint32(unsafe.Sizeof(list))
	size := list.size
	r, _, _ := procQueryOption.Call(0, optionPerConnection, uintptr(unsafe.Pointer(&list)), uintptr(unsafe.Pointer(&size)))
	return r != 0
}

// rasEntryName is RASENTRYNAMEW.
type rasEntryName struct {
	size      uint32
	name      [257]uint16 // RAS_MaxEntryName + 1
	flags     uint32
	phonebook [261]uint16 // MAX_PATH + 1
}

// rasConnections lists the dial-up / VPN phone-book entries by name.
func rasConnections() []*uint16 {
	if procRasEnumEntries.Find() != nil {
		return nil
	}
	const errBufferTooSmall = 603
	entries := make([]rasEntryName, 1)
	for attempt := 0; attempt < 2; attempt++ {
		entrySize := uint32(unsafe.Sizeof(entries[0]))
		entries[0].size = entrySize
		bytes, count := entrySize*uint32(len(entries)), uint32(0)
		r, _, _ := procRasEnumEntries.Call(0, 0, uintptr(unsafe.Pointer(&entries[0])),
			uintptr(unsafe.Pointer(&bytes)), uintptr(unsafe.Pointer(&count)))
		if r == errBufferTooSmall {
			entries = make([]rasEntryName, bytes/entrySize+1)
			continue
		}
		if r != 0 || int(count) > len(entries) {
			return nil
		}
		out := make([]*uint16, 0, count)
		for i := range entries[:count] {
			out = append(out, &entries[i].name[0])
		}
		return out
	}
	return nil
}

func notifyProxyChanged() {
	if procSetOption.Find() != nil {
		return
	}
	procSetOption.Call(0, optionSettingsChanged, 0, 0)
	procSetOption.Call(0, optionRefresh, 0, 0)
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

// PrepareCmd hides the console window of a child process. The child gets a
// process group of its own, so that Interrupt can reach it.
func PrepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}
}

var consoleMu sync.Mutex // a process can be attached to one console at a time

// Interrupt asks a child started with PrepareCmd to shut down by itself, like
// pressing Ctrl+Break in its (hidden) console. It fails when this process has
// a console of its own (go test, wails dev); the caller then kills the child.
func Interrupt(pid int) error {
	consoleMu.Lock()
	defer consoleMu.Unlock()
	if r, _, err := procAttachConsole.Call(uintptr(pid)); r == 0 {
		return err
	}
	defer procFreeConsole.Call()
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(pid))
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
