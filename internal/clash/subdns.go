package clash

// A subscription's own DNS servers (订阅专属 DNS): the airport's DNS that a
// Clash config names in dns.nameserver / dns.proxy-server-nameserver, or a
// sing-box config in dns.servers. Well-known public DNS servers are left
// out: a config naming 8.8.8.8 is not asking for anything special.

import (
	"encoding/json"
	"net"
	"net/url"
	"strings"

	"nextbox/internal/store"
)

// publicDNS are the addresses and host names of well-known public DNS
// services.
var publicDNS = map[string]bool{
	"8.8.8.8": true, "8.8.4.4": true, "1.1.1.1": true, "1.0.0.1": true, "9.9.9.9": true, "149.112.112.112": true,
	"208.67.222.222": true, "208.67.220.220": true, "94.140.14.14": true, "94.140.15.15": true, "185.222.222.222": true,
	"223.5.5.5": true, "223.6.6.6": true, "119.29.29.29": true, "119.28.28.28": true, "180.76.76.76": true,
	"114.114.114.114": true, "114.114.115.115": true, "1.2.4.8": true, "210.2.4.8": true, "101.226.4.6": true,
	"2001:4860:4860::8888": true, "2001:4860:4860::8844": true, "2606:4700:4700::1111": true, "2606:4700:4700::1001": true,
	"dns.google": true, "dns.google.com": true, "cloudflare-dns.com": true, "one.one.one.one": true,
	"1dot1dot1dot1.cloudflare-dns.com": true, "security.cloudflare-dns.com": true, "dns.quad9.net": true,
	"dns.adguard.com": true, "dns.adguard-dns.com": true, "doh.opendns.com": true, "dns.alidns.com": true,
	"doh.pub": true, "dot.pub": true, "dns.pub": true, "doh.360.cn": true, "dot.360.cn": true, "dns.twnic.tw": true,
	"dns.nextdns.io": true, "doh.dns.sb": true, "dns.sb": true,
}

// usableServer turns a DNS server of a Clash config into the form NextBox
// takes (223.5.5.5, tls://, https:// ...), or "" when it has no counterpart
// (system, dhcp, rcode) or is a public one.
func usableServer(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "#"); i >= 0 {
		s = s[:i] // Clash options ("#PROXY", "#h3=true")
	}
	if s == "" || s == "system" || strings.HasPrefix(s, "dhcp://") || strings.HasPrefix(s, "rcode://") || strings.HasPrefix(s, "system://") {
		return ""
	}
	if publicDNS[strings.ToLower(dnsHost(s))] {
		return ""
	}
	return s
}

// dnsHost is the host of a DNS server address.
func dnsHost(s string) string {
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			return u.Hostname()
		}
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		return strings.Trim(h, "[]")
	}
	return strings.Trim(s, "[]")
}

// clashSubDNS picks the first usable servers of nameserver and
// proxy-server-nameserver; nil when there are none.
func clashSubDNS(nameservers, proxyServer []string) *store.GroupDNS {
	d := &store.GroupDNS{}
	for _, s := range nameservers {
		if d.Remote = usableServer(s); d.Remote != "" {
			break
		}
	}
	for _, s := range proxyServer {
		if d.Resolver = usableServer(s); d.Resolver != "" {
			break
		}
	}
	if d.Empty() {
		return nil
	}
	return d
}

// ParseSingBox reads the DNS servers of a sing-box config subscription
// (the only part besides its outbounds NextBox takes from one); nil when it
// has none worth taking. A server reached through an outbound (detour) is
// the remote DNS, a direct one resolves the node names.
func ParseSingBox(text string) *Extras {
	var cfg struct {
		DNS struct {
			Servers []map[string]any `json:"servers"`
		} `json:"dns"`
	}
	if json.Unmarshal([]byte(text), &cfg) != nil {
		return nil
	}
	d := &store.GroupDNS{}
	for _, srv := range cfg.DNS.Servers {
		addr := singBoxServer(srv)
		if addr == "" || publicDNS[strings.ToLower(dnsHost(addr))] {
			continue
		}
		if detour := str(srv["detour"]); detour != "" && detour != "direct" {
			if d.Remote == "" {
				d.Remote = addr
			}
		} else if d.Resolver == "" {
			d.Resolver = addr
		}
	}
	if d.Empty() {
		return nil
	}
	return &Extras{SubDNS: d}
}

// singBoxServer turns a DNS server of a sing-box config, new (type +
// server) or old (address) form, into NextBox's form; "" for local, fakeip,
// hosts and the like.
func singBoxServer(srv map[string]any) string {
	if a := str(srv["address"]); a != "" { // before 1.12
		if a == "local" || strings.HasPrefix(a, "dhcp://") || strings.HasPrefix(a, "fakeip") || strings.HasPrefix(a, "rcode://") {
			return ""
		}
		return a
	}
	host := str(srv["server"])
	if host == "" {
		return ""
	}
	if p := str(srv["server_port"]); p != "" && p != "0" {
		host = net.JoinHostPort(host, p)
	}
	switch str(srv["type"]) {
	case "udp", "":
		return host
	case "tcp", "tls", "quic":
		return str(srv["type"]) + "://" + host
	case "https", "h3":
		path := str(srv["path"])
		if path == "" {
			path = "/dns-query"
		}
		return str(srv["type"]) + "://" + host + path
	}
	return ""
}
