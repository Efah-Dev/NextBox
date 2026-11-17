//go:build windows

package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"nextbox/internal/core"
)

// A server in this process, on a pipe of its own: hello, autostart, and a
// core started through it that streams its log, answers its API and stops
// when asked.
func TestPipe(t *testing.T) {
	bin, _ := filepath.Abs(filepath.Join("..", "..", "build", "bin", "core", "sing-box.exe"))
	if _, err := os.Stat(bin); err != nil {
		t.Skip("sing-box core not built: " + bin)
	}
	dir := t.TempDir()
	if err := copyFile(bin, filepath.Join(dir, "sing-box.exe")); err != nil {
		t.Fatal(err)
	}
	sid, err := CurrentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	pipePath = `\\.\pipe\NextBoxServiceTest` + strconv.Itoa(os.Getpid())
	expectedServerPID = func() uint32 { return uint32(os.Getpid()) }
	s := &server{version: "test", dir: dir, st: &state{Users: map[string]*user{sid: {}}}, launched: map[uint64]bool{}}
	l, err := s.listen()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go s.serve(l)

	ctx := context.Background()
	st, err := Hello(ctx)
	if err != nil || st.Version != "test" || st.Protocol != Protocol || st.Running {
		t.Fatalf("hello: %+v %v", st, err)
	}
	exe := `C:\Apps\NextBox\NextBox.exe`
	if st, err := SetAutostart(ctx, exe, true); err != nil || st.Autostart != exe {
		t.Fatalf("autostart: %+v %v", st, err)
	}
	if _, err := SetAutostart(ctx, "relative.exe", true); err == nil {
		t.Error("relative autostart path accepted")
	}

	// a config the service has to rewrite: a rule-set elsewhere, the cache file
	port, _ := core.FreePort()
	apiPort, _ := core.FreePort()
	api := core.NewAPI("127.0.0.1:"+strconv.Itoa(apiPort), core.RandomSecret())
	cfg, _ := json.Marshal(map[string]any{
		"log":       map[string]any{"level": "info"},
		"inbounds":  []any{map[string]any{"type": "mixed", "tag": "in", "listen": "127.0.0.1", "listen_port": port}},
		"outbounds": []any{map[string]any{"type": "direct", "tag": "direct"}},
		"experimental": map[string]any{
			"clash_api":  map[string]any{"external_controller": api.Addr, "secret": api.Secret},
			"cache_file": map[string]any{"enabled": true, "path": `C:\Windows\Temp\nope.db`},
		},
	})
	var mu sync.Mutex
	var lines []string
	p, err := core.Attach(ctx, api, core.Options{}, func(line func(string)) (core.Backend, error) {
		return StartCore(ctx, cfg, nil, func(s string) {
			mu.Lock()
			lines = append(lines, s)
			mu.Unlock()
			line(s)
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := Hello(ctx); !st.Running {
		t.Error("not running")
	}
	if _, err := os.Stat(filepath.Join(dir, "run", "cache.db")); err != nil {
		t.Errorf("cache file not in the service folder: %v", err)
	}
	p.Stop()
	select {
	case <-p.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("core did not stop")
	}
	mu.Lock()
	log := strings.Join(lines, "\n")
	mu.Unlock()
	if !strings.Contains(log, "started") {
		t.Errorf("no core log came through:\n%s", log)
	}
	time.Sleep(200 * time.Millisecond)
	if st, _ := Hello(ctx); st.Running {
		t.Error("still running after stop")
	}
}
