package main

// Policy groups ("分组代理") and the rule tester.

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/geo"
	"nextbox/internal/i18n"
	"nextbox/internal/store"
	"nextbox/internal/sys"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// nodeCountryCode is where a node is, as the UI shows it: the detected
// country for the current source, otherwise a guess from the name.
func nodeCountryCode(p store.Profile, source string) string {
	if p.Country != "" && p.CountrySource == source {
		return p.Country
	}
	return geo.Guess(p.Name)
}

// policyMembers returns the nodes a policy group currently holds, in order.
func (a *App) policyMembers(p store.Policy) []store.Profile {
	source := a.store.Settings().CountrySource
	var out []store.Profile
	add := func(n store.Profile) bool {
		if a.chainError(n) != nil {
			return true
		}
		out = append(out, n)
		return len(out) < store.MaxPolicyMembers
	}
	if p.Match == store.PolicyManual {
		for _, n := range a.store.ProfilesByIDs(p.Members) {
			if !add(n) {
				break
			}
		}
		return out
	}
	keywords := p.Keywords()
	for _, n := range a.store.Profiles(p.GroupID) {
		match := false
		switch p.Match {
		case store.PolicyCountry:
			match = p.Country != "" && nodeCountryCode(n, source) == p.Country
		case store.PolicyKeyword:
			name := strings.ToLower(n.Name)
			for _, k := range keywords {
				if strings.Contains(name, k) {
					match = true
					break
				}
			}
		}
		if match && !add(n) {
			break
		}
	}
	return out
}

// policyInputs resolves the policy groups the current rules use.
func (a *App) policyInputs(s store.Settings) []config.PolicyInput {
	var out []config.PolicyInput
	for _, id := range config.UsedPolicies(s) {
		p, ok := a.store.Policy(id)
		if !ok {
			a.log(i18n.T("规则用到的分组代理已被删除，改走当前节点"))
			continue
		}
		members := a.policyMembers(p)
		if len(members) == 0 {
			a.log(i18n.T("分组代理“") + p.Name + i18n.T("”没有可用节点，改走当前节点"))
			continue
		}
		out = append(out, config.PolicyInput{Policy: p, Members: members})
	}
	return out
}

// PolicyMember is a node of a policy group as the settings page lists it.
type PolicyMember struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Latency int    `json:"latency"`
	Country string `json:"country"`
}

type PolicyView struct {
	store.Policy
	Nodes []PolicyMember `json:"nodes"` // the matching nodes (Members holds a manual choice)
	// Now is the member the running core uses (0 when not connected or the
	// group is not used by any rule).
	Now int64 `json:"now"`
	// Rules counts the rules and scenes sending traffic here.
	Rules int `json:"rules"`
}

func (a *App) GetPolicies() []PolicyView {
	s := a.store.Settings()
	uses := map[int64]int{}
	for _, r := range s.Rules {
		uses[store.TargetPolicy(r.Target)]++
	}
	for _, sc := range s.Scenes {
		uses[store.TargetPolicy(sc.Target)]++
	}
	a.mu.Lock()
	proc, ptags, ntags := a.proc, a.policyTags, a.nodeTags
	a.mu.Unlock()
	byTag := map[string]int64{}
	for id, tag := range ntags {
		byTag[tag] = id
	}
	out := []PolicyView{}
	for _, p := range a.store.Policies() {
		v := PolicyView{Policy: p, Nodes: []PolicyMember{}, Rules: uses[p.ID]}
		for _, n := range a.policyMembers(p) {
			v.Nodes = append(v.Nodes, PolicyMember{ID: n.ID, Name: n.Name, Type: n.Type, Latency: n.Latency, Country: nodeCountryCode(n, s.CountrySource)})
		}
		if tag, ok := ptags[p.ID]; ok && proc != nil {
			ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
			if now, err := proc.API.SelectedProxy(ctx, tag); err == nil {
				v.Now = byTag[now]
			}
			cancel()
		}
		out = append(out, v)
	}
	return out
}

func (a *App) SavePolicy(p store.Policy) (store.Policy, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return p, errors.New(i18n.T("请填写名称"))
	}
	switch p.Match {
	case store.PolicyCountry:
		if p.Country == "" {
			return p, errors.New(i18n.T("请选择地区"))
		}
	case store.PolicyKeyword:
		if len(p.Keywords()) == 0 {
			return p, errors.New(i18n.T("请填写关键词"))
		}
	case store.PolicyManual:
		if len(p.Members) == 0 {
			return p, errors.New(i18n.T("请至少选择一个节点"))
		}
	default:
		return p, errors.New(i18n.T("未知的筛选方式"))
	}
	if p.Mode != store.PolicySelect {
		p.Mode = store.PolicyAuto
	}
	saved, err := a.store.SavePolicy(p)
	if err != nil {
		return saved, err
	}
	a.restartIfPolicyUsed(saved.ID)
	return saved, nil
}

// DeletePolicy removes a policy group; rules and scenes that used it send
// their traffic to the selected node instead.
func (a *App) DeletePolicy(id int64) error {
	if err := a.store.DeletePolicy(id); err != nil {
		return err
	}
	s := a.store.Settings()
	target := store.PolicyTarget(id)
	changed := false
	for i := range s.Rules {
		if s.Rules[i].Target == target {
			s.Rules[i].Target, changed = store.TargetProxy, true
		}
	}
	for i := range s.Scenes {
		if s.Scenes[i].Target == target {
			s.Scenes[i].Target, changed = store.TargetProxy, true
		}
	}
	if changed {
		return a.SaveSettings(s)
	}
	return nil
}

func (a *App) restartIfPolicyUsed(id int64) {
	for _, used := range config.UsedPolicies(a.store.Settings()) {
		if used == id {
			go a.restartIfRunning()
			return
		}
	}
}

// SelectPolicyNode picks the node of a policy group in select mode. A running
// core switches at once when it has the node loaded.
func (a *App) SelectPolicyNode(id, node int64) error {
	if err := a.store.SetPolicySelected(id, node); err != nil {
		return err
	}
	a.mu.Lock()
	proc, tag, nodeTag := a.proc, a.policyTags[id], a.nodeTags[node]
	a.mu.Unlock()
	if proc == nil || tag == "" {
		return nil
	}
	if nodeTag != "" && proc.API.SelectProxy(a.ctx, tag, nodeTag) == nil {
		return nil
	}
	return a.restartIfRunning()
}

// ---------------------------------------------------------------- rule tester

// RouteMatch tells where a domain or IP address would go.
type RouteMatch struct {
	Input string `json:"input"`
	// Source is "excluded" (a TUN exclusion), "private", "ads", "rule",
	// "scene" or "final".
	Source string `json:"source"`
	Index  int    `json:"index"` // the step in Settings.Rules (ads, rule, scene); -1 when it has none
	Scene  string `json:"scene"`
	Target string `json:"target"` // store.Target* / policy target; "final" uses the mode
	// Note explains a limit of the test, e.g. that IP rules cannot be checked
	// against a domain name.
	Note string `json:"note,omitempty"`
	// Resolved is the address the domain name was resolved to before the
	// steps that match IP addresses (as the core does), "" when it was not.
	Resolved string `json:"resolved,omitempty"`
}

var hostOnly = regexp.MustCompile(`^[a-z0-9.\-_:\[\]]+$`)

// TestRoute checks a domain name or an IP address against the current rules
// in order, like the core would for a new connection. Rule-sets are looked up
// with the core's own "rule-set match" command.
func (a *App) TestRoute(input string) (RouteMatch, error) {
	host := strings.ToLower(strings.TrimSpace(input))
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" || !hostOnly.MatchString(host) {
		return RouteMatch{}, errors.New(i18n.T("请输入网址、域名或 IP 地址"))
	}
	res := RouteMatch{Input: host, Index: -1}
	s := a.store.Settings()
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		res.Source, res.Target = "private", store.TargetDirect
		return res, nil
	}
	if ip != nil {
		excl, _ := config.ParseCIDRs(s.TunExclude)
		for _, c := range excl {
			if p, err := netip.ParsePrefix(c); err == nil {
				if addr, ok := netip.AddrFromSlice(ip); ok && p.Contains(addr.Unmap()) {
					res.Source, res.Target = "excluded", store.TargetDirect
					return res, nil
				}
			}
		}
	}
	// Like the core, a domain name is resolved with the direct DNS before
	// the first step that matches IP addresses, and from then on the steps
	// are checked against both.
	isDomain := ip == nil
	var addrs []net.IP
	resolved := !isDomain
	if !isDomain {
		addrs = []net.IP{ip}
	}
	for _, e := range config.Entries(s) {
		if !resolved && e.HasIP() {
			resolved = true
			addrs = a.resolveDirect(s.DirectDNS, host)
			if len(addrs) > 0 {
				res.Resolved = addrs[0].String()
			} else {
				res.Note = i18n.T("直连 DNS 没有解析出这个域名的 IP，按 IP 判断的规则（IP 段、IP 归属地）已跳过")
			}
		}
		if a.entryMatches(e, host, isDomain, addrs) {
			res.Source, res.Index, res.Scene, res.Target = e.Source, e.Index, e.Scene, e.Target
			return res, nil
		}
	}
	res.Source, res.Target, res.Index = "final", "final", -1
	return res, nil
}

// resolveDirect looks a name up the way the core's direct DNS would: with
// the server given there when it is plain DNS (an address, udp:// or
// tcp://), with the system's resolver otherwise. It gives up after a few
// seconds.
func (a *App) resolveDirect(directDNS, host string) []net.IP {
	r := net.DefaultResolver
	addr := strings.TrimSpace(directDNS)
	network := "udp"
	switch {
	case strings.HasPrefix(addr, "udp://"):
		addr = strings.TrimPrefix(addr, "udp://")
	case strings.HasPrefix(addr, "tcp://"):
		addr, network = strings.TrimPrefix(addr, "tcp://"), "tcp"
	case strings.Contains(addr, "://"), addr == "", strings.EqualFold(addr, "local"), strings.EqualFold(addr, "dhcp"):
		addr = ""
	}
	if addr != "" {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(strings.Trim(addr, "[]"), "53")
		}
		r = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		}}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()
	ips, err := r.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil
	}
	return ips
}

// entryMatches reports whether a routing step matches a domain name (when
// isDomain) and its addresses, or an IP address (addrs alone).
func (a *App) entryMatches(e config.Entry, host string, isDomain bool, addrs []net.IP) bool {
	if isDomain {
		domain, suffix, keyword, regex := e.Domains()
		for _, d := range domain {
			if host == d {
				return true
			}
		}
		for _, d := range suffix {
			if host == d || strings.HasSuffix(host, "."+d) {
				return true
			}
		}
		for _, k := range keyword {
			if strings.Contains(host, k) {
				return true
			}
		}
		for _, r := range regex {
			if re, err := regexp.Compile(r); err == nil && re.MatchString(host) {
				return true
			}
		}
	}
	for _, ip := range addrs {
		for _, c := range e.CIDRs() {
			if _, n, err := net.ParseCIDR(c); err == nil && n.Contains(ip) {
				return true
			}
		}
	}
	for _, set := range e.RuleSetTags() {
		// a remote rule-set may hold domains and IP ranges alike
		if (isDomain && !strings.HasPrefix(set, "geoip-")) && a.ruleSetMatches(set, host) {
			return true
		}
		if strings.HasPrefix(set, "geosite-") {
			continue
		}
		for _, ip := range addrs {
			if a.ruleSetMatches(set, ip.String()) {
				return true
			}
		}
	}
	return false
}

func (a *App) ruleSetMatches(set, host string) bool {
	path := filepath.Join(a.ruleSetDir(), set+".srs")
	if _, err := os.Stat(path); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.corePath(), "rule-set", "match", "-f", "binary", path, host)
	sys.PrepareCmd(cmd)
	out, err := cmd.CombinedOutput() // the result is printed to stderr
	return err == nil && strings.Contains(string(out), "match rules")
}

// ---------------------------------------------------------------- picking programs

// RunningPrograms lists the programs running now, for the program rules.
func (a *App) RunningPrograms() ([]sys.RunningProgram, error) {
	list, err := sys.RunningPrograms()
	if list == nil {
		list = []sys.RunningProgram{}
	}
	return list, err
}

// PickProgram asks for a program file and returns its file name (what
// process rules match), or "" when the dialog was cancelled.
func (a *App) PickProgram() (string, error) {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   i18n.T("选择程序"),
		Filters: []wruntime.FileFilter{{DisplayName: i18n.T("程序") + " (*.exe)", Pattern: "*.exe"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	return filepath.Base(path), nil
}
