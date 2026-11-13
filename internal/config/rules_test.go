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
	for _, name := range []string{"geosite-cn", "geoip-cn"} {
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
			outs = append(outs, ob)
		} else if a, _ := m["action"].(string); a == "reject" {
			outs = append(outs, "reject")
		}
	}
	want := []string{"direct", "日本 自动", "reality #101", "direct", "reject", "proxy", "direct"}
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
	if got.ProxyRules != "" || len(got.Rules) != 5 {
		t.Fatalf("rules = %+v", got.Rules)
	}
	want := []string{"keyword:ads>block", "suffix:google.com,youtube.com>proxy", "geosite:netflix>proxy", "cidr:1.2.3.4/32>direct", "process:WeChat.exe>direct"}
	for i, r := range got.Rules {
		if g := r.Kind + ":" + strings.Join(r.Values, ",") + ">" + r.Target; g != want[i] {
			t.Errorf("rule %d = %s, want %s", i, g, want[i])
		}
	}
}
