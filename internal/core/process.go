// Package core runs sing-box as a child process and talks to it through the
// clash API.
package core

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"nextbox/internal/i18n"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nextbox/internal/sys"
)

type Process struct {
	API *API

	cmd     *exec.Cmd
	done    chan struct{}
	exitErr error

	mu   sync.Mutex
	tail []string // last log lines, used in error messages
}

// Start writes cfg to workDir/configName and launches `sing-box run`. It
// returns once the clash API answers, or with an error if the core exits.
func Start(ctx context.Context, bin, workDir, configName string, cfg map[string]any, api *API, onLog func(string)) (*Process, error) {
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf(i18n.T("找不到内核 %s，请把 sing-box 放到 core 目录"), bin)
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(workDir, configName)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		return nil, err
	}

	cmd := exec.Command(bin, "run", "-c", cfgPath, "-D", workDir, "--disable-color")
	cmd.Dir = workDir
	sys.PrepareCmd(cmd)
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sys.AttachCmd(cmd)

	p := &Process{API: api, cmd: cmd, done: make(chan struct{})}
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			p.mu.Lock()
			p.tail = append(p.tail, line)
			if len(p.tail) > 20 {
				p.tail = p.tail[len(p.tail)-20:]
			}
			p.mu.Unlock()
			if onLog != nil {
				onLog(line)
			}
		}
	}()
	go func() {
		p.exitErr = cmd.Wait()
		pw.Close()
		close(p.done)
	}()

	if err := p.waitReady(ctx); err != nil {
		p.Stop()
		return nil, err
	}
	return p, nil
}

func (p *Process) waitReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-p.done:
			// give the log reader a moment to collect the final lines
			time.Sleep(100 * time.Millisecond)
			return fmt.Errorf(i18n.T("内核启动失败:\n%s"), p.Tail())
		case <-ctx.Done():
			return errors.New(i18n.T("等待内核启动超时"))
		case <-tick.C:
			if p.API.Ping(ctx) == nil {
				return nil
			}
		}
	}
}

// Tail returns the last log lines of the core.
func (p *Process) Tail() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return strings.Join(p.tail, "\n")
}

func (p *Process) Done() <-chan struct{} { return p.done }

func (p *Process) ExitErr() error {
	select {
	case <-p.done:
		return p.exitErr
	default:
		return nil
	}
}

func (p *Process) Stop() {
	select {
	case <-p.done:
		return
	default:
	}
	if p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
	}
}

// FreePort asks the OS for an unused local TCP port.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func RandomSecret() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
