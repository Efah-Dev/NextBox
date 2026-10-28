package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"nextbox/internal/link"
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
	profiles := sampleProfiles(t)
	for _, mode := range []string{"global", "direct"} {
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

func TestTestConfigAccepted(t *testing.T) {
	bin := singBox(t)
	cfg, err := BuildTest(store.DefaultSettings(), sampleProfiles(t), "127.0.0.1:9090", "x", 19090)
	if err != nil {
		t.Fatal(err)
	}
	check(t, bin, cfg)
}
