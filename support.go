package main

import (
	"context"
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
)

// logBuffer keeps the most recent log lines for the log page. New lines are
// delivered to onFlush in batches, so a busy core does not flood the UI with
// one event per line.
type logBuffer struct {
	mu      sync.Mutex
	lines   []string
	max     int
	pending []string
	onFlush func([]string)
	sink    func(string) // optional, receives every line (log file)
}

func newLogBuffer(max int) *logBuffer { return &logBuffer{max: max} }

func (b *logBuffer) Append(line string) {
	b.mu.Lock()
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
	batch, cb := b.pending, b.onFlush
	b.pending = nil
	b.mu.Unlock()
	if cb != nil && len(batch) > 0 {
		cb(batch)
	}
}

func (b *logBuffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string{}, b.lines...)
}

func (b *logBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = nil
}

// downloadRuleSets fetches .srs files from the configured mirror. proxy is an
// optional "http://host:port".
func (a *App) downloadRuleSets(names []string, proxy string) error {
	s := a.store.Settings()
	dir := a.ruleSetDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		if pu, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}
	client := &http.Client{Transport: transport, Timeout: 60 * time.Second}
	for _, name := range names {
		u := config.RuleSetURL(s.RuleSetMirror, name)
		if err := downloadFile(a.ctx, client, u, filepath.Join(dir, name+".srs")); err != nil {
			return fmt.Errorf(i18n.T("下载规则集 %s 失败: %w（可在设置中更换镜像）"), name, err)
		}
		a.log(i18n.T("规则集已下载: ") + name)
	}
	return nil
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
