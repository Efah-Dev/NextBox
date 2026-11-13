//go:build !windows

package sys

import (
	"errors"
	"nextbox/internal/i18n"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func errUnsupported() error { return errors.New(i18n.T("当前系统暂不支持此功能")) }

func SetSystemProxy(addr string) error   { return errUnsupported() }
func ClearSystemProxy(addr string) error { return nil }
func IsElevated() bool                   { return os.Geteuid() == 0 }
func RestartAsAdmin() error              { return errUnsupported() }

// Autostart and session handling are Windows only.
const (
	AutoStartRun  = "run"
	AutoStartTask = "task"
)

type AutoStartInfo struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Disabled bool   `json:"disabled"`
}

func SetAutoStart(method string) error {
	if method != "" {
		return errUnsupported()
	}
	return nil
}
func GetAutoStart() AutoStartInfo       { return AutoStartInfo{} }
func SameExe(path string) bool          { return false }
func RepairAutoStart() error            { return errUnsupported() }
func SetProxyCleanup(addr string) error { return nil }
func OnSessionEnd(fn func())            {}

func WaitPid(pid int) {
	for i := 0; i < 100; i++ {
		p, err := os.FindProcess(pid)
		if err != nil || p.Signal(syscall.Signal(0)) != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Interrupt asks a child process to shut down by itself.
func Interrupt(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(os.Interrupt)
}

func PrepareCmd(cmd *exec.Cmd) {}
func AttachCmd(cmd *exec.Cmd)  {}
func CoreBinaryName() string   { return "sing-box" }

func Protect(data []byte) ([]byte, error)   { return nil, errUnsupported() }
func Unprotect(data []byte) ([]byte, error) { return nil, errUnsupported() }
