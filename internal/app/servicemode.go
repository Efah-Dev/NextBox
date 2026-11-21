package app

// 服务模式 (v2.3): the NextBox service runs the core in TUN mode, so that
// NextBox does not have to run as administrator, and starts NextBox when
// the user logs on. Installing and removing it are the only steps that need
// administrator rights (one UAC prompt each); both are the user's choice.
// See internal/service.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nextbox/internal/i18n"
	"nextbox/internal/service"
	"nextbox/internal/sys"
)

// ServiceInfo is what the TUN section shows about the service.
type ServiceInfo struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Usable    bool   `json:"usable"` // TUN mode can use it now
	Dir       string `json:"dir"`    // where it is (or would be) installed
	Version   string `json:"version,omitempty"`
	// Problem is "stopped" (installed but not running), "notAllowed"
	// (installed by another Windows user), "outdated" (from another NextBox
	// version, or with another core than NextBox's own folder: 更新服务) or
	// "error" (Detail).
	Problem string `json:"problem,omitempty"`
	Detail  string `json:"detail,omitempty"`
	// CanInstall is false when NextBox's folder lacks the files to install
	// from (core\NextBoxService.exe).
	CanInstall bool `json:"canInstall"`
}

func (a *App) GetServiceInfo() ServiceInfo {
	info := ServiceInfo{Dir: service.Dir()}
	// whether TUN mode needs administrator rights shows on several pages:
	// tell them when that changed
	was := a.serviceOK.Swap(false)
	defer func() {
		if a.serviceOK.Load() != was && a.ctx != nil {
			go a.emitState()
		}
	}()
	_, err := os.Stat(filepath.Join(a.coreDir(), service.ExeName))
	info.CanInstall = err == nil
	q := service.QueryInstalled()
	info.Installed, info.Running = q.Installed, q.Running
	if !q.Installed {
		return info
	}
	if !q.Running {
		info.Problem = "stopped"
		return info
	}
	ctx, cancel := context.WithTimeout(a.ctx, 3*time.Second)
	defer cancel()
	st, err := service.Hello(ctx)
	switch {
	case errors.Is(err, service.ErrNotAllowed):
		info.Problem = "notAllowed"
		return info
	case err != nil:
		info.Problem, info.Detail = "error", err.Error()
		return info
	}
	info.Version = st.Version
	info.Usable = st.Protocol == service.Protocol
	a.serviceOK.Store(info.Usable)
	if !info.Usable || st.Version != Version || (st.CoreHash != "" && st.CoreHash != service.FileHash(a.corePath())) {
		info.Problem = "outdated"
	}
	return info
}

// serviceUsable tells why TUN mode cannot run through the service, nil
// when it can.
func (a *App) serviceUsable() error {
	info := a.GetServiceInfo()
	switch {
	case info.Usable:
		return nil
	case !info.Installed:
		return errors.New(i18n.T("TUN 模式需要管理员权限：请在「设置 → TUN 模式」安装 NextBox 服务（只需授权一次），或以管理员身份重启 NextBox"))
	case info.Problem == "stopped":
		return errors.New(i18n.T("NextBox 服务没有运行：请在「设置 → TUN 模式」重新安装服务，或在 Windows 的「服务」里启动 NextBox Service"))
	case info.Problem == "notAllowed":
		return errors.New(i18n.T("NextBox 服务是另一个 Windows 账户安装的：请在「设置 → TUN 模式」点「安装服务」，允许当前账户使用"))
	case info.Problem == "outdated":
		return errors.New(i18n.T("NextBox 服务的版本和当前程序不一致：请在「设置 → TUN 模式」点「更新服务」"))
	}
	return errors.New(i18n.T("无法连接 NextBox 服务: ") + info.Detail)
}

func serviceError(err error) error {
	switch {
	case errors.Is(err, service.ErrNotRunning):
		return errors.New(i18n.T("NextBox 服务没有运行"))
	case errors.Is(err, service.ErrNotAllowed):
		return errors.New(i18n.T("NextBox 服务是另一个 Windows 账户安装的"))
	}
	return errors.New(i18n.T("NextBox 服务: ") + err.Error())
}

func (a *App) coreDir() string { return filepath.Dir(a.corePath()) }

// InstallService installs or updates the service (a UAC prompt).
func (a *App) InstallService() (ServiceInfo, error) {
	if _, err := os.Stat(filepath.Join(a.coreDir(), service.ExeName)); err != nil {
		return a.GetServiceInfo(), errors.New(i18n.T("程序目录的 core 文件夹里缺少 NextBoxService.exe，请重新下载完整的 NextBox"))
	}
	sid, err := service.CurrentUserSID()
	if err != nil {
		return a.GetServiceInfo(), err
	}
	// the core may run in the service right now: it is restarted afterwards
	wasRunning := a.isRunning() && a.store.Settings().TunEnabled && !sys.IsElevated()
	if wasRunning {
		a.Disconnect()
	}
	err = a.runServiceHelper("--service-install --user=" + sid)
	if err == nil {
		a.log(i18n.T("已安装 NextBox 服务: ") + service.Dir())
		a.applyAutoStart(a.store.Settings()) // 开机自启 now goes through the service
	}
	if wasRunning {
		go a.Connect()
	}
	// refresh what the app state says about the service before sending it:
	// the TUN prompts for administrator rights go away at once
	info := a.GetServiceInfo()
	a.emitState()
	return info, err
}

// UninstallService removes the service (a UAC prompt). TUN mode then needs
// NextBox to run as administrator again.
func (a *App) UninstallService() (ServiceInfo, error) {
	if a.isRunning() && a.store.Settings().TunEnabled && !sys.IsElevated() {
		a.Disconnect()
	}
	err := a.runServiceHelper("--service-uninstall")
	if err == nil {
		a.log(i18n.T("已卸载 NextBox 服务"))
	}
	a.applyAutoStart(a.store.Settings()) // back to the Run key / scheduled task
	info := a.GetServiceInfo()
	a.emitState()
	return info, err
}

// runServiceHelper runs `NextBox.exe <args>` as administrator and returns
// what it reported (see serviceHelper).
func (a *App) runServiceHelper(args string) error {
	result := filepath.Join(a.dataDir(), "service-result.txt")
	os.Remove(result)
	code, err := sys.RunElevated(args + ` --result="` + result + `"`)
	if errors.Is(err, sys.ErrCancelled) {
		return errors.New(i18n.T("已取消：没有获得管理员授权"))
	}
	if err != nil {
		return err
	}
	if code != 0 {
		msg, _ := os.ReadFile(result)
		os.Remove(result)
		if len(msg) == 0 {
			return fmt.Errorf(i18n.T("操作失败（代码 %d）"), code)
		}
		return errors.New(strings.TrimSpace(string(msg)))
	}
	os.Remove(result)
	return nil
}

// serviceHelper is what NextBox.exe does when started by runServiceHelper:
// it installs or removes the service and exits, without any window. It
// returns the exit code.
func serviceHelper(args []string) int {
	var op, sid, result string
	for _, arg := range args {
		switch {
		case arg == "--service-install" || arg == "--service-uninstall":
			op = arg
		case strings.HasPrefix(arg, "--user="):
			sid = strings.TrimPrefix(arg, "--user=")
		case strings.HasPrefix(arg, "--result="):
			result = strings.Trim(strings.TrimPrefix(arg, "--result="), `"`)
		}
	}
	var err error
	switch op {
	case "--service-install":
		exe, _ := os.Executable()
		err = service.Install(filepath.Join(filepath.Dir(exe), "core"), sid, Version)
		if err == nil {
			// 开机自启 goes through the service from now on: the scheduled task
			// of earlier versions (which needs these rights to remove) goes
			sys.DeleteAutoStartTask()
		}
	case "--service-uninstall":
		err = service.Uninstall()
	default:
		err = errors.New("unknown operation")
	}
	if err != nil {
		if result != "" {
			os.WriteFile(result, []byte(err.Error()), 0o600)
		}
		return 1
	}
	return 0
}
