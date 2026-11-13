package main

// Starting with Windows, and leaving nothing behind: temporary files, and a
// system proxy pointing at a port nobody listens on any more.

import (
	"errors"
	"os"
	"path/filepath"

	"nextbox/internal/i18n"
	"nextbox/internal/store"
	"nextbox/internal/sys"
)

// removeLeftovers deletes files of older versions and of a run that did not
// end cleanly: the latency test directory (now "cache") and core configs,
// which held node passwords in plain text before they went through stdin.
func (a *App) removeLeftovers() {
	os.RemoveAll(filepath.Join(a.dataDir(), "test"))
	os.Remove(filepath.Join(a.runDir(), "config.json"))
	a.clearCache()
}

// clearCache empties the directory latency tests run in.
func (a *App) clearCache() {
	entries, err := os.ReadDir(a.cacheDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		os.RemoveAll(filepath.Join(a.cacheDir(), e.Name()))
	}
}

// onSessionEnd runs when Windows logs off or shuts down. It has very little
// time, so the system proxy goes first: a browser after the next boot must
// not point at a port that is gone.
func (a *App) onSessionEnd() {
	a.applySystemProxy(store.Settings{}, false)
	if a.store != nil {
		a.store.Flush()
	}
}

// ---------------------------------------------------------------- autostart

// AutoStartStatus is what the settings page shows next to 开机自启.
type AutoStartStatus struct {
	Enabled bool   `json:"enabled"` // the setting
	OK      bool   `json:"ok"`      // Windows will really start NextBox
	Method  string `json:"method"`  // sys.AutoStartRun or sys.AutoStartTask
	// Problem is "missing" (no entry), "moved" (the entry starts another
	// copy of NextBox), "disabled" (switched off in Windows), "tun" (TUN mode
	// needs administrator rights, which the Run entry cannot give) or
	// "error" (Detail says why).
	Problem string `json:"problem"`
	Detail  string `json:"detail,omitempty"`
}

// wantedAutoStart is the method that suits the settings: a scheduled task
// with administrator rights for TUN mode (it can only be created while
// running as administrator), the Run entry otherwise.
func (a *App) wantedAutoStart(s store.Settings) string {
	if !s.AutoStart {
		return ""
	}
	if s.TunEnabled && sys.IsElevated() {
		return sys.AutoStartTask
	}
	if cur := sys.GetAutoStart(); cur.Method == sys.AutoStartTask && sys.SameExe(cur.Path) && !sys.IsElevated() {
		return sys.AutoStartTask // keep it: it cannot be replaced without rights anyway
	}
	return sys.AutoStartRun
}

func (a *App) applyAutoStart(s store.Settings) error {
	method := a.wantedAutoStart(s)
	if method == sys.AutoStartTask {
		if cur := sys.GetAutoStart(); cur.Method == sys.AutoStartTask && sys.SameExe(cur.Path) {
			return nil
		}
	}
	err := sys.SetAutoStart(method)
	if err != nil {
		a.log(i18n.T("设置开机自启失败: ") + err.Error())
	}
	return err
}

func (a *App) GetAutoStartStatus() AutoStartStatus {
	s := a.store.Settings()
	st := AutoStartStatus{Enabled: s.AutoStart}
	if !s.AutoStart {
		return st
	}
	info := sys.GetAutoStart()
	st.Method = info.Method
	switch {
	case info.Method == "":
		st.Problem = "missing"
	case !sys.SameExe(info.Path):
		st.Problem, st.Detail = "moved", info.Path
	case info.Disabled:
		st.Problem = "disabled"
	case s.TunEnabled && info.Method == sys.AutoStartRun:
		st.Problem = "tun"
	default:
		st.OK = true
	}
	return st
}

// RepairAutoStart fixes what GetAutoStartStatus found.
func (a *App) RepairAutoStart() (AutoStartStatus, error) {
	s := a.store.Settings()
	st := a.GetAutoStartStatus()
	var err error
	switch st.Problem {
	case "":
		return st, nil
	case "disabled":
		if st.Method == sys.AutoStartRun {
			err = sys.RepairAutoStart()
		} else {
			err = a.recreateAutoStart(s)
		}
	case "tun":
		if !sys.IsElevated() {
			return st, errors.New(i18n.T("TUN 模式开机自启需要管理员权限：请先点击“以管理员身份重启”，再点修复"))
		}
		err = sys.SetAutoStart(sys.AutoStartTask)
	default:
		err = a.recreateAutoStart(s)
	}
	if err != nil {
		return a.GetAutoStartStatus(), err
	}
	return a.GetAutoStartStatus(), nil
}

func (a *App) recreateAutoStart(s store.Settings) error {
	method := a.wantedAutoStart(s)
	if cur := sys.GetAutoStart(); cur.Method == sys.AutoStartTask && !sys.IsElevated() {
		return errors.New(i18n.T("开机自启由管理员权限的计划任务负责，修复需要管理员权限：请先点击“以管理员身份重启”"))
	}
	return sys.SetAutoStart(method)
}
