package app

import (
	"context"
	"fmt"
	"log"
	"nextbox/internal/i18n"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"nextbox/internal/icon"
	"nextbox/internal/store"
	"nextbox/internal/sys"

	"fyne.io/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// trayState is the Windows notification-area icon and its right-click menu.
type trayState struct {
	mu     sync.Mutex
	build  sync.Mutex // one rebuild at a time: two at once mix up the items
	ready  bool
	timer  *time.Timer
	cancel context.CancelFunc
	sig    string            // what the menu shows, see rebuildTray
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
	// the tray library reports its failures through the standard logger
	log.SetFlags(0)
	log.SetOutput(logLine(func(s string) { a.log(strings.TrimSpace(s)) }))
	go func() {
		// Right after logon (a start by Windows or by the NextBox service)
		// Explorer runs before its taskbar answers. Adding the icon then can
		// time out: Windows still shows an empty icon, but the tray library
		// takes the time-out for a failure and never sets the picture, the
		// menu or the click handlers, which left a blank icon that did
		// nothing. So the icon is added once the taskbar answers.
		if !sys.TaskbarReady() {
			a.log(i18n.T("等待任务栏就绪后再显示托盘图标"))
			if !sys.WaitTaskbar(3 * time.Minute) {
				a.log(i18n.T("任务栏 3 分钟内没有就绪，仍然尝试显示托盘图标"))
			}
			time.Sleep(2 * time.Second) // the notification area comes a moment after the bar
		}
		// if the icon still could not be added, the window is the only way in
		time.AfterFunc(30*time.Second, func() {
			a.tray.mu.Lock()
			ready := a.tray.ready
			a.tray.mu.Unlock()
			if !ready {
				a.log(i18n.T("托盘图标创建失败，已显示主窗口"))
				a.ShowWindow()
			}
		})
		// The tray window and its message loop must live on one OS thread.
		goruntime.LockOSThread()
		systray.Run(a.onTrayReady, a.onTrayExit)
	}()
}

// logLine is an io.Writer that hands each write to fn.
type logLine func(string)

func (f logLine) Write(p []byte) (int, error) {
	f(string(p))
	return len(p), nil
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
	a.tray.build.Lock()
	defer a.tray.build.Unlock()
	st := a.GetState()
	selected, hasSelected := a.store.Profile(st.SelectedID)

	tip := i18n.T("NextBox · 未连接")
	if st.Running {
		mode := map[string]string{"system": i18n.T("系统代理"), "tun": i18n.T("TUN 模式"), "": i18n.T("仅端口")}[st.ProxyMode]
		tip = "NextBox · " + mode
		if hasSelected {
			tip += " · " + selected.Name
		}
	}

	var items []trayItem
	add := func(it trayItem) { items = append(items, it) }
	sep := func() { add(trayItem{sep: true}) }
	header := func(text string) { add(trayItem{label: text, disabled: true}) }

	header("NextBox " + Version)
	add(trayItem{label: i18n.T("显示主界面"), fn: a.ShowWindow})
	sep()
	if st.Running {
		add(trayItem{label: i18n.T("关闭代理"), fn: func() { a.Disconnect() }})
	} else {
		add(trayItem{label: i18n.T("启动代理"), fn: func() {
			if err := a.Connect(); err != nil {
				a.ShowWindow()
				a.emitError(err.Error())
			}
		}})
	}
	add(trayItem{label: i18n.T("TUN 模式"), check: true, checked: st.Settings.TunEnabled, fn: func() {
		s := a.store.Settings()
		if !s.TunEnabled && !st.Elevated && !a.serviceReady() {
			a.ShowWindow()
			a.emitError(i18n.T("TUN 模式需要管理员权限，安装服务模式或以管理员运行。"))
			return
		}
		s.TunEnabled = !s.TunEnabled
		if err := a.SaveSettings(s); err != nil {
			a.emitError(err.Error())
		}
	}})

	// Route modes and nodes are listed in the main menu rather than in
	// submenus: the tray sits at the right edge of the screen, so Windows
	// would open any submenu towards the left.
	sep()
	header(i18n.T("路由模式"))
	for _, m := range []struct{ value, label string }{{"rule", i18n.T("规则分流")}, {"global", i18n.T("全局代理")}, {"direct", i18n.T("全部直连")}} {
		value := m.value
		add(trayItem{label: m.label, check: true, checked: st.Settings.RouteMode == value, fn: func() {
			s := a.store.Settings()
			s.RouteMode = value
			if err := a.SaveSettings(s); err != nil {
				a.emitError(err.Error())
			}
		}})
	}

	sep()
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
		add(trayItem{label: label, check: true, checked: id == st.SelectedID, fn: func() {
			if err := a.SelectProfile(id); err != nil {
				a.emitError(err.Error())
			}
		}})
	}
	if len(profiles) > trayNodeLimit {
		add(trayItem{label: fmt.Sprintf(i18n.T("更多节点…（共 %d 个）"), len(profiles)), fn: func() {
			a.ShowWindow()
			wruntime.EventsEmit(a.ctx, "navigate", "nodes")
		}})
	}
	sep()
	add(trayItem{label: i18n.T("退出"), fn: a.Quit})

	// nothing that is shown changed (most state updates are about something
	// else): leave the menu alone, it may be open
	sig := st.ProxyMode + "\x00" + tip
	for _, it := range items {
		sig += fmt.Sprintf("\x00%s|%t|%t|%t|%t", it.label, it.sep, it.check, it.checked, it.disabled)
	}
	a.tray.mu.Lock()
	if sig == a.tray.sig {
		a.tray.mu.Unlock()
		return
	}
	a.tray.sig = sig
	if a.tray.cancel != nil {
		a.tray.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.tray.cancel = cancel
	a.tray.mu.Unlock()

	systray.SetIcon(a.tray.icons[st.ProxyMode])
	systray.SetTooltip(tip)
	systray.ResetMenu()
	for _, it := range items {
		switch {
		case it.sep:
			systray.AddSeparator()
			continue
		case it.check:
			trayClick(ctx, systray.AddMenuItemCheckbox(it.label, "", it.checked), it.fn)
		default:
			m := systray.AddMenuItem(it.label, "")
			if it.disabled {
				m.Disable()
			} else {
				trayClick(ctx, m, it.fn)
			}
		}
	}
}

// trayItem is one line of the tray menu.
type trayItem struct {
	label          string
	sep            bool // a separator line
	check, checked bool // a check box, and its state
	disabled       bool // a greyed-out title
	fn             func()
}

// trayClick runs fn whenever item is clicked, until ctx (the menu) ends.
func trayClick(ctx context.Context, item *systray.MenuItem, fn func()) {
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

// trayReady reports whether the tray icon exists, so that the window can be
// hidden to it.
func (a *App) trayReady() bool {
	a.tray.mu.Lock()
	defer a.tray.mu.Unlock()
	return a.tray.ready
}
