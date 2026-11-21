package config

import (
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"nextbox/internal/store"
)

// SceneDef is what a scene (store.Scene) matches.
type SceneDef struct {
	ID     string
	Sets   []string // rule-set tags
	CIDR   []string
	Suffix []string // domain suffixes
}

// SceneDefs, in the order they are checked. Bypassing mainland China comes
// last so that the more specific scenes win. The game downloads include
// Steam's content servers, whose store and community are left to the rest
// of the route (store.SteamDownloadDomains).
var SceneDefs = []SceneDef{
	{ID: "ai", Sets: []string{"geosite-category-ai-!cn"}},
	{ID: "media", Sets: []string{"geosite-youtube", "geosite-netflix", "geosite-disney", "geosite-spotify", "geosite-hbo", "geosite-primevideo", "geosite-tiktok"}},
	{ID: "games", Sets: []string{"geosite-category-games@cn"}, Suffix: store.SteamDownloadDomains},
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

// URLRuleSetTag is the tag (and file name, with ".srs") of a rule-set
// downloaded from u: "url-" and a hash of the address.
func URLRuleSetTag(u string) string {
	h := sha256.Sum256([]byte(u))
	return "url-" + hex.EncodeToString(h[:6])
}

// IsURLRuleSet reports whether a rule-set tag is one of URLRuleSetTag.
func IsURLRuleSet(tag string) bool { return strings.HasPrefix(tag, "url-") }

// URLRuleSets maps the tags of the remote rule-sets the settings use (in
// rule sets of rule mode) to their addresses.
func URLRuleSets(s store.Settings) map[string]string {
	out := map[string]string{}
	if s.RouteMode != "rule" {
		return out
	}
	for _, r := range s.Rules {
		if !r.IsSet() || r.Disabled {
			continue
		}
		for _, it := range r.Items {
			if it.Kind == store.KindURL {
				for _, u := range it.Values {
					out[URLRuleSetTag(u)] = u
				}
			}
		}
	}
	return out
}

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
	domain, suffix, keyword, regex, cidr, process, pkg, sets []string
}

func (m matcher) empty() bool {
	return len(m.domain)+len(m.suffix)+len(m.keyword)+len(m.regex)+len(m.cidr)+len(m.process)+len(m.pkg)+len(m.sets) == 0
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
		case "package":
			// Android apps: kept in the data everywhere, only matched on Android
			if runtime.GOOS == "android" {
				m.pkg = append(m.pkg, v)
			}
		case store.KindURL:
			m.sets = append(m.sets, URLRuleSetTag(v))
		}
	}
}

// hasIP reports whether the entry has parts that match IP addresses (IP
// ranges, IP rule-sets, or a remote rule-set, which may have both).
func (m matcher) hasIP() bool {
	if len(m.cidr) > 0 {
		return true
	}
	for _, s := range m.sets {
		if strings.HasPrefix(s, "geoip-") || IsURLRuleSet(s) {
			return true
		}
	}
	return false
}

// anyOf joins sing-box rules: one stays as it is, more become a logical-or
// rule. Fields of different groups in one sing-box rule (destination,
// process, rule-set) must all match, so an entry matching by several of them
// needs one rule per group.
func anyOf(parts []map[string]any) map[string]any {
	switch len(parts) {
	case 0:
		return nil
	case 1:
		return parts[0]
	}
	rules := make([]any, len(parts))
	for i, p := range parts {
		rules[i] = p
	}
	return map[string]any{"type": "logical", "mode": "or", "rules": rules}
}

// routeRule returns the sing-box rule that matches when any part of the
// entry does.
func (m matcher) routeRule() map[string]any {
	var parts []map[string]any
	dest := m.domainRule()
	if len(m.cidr) > 0 {
		dest["ip_cidr"] = m.cidr // the same group as the domains in sing-box
	}
	if len(dest) > 0 {
		parts = append(parts, dest)
	}
	if len(m.sets) > 0 {
		parts = append(parts, map[string]any{"rule_set": m.sets})
	}
	if len(m.process) > 0 {
		parts = append(parts, map[string]any{"process_name": m.process})
	}
	if len(m.pkg) > 0 {
		parts = append(parts, map[string]any{"package_name": m.pkg})
	}
	return anyOf(parts)
}

// dnsRule is the part of the matcher that applies to DNS queries (domains
// and domain rule-sets); nil when there is none. Remote rule-sets are left
// out: they may hold IP rules, which mean something else for DNS.
func (m matcher) dnsRule() map[string]any {
	var parts []map[string]any
	if r := m.domainRule(); len(r) > 0 {
		parts = append(parts, r)
	}
	var geosite []string
	for _, s := range m.sets {
		if strings.HasPrefix(s, "geosite-") {
			geosite = append(geosite, s)
		}
	}
	if len(geosite) > 0 {
		parts = append(parts, map[string]any{"rule_set": geosite})
	}
	return anyOf(parts)
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
// are checked (LAN addresses, which always go direct, come before them).
// In rule mode that is the order of Settings.Rules, where ad blocking and
// each scene have a place of their own (store.KindAds, store.KindScene);
// in the other modes only ad blocking applies. Traffic that matches none
// goes to the node (global and rule mode) or direct.
func Entries(s store.Settings) []Entry {
	ads := Entry{Source: "ads", Index: -1, Target: store.TargetBlock, matcher: matcher{sets: []string{adsRuleSet}}}
	if s.RouteMode != "rule" {
		if s.BlockAds {
			return []Entry{ads}
		}
		return nil
	}
	enabled := map[string]store.Scene{}
	for _, sc := range s.Scenes {
		if sc.Enabled {
			enabled[sc.ID] = sc
		}
	}
	scene := func(id string, index int) (Entry, bool) {
		sc, on := enabled[id]
		d, known := sceneDef(id)
		if !on || !known {
			return Entry{}, false
		}
		target := sc.Target
		if id == "cn" || target == "" {
			target = store.TargetDirect
		}
		return Entry{Source: "scene", Index: index, Scene: id, Target: target,
			matcher: matcher{sets: append([]string(nil), d.Sets...), cidr: append([]string(nil), d.CIDR...), suffix: append([]string(nil), d.Suffix...)}}, true
	}
	var out []Entry
	adsPlaced := false
	placed := map[string]bool{}
	for i, r := range s.Rules {
		switch r.Kind {
		case store.KindAds:
			if !adsPlaced && s.BlockAds {
				a := ads
				a.Index = i
				out = append(out, a)
			}
			adsPlaced = true
			continue
		case store.KindScene:
			if len(r.Values) > 0 && !placed[r.Values[0]] {
				placed[r.Values[0]] = true
				if e, ok := scene(r.Values[0], i); ok {
					out = append(out, e)
				}
			}
			continue
		}
		if r.Disabled {
			continue
		}
		e := Entry{Source: "rule", Index: i, Target: r.Target}
		if r.IsSet() {
			for _, it := range r.Items {
				e.add(it.Kind, it.Values)
			}
		} else {
			e.add(r.Kind, r.Values)
		}
		if !e.empty() {
			out = append(out, e)
		}
	}
	// settings that were not normalized (store.placeSteps): the order of v2.0
	if !adsPlaced && s.BlockAds {
		out = append([]Entry{ads}, out...)
	}
	for _, d := range SceneDefs {
		if !placed[d.ID] {
			if e, ok := scene(d.ID, -1); ok {
				out = append(out, e)
			}
		}
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
		if len(e.process)+len(e.pkg) > 0 {
			return true
		}
	}
	return false
}

// RequiredRuleSets lists the rule-sets the current settings need.
func RequiredRuleSets(s store.Settings) []string {
	entries := Entries(s)
	names := RuleSets(entries)
	if needsResolve(entries) {
		names = append(names, ForeignSites)
	}
	return names
}

// ForeignSites is the rule-set of known sites outside mainland China. Their
// names are not resolved for the steps that match IP addresses: they go
// through the proxy by name. The direct DNS answers some of them (blocked
// ones) with wrong addresses, and the proxy was then sent to those.
const ForeignSites = "geosite-geolocation-!cn"

// ChinaSites is the rule-set of sites in mainland China (the domains of the
// "cn" scene).
const ChinaSites = "geosite-cn"

// needsResolve reports whether a step matches IP addresses, so that domain
// names are resolved before it (see Build).
func needsResolve(entries []Entry) bool {
	for _, e := range entries {
		if e.routeRule() != nil && e.hasIP() {
			return true
		}
	}
	return false
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
func (e Entry) Packages() []string    { return e.pkg }
func (e Entry) RuleSetTags() []string { return e.sets }

// Outbound tags of the routing steps. Every step that sends traffic
// somewhere does so through a selector of its own that only holds the real
// target, so that a connection's chain names the step it matched (the clash
// API reports chains, not rules).
const stepTagPrefix = "nb-step:"

// StepTag is the outbound tag of an entry: "nb-step:rule:3" for the rule at
// Settings.Rules[3], "nb-step:scene:cn" for a scene.
func StepTag(e Entry) string {
	if e.Source == "scene" {
		return stepTagPrefix + "scene:" + e.Scene
	}
	return stepTagPrefix + e.Source + ":" + strconv.Itoa(e.Index)
}

// ParseStepTag reads a StepTag: source "rule" with the rule's index, or
// "scene" with the scene ID. ok is false for any other outbound.
func ParseStepTag(tag string) (source string, index int, scene string, ok bool) {
	rest, found := strings.CutPrefix(tag, stepTagPrefix)
	if !found {
		return "", 0, "", false
	}
	source, arg, _ := strings.Cut(rest, ":")
	switch source {
	case "scene":
		return source, -1, arg, true
	case "rule":
		i, err := strconv.Atoi(arg)
		return source, i, "", err == nil
	}
	return "", 0, "", false
}

// HasIP reports whether the entry matches IP addresses too (see hasIP): the
// core resolves domain names before the first such entry.
func (e Entry) HasIP() bool { return e.hasIP() }

// setField sets a match field on a rule made by anyOf. A logical rule takes
// match fields only in its sub-rules (sing-box rejects them on the logical
// rule itself), so there it goes into each one: (a or b) and f is
// (a and f) or (b and f).
func setField(r map[string]any, key string, value any) {
	if r["type"] == "logical" {
		for _, sub := range r["rules"].([]any) {
			setField(sub.(map[string]any), key, value)
		}
		return
	}
	r[key] = value
}
