//go:build windows

package service

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"

	"nextbox/internal/core"
)

// Run runs the service; it returns when Windows stops it.
func Run(version string) error {
	return svc.Run(Name, &server{version: version, dir: Dir()})
}

type server struct {
	version string
	dir     string

	mu       sync.Mutex // guards the fields below
	st       *state
	running  *run // the core, if any
	coreHash string
	launched map[uint64]bool // logons NextBox was started for (by logon ID)
}

// run is a core started for a connection.
type run struct {
	b    core.Backend
	done chan struct{}
	stop sync.Once
}

func (s *server) Execute(args []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	s.st = loadState(s.dir)
	s.coreHash = FileHash(filepath.Join(s.dir, "sing-box.exe"))
	s.launched = map[uint64]bool{}
	logf("service %s started", s.version)

	l, err := s.listen()
	if err != nil {
		logf("listen: %v", err)
		return true, 1
	}
	go s.serve(l)
	// started with Windows after the user had already logged on (automatic
	// logon): start NextBox for them now
	if uptime() < 10*time.Minute {
		go s.launchAll()
	}

	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.AcceptSessionChange}
	for c := range req {
		switch c.Cmd {
		case svc.Interrogate:
			status <- c.CurrentStatus
		case svc.SessionChange:
			if c.EventType == wtsSessionLogon {
				go s.launchAll() // each logon is started once (see launch)
			}
		case svc.Stop, svc.Shutdown:
			status <- svc.Status{State: svc.StopPending}
			l.Close()
			s.stopCore()
			return false, 0
		}
	}
	return false, 0
}

const wtsSessionLogon = 5

func (s *server) listen() (net.Listener, error) {
	s.mu.Lock()
	sids := make([]string, 0, len(s.st.Users))
	for sid := range s.st.Users {
		sids = append(sids, sid)
	}
	s.mu.Unlock()
	return winio.ListenPipe(pipePath, &winio.PipeConfig{
		SecurityDescriptor: pipeSDDL(sids),
		InputBufferSize:    64 << 10,
		OutputBufferSize:   64 << 10,
	})
}

func (s *server) serve(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			if errors.Is(err, winio.ErrPipeListenerClosed) {
				return
			}
			logf("accept: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go s.handle(c)
	}
}

// conn is one client; replies are written one line at a time.
type conn struct {
	net.Conn
	sid string
	wmu sync.Mutex
}

func (c *conn) reply(r Reply) error {
	b, _ := json.Marshal(r)
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.Write(append(b, '\n'))
	return err
}

func (s *server) handle(nc net.Conn) {
	defer nc.Close()
	sid, err := clientSID(nc)
	if err != nil {
		logf("client: %v", err)
		return
	}
	c := &conn{Conn: nc, sid: sid}
	r2 := bufio.NewScanner(nc)
	r2.Buffer(make([]byte, 1<<20), 64<<20) // configs with many nodes and rule-sets
	for r2.Scan() {
		var req Request
		if err := json.Unmarshal(r2.Bytes(), &req); err != nil {
			c.reply(Reply{Error: "bad request"})
			return
		}
		switch req.Op {
		case "hello":
			c.reply(Reply{Status: s.status(sid)})
		case "autostart":
			if err := s.setAutostart(sid, req.Path, req.Enabled); err != nil {
				c.reply(Reply{Error: err.Error()})
				continue
			}
			c.reply(Reply{Status: s.status(sid)})
		case "start":
			s.startCore(c, r2, req)
			return // the connection belonged to that core
		default:
			c.reply(Reply{Error: "unknown op " + req.Op})
		}
	}
}

// clientSID is the user of the process at the other end of the pipe.
func clientSID(c net.Conn) (string, error) {
	f, ok := c.(interface{ Fd() uintptr })
	if !ok {
		return "", errors.New("not a pipe")
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(windows.Handle(f.Fd()), &pid); err != nil {
		return "", err
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	var t windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY, &t); err != nil {
		return "", err
	}
	defer t.Close()
	return tokenSID(t)
}

func (s *server) status(sid string) *Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &Status{Version: s.version, Protocol: Protocol, Dir: s.dir, CoreHash: s.coreHash, Running: s.running != nil}
	if u := s.st.Users[sid]; u != nil {
		st.Autostart = u.Autostart
	}
	return st
}

func (s *server) setAutostart(sid, path string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.st.Users[sid]
	if u == nil {
		// an administrator who did not install it: allowed, like everything
		u = &user{}
		s.st.Users[sid] = u
	}
	if enabled {
		if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") {
			return errors.New("not a program: " + path)
		}
		u.Autostart = path
	} else {
		u.Autostart = ""
	}
	return s.st.save(s.dir)
}

// ---------------------------------------------------------------- the core

func (s *server) startCore(c *conn, sc *bufio.Scanner, req Request) {
	var cfg map[string]any
	if err := json.Unmarshal(req.Config, &cfg); err != nil {
		c.reply(Reply{Error: "bad config"})
		return
	}
	work := filepath.Join(s.dir, "run")
	sets := filepath.Join(work, "rule-set")
	names, err := Sanitize(cfg, work, sets)
	if err != nil {
		c.reply(Reply{Error: err.Error()})
		return
	}
	s.stopCore() // one core at a time: a new start replaces the old one
	os.MkdirAll(sets, 0o755)
	for _, n := range names {
		b, ok := req.RuleSets[n]
		if !ok {
			c.reply(Reply{Error: "missing rule-set " + n})
			return
		}
		if err := os.WriteFile(filepath.Join(sets, n), b, 0o644); err != nil {
			c.reply(Reply{Error: err.Error()})
			return
		}
	}
	b, _ := json.Marshal(cfg)
	be, err := core.StartChild(filepath.Join(s.dir, "sing-box.exe"), work, b, func(line string) {
		c.reply(Reply{Log: line})
	})
	if err != nil {
		c.reply(Reply{Error: err.Error()})
		return
	}
	rn := &run{b: be, done: make(chan struct{})}
	s.mu.Lock()
	s.running = rn
	s.mu.Unlock()
	logf("core started")
	c.reply(Reply{Started: true})

	exited := make(chan error, 1)
	go func() {
		exited <- be.Wait()
		close(rn.done)
	}()
	// the client says stop, or goes away
	go func() {
		for sc.Scan() {
			var r Request
			if json.Unmarshal(sc.Bytes(), &r) == nil && r.Op == "stop" {
				break
			}
		}
		s.stopRun(rn)
	}()
	err = <-exited
	s.mu.Lock()
	if s.running == rn {
		s.running = nil
	}
	s.mu.Unlock()
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	logf("core exited %s", msg)
	c.reply(Reply{Exit: &msg})
}

// stopRun asks a core to exit, and ends it when that takes too long.
func (s *server) stopRun(rn *run) {
	rn.stop.Do(func() {
		if rn.b.Interrupt() == nil {
			select {
			case <-rn.done:
				return
			case <-time.After(3 * time.Second):
			}
		}
		rn.b.Kill()
		select {
		case <-rn.done:
		case <-time.After(5 * time.Second):
		}
	})
}

func (s *server) stopCore() {
	s.mu.Lock()
	rn := s.running
	s.mu.Unlock()
	if rn != nil {
		s.stopRun(rn)
	}
}

// ---------------------------------------------------------------- start at logon

// launchAll starts NextBox in every session whose user asked for it.
func (s *server) launchAll() {
	var info *windows.WTS_SESSION_INFO
	var n uint32
	if windows.WTSEnumerateSessions(0, 0, 1, &info, &n) != nil {
		return
	}
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(info)))
	for _, si := range unsafe.Slice(info, n) {
		if si.State == windows.WTSActive {
			s.launch(si.SessionID)
		}
	}
}

// launch starts NextBox (minimised to the tray) for the user of a session
// who asked for it, as that user, once the desktop is ready.
func (s *server) launch(session uint32) {
	var t windows.Token
	if err := windows.WTSQueryUserToken(session, &t); err != nil {
		return // no user logged on there
	}
	defer t.Close()
	sid, err := tokenSID(t)
	if err != nil {
		return
	}
	s.mu.Lock()
	path := ""
	if u := s.st.Users[sid]; u != nil {
		path = u.Autostart
	}
	logon := logonID(t)
	if path == "" || s.launched[logon] {
		s.mu.Unlock()
		return
	}
	s.launched[logon] = true
	s.mu.Unlock()

	// the tray needs the taskbar: wait for Explorer in that session
	for i := 0; i < 120 && !explorerIn(session); i++ {
		time.Sleep(time.Second)
	}
	if err := startAs(t, path, "--minimized"); err != nil {
		logf("start %s: %v", path, err)
		s.mu.Lock()
		delete(s.launched, logon)
		s.mu.Unlock()
		return
	}
	logf("started %s in session %d", path, session)
}

func explorerIn(session uint32) bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return true
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), "explorer.exe") {
			var sess uint32
			if windows.ProcessIdToSessionId(e.ProcessID, &sess) == nil && sess == session {
				return true
			}
		}
	}
	return false
}

// startAs starts a program as the user of token, on their desktop.
func startAs(t windows.Token, path, args string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, t, false); err != nil {
		return err
	}
	defer windows.DestroyEnvironmentBlock(env)
	cmd, _ := windows.UTF16PtrFromString(fmt.Sprintf(`"%s" %s`, path, args))
	app, _ := windows.UTF16PtrFromString(path)
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(path))
	desktop, _ := windows.UTF16PtrFromString(`winsta0\default`)
	si := windows.StartupInfo{Desktop: desktop}
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation
	if err := windows.CreateProcessAsUser(t, app, cmd, nil, nil, false,
		windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NEW_PROCESS_GROUP, env, dir, &si, &pi); err != nil {
		return err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)
	return nil
}

var procGetTickCount64 = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetTickCount64")

func uptime() time.Duration {
	ms, _, _ := procGetTickCount64.Call()
	return time.Duration(ms) * time.Millisecond
}

// ---------------------------------------------------------------- log

var (
	logMu  sync.Mutex
	logger *log.Logger
)

// logf writes to Dir()\service.log (kept small: started anew past 1 MB).
func logf(format string, args ...any) {
	logMu.Lock()
	defer logMu.Unlock()
	if logger == nil {
		p := filepath.Join(Dir(), "service.log")
		if st, err := os.Stat(p); err == nil && st.Size() > 1<<20 {
			os.Remove(p)
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		logger = log.New(f, "", log.LstdFlags)
	}
	logger.Printf(format, args...)
}

// logonID identifies the logon a token belongs to (TOKEN_STATISTICS'
// AuthenticationId): a session number is used again after a log off, a
// logon ID is not.
func logonID(t windows.Token) uint64 {
	var buf [128]byte
	var n uint32
	if windows.GetTokenInformation(t, windows.TokenStatistics, &buf[0], uint32(len(buf)), &n) != nil {
		return 0
	}
	// TokenId (LUID, 8 bytes), then AuthenticationId (LUID)
	return uint64(binary.LittleEndian.Uint32(buf[8:12])) | uint64(binary.LittleEndian.Uint32(buf[12:16]))<<32
}
