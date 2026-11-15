package config

import (
	"reflect"
	"strings"
	"testing"

	"nextbox/internal/link"
	"nextbox/internal/store"
)

// steps lists the entries as "source:what>target".
func steps(es []Entry) []string {
	var out []string
	for _, e := range es {
		what := e.Scene
		if e.Source == "rule" {
			d, s, _, _ := e.Domains()
			what = strings.Join(append(d, s...), ",")
		}
		out = append(out, e.Source+":"+what+">"+e.Target)
	}
	return out
}

func TestEntriesFollowUserOrder(t *testing.T) {
	s := store.DefaultSettings()
	s.RouteMode = "rule"
	s.BlockAds = true
	s.Rules = []store.Rule{
		{Kind: "suffix", Values: []string{"github.com"}, Target: "direct"},
		{Kind: store.KindScene, Values: []string{"cn"}},
		{Kind: store.KindAds},
		{Kind: "suffix", Values: []string{"bilibili.com"}, Target: "proxy"},
	}
	s.Normalize()
	// the scenes not placed yet go last, the disabled ones are skipped
	got := steps(Entries(s))
	want := []string{"rule:github.com>direct", "scene:cn>direct", "ads:>block", "rule:bilibili.com>proxy"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	// indexes point into Settings.Rules, placeholders included
	for _, e := range Entries(s) {
		if e.Index < 0 || e.Index >= len(s.Rules) {
			t.Fatalf("index %d of %s", e.Index, e.Source)
		}
		if r := s.Rules[e.Index]; e.Source == "ads" && r.Kind != store.KindAds || e.Source == "scene" && r.Values[0] != e.Scene {
			t.Fatalf("entry %s points at %+v", e.Source, r)
		}
	}
	s.Scenes[1].Enabled = true // ai, placed last by Normalize
	if got := steps(Entries(s)); got[len(got)-1] != "scene:ai>proxy" {
		t.Fatalf("ai scene: %v", got)
	}
	// global mode: ad blocking only
	s.RouteMode = "global"
	if got := steps(Entries(s)); !reflect.DeepEqual(got, []string{"ads:>block"}) {
		t.Fatalf("global: %v", got)
	}
}

func TestPlaceStepsOnce(t *testing.T) {
	s := store.DefaultSettings()
	s.Rules = []store.Rule{{Kind: store.KindAds}, {Kind: store.KindAds}, {Kind: store.KindScene, Values: []string{"CN"}}, {Kind: store.KindScene, Values: []string{"nope"}}}
	s.Normalize()
	var kinds []string
	for _, r := range s.Rules {
		kinds = append(kinds, r.Kind+":"+strings.Join(r.Values, ""))
	}
	want := []string{"ads:", "scene:cn", "scene:ai", "scene:media", "scene:games"}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("rules = %v, want %v", kinds, want)
	}
}

func TestParseCIDRs(t *testing.T) {
	got, bad := ParseCIDRs("100.64.0.0/10\n# tailscale v6\nfd7a:115c:a1e0::/48\n192.0.2.10 # one host\n10.1.2.3/8\nnot an ip\n100.64.0.0/10")
	want := []string{"100.64.0.0/10", "fd7a:115c:a1e0::/48", "192.0.2.10/32", "10.0.0.0/8"}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(bad, []int{6}) {
		t.Fatalf("got %v bad %v", got, bad)
	}
}

func TestTunExclude(t *testing.T) {
	s := store.DefaultSettings()
	s.TunEnabled = true
	s.TunExclude = "100.64.0.0/10"
	node := store.Profile{ID: 1, Name: "n", Type: "socks", Outbound: map[string]any{"type": "socks", "server": "127.0.0.1", "server_port": 1080}}
	in := Input{Settings: s, Node: &node, Nodes: []store.Profile{node}, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: t.TempDir()}
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	var tun map[string]any
	for _, ib := range cfg["inbounds"].([]any) {
		if m := ib.(map[string]any); m["type"] == "tun" {
			tun = m
		}
	}
	if !reflect.DeepEqual(tun["route_exclude_address"], []string{"100.64.0.0/10"}) {
		t.Fatalf("tun = %v", tun)
	}
	rules := cfg["route"].(map[string]any)["rules"].([]any)
	found := false
	for _, r := range rules {
		if c, ok := r.(map[string]any)["ip_cidr"]; ok && reflect.DeepEqual(c, []string{"100.64.0.0/10"}) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no direct rule for the excluded range: %v", rules)
	}
}

// A Shadowsocks node with ShadowTLS in front (NextBox for Android writes
// these too) becomes two outbounds that sing-box accepts.
func TestShadowTLSFromClash(t *testing.T) {
	nodes, _, err := link.ParseContent(`proxies:
  - name: ss-stls
    type: ss
    server: 203.0.113.9
    port: 443
    cipher: 2022-blake3-aes-128-gcm
    password: "MTIzNDU2Nzg5MDEyMzQ1Ng=="
    plugin: shadow-tls
    client-fingerprint: chrome
    plugin-opts:
      host: www.microsoft.com
      password: stls-pass
      version: 3
`)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("parse: %v %d", err, len(nodes))
	}
	p := store.Profile{ID: 7, Name: nodes[0].Name, Type: "shadowsocks", Outbound: nodes[0].Outbound}
	s := store.DefaultSettings()
	in := Input{Settings: s, Node: &p, Nodes: []store.Profile{p}, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: t.TempDir()}
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]bool{}
	for _, ob := range cfg["outbounds"].([]any) {
		m := ob.(map[string]any)
		types[m["type"].(string)] = true
		if _, ok := m["shadowtls"]; ok {
			t.Fatalf("shadowtls field left in %v", m)
		}
	}
	if !types["shadowtls"] || !types["shadowsocks"] {
		t.Fatalf("outbounds: %v", types)
	}
	check(t, singBox(t), cfg)
}
