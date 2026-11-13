package store

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestPasswordEncryption(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	g, _ := s.AddGroup(Group{Name: "mine"})
	s.AddProfiles(g.ID, []Profile{{Name: "secret-node", Type: "vless", Outbound: map[string]any{"uuid": "pw-123"}}})
	if err := s.SetEncryption(EncryptPassword, "123"); err == nil {
		t.Error("short password accepted")
	}
	if err := s.SetEncryption(EncryptPassword, "hunter22"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(s.profilesPath())
	if strings.Contains(string(b), "pw-123") || strings.Contains(string(b), "secret-node") {
		t.Fatal("profiles.json is readable")
	}

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Locked() || len(r.Profiles(0)) != 0 {
		t.Fatal("not locked")
	}
	r.Flush() // must not overwrite the sealed file
	if err := r.Unlock("nope"); err == nil {
		t.Error("wrong password accepted")
	}
	if err := r.Unlock("hunter22"); err != nil {
		t.Fatal(err)
	}
	if len(r.Profiles(0)) != 1 || r.Locked() {
		t.Fatal("unlock did not load the nodes")
	}
	// saving keeps the password
	r.SetSelected(r.Profiles(0)[0].ID)
	again, _ := Open(dir)
	if err := again.Unlock("hunter22"); err != nil || again.SelectedID() == 0 {
		t.Fatalf("after save: %v", err)
	}
	if err := again.SetEncryption(EncryptNone, ""); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(s.profilesPath())
	if !strings.Contains(string(b), "secret-node") {
		t.Error("decryption did not write plain text")
	}
}

func TestWindowsEncryption(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip()
	}
	dir := t.TempDir()
	s, _ := Open(dir)
	g, _ := s.AddGroup(Group{Name: "mine"})
	s.AddProfiles(g.ID, []Profile{{Name: "secret-node", Type: "vless"}})
	if err := s.SetEncryption(EncryptWindows, ""); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(s.profilesPath())
	if strings.Contains(string(b), "secret-node") {
		t.Fatal("readable")
	}
	r, err := Open(dir)
	if err != nil || r.Locked() || len(r.Profiles(0)) != 1 || r.Encryption() != EncryptWindows {
		t.Fatalf("reopen: %v", err)
	}
}

func TestImportMerge(t *testing.T) {
	src, _ := Open(t.TempDir())
	sub, _ := src.AddGroup(Group{Name: "airport", URL: "https://sub.example/x"})
	own, _ := src.AddGroup(Group{Name: "vps"})
	a, _ := src.AddProfiles(sub.ID, []Profile{{Name: "JP 1", Type: "vless"}, {Name: "US 2", Type: "trojan"}})
	b, _ := src.AddProfiles(own.ID, []Profile{{Name: "home", Type: "vless"}})
	src.SaveProfile(Profile{GroupID: own.ID, Name: "chain", Type: TypeChain, Chain: []int64{a[0].ID, b[0].ID}})
	pol, _ := src.SavePolicy(Policy{Name: "pick", Match: PolicyManual, Members: []int64{a[1].ID}, Mode: PolicySelect, Selected: a[1].ID})
	raw, _ := src.ExportData()

	dst, _ := Open(t.TempDir())
	have, _ := dst.AddGroup(Group{Name: "airport (here)", URL: "https://sub.example/x"})
	here, _ := dst.AddProfiles(have.ID, []Profile{{Name: "JP 1", Type: "vless"}, {Name: "US 2", Type: "trojan"}})
	policyMap, err := dst.ImportData(raw, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(dst.Groups()) != 2 {
		t.Errorf("groups: %+v (the subscription must not be duplicated)", dst.Groups())
	}
	var chain Profile
	for _, p := range dst.Profiles(0) {
		if p.IsChain() {
			chain = p
		}
	}
	if len(chain.Chain) != 2 || chain.Chain[0] != here[0].ID {
		t.Errorf("chain members not mapped: %v (want first %d)", chain.Chain, here[0].ID)
	}
	p, ok := dst.Policy(policyMap[pol.ID])
	if !ok || p.Selected != here[1].ID || len(p.Members) != 1 || p.Members[0] != here[1].ID {
		t.Errorf("policy not mapped: %+v", p)
	}
	sum, _ := SummarizeData(raw)
	if sum.Subscriptions != 1 || sum.Nodes != 3 || sum.Chains != 1 || sum.Policies != 1 {
		t.Errorf("summary %+v", sum)
	}

	// replace keeps the backup as it is
	if _, err := dst.ImportData(raw, true); err != nil {
		t.Fatal(err)
	}
	if len(dst.Profiles(0)) != 4 || len(dst.Groups()) != 2 {
		t.Errorf("replace: %d nodes, %d groups", len(dst.Profiles(0)), len(dst.Groups()))
	}
}
