package config

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nextbox/internal/ruleset"
	"nextbox/internal/store"
)

// A rule set with items of several kinds becomes a logical rule. With TUN and
// FakeIP on, its DNS rule also gets a query type, which sing-box takes only
// inside the sub-rules: on the logical rule itself the core refused to start
// ("dns.rules[n].query_type: unknown field").
func TestRuleSetConfigAccepted(t *testing.T) {
	bin := singBox(t)
	ruleDir := t.TempDir()
	for _, name := range []string{"geosite-cn", "geoip-cn", ForeignSites} {
		if err := ruleset.Install(name, filepath.Join(ruleDir, name+".srs")); err != nil {
			t.Fatal(err)
		}
	}
	node := sampleProfiles(t)[0]
	items := []store.RuleItem{
		{Kind: "suffix", Values: []string{"example.com"}},
		{Kind: "domain", Values: []string{"www.example.org"}},
		{Kind: "keyword", Values: []string{"tracker"}},
		{Kind: "regex", Values: []string{`^api\.`}},
		{Kind: "geosite", Values: []string{"cn"}},
		{Kind: "cidr", Values: []string{"203.0.113.0/24"}},
		{Kind: "geoip", Values: []string{"cn"}},
		{Kind: "process", Values: []string{"chrome.exe"}},
	}
	for _, target := range []string{"proxy", "direct", "block"} {
		for _, fakeIP := range []bool{true, false} {
			for _, n := range []int{2, 3, len(items)} {
				s := store.DefaultSettings()
				s.RouteMode = "rule"
				s.TunEnabled = true
				s.TunFakeIP = fakeIP
				s.Rules = append([]store.Rule{{Kind: store.KindSet, Name: "set", Target: target, Items: items[:n]}}, s.Rules...)
				cfg, err := Build(Input{Settings: s, Node: &node, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: ruleDir})
				if err != nil {
					t.Fatal(err)
				}
				name := target
				if fakeIP {
					name += "/fakeip"
				}
				t.Run(name+"/"+items[n-1].Kind, func(t *testing.T) { check(t, bin, cfg) })
			}
		}
	}
}

// Names are resolved for the IP steps except those of known foreign sites,
// with the DNS server the settings choose; the core accepts both.
func TestResolveStep(t *testing.T) {
	bin := singBox(t)
	ruleDir := t.TempDir()
	for _, name := range []string{"geosite-cn", "geoip-cn", ForeignSites} {
		if err := ruleset.Install(name, filepath.Join(ruleDir, name+".srs")); err != nil {
			t.Fatal(err)
		}
	}
	node := sampleProfiles(t)[0]
	for _, mode := range []string{store.ResolveDirect, store.ResolveRemote} {
		s := store.DefaultSettings()
		s.RouteMode = "rule" // "bypass mainland China" is on by default
		s.ResolveDNS = mode
		cfg, err := Build(Input{Settings: s, Node: &node, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: ruleDir})
		if err != nil {
			t.Fatal(err)
		}
		var resolve map[string]any
		for _, r := range cfg["route"].(map[string]any)["rules"].([]any) {
			if m := r.(map[string]any); m["action"] == "resolve" {
				resolve = m
			}
		}
		want := TagDNSDirect
		if mode == store.ResolveRemote {
			want = TagDNSRemote
		}
		if resolve == nil || resolve["server"] != want || resolve["invert"] != true || !slices.Equal(resolve["rule_set"].([]string), []string{ForeignSites, ChinaSites}) {
			t.Fatalf("%s: resolve step %v", mode, resolve)
		}
		t.Run(mode, func(t *testing.T) { check(t, bin, cfg) })
	}
	// the sites of the reported failure are in the list, so they keep their names
	for _, host := range []string{"www.google.com", "graph.facebook.com", "www.youtube.com"} {
		out, err := exec.Command(bin, "rule-set", "match", "-f", "binary", filepath.Join(ruleDir, ForeignSites+".srs"), host).CombinedOutput()
		if err != nil || !strings.Contains(string(out), "match rules") {
			t.Errorf("%s not in %s: %v %s", host, ForeignSites, err, out)
		}
	}
	// and a mainland site is not
	out, _ := exec.Command(bin, "rule-set", "match", "-f", "binary", filepath.Join(ruleDir, ForeignSites+".srs"), "www.baidu.com").CombinedOutput()
	if strings.Contains(string(out), "match rules") {
		t.Errorf("www.baidu.com is in %s", ForeignSites)
	}
}
