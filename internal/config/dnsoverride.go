package config

import (
	"fmt"
	"net"
	"strings"

	"nextbox/internal/i18n"
	"nextbox/internal/store"
)

// TagDNSHosts is the DNS server of the overrides with fixed addresses.
const TagDNSHosts = "dns-hosts"

// overrides is what Settings.DNSOverrides add to the config: DNS servers,
// DNS rules (before the others) and route rules that resolve the domains
// with them, so that connections by name (the system proxy) use those
// addresses too.
type overrides struct {
	servers    []any
	dnsRules   []any
	routeRules []any
	suffixes   []string // domains answered by an override server
	names      []string // full names with fixed addresses
}

// dnsOverrides builds the enabled overrides. viaProxy says whether a proxy
// is there for the servers asked through it.
func dnsOverrides(list []store.DNSOverride, viaProxy bool) (overrides, error) {
	var ov overrides
	hosts := map[string]any{}
	for i, o := range list {
		domains := cleanDomains(o.Domains)
		if o.Disabled || len(domains) == 0 {
			continue
		}
		if len(o.IPs) > 0 {
			var ips []string
			for _, ip := range o.IPs {
				if a := net.ParseIP(strings.TrimSpace(ip)); a != nil {
					ips = append(ips, a.String())
				}
			}
			if len(ips) == 0 {
				continue
			}
			for _, d := range domains {
				hosts[d] = ips
			}
			ov.names = append(ov.names, domains...)
			continue
		}
		if strings.TrimSpace(o.Server) == "" {
			continue
		}
		tag := fmt.Sprintf("dns-override-%d", i+1)
		srv, err := parseDNS(o.Server, tag)
		if err != nil {
			return ov, fmt.Errorf(i18n.T("DNS 复写（%s）: %w"), domains[0], err)
		}
		if srv["type"] != "local" {
			if o.ViaProxy && viaProxy {
				srv["detour"] = TagProxy
			}
			if host, _ := srv["server"].(string); host != "" && net.ParseIP(host) == nil {
				srv["domain_resolver"] = TagDNSDirect
			}
		}
		ov.servers = append(ov.servers, srv)
		ov.dnsRules = append(ov.dnsRules, map[string]any{"domain_suffix": domains, "server": tag})
		ov.routeRules = append(ov.routeRules, map[string]any{"domain_suffix": domains, "action": "resolve", "server": tag})
		ov.suffixes = append(ov.suffixes, domains...)
	}
	if len(hosts) > 0 {
		ov.servers = append(ov.servers, map[string]any{"type": "hosts", "tag": TagDNSHosts, "predefined": hosts})
		// fixed addresses come before the servers
		ov.dnsRules = append([]any{map[string]any{"domain": ov.names, "server": TagDNSHosts}}, ov.dnsRules...)
		ov.routeRules = append([]any{map[string]any{"domain": ov.names, "action": "resolve", "server": TagDNSHosts}}, ov.routeRules...)
	}
	return ov, nil
}

// withoutOverridden makes the resolve step for the rules that match IP
// addresses (rule, without action) leave the overridden domains alone: they
// were resolved already, and resolving again would replace their addresses.
func (ov overrides) withoutOverridden(rule map[string]any, server string) map[string]any {
	if len(ov.suffixes)+len(ov.names) == 0 {
		rule["action"], rule["server"] = "resolve", server
		return rule
	}
	not := map[string]any{"invert": true}
	if len(ov.suffixes) > 0 {
		not["domain_suffix"] = ov.suffixes
	}
	if len(ov.names) > 0 {
		not["domain"] = ov.names
	}
	return map[string]any{
		"type": "logical", "mode": "and", "rules": []any{rule, not},
		"action": "resolve", "server": server,
	}
}

// cleanDomains lowers the case and drops "*.", leading dots, empty and
// repeated names.
func cleanDomains(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, "*.")
		d = strings.Trim(d, ".")
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}
