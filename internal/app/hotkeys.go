package app

// System-wide hotkeys (Settings.Hotkeys): each action the user gave a key
// combination runs from anywhere, also while NextBox is in the tray.

import (
	"fmt"
	"slices"

	"nextbox/internal/i18n"
	"nextbox/internal/store"
	"nextbox/internal/sys"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// hotkeyActions are the actions a hotkey can run; a registration's ID is its
// index + 1. Keep in sync with the list in frontend/src/views/settings/HotkeySection.vue.
var hotkeyActions = []string{"toggleProxy", "systemProxy", "tun", "routeMode", "showWindow", "testGroup"}

func (a *App) startHotkeys() {
	a.hotkeys = sys.StartHotkeys(func(id int) {
		if id >= 1 && id <= len(hotkeyActions) {
			go a.runHotkey(hotkeyActions[id-1])
		}
	})
	a.applyHotkeys(a.store.Settings())
}

// applyHotkeys registers the combinations of the settings, replacing the
// ones before, and logs those another program has taken.
func (a *App) applyHotkeys(s store.Settings) {
	if a.hotkeys == nil {
		return
	}
	list := map[int]sys.Combo{}
	for action, combo := range s.Hotkeys {
		i := slices.Index(hotkeyActions, action)
		c, err := sys.ParseCombo(combo)
		if i < 0 || combo == "" || err != nil {
			continue
		}
		list[i+1] = c
	}
	for id, err := range a.hotkeys.Set(list) {
		if err != nil {
			a.log(fmt.Sprintf(i18n.T("快捷键 %s 已被其他程序占用，未生效"), s.Hotkeys[hotkeyActions[id-1]]))
		}
	}
}

// TestHotkey reports whether a combination can be used: it is valid and no
// other program has it. (Combinations NextBox itself uses for other actions
// are checked by the settings page.)
func (a *App) TestHotkey(combo string) error {
	c, err := sys.ParseCombo(combo)
	if err != nil {
		return err
	}
	for _, mine := range a.store.Settings().Hotkeys {
		if mc, err := sys.ParseCombo(mine); err == nil && mc == c {
			return nil // ours already: free once the old action lets it go
		}
	}
	if a.hotkeys == nil {
		return nil
	}
	return a.hotkeys.Test(c)
}

// notice shows a message in the bottom-right corner of the window.
func (a *App) notice(text, kind string) {
	wruntime.EventsEmit(a.ctx, "notice", map[string]string{"text": text, "kind": kind})
}

func (a *App) runHotkey(action string) {
	s := a.store.Settings()
	switch action {
	case "toggleProxy":
		if a.isRunning() {
			a.Disconnect()
			a.notice(i18n.T("已断开"), "info")
			return
		}
		if err := a.Connect(); err != nil {
			a.ShowWindow()
			a.emitError(err.Error())
			return
		}
		a.notice(i18n.T("已连接"), "success")
	case "systemProxy":
		s.SystemProxy = !s.SystemProxy
		if err := a.SaveSettings(s); err != nil {
			a.emitError(err.Error())
			return
		}
		a.notice(map[bool]string{true: i18n.T("系统代理已开启"), false: i18n.T("系统代理已关闭")}[s.SystemProxy], "info")
	case "tun":
		a.toggleTun()
	case "routeMode":
		next := map[string]string{"rule": "global", "global": "direct", "direct": "rule"}[s.RouteMode]
		if next == "" {
			next = "rule"
		}
		s.RouteMode = next
		if err := a.SaveSettings(s); err != nil {
			a.emitError(err.Error())
			return
		}
		a.notice(i18n.T("路由模式：")+routeModeName(next), "info")
	case "showWindow":
		a.ShowWindow()
	case "testGroup":
		if err := a.testGroup(a.store.CurrentGroupID()); err != nil {
			a.emitError(err.Error())
		}
	}
}

// toggleTun switches TUN mode, when NextBox may use it (tray, hotkey).
func (a *App) toggleTun() {
	s := a.store.Settings()
	if !s.TunEnabled && !a.GetState().Elevated && !a.serviceReady() {
		a.ShowWindow()
		a.emitError(i18n.T("TUN 模式需要管理员权限：请安装服务模式，或以管理员身份运行。"))
		return
	}
	s.TunEnabled = !s.TunEnabled
	if err := a.SaveSettings(s); err != nil {
		a.emitError(err.Error())
		return
	}
	a.notice(map[bool]string{true: i18n.T("TUN 模式已开启"), false: i18n.T("TUN 模式已关闭")}[s.TunEnabled], "info")
}

func routeModeName(m string) string {
	return map[string]string{"rule": i18n.T("规则分流"), "global": i18n.T("全局代理"), "direct": i18n.T("全部直连")}[m]
}
