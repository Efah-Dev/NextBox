package store

import "testing"

func socks(name, server string) Profile {
	return Profile{Name: name, Type: "socks", Outbound: map[string]any{"type": "socks", "tag": name, "server": server, "server_port": 1080}}
}

func TestDuplicates(t *testing.T) {
	s := open(t, t.TempDir())
	subA, _ := s.AddGroup(Group{Name: "sub a", URL: "https://a.example/sub"})
	mine, _ := s.AddGroup(Group{Name: "mine"})
	a, err := s.AddProfiles(subA.ID, []Profile{socks("HK 01", "1.1.1.1"), socks("HK 01 copy", "1.1.1.1"), socks("JP", "2.2.2.2"), socks("US", "3.3.3.3")})
	if err != nil {
		t.Fatal(err)
	}
	m, _ := s.AddProfiles(mine.ID, []Profile{socks("my JP", "2.2.2.2"), socks("my US", "3.3.3.3")})
	// a chain through a copy, a policy with a copy, and a copy selected
	chain, _ := s.SaveProfile(Profile{GroupID: mine.ID, Name: "chain", Type: TypeChain, Chain: []int64{a[2].ID, a[0].ID}})
	pol, _ := s.SavePolicy(Policy{Name: "p", Match: PolicyManual, Members: []int64{a[3].ID, m[1].ID, a[1].ID}, Mode: PolicySelect, Selected: a[3].ID})
	if err := s.SetSelected(a[1].ID); err != nil {
		t.Fatal(err)
	}

	sets := s.FindDuplicates(0)
	if len(sets) != 3 {
		t.Fatalf("want 3 sets, got %+v", sets)
	}
	keep := map[string]string{}
	for _, d := range sets {
		for _, p := range d.Drop {
			keep[p.Name] = d.Keep.Name
		}
	}
	// the selected copy is kept over the first one; a group of one's own wins
	// over a subscription
	want := map[string]string{"HK 01": "HK 01 copy", "JP": "my JP", "US": "my US"}
	if len(keep) != len(want) {
		t.Fatalf("dropped %v", keep)
	}
	for k, v := range want {
		if keep[k] != v {
			t.Errorf("%s: kept %q, want %q", k, keep[k], v)
		}
	}

	n, err := s.DeleteDuplicates(0)
	if err != nil || n != 3 {
		t.Fatalf("deleted %d, %v", n, err)
	}
	if len(s.Profiles(subA.ID)) != 1 || len(s.Profiles(mine.ID)) != 3 {
		t.Fatalf("left: %+v / %+v", s.Profiles(subA.ID), s.Profiles(mine.ID))
	}
	c := s.ProfilesByIDs([]int64{chain.ID})[0]
	if c.Chain[0] != m[0].ID || c.Chain[1] != a[1].ID {
		t.Errorf("chain now %v", c.Chain)
	}
	for _, p := range s.Policies() {
		if p.ID == pol.ID && (len(p.Members) != 2 || p.Members[0] != m[1].ID || p.Members[1] != a[1].ID || p.Selected != m[1].ID) {
			t.Errorf("policy now %+v", p)
		}
	}
	if s.SelectedID() != a[1].ID {
		t.Errorf("selected %d", s.SelectedID())
	}
	if again := s.FindDuplicates(0); len(again) != 0 {
		t.Errorf("still duplicates: %+v", again)
	}
}

func TestDedupeList(t *testing.T) {
	in := []Profile{socks("a", "1.1.1.1"), socks("b", "1.1.1.1"), socks("c", "2.2.2.2"), {Name: "chain", Type: TypeChain}, {Name: "chain 2", Type: TypeChain}}
	out := DedupeList(in)
	if len(out) != 4 || out[0].Name != "a" || out[1].Name != "c" {
		t.Fatalf("got %+v", out)
	}
}
