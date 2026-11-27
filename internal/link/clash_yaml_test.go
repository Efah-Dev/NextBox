package link

import (
	"errors"
	"strings"
	"testing"
)

const clashHead = "mixed-port: 7890\nallow-lan: false\nmode: rule\ndns:\n  nameserver: ['https://dns.alidns.com/dns-query']\n"

// A Clash config that cannot be used is reported as such, never as its
// first line being an unsupported share link.
func TestParseClashErrors(t *testing.T) {
	cases := map[string]struct{ text, want string }{
		"broken yaml":     {clashHead + "proxies:\n  - {name: a, type: ss\n", "Clash 配置格式有误"},
		"no proxies":      {clashHead + "rules:\n  - MATCH,DIRECT\n", "Clash 配置里没有节点"},
		"all unsupported": {clashHead + "proxies:\n  - {name: r1, type: ssr, server: a.example, port: 1}\n", "r1：不支持的协议 ssr"},
		"providers only":  {clashHead + "proxy-providers:\n  p1: {type: http, url: 'https://x.example/p1'}\n", "proxy-providers"},
	}
	for name, c := range cases {
		_, _, err := ParseContent(c.text)
		if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "mixed-port") {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, c.want)
		}
	}
	if _, _, err := ParseContent(clashHead + "proxy-providers:\n  p1: {type: http, url: 'https://x.example/p1'}\n"); !errors.Is(err, ErrProvidersOnly) {
		t.Errorf("providers only: %v is not ErrProvidersOnly", err)
	}
}

// Repeated keys (common in generated configs) and the pre-1.0 "Proxy" key
// do not stop the import; hysteria (v1) nodes are read.
func TestParseClashLenient(t *testing.T) {
	text := clashHead + `proxies:
  - name: dup
    type: trojan
    server: a.example
    port: 443
    password: p
    udp: true
    udp: false
  - &base {name: hy, type: hysteria, server: b.example, port: 8443, auth-str: secret, up: "30 Mbps", down: 100, sni: b.example}
  - {<<: *base, name: hy2-alias}
Proxy:
  - {name: old, type: ss, server: c.example, port: 8388, cipher: aes-128-gcm, password: p}
`
	nodes, skipped, err := ParseContent(text)
	if err != nil || len(nodes) != 4 || len(skipped) != 0 {
		t.Fatalf("nodes %d skipped %v err %v", len(nodes), skipped, err)
	}
	hy := nodes[1].Outbound
	if hy["type"] != "hysteria" || hy["auth_str"] != "secret" || hy["up_mbps"] != 30 || hy["down_mbps"] != 100 || hy["tls"] == nil {
		t.Errorf("hysteria: %v", hy)
	}
	if nodes[2].Name != "hy2-alias" || nodes[2].Outbound["server"] != "b.example" {
		t.Errorf("merge key: %s %v", nodes[2].Name, nodes[2].Outbound)
	}
	if nodes[3].Name != "old" {
		t.Errorf("Proxy key: %s", nodes[3].Name)
	}
}

func TestClashProviders(t *testing.T) {
	text := clashHead + `proxy-providers:
  b: {type: http, url: 'https://x.example/b', filter: 'HK|JP', exclude-filter: '过期'}
  a: {type: http, url: 'https://x.example/a'}
  local: {type: file, path: ./p.yaml}
`
	got := ClashProviders(text)
	if len(got) != 2 || got[0].Name != "a" || got[1].Filter != "HK|JP" || got[1].Exclude != "过期" {
		t.Errorf("providers: %+v", got)
	}
	if ClashProviders("trojan://p@a.example:443#x\n") != nil {
		t.Error("share links have no providers")
	}
}
