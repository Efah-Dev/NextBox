package store

import "testing"

// The switch takes the DNS the subscription names and drops it again; a DNS
// set by hand stops updates from replacing it.
func TestSubscriptionDNS(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	g, _ := s.AddGroup(Group{Name: "Sub", URL: "https://example.com/sub"})
	g.SuggestedDNS = &GroupDNS{Remote: "https://dns.airport.example/dns-query"}
	if err := s.UpdateGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := s.UseSubscriptionDNS(g.ID, true); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Group(g.ID)
	if got.DNS == nil || got.DNS.Remote != "https://dns.airport.example/dns-query" || got.Clash == nil || !got.Clash.SubDNS {
		t.Fatalf("on: %+v %+v", got.DNS, got.Clash)
	}
	if s.GroupDNS()[g.ID].Remote == "" {
		t.Error("GroupDNS misses the subscription")
	}
	s.UseSubscriptionDNS(g.ID, false)
	if got, _ := s.Group(g.ID); got.DNS != nil || got.Clash.SubDNS {
		t.Fatalf("off: %+v %+v", got.DNS, got.Clash)
	}
	s.SetGroupDNS(g.ID, &GroupDNS{Resolver: " tls://10.0.0.53 "})
	if got, _ := s.Group(g.ID); got.DNS == nil || got.DNS.Resolver != "tls://10.0.0.53" || got.Clash.SubDNS {
		t.Fatalf("by hand: %+v", got.DNS)
	}
	s.SetGroupDNS(g.ID, &GroupDNS{})
	if got, _ := s.Group(g.ID); got.DNS != nil {
		t.Fatal("empty values kept")
	}
	// a subscription that names none cannot be switched on
	s.UpdateGroup(Group{ID: g.ID, Name: "Sub", URL: g.URL})
	s.UseSubscriptionDNS(g.ID, true)
	if got, _ := s.Group(g.ID); got.DNS != nil {
		t.Fatal("switched on without a DNS")
	}
}
