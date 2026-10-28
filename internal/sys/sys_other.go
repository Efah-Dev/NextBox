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
func SetAutoStart(name string, enable bool) error {
	if enable {
		return errUnsupported()
	}
	return nil
}

func WaitPid(pid int) {
	for i := 0; i < 100; i++ {
		p, err := os.FindProcess(pid)
		if err != nil || p.Signal(syscall.Signal(0)) != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func PrepareCmd(cmd *exec.Cmd) {}
func AttachCmd(cmd *exec.Cmd)  {}
func CoreBinaryName() string   { return "sing-box" }
