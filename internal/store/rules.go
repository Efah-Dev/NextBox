package store

import (
	"net"
	"strconv"
	"strings"
)

// Rule sends traffic that matches any of Values to Target. Rules apply in
// rule mode only and are checked in order, before the scenes.
type Rule struct {
	Kind     string   `json:"kind"` // one of RuleKinds
	Values   []string `json:"values"`
	Target   string   `json:"target"` // TargetProxy, TargetDirect, TargetBlock or PolicyTarget(id)
	Disabled bool     `json:"disabled,omitempty"`
}

// RuleKinds are the things a rule can match.
var RuleKinds = []string{"geosite", "suffix", "domain", "keyword", "regex", "geoip", "cidr", "process"}

const (
	TargetProxy  = "proxy" // the node selected on the home page
	TargetDirect = "direct"
	TargetBlock  = "block"
)

// PolicyTarget is the rule target that sends traffic to a policy group.
func PolicyTarget(id int64) string { return "policy:" + strconv.FormatInt(id, 10) }

// TargetPolicy returns the policy group a target names, or 0.
func TargetPolicy(target string) int64 {
	if v, ok := strings.CutPrefix(target, "policy:"); ok {
		id, _ := strconv.ParseInt(v, 10, 64)
		return id
	}
	return 0
}

// IsDomainKind reports whether rules of this kind match domain names (as
// opposed to IP addresses or processes).
func IsDomainKind(kind string) bool {
	switch kind {
	case "geosite", "suffix", "domain", "keyword", "regex":
		return true
	}
	return false
}

// Scene is a ready-made set of rules that can be switched on as a whole (see
// config.SceneDefs for what each one matches).
type Scene struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Target  string `json:"target"`
}

// DefaultScenes lists every scene in display order with its default state.
func DefaultScenes() []Scene {
	return []Scene{
		{ID: "cn", Enabled: true, Target: TargetDirect},
		{ID: "ai", Target: TargetProxy},
		{ID: "media", Target: TargetProxy},
		{ID: "games", Target: TargetDirect},
	}
}

// mergeScenes keeps the user's state of the known scenes, in the default
// order, and adds scenes introduced by newer versions.
func mergeScenes(have []Scene) []Scene {
	byID := map[string]Scene{}
	for _, s := range have {
		byID[s.ID] = s
	}
	out := DefaultScenes()
	for i, d := range out {
		if s, ok := byID[d.ID]; ok {
			out[i].Enabled = s.Enabled
			if s.Target != "" && d.ID != "cn" { // bypassing mainland China always goes direct
				out[i].Target = s.Target
			}
		}
	}
	return out
}

// DefaultFakeIPExclude are domains that must resolve to real addresses even
// with FakeIP on: Windows' connectivity check, local names, time servers and
// console / game NAT tests.
const DefaultFakeIPExclude = `msftconnecttest.com
msftncsi.com
lan
local
localhost
home.arpa
time.windows.com
ntp.org
stun.l.google.com
xboxlive.com
playstation.net
nintendo.net`

// DefaultDarkLevel reproduces the dark theme of earlier versions.
const DefaultDarkLevel = 60

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

// Normalize repairs settings read from elsewhere (a backup) the way Open
// does: defaults for missing values, rules of older versions converted.
func (s *Settings) Normalize() { s.fillDefaults() }

// ParseRuleValue reads one rule in the "kind:value" form of the text editor
// and of older versions ("google.com", "suffix:google.com", "geosite:netflix",
// "1.2.3.0/24", "process:chrome.exe" ...). ok is false for an unknown kind.
func ParseRuleValue(line string) (kind, value string, ok bool) {
	line = strings.TrimSpace(line)
	k, v, found := strings.Cut(line, ":")
	if found {
		if norm, known := kindAliases[strings.ToLower(strings.TrimSpace(k))]; known {
			return norm, strings.TrimSpace(v), true
		}
		if net.ParseIP(line) == nil && !strings.Contains(line, "/") && !strings.Contains(k, ".") {
			return "", "", false // "geosit:xx", but not an IPv6 address
		}
	}
	// no prefix: an IP / CIDR or a domain suffix
	if _, _, err := net.ParseCIDR(line); err == nil || net.ParseIP(line) != nil {
		return "cidr", line, true
	}
	return "suffix", line, true
}

var kindAliases = map[string]string{
	"geosite": "geosite", "suffix": "suffix", "domain_suffix": "suffix",
	"domain": "domain", "full": "domain", "keyword": "keyword", "domain_keyword": "keyword",
	"regex": "regex", "regexp": "regex", "domain_regex": "regex", "geoip": "geoip",
	"cidr": "cidr", "ip": "cidr", "ip_cidr": "cidr", "process": "process", "process_name": "process",
}

// NormalizeValue tidies a rule value for its kind ("" drops it).
func NormalizeValue(kind, v string) string {
	v = strings.TrimSpace(v)
	switch kind {
	case "suffix":
		v = strings.TrimPrefix(strings.ToLower(v), ".")
	case "domain", "keyword":
		v = strings.ToLower(v)
	case "geosite", "geoip":
		v = strings.ToLower(v)
	case "cidr":
		if ip := net.ParseIP(v); ip != nil {
			if ip.To4() != nil {
				v += "/32"
			} else {
				v += "/128"
			}
		}
	}
	return v
}

// cleanRules drops empty values and rules of unknown kinds. A rule without
// values matches nothing (see config.Entries) but stays for the editor.
func cleanRules(rules []Rule) []Rule {
	out := make([]Rule, 0, len(rules))
	known := map[string]bool{}
	for _, k := range RuleKinds {
		known[k] = true
	}
	for _, r := range rules {
		if !known[r.Kind] {
			continue
		}
		values := []string{} // a rule still being filled in keeps its place
		seen := map[string]bool{}
		for _, v := range r.Values {
			if v = NormalizeValue(r.Kind, v); v != "" && !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}
		r.Values = values
		if r.Target == "" {
			r.Target = TargetProxy
		}
		out = append(out, r)
	}
	return out
}

// migrateRules turns the rule lists of v1.2 (and of the Android version) into
// Rules, in the order they used to apply: block, proxy, then direct.
func (s *Settings) migrateRules() {
	if s.BlockRules == "" && s.ProxyRules == "" && s.DirectRules == "" {
		return
	}
	var added []Rule
	for _, list := range []struct{ text, target string }{
		{s.BlockRules, TargetBlock}, {s.ProxyRules, TargetProxy}, {s.DirectRules, TargetDirect},
	} {
		for _, line := range strings.Split(list.text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			kind, value, ok := ParseRuleValue(line)
			if !ok {
				continue
			}
			// consecutive values of the same kind become one rule
			if n := len(added); n > 0 && added[n-1].Kind == kind && added[n-1].Target == list.target {
				added[n-1].Values = append(added[n-1].Values, value)
				continue
			}
			added = append(added, Rule{Kind: kind, Values: []string{value}, Target: list.target})
		}
	}
	s.Rules = append(added, s.Rules...)
	s.BlockRules, s.ProxyRules, s.DirectRules = "", "", ""
}

// ---------------------------------------------------------------- policy groups

// Policy is a group of nodes that rules can send traffic to ("分组代理"),
// for example "the fastest Japanese node" for YouTube. Its members are chosen
// by a condition, so that they survive subscription updates.
type Policy struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Match is PolicyCountry (nodes in Country), PolicyKeyword (nodes whose
	// name contains any of the space separated Keyword) or PolicyManual
	// (the nodes in Members).
	Match   string  `json:"match"`
	Country string  `json:"country,omitempty"`
	Keyword string  `json:"keyword,omitempty"`
	Members []int64 `json:"members,omitempty"`
	GroupID int64   `json:"groupId,omitempty"` // only nodes of this group; 0 = all groups
	// Mode is PolicyAuto (the fastest member, tested periodically) or
	// PolicySelect (Selected, chosen by the user).
	Mode     string `json:"mode"`
	Selected int64  `json:"selected,omitempty"`
}

const (
	PolicyCountry = "country"
	PolicyKeyword = "keyword"
	PolicyManual  = "manual"

	PolicyAuto   = "auto"
	PolicySelect = "select"

	// MaxPolicyMembers caps how many nodes one policy group loads into the core.
	MaxPolicyMembers = 100
)

// Keywords splits the keyword condition.
func (p Policy) Keywords() []string {
	var out []string
	for _, k := range strings.FieldsFunc(p.Keyword, func(r rune) bool { return r == ' ' || r == ',' || r == '，' || r == '|' }) {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, strings.ToLower(k))
		}
	}
	return out
}
