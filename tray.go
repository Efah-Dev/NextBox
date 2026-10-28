package main

import (
	"context"
	"fmt"
	"nextbox/internal/i18n"
	goruntime "runtime"
	"sync"
	"time"

	"nextbox/internal/icon"
	"nextbox/internal/store"

	"fyne.io/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// trayState is the Windows notification-area icon and its right-click menu.
type trayState struct {
	mu     sync.Mutex
	ready  bool
	timer  *time.Timer
	cancel context.CancelFunc
	icons  map[string][]byte // keyed by proxy mode: "", "system", "tun"
}

const trayNodeLimit = 20 // nodes listed directly in the tray menu

func (a *App) startTray() {
	sizes := []int{16, 20, 24, 32, 48}
	a.tray.icons = map[string][]byte{
		"":       icon.ICO(sizes, nil),
		"system": icon.ICO(sizes, &icon.Green),
		"tun":    icon.ICO(sizes, &icon.Red),
	}
	go func() {
		// The tray window and its message loop must live on one OS thread.
		goruntime.LockOSThread()
		systray.Run(a.onTrayReady, a.onTrayExit)
	}()
}

func (a *App) stopTray() { systray.Quit() }

// onTrayExit runs when the tray goes away: on a normal quit (after shutdown
// already cleaned up) and when Windows logs off or shuts down, which Wails
// itself does not report. Restoring the system proxy here keeps browsers from
// pointing at a dead port after a reboot. It must not take opMu: shutdown
// holds it while calling stopTray.
func (a *App) onTrayExit() {
	a.applySystemProxy(store.Settings{}, false)
	if a.store != nil {
		a.store.Flush()
	}
	if w := a.fileLog.Load(); w != nil {
		w.Close()
	}
}

func (a *App) onTrayReady() {
	systray.SetOnTapped(a.ShowWindow) // left click opens the window, right click the menu
	a.tray.mu.Lock()
	a.tray.ready = true
	a.tray.mu.Unlock()
	a.rebuildTray()
}

// refreshTray schedules a menu rebuild; bursts of state changes are coalesced.
func (a *App) refreshTray() {
	a.tray.mu.Lock()
	defer a.tray.mu.Unlock()
	if !a.tray.ready {
		return
	}
	if a.tray.timer != nil {
		a.tray.timer.Stop()
	}
	a.tray.timer = time.AfterFunc(250*time.Millisecond, a.rebuildTray)
}

func (a *App) rebuildTray() {
	a.tray.mu.Lock()
	if a.tray.cancel != nil {
		a.tray.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.tray.cancel = cancel
	a.tray.mu.Unlock()

	st := a.GetState()
	selected, hasSelected := a.store.Profile(st.SelectedID)

	systray.SetIcon(a.tray.icons[st.ProxyMode])
	tip := i18n.T("NextBox · 未连接")
	if st.Running {
		mode := map[string]string{"system": i18n.T("系统代理"), "tun": i18n.T("TUN 模式"), "": i18n.T("仅端口")}[st.ProxyMode]
		tip = "NextBox · " + mode
		if hasSelected {
			tip += " · " + selected.Name
		}
	}
	systray.SetTooltip(tip)

	systray.ResetMenu()
	on := func(item *systray.MenuItem, fn func()) {
		go func() {
			for {
				select {
				case <-item.ClickedCh:
					fn()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	title := systray.AddMenuItem("NextBox "+Version, "")
	title.Disable()
	on(systray.AddMenuItem(i18n.T("显示主界面"), ""), a.ShowWindow)
	systray.AddSeparator()

	if st.Running {
		on(systray.AddMenuItem(i18n.T("关闭代理"), ""), func() { a.Disconnect() })
	} else {
		on(systray.AddMenuItem(i18n.T("启动代理"), ""), func() {
			if err := a.Connect(); err != nil {
				a.ShowWindow()
				a.emitError(err.Error())
			}
		})
	}

	tun := systray.AddMenuItemCheckbox(i18n.T("TUN 模式"), "", st.Settings.TunEnabled)
	on(tun, func() {
		s := a.store.Settings()
		if !s.TunEnabled && !st.Elevated {
			a.ShowWindow()
			a.emitError(i18n.T("TUN 模式需要管理员权限，请先“以管理员身份重启”"))
			return
		}
		s.TunEnabled = !s.TunEnabled
		if err := a.SaveSettings(s); err != nil {
			a.emitError(err.Error())
		}
	})

	// Route modes and nodes are listed in the main menu rather than in
	// submenus: the tray sits at the right edge of the screen, so Windows
	// would open any submenu towards the left.
	systray.AddSeparator()
	header(i18n.T("路由模式"))
	for _, m := range []struct{ value, label string }{{"rule", i18n.T("规则分流")}, {"global", i18n.T("全局代理")}, {"direct", i18n.T("全部直连")}} {
		value := m.value
		item := systray.AddMenuItemCheckbox(m.label, "", st.Settings.RouteMode == value)
		on(item, func() {
			s := a.store.Settings()
			s.RouteMode = value
			if err := a.SaveSettings(s); err != nil {
				a.emitError(err.Error())
			}
		})
	}

	systray.AddSeparator()
	profiles := a.store.Profiles(st.CurrentGroupID)
	groupName := i18n.T("节点")
	if g, ok := a.store.Group(st.CurrentGroupID); ok {
		name := g.Name
		if g.Default {
			name = i18n.T("默认分组")
		}
		groupName = i18n.T("节点 · ") + name
	}
	header(groupName)
	if len(profiles) == 0 {
		header(i18n.T("当前分组没有节点"))
	}
	for _, p := range trayProfiles(profiles, st.SelectedID) {
		label := p.Name
		if p.Latency > 0 {
			label += fmt.Sprintf("    %d ms", p.Latency)
		}
		id := p.ID
		on(systray.AddMenuItemCheckbox(label, "", id == st.SelectedID), func() {
			if err := a.SelectProfile(id); err != nil {
				a.emitError(err.Error())
			}
		})
	}
	if len(profiles) > trayNodeLimit {
		on(systray.AddMenuItem(fmt.Sprintf(i18n.T("更多节点…（共 %d 个）"), len(profiles)), ""), func() {
			a.ShowWindow()
			wruntime.EventsEmit(a.ctx, "navigate", "nodes")
		})
	}

	systray.AddSeparator()
	on(systray.AddMenuItem(i18n.T("退出"), ""), a.Quit)
}

// header adds a greyed-out label that titles the items below it.
func header(text string) {
	systray.AddMenuItem(text, "").Disable()
}

// trayProfiles picks the nodes shown in the tray: the first trayNodeLimit,
// always including the selected one.
func trayProfiles(list []store.Profile, selected int64) []store.Profile {
	if len(list) <= trayNodeLimit {
		return list
	}
	out := append([]store.Profile(nil), list[:trayNodeLimit]...)
	for _, p := range list[trayNodeLimit:] {
		if p.ID == selected {
			out[trayNodeLimit-1] = p
		}
	}
	return out
}
