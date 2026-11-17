package service

import (
	"path/filepath"
	"testing"
)

// The config the service runs as LocalSystem may not read or write files of
// the sender's choosing.
func TestSanitize(t *testing.T) {
	work, sets := `C:\svc\run`, `C:\svc\run\rule-set`
	ok := func() map[string]any {
		return map[string]any{
			"log":      map[string]any{"level": "info", "output": `C:\Windows\x.log`},
			"services": []any{map[string]any{"type": "ssm-api"}},
			"route": map[string]any{"rule_set": []any{
				map[string]any{"type": "local", "tag": "geoip-cn", "format": "binary", "path": `D:\NextBox\config\rule-set\geoip-cn.srs`},
			}},
			"experimental": map[string]any{
				"clash_api":  map[string]any{"external_controller": "127.0.0.1:9090", "secret": "x", "external_ui": `C:\Windows`},
				"cache_file": map[string]any{"enabled": true, "path": `C:\Windows\System32\x.db`},
			},
			"outbounds": []any{map[string]any{"type": "vless", "transport": map[string]any{"type": "ws", "path": "/ws"}}},
		}
	}
	cfg := ok()
	names, err := Sanitize(cfg, work, sets)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "geoip-cn.srs" {
		t.Fatalf("rule-sets = %v", names)
	}
	if _, ok := cfg["services"]; ok {
		t.Error("services kept")
	}
	if _, ok := cfg["log"].(map[string]any)["output"]; ok {
		t.Error("log output kept")
	}
	ex := cfg["experimental"].(map[string]any)
	if ex["cache_file"].(map[string]any)["path"] != filepath.Join(work, "cache.db") {
		t.Errorf("cache file = %v", ex["cache_file"])
	}
	if _, ok := ex["clash_api"].(map[string]any)["external_ui"]; ok {
		t.Error("external_ui kept")
	}
	rs := cfg["route"].(map[string]any)["rule_set"].([]any)[0].(map[string]any)
	if rs["path"] != filepath.Join(sets, "geoip-cn.srs") {
		t.Errorf("rule-set path = %v", rs["path"])
	}

	bad := []func(map[string]any){
		func(c map[string]any) {
			c["experimental"].(map[string]any)["clash_api"].(map[string]any)["external_controller"] = "0.0.0.0:9090"
		},
		func(c map[string]any) {
			c["route"].(map[string]any)["rule_set"] = []any{map[string]any{"type": "remote", "url": "https://x"}}
		},
		func(c map[string]any) {
			c["route"].(map[string]any)["rule_set"] = []any{map[string]any{"type": "local", "path": `C:\Windows\System32\config\SAM`}}
		},
		func(c map[string]any) {
			c["outbounds"] = []any{map[string]any{"type": "ssh", "private_key_path": `C:\Users\a\.ssh\id_rsa`}}
		},
		func(c map[string]any) {
			c["outbounds"] = []any{map[string]any{"type": "trojan", "tls": map[string]any{"certificate_path": `C:\x.pem`}}}
		},
		func(c map[string]any) {
			c["dns"] = map[string]any{"servers": []any{map[string]any{"type": "hosts", "path": []any{`C:\x`}}}}
		},
	}
	for i, f := range bad {
		c := ok()
		f(c)
		if _, err := Sanitize(c, work, sets); err == nil {
			t.Errorf("bad config %d accepted", i)
		}
	}
}
