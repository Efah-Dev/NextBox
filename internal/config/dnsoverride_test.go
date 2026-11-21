package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"nextbox/internal/ruleset"
	"nextbox/internal/store"
)

// DNS overrides: a server asked through the proxy, one asked directly and
// fixed addresses, with the mainland China bypass on, in TUN mode with and
// without FakeIP; the core accepts all of them.
func TestDNSOverridesAccepted(t *testing.T) {
	bin := singBox(t)
	ruleDir := t.TempDir()
	for _, name := range []string{"geosite-cn", "geoip-cn", ForeignSites} {
		if err := ruleset.Install(name, filepath.Join(ruleDir, name+".srs")); err != nil {
			t.Fatal(err)
		}
	}
	node := sampleProfiles(t)[0]
	for _, fakeIP := range []bool{false, true} {
		s := store.DefaultSettings()
		s.RouteMode = "rule"
		s.TunEnabled, s.TunFakeIP = true, fakeIP
		s.DNSOverrides = []store.DNSOverride{
			{Domains: []string{"Netflix.com", "*.nflxvideo.net", " "}, Server: "10.8.0.53", ViaProxy: true},
			{Domains: []string{"corp.example"}, Server: "https://dns.example.org/dns-query"},
			{Domains: []string{"nas.home.arpa"}, IPs: []string{"192.168.1.20", "not an ip"}},
			{Domains: []string{"off.example"}, Server: "1.1.1.1", Disabled: true},
		}
		cfg, err := Build(Input{Settings: s, Node: &node, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: ruleDir})
		if err != nil {
			t.Fatal(err)
		}
		j, _ := json.Marshal(cfg)
		text := string(j)
		for _, want := range []string{`"nflxvideo.net"`, `"dns-override-1"`, `"detour":"proxy"`, `"predefined":{"nas.home.arpa":["192.168.1.20"]}`, `"mode":"and"`} {
			if !strings.Contains(text, want) {
				t.Errorf("fakeIP=%v: config lacks %s", fakeIP, want)
			}
		}
		if strings.Contains(text, "off.example") || strings.Contains(text, "not an ip") {
			t.Errorf("fakeIP=%v: disabled override or bad IP in the config", fakeIP)
		}
		// the overrides come before every other DNS rule
		first := cfg["dns"].(map[string]any)["rules"].([]any)[0].(map[string]any)
		if first["server"] != TagDNSHosts {
			t.Errorf("fakeIP=%v: first DNS rule %v", fakeIP, first)
		}
		check(t, bin, cfg)
	}
}

func TestCleanDomains(t *testing.T) {
	got := cleanDomains([]string{" *.Example.COM ", ".a.org.", "example.com", ""})
	if strings.Join(got, ",") != "example.com,a.org" {
		t.Fatal(got)
	}
}

// A subscription's own DNS: its remote DNS replaces the global one while one
// of its nodes is connected, and its resolver looks up its nodes' server
// names; nodes of other groups keep the direct DNS. The core accepts it, in
// the main config and in the speed test config.
func TestSubscriptionDNS(t *testing.T) {
	bin := singBox(t)
	nodes := sampleProfiles(t)
	if len(nodes) < 2 {
		t.Skip("needs two sample nodes")
	}
	nodes[0].GroupID, nodes[1].GroupID = 7, 8
	nodes[0].Outbound["server"] = "node.airport.example"
	s := store.DefaultSettings()
	s.RouteMode = "global"
	gd := map[int64]store.GroupDNS{7: {Remote: "https://dns.airport.example/dns-query", Resolver: "tls://10.0.0.53"}}
	cfg, err := Build(Input{Settings: s, Node: &nodes[0], Nodes: []store.Profile{nodes[1]}, GroupDNS: gd, ClashAPI: "127.0.0.1:9090", Secret: "x"})
	if err != nil {
		t.Fatal(err)
	}
	j, _ := json.Marshal(cfg)
	text := string(j)
	for _, want := range []string{`"server":"dns.airport.example"`, `"tag":"dns-sub-7"`, `"domain_resolver":"dns-sub-7"`} {
		if !strings.Contains(text, want) {
			t.Errorf("config lacks %s", want)
		}
	}
	if strings.Count(text, `"domain_resolver":"dns-sub-7"`) != 1 {
		t.Errorf("the resolver is used beyond the subscription's node: %s", text)
	}
	check(t, bin, cfg)
	test, err := BuildTest(s, nodes[:2], nil, gd, "127.0.0.1:9090", "x", 19092)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := json.Marshal(test); !strings.Contains(string(b), `"domain_resolver":"dns-sub-7"`) {
		t.Error("speed test config without the subscription's resolver")
	}
	check(t, bin, test)
}
