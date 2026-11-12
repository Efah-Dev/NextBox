package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"nextbox/internal/geo"
	"nextbox/internal/store"
)

// relay starts a sing-box instance that forwards whatever reaches its mixed
// port, and returns that port and its clash API for traffic counters.
func relay(t *testing.T, bin string) (int, *API) {
	t.Helper()
	port, _ := FreePort()
	apiPort, _ := FreePort()
	api := NewAPI("127.0.0.1:"+strconv.Itoa(apiPort), "s")
	p, err := Start(context.Background(), bin, t.TempDir(), "relay.json", map[string]any{
		"log":       map[string]any{"level": "error"},
		"inbounds":  []any{map[string]any{"type": "mixed", "listen": "127.0.0.1", "listen_port": port}},
		"outbounds": []any{map[string]any{"type": "direct"}},
		"experimental": map[string]any{
			"clash_api": map[string]any{"external_controller": api.Addr, "secret": api.Secret},
		},
	}, api, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Stop)
	return port, api
}

// TestChainCarriesTraffic tests a two-hop chain proxy end to end: the request
// has to pass through both relays, and the exit lookup goes along.
func TestChainCarriesTraffic(t *testing.T) {
	bin := coreBin(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/trace" {
			w.Write([]byte("ip=203.0.113.9\nloc=JP\n"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	oldTrace := geo.TraceURL
	geo.TraceURL = target.URL + "/trace"
	defer func() { geo.TraceURL = oldTrace }()

	entryPort, entryAPI := relay(t, bin)
	exitPort, exitAPI := relay(t, bin)
	members := map[int64]store.Profile{
		1: {ID: 1, Name: "entry", Type: "socks", Outbound: map[string]any{"type": "socks", "server": "127.0.0.1", "server_port": entryPort}},
		2: {ID: 2, Name: "exit", Type: "socks", Outbound: map[string]any{"type": "socks", "server": "127.0.0.1", "server_port": exitPort}},
	}
	member := func(id int64) (store.Profile, bool) {
		p, ok := members[id]
		return p, ok
	}
	nodes := []store.Profile{
		{ID: 10, Name: "chain", Type: store.TypeChain, Chain: []int64{1, 2}},
		{ID: 11, Name: "broken", Type: store.TypeChain, Chain: []int64{1, 99}},
	}
	s := store.DefaultSettings()
	s.TestURL = target.URL
	s.TestTimeout = 3000

	var mu sync.Mutex
	results := map[int64]LatencyResult{}
	TestURL(context.Background(), bin, t.TempDir(), s, nodes, Progress{
		Start:  func(int64) {},
		Result: func(r LatencyResult) { mu.Lock(); results[r.ID] = r; mu.Unlock() },
		Member: member,
		Locate: func(id int64) bool { return id == 10 },
	})

	r := results[10]
	if r.Latency <= 0 {
		t.Fatalf("chain: %+v", r)
	}
	if r.Exit == nil || r.Exit.Country != "JP" || r.Exit.IP != "203.0.113.9" {
		t.Fatalf("exit lookup: %+v", r.Exit)
	}
	for name, api := range map[string]*API{"entry": entryAPI, "exit": exitAPI} {
		c, err := api.Connections(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if c.UploadTotal == 0 || c.DownloadTotal == 0 {
			t.Errorf("no traffic went through the %s relay: up %d, down %d", name, c.UploadTotal, c.DownloadTotal)
		}
	}
	if b := results[11]; b.Latency != -1 || !strings.Contains(b.Error, "2") {
		t.Fatalf("chain with a missing member: %+v", b)
	} else {
		t.Logf("broken chain: %s", b.Error)
	}
}
