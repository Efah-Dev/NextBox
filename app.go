package main

import (
	"context"
	"errors"
	"fmt"
	"nextbox/internal/i18n"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/core"
	"nextbox/internal/link"
	"nextbox/internal/logfile"
	"nextbox/internal/store"
	"nextbox/internal/sub"
	"nextbox/internal/sys"

	"github.com/wailsapp/wails/v2/pkg/options"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound to the frontend: every exported method is callable from JS.
type App struct {
	ctx     context.Context
	baseDir string
	store   *store.Store
	logs    *logBuffer

	opMu      sync.Mutex // serialises connect / disconnect / restart
	ruleSetMu sync.Mutex // serialises writes to the rule-set directory

	mu           sync.Mutex // guards the fields below
	proc         *core.Process
	runningID    int64
	startedAt    time.Time
	sysProxyAddr string
	stopTraffic  context.CancelFunc
	testCancel   context.CancelFunc
	tunActive    bool
	fileLog      atomic.Pointer[logfile.Writer] // nil while file logging is off
	quitting     bool                           // set by Quit so the close handler lets the window go

	tray trayState
}

func NewApp() *App {
	exe, err := os.Executable()
	base := "."
	if err == nil {
		base = filepath.Dir(exe)
	}
	return &App{baseDir: base, logs: newLogBuffer(2000)}
}

func (a *App) dataDir() string    { return filepath.Join(a.baseDir, "config") }
func (a *App) runDir() string     { return filepath.Join(a.dataDir(), "run") }
func (a *App) testDir() string    { return filepath.Join(a.dataDir(), "test") }
func (a *App) ruleSetDir() string { return filepath.Join(a.dataDir(), "rule-set") }
func (a *App) logDir() string     { return filepath.Join(a.dataDir(), "logs") }
func (a *App) corePath() string   { return filepath.Join(a.baseDir, "core", sys.CoreBinaryName()) }

// ---------------------------------------------------------------- lifecycle

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	st, err := store.Open(a.dataDir())
	if err != nil {
		wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{
			Type: wruntime.ErrorDialog, Title: "NextBox", Message: i18n.T("读取配置失败: ") + err.Error(),
		})
		st, _ = store.Open(filepath.Join(os.TempDir(), "NextBox"))
	}
	a.store = st
	s := st.Settings()
	i18n.SetLang(s.Language)
	a.logs.sink = func(line string) {
		if w := a.fileLog.Load(); w != nil {
			w.WriteLine(line)
		}
	}
	// also prunes files older than the retention period
	a.applyLogFile(store.Settings{}, s)
	a.startTray()
	a.logs.onFlush = func(b LogBatch) { wruntime.EventsEmit(a.ctx, "logs", b) }

	// A crash may have left the system proxy pointing at us.
	sys.ClearSystemProxy(proxyAddr(s))

	a.log("NextBox " + Version + i18n.T(" 已启动"))
	if _, err := os.Stat(a.corePath()); err != nil {
		a.log(i18n.T("未找到内核: ") + a.corePath())
	}
	// Get the rule-sets ready in the background; a failure (no network, a
	// timeout) is only logged, connecting simply skips the affected rules.
	go a.ensureRuleSets()
	if s.ConnectOnStart && st.SelectedID() != 0 {
		go func() {
			if err := a.Connect(); err != nil {
				a.log(i18n.T("自动连接失败: ") + err.Error())
			}
		}()
	}
}

// beforeClose hides the window to the tray instead of quitting, unless the
// user turned that off or asked to quit explicitly.
func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	quitting := a.quitting
	a.mu.Unlock()
	if !quitting && a.store != nil && a.store.Settings().CloseToTray {
		wruntime.WindowHide(a.ctx)
		return true
	}
	return false
}

func (a *App) shutdown(ctx context.Context) {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	if a.testCancel != nil {
		a.testCancel()
	}
	a.mu.Unlock()
	a.stopLocked()
	if a.store != nil {
		a.store.Flush()
	}
	a.stopTray()
	if w := a.fileLog.Load(); w != nil {
		w.Close()
	}
}

func (a *App) onSecondInstance(data options.SecondInstanceData) { a.ShowWindow() }

// proxyModeLocked reports how traffic is captured: "tun", "system" or "".
func (a *App) proxyModeLocked() string {
	switch {
	case a.proc == nil:
		return ""
	case a.tunActive:
		return "tun"
	case a.sysProxyAddr != "":
		return "system"
	}
	return ""
}

func (a *App) log(line string) {
	a.logs.Append(time.Now().Format("15:04:05") + " [GUI] " + line)
}

// ---------------------------------------------------------------- state

type StateInfo struct {
	Version        string         `json:"version"`
	Running        bool           `json:"running"`
	RunningID      int64          `json:"runningId"`
	SelectedID     int64          `json:"selectedId"`
	CurrentGroupID int64          `json:"currentGroupId"`
	StartedAt      int64          `json:"startedAt"`
	Testing        bool           `json:"testing"`
	ProxyMode      string         `json:"proxyMode"` // "", "system" or "tun"
	Elevated       bool           `json:"elevated"`
	CoreFound      bool           `json:"coreFound"`
	CorePath       string         `json:"corePath"`
	DataDir        string         `json:"dataDir"`
	Settings       store.Settings `json:"settings"`
	Groups         []store.Group  `json:"groups"`
}

func (a *App) GetState() StateInfo {
	a.mu.Lock()
	running := a.proc != nil
	info := StateInfo{
		Running:   running,
		RunningID: a.runningID,
		Testing:   a.testCancel != nil,
	}
	if running {
		info.StartedAt = a.startedAt.UnixMilli()
		info.ProxyMode = a.proxyModeLocked()
	}
	a.mu.Unlock()
	_, err := os.Stat(a.corePath())
	info.Version = Version
	info.SelectedID = a.store.SelectedID()
	info.CurrentGroupID = a.store.CurrentGroupID()
	info.Elevated = sys.IsElevated()
	info.CoreFound = err == nil
	info.CorePath = a.corePath()
	info.DataDir = a.dataDir()
	info.Settings = a.store.Settings()
	info.Groups = a.store.Groups()
	return info
}

func (a *App) emitState() {
	wruntime.EventsEmit(a.ctx, "state", a.GetState())
	a.refreshTray()
}

func (a *App) emitError(msg string) { wruntime.EventsEmit(a.ctx, "error", msg) }

func (a *App) emitProfiles(groupID int64) {
	wruntime.EventsEmit(a.ctx, "profiles", groupID)
	a.refreshTray()
}

// ---------------------------------------------------------------- core control

func (a *App) Connect() error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	err := a.startLocked()
	a.emitState()
	return err
}

func (a *App) Disconnect() error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.stopLocked()
	a.emitState()
	return nil
}

func (a *App) isRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.proc != nil
}

// restartIfRunning applies new settings / node to a running core.
func (a *App) restartIfRunning() error {
	if !a.isRunning() {
		return nil
	}
	return a.Connect()
}

func (a *App) startLocked() error {
	a.stopLocked()
	s := a.store.Settings()
	if s.TunEnabled && !sys.IsElevated() {
		return errors.New(i18n.T("TUN 模式需要管理员权限，请先“以管理员身份重启”"))
	}
	var node *store.Profile
	if id := a.store.SelectedID(); id != 0 {
		if p, ok := a.store.Profile(id); ok {
			node = &p
		}
	}
	port, err := core.FreePort()
	if err != nil {
		return err
	}
	api := core.NewAPI("127.0.0.1:"+strconv.Itoa(port), core.RandomSecret())
	if missing := a.ensureRuleSets(); len(missing) > 0 {
		a.log(i18n.T("规则集不可用，已暂时跳过相关规则: ") + strings.Join(missing, ", "))
	}
	in := config.Input{
		Settings: s, Node: node, ClashAPI: api.Addr, Secret: api.Secret,
		RuleSetDir: a.ruleSetDir(), SkipMissingRuleSets: true,
	}
	cfg, err := config.Build(in)
	if err != nil {
		return err
	}

	if node != nil {
		a.log(i18n.T("正在连接: ") + node.Name)
	} else {
		a.log(i18n.T("正在启动直连模式"))
	}
	proc, err := core.Start(a.ctx, a.corePath(), a.runDir(), "config.json", cfg, api, a.logs.Append)
	if err != nil {
		a.log(err.Error())
		return err
	}

	var nodeID int64
	if node != nil {
		nodeID = node.ID
	}
	a.mu.Lock()
	a.proc = proc
	a.runningID = nodeID
	a.tunActive = s.TunEnabled
	a.startedAt = time.Now()
	trafficCtx, cancel := context.WithCancel(a.ctx)
	a.stopTraffic = cancel
	a.mu.Unlock()

	if s.SystemProxy && !s.TunEnabled {
		if err := a.applySystemProxy(s, true); err != nil {
			a.log(i18n.T("设置系统代理失败: ") + err.Error())
		}
	}
	go a.trafficLoop(trafficCtx, proc, nodeID)
	go a.watch(proc)
	a.log(i18n.T("内核已启动，混合代理端口 ") + strconv.Itoa(s.MixedPort))
	return nil
}

func (a *App) stopLocked() {
	a.mu.Lock()
	proc := a.proc
	a.proc = nil
	a.runningID = 0
	if a.stopTraffic != nil {
		a.stopTraffic()
		a.stopTraffic = nil
	}
	a.mu.Unlock()
	if proc == nil {
		return
	}
	a.applySystemProxy(store.Settings{}, false)
	proc.Stop()
	a.store.Flush()
	a.log(i18n.T("内核已停止"))
}

// watch reports an unexpected core exit.
func (a *App) watch(proc *core.Process) {
	<-proc.Done()
	a.mu.Lock()
	unexpected := a.proc == proc
	if unexpected {
		a.proc = nil
		a.runningID = 0
		if a.stopTraffic != nil {
			a.stopTraffic()
			a.stopTraffic = nil
		}
	}
	a.mu.Unlock()
	if !unexpected {
		return
	}
	a.applySystemProxy(store.Settings{}, false)
	msg := i18n.T("内核意外退出")
	if err := proc.ExitErr(); err != nil {
		msg += ": " + err.Error()
	}
	a.log(msg)
	a.emitState()
	wruntime.EventsEmit(a.ctx, "error", msg)
}

func (a *App) trafficLoop(ctx context.Context, proc *core.Process, nodeID int64) {
	for ctx.Err() == nil {
		proc.API.StreamTraffic(ctx, func(up, down int64) {
			wruntime.EventsEmit(a.ctx, "traffic", map[string]int64{"up": up, "down": down})
			if nodeID != 0 && up+down > 0 {
				a.store.AddTraffic(nodeID, up, down)
			}
		})
		select {
		case <-ctx.Done():
		case <-proc.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func proxyAddr(s store.Settings) string {
	host := s.ListenAddress
	if host == "" || host == "0.0.0.0" || host == "::" || s.AllowLAN {
		host = "127.0.0.1"
	}
	return host + ":" + strconv.Itoa(s.MixedPort)
}

func (a *App) applySystemProxy(s store.Settings, enable bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !enable {
		if a.sysProxyAddr == "" {
			return nil
		}
		err := sys.ClearSystemProxy(a.sysProxyAddr)
		a.sysProxyAddr = ""
		return err
	}
	addr := proxyAddr(s)
	if err := sys.SetSystemProxy(addr); err != nil {
		return err
	}
	a.sysProxyAddr = addr
	return nil
}

// ---------------------------------------------------------------- settings

func (a *App) SaveSettings(s store.Settings) error {
	old := a.store.Settings()
	if err := a.store.SaveSettings(s); err != nil {
		return err
	}
	s = a.store.Settings()
	if old.AutoStart != s.AutoStart {
		if err := sys.SetAutoStart("NextBox", s.AutoStart); err != nil {
			a.log(i18n.T("设置开机自启失败: ") + err.Error())
		}
	}
	i18n.SetLang(s.Language)
	a.applyLogFile(old, s)
	defer a.emitState()
	if !a.isRunning() {
		return nil
	}
	if coreSettings(old) != coreSettings(s) {
		return a.restartIfRunning()
	}
	if old.SystemProxy != s.SystemProxy {
		return a.applySystemProxy(s, s.SystemProxy && !s.TunEnabled)
	}
	return nil
}

// coreSettings keeps only the fields that end up in the sing-box config.
func coreSettings(s store.Settings) store.Settings {
	s.Theme, s.AccentColor, s.Language = "", "", ""
	s.TestURL, s.TestTimeout, s.TestConcurrency = "", 0, 0
	s.SubUserAgent, s.SubUpdateByProxy = "", false
	s.ConnectOnStart, s.AutoStart, s.CloseToTray, s.SystemProxy = false, false, false, false
	s.LogToFile, s.LogFileMaxMB, s.LogKeepDays, s.LogViewLimit = false, 0, 0, 0
	return s
}

// ---------------------------------------------------------------- groups

func (a *App) SetCurrentGroup(id int64) error {
	err := a.store.SetCurrentGroup(id)
	a.emitState()
	return err
}

func (a *App) AddGroup(name string) (store.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Group{}, errors.New(i18n.T("请输入分组名称"))
	}
	g, err := a.store.AddGroup(store.Group{Name: name})
	if err == nil {
		a.store.SetCurrentGroup(g.ID)
	}
	a.emitState()
	return g, err
}

func (a *App) AddSubscription(name, url string) (string, error) {
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", errors.New(i18n.T("订阅地址必须以 http:// 或 https:// 开头"))
	}
	if strings.TrimSpace(name) == "" {
		name = i18n.T("新订阅")
	}
	g, err := a.store.AddGroup(store.Group{Name: strings.TrimSpace(name), URL: url})
	if err != nil {
		return "", err
	}
	a.store.SetCurrentGroup(g.ID)
	a.emitState()
	return a.UpdateSubscription(g.ID)
}

func (a *App) EditGroup(g store.Group) error {
	old, ok := a.store.Group(g.ID)
	if !ok {
		return errors.New(i18n.T("分组不存在"))
	}
	old.Name = strings.TrimSpace(g.Name)
	old.URL = strings.TrimSpace(g.URL)
	old.UserAgent = strings.TrimSpace(g.UserAgent)
	err := a.store.UpdateGroup(old)
	a.emitState()
	return err
}

func (a *App) DeleteGroup(id int64) error {
	err := a.store.DeleteGroup(id)
	a.emitState()
	return err
}

func (a *App) UpdateSubscription(groupID int64) (string, error) {
	g, ok := a.store.Group(groupID)
	if !ok {
		return "", errors.New(i18n.T("分组不存在"))
	}
	if g.URL == "" {
		return "", errors.New(i18n.T("这个分组不是订阅"))
	}
	s := a.store.Settings()
	ua := g.UserAgent
	if ua == "" {
		ua = s.SubUserAgent
	}
	proxy := ""
	if s.SubUpdateByProxy && a.isRunning() {
		proxy = "http://" + proxyAddr(s)
	}
	a.log(i18n.T("正在更新订阅: ") + g.Name)
	res, err := sub.Fetch(a.ctx, g.URL, ua, proxy)
	if err != nil {
		a.log(err.Error())
		return "", err
	}
	profiles := make([]store.Profile, 0, len(res.Nodes))
	for _, n := range res.Nodes {
		profiles = append(profiles, store.Profile{Name: n.Name, Type: n.Type(), Outbound: n.Outbound})
	}
	if err := a.store.ReplaceGroupProfiles(groupID, profiles); err != nil {
		return "", err
	}
	g, _ = a.store.Group(groupID)
	g.UpdatedAt = time.Now().Unix()
	if res.Usage != nil {
		g.Usage = res.Usage
	}
	a.store.UpdateGroup(g)
	msg := fmt.Sprintf(i18n.T("“%s”更新完成：%d 个节点"), g.Name, len(profiles))
	msg += a.skipNote(res.Skipped)
	a.log(msg)
	a.emitState()
	a.emitProfiles(groupID)
	return msg, nil
}

func (a *App) UpdateAllSubscriptions() (string, error) {
	var ok, failed int
	for _, g := range a.store.Groups() {
		if g.URL == "" {
			continue
		}
		if _, err := a.UpdateSubscription(g.ID); err != nil {
			failed++
		} else {
			ok++
		}
	}
	if ok+failed == 0 {
		return "", errors.New(i18n.T("还没有添加订阅"))
	}
	return fmt.Sprintf(i18n.T("已更新 %d 个订阅，失败 %d 个"), ok, failed), nil
}

// ---------------------------------------------------------------- profiles

func (a *App) GetProfiles(groupID int64) []store.Profile {
	return a.store.Profiles(groupID)
}

func (a *App) GetProfile(id int64) (store.Profile, error) {
	p, ok := a.store.Profile(id)
	if !ok {
		return p, errors.New(i18n.T("节点不存在"))
	}
	return p, nil
}

// ImportText imports share links / subscription content into a group.
func (a *App) ImportText(text string, groupID int64) (string, error) {
	nodes, skipped, err := link.ParseContent(text)
	if err != nil {
		a.skipNote(skipped)
		return "", err
	}
	list := make([]store.Profile, 0, len(nodes))
	for _, n := range nodes {
		list = append(list, store.Profile{Name: n.Name, Type: n.Type(), Outbound: n.Outbound})
	}
	if _, err := a.store.AddProfiles(groupID, list); err != nil {
		return "", err
	}
	a.emitProfiles(groupID)
	msg := fmt.Sprintf(i18n.T("已导入 %d 个节点"), len(list))
	msg += a.skipNote(skipped)
	return msg, nil
}

func (a *App) SaveProfile(p store.Profile) (store.Profile, error) {
	typ, _ := p.Outbound["type"].(string)
	if typ == "" {
		return p, errors.New(i18n.T("出站配置缺少 type 字段"))
	}
	delete(p.Outbound, "tag")
	p.Type = typ
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = typ
	}
	if p.ID != 0 {
		if old, ok := a.store.Profile(p.ID); ok {
			p.Latency, p.LatencyError, p.TestedAt = old.Latency, old.LatencyError, old.TestedAt
			p.Upload, p.Download = old.Upload, old.Download
		}
	}
	saved, err := a.store.SaveProfile(p)
	if err != nil {
		return saved, err
	}
	a.emitProfiles(saved.GroupID)
	a.mu.Lock()
	isRunningNode := a.runningID == saved.ID && a.proc != nil
	a.mu.Unlock()
	if isRunningNode {
		go a.restartIfRunning()
	}
	return saved, nil
}

func (a *App) DeleteProfiles(ids []int64) error {
	groups := map[int64]bool{}
	for _, p := range a.store.ProfilesByIDs(ids) {
		groups[p.GroupID] = true
	}
	err := a.store.DeleteProfiles(ids)
	for g := range groups {
		a.emitProfiles(g)
	}
	a.emitState()
	return err
}

// DeleteUnavailable removes nodes whose last latency test failed.
func (a *App) DeleteUnavailable(groupID int64) (int, error) {
	var ids []int64
	for _, p := range a.store.Profiles(groupID) {
		if p.Latency < 0 {
			ids = append(ids, p.ID)
		}
	}
	return len(ids), a.DeleteProfiles(ids)
}

func (a *App) SortGroup(groupID int64, by string) error {
	err := a.store.SortGroup(groupID, by)
	a.emitProfiles(groupID)
	return err
}

func (a *App) ExportLinks(ids []int64) (string, error) {
	var lines []string
	for _, p := range a.store.ProfilesByIDs(ids) {
		l, err := link.Export(p.Name, p.Outbound)
		if err != nil {
			return "", fmt.Errorf("%s: %w", p.Name, err)
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n"), nil
}

func (a *App) SelectProfile(id int64) error {
	if err := a.store.SetSelected(id); err != nil {
		return err
	}
	a.emitState()
	return a.restartIfRunning()
}

// ---------------------------------------------------------------- latency

// TestLatency tests nodes in the background; results arrive as "latency"
// events, followed by "latency-done". mode is "url" or "tcp".
func (a *App) TestLatency(ids []int64, mode string) error {
	nodes := a.store.ProfilesByIDs(ids)
	if len(nodes) == 0 {
		return errors.New(i18n.T("没有要测试的节点"))
	}
	a.mu.Lock()
	if a.testCancel != nil {
		a.mu.Unlock()
		return errors.New(i18n.T("正在测速，请稍候"))
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.testCancel = cancel
	a.mu.Unlock()
	a.emitState()

	s := a.store.Settings()
	go func() {
		defer func() {
			cancel()
			a.mu.Lock()
			a.testCancel = nil
			a.mu.Unlock()
			a.store.Flush()
			wruntime.EventsEmit(a.ctx, "latency-done")
			a.emitState()
		}()
		onResult := func(r core.LatencyResult) {
			a.store.SetLatency(r.ID, r.Latency, r.Error, time.Now().Unix())
			wruntime.EventsEmit(a.ctx, "latency", r)
		}
		if mode == "tcp" {
			core.TCPPing(ctx, s, nodes, onResult)
		} else {
			core.TestURL(ctx, a.corePath(), a.testDir(), s, nodes, onResult)
		}
	}()
	return nil
}

func (a *App) StopTest() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.testCancel != nil {
		a.testCancel()
	}
}

// ---------------------------------------------------------------- connections & logs

func (a *App) currentAPI() (*core.API, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proc == nil {
		return nil, errors.New(i18n.T("未连接"))
	}
	return a.proc.API, nil
}

func (a *App) GetConnections() (*core.Connections, error) {
	api, err := a.currentAPI()
	if err != nil {
		return &core.Connections{Connections: []core.Connection{}}, nil
	}
	return api.Connections(a.ctx)
}

// CloseConnection closes one connection, or all of them when id is empty.
func (a *App) CloseConnection(id string) error {
	api, err := a.currentAPI()
	if err != nil {
		return err
	}
	return api.CloseConnection(a.ctx, id)
}

func (a *App) GetLogs() LogBatch { return a.logs.Snapshot() }

func (a *App) ClearLogs() { a.logs.Clear() }

// ---------------------------------------------------------------- rule-sets

func (a *App) UpdateRuleSets() (string, error) {
	s := a.store.Settings()
	names := config.RequiredRuleSets(s)
	if len(names) == 0 {
		return i18n.T("当前设置不需要规则集"), nil
	}
	proxy := ""
	if a.isRunning() {
		proxy = "http://" + proxyAddr(s)
	}
	a.ruleSetMu.Lock()
	err := a.fetchRuleSetsLocked(names, proxy)
	a.ruleSetMu.Unlock()
	if err != nil {
		return "", err
	}
	go a.restartIfRunning()
	return fmt.Sprintf(i18n.T("已更新 %d 个规则集"), len(names)), nil
}

// ---------------------------------------------------------------- misc

// RestartAsAdmin starts an elevated instance and exits this one, like
// nekoray. It must really quit: with close-to-tray on, a plain Quit would only
// hide the window, and the new instance would hand over to this one.
func (a *App) RestartAsAdmin() error {
	if err := sys.RestartAsAdmin(); err != nil {
		return err
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		a.Quit()
	}()
	return nil
}

func (a *App) OpenDataDir() error {
	dir := a.dataDir()
	if goruntime.GOOS == "windows" {
		return exec.Command("explorer", dir).Start()
	}
	return exec.Command("xdg-open", dir).Start()
}

func (a *App) OpenCoreDir() error {
	dir := filepath.Dir(a.corePath())
	os.MkdirAll(dir, 0o755)
	if goruntime.GOOS == "windows" {
		return exec.Command("explorer", dir).Start()
	}
	return exec.Command("xdg-open", dir).Start()
}

func (a *App) OpenURL(url string) { wruntime.BrowserOpenURL(a.ctx, url) }

// Quit exits the application (tray menu, or the close button when
// close-to-tray is off).
func (a *App) Quit() {
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
	wruntime.Quit(a.ctx)
}

// CloseWindow is the title bar close button.
func (a *App) CloseWindow() {
	if a.store.Settings().CloseToTray {
		wruntime.WindowHide(a.ctx)
		return
	}
	a.Quit()
}

// ShowWindow brings the window back from the tray.
func (a *App) ShowWindow() {
	wruntime.WindowShow(a.ctx)
	wruntime.WindowUnminimise(a.ctx)
	wruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wruntime.WindowSetAlwaysOnTop(a.ctx, false)
}

// skipNote logs why entries were skipped during an import and returns a
// short suffix for the result message.
func (a *App) skipNote(skipped []string) string {
	if len(skipped) == 0 {
		return ""
	}
	for i, reason := range skipped {
		if i == 20 {
			a.log(fmt.Sprintf(i18n.T("……另有 %d 条被跳过"), len(skipped)-20))
			break
		}
		a.log(i18n.T("已跳过 ") + reason)
	}
	return fmt.Sprintf(i18n.T("，跳过 %d 条（%s）"), len(skipped), skipped[0])
}

// ---------------------------------------------------------------- log files

// applyLogFile starts, stops or reconfigures file logging after a settings change.
func (a *App) applyLogFile(old, s store.Settings) {
	switch {
	case s.LogToFile && !old.LogToFile:
		a.fileLog.Store(logfile.New(a.logDir(), s.LogFileMaxMB))
		a.log(i18n.T("开始记录日志到文件: ") + a.logDir())
	case !s.LogToFile && old.LogToFile:
		if w := a.fileLog.Swap(nil); w != nil {
			w.Close()
		}
	case s.LogToFile && old.LogFileMaxMB != s.LogFileMaxMB:
		if w := a.fileLog.Load(); w != nil {
			w.SetMaxMB(s.LogFileMaxMB)
		}
	}
	if s.LogKeepDays != old.LogKeepDays {
		logfile.Prune(a.logDir(), s.LogKeepDays)
	}
}

type LogFilesInfo struct {
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
	Dir   string `json:"dir"`
}

func (a *App) GetLogFilesInfo() LogFilesInfo {
	count, bytes := logfile.Stats(a.logDir())
	return LogFilesInfo{Count: count, Bytes: bytes, Dir: a.logDir()}
}

// ClearLogFiles deletes all log files; if logging is on, the next line
// starts a fresh file.
func (a *App) ClearLogFiles() int {
	if w := a.fileLog.Load(); w != nil {
		w.Close()
	}
	return logfile.RemoveAll(a.logDir())
}

func (a *App) OpenLogDir() error {
	dir := a.logDir()
	os.MkdirAll(dir, 0o755)
	if goruntime.GOOS == "windows" {
		return exec.Command("explorer", dir).Start()
	}
	return exec.Command("xdg-open", dir).Start()
}

// ---------------------------------------------------------------- core info

// CoreInfo describes the sing-box binary, parsed from `sing-box version`.
type CoreInfo struct {
	Found       bool   `json:"found"`
	Version     string `json:"version"`     // e.g. 1.14.4
	Channel     string `json:"channel"`     // stable | testing
	Environment string `json:"environment"` // Go version and platform
	Tags        string `json:"tags"`        // build tags (enabled features)
	Revision    string `json:"revision"`    // git commit, when built from a checkout
	CGO         string `json:"cgo"`
}

func (a *App) GetCoreInfo() CoreInfo {
	cmd := exec.Command(a.corePath(), "version")
	sys.PrepareCmd(cmd)
	out, err := cmd.Output()
	if err != nil {
		return CoreInfo{}
	}
	info := CoreInfo{Found: true, Channel: "stable"}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, _ := strings.Cut(strings.TrimSpace(line), ":")
		value = strings.TrimSpace(value)
		switch {
		case strings.HasPrefix(key, "sing-box version "):
			info.Version = strings.TrimPrefix(key, "sing-box version ")
		case key == "Environment":
			info.Environment = value
		case key == "Tags":
			info.Tags = value
		case key == "Revision":
			info.Revision = value
		case key == "CGO":
			info.CGO = value
		}
	}
	// pre-releases (x.y.z-alpha.n / -beta.n / -rc.n) come from the testing branch
	if strings.Contains(info.Version, "-") {
		info.Channel = "testing"
	}
	return info
}
