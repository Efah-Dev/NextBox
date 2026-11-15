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

// Steps of the route that are not rules of their own but have a place among
// them: ad blocking, and a scene (Values[0] is its ID). They keep the order
// the user gives them; their state lives in BlockAds and Scenes.
const (
	KindAds   = "ads"
	KindScene = "scene"
)

// IsStep reports whether r only marks the place of ad blocking or a scene.
func (r Rule) IsStep() bool { return r.Kind == KindAds || r.Kind == KindScene }

// RuleKinds are the things a rule can match.
// "package" (an Android app) is only used by NextBox for Android.
var RuleKinds = []string{"geosite", "suffix", "domain", "keyword", "regex", "geoip", "cidr", "process", "package"}

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

// The levels that reproduce the light and dark themes of earlier versions,
// on the scales of ThemeScale.
const (
	DefaultDarkLevel  = 60
	DefaultLightLevel = 20
	ThemeScale        = 2
)

// migrateLevels converts levels of the first scale (v2.0), whose end points
// were closer together, so that the colours stay as they were: the light
// scale gained a brighter part below DefaultLightLevel and reaches 1.55
// times as far towards grey; the dark scale reaches 1.55 times as far
// towards the lighter end (the black end is new and starts the same way).
func (s *Settings) migrateLevels() {
	if s.ThemeScale >= ThemeScale {
		return
	}
	s.LightLevel = DefaultLightLevel + int(float64(s.LightLevel)*float64(100-DefaultLightLevel)/155+0.5)
	if s.DarkLevel < DefaultDarkLevel {
		s.DarkLevel = DefaultDarkLevel - int(float64(DefaultDarkLevel-s.DarkLevel)/1.55+0.5)
	}
	s.ThemeScale = ThemeScale
}

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
	"package": "package", "package_name": "package", "app": "package",
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
		if r.IsStep() {
			out = append(out, r) // see placeSteps
			continue
		}
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

// placeSteps gives ad blocking and every scene exactly one place among the
// rules. Data of v2.0 and of the Android version has none: ad blocking then
// comes first and the scenes last, in the order they were checked in.
func placeSteps(rules []Rule, scenes []Scene) []Rule {
	known := map[string]bool{}
	for _, sc := range scenes {
		known[sc.ID] = true
	}
	seen := map[string]bool{}
	out := make([]Rule, 0, len(rules)+len(scenes)+1)
	for _, r := range rules {
		if !r.IsStep() {
			out = append(out, r)
			continue
		}
		scene := ""
		if r.Kind == KindScene {
			if len(r.Values) > 0 {
				scene = strings.ToLower(strings.TrimSpace(r.Values[0]))
			}
			if !known[scene] {
				continue
			}
		}
		if key := r.Kind + ":" + scene; !seen[key] {
			seen[key] = true
			out = append(out, stepRule(r.Kind, scene))
		}
	}
	if !seen[KindAds+":"] {
		out = append([]Rule{stepRule(KindAds, "")}, out...)
	}
	for _, sc := range sceneOrder {
		if known[sc] && !seen[KindScene+":"+sc] {
			out = append(out, stepRule(KindScene, sc))
		}
	}
	return out
}

// sceneOrder is the order the scenes were checked in before they could be
// moved (v2.0): the specific ones first, bypassing mainland China last.
var sceneOrder = []string{"ai", "media", "games", "cn"}

func stepRule(kind, scene string) Rule {
	r := Rule{Kind: kind, Values: []string{}}
	if kind == KindScene {
		r.Values = []string{scene}
	}
	return r
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
