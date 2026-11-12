// Package config builds sing-box (1.12+ format) configurations.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"nextbox/internal/i18n"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"nextbox/internal/store"
)

const (
	TagProxy     = "proxy"
	TagDirect    = "direct"
	TagDNSRemote = "dns-remote"
	TagDNSDirect = "dns-direct"
)

type Input struct {
	Settings store.Settings
	Node     *store.Profile // the selected node; may be nil in direct mode
	// Nodes are loaded next to Node so that the selection can be changed
	// through the clash API without restarting the core (see NodeTags).
	Nodes []store.Profile
	// Member looks up the member nodes of chain proxies; may be nil when no
	// chain proxy is involved.
	Member     func(id int64) (store.Profile, bool)
	ClashAPI   string // "127.0.0.1:port"
	Secret     string
	RuleSetDir string
	// SkipMissingRuleSets leaves out the rules that need a rule-set file which
	// is not on disk, instead of failing with *MissingRuleSetsError.
	SkipMissingRuleSets bool
}

// Build generates the main config. It returns *MissingRuleSetsError when a
// required rule-set file has not been downloaded yet (see SkipMissingRuleSets).
func Build(in Input) (map[string]any, error) {
	s := in.Settings
	if in.Node == nil && s.RouteMode != "direct" {
		return nil, errors.New(i18n.T("请先选择一个节点"))
	}
	rules := collectRules(s)
	if missing := MissingRuleSets(in.RuleSetDir, rules.ruleSets()); len(missing) > 0 {
		if !in.SkipMissingRuleSets {
			return nil, &MissingRuleSetsError{Names: missing}
		}
		rules = rules.without(missing)
	}

	// ---- outbounds ----
	outbounds := []any{}
	hasProxy := in.Node != nil
	if hasProxy {
		// "proxy" is a selector over the loaded nodes: rules and DNS refer to
		// it, and switching nodes only changes what it points at.
		nodes := in.Switchable()
		tags := NodeTags(nodes)
		used := map[string]bool{TagProxy: true, TagDirect: true}
		for _, tag := range tags {
			used[tag] = true
		}
		names := make([]string, 0, len(nodes))
		var nodeOutbounds []any
		for _, n := range nodes {
			obs, err := Expand(n, tags[n.ID], in.Member, used)
			if err != nil {
				if n.ID == in.Node.ID {
					return nil, err
				}
				continue // another node of the group that cannot be used: leave it out
			}
			names = append(names, tags[n.ID])
			for _, ob := range obs {
				nodeOutbounds = append(nodeOutbounds, ob)
			}
		}
		outbounds = append(outbounds, map[string]any{
			"type":      "selector",
			"tag":       TagProxy,
			"outbounds": names,
			"default":   tags[in.Node.ID],
		})
		outbounds = append(outbounds, nodeOutbounds...)
	}
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": TagDirect})

	// ---- dns ----
	directDNS, err := parseDNS(s.DirectDNS, TagDNSDirect)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("直连 DNS: %w"), err)
	}
	dnsServers := []any{directDNS}
	dnsFinal := TagDNSDirect
	if hasProxy && s.RouteMode != "direct" {
		remote, err := parseDNS(s.RemoteDNS, TagDNSRemote)
		if err != nil {
			return nil, fmt.Errorf(i18n.T("远程 DNS: %w"), err)
		}
		if remote["type"] != "local" {
			remote["detour"] = TagProxy
			if host, _ := remote["server"].(string); host != "" && net.ParseIP(host) == nil {
				remote["domain_resolver"] = TagDNSDirect
			}
		}
		dnsServers = append([]any{remote}, dnsServers...)
		dnsFinal = TagDNSRemote
	}
	dnsRules := []any{}
	if r := rules.block.domainMatcher(); r != nil {
		r["action"] = "reject"
		dnsRules = append(dnsRules, r)
	}
	if dnsFinal == TagDNSRemote {
		if r := rules.proxy.domainMatcher(); r != nil {
			r["server"] = TagDNSRemote
			dnsRules = append(dnsRules, r)
		}
		if r := rules.direct.domainMatcher(); r != nil {
			r["server"] = TagDNSDirect
			dnsRules = append(dnsRules, r)
		}
		if rules.hasBuiltin("geosite-cn") {
			dnsRules = append(dnsRules, map[string]any{"rule_set": []string{"geosite-cn"}, "server": TagDNSDirect})
		}
	}
	strategy := "ipv4_only"
	if s.EnableIPv6 {
		strategy = "prefer_ipv4"
	}
	dns := map[string]any{
		"servers":  dnsServers,
		"rules":    dnsRules,
		"final":    dnsFinal,
		"strategy": strategy,
	}

	// ---- inbounds ----
	listen := s.ListenAddress
	if s.AllowLAN {
		listen = "::"
	}
	inbounds := []any{map[string]any{
		"type":        "mixed",
		"tag":         "mixed-in",
		"listen":      listen,
		"listen_port": s.MixedPort,
	}}
	if s.TunEnabled {
		address := []string{"172.19.0.1/30"}
		if s.EnableIPv6 {
			address = append(address, "fdfe:dcba:9876::1/126")
		}
		inbounds = append(inbounds, map[string]any{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": "NextBox",
			"address":        address,
			"mtu":            s.TunMTU,
			"auto_route":     true,
			"strict_route":   s.TunStrictRoute,
			"stack":          s.TunStack,
		})
	}

	// ---- route ----
	final := TagProxy
	if s.RouteMode == "direct" || !hasProxy {
		final = TagDirect
	}
	routeRules := []any{}
	if s.Sniff {
		routeRules = append(routeRules, map[string]any{"action": "sniff"})
	}
	routeRules = append(routeRules, map[string]any{"protocol": "dns", "action": "hijack-dns"})
	if r := rules.block.matcher(); r != nil {
		r["action"] = "reject"
		routeRules = append(routeRules, r)
	}
	routeRules = append(routeRules, map[string]any{"ip_is_private": true, "outbound": TagDirect})
	if final == TagProxy {
		if r := rules.proxy.matcher(); r != nil {
			r["outbound"] = TagProxy
			routeRules = append(routeRules, r)
		}
		if r := rules.direct.matcher(); r != nil {
			r["outbound"] = TagDirect
			routeRules = append(routeRules, r)
		}
		for _, name := range []string{"geosite-cn", "geoip-cn"} {
			if rules.hasBuiltin(name) {
				routeRules = append(routeRules, map[string]any{"rule_set": []string{name}, "outbound": TagDirect})
			}
		}
	}
	ruleSets := []any{}
	for _, name := range rules.ruleSets() {
		ruleSets = append(ruleSets, map[string]any{
			"type":   "local",
			"tag":    name,
			"format": "binary",
			"path":   filepath.Join(in.RuleSetDir, name+".srs"),
		})
	}

	cfg := map[string]any{
		"log":       map[string]any{"level": s.LogLevel, "timestamp": true},
		"dns":       dns,
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route": map[string]any{
			"rules":                   routeRules,
			"rule_set":                ruleSets,
			"final":                   final,
			"auto_detect_interface":   true,
			"find_process":            s.FindProcess || rules.usesProcess(),
			"default_domain_resolver": TagDNSDirect,
		},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": in.ClashAPI,
				"secret":              in.Secret,
			},
			"cache_file": map[string]any{"enabled": true, "path": "cache.db"},
		},
	}
	return cfg, nil
}

// Switchable returns the nodes the config is built with: the selected node
// first, then the others without duplicates.
func (in Input) Switchable() []store.Profile {
	if in.Node == nil {
		return nil
	}
	nodes := []store.Profile{*in.Node}
	seen := map[int64]bool{in.Node.ID: true}
	for _, n := range in.Nodes {
		if !seen[n.ID] {
			seen[n.ID] = true
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// NodeTags gives every node a unique outbound tag. The node name is used
// where possible, because the tag is what the connection list shows.
func NodeTags(nodes []store.Profile) map[int64]string {
	used := map[string]bool{TagProxy: true, TagDirect: true}
	tags := make(map[int64]string, len(nodes))
	for _, n := range nodes {
		tag := strings.TrimSpace(n.Name)
		for tag == "" || used[tag] {
			tag = strings.TrimSpace(tag + " #" + strconv.FormatInt(n.ID, 10))
		}
		used[tag] = true
		tags[n.ID] = tag
	}
	return tags
}

// ChainError explains why a chain proxy cannot be used.
type ChainError struct {
	Chain string // name of the chain proxy
	Hop   int    // 1-based position of the offending member, 0 for the chain itself
	Msg   string
}

func (e *ChainError) Error() string {
	if e.Hop == 0 {
		return fmt.Sprintf(i18n.T("链式代理“%s”：%s"), e.Chain, e.Msg)
	}
	return fmt.Sprintf(i18n.T("链式代理“%s”的第 %d 跳：%s"), e.Chain, e.Hop, e.Msg)
}

// Expand returns the outbounds for one profile: one for a plain node, one per
// hop for a chain proxy. The last outbound (the exit) gets tag; the hops
// before it get "tag / n" and each dials through the one before it. used
// holds the tags already taken and receives the hop tags (it may be nil).
func Expand(p store.Profile, tag string, member func(int64) (store.Profile, bool), used map[string]bool) ([]map[string]any, error) {
	if !p.IsChain() {
		return []map[string]any{withTag(p.Outbound, tag)}, nil
	}
	if len(p.Chain) < store.MinChainLength || len(p.Chain) > store.MaxChainLength {
		return nil, &ChainError{Chain: p.Name, Msg: fmt.Sprintf(i18n.T("需要 %d 到 %d 个节点"), store.MinChainLength, store.MaxChainLength)}
	}
	out := make([]map[string]any, 0, len(p.Chain))
	prev := ""
	for i, id := range p.Chain {
		var m store.Profile
		ok := false
		if member != nil {
			m, ok = member(id)
		}
		switch {
		case !ok:
			return nil, &ChainError{Chain: p.Name, Hop: i + 1, Msg: i18n.T("节点已不存在")}
		case m.IsChain():
			return nil, &ChainError{Chain: p.Name, Hop: i + 1, Msg: i18n.T("不能是另一个链式代理")}
		}
		hopTag := tag
		if i < len(p.Chain)-1 {
			hopTag = tag + " / " + strconv.Itoa(i+1)
			for used != nil && used[hopTag] {
				hopTag += "'"
			}
		}
		if used != nil {
			used[hopTag] = true
		}
		ob := withTag(m.Outbound, hopTag)
		delete(ob, "detour") // the chain decides how each hop is reached
		if prev != "" {
			ob["detour"] = prev
		}
		out = append(out, ob)
		prev = hopTag
	}
	return out, nil
}

func withTag(outbound map[string]any, tag string) map[string]any {
	ob := make(map[string]any, len(outbound)+1)
	for k, v := range outbound {
		ob[k] = v
	}
	ob["tag"] = tag
	return ob
}

// BuildTest generates a config for latency tests: one outbound per node (one
// per hop for a chain proxy) and a single local mixed inbound. Each node is
// reached by authenticating with its own username (see TestUser), so one port
// serves every node. member looks up the members of chain proxies.
func BuildTest(s store.Settings, nodes []store.Profile, member func(int64) (store.Profile, bool), clashAPI, secret string, port int) (map[string]any, error) {
	directDNS, err := parseDNS(s.DirectDNS, TagDNSDirect)
	if err != nil {
		return nil, err
	}
	outbounds := []any{}
	users := []any{}
	rules := []any{}
	used := map[string]bool{TagDirect: true}
	for _, n := range nodes {
		used[TestTag(n.ID)] = true
	}
	for _, n := range nodes {
		obs, err := Expand(n, TestTag(n.ID), member, used)
		if err != nil {
			return nil, err
		}
		for _, ob := range obs {
			outbounds = append(outbounds, ob)
		}
		users = append(users, map[string]any{"username": TestUser(n.ID), "password": secret})
		rules = append(rules, map[string]any{"auth_user": []string{TestUser(n.ID)}, "outbound": TestTag(n.ID)})
	}
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": TagDirect})
	strategy := "ipv4_only"
	if s.EnableIPv6 {
		strategy = "prefer_ipv4"
	}
	return map[string]any{
		"log": map[string]any{"level": "error", "timestamp": true},
		"dns": map[string]any{"servers": []any{directDNS}, "strategy": strategy},
		"inbounds": []any{map[string]any{
			"type":        "mixed",
			"tag":         "test-in",
			"listen":      "127.0.0.1",
			"listen_port": port,
			"users":       users,
		}},
		"outbounds": outbounds,
		"route": map[string]any{
			"rules":                   rules,
			"final":                   TagDirect,
			"auto_detect_interface":   true,
			"default_domain_resolver": TagDNSDirect,
		},
		"experimental": map[string]any{
			"clash_api": map[string]any{"external_controller": clashAPI, "secret": secret},
		},
	}, nil
}

func TestTag(id int64) string { return "node-" + strconv.FormatInt(id, 10) }

// TestUser is the inbound username that routes a test request to node id.
func TestUser(id int64) string { return "n" + strconv.FormatInt(id, 10) }

// ---- DNS address parsing ----

// parseDNS accepts "local", "dhcp", "1.1.1.1", "udp://", "tcp://", "tls://",
// "quic://", "https://" and "h3://" addresses.
func parseDNS(addr, tag string) (map[string]any, error) {
	addr = strings.TrimSpace(addr)
	switch strings.ToLower(addr) {
	case "", "local":
		return map[string]any{"type": "local", "tag": tag}, nil
	case "dhcp", "dhcp://auto":
		return map[string]any{"type": "dhcp", "tag": tag}, nil
	}
	if !strings.Contains(addr, "://") {
		addr = "udp://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf(i18n.T("无效 DNS 地址 %q"), addr)
	}
	types := map[string]string{"udp": "udp", "tcp": "tcp", "tls": "tls", "quic": "quic", "https": "https", "h3": "http3"}
	typ, ok := types[strings.ToLower(u.Scheme)]
	if !ok {
		return nil, fmt.Errorf(i18n.T("不支持的 DNS 协议 %q"), u.Scheme)
	}
	server := map[string]any{"type": typ, "tag": tag, "server": u.Hostname()}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf(i18n.T("无效 DNS 端口 %q"), u.Port())
		}
		server["server_port"] = port
	}
	if (typ == "https" || typ == "http3") && u.Path != "" && u.Path != "/dns-query" {
		server["path"] = u.Path
	}
	return server, nil
}

// ---- user rules ----

// ruleList is a parsed block of user rules, one per line:
//
//	example.com / suffix:example.com   domain suffix
//	domain:example.com / full:...      exact domain
//	keyword:google                     domain keyword
//	regex:^ads?\.                      domain regex
//	geosite:netflix / geoip:jp         remote rule-set (auto downloaded)
//	1.2.3.0/24 / ip:1.2.3.4            IP CIDR
//	process:chrome.exe                 process name
type ruleList struct {
	domain, suffix, keyword, regex, cidr, process, sets []string
}

var ruleKinds = map[string]bool{
	"domain": true, "full": true, "suffix": true, "domain_suffix": true,
	"keyword": true, "regex": true, "regexp": true, "geosite": true, "geoip": true,
	"ip": true, "ip_cidr": true, "process": true,
}

func parseRuleLines(text string) ruleList {
	var r ruleList
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kind, value, ok := strings.Cut(line, ":")
		if !ok || !ruleKinds[strings.ToLower(kind)] {
			// no prefix (also covers IPv6 addresses, which contain ':')
			kind, value = "", line
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(kind) {
		case "domain", "full":
			r.domain = append(r.domain, value)
		case "suffix", "domain_suffix":
			r.suffix = append(r.suffix, strings.TrimPrefix(value, "."))
		case "keyword":
			r.keyword = append(r.keyword, value)
		case "regex", "regexp":
			r.regex = append(r.regex, value)
		case "geosite":
			r.sets = append(r.sets, "geosite-"+strings.ToLower(value))
		case "geoip":
			r.sets = append(r.sets, "geoip-"+strings.ToLower(value))
		case "ip", "ip_cidr":
			r.cidr = append(r.cidr, normalizeCIDR(value))
		case "process":
			r.process = append(r.process, value)
		default:
			if _, _, err := net.ParseCIDR(value); err == nil || net.ParseIP(value) != nil {
				r.cidr = append(r.cidr, normalizeCIDR(value))
			} else {
				r.suffix = append(r.suffix, strings.TrimPrefix(value, "."))
			}
		}
	}
	return r
}

func normalizeCIDR(v string) string {
	if ip := net.ParseIP(v); ip != nil {
		if ip.To4() != nil {
			return v + "/32"
		}
		return v + "/128"
	}
	return v
}

// matcher returns a logical-or route rule matching any entry, or nil.
func (r ruleList) matcher() map[string]any {
	m := r.domainMatcherMap()
	if len(r.cidr) > 0 {
		m["ip_cidr"] = r.cidr
	}
	if len(r.process) > 0 {
		m["process_name"] = r.process
	}
	if len(r.sets) > 0 {
		m["rule_set"] = r.sets
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// domainMatcher is used in DNS rules, which only care about domains.
func (r ruleList) domainMatcher() map[string]any {
	m := r.domainMatcherMap()
	var geosite []string
	for _, s := range r.sets {
		if strings.HasPrefix(s, "geosite-") {
			geosite = append(geosite, s)
		}
	}
	if len(geosite) > 0 {
		m["rule_set"] = geosite
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func (r ruleList) domainMatcherMap() map[string]any {
	m := map[string]any{}
	if len(r.domain) > 0 {
		m["domain"] = r.domain
	}
	if len(r.suffix) > 0 {
		m["domain_suffix"] = r.suffix
	}
	if len(r.keyword) > 0 {
		m["domain_keyword"] = r.keyword
	}
	if len(r.regex) > 0 {
		m["domain_regex"] = r.regex
	}
	return m
}

type userRules struct {
	block, proxy, direct ruleList
	builtin              []string
}

func collectRules(s store.Settings) userRules {
	u := userRules{
		block:  parseRuleLines(s.BlockRules),
		proxy:  parseRuleLines(s.ProxyRules),
		direct: parseRuleLines(s.DirectRules),
	}
	if s.BlockAds {
		u.block.sets = append(u.block.sets, "geosite-category-ads-all")
	}
	if s.RouteMode == "rule" {
		u.builtin = []string{"geosite-cn", "geoip-cn"}
	}
	if s.RouteMode == "direct" {
		// everything goes direct, only block rules matter
		u.proxy, u.direct = ruleList{}, ruleList{}
	}
	return u
}

func (u userRules) ruleSets() []string {
	seen := map[string]bool{}
	for _, list := range [][]string{u.builtin, u.block.sets, u.proxy.sets, u.direct.sets} {
		for _, s := range list {
			seen[s] = true
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func (u userRules) hasBuiltin(name string) bool {
	for _, s := range u.builtin {
		if s == name {
			return true
		}
	}
	return false
}

// without drops every reference to the given rule-sets.
func (u userRules) without(names []string) userRules {
	drop := map[string]bool{}
	for _, n := range names {
		drop[n] = true
	}
	keep := func(list []string) []string {
		var out []string
		for _, s := range list {
			if !drop[s] {
				out = append(out, s)
			}
		}
		return out
	}
	u.builtin = keep(u.builtin)
	u.block.sets, u.proxy.sets, u.direct.sets = keep(u.block.sets), keep(u.proxy.sets), keep(u.direct.sets)
	return u
}

// usesProcess reports whether any rule matches by process name, which needs
// the core to look up the owning process of every connection.
func (u userRules) usesProcess() bool {
	return len(u.block.process)+len(u.proxy.process)+len(u.direct.process) > 0
}

// RequiredRuleSets lists the rule-sets the current settings need.
func RequiredRuleSets(s store.Settings) []string {
	return collectRules(s).ruleSets()
}

type MissingRuleSetsError struct{ Names []string }

func (e *MissingRuleSetsError) Error() string {
	return i18n.T("缺少规则集: ") + strings.Join(e.Names, ", ")
}

func MissingRuleSets(dir string, names []string) []string {
	var missing []string
	for _, n := range names {
		if st, err := os.Stat(filepath.Join(dir, n+".srs")); err != nil || st.Size() == 0 {
			missing = append(missing, n)
		}
	}
	return missing
}

// RuleSetURL expands the mirror template ({kind} = geoip|geosite, {name} = rule-set tag).
func RuleSetURL(mirror, name string) string {
	kind := "geosite"
	if strings.HasPrefix(name, "geoip-") {
		kind = "geoip"
	}
	return strings.NewReplacer("{kind}", kind, "{name}", name).Replace(mirror)
}
