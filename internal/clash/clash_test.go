package clash

import (
	"encoding/json"
	"strings"
	"testing"

	"nextbox/internal/store"
)

const sample = `
proxies:
  - {name: "HK 01", type: ss, server: a.example, port: 1, cipher: aes-128-gcm, password: x}
  - {name: "HK 02", type: ss, server: b.example, port: 1, cipher: aes-128-gcm, password: x}
  - {name: "JP 01", type: ss, server: c.example, port: 1, cipher: aes-128-gcm, password: x}
proxy-groups:
  - {name: Proxy, type: select, proxies: [Auto, HK, "JP 01", DIRECT]}
  - {name: Auto, type: url-test, include-all: true}
  - {name: HK, type: select, include-all: true, filter: "HK"}
  - {name: Empty, type: select, proxies: [DIRECT]}
rule-providers:
  ads: {type: http, behavior: domain, url: "https://example.com/ads.yaml", format: yaml}
  bin: {type: http, behavior: domain, url: "https://example.com/x.mrs", format: mrs}
rules:
  - DOMAIN-SUFFIX,google.com,Proxy
  - DOMAIN,www.example.org,HK
  - DOMAIN-KEYWORD,tracker,REJECT
  - GEOIP,LAN,DIRECT
  - GEOIP,CN,DIRECT
  - IP-CIDR,203.0.113.0/24,Proxy,no-resolve
  - RULE-SET,ads,REJECT
  - RULE-SET,bin,REJECT
  - DST-PORT,123,DIRECT
  - PROCESS-NAME,game.exe,JP 01
  - MATCH,Proxy
dns:
  nameserver-policy:
    "+.netflix.com,+.nflxvideo.net": 10.8.0.53
    "geosite:cn": 223.5.5.5
    "corp.example": [https://dns.example.org/dns-query]
hosts:
  nas.home.arpa: 192.168.1.20
`

func TestParse(t *testing.T) {
	ex, err := Parse(sample)
	if err != nil || ex == nil {
		t.Fatal(ex, err)
	}
	if len(ex.Groups) != 4 {
		t.Errorf("groups %v", ex.Groups)
	}
	// LAN and MATCH do not count; DST-PORT and the mrs provider are skipped
	if ex.RuleTotal != 9 || len(ex.Rules) != 7 || len(ex.Skipped) != 2 {
		t.Errorf("total %d rules %v skipped %v", ex.RuleTotal, ex.Rules, ex.Skipped)
	}
	if r := ex.Rules[3]; r.Kind != "geoip" || r.Value != "cn" {
		t.Errorf("GEOIP rule %v", r)
	}
	if r := ex.Rules[5]; r.Kind != store.KindURL || r.Value != "https://example.com/ads.yaml" {
		t.Errorf("provider rule %v", r)
	}
	j, _ := json.Marshal(ex.DNS)
	for _, want := range []string{`"domains":["netflix.com","nflxvideo.net"],"server":"10.8.0.53","viaProxy":true`, `"server":"https://dns.example.org/dns-query"`, `"domains":["nas.home.arpa"],"ips":["192.168.1.20"]`} {
		if !strings.Contains(string(j), want) {
			t.Errorf("DNS %s lacks %s", j, want)
		}
	}
	if strings.Contains(string(j), "223.5.5.5") {
		t.Errorf("geosite key kept: %s", j)
	}
	if ex, _ := Parse("proxies:\n  - {name: a, type: ss}\n"); ex != nil {
		t.Errorf("proxies only: %v", ex)
	}
}

func TestApply(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	g, _ := st.AddGroup(store.Group{Name: "Sub", URL: "https://example.com/sub"})
	var list []store.Profile
	for _, n := range []string{"HK 01", "HK 02", "JP 01"} {
		list = append(list, store.Profile{Name: n, Type: "shadowsocks", Outbound: map[string]any{"type": "shadowsocks"}})
	}
	if _, err := st.AddProfiles(g.ID, list); err != nil {
		t.Fatal(err)
	}
	ex, _ := Parse(sample)
	if p := ex.Preview(st.Profiles(g.ID)); len(p.Groups) != 3 || p.Sets != 5 || p.DNS != 3 {
		t.Errorf("preview %+v", p)
	}
	all := store.ClashImport{Groups: true, Rules: true, DNS: true}
	sum, err := Apply(st, ex, g, all)
	if err != nil || sum.Policies != 3 || sum.Sets != 5 || sum.DNS != 3 {
		t.Fatalf("%+v %v", sum, err)
	}
	pols := st.Policies()
	hk := pols[policyAt(pols, "HK")]
	if len(hk.Members) != 2 || hk.Mode != store.PolicySelect || pols[policyAt(pols, "Auto")].Mode != store.PolicyAuto {
		t.Errorf("policies %+v", pols)
	}
	s := st.Settings()
	cn := -1
	var sets []store.Rule
	for i, r := range s.Rules {
		if r.From == g.ID {
			sets = append(sets, r)
			if cn >= 0 {
				t.Errorf("rule set after the cn step")
			}
		}
		if r.Kind == store.KindScene && r.Values[0] == "cn" {
			cn = i
		}
	}
	if len(sets) != 5 || sets[0].Target != store.PolicyTarget(pols[policyAt(pols, "Proxy")].ID) || sets[0].Name != "Sub · Proxy" {
		t.Errorf("sets %+v", sets)
	}
	// a node as target goes through the current node
	if sets[len(sets)-1].Target != store.TargetProxy {
		t.Errorf("node target: %+v", sets[len(sets)-1])
	}

	// an update keeps the IDs of the policy groups and the place of the rule sets
	user := store.Rule{Kind: "suffix", Values: []string{"mine.example"}, Target: store.TargetDirect}
	s.Rules = append([]store.Rule{user}, s.Rules...)
	st.SaveSettings(s)
	before := pols[policyAt(pols, "HK")].ID
	if _, err := Apply(st, ex, g, store.ClashImport{Groups: true, Rules: true}); err != nil {
		t.Fatal(err)
	}
	pols = st.Policies()
	if pols[policyAt(pols, "HK")].ID != before {
		t.Errorf("policy ID changed")
	}
	s = st.Settings()
	cnAt := len(s.Rules) - 1
	if s.Rules[0].Values[0] != "mine.example" || s.Rules[cnAt].Values[0] != "cn" || s.Rules[cnAt-1].From != g.ID || s.Rules[cnAt-5].From != g.ID {
		t.Errorf("order after update: %+v", s.Rules)
	}
	if len(s.DNSOverrides) != 0 {
		t.Errorf("DNS kept although not chosen: %+v", s.DNSOverrides)
	}
	// taking nothing removes it all
	if _, err := Apply(st, ex, g, store.ClashImport{}); err != nil {
		t.Fatal(err)
	}
	for _, r := range st.Settings().Rules {
		if r.From != 0 {
			t.Errorf("left after removing: %+v", r)
		}
	}
	if len(st.Policies()) != 0 {
		t.Errorf("policies left: %+v", st.Policies())
	}
}

func policyAt(pols []store.Policy, name string) int {
	for i, p := range pols {
		if p.Name == name {
			return i
		}
	}
	return -1
}

func TestProviderToSource(t *testing.T) {
	yamlBody := "payload:\n  - '+.google.com'\n  - 'www.example.org'\n  - '.example.net'\n"
	j, err := ProviderToSource([]byte(yamlBody))
	if err != nil || string(j) != `{"rules":[{"domain":["www.example.org"]},{"domain_suffix":["google.com","example.net"]}],"version":2}` {
		t.Errorf("%s %v", j, err)
	}
	text := "# comment\nDOMAIN-SUFFIX,youtube.com\nIP-CIDR,1.2.3.0/24,no-resolve\nPROCESS-NAME,a.exe\n10.0.0.0/8\n8.8.8.8\n"
	j, err = ProviderToSource([]byte(text))
	if err != nil || string(j) != `{"rules":[{"domain_suffix":["youtube.com"]},{"ip_cidr":["1.2.3.0/24","10.0.0.0/8","8.8.8.8"]},{"process_name":["a.exe"]}],"version":2}` {
		t.Errorf("%s %v", j, err)
	}
	if _, err := ProviderToSource([]byte("MRS\x00\x01")); err == nil {
		t.Error("mrs accepted")
	}
}

func TestSubDNS(t *testing.T) {
	clashCfg := "proxies: []\ndns:\n  nameserver: [223.5.5.5, 'https://dns.airport.example/dns-query#PROXY']\n  proxy-server-nameserver: [system, 'tls://10.0.0.53']\n"
	ex, err := Parse(clashCfg)
	if err != nil || ex == nil || ex.SubDNS == nil {
		t.Fatal(ex, err)
	}
	if ex.SubDNS.Remote != "https://dns.airport.example/dns-query" || ex.SubDNS.Resolver != "tls://10.0.0.53" {
		t.Errorf("%+v", ex.SubDNS)
	}
	// only public servers: nothing to suggest
	if ex, _ := Parse("proxies: []\ndns:\n  nameserver: [8.8.8.8, 'https://dns.alidns.com/dns-query']\n"); ex != nil {
		t.Errorf("public DNS suggested: %+v", ex.SubDNS)
	}
	sb := `{"dns":{"servers":[{"type":"https","server":"dns.google"},{"type":"https","server":"dns.airport.example","detour":"proxy"},{"type":"udp","server":"10.0.0.53","server_port":53},{"type":"fakeip","tag":"f"}]},"outbounds":[]}`
	ex = ParseSingBox(sb)
	if ex == nil || ex.SubDNS.Remote != "https://dns.airport.example/dns-query" || ex.SubDNS.Resolver != "10.0.0.53:53" {
		t.Fatalf("%+v", ex)
	}
	if ParseSingBox(`{"dns":{"servers":[{"address":"local"},{"address":"https://1.1.1.1/dns-query"}]}}`) != nil {
		t.Error("local/public sing-box DNS suggested")
	}
}
