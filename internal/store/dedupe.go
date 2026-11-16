package store

import (
	"encoding/json"
)

// Duplicate nodes: nodes that connect the same way (type and outbound equal
// apart from the name), wherever they are. Chain proxies are never counted.

// DupKey identifies how a node connects; "" for a chain proxy.
func DupKey(p *Profile) string {
	if p.IsChain() || len(p.Outbound) == 0 {
		return ""
	}
	ob := make(map[string]any, len(p.Outbound))
	for k, v := range p.Outbound {
		if k != "tag" {
			ob[k] = v
		}
	}
	b, err := json.Marshal(ob) // map keys come out sorted
	if err != nil {
		return ""
	}
	return p.Type + "|" + string(b)
}

// DupSet is one node that is kept and the copies of it that go.
type DupSet struct {
	Keep    Profile   `json:"keep"`
	Drop    []Profile `json:"drop"`
	Running bool      `json:"running,omitempty"` // Keep is the node in use
}

// FindDuplicates lists the duplicate nodes of all groups. The node that is
// kept is, in this order: the one in use (running, or selected), one in a
// group of one's own (not a subscription, which would bring the copies back
// on its next update), and the first in the order of the groups and nodes.
func (s *Store) FindDuplicates(running int64) []DupSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.findDuplicatesLocked(running)
}

func (s *Store) findDuplicatesLocked(running int64) []DupSet {
	manual := map[int64]bool{}
	groupPos := map[int64]int{}
	for i, g := range s.data.Groups {
		manual[g.ID] = g.URL == ""
		groupPos[g.ID] = i
	}
	inUse := running
	if inUse == 0 {
		inUse = s.data.SelectedID
	}
	rank := func(p *Profile) int {
		switch {
		case p.ID == inUse:
			return 0
		case manual[p.GroupID]:
			return 1
		}
		return 2
	}
	// the nodes in display order: groups in their order, nodes in theirs
	order := make([]*Profile, 0, len(s.data.Profiles))
	for _, g := range s.data.Groups {
		for _, p := range s.data.Profiles {
			if p.GroupID == g.ID {
				order = append(order, p)
			}
		}
	}
	sets := map[string][]*Profile{}
	var keys []string
	for _, p := range order {
		k := DupKey(p)
		if k == "" {
			continue
		}
		if _, ok := sets[k]; !ok {
			keys = append(keys, k)
		}
		sets[k] = append(sets[k], p)
	}
	var out []DupSet
	for _, k := range keys {
		list := sets[k]
		if len(list) < 2 {
			continue
		}
		best := 0
		for i, p := range list {
			if rank(p) < rank(list[best]) {
				best = i
			}
		}
		d := DupSet{Keep: *list[best], Running: list[best].ID == inUse}
		for i, p := range list {
			if i != best {
				d.Drop = append(d.Drop, *p)
			}
		}
		out = append(out, d)
	}
	return out
}

// DeleteDuplicates removes the copies FindDuplicates lists. Chain proxies and
// policy groups that used a copy use the node that is kept instead. It
// returns how many nodes were removed.
func (s *Store) DeleteDuplicates(running int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sets := s.findDuplicatesLocked(running)
	if len(sets) == 0 {
		return 0, nil
	}
	to := map[int64]int64{} // removed → kept
	for _, d := range sets {
		for _, p := range d.Drop {
			to[p.ID] = d.Keep.ID
		}
	}
	kept := s.data.Profiles[:0]
	for _, p := range s.data.Profiles {
		if _, gone := to[p.ID]; !gone {
			kept = append(kept, p)
		}
	}
	s.data.Profiles = kept
	for _, p := range s.data.Profiles {
		for i, id := range p.Chain {
			if k, ok := to[id]; ok {
				p.Chain[i] = k
			}
		}
	}
	for _, pol := range s.data.Policies {
		members := pol.Members[:0]
		seen := map[int64]bool{}
		for _, id := range pol.Members {
			if k, ok := to[id]; ok {
				id = k
			}
			if !seen[id] {
				seen[id] = true
				members = append(members, id)
			}
		}
		pol.Members = members
		if k, ok := to[pol.Selected]; ok {
			pol.Selected = k
		}
	}
	if k, ok := to[s.data.SelectedID]; ok {
		s.data.SelectedID = k
	}
	return len(to), s.saveDataLocked()
}

// DedupeList drops the nodes of a subscription that connect the same way as
// one before them in the list.
func DedupeList(list []Profile) []Profile {
	seen := map[string]bool{}
	out := list[:0:0]
	for _, p := range list {
		k := DupKey(&p)
		if k != "" && seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}
