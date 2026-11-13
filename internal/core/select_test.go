package core

import (
	"context"
	"strconv"
	"testing"

	"nextbox/internal/config"
	"nextbox/internal/store"
)

// TestSelectProxy starts the core with two nodes and switches between them
// through the clash API, the way the app changes nodes without a restart.
func TestSelectProxy(t *testing.T) {
	bin := coreBin(t)
	apiPort, err := FreePort()
	if err != nil {
		t.Fatal(err)
	}
	mixedPort, err := FreePort()
	if err != nil {
		t.Fatal(err)
	}
	socks := func(port int) map[string]any {
		return map[string]any{"type": "socks", "server": "127.0.0.1", "server_port": port}
	}
	// two nodes with the same name must still get distinct tags
	nodes := []store.Profile{
		{ID: 1, Name: "node", Outbound: socks(1081)},
		{ID: 2, Name: "node", Outbound: socks(1082)},
	}
	s := store.DefaultSettings()
	s.RouteMode, s.MixedPort = "global", mixedPort
	api := NewAPI("127.0.0.1:"+strconv.Itoa(apiPort), RandomSecret())
	in := config.Input{Settings: s, Node: &nodes[0], Nodes: nodes, ClashAPI: api.Addr, Secret: api.Secret, RuleSetDir: t.TempDir()}
	cfg, err := config.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p, err := Start(ctx, bin, t.TempDir(), cfg, api, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	tags := config.NodeTags(in.Switchable())
	if tags[1] == tags[2] {
		t.Fatalf("duplicate tags: %v", tags)
	}
	for _, id := range []int64{1, 2, 1} {
		if err := api.SelectProxy(ctx, config.TagProxy, tags[id]); err != nil {
			t.Fatalf("select node %d: %v", id, err)
		}
		now, err := api.SelectedProxy(ctx, config.TagProxy)
		if err != nil || now != tags[id] {
			t.Fatalf("selected = %q (%v), want %q", now, err, tags[id])
		}
	}
	if err := api.SelectProxy(ctx, config.TagProxy, "no such node"); err == nil {
		t.Fatal("selecting an unknown node did not fail")
	}
}
