// Package core runs sing-box as a child process and talks to it through the
// clash API.
package core

import (
	"bufio"
	"bytes"
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
	"strings"
	"sync"
	"time"

	"nextbox/internal/sys"
)

// Process is a running core: a child process of NextBox (Start), or one the
// NextBox service runs for it (see Attach).
type Process struct {
	API *API

	b       Backend
	done    chan struct{}
	exitErr error

	mu   sync.Mutex
	tail []string // last log lines, used in error messages
}

// Backend is where a core runs.
type Backend interface {
	// Interrupt asks the core to exit by itself (so that it removes its
	// TUN routes and closes its cache file); Kill ends it at once.
	Interrupt() error
	Kill() error
	// Wait blocks until the core has exited and returns why.
	Wait() error
}

// DefaultReadyTimeout is how long Start and Attach wait for the clash API
// to answer. Right after Windows starts, a core may take much longer than
// that (see Options.ReadyTimeout).
const DefaultReadyTimeout = 10 * time.Second

// NotReadyError is returned when the core was still running but its clash
// API did not answer in time; the message has the core's last log lines.
type NotReadyError struct{ msg string }

func (e *NotReadyError) Error() string { return e.msg }

type Options struct {
	ReadyTimeout time.Duration // 0 = DefaultReadyTimeout
	OnLog        func(string)
}

// Start launches `sing-box run` in workDir with cfg. The config holds node
// passwords, so it is handed over through stdin and never written to disk.
// Start returns once the clash API answers, or with an error if the core exits.
func Start(ctx context.Context, bin, workDir string, cfg map[string]any, api *API, o Options) (*Process, error) {
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf(i18n.T("找不到内核 %s，请把 sing-box 放到 core 目录"), bin)
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return Attach(ctx, api, o, func(line func(string)) (Backend, error) {
		return StartChild(bin, workDir, b, line)
	})
}

// Attach starts a core with start, which reports the core's log lines to
// line, and waits for its clash API like Start.
func Attach(ctx context.Context, api *API, o Options, start func(line func(string)) (Backend, error)) (*Process, error) {
	p := &Process{API: api, done: make(chan struct{})}
	b, err := start(func(s string) {
		p.mu.Lock()
		p.tail = append(p.tail, s)
		if len(p.tail) > 20 {
			p.tail = p.tail[len(p.tail)-20:]
		}
		p.mu.Unlock()
		if o.OnLog != nil {
			o.OnLog(s)
		}
	})
	if err != nil {
		return nil, err
	}
	p.b = b
	go func() {
		p.exitErr = b.Wait()
		close(p.done)
	}()
	timeout := o.ReadyTimeout
	if timeout <= 0 {
		timeout = DefaultReadyTimeout
	}
	if err := p.waitReady(ctx, timeout); err != nil {
		p.Stop()
		return nil, err
	}
	return p, nil
}

// childBackend is a core running as a child process.
type childBackend struct {
	cmd  *exec.Cmd
	pw   *io.PipeWriter
	read chan struct{} // closed when all output has been read
}

// StartChild starts `sing-box run` with the config cfg (JSON) in workDir,
// bound to this process (it ends when this process does), and reports
// every line it prints.
func StartChild(bin, workDir string, cfg []byte, line func(string)) (Backend, error) {
	cmd := exec.Command(bin, "run", "-c", "stdin", "-D", workDir, "--disable-color")
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(cfg)
	sys.PrepareCmd(cmd)
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sys.AttachCmd(cmd)
	c := &childBackend{cmd: cmd, pw: pw, read: make(chan struct{})}
	go func() {
		defer close(c.read)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			line(sc.Text())
		}
		io.Copy(io.Discard, pr)
	}()
	return c, nil
}

func (c *childBackend) Interrupt() error { return sys.Interrupt(c.cmd.Process.Pid) }
func (c *childBackend) Kill() error      { return c.cmd.Process.Kill() }
func (c *childBackend) Wait() error {
	err := c.cmd.Wait()
	c.pw.Close()
	select { // let the reader collect the final lines
	case <-c.read:
	case <-time.After(time.Second):
	}
	return err
}

func (p *Process) waitReady(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-p.done:
			tail := p.Tail()
			if strings.Contains(tail, "naive outbound is not included") || strings.Contains(strings.ToLower(tail), "libcronet") {
				// a core without NaiveProxy, or its libcronet.dll is missing
				return errors.New(i18n.T("当前内核不支持 NaiveProxy：请换回 NextBox 自带的内核，并确认 core 文件夹里有 libcronet.dll") + "\n" + tail)
			}
			if p.exitErr != nil && tail == "" {
				return fmt.Errorf(i18n.T("内核启动失败:\n%s"), p.exitErr.Error())
			}
			return fmt.Errorf(i18n.T("内核启动失败:\n%s"), tail)
		case <-ctx.Done():
			msg := i18n.T("等待内核启动超时")
			if tail := p.Tail(); tail != "" {
				msg += i18n.T("，内核最后的输出:\n") + tail
			}
			return &NotReadyError{msg}
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

// Stop shuts the core down. It first asks it to exit by itself, so that
// sing-box can remove the TUN routes and close its cache file, and kills it
// when that is not possible or takes too long.
func (p *Process) Stop() {
	select {
	case <-p.done:
		return
	default:
	}
	if p.b.Interrupt() == nil {
		select {
		case <-p.done:
			return
		case <-time.After(3 * time.Second):
		}
	}
	p.b.Kill()
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
