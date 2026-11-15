package main

// What happened each time Windows started NextBox (设置 → 开机自启 shows
// the last ones), and whether it did not start it at all, so that a failed
// autostart can be told apart: not started, started but not connected, or
// connected with the window in the tray.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"nextbox/internal/i18n"
	"nextbox/internal/sys"
)

// AutoStartRecord is one start by Windows.
type AutoStartRecord struct {
	At int64 `json:"at"` // unix seconds
	// Result is "connected", "failed" (Detail says why), "locked" (waits for
	// the start-up password), "noNode" (no node selected yet), "started"
	// (autoconnect is off, or still connecting) or "missed" (Windows did not
	// start NextBox after this boot; At is the boot time).
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
}

type autoStartLog struct {
	EnabledAt int64             `json:"enabledAt,omitempty"` // when autostart was last switched on
	Records   []AutoStartRecord `json:"records"`             // newest first
}

const keepAutoStartRecords = 5

func (a *App) autoStartLogPath() string { return filepath.Join(a.dataDir(), "autostart.json") }

func (a *App) readAutoStartLog() autoStartLog {
	var l autoStartLog
	if b, err := os.ReadFile(a.autoStartLogPath()); err == nil {
		json.Unmarshal(b, &l)
	}
	return l
}

// updateAutoStartLog changes the log under a lock and saves it.
func (a *App) updateAutoStartLog(fn func(*autoStartLog)) {
	a.autoLogMu.Lock()
	defer a.autoLogMu.Unlock()
	l := a.readAutoStartLog()
	fn(&l)
	if len(l.Records) > keepAutoStartRecords {
		l.Records = l.Records[:keepAutoStartRecords]
	}
	if b, err := json.MarshalIndent(l, "", "  "); err == nil {
		os.WriteFile(a.autoStartLogPath(), b, 0o600)
	}
}

// recordAutoStart adds a record for this start, or updates it.
func (a *App) recordAutoStart(result, detail string) {
	a.updateAutoStartLog(func(l *autoStartLog) {
		if a.autoStartAt == 0 {
			a.autoStartAt = time.Now().Unix()
		}
		for i, r := range l.Records {
			if r.At == a.autoStartAt && r.Result != "missed" {
				l.Records[i].Result, l.Records[i].Detail = result, detail
				return
			}
		}
		l.Records = append([]AutoStartRecord{{At: a.autoStartAt, Result: result, Detail: detail}}, l.Records...)
	})
}

// autoStartFailed records why connecting after a start by Windows did not
// work and brings the window up, so that it is not a silent failure.
func (a *App) autoStartFailed(result, detail string) {
	a.recordAutoStart(result, detail)
	if detail != "" {
		a.log(i18n.T("开机自启后自动连接失败: ") + detail)
	}
	a.ShowWindow()
}

// checkMissedAutoStart notes a boot after which Windows did not start
// NextBox although autostart was on (NextBox was opened by hand instead).
// The Run entries of a logon can take a while, so it waits until Windows has
// been up for a few minutes.
func (a *App) checkMissedAutoStart() {
	if !a.store.Settings().AutoStart || a.autoStarted {
		return
	}
	boot := sys.BootTime()
	if boot.IsZero() {
		return
	}
	if wait := time.Until(boot.Add(3 * time.Minute)); wait > 0 {
		time.Sleep(wait)
	}
	if !a.store.Settings().AutoStart || a.autoStartAt != 0 {
		return
	}
	a.updateAutoStartLog(func(l *autoStartLog) {
		if l.EnabledAt == 0 || l.EnabledAt > boot.Unix() {
			return // switched on after this boot, or before records were kept
		}
		for _, r := range l.Records {
			if r.At >= boot.Unix()-60 {
				return // started (or already noted) after this boot
			}
		}
		l.Records = append([]AutoStartRecord{{At: boot.Unix(), Result: "missed"}}, l.Records...)
	})
}

// autoStartEnabled marks when autostart was switched on: boots before that
// are not expected to have started NextBox.
func (a *App) autoStartEnabled() {
	a.updateAutoStartLog(func(l *autoStartLog) { l.EnabledAt = time.Now().Unix() })
}
