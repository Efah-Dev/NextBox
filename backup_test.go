package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"nextbox/internal/store"
	"nextbox/internal/vault"
)

func writeFile(t *testing.T, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadBackup(t *testing.T) {
	doc := []byte(`{"app":"NextBox","version":1,"profiles":{"groups":[],"profiles":[]},"settings":{"routeMode":"rule","proxyRules":"google.com","mixedPort":7890}}`)

	// an Android backup (version 1), plain and encrypted
	d, enc, err := readBackup(writeFile(t, "a.json", doc), "")
	if err != nil || enc || d.Platform != "android" {
		t.Fatalf("plain: %+v %v %v", d, enc, err)
	}
	sealed, _ := vault.Encrypt(doc, "secret1")
	p := writeFile(t, "b.json", sealed)
	if _, enc, err := readBackup(p, ""); err != nil || !enc {
		t.Errorf("encrypted without password: %v %v", enc, err)
	}
	if _, _, err := readBackup(p, "wrong"); err == nil {
		t.Error("wrong password accepted")
	}
	if d, _, err := readBackup(p, "secret1"); err != nil || len(d.Settings) == 0 {
		t.Errorf("decrypt: %v", err)
	}
	// a bare profiles.json
	if d, _, err := readBackup(writeFile(t, "c.json", []byte(`{"nextId":3,"groups":[],"profiles":[]}`)), ""); err != nil || len(d.Profiles) == 0 {
		t.Errorf("bare profiles.json: %v", err)
	}
	if _, _, err := readBackup(writeFile(t, "d.json", []byte(`{"hello":1}`)), ""); err == nil {
		t.Error("random JSON accepted")
	}
}

// ownRules leaves out the places of ad blocking and the scenes.
func ownRules(rules []store.Rule) []store.Rule {
	var out []store.Rule
	for _, r := range rules {
		if !r.IsStep() {
			out = append(out, r)
		}
	}
	return out
}

// A backup made by NextBox for Android (v1.1): Android-only rules and
// preferences come through, the home page mode stays.
func TestAndroidBackup(t *testing.T) {
	doc := []byte(`{"app":"NextBox","version":2,"platform":"android",
"profiles":{"nextId":3,"groups":[{"id":1,"name":"g"}],"profiles":[{"id":2,"groupId":1,"name":"n","type":"naive","outbound":{"type":"naive","server":"h.example.com","server_port":443,"username":"u","password":"p","tls":{"enabled":true}}}]},
"settings":{"routeMode":"rule","blockAds":true,"lightLevel":0,"mixedPort":7890,
"rules":[{"kind":"package","values":["com.example.app"],"target":"direct"},{"kind":"suffix","values":["example.org"],"target":"proxy"}],
"scenes":[{"id":"cn","enabled":false,"target":"direct"}]},
"android":{"perAppMode":"exclude","perAppList":["com.example.bank"]}}`)
	sealed, _ := vault.Encrypt(doc, "secret1")
	p := writeFile(t, "android.json", sealed)
	a := &App{}
	info, err := a.InspectBackup(p, "secret1")
	if err != nil || info.Platform != "android" || info.Rules != 2 || info.AndroidRules != 1 || !info.HasNodes {
		t.Fatalf("inspect: %+v %v", info, err)
	}
	d, _, err := readBackup(p, "secret1")
	if err != nil || string(d.Android) == "" {
		t.Fatalf("android preferences lost: %v", err)
	}

	cur := store.DefaultSettings()
	cur.RouteMode = "global"
	keys := append(append(append([]string{}, ruleSettings...), portableSettings...), appearanceSettings...)
	keys = slices.DeleteFunc(keys, func(k string) bool { return k == "routeMode" })
	next, err := mergeSettings(cur, d.Settings, keys, true)
	if err != nil {
		t.Fatal(err)
	}
	next.Normalize()
	if next.RouteMode != "global" || next.MixedPort != cur.MixedPort {
		t.Errorf("mode %s port %d", next.RouteMode, next.MixedPort)
	}
	own := ownRules(next.Rules)
	if len(own) != 2 || own[0].Kind != "package" {
		t.Errorf("rules: %+v", next.Rules)
	}
	// a level of the old scale is converted
	if next.LightLevel != store.DefaultLightLevel {
		t.Errorf("light level %d", next.LightLevel)
	}
}

func TestMergeSettings(t *testing.T) {
	cur := store.DefaultSettings()
	cur.MixedPort, cur.Theme = 2030, "light"
	imported := json.RawMessage(`{"routeMode":"rule","proxyRules":"google.com","mixedPort":7890,"theme":"dark","tunEnabled":true}`)

	// from Android: rules, the look and the portable settings only
	next, err := mergeSettings(cur, imported, importKeys(backupDoc{Platform: "android", Settings: imported}, BackupOptions{Rules: true, Appearance: true, Settings: true}), true)
	if err != nil {
		t.Fatal(err)
	}
	next.Normalize()
	if next.MixedPort != 2030 || next.TunEnabled || next.Theme != "dark" || next.RouteMode != cur.RouteMode { // the mode stays this device's
		t.Errorf("cross-platform merge: port %d tun %v theme %s mode %s", next.MixedPort, next.TunEnabled, next.Theme, next.RouteMode)
	}
	if r := ownRules(next.Rules); len(r) != 1 || r[0].Values[0] != "google.com" {
		t.Errorf("old rule text not converted: %+v", next.Rules)
	}

	// same platform, everything but the rules
	next, _ = mergeSettings(cur, imported, nil, false)
	if next.MixedPort != 7890 || next.RouteMode != cur.RouteMode {
		t.Errorf("same platform: port %d mode %s", next.MixedPort, next.RouteMode)
	}

	s := store.DefaultSettings()
	s.Rules = []store.Rule{{Kind: "suffix", Values: []string{"a.com"}, Target: store.PolicyTarget(7)}, {Kind: "suffix", Values: []string{"b.com"}, Target: store.PolicyTarget(8)}}
	remapPolicies(&s, map[int64]int64{7: 42})
	if s.Rules[0].Target != store.PolicyTarget(42) || s.Rules[1].Target != store.TargetProxy {
		t.Errorf("remap: %+v", s.Rules)
	}
}

// The look of the app is a part of its own: exported and imported alone,
// and taken from an older backup that has it among the other settings.
func TestAppearancePart(t *testing.T) {
	s := store.DefaultSettings()
	s.Theme, s.AccentColor, s.RecentColors, s.MixedPort = "dark", "#123456", []string{"#123456"}, 7890
	raw, _ := json.Marshal(s)
	look, err := keepParts(raw, BackupOptions{Appearance: true})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(look, &m)
	if _, ok := m["theme"]; !ok || len(m) != len(appearanceSettings) {
		t.Fatalf("appearance part: %s", look)
	}

	cur := store.DefaultSettings()
	cur.MixedPort = 2030
	for _, platform := range []string{backupPlatform, "android"} {
		// an older backup: the look among the other settings
		doc := backupDoc{Platform: platform, Settings: raw}
		next, err := mergeSettings(cur, raw, importKeys(doc, BackupOptions{Appearance: true}), false)
		if err != nil {
			t.Fatal(err)
		}
		if next.Theme != "dark" || next.AccentColor != "#123456" || len(next.RecentColors) != 1 || next.MixedPort != 2030 {
			t.Errorf("%s: theme %s accent %s recent %v port %d", platform, next.Theme, next.AccentColor, next.RecentColors, next.MixedPort)
		}
		// the other settings alone leave the look as it is
		next, _ = mergeSettings(cur, raw, importKeys(doc, BackupOptions{Settings: true}), false)
		if next.Theme != cur.Theme || next.AccentColor != cur.AccentColor {
			t.Errorf("%s: other settings changed the look", platform)
		}
	}
}
