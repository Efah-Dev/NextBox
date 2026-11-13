package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestMergeSettings(t *testing.T) {
	cur := store.DefaultSettings()
	cur.MixedPort, cur.Theme = 2030, "light"
	imported := json.RawMessage(`{"routeMode":"rule","proxyRules":"google.com","mixedPort":7890,"theme":"dark","tunEnabled":true}`)

	// from Android: rules and portable settings only
	next, err := mergeSettings(cur, imported, append(append([]string{}, ruleSettings...), portableSettings...), true)
	if err != nil {
		t.Fatal(err)
	}
	next.Normalize()
	if next.MixedPort != 2030 || next.TunEnabled || next.Theme != "dark" || next.RouteMode != "rule" {
		t.Errorf("cross-platform merge: port %d tun %v theme %s mode %s", next.MixedPort, next.TunEnabled, next.Theme, next.RouteMode)
	}
	if len(next.Rules) != 1 || next.Rules[0].Values[0] != "google.com" {
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
