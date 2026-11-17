//go:build windows

package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// State of the installed service, as NextBox shows it.
type Installed struct {
	Installed bool `json:"installed"`
	Running   bool `json:"running"`
}

// QueryInstalled asks Windows whether the service is installed and running;
// it needs no rights.
func QueryInstalled() Installed {
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return Installed{}
	}
	defer windows.CloseServiceHandle(h)
	name, _ := windows.UTF16PtrFromString(Name)
	s, err := windows.OpenService(h, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return Installed{}
	}
	defer windows.CloseServiceHandle(s)
	var st windows.SERVICE_STATUS_PROCESS
	var n uint32
	if err := windows.QueryServiceStatusEx(s, windows.SC_STATUS_PROCESS_INFO, (*byte)(unsafePointer(&st)), uint32(sizeofStatus), &n); err != nil {
		return Installed{Installed: true}
	}
	return Installed{Installed: true, Running: st.CurrentState == windows.SERVICE_RUNNING}
}

// servicePID is the process the service runs in, 0 when it is not running.
func servicePID() uint32 {
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return 0
	}
	defer windows.CloseServiceHandle(h)
	name, _ := windows.UTF16PtrFromString(Name)
	s, err := windows.OpenService(h, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return 0
	}
	defer windows.CloseServiceHandle(s)
	var st windows.SERVICE_STATUS_PROCESS
	var n uint32
	if windows.QueryServiceStatusEx(s, windows.SC_STATUS_PROCESS_INFO, (*byte)(unsafePointer(&st)), uint32(sizeofStatus), &n) != nil ||
		st.CurrentState != windows.SERVICE_RUNNING {
		return 0
	}
	return st.ProcessId
}

// dial connects to the service, and makes sure that it is the service at
// the other end: the config holds node passwords, and any program could
// create a pipe of that name while the service is not running.
func dial(ctx context.Context) (net.Conn, error) {
	pid := expectedServerPID()
	if pid == 0 {
		return nil, ErrNotRunning
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	c, err := winio.DialPipeContext(ctx, pipePath)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil, ErrNotAllowed
		}
		return nil, err
	}
	f, ok := c.(interface{ Fd() uintptr })
	var server uint32
	if !ok || windows.GetNamedPipeServerProcessId(windows.Handle(f.Fd()), &server) != nil || server != pid {
		c.Close()
		return nil, errors.New("the pipe does not belong to the NextBox service")
	}
	return c, nil
}

var (
	ErrNotRunning = errors.New("the NextBox service is not running")
	ErrNotAllowed = errors.New("the NextBox service was installed by another user")
)

// call sends one request and reads its reply.
func call(ctx context.Context, req Request) (Reply, error) {
	c, err := dial(ctx)
	if err != nil {
		return Reply{}, err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(10 * time.Second))
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return Reply{}, err
	}
	var r Reply
	if err := json.NewDecoder(c).Decode(&r); err != nil {
		return Reply{}, err
	}
	if r.Error != "" {
		return r, errors.New(r.Error)
	}
	return r, nil
}

// Hello asks the running service about itself.
func Hello(ctx context.Context) (*Status, error) {
	r, err := call(ctx, Request{Op: "hello"})
	if err != nil {
		return nil, err
	}
	if r.Status == nil {
		return nil, errors.New("no status")
	}
	return r.Status, nil
}

// SetAutostart has the service start path when the current user logs on
// (enabled), or stop doing so.
func SetAutostart(ctx context.Context, path string, enabled bool) (*Status, error) {
	r, err := call(ctx, Request{Op: "autostart", Path: path, Enabled: enabled})
	if err != nil {
		return nil, err
	}
	return r.Status, nil
}

// Remote is a core the service runs for NextBox (a core.Backend).
type Remote struct {
	c       net.Conn
	wmu     sync.Mutex
	done    chan struct{}
	exitErr error
}

// StartCore has the service run a core with cfg (JSON) and the rule-set
// files it needs; log gets every line the core prints.
func StartCore(ctx context.Context, cfg []byte, ruleSets map[string][]byte, log func(string)) (*Remote, error) {
	c, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	c.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := json.NewEncoder(c).Encode(Request{Op: "start", Config: cfg, RuleSets: ruleSets}); err != nil {
		c.Close()
		return nil, err
	}
	c.SetWriteDeadline(time.Time{})
	dec := json.NewDecoder(bufio.NewReader(c))
	c.SetReadDeadline(time.Now().Add(30 * time.Second))
	var first Reply
	if err := dec.Decode(&first); err != nil {
		c.Close()
		return nil, err
	}
	if first.Error != "" || !first.Started {
		c.Close()
		return nil, errors.New("NextBox service: " + first.Error)
	}
	c.SetReadDeadline(time.Time{})
	r := &Remote{c: c, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		for {
			var m Reply
			if err := dec.Decode(&m); err != nil {
				r.exitErr = errors.New("lost the connection to the NextBox service")
				return
			}
			switch {
			case m.Exit != nil:
				if *m.Exit != "" {
					r.exitErr = errors.New(*m.Exit)
				}
				return
			case m.Log != "":
				log(m.Log)
			}
		}
	}()
	return r, nil
}

func (r *Remote) Interrupt() error {
	r.wmu.Lock()
	defer r.wmu.Unlock()
	r.c.SetWriteDeadline(time.Now().Add(3 * time.Second))
	return json.NewEncoder(r.c).Encode(Request{Op: "stop"})
}

// Kill drops the connection: the service then stops the core.
func (r *Remote) Kill() error { return r.c.Close() }

func (r *Remote) Wait() error {
	<-r.done
	r.c.Close()
	return r.exitErr
}

// ---------------------------------------------------------------- install

// Install installs (or updates) the service from the files in srcDir
// (NextBoxService.exe, sing-box.exe and libcronet.dll, NextBox's "core"
// folder), lets the user sid use it and starts it. It needs administrator
// rights.
func Install(srcDir, sid, version string) error {
	dir := Dir()
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s, err := m.OpenService(Name); err == nil {
		stopService(s)
		s.Close()
	}
	if err := copyFiles(srcDir, dir); err != nil {
		return err
	}
	st := loadState(dir)
	st.Version = version
	if _, ok := st.Users[sid]; !ok {
		st.Users[sid] = &user{}
	}
	if err := st.save(dir); err != nil {
		return err
	}
	exe := dir + `\` + ExeName
	cfg := mgr.Config{
		DisplayName:  DisplayName,
		Description:  "Runs the sing-box core for NextBox in TUN mode, and starts NextBox when its users log on.",
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	}
	s, err := m.OpenService(Name)
	if err != nil {
		s, err = m.CreateService(Name, exe, cfg)
		if err != nil {
			return err
		}
	} else {
		old, _ := s.Config()
		old.BinaryPathName = `"` + exe + `"`
		old.DisplayName, old.Description, old.StartType = cfg.DisplayName, cfg.Description, cfg.StartType
		if err := s.UpdateConfig(old); err != nil {
			s.Close()
			return err
		}
	}
	defer s.Close()
	// restart it when it crashes
	s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 2 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
	}, 24*60*60)
	if err := s.Start(); err != nil {
		return err
	}
	return waitState(s, svc.Running)
}

// Uninstall stops and removes the service and its folder. It needs
// administrator rights.
func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s, err := m.OpenService(Name); err == nil {
		stopService(s)
		err := s.Delete()
		s.Close()
		if err != nil {
			return err
		}
	}
	return removeDir(Dir())
}

func stopService(s *mgr.Service) {
	if st, err := s.Query(); err == nil && st.State != svc.Stopped {
		s.Control(svc.Stop)
		waitState(s, svc.Stopped)
	}
}

func waitState(s *mgr.Service, want svc.State) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		st, err := s.Query()
		if err != nil {
			return err
		}
		if st.State == want {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("the NextBox service did not respond in time")
}

// The pipe, and the process that must be at its other end; tests replace
// them to run a server in the test process.
var (
	pipePath          = PipeName
	expectedServerPID = servicePID
)
