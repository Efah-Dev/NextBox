package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"nextbox/internal/store"
)

func coreBin(t *testing.T) string {
	name := "sing-box"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin, _ := filepath.Abs(filepath.Join("..", "..", "build", "bin", "core", name))
	if _, err := os.Stat(bin); err != nil {
		t.Skip("sing-box core not built: " + bin)
	}
	return bin
}

// TestURLMeasuresThroughNode runs the whole latency pipeline locally: a
// sing-box "server" exposes SOCKS, a node points at it, and the target is a
// local HTTP server. A second node points at a closed port and must fail.
func TestURLMeasuresThroughNode(t *testing.T) {
	bin := coreBin(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	serverPort, _ := FreePort()
	apiPort, _ := FreePort()
	api := NewAPI("127.0.0.1:"+strconv.Itoa(apiPort), "s")
	server, err := Start(context.Background(), bin, t.TempDir(), "server.json", map[string]any{
		"log":       map[string]any{"level": "error"},
		"inbounds":  []any{map[string]any{"type": "mixed", "listen": "127.0.0.1", "listen_port": serverPort}},
		"outbounds": []any{map[string]any{"type": "direct"}},
		"experimental": map[string]any{
			"clash_api": map[string]any{"external_controller": api.Addr, "secret": api.Secret},
		},
	}, api, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	closedPort, _ := FreePort()
	nodes := []store.Profile{
		{ID: 1, Outbound: map[string]any{"type": "socks", "server": "127.0.0.1", "server_port": serverPort}},
		{ID: 2, Outbound: map[string]any{"type": "socks", "server": "127.0.0.1", "server_port": closedPort}},
	}
	s := store.DefaultSettings()
	s.TestURL = target.URL
	s.TestTimeout = 3000

	var mu sync.Mutex
	results := map[int64]LatencyResult{}
	TestURL(context.Background(), bin, t.TempDir(), s, nodes, func(r LatencyResult) {
		mu.Lock()
		results[r.ID] = r
		mu.Unlock()
	})

	if r := results[1]; r.Latency <= 0 {
		t.Fatalf("working node: got %+v", r)
	} else {
		t.Logf("working node RTT: %d ms", r.Latency)
	}
	if r := results[2]; r.Latency != -1 || r.Error == "" {
		t.Fatalf("broken node: got %+v", r)
	} else {
		t.Logf("broken node error: %s", r.Error)
	}
}
