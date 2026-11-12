package main

// Connections, logs, log files and the core version, for the UI.

import (
	"errors"
	"nextbox/internal/i18n"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"

	"nextbox/internal/core"
	"nextbox/internal/logfile"
	"nextbox/internal/store"
	"nextbox/internal/sys"
)

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
