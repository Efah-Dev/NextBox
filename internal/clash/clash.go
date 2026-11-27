// Package clash converts what a Clash subscription has besides its proxies
// (link.ParseContent reads those) into NextBox's own settings: proxy groups
// into policy groups (分组代理), rules into rule sets, one per target, and the
// DNS settings (nameserver-policy, hosts) into DNS overrides (DNS 复写). It is
// shared by NextBox for Windows and NextBox for Android.
package clash

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"nextbox/internal/i18n"
	"nextbox/internal/store"
)

// Extras is what a Clash config has besides its proxies.
type Extras struct {
	Groups []Group
	// Rules are the rules that can be converted, in their order.
	Rules []Rule
	// Skipped are the rules that cannot ("DST-PORT,443,DIRECT").
	Skipped []string
	// RuleTotal counts the rules of the file, without MATCH.
	RuleTotal int
	DNS       []store.DNSOverride
	// SubDNS is the subscription's own DNS (订阅专属 DNS), see subdns.go.
	SubDNS *store.GroupDNS
}

// Group is a proxy group of the config.
type Group struct {
	Name     string
	Type     string // select, url-test, fallback, load-balance, relay
	Proxies  []string
	Filter   string // a regular expression on the proxy names
	All      bool   // include-all / include-all-proxies
	Interval int    // url-test interval in seconds; 0 = not given
}

// Rule is a converted rule: Kind is one of store.RuleKinds or store.KindURL
// (a rule provider, downloaded by its address), Target a name of the config
// (DIRECT, REJECT, a group or a proxy).
type Rule struct {
	Kind, Value, Target string
}

// Parse reads the Clash parts of a subscription; nil when it has none (not a
// Clash config, or only proxies).
func Parse(text string) (*Extras, error) {
	text = strings.TrimPrefix(strings.TrimSpace(text), string(rune(0xFEFF)))
	if !strings.Contains(text, "proxy-groups:") && !strings.Contains(text, "rules:") && !strings.Contains(text, "nameserver") {
		return nil, nil
	}
	var cfg struct {
		Groups    []map[string]any          `yaml:"proxy-groups"`
		Rules     []string                  `yaml:"rules"`
		Providers map[string]map[string]any `yaml:"rule-providers"`
		DNS       struct {
			Policy      map[string]any `yaml:"nameserver-policy"`
			Nameservers []string       `yaml:"nameserver"`
			ProxyServer []string       `yaml:"proxy-server-nameserver"`
		} `yaml:"dns"`
		Hosts map[string]any `yaml:"hosts"`
	}
	if err := yaml.Unmarshal([]byte(text), &cfg); err != nil {
		return nil, err
	}
	ex := &Extras{}
	for _, g := range cfg.Groups {
		grp := Group{Name: str(g["name"]), Type: str(g["type"]), Proxies: strs(g["proxies"]), Filter: str(g["filter"])}
		grp.All = g["include-all"] == true || g["include-all-proxies"] == true
		grp.Interval, _ = strconv.Atoi(str(g["interval"]))
		if grp.Name != "" {
			ex.Groups = append(ex.Groups, grp)
		}
	}
	for _, line := range cfg.Rules {
		ex.addRule(line, cfg.Providers)
	}
	ex.DNS = dnsOverrides(cfg.DNS.Policy, cfg.Hosts)
	ex.SubDNS = clashSubDNS(cfg.DNS.Nameservers, cfg.DNS.ProxyServer)
	if len(ex.Groups) == 0 && ex.RuleTotal == 0 && len(ex.DNS) == 0 && ex.SubDNS == nil {
		return nil, nil
	}
	return ex, nil
}

// ruleKinds maps Clash rule types to rule kinds.
var ruleKinds = map[string]string{
	"DOMAIN": "domain", "DOMAIN-SUFFIX": "suffix", "DOMAIN-KEYWORD": "keyword", "DOMAIN-REGEX": "regex",
	"GEOSITE": "geosite", "GEOIP": "geoip", "IP-CIDR": "cidr", "IP-CIDR6": "cidr",
	"PROCESS-NAME": "process",
}

func (ex *Extras) addRule(line string, providers map[string]map[string]any) {
	parts := strings.Split(line, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	typ := strings.ToUpper(parts[0])
	if typ == "MATCH" || typ == "FINAL" || typ == "" {
		return // the rest of the traffic: NextBox's own last step
	}
	ex.RuleTotal++
	if len(parts) < 3 {
		ex.Skipped = append(ex.Skipped, line)
		return
	}
	value, target := parts[1], parts[2]
	switch kind, ok := ruleKinds[typ]; {
	case ok && kind == "geoip" && strings.EqualFold(value, "LAN"):
		ex.RuleTotal-- // LAN addresses always go direct in NextBox
	case ok:
		if kind == "geosite" || kind == "geoip" {
			value = strings.ToLower(value)
		}
		ex.Rules = append(ex.Rules, Rule{kind, value, target})
	case typ == "RULE-SET":
		p := providers[value]
		url, format := str(p["url"]), strings.ToLower(str(p["format"]))
		if url == "" || format == "mrs" {
			ex.Skipped = append(ex.Skipped, line)
			return
		}
		ex.Rules = append(ex.Rules, Rule{store.KindURL, url, target})
	default:
		ex.Skipped = append(ex.Skipped, line)
	}
}

// dnsOverrides turns nameserver-policy (domains → servers) and hosts
// (names → addresses) into DNS overrides. Servers of geosite: and rule-set:
// keys, and system or DHCP servers, have no counterpart and are left out.
func dnsOverrides(policy, hosts map[string]any) []store.DNSOverride {
	var out []store.DNSOverride
	byServer := map[string]int{}
	keys := make([]string, 0, len(policy))
	for k := range policy {
		keys = append(keys, k)
	}
	slices.Sort(keys) // a stable order between updates
	for _, key := range keys {
		servers := strs(policy[key])
		if len(servers) == 0 {
			servers = []string{str(policy[key])}
		}
		server := strings.TrimSpace(servers[0])
		if server == "" || server == "system" || strings.HasPrefix(server, "dhcp://") {
			continue
		}
		for _, d := range strings.Split(key, ",") {
			d = strings.TrimSpace(d)
			if d == "" || strings.Contains(d, ":") {
				continue // geosite:, rule-set:
			}
			d = strings.TrimPrefix(strings.TrimPrefix(d, "+."), "*.")
			i, ok := byServer[server]
			if !ok {
				i = len(out)
				byServer[server] = i
				out = append(out, store.DNSOverride{Server: server, ViaProxy: privateServer(server)})
			}
			out[i].Domains = append(out[i].Domains, d)
		}
	}
	names := make([]string, 0, len(hosts))
	for k := range hosts {
		names = append(names, k)
	}
	slices.Sort(names)
	for _, name := range names {
		ips := strs(hosts[name])
		if len(ips) == 0 {
			ips = []string{str(hosts[name])}
		}
		var valid []string
		for _, ip := range ips {
			if net.ParseIP(strings.TrimSpace(ip)) != nil {
				valid = append(valid, strings.TrimSpace(ip))
			}
		}
		name = strings.TrimPrefix(strings.TrimPrefix(name, "+."), "*.")
		if len(valid) > 0 && name != "" {
			out = append(out, store.DNSOverride{Domains: []string{name}, IPs: valid})
		}
	}
	return out
}

// privateServer reports whether a DNS server is at a private address, which
// a provider's DNS only answers through its proxy.
func privateServer(server string) bool {
	host := server
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	host = strings.SplitN(host, "/", 2)[0]
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsPrivate()
}

// ---------------------------------------------------------------- preview

// Preview says what an import would make, for the dialog that asks.
type Preview struct {
	Groups    []string `json:"groups"`    // proxy groups with nodes of the subscription
	Rules     int      `json:"rules"`     // rules that can be converted
	RuleTotal int      `json:"ruleTotal"` // rules in the file
	Sets      int      `json:"sets"`      // rule sets they make (one per target)
	DNS       int      `json:"dns"`       // DNS overrides
	// SubDNS is the subscription's own DNS, when it names one
	SubDNS  *store.GroupDNS `json:"subDns"`
	Skipped []string        `json:"skipped"` // rules that cannot be converted (at most 20)
}

// Preview counts what Apply would make with the nodes of the subscription.
func (ex *Extras) Preview(nodes []store.Profile) Preview {
	p := Preview{Rules: len(ex.Rules), RuleTotal: ex.RuleTotal, DNS: len(ex.DNS), SubDNS: ex.SubDNS}
	names := nodeNames(nodes)
	for _, g := range ex.Groups {
		if len(ex.members(g.Name, names, nil)) > 0 {
			p.Groups = append(p.Groups, g.Name)
		}
	}
	targets := map[string]bool{}
	for _, r := range ex.Rules {
		targets[r.Target] = true
	}
	p.Sets = len(targets)
	p.Skipped = ex.Skipped[:min(len(ex.Skipped), 20)]
	return p
}

func nodeNames(nodes []store.Profile) map[string]int64 {
	m := make(map[string]int64, len(nodes))
	for _, n := range nodes {
		m[n.Name] = n.ID
	}
	return m
}

func (ex *Extras) group(name string) *Group {
	for i := range ex.Groups {
		if ex.Groups[i].Name == name {
			return &ex.Groups[i]
		}
	}
	return nil
}

// members lists the node names a group ends up with: its proxies, the
// members of the groups it names, all nodes (include-all), then the filter.
func (ex *Extras) members(name string, nodes map[string]int64, seen map[string]bool) []string {
	g := ex.group(name)
	if g == nil || seen[name] {
		return nil
	}
	if seen == nil {
		seen = map[string]bool{}
	}
	seen[name] = true
	var out []string
	add := func(n string) {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	for _, p := range g.Proxies {
		if _, ok := nodes[p]; ok {
			add(p)
		} else {
			for _, m := range ex.members(p, nodes, seen) {
				add(m)
			}
		}
	}
	if g.All {
		all := make([]string, 0, len(nodes))
		for n := range nodes {
			all = append(all, n)
		}
		slices.Sort(all)
		for _, n := range all {
			add(n)
		}
	}
	if g.Filter != "" {
		if re, err := regexp.Compile(g.Filter); err == nil {
			out = slices.DeleteFunc(out, func(n string) bool { return !re.MatchString(n) })
		}
	}
	return out
}

// ---------------------------------------------------------------- apply

// Summary says what Apply made.
type Summary struct {
	Policies int  `json:"policies"`
	Sets     int  `json:"sets"`
	DNS      int  `json:"dns"`
	SubDNS   bool `json:"subDns"` // the subscription's own DNS was set
}

func (s Summary) String() string {
	if s.Policies+s.Sets+s.DNS == 0 {
		if s.SubDNS {
			return i18n.T("已使用订阅专属 DNS")
		}
		return ""
	}
	msg := fmt.Sprintf(i18n.T("Clash 订阅：%d 个分组代理、%d 个规则集、%d 条 DNS 复写"), s.Policies, s.Sets, s.DNS)
	if s.SubDNS {
		msg += i18n.T("，已使用订阅专属 DNS")
	}
	return msg
}

// Apply replaces what an earlier import of the subscription g made with the
// parts opt chooses: its nodes must be in the store already. Policy groups
// keep their IDs by name, so rules pointing to them stay valid; rule sets
// keep the place of the earlier ones (new ones go before the mainland China
// bypass, after the user's own rules).
func Apply(st *store.Store, ex *Extras, g store.Group, opt store.ClashImport) (Summary, error) {
	var sum Summary
	nodes := nodeNames(st.Profiles(g.ID))
	old := map[string]store.Policy{}
	for _, p := range st.Policies() {
		if p.From == g.ID {
			old[p.Name] = p
		}
	}
	targets := map[string]string{"DIRECT": store.TargetDirect, "REJECT": store.TargetBlock, "REJECT-DROP": store.TargetBlock}
	labels := map[string]string{store.TargetDirect: i18n.T("直连"), store.TargetBlock: i18n.T("拦截"), store.TargetProxy: i18n.T("当前节点")}
	if opt.Groups {
		for _, cg := range ex.Groups {
			var ids []int64
			for _, n := range ex.members(cg.Name, nodes, nil) {
				ids = append(ids, nodes[n])
			}
			if len(ids) == 0 {
				continue
			}
			ids = ids[:min(len(ids), store.MaxPolicyMembers)]
			p := store.Policy{Name: cg.Name, Match: store.PolicyManual, Members: ids, GroupID: g.ID, Mode: store.PolicySelect, From: g.ID}
			if cg.Type == "url-test" || cg.Type == "fallback" || cg.Type == "load-balance" {
				p.Mode = store.PolicyAuto
				if cg.Interval > 0 {
					p.Interval = min(max((cg.Interval+59)/60, 1), store.MaxPolicyInterval)
				}
			}
			if prev, ok := old[cg.Name]; ok {
				p.ID = prev.ID
				if slices.Contains(ids, prev.Selected) {
					p.Selected = prev.Selected
				}
				delete(old, cg.Name)
			}
			saved, err := st.SavePolicy(p)
			if err != nil {
				return sum, err
			}
			targets[cg.Name] = store.PolicyTarget(saved.ID)
			labels[targets[cg.Name]] = cg.Name
			sum.Policies++
		}
	}
	for _, p := range old {
		if err := st.DeletePolicy(p.ID); err != nil {
			return sum, err
		}
	}

	set := st.Settings()
	// rule sets: the earlier ones go, the new ones take their place
	at := -1
	rules := set.Rules[:0:0]
	for _, r := range set.Rules {
		if r.From == g.ID {
			if at < 0 {
				at = len(rules)
			}
			continue
		}
		rules = append(rules, r)
	}
	if opt.Rules && len(ex.Rules) > 0 {
		var made []store.Rule
		byTarget := map[string]int{}
		for _, r := range ex.Rules {
			target, ok := targets[r.Target]
			if !ok {
				target = store.TargetProxy // a single proxy, or a group not taken over
			}
			i, ok := byTarget[target]
			if !ok {
				i = len(made)
				byTarget[target] = i
				made = append(made, store.Rule{Kind: store.KindSet, Name: g.Name + " · " + labels[target], Target: target, From: g.ID})
			}
			addItem(&made[i], r.Kind, r.Value)
		}
		if at < 0 {
			at = slices.IndexFunc(rules, func(r store.Rule) bool { return r.Kind == store.KindScene && len(r.Values) > 0 && r.Values[0] == "cn" })
			if at < 0 {
				at = len(rules)
			}
		}
		rules = slices.Insert(rules, at, made...)
		sum.Sets = len(made)
	}
	set.Rules = rules

	dns := set.DNSOverrides[:0:0]
	for _, o := range set.DNSOverrides {
		if o.From != g.ID {
			dns = append(dns, o)
		}
	}
	if opt.DNS {
		for _, o := range ex.DNS {
			o.From = g.ID
			dns = append(dns, o)
		}
		sum.DNS = len(ex.DNS)
	}
	set.DNSOverrides = dns
	if opt.SubDNS && ex.SubDNS != nil {
		if cur, ok := st.Group(g.ID); ok {
			d := *ex.SubDNS
			cur.DNS = &d
			if err := st.UpdateGroup(cur); err != nil {
				return sum, err
			}
			sum.SubDNS = true
		}
	}
	return sum, st.SaveSettings(set)
}

func addItem(r *store.Rule, kind, value string) {
	for i := range r.Items {
		if r.Items[i].Kind == kind {
			if !slices.Contains(r.Items[i].Values, value) {
				r.Items[i].Values = append(r.Items[i].Values, value)
			}
			return
		}
	}
	r.Items = append(r.Items, store.RuleItem{Kind: kind, Values: []string{value}})
}

// ---------------------------------------------------------------- yaml helpers

func str(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func strs(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, x := range list {
		if s := str(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}
