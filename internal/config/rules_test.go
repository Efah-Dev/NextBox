package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"nextbox/internal/ruleset"
	"nextbox/internal/store"
)

// Rules with their own targets, policy groups (auto and select), scenes and
// FakeIP all end up in a config the core accepts, in the right order.
func TestRulesAndPolicies(t *testing.T) {
	bin := singBox(t)
	ruleDir := t.TempDir()
	for _, name := range []string{"geosite-cn", "geoip-cn", ForeignSites} {
		if err := ruleset.Install(name, filepath.Join(ruleDir, name+".srs")); err != nil {
			t.Fatal(err)
		}
	}
	nodes := sampleProfiles(t)
	s := store.DefaultSettings()
	s.RouteMode = "rule"
	s.BlockAds = true // missing here: skipped
	s.TunEnabled, s.TunFakeIP = true, true
	s.Rules = []store.Rule{
		{Kind: "suffix", Values: []string{"youtube.com", "ytimg.com"}, Target: store.PolicyTarget(100)},
		{Kind: "suffix", Values: []string{"google.com"}, Target: store.PolicyTarget(101)},
		{Kind: "process", Values: []string{"WeChat.exe"}, Target: store.TargetDirect},
		{Kind: "keyword", Values: []string{"tracker"}, Target: store.TargetBlock},
		{Kind: "suffix", Values: []string{"azure.com"}, Target: store.PolicyTarget(999)}, // no such group
		{Kind: "suffix", Values: []string{"disabled.example"}, Target: store.TargetBlock, Disabled: true},
	}
	in := Input{
		Settings: s, Node: &nodes[0], Nodes: nodes[1:3], Member: nil,
		ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: ruleDir, SkipMissingRuleSets: true,
		Policies: []PolicyInput{
			{Policy: store.Policy{ID: 100, Name: "日本 自动", Mode: store.PolicyAuto}, Members: []store.Profile{nodes[1], nodes[4]}},
			{Policy: store.Policy{ID: 101, Name: "reality", Mode: store.PolicySelect, Selected: nodes[5].ID}, Members: []store.Profile{nodes[5], nodes[6]}},
		},
	}
	if got := UsedPolicies(s); len(got) != 3 {
		t.Errorf("UsedPolicies = %v", got)
	}
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	check(t, bin, cfg)

	b, _ := json.Marshal(cfg)
	js := string(b)
	for _, want := range []string{`"type":"urltest"`, `"tag":"reality #101"`, `"store_fakeip":true`, `"msftconnecttest.com"`} {
		if !strings.Contains(js, want) {
			t.Errorf("config lacks %s", want)
		}
	}
	if strings.Contains(js, "disabled.example") {
		t.Error("disabled rule was used")
	}

	// route rules: each user rule in order with its own outbound, then the
	// China bypass; the unknown group falls back to the node
	var outs []string
	for _, r := range cfg["route"].(map[string]any)["rules"].([]any) {
		m := r.(map[string]any)
		if ob, ok := m["outbound"].(string); ok {
			outs = append(outs, stepTarget(cfg, ob))
		} else if a, _ := m["action"].(string); a == "reject" || a == "resolve" {
			outs = append(outs, a)
		}
	}
	// the domains are resolved right before the China bypass, the first
	// step that matches IP addresses
	want := []string{"direct", "日本 自动", "reality #101", "direct", "reject", "proxy", "resolve", "direct"}
	if strings.Join(outs, ",") != strings.Join(want, ",") {
		t.Errorf("route outbounds = %v, want %v", outs, want)
	}

	// the selected node of a select group is its default
	for _, ob := range cfg["outbounds"].([]any) {
		m := ob.(map[string]any)
		if m["tag"] == "reality #101" && m["default"] != NodeTags([]store.Profile{nodes[5]})[nodes[5].ID] {
			t.Errorf("select group default = %v", m["default"])
		}
	}

	// global mode ignores the rules but keeps ad blocking
	s.RouteMode = "global"
	if e := Entries(s); len(e) != 1 || e[0].Source != "ads" {
		t.Errorf("global entries = %+v", e)
	}
	in.Settings, in.Policies = s, nil
	cfg, err = Build(in)
	if err != nil {
		t.Fatal(err)
	}
	check(t, bin, cfg)
}

func TestMigrateRules(t *testing.T) {
	s := store.DefaultSettings()
	s.BlockRules = "keyword:ads"
	s.ProxyRules = "google.com\nsuffix:youtube.com\ngeosite:netflix"
	s.DirectRules = "1.2.3.4\nprocess:WeChat.exe\n# comment"
	b, _ := json.Marshal(s)
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var loaded store.Settings
	json.Unmarshal(b, &loaded)
	if err := st.SaveSettings(loaded); err != nil {
		t.Fatal(err)
	}
	got := st.Settings()
	if got.ProxyRules != "" || len(got.Rules) != 10 {
		t.Fatalf("rules = %+v", got.Rules)
	}
	// ad blocking first and the scenes last, as v2.0 checked them
	want := []string{"ads:>", "keyword:ads>block", "suffix:google.com,youtube.com>proxy", "geosite:netflix>proxy", "cidr:1.2.3.4/32>direct", "process:WeChat.exe>direct",
		"scene:ai>", "scene:media>", "scene:games>", "scene:cn>"}
	for i, r := range got.Rules {
		if g := r.Kind + ":" + strings.Join(r.Values, ",") + ">" + r.Target; g != want[i] {
			t.Errorf("rule %d = %s, want %s", i, g, want[i])
		}
	}
}

// stepTarget follows a step's own selector (StepTag) to the outbound it
// stands for; other tags are returned as they are.
func stepTarget(cfg map[string]any, tag string) string {
	if _, _, _, ok := ParseStepTag(tag); !ok {
		return tag
	}
	for _, ob := range cfg["outbounds"].([]any) {
		m := ob.(map[string]any)
		if m["tag"] == tag {
			return m["outbounds"].([]string)[0]
		}
	}
	return "missing " + tag
}

// A rule set matches by all its items at once: domains, IP ranges,
// programs and rule-sets each in a part of a logical-or rule, with one
// target; the core accepts it, and its steps can be told apart by tag.
func TestRuleSet(t *testing.T) {
	bin := singBox(t)
	ruleDir := t.TempDir()
	for _, name := range []string{"geosite-cn", "geoip-cn", ForeignSites} {
		if err := ruleset.Install(name, filepath.Join(ruleDir, name+".srs")); err != nil {
			t.Fatal(err)
		}
	}
	nodes := sampleProfiles(t)
	s := store.DefaultSettings()
	s.RouteMode = "rule"
	s.Rules = []store.Rule{
		{Kind: store.KindSet, Name: "工作", Target: store.TargetDirect, Items: []store.RuleItem{
			{Kind: "suffix", Values: []string{"corp.example"}},
			{Kind: "cidr", Values: []string{"10.8.0.0/16"}},
			{Kind: "process", Values: []string{"Teams.exe"}},
			{Kind: "geosite", Values: []string{"cn"}},
		}},
		{Kind: store.KindSet, Name: "only domains", Target: store.TargetProxy, Items: []store.RuleItem{
			{Kind: "domain", Values: []string{"a.example"}}, {Kind: "keyword", Values: []string{"bee"}},
		}},
		{Kind: store.KindSet, Name: "off", Target: store.TargetBlock, Disabled: true, Items: []store.RuleItem{{Kind: "suffix", Values: []string{"off.example"}}}},
	}
	s.Normalize()
	cfg, err := Build(Input{Settings: s, Node: &nodes[0], ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: ruleDir})
	if err != nil {
		t.Fatal(err)
	}
	check(t, bin, cfg)
	indexOf := func(name string) int {
		for i, r := range s.Rules {
			if r.Name == name {
				return i
			}
		}
		return -1
	}
	rules := cfg["route"].(map[string]any)["rules"].([]any)
	var set, domains map[string]any
	for _, r := range rules {
		m := r.(map[string]any)
		switch m["outbound"] {
		case StepTag(Entry{Source: "rule", Index: indexOf("工作")}):
			set = m
		case StepTag(Entry{Source: "rule", Index: indexOf("only domains")}):
			domains = m
		}
	}
	if set == nil || set["type"] != "logical" || len(set["rules"].([]any)) != 3 {
		t.Fatalf("rule set = %v", set)
	}
	if domains == nil || domains["type"] != nil || domains["domain"] == nil || domains["domain_keyword"] == nil {
		t.Fatalf("domain-only set = %v", domains)
	}
	b, _ := json.Marshal(cfg)
	if strings.Contains(string(b), "off.example") {
		t.Error("disabled rule set was used")
	}
	if src, i, _, ok := ParseStepTag(StepTag(Entry{Source: "rule", Index: 0})); !ok || src != "rule" || i != 0 {
		t.Error("ParseStepTag")
	}
	if src, _, sc, ok := ParseStepTag(StepTag(Entry{Source: "scene", Scene: "cn"})); !ok || src != "scene" || sc != "cn" {
		t.Error("ParseStepTag scene")
	}
}
