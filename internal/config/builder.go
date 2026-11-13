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
	// Policies are the policy groups the rules send traffic to, with their
	// member nodes (see UsedPolicies). Rules naming a policy group that is
	// missing here, or has no usable member, use the selected node instead.
	Policies []PolicyInput
}

type PolicyInput struct {
	Policy  store.Policy
	Members []store.Profile
}

const TagFakeIP = "dns-fakeip"

// Build generates the main config. It returns *MissingRuleSetsError when a
// required rule-set file has not been downloaded yet (see SkipMissingRuleSets).
func Build(in Input) (map[string]any, error) {
	s := in.Settings
	if in.Node == nil && s.RouteMode != "direct" {
		return nil, errors.New(i18n.T("请先选择一个节点"))
	}
	entries := Entries(s)
	if missing := MissingRuleSets(in.RuleSetDir, RuleSets(entries)); len(missing) > 0 {
		if !in.SkipMissingRuleSets {
			return nil, &MissingRuleSetsError{Names: missing}
		}
		entries = withoutSets(entries, missing)
	}

	// ---- outbounds ----
	outbounds := []any{}
	hasProxy := in.Node != nil
	policyTags := map[int64]string{}
	if hasProxy {
		// "proxy" is a selector over the loaded nodes: rules and DNS refer to
		// it, and switching nodes only changes what it points at.
		all, tags, ptags := in.Tags()
		used := map[string]bool{TagProxy: true, TagDirect: true}
		for _, tag := range tags {
			used[tag] = true
		}
		for _, tag := range ptags {
			used[tag] = true
		}
		ok := map[int64]bool{}
		var nodeOutbounds []any
		for _, n := range all {
			obs, err := Expand(n, tags[n.ID], in.Member, used)
			if err != nil {
				if n.ID == in.Node.ID {
					return nil, err
				}
				continue // a node that cannot be used: leave it out
			}
			ok[n.ID] = true
			for _, ob := range obs {
				nodeOutbounds = append(nodeOutbounds, ob)
			}
		}
		var names []string
		for _, n := range in.Switchable() {
			if ok[n.ID] {
				names = append(names, tags[n.ID])
			}
		}
		outbounds = append(outbounds, map[string]any{
			"type":      "selector",
			"tag":       TagProxy,
			"outbounds": names,
			"default":   tags[in.Node.ID],
		})
		for _, pi := range in.Policies {
			var members []string
			def := ""
			for _, m := range pi.Members {
				if ok[m.ID] {
					members = append(members, tags[m.ID])
					if m.ID == pi.Policy.Selected {
						def = tags[m.ID]
					}
				}
			}
			if len(members) == 0 {
				continue // its rules fall back to the selected node
			}
			tag := ptags[pi.Policy.ID]
			policyTags[pi.Policy.ID] = tag
			if pi.Policy.Mode == store.PolicySelect {
				if def == "" {
					def = members[0]
				}
				outbounds = append(outbounds, map[string]any{"type": "selector", "tag": tag, "outbounds": members, "default": def})
			} else {
				outbounds = append(outbounds, map[string]any{
					"type": "urltest", "tag": tag, "outbounds": members,
					"url": s.TestURL, "interval": "10m", "tolerance": 50, "idle_timeout": "30m",
				})
			}
		}
		outbounds = append(outbounds, nodeOutbounds...)
	}
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": TagDirect})

	// where an entry's traffic goes: an outbound tag, or "" to block it
	outboundOf := func(target string) string {
		switch target {
		case store.TargetBlock:
			return ""
		case store.TargetDirect:
			return TagDirect
		}
		if !hasProxy {
			return TagDirect
		}
		if tag, ok := policyTags[store.TargetPolicy(target)]; ok {
			return tag
		}
		return TagProxy
	}

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
	// FakeIP only makes sense when the core sees the DNS queries (TUN)
	fakeIP := s.TunEnabled && s.TunFakeIP && dnsFinal == TagDNSRemote
	if fakeIP {
		dnsServers = append(dnsServers, map[string]any{
			"type": "fakeip", "tag": TagFakeIP,
			"inet4_range": "198.18.0.0/15", "inet6_range": "fc00::/18",
		})
	}
	dnsRules := []any{}
	for _, e := range entries {
		if e.Target == store.TargetBlock {
			if r := e.dnsRule(); r != nil {
				r["action"] = "reject"
				dnsRules = append(dnsRules, r)
			}
		}
	}
	if fakeIP {
		if excl := fakeIPExclusions(s.FakeIPExclude); len(excl) > 0 {
			dnsRules = append(dnsRules, map[string]any{"domain_suffix": excl, "server": TagDNSDirect})
		}
	}
	if dnsFinal == TagDNSRemote {
		for _, e := range entries {
			if e.Target == store.TargetBlock {
				continue
			}
			r := e.dnsRule()
			if r == nil {
				continue
			}
			switch {
			case outboundOf(e.Target) == TagDirect:
				r["server"] = TagDNSDirect
			case fakeIP:
				r["server"] = TagFakeIP
				r["query_type"] = []string{"A", "AAAA"}
			default:
				r["server"] = TagDNSRemote
			}
			dnsRules = append(dnsRules, r)
		}
		if fakeIP {
			dnsRules = append(dnsRules, map[string]any{"query_type": []string{"A", "AAAA"}, "server": TagFakeIP})
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
	privateDone := false
	for _, e := range entries {
		// ad blocking comes before LAN addresses, everything else after
		if e.Source != "ads" && !privateDone {
			routeRules = append(routeRules, map[string]any{"ip_is_private": true, "outbound": TagDirect})
			privateDone = true
		}
		r := e.routeRule()
		if r == nil {
			continue
		}
		if ob := outboundOf(e.Target); ob == "" {
			r["action"] = "reject"
		} else {
			r["outbound"] = ob
		}
		routeRules = append(routeRules, r)
	}
	if !privateDone {
		routeRules = append(routeRules, map[string]any{"ip_is_private": true, "outbound": TagDirect})
	}
	ruleSets := []any{}
	for _, name := range RuleSets(entries) {
		ruleSets = append(ruleSets, map[string]any{
			"type":   "local",
			"tag":    name,
			"format": "binary",
			"path":   filepath.Join(in.RuleSetDir, name+".srs"),
		})
	}

	cacheFile := map[string]any{"enabled": true, "path": "cache.db"}
	if fakeIP {
		cacheFile["store_fakeip"] = true
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
			"find_process":            s.FindProcess || usesProcess(entries),
			"default_domain_resolver": TagDNSDirect,
		},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": in.ClashAPI,
				"secret":              in.Secret,
			},
			"cache_file": cacheFile,
		},
	}
	return cfg, nil
}

func fakeIPExclusions(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimPrefix(strings.TrimSpace(line), "."); line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, strings.ToLower(line))
		}
	}
	return out
}

// Tags returns every node the config loads (the switchable ones first, then
// the members of policy groups), their outbound tags, and the tags of the
// policy groups. Switchable nodes get the same tags as from NodeTags alone.
func (in Input) Tags() (all []store.Profile, nodeTags, policyTags map[int64]string) {
	all = in.Switchable()
	seen := map[int64]bool{}
	for _, n := range all {
		seen[n.ID] = true
	}
	for _, pi := range in.Policies {
		for _, m := range pi.Members {
			if !seen[m.ID] {
				seen[m.ID] = true
				all = append(all, m)
			}
		}
	}
	nodeTags = NodeTags(all)
	used := map[string]bool{TagProxy: true, TagDirect: true}
	for _, t := range nodeTags {
		used[t] = true
	}
	policyTags = map[int64]string{}
	for _, pi := range in.Policies {
		tag := strings.TrimSpace(pi.Policy.Name)
		for tag == "" || used[tag] {
			tag = strings.TrimSpace(tag + " #" + strconv.FormatInt(pi.Policy.ID, 10))
		}
		used[tag] = true
		policyTags[pi.Policy.ID] = tag
	}
	return all, nodeTags, policyTags
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
