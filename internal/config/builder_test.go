package config

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nextbox/internal/link"
	"nextbox/internal/ruleset"
	"nextbox/internal/store"
)

var sampleLinks = []string{
	"vless://11111111-2222-3333-4444-555555555555@example.com:443?security=reality&sni=www.microsoft.com&fp=chrome&pbk=jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0&sid=6ba85179e30d4fc2&flow=xtls-rprx-vision&type=tcp#reality",
	"vmess://eyJ2IjoiMiIsInBzIjoidm1lc3Mtd3MiLCJhZGQiOiJleGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjExMTExMTExLTIyMjItMzMzMy00NDQ0LTU1NTU1NTU1NTU1NSIsImFpZCI6IjAiLCJuZXQiOiJ3cyIsInBhdGgiOiIvd3M/ZWQ9MjA0OCIsImhvc3QiOiJleGFtcGxlLmNvbSIsInRscyI6InRscyJ9",
	"trojan://pass@example.com:443?type=grpc&serviceName=svc#trojan-grpc",
	"ss://YWVzLTI1Ni1nY206cGFzcw@1.2.3.4:8388#ss",
	"hysteria2://pass@example.com:443?sni=example.com&obfs=salamander&obfs-password=x&mport=20000-30000#hy2",
	"tuic://11111111-2222-3333-4444-555555555555:pass@example.com:443?congestion_control=bbr#tuic",
	"anytls://pass@example.com:443?sni=example.com#anytls",
	"socks5://user:pw@127.0.0.1:1080#socks",
}

// singBox returns the core built by scripts/build-core.ps1, or skips the test.
func singBox(t *testing.T) string {
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

func check(t *testing.T, bin string, cfg map[string]any) {
	t.Helper()
	b, _ := json.MarshalIndent(cfg, "", "  ")
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, b, 0o644)
	out, err := exec.Command(bin, "check", "-c", path).CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check failed: %v\n%s\n--- config ---\n%s", err, out, b)
	}
}

func sampleProfiles(t *testing.T) []store.Profile {
	var list []store.Profile
	for i, l := range sampleLinks {
		n, err := link.Parse(l)
		if err != nil {
			t.Fatalf("parse %q: %v", l, err)
		}
		// round-trip through JSON like the store does
		b, _ := json.Marshal(n.Outbound)
		var ob map[string]any
		json.Unmarshal(b, &ob)
		list = append(list, store.Profile{ID: int64(i + 1), Name: n.Name, Type: n.Type(), Outbound: ob})
	}
	return list
}

func TestMainConfigAccepted(t *testing.T) {
	bin := singBox(t)
	ruleDir := t.TempDir()
	// rule mode uses the bundled rule-sets, as with the default local mirror
	for _, name := range []string{"geosite-cn", "geoip-cn"} {
		if err := ruleset.Install(name, filepath.Join(ruleDir, name+".srs")); err != nil {
			t.Fatal(err)
		}
	}
	profiles := sampleProfiles(t)
	for _, mode := range []string{"rule", "global", "direct"} {
		for _, p := range profiles {
			s := store.DefaultSettings()
			s.RouteMode = mode
			s.TunEnabled = true
			s.ProxyRules = "google.com\nkeyword:youtube\n8.8.8.8\nprocess:chrome.exe"
			s.DirectRules = "domain:example.cn\n2001:db8::/32"
			s.BlockRules = "regex:^ads\\."
			node := p
			cfg, err := Build(Input{Settings: s, Node: &node, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: ruleDir})
			if err != nil {
				t.Fatalf("%s/%s: %v", mode, p.Name, err)
			}
			t.Run(mode+"/"+p.Name, func(t *testing.T) { check(t, bin, cfg) })
		}
	}
}

// A whole group is loaded behind the "proxy" selector, so that the node can be
// switched without restarting the core.
func TestGroupConfigAccepted(t *testing.T) {
	bin := singBox(t)
	profiles := sampleProfiles(t)
	// same name twice, an empty name and a name that is a reserved tag
	profiles[1].Name, profiles[2].Name, profiles[3].Name = profiles[0].Name, "", TagProxy
	s := store.DefaultSettings()
	s.RouteMode = "global"
	in := Input{Settings: s, Node: &profiles[2], Nodes: profiles, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: t.TempDir()}
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	nodes := in.Switchable()
	if len(nodes) != len(profiles) || nodes[0].ID != profiles[2].ID {
		t.Fatalf("switchable nodes: got %d, first %d", len(nodes), nodes[0].ID)
	}
	seen := map[string]bool{TagProxy: true, TagDirect: true}
	for id, tag := range NodeTags(nodes) {
		if tag == "" || seen[tag] {
			t.Fatalf("node %d has the unusable tag %q", id, tag)
		}
		seen[tag] = true
	}
	selector := cfg["outbounds"].([]any)[0].(map[string]any)
	if selector["type"] != "selector" || selector["default"] != NodeTags(nodes)[profiles[2].ID] {
		t.Fatalf("unexpected selector: %v", selector)
	}
	check(t, bin, cfg)
}

// Chain proxies are expanded into one outbound per hop, each dialing through
// the one before, and the result is a config sing-box accepts.
func TestChainConfigAccepted(t *testing.T) {
	bin := singBox(t)
	profiles := sampleProfiles(t) // IDs 1..8: reality, vmess, trojan, ss, hy2, tuic, anytls, socks
	byID := map[int64]store.Profile{}
	for _, p := range profiles {
		byID[p.ID] = p
	}
	// a member that brings its own detour must not keep it inside a chain
	withDetour := byID[4]
	withDetour.Outbound = map[string]any{}
	for k, v := range byID[4].Outbound {
		withDetour.Outbound[k] = v
	}
	withDetour.Outbound["detour"] = "somewhere"
	byID[4] = withDetour
	member := func(id int64) (store.Profile, bool) { p, ok := byID[id]; return p, ok }

	chain := store.Profile{ID: 100, Name: "Tokyo 01", Type: store.TypeChain, Chain: []int64{4, 3, 1}}
	broken := store.Profile{ID: 101, Name: "broken", Type: store.TypeChain, Chain: []int64{4, 999}}
	nested := store.Profile{ID: 102, Name: "nested", Type: store.TypeChain, Chain: []int64{100, 1}}
	byID[100] = chain

	obs, err := Expand(chain, "Tokyo 01", member, map[string]bool{"Tokyo 01 / 1": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 3 {
		t.Fatalf("got %d outbounds", len(obs))
	}
	if _, ok := obs[0]["detour"]; ok {
		t.Errorf("first hop kept a detour: %v", obs[0]["detour"])
	}
	if obs[0]["tag"] == "Tokyo 01 / 1" {
		t.Error("hop tag collides with a tag already in use")
	}
	if obs[1]["detour"] != obs[0]["tag"] || obs[2]["detour"] != obs[1]["tag"] || obs[2]["tag"] != "Tokyo 01" {
		t.Errorf("hops not linked: %v / %v / %v", obs[0]["tag"], obs[1]["detour"], obs[2]["detour"])
	}
	for _, bad := range []store.Profile{broken, nested, {Name: "short", Type: store.TypeChain, Chain: []int64{1}}} {
		if _, err := Expand(bad, "x", member, nil); err == nil {
			t.Errorf("%s: no error", bad.Name)
		} else {
			t.Logf("%s: %v", bad.Name, err)
		}
	}

	// in the main config: the chain is selectable next to plain nodes, the
	// broken one is left out, and a chain node named like a plain one is fine
	s := store.DefaultSettings()
	s.RouteMode = "global"
	in := Input{Settings: s, Node: &chain, Nodes: []store.Profile{byID[1], broken, byID[5]}, Member: member,
		ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: t.TempDir()}
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	selector := cfg["outbounds"].([]any)[0].(map[string]any)
	if names := selector["outbounds"].([]string); len(names) != 3 {
		t.Errorf("selector outbounds: %v", names)
	}
	check(t, bin, cfg)

	// the selected node being broken is an error, not a silent fallback
	in.Node = &broken
	if _, err := Build(in); err == nil {
		t.Error("building with a broken selected chain did not fail")
	}

	test, err := BuildTest(s, []store.Profile{chain, byID[2]}, member, "127.0.0.1:9090", "x", 19091)
	if err != nil {
		t.Fatal(err)
	}
	check(t, bin, test)
}

// A rule-set that could not be fetched must not stop the core from starting:
// only the rules that need it are left out.
func TestSkipMissingRuleSets(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "geoip-cn.srs"), []byte("x"), 0o644)
	s := store.DefaultSettings()
	s.RouteMode = "rule"
	s.BlockAds = true
	node := sampleProfiles(t)[0]
	in := Input{Settings: s, Node: &node, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: dir}

	var missing *MissingRuleSetsError
	if _, err := Build(in); !errors.As(err, &missing) || len(missing.Names) != 2 {
		t.Fatalf("want missing geosite-cn and ads, got %v", err)
	}

	in.SkipMissingRuleSets = true
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(cfg)
	for _, name := range []string{"geosite-cn", "geosite-category-ads-all"} {
		if strings.Contains(string(b), name) {
			t.Errorf("config still references missing %s", name)
		}
	}
	if !strings.Contains(string(b), `"geoip-cn"`) {
		t.Error("available geoip-cn rule was dropped")
	}
}

func TestTestConfigAccepted(t *testing.T) {
	bin := singBox(t)
	cfg, err := BuildTest(store.DefaultSettings(), sampleProfiles(t), nil, "127.0.0.1:9090", "x", 19090)
	if err != nil {
		t.Fatal(err)
	}
	check(t, bin, cfg)
}
