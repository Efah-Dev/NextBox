package link

import (
	"reflect"
	"strings"
	"testing"
)

// A gost-plugin node goes out as an ss:// link and comes back the same.
func TestGostLinkRoundTrip(t *testing.T) {
	nodes, _, err := ParseContent("mixed-port: 7890\nproxies:\n" +
		"  - { name: 'US 03', type: ss, server: a.example, port: 80, cipher: 2022-blake3-aes-128-gcm, password: 'MTIzNDU2Nzg5MDEyMzQ1Ng==', plugin: gost-plugin, plugin-opts: { mode: websocket, host: cdn.example, path: /api/v1/stream, mux: true }, udp-over-tcp: true, udp-over-tcp-version: 2 }\n" +
		"  - { name: tls, type: ss, server: b.example, port: 443, cipher: aes-128-gcm, password: p, plugin: gost-plugin, plugin-opts: { mode: websocket, host: c.example, path: /w, tls: true, skip-cert-verify: true, mux: false } }\n")
	if err != nil || len(nodes) != 2 {
		t.Fatalf("parse: %v %d", err, len(nodes))
	}
	for _, n := range nodes {
		l, err := Export(n.Name, n.Outbound)
		if err != nil {
			t.Fatalf("%s: %v", n.Name, err)
		}
		if !strings.Contains(l, "plugin=gost-plugin") {
			t.Errorf("link %s", l)
		}
		back, err := Parse(l)
		if err != nil {
			t.Fatalf("parse %s: %v", l, err)
		}
		if back.Name != n.Name || !reflect.DeepEqual(back.Outbound, n.Outbound) {
			t.Errorf("round trip of %s:\n got  %v\n want %v", n.Name, back.Outbound, n.Outbound)
		}
	}
}

// Links from elsewhere: options left out take Clash.Meta's defaults.
func TestGostLinkDefaults(t *testing.T) {
	n, err := Parse("ss://YWVzLTEyOC1nY206cA@a.example:80/?plugin=gost-plugin%3Bmode%3Dwebsocket#x")
	if err != nil {
		t.Fatal(err)
	}
	g, _ := n.Outbound["gost"].(map[string]any)
	if g["host"] != "bing.com" || g["path"] != "/" || g["mux"] != true || n.Outbound["plugin"] != nil {
		t.Errorf("%v", n.Outbound)
	}
	if _, err := Parse("ss://YWVzLTEyOC1nY206cA@a.example:80/?plugin=gost-plugin%3Bmode%3Dquic#x"); err == nil {
		t.Error("gost-plugin quic mode accepted")
	}
}
