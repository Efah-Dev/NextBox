package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nextbox/internal/i18n"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/ruleset"
	"nextbox/internal/store"
)

// LogBatch is a run of consecutive log lines. Next is the number of lines
// appended so far, up to and including the last one, so the UI can tell which
// lines of a batch it already got from GetLogs.
type LogBatch struct {
	Lines []string `json:"lines"`
	Next  int64    `json:"next"`
}

// logBuffer keeps the most recent log lines for the log page. New lines are
// delivered to onFlush in batches, so a busy core does not flood the UI with
// one event per line.
type logBuffer struct {
	mu      sync.Mutex
	lines   []string
	max     int
	total   int64 // lines appended so far
	pending []string
	onFlush func(LogBatch)
	sink    func(string) // optional, receives every line (log file)
}

func newLogBuffer(max int) *logBuffer { return &logBuffer{max: max} }

func (b *logBuffer) Append(line string) {
	b.mu.Lock()
	b.total++
	b.lines = append(b.lines, line)
	if len(b.lines) > b.max {
		b.lines = append([]string(nil), b.lines[len(b.lines)-b.max:]...)
	}
	b.pending = append(b.pending, line)
	if len(b.pending) == 1 {
		time.AfterFunc(200*time.Millisecond, b.flush)
	}
	b.mu.Unlock()
	if b.sink != nil {
		b.sink(line)
	}
}

func (b *logBuffer) flush() {
	b.mu.Lock()
	// pending is always the tail of everything appended, so it ends at total
	batch, cb := LogBatch{Lines: b.pending, Next: b.total}, b.onFlush
	b.pending = nil
	b.mu.Unlock()
	if cb != nil && len(batch.Lines) > 0 {
		cb(batch)
	}
}

func (b *logBuffer) Snapshot() LogBatch {
	b.mu.Lock()
	defer b.mu.Unlock()
	return LogBatch{Lines: append([]string{}, b.lines...), Next: b.total}
}

func (b *logBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = nil
}

// ensureRuleSets fetches the rule-sets the current settings need but which
// are not on disk yet, and returns those still missing afterwards. Failures
// are only logged: the caller carries on without the affected rules.
func (a *App) ensureRuleSets() []string {
	a.ruleSetMu.Lock()
	defer a.ruleSetMu.Unlock()
	names := config.RequiredRuleSets(a.store.Settings())
	missing := config.MissingRuleSets(a.ruleSetDir(), names)
	if len(missing) == 0 {
		return nil
	}
	a.fetchRuleSetsLocked(missing, "")
	return config.MissingRuleSets(a.ruleSetDir(), names)
}

// fetchRuleSetsLocked makes the named .srs files available. With the local
// mirror the bundled copies are used; everything else is downloaded, and a
// failed download falls back to the bundled copy when there is no file yet.
// proxy is an optional "http://host:port". The caller holds ruleSetMu.
func (a *App) fetchRuleSetsLocked(names []string, proxy string) error {
	mirror := a.store.Settings().RuleSetMirror
	dir := a.ruleSetDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.log(i18n.T("规则集获取失败: ") + err.Error())
		return err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		if pu, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
	var errs []error
	for _, name := range names {
		dest := filepath.Join(dir, name+".srs")
		if mirror == store.LocalRuleSetMirror && ruleset.Has(name) {
			if err := ruleset.Install(name, dest); err != nil {
				a.log(i18n.T("规则集获取失败: ") + err.Error())
				errs = append(errs, err)
				continue
			}
			a.log(i18n.T("已加载内置规则集: ") + name)
			continue
		}
		remote := mirror
		if remote == store.LocalRuleSetMirror {
			remote = store.RemoteRuleSetMirror
		}
		err := downloadFile(a.ctx, client, config.RuleSetURL(remote, name), dest)
		if err == nil {
			a.log(i18n.T("规则集已下载: ") + name)
			continue
		}
		err = fmt.Errorf(i18n.T("下载规则集 %s 失败: %w（可在设置中更换镜像）"), name, err)
		a.log(err.Error())
		if _, statErr := os.Stat(dest); statErr != nil && ruleset.Has(name) && ruleset.Install(name, dest) == nil {
			a.log(i18n.T("已改用内置规则集: ") + name)
			continue
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func downloadFile(ctx context.Context, client *http.Client, u, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := dest + ".download"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, io.LimitReader(resp.Body, 64<<20))
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}
