package config

import (
	"sort"
	"strings"

	"nextbox/internal/store"
)

// SceneDef is what a scene (store.Scene) matches.
type SceneDef struct {
	ID   string
	Sets []string // rule-set tags
	CIDR []string
}

// SceneDefs, in the order they are checked. Bypassing mainland China comes
// last so that the more specific scenes win.
var SceneDefs = []SceneDef{
	{ID: "ai", Sets: []string{"geosite-category-ai-!cn"}},
	{ID: "media", Sets: []string{"geosite-youtube", "geosite-netflix", "geosite-disney", "geosite-spotify", "geosite-hbo", "geosite-primevideo", "geosite-tiktok"}},
	{ID: "games", Sets: []string{"geosite-category-games@cn"}},
	{ID: "cn", Sets: []string{"geosite-cn", "geoip-cn"}},
}

func sceneDef(id string) (SceneDef, bool) {
	for _, d := range SceneDefs {
		if d.ID == id {
			return d, true
		}
	}
	return SceneDef{}, false
}

const adsRuleSet = "geosite-category-ads-all"

// Entry is one step of the routing decision, in order.
type Entry struct {
	Source string // "ads", "rule" or "scene"
	Index  int    // position in Settings.Rules for a rule
	Scene  string // scene ID
	Target string // store.Target* or store.PolicyTarget

	matcher
}

// matcher holds what an entry matches; any field may match.
type matcher struct {
	domain, suffix, keyword, regex, cidr, process, sets []string
}

func (m matcher) empty() bool {
	return len(m.domain)+len(m.suffix)+len(m.keyword)+len(m.regex)+len(m.cidr)+len(m.process)+len(m.sets) == 0
}

func (m *matcher) add(kind string, values []string) {
	for _, v := range values {
		switch kind {
		case "domain":
			m.domain = append(m.domain, v)
		case "suffix":
			m.suffix = append(m.suffix, v)
		case "keyword":
			m.keyword = append(m.keyword, v)
		case "regex":
			m.regex = append(m.regex, v)
		case "geosite":
			m.sets = append(m.sets, "geosite-"+v)
		case "geoip":
			m.sets = append(m.sets, "geoip-"+v)
		case "cidr":
			m.cidr = append(m.cidr, v)
		case "process":
			m.process = append(m.process, v)
		}
	}
}

// routeRule returns a logical-or sing-box rule matching any entry.
func (m matcher) routeRule() map[string]any {
	r := m.domainRule()
	if len(m.cidr) > 0 {
		r["ip_cidr"] = m.cidr
	}
	if len(m.process) > 0 {
		r["process_name"] = m.process
	}
	if len(m.sets) > 0 {
		r["rule_set"] = m.sets
	}
	if len(r) == 0 {
		return nil
	}
	return r
}

// dnsRule is the part of the matcher that applies to DNS queries (domains
// only); nil when there is none.
func (m matcher) dnsRule() map[string]any {
	r := m.domainRule()
	var geosite []string
	for _, s := range m.sets {
		if strings.HasPrefix(s, "geosite-") {
			geosite = append(geosite, s)
		}
	}
	if len(geosite) > 0 {
		r["rule_set"] = geosite
	}
	if len(r) == 0 {
		return nil
	}
	return r
}

func (m matcher) domainRule() map[string]any {
	r := map[string]any{}
	if len(m.domain) > 0 {
		r["domain"] = m.domain
	}
	if len(m.suffix) > 0 {
		r["domain_suffix"] = m.suffix
	}
	if len(m.keyword) > 0 {
		r["domain_keyword"] = m.keyword
	}
	if len(m.regex) > 0 {
		r["domain_regex"] = m.regex
	}
	return r
}

// Entries lists the routing steps the settings ask for, in the order they
// are checked: ad blocking (every mode), then in rule mode the user's rules
// and the enabled scenes. Traffic that matches none goes to the node (global
// and rule mode) or direct.
func Entries(s store.Settings) []Entry {
	var out []Entry
	if s.BlockAds {
		out = append(out, Entry{Source: "ads", Target: store.TargetBlock, matcher: matcher{sets: []string{adsRuleSet}}})
	}
	if s.RouteMode != "rule" {
		return out
	}
	for i, r := range s.Rules {
		if r.Disabled {
			continue
		}
		e := Entry{Source: "rule", Index: i, Target: r.Target}
		e.add(r.Kind, r.Values)
		if !e.empty() {
			out = append(out, e)
		}
	}
	enabled := map[string]store.Scene{}
	for _, sc := range s.Scenes {
		if sc.Enabled {
			enabled[sc.ID] = sc
		}
	}
	for _, d := range SceneDefs {
		sc, ok := enabled[d.ID]
		if !ok {
			continue
		}
		target := sc.Target
		if d.ID == "cn" {
			target = store.TargetDirect
		}
		out = append(out, Entry{Source: "scene", Scene: d.ID, Target: target,
			matcher: matcher{sets: append([]string(nil), d.Sets...), cidr: append([]string(nil), d.CIDR...)}})
	}
	return out
}

// RuleSets lists the rule-set tags the entries need.
func RuleSets(entries []Entry) []string {
	seen := map[string]bool{}
	for _, e := range entries {
		for _, s := range e.sets {
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

// withoutSets drops references to the given rule-sets, and entries left
// matching nothing.
func withoutSets(entries []Entry, names []string) []Entry {
	drop := map[string]bool{}
	for _, n := range names {
		drop[n] = true
	}
	out := entries[:0:0]
	for _, e := range entries {
		var sets []string
		for _, s := range e.sets {
			if !drop[s] {
				sets = append(sets, s)
			}
		}
		e.sets = sets
		if !e.empty() {
			out = append(out, e)
		}
	}
	return out
}

func usesProcess(entries []Entry) bool {
	for _, e := range entries {
		if len(e.process) > 0 {
			return true
		}
	}
	return false
}

// RequiredRuleSets lists the rule-sets the current settings need.
func RequiredRuleSets(s store.Settings) []string {
	return RuleSets(Entries(s))
}

// UsedPolicies lists the policy groups the settings send traffic to.
func UsedPolicies(s store.Settings) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, e := range Entries(s) {
		if id := store.TargetPolicy(e.Target); id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Values of an entry, for the rule tester (see the main package).
func (e Entry) Domains() (domain, suffix, keyword, regex []string) {
	return e.domain, e.suffix, e.keyword, e.regex
}
func (e Entry) CIDRs() []string       { return e.cidr }
func (e Entry) Processes() []string   { return e.process }
func (e Entry) RuleSetTags() []string { return e.sets }
