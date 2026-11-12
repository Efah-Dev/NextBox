package main

import (
	"context"
	"fmt"
	"nextbox/internal/i18n"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"sync/atomic"
	"time"

	"nextbox/internal/core"
	"nextbox/internal/logfile"
	"nextbox/internal/store"
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
	geoMu     sync.Mutex // one server-country lookup at a time

	mu           sync.Mutex // guards the fields below
	proc         *core.Process
	runningID    int64
	loaded       map[int64]loadedNode // nodes in the running core, by profile id
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
	go a.backgroundLoop(ctx)
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
	if old.CountrySource != s.CountrySource {
		go a.locateServers(nil, false)
	}
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
	s.SubAutoUpdate, s.SubUpdateHours, s.AutoCheckUpdate = false, 0, false
	s.CountrySource, s.CountryNoticeShown = "", false
	s.ConnectOnStart, s.AutoStart, s.CloseToTray, s.SystemProxy = false, false, false, false
	s.LogToFile, s.LogFileMaxMB, s.LogKeepDays, s.LogViewLimit = false, 0, 0, 0
	return s
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
