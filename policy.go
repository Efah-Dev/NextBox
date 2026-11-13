package main

// Policy groups ("分组代理") and the rule tester.

import (
	"context"
	"errors"
	"net"
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
	Input  string `json:"input"`
	Source string `json:"source"` // "ads", "rule", "scene", "private" or "final"
	Index  int    `json:"index"`  // the rule, for source "rule"
	Scene  string `json:"scene"`
	Target string `json:"target"` // store.Target* / policy target; "final" uses the mode
	// Note explains a limit of the test, e.g. that IP rules cannot be checked
	// against a domain name.
	Note string `json:"note,omitempty"`
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
	res := RouteMatch{Input: host}
	s := a.store.Settings()
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		res.Source, res.Target = "private", store.TargetDirect
		return res, nil
	}
	skippedIP := false
	for _, e := range config.Entries(s) {
		hit, skipped := a.entryMatches(e, host, ip)
		skippedIP = skippedIP || skipped
		if hit {
			res.Source, res.Index, res.Scene, res.Target = e.Source, e.Index, e.Scene, e.Target
			return res, nil
		}
	}
	res.Source, res.Target = "final", "final"
	if skippedIP {
		res.Note = i18n.T("按 IP 判断的规则（IP 段、IP 归属地）只对 IP 地址生效，测试域名时已跳过")
	}
	return res, nil
}

// entryMatches reports whether a routing step matches; skipped is set when
// it has IP rules that cannot be checked against a domain name.
func (a *App) entryMatches(e config.Entry, host string, ip net.IP) (hit, skipped bool) {
	if ip == nil {
		domain, suffix, keyword, regex := e.Domains()
		for _, d := range domain {
			if host == d {
				return true, false
			}
		}
		for _, d := range suffix {
			if host == d || strings.HasSuffix(host, "."+d) {
				return true, false
			}
		}
		for _, k := range keyword {
			if strings.Contains(host, k) {
				return true, false
			}
		}
		for _, r := range regex {
			if re, err := regexp.Compile(r); err == nil && re.MatchString(host) {
				return true, false
			}
		}
	} else {
		for _, c := range e.CIDRs() {
			if _, n, err := net.ParseCIDR(c); err == nil && n.Contains(ip) {
				return true, false
			}
		}
	}
	for _, set := range e.RuleSetTags() {
		isIP := strings.HasPrefix(set, "geoip-")
		if isIP != (ip != nil) {
			skipped = skipped || isIP
			continue
		}
		if a.ruleSetMatches(set, host) {
			return true, false
		}
	}
	if ip == nil && len(e.CIDRs()) > 0 {
		skipped = true
	}
	return false, skipped
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
