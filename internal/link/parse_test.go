package link

import (
	"strings"
	"testing"
)

func TestParseVLESSReality(t *testing.T) {
	cases := map[string]string{
		"3x-ui style":       "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?type=tcp&security=reality&pbk=jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0&fp=chrome&sni=www.microsoft.com&sid=6ba8&spx=%2F&flow=xtls-rprx-vision&encryption=none&headerType=none#HK-01",
		"name with % sign":  "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?security=reality&pbk=key&sni=a.com#香港 01 | 100%",
		"name with emoji":   "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?security=reality&pbk=key&sni=a.com#🇭🇰 Hong Kong",
		"camelCase params":  "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?security=reality&publicKey=key&shortId=ab&serverName=a.com&fingerprint=firefox",
		"no security param": "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?pbk=key&sni=a.com&flow=xtls-rprx-vision",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			n, err := Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			tls, _ := n.Outbound["tls"].(map[string]any)
			reality, _ := tls["reality"].(map[string]any)
			if reality == nil || reality["public_key"] == "" {
				t.Fatalf("reality not detected: %#v", n.Outbound)
			}
			if tls["server_name"] == nil {
				t.Fatalf("missing SNI: %#v", tls)
			}
			if _, ok := tls["utls"]; !ok {
				t.Fatalf("reality needs uTLS: %#v", tls)
			}
		})
	}
	if n, _ := Parse(cases["name with % sign"]); n.Name != "香港 01 | 100%" {
		t.Errorf("name = %q", n.Name)
	}
}

func TestParseUnsupportedExplains(t *testing.T) {
	for raw, want := range map[string]string{
		"vless://u@1.2.3.4:443?security=reality&pbk=k&type=xhttp#x":                   "XHTTP",
		"vless://u@1.2.3.4:443?encryption=mlkem768x25519plus.native.0rtt.abc&pbk=k#x": "encryption",
	} {
		_, err := Parse(raw)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want mention of %s", raw, err, want)
		}
	}
}

func TestParseContentReportsReasons(t *testing.T) {
	_, skipped, err := ParseContent("vless://u@1.2.3.4:443?type=xhttp#我的节点\n")
	if err == nil || len(skipped) != 1 || !strings.Contains(err.Error(), "我的节点") {
		t.Fatalf("err=%v skipped=%v", err, skipped)
	}
}

// A panel-generated link with an encoded space before the IP and a SNI that
// was turned into a Markdown link (reported by a user; IP and keys replaced).
func TestParseVLESSDirtyLink(t *testing.T) {
	raw := "vless://11111111-2222-3333-4444-555555555555@%20203.0.113.1:8443?encryption=none&flow=xtls-rprx-vision&security=reality" +
		"&sni=%5Bwww.apple.com%5D(https%3A%2F%2Fwww.apple.com)&fp=chrome&pbk=jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0&sid=6ba8&type=tcp#Reality_US"
	n, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.Outbound["server"] != "203.0.113.1" || n.Outbound["server_port"] != 8443 {
		t.Fatalf("server = %v:%v", n.Outbound["server"], n.Outbound["server_port"])
	}
	tls := n.Outbound["tls"].(map[string]any)
	if tls["server_name"] != "www.apple.com" {
		t.Fatalf("sni = %v", tls["server_name"])
	}
	if n.Name != "Reality_US" {
		t.Fatalf("name = %q", n.Name)
	}
}

// Names keep their spaces also in links without a query string, where the
// "#name" directly follows the server address.
func TestNameKeepsSpaces(t *testing.T) {
	for link, want := range map[string]string{
		"socks5://127.0.0.1:1080#HK-02%20IPLC":                     "HK-02 IPLC",
		"trojan://p@example.com:443#Tokyo%2002":                    "Tokyo 02",
		"ss://YWVzLTI1Ni1nY206cGFzcw@1.2.3.4:8388#US%20San%20Jose": "US San Jose",
		"trojan://p@%20example.com:443#A%20B":                      "A B",
	} {
		n, err := Parse(link)
		if err != nil {
			t.Fatalf("%s: %v", link, err)
		}
		if n.Name != want {
			t.Errorf("%s: name %q, want %q", link, n.Name, want)
		}
		if s, _ := n.Outbound["server"].(string); strings.Contains(s, " ") {
			t.Errorf("%s: server %q still has a space", link, s)
		}
	}
}
