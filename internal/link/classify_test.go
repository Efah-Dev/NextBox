package link

import "testing"

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		in    string
		kind  string
		url   string
		name  string
		named bool
		proxy bool
	}{
		{"https://sub.example.com/api/v1/client/subscribe?token=abc", "subscription", "https://sub.example.com/api/v1/client/subscribe?token=abc", "sub.example.com", false, false},
		{"https://www.example.com/s#My%20Air", "subscription", "https://www.example.com/s", "My Air", true, false},
		{"clash://install-config?url=https%3A%2F%2Fa.example%2Fs&name=Air", "subscription", "https://a.example/s", "Air", true, false},
		{"sub://aHR0cHM6Ly9hLmV4YW1wbGUvcw==#Rocket", "subscription", "https://a.example/s", "Rocket", true, false},
		{"http://user:pass@1.2.3.4:8080", "subscription", "http://user:pass@1.2.3.4:8080", "1.2.3.4", false, true},
		{"trojan://pw@h.example:443#T", "nodes", "", "", false, false},
		{"hello", "none", "", "", false, false},
		{"", "none", "", "", false, false},
	} {
		r := Classify(c.in)
		if r.Kind != c.kind || r.URL != c.url || r.Name != c.name || r.Named != c.named || r.LikelyProxy != c.proxy {
			t.Errorf("Classify(%q) = %+v", c.in, r)
		}
	}
}
