package main

// Where the traffic of the connected node leaves: shown on the home page
// (日志卡片右下角) with a status light.

import (
	"context"
	"net/url"
	"time"

	"nextbox/internal/core"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExitInfo is the result of the last check through the running core.
type ExitInfo struct {
	NodeID   int64  `json:"nodeId"`
	Checking bool   `json:"checking"`
	IP       string `json:"ip"`
	Country  string `json:"country"`
	Latency  int    `json:"latency"` // ms; -1 failed
	Error    string `json:"error,omitempty"`
	At       int64  `json:"at"` // unix ms
}

const exitRecheck = 5 * time.Minute

func (a *App) GetExitInfo() ExitInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proc == nil {
		return ExitInfo{}
	}
	return a.exit
}

// RefreshExit checks again now (the status pill was clicked).
func (a *App) RefreshExit() { go a.checkExit() }

func (a *App) setExit(e ExitInfo) {
	a.mu.Lock()
	a.exit = e
	a.mu.Unlock()
	wruntime.EventsEmit(a.ctx, "exit", e)
}

// checkExit asks Cloudflare, through the local proxy port, which address and
// country requests come from, and how long a request takes.
func (a *App) checkExit() {
	a.exitMu.Lock()
	defer a.exitMu.Unlock()
	a.mu.Lock()
	proc, node := a.proc, a.runningID
	a.mu.Unlock()
	if proc == nil {
		return
	}
	a.setExit(ExitInfo{NodeID: node, Checking: true})
	s := a.store.Settings()
	proxy := &url.URL{Scheme: "http", Host: proxyAddr(s)}
	timeout := time.Duration(max(s.TestTimeout, 3000)) * time.Millisecond
	ctx, cancel := context.WithTimeout(a.ctx, 2*timeout+5*time.Second)
	defer cancel()
	ms, loc, err := core.ProbeExit(ctx, proxy, s.TestURL, timeout)
	e := ExitInfo{NodeID: node, Latency: ms, IP: loc.IP, Country: loc.Country, At: time.Now().UnixMilli()}
	if err != nil {
		e.Latency, e.Error = -1, err.Error()
	}
	// the node may have changed or the core stopped meanwhile
	a.mu.Lock()
	stale := a.proc != proc || a.runningID != node
	a.mu.Unlock()
	if !stale {
		a.setExit(e)
	}
}

// exitLoop checks after connecting and then every few minutes.
func (a *App) exitLoop(ctx context.Context) {
	time.Sleep(500 * time.Millisecond) // let the system proxy settle
	for {
		a.checkExit()
		select {
		case <-ctx.Done():
			return
		case <-time.After(exitRecheck):
		}
	}
}
