package store

import (
	"os"
	"path/filepath"
	"testing"
)

func open(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func node(name string) Profile {
	return Profile{Name: name, Type: "socks", Outbound: map[string]any{"type": "socks", "server": "example.com", "server_port": 1080}}
}

func TestDefaultGroup(t *testing.T) {
	s := open(t, t.TempDir())
	groups := s.Groups()
	if len(groups) != 1 || !groups[0].Default || s.CurrentGroupID() != groups[0].ID {
		t.Fatalf("fresh store: groups %+v, current %d", groups, s.CurrentGroupID())
	}

	// the empty placeholder goes away once the user has a group of their own
	g, err := s.AddGroup(Group{Name: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	if groups = s.Groups(); len(groups) != 1 || groups[0].ID != g.ID || s.CurrentGroupID() != g.ID {
		t.Fatalf("after AddGroup: groups %+v, current %d", groups, s.CurrentGroupID())
	}

	// ... and comes back when the last group is deleted
	if err := s.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	if groups = s.Groups(); len(groups) != 1 || !groups[0].Default || s.CurrentGroupID() != groups[0].ID {
		t.Fatalf("after DeleteGroup: groups %+v, current %d", groups, s.CurrentGroupID())
	}
}

func TestDefaultGroupWithNodesIsKept(t *testing.T) {
	s := open(t, t.TempDir())
	def := s.Groups()[0]
	if _, err := s.AddProfiles(def.ID, []Profile{node("a")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddGroup(Group{Name: "mine"}); err != nil {
		t.Fatal(err)
	}
	if len(s.Groups()) != 2 {
		t.Fatalf("default group with nodes was dropped: %+v", s.Groups())
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	g, _ := s.AddGroup(Group{Name: "sub", URL: "https://example.com/sub"})
	added, err := s.AddProfiles(g.ID, []Profile{node("a"), node("b")})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetSelected(added[1].ID); err != nil {
		t.Fatal(err)
	}
	// latency and traffic only reach the disk with Flush
	s.SetLatency(added[0].ID, 42, "", 100)
	s.AddTraffic(added[0].ID, 10, 20)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	set := s.Settings()
	set.MixedPort, set.RouteMode = 1234, "global"
	if err := s.SaveSettings(set); err != nil {
		t.Fatal(err)
	}

	r := open(t, dir)
	if r.SelectedID() != added[1].ID || r.CurrentGroupID() != g.ID {
		t.Errorf("selection lost: selected %d, group %d", r.SelectedID(), r.CurrentGroupID())
	}
	p, ok := r.Profile(added[0].ID)
	if !ok || p.Latency != 42 || p.Upload != 10 || p.Download != 20 || p.Server() != "example.com:1080" {
		t.Errorf("profile after reload: %+v (found %v)", p, ok)
	}
	if got := r.Settings(); got.MixedPort != 1234 || got.RouteMode != "global" {
		t.Errorf("settings after reload: port %d, mode %q", got.MixedPort, got.RouteMode)
	}
	// IDs keep counting up after a reload
	n, _ := r.SaveProfile(Profile{GroupID: g.ID, Name: "c", Type: "socks", Outbound: node("c").Outbound})
	if n.ID <= added[1].ID {
		t.Errorf("new ID %d collides with existing ones", n.ID)
	}
	if _, err := os.Stat(filepath.Join(dir, "profiles.json.tmp")); err == nil {
		t.Error("temporary file left behind")
	}
}

func TestSettingsDefaults(t *testing.T) {
	dir := t.TempDir()
	// a hand-edited file with holes and nonsense
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"mixedPort": 70000, "theme": "dark", "subUpdateHours": -1}`), 0o644)
	s := open(t, dir).Settings()
	d := DefaultSettings()
	if s.MixedPort != d.MixedPort || s.Theme != "dark" || s.RouteMode != d.RouteMode ||
		s.TestTimeout != d.TestTimeout || s.SubUpdateHours != d.SubUpdateHours {
		t.Fatalf("defaults not applied: %+v", s)
	}
}

func TestBrokenDataFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(`{not json`), 0o644)
	if _, err := Open(dir); err == nil {
		t.Fatal("a corrupt profiles.json must be reported, not silently replaced")
	}
}

func TestReplaceGroupProfiles(t *testing.T) {
	s := open(t, t.TempDir())
	g, _ := s.AddGroup(Group{Name: "sub", URL: "https://example.com/sub"})
	other, _ := s.AddGroup(Group{Name: "manual"})
	old, _ := s.AddProfiles(g.ID, []Profile{node("keep"), node("gone")})
	untouched, _ := s.AddProfiles(other.ID, []Profile{node("keep")})
	s.AddTraffic(old[0].ID, 5, 7)
	s.SetSelected(old[0].ID)

	if err := s.ReplaceGroupProfiles(g.ID, []Profile{node("new"), node("keep")}); err != nil {
		t.Fatal(err)
	}
	list := s.Profiles(g.ID)
	if len(list) != 2 || list[0].Name != "new" || list[1].Name != "keep" {
		t.Fatalf("group content: %+v", list)
	}
	// a node that survives keeps its ID, statistics and the selection
	if list[1].ID != old[0].ID || list[1].Upload != 5 || list[1].Download != 7 || s.SelectedID() != old[0].ID {
		t.Errorf("surviving node: %+v, selected %d", list[1], s.SelectedID())
	}
	if list[0].ID == old[1].ID {
		t.Error("a new node reused the ID of a removed one")
	}
	if got := s.Profiles(other.ID); len(got) != 1 || got[0].ID != untouched[0].ID {
		t.Errorf("another group was changed: %+v", got)
	}

	// the selection is cleared when the selected node disappears
	if err := s.ReplaceGroupProfiles(g.ID, []Profile{node("new")}); err != nil {
		t.Fatal(err)
	}
	if s.SelectedID() != 0 {
		t.Errorf("selection still points at a removed node: %d", s.SelectedID())
	}
}

func TestDeleteClearsSelection(t *testing.T) {
	s := open(t, t.TempDir())
	g, _ := s.AddGroup(Group{Name: "g"})
	list, _ := s.AddProfiles(g.ID, []Profile{node("a"), node("b")})
	s.SetSelected(list[0].ID)
	if err := s.DeleteProfiles([]int64{list[1].ID}); err != nil || s.SelectedID() != list[0].ID {
		t.Fatalf("deleting another node changed the selection: %d (%v)", s.SelectedID(), err)
	}
	if err := s.DeleteProfiles([]int64{list[0].ID}); err != nil || s.SelectedID() != 0 {
		t.Fatalf("selection after deleting the selected node: %d (%v)", s.SelectedID(), err)
	}
	if err := s.SetSelected(999); err == nil {
		t.Error("selecting a missing node did not fail")
	}

	list, _ = s.AddProfiles(g.ID, []Profile{node("c")})
	s.SetSelected(list[0].ID)
	if err := s.DeleteGroup(g.ID); err != nil || s.SelectedID() != 0 || len(s.Profiles(0)) != 0 {
		t.Fatalf("after deleting the group: selected %d, %d profiles (%v)", s.SelectedID(), len(s.Profiles(0)), err)
	}
}

func TestSortGroup(t *testing.T) {
	s := open(t, t.TempDir())
	g, _ := s.AddGroup(Group{Name: "g"})
	other, _ := s.AddGroup(Group{Name: "other"})
	list, _ := s.AddProfiles(g.ID, []Profile{node("slow"), node("untested"), node("failed"), node("fast")})
	s.AddProfiles(other.ID, []Profile{node("zzz")})
	s.SetLatency(list[0].ID, 300, "", 1)
	s.SetLatency(list[2].ID, -1, "timeout", 1)
	s.SetLatency(list[3].ID, 20, "", 1)

	names := func() string {
		out := ""
		for _, p := range s.Profiles(g.ID) {
			out += p.Name + " "
		}
		return out
	}
	// working nodes first by latency, then untested, then failed
	if s.SortGroup(g.ID, "latency"); names() != "fast slow untested failed " {
		t.Errorf("by latency: %s", names())
	}
	if s.SortGroup(g.ID, "name"); names() != "failed fast slow untested " {
		t.Errorf("by name: %s", names())
	}
	if got := s.Profiles(other.ID); len(got) != 1 || got[0].Name != "zzz" {
		t.Errorf("sorting touched another group: %+v", got)
	}
}

// Returned values are copies: changing them must not change the store.
func TestReturnedValuesAreCopies(t *testing.T) {
	s := open(t, t.TempDir())
	g, _ := s.AddGroup(Group{Name: "g"})
	list, _ := s.AddProfiles(g.ID, []Profile{node("a")})
	p, _ := s.Profile(list[0].ID)
	p.Name = "changed"
	groups := s.Groups()
	groups[0].Name = "changed"
	if got, _ := s.Profile(list[0].ID); got.Name != "a" {
		t.Errorf("profile name changed through a copy: %q", got.Name)
	}
	if got, _ := s.Group(g.ID); got.Name != "g" {
		t.Errorf("group name changed through a copy: %q", got.Name)
	}
}
