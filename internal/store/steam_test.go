package store

import "testing"

// The Steam rule of v2.2 (geosite "steam", everything one way) becomes the
// store and the downloads as two rules with the same target and state.
func TestSplitSteam(t *testing.T) {
	s := DefaultSettings()
	s.Rules = []Rule{
		{Kind: "suffix", Values: []string{"a.example"}, Target: TargetProxy},
		{Kind: "geosite", Values: []string{"steam"}, Target: TargetDirect, App: SteamStoreApp},
	}
	s.Normalize()
	var dl, st *Rule
	for i, r := range s.Rules {
		switch r.App {
		case SteamDownloadApp:
			dl = &s.Rules[i]
		case SteamStoreApp:
			st = &s.Rules[i]
		}
	}
	if dl == nil || st == nil {
		t.Fatalf("rules = %+v", s.Rules)
	}
	if dl.Kind != "suffix" || dl.Target != TargetDirect || st.Kind != "suffix" || st.Target != TargetDirect {
		t.Errorf("download %+v, store %+v", *dl, *st)
	}
	// once split, it stays as it is
	before := len(s.Rules)
	s.Normalize()
	if len(s.Rules) != before {
		t.Errorf("split twice: %d rules, then %d", before, len(s.Rules))
	}
}

// A rule set keeps its name and target; its items are tidied, merged by
// kind, and the ones of unknown kinds or without values go.
func TestCleanRuleSet(t *testing.T) {
	s := DefaultSettings()
	s.Rules = []Rule{{Kind: KindSet, Name: "  游戏 ", Target: TargetDirect, Items: []RuleItem{
		{Kind: "suffix", Values: []string{".Example.COM", "example.com"}},
		{Kind: "cidr", Values: []string{"1.2.3.4", "/32", "nope"}},
		{Kind: "suffix", Values: []string{"b.example"}},
		{Kind: "nope", Values: []string{"x"}},
		{Kind: KindURL, Values: []string{"ftp://x/y.srs", "https://example.com/r.srs"}},
		{Kind: "process", Values: []string{}},
	}}}
	s.Normalize()
	var set Rule
	for _, r := range s.Rules {
		if r.IsSet() {
			set = r
		}
	}
	if set.Name != "游戏" || set.Target != TargetDirect || len(set.Items) != 3 {
		t.Fatalf("set = %+v", set)
	}
	want := map[string][]string{
		"suffix": {"example.com", "b.example"},
		"cidr":   {"1.2.3.4/32"},
		KindURL:  {"https://example.com/r.srs"},
	}
	for _, it := range set.Items {
		if w := want[it.Kind]; len(w) != len(it.Values) || w[0] != it.Values[0] {
			t.Errorf("%s = %v, want %v", it.Kind, it.Values, w)
		}
	}
	if set.Empty() {
		t.Error("Empty")
	}
	if !(Rule{Kind: KindSet, Name: "x"}).Empty() || !(Rule{Kind: "suffix"}).Empty() || (Rule{Kind: KindAds}).Empty() {
		t.Error("Empty of empty rules / steps")
	}
}
