package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"nextbox/internal/i18n"
	goruntime "runtime"
	"slices"
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
	popup  *trayPopup        // NextBox's own menu (traypopup_windows.go)
	liveAt time.Time         // the last render of a test's progress
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
	systray.SetOnTapped(a.ShowWindow)            // left click opens the window,
	systray.SetOnSecondaryTapped(a.openTrayMenu) // right click NextBox's menu
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
	a.tray.timer = time.AfterFunc(250*time.Millisecond, func() {
		a.rebuildTray()
		a.updateTrayMenu()
	})
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
	add(trayItem{label: i18n.T("TUN 模式"), check: true, checked: st.Settings.TunEnabled, fn: a.toggleTun})

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

// ---------------------------------------------------------------- NextBox's own menu

// Tray menu parts the user can hide (Settings.TrayItems; nil = all).
var trayParts = []string{"quick", "mode", "nodes", "policies", "latency"}

func (a *App) trayShows(part string, s store.Settings) bool {
	return s.TrayItems == nil || slices.Contains(s.TrayItems, part)
}

// trayView is what traymenu.html renders.
type trayView struct {
	Theme      string            `json:"theme"` // auto, light, dark
	Dark       bool              `json:"dark"`  // the theme in effect (auto follows Windows)
	Accent     string            `json:"accent"`
	Parts      map[string]bool   `json:"parts"`
	Running    bool              `json:"running"`
	Testing    bool              `json:"testing"` // a latency test runs
	Since      int64             `json:"since"`   // ms
	Mode       string            `json:"mode"`
	SysProxy   bool              `json:"sysProxy"`
	Tun        bool              `json:"tun"`
	Current    string            `json:"current"` // the node, or the policy group in use
	Via        string            `json:"via"`     // its member, when a policy group is used
	Latency    int               `json:"latency"` // of the node in use
	Version    string            `json:"version"`
	Groups     []trayGroup       `json:"groups"`
	Policies   []trayPolicy      `json:"policies"`
	Labels     map[string]string `json:"l"`
	GroupID    int64             `json:"groupId"`    // the group the node in use is in
	PolicyID   int64             `json:"policyId"`   // the policy group in use
	SelectedID int64             `json:"selectedId"` // the node in use
}

type trayGroup struct {
	ID    int64      `json:"id"`
	Name  string     `json:"name"`
	Count int        `json:"count"`
	Nodes []trayNode `json:"nodes"`
}

type trayNode struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Latency int    `json:"latency"`
	Country string `json:"country,omitempty"` // ISO code, for the flag
}

type trayPolicy struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Auto bool   `json:"auto"`
	Now  string `json:"now"`
}

var trayLabels = []string{
	"未连接", "已连接", "系统代理", "设为系统代理", "连接后让浏览器等遵循 Windows 代理设置的程序经过 NextBox", "接管所有程序的流量", "TUN 模式", "全局", "规则", "直连", "节点", "分组代理", "退出",
	"返回", "搜索节点", "没有节点", "自动选最快", "手动选择", "当前", "没有找到", "经", "测延迟", "停止",
}

func (a *App) trayState() trayView {
	st := a.GetState()
	s := st.Settings
	v := trayView{
		Theme: s.Theme, Dark: s.Theme == "dark" || (s.Theme != "light" && sys.SystemDark()), Accent: s.AccentColor, Running: st.Running, Testing: st.Testing, Mode: s.RouteMode,
		SysProxy: s.SystemProxy, Tun: s.TunEnabled, Version: Version, Parts: map[string]bool{},
		Labels: map[string]string{}, SelectedID: st.SelectedID, PolicyID: a.store.SelectedPolicy(),
	}
	if st.Running {
		v.Since = st.StartedAt
	}
	for _, p := range trayParts {
		v.Parts[p] = a.trayShows(p, s)
	}
	for _, l := range trayLabels {
		v.Labels[l] = i18n.T(l)
	}
	if p, ok := a.store.Profile(st.SelectedID); ok {
		v.Current, v.Latency, v.GroupID = p.Name, p.Latency, p.GroupID
	}
	if pol, ok := a.store.Policy(v.PolicyID); ok {
		v.Current = pol.Name
		v.Via = a.policyNowName(pol.ID)
	}
	for _, g := range a.store.Groups() {
		tg := trayGroup{ID: g.ID, Name: g.Name}
		if g.Default {
			tg.Name = i18n.T("默认分组")
		}
		for _, p := range a.store.Profiles(g.ID) {
			tg.Nodes = append(tg.Nodes, trayNode{ID: p.ID, Name: p.Name, Latency: p.Latency, Country: nodeCountryCode(p, s.CountrySource)})
		}
		tg.Count = len(tg.Nodes)
		if tg.Count > 0 || g.ID == st.CurrentGroupID {
			v.Groups = append(v.Groups, tg)
		}
	}
	for _, p := range a.store.Policies() {
		v.Policies = append(v.Policies, trayPolicy{ID: p.ID, Name: p.Name, Auto: p.Mode == store.PolicyAuto, Now: a.policyNowName(p.ID)})
	}
	return v
}

// openTrayMenu is the right click on the icon: NextBox's menu, or the plain
// Windows one when the menu window cannot be shown.
// It runs on the tray's thread, which must not wait: the menu window is
// set up (the first time) and shown from another goroutine.
func (a *App) openTrayMenu() {
	if a.tray.popup == nil {
		a.tray.popup = &trayPopup{onMessage: a.trayAction, files: a.ui, log: a.log}
	}
	p := a.tray.popup
	if p.failed {
		systray.ShowMenu()
		return
	}
	go func() {
		b, _ := json.Marshal(a.trayState())
		if err := p.Show(string(b)); err != nil {
			// from now on a right click opens the plain Windows menu
			a.log(i18n.T("无法显示托盘菜单窗口，改用系统菜单"))
			systray.SetOnSecondaryTapped(nil)
		}
	}()
}

// trayLive shows a latency test's results in the open menu as they come,
// a few times a second at most.
func (a *App) trayLive() {
	p := a.tray.popup
	if p == nil || !p.open.Load() {
		return
	}
	a.tray.mu.Lock()
	if time.Since(a.tray.liveAt) < 300*time.Millisecond {
		a.tray.mu.Unlock()
		return
	}
	a.tray.liveAt = time.Now()
	a.tray.mu.Unlock()
	a.updateTrayMenu()
}

// updateTrayMenu renders the state again while the menu is open.
func (a *App) updateTrayMenu() {
	if p := a.tray.popup; p != nil {
		b, _ := json.Marshal(a.trayState())
		p.Update(string(b))
	}
}

// trayAction runs what was picked in the menu.
func (a *App) trayAction(msg string) {
	var m struct {
		A  string `json:"a"`
		ID int64  `json:"id"`
		V  string `json:"v"`
	}
	if json.Unmarshal([]byte(msg), &m) != nil {
		return
	}
	s := a.store.Settings()
	switch m.A {
	case "toggle":
		if a.isRunning() {
			a.Disconnect()
		} else if err := a.Connect(); err != nil {
			a.ShowWindow()
			a.emitError(err.Error())
		}
	case "sysProxy":
		s.SystemProxy = !s.SystemProxy
		if err := a.SaveSettings(s); err != nil {
			a.emitError(err.Error())
		}
	case "tun":
		a.toggleTun()
	case "mode":
		s.RouteMode = m.V
		if err := a.SaveSettings(s); err != nil {
			a.emitError(err.Error())
		}
	case "node": // the menu stays open: the new pick shows in it
		if err := a.SelectProfile(m.ID); err != nil {
			a.emitError(err.Error())
		}
	case "policy":
		if err := a.UsePolicy(m.ID); err != nil {
			a.emitError(err.Error())
		}
	case "testGroup": // the button of a subscription: test it, or stop the test
		if a.GetState().Testing {
			a.StopTest()
		} else if err := a.testGroup(m.ID); err != nil {
			a.emitError(err.Error())
		}
	case "quit":
		a.tray.popup.Hide()
		a.Quit()
	}
	a.updateTrayMenu()
}
