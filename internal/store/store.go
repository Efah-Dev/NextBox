package store

import (
	"encoding/json"
	"errors"
	"nextbox/internal/i18n"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"nextbox/internal/vault"
)

type data struct {
	NextID         int64      `json:"nextId"`
	Groups         []*Group   `json:"groups"`
	Profiles       []*Profile `json:"profiles"`
	SelectedID     int64      `json:"selectedId"`
	CurrentGroupID int64      `json:"currentGroupId"`
	Policies       []*Policy  `json:"policies,omitempty"`
}

// Store keeps all user data in memory and persists it as JSON files in Dir.
// Every exported method is safe for concurrent use; returned values are copies.
type Store struct {
	Dir string

	mu       sync.RWMutex
	data     data
	settings Settings

	// see crypt.go
	encryption string
	locked     bool               // waiting for the password; nothing is saved meanwhile
	sealed     vault.Envelope     // the locked file
	key        *vault.PasswordKey // for EncryptPassword
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{Dir: dir, settings: DefaultSettings()}
	if err := s.loadProfiles(); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "settings.json"), &s.settings); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	s.settings.fillDefaults()
	s.ensureGroupLocked()
	return s, nil
}

// ensureGroupLocked keeps at least one group around (a placeholder default
// group) and makes sure the current group exists.
func (s *Store) ensureGroupLocked() {
	if len(s.data.Groups) == 0 {
		// fixed name; the UI shows default groups translated, by their flag
		s.data.Groups = []*Group{{ID: s.nextIDLocked(), Name: "默认分组", Default: true}}
	}
	if s.groupLocked(s.data.CurrentGroupID) == nil {
		s.data.CurrentGroupID = s.data.Groups[0].ID
	}
}

// dropEmptyDefaultLocked removes the placeholder group once the user has a
// group of their own, unless nodes were imported into it.
func (s *Store) dropEmptyDefaultLocked() {
	groups := s.data.Groups[:0]
	for _, g := range s.data.Groups {
		if g.Default && !s.groupHasProfilesLocked(g.ID) {
			continue
		}
		groups = append(groups, g)
	}
	s.data.Groups = groups
}

func (s *Store) groupHasProfilesLocked(id int64) bool {
	for _, p := range s.data.Profiles {
		if p.GroupID == id {
			return true
		}
	}
	return false
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) saveDataLocked() error {
	if s.locked {
		return nil // the real list is still sealed on disk
	}
	b, err := s.encodeDataLocked()
	if err != nil {
		return err
	}
	return writeFileAtomic(s.profilesPath(), b)
}

func (s *Store) nextIDLocked() int64 {
	s.data.NextID++
	return s.data.NextID
}

// ---- settings ----

func (s *Store) Settings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Store) SaveSettings(v Settings) error {
	v.fillDefaults()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = v
	return writeJSON(filepath.Join(s.Dir, "settings.json"), &s.settings)
}

// ---- groups ----

func (s *Store) groupLocked(id int64) *Group {
	for _, g := range s.data.Groups {
		if g.ID == id {
			return g
		}
	}
	return nil
}

func (s *Store) Groups() []Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Group, 0, len(s.data.Groups))
	for _, g := range s.data.Groups {
		out = append(out, *g)
	}
	return out
}

func (s *Store) Group(id int64) (Group, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if g := s.groupLocked(id); g != nil {
		return *g, true
	}
	return Group{}, false
}

func (s *Store) AddGroup(g Group) (Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g.ID = s.nextIDLocked()
	g.Default = false
	s.dropEmptyDefaultLocked()
	s.data.Groups = append(s.data.Groups, &g)
	s.ensureGroupLocked()
	return g, s.saveDataLocked()
}

func (s *Store) UpdateGroup(g Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.groupLocked(g.ID)
	if old == nil {
		return errors.New(i18n.T("分组不存在"))
	}
	*old = g
	return s.saveDataLocked()
}

func (s *Store) DeleteGroup(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups := s.data.Groups[:0]
	for _, g := range s.data.Groups {
		if g.ID != id {
			groups = append(groups, g)
		}
	}
	s.data.Groups = groups
	profiles := s.data.Profiles[:0]
	for _, p := range s.data.Profiles {
		if p.GroupID != id {
			profiles = append(profiles, p)
		} else if p.ID == s.data.SelectedID {
			s.data.SelectedID = 0
		}
	}
	s.data.Profiles = profiles
	// deleting the last group brings back the placeholder default group
	s.ensureGroupLocked()
	return s.saveDataLocked()
}

func (s *Store) CurrentGroupID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.CurrentGroupID
}

func (s *Store) SetCurrentGroup(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groupLocked(id) == nil {
		return errors.New(i18n.T("分组不存在"))
	}
	s.data.CurrentGroupID = id
	return s.saveDataLocked()
}

// ---- profiles ----

func (s *Store) profileLocked(id int64) *Profile {
	for _, p := range s.data.Profiles {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (s *Store) Profiles(groupID int64) []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Profile{}
	for _, p := range s.data.Profiles {
		if groupID == 0 || p.GroupID == groupID {
			out = append(out, *p)
		}
	}
	return out
}

func (s *Store) Profile(id int64) (Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p := s.profileLocked(id); p != nil {
		return *p, true
	}
	return Profile{}, false
}

func (s *Store) ProfilesByIDs(ids []int64) []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Profile{}
	for _, id := range ids {
		if p := s.profileLocked(id); p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// SaveProfile inserts the profile when ID is 0, otherwise replaces it.
func (s *Store) SaveProfile(p Profile) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groupLocked(p.GroupID) == nil {
		return p, errors.New(i18n.T("分组不存在"))
	}
	if p.ID == 0 {
		p.ID = s.nextIDLocked()
		s.data.Profiles = append(s.data.Profiles, &p)
	} else if old := s.profileLocked(p.ID); old != nil {
		*old = p
	} else {
		return p, errors.New(i18n.T("节点不存在"))
	}
	return p, s.saveDataLocked()
}

func (s *Store) AddProfiles(groupID int64, list []Profile) ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groupLocked(groupID) == nil {
		return nil, errors.New(i18n.T("分组不存在"))
	}
	out := make([]Profile, 0, len(list))
	for _, p := range list {
		p := p
		p.ID = s.nextIDLocked()
		p.GroupID = groupID
		s.data.Profiles = append(s.data.Profiles, &p)
		out = append(out, p)
	}
	return out, s.saveDataLocked()
}

// ReplaceGroupProfiles swaps the content of a subscription group. Nodes whose
// name and type survive the update keep their ID, so the selection, chain
// proxies using them and their statistics are preserved.
//
// The group takes the order of list and its manual order is cleared, unless
// keepOrder is set: then surviving nodes keep the order they had and new ones
// are appended (used by background updates, which cannot ask the user).
func (s *Store) ReplaceGroupProfiles(groupID int64, list []Profile, keepOrder bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.groupLocked(groupID)
	if g == nil {
		return errors.New(i18n.T("分组不存在"))
	}
	old := map[string]*Profile{}
	var oldOrder []*Profile
	kept := s.data.Profiles[:0]
	for _, p := range s.data.Profiles {
		if p.GroupID == groupID {
			old[p.Type+"|"+p.Name] = p
			oldOrder = append(oldOrder, p)
		} else {
			kept = append(kept, p)
		}
	}
	fresh := make([]*Profile, 0, len(list))
	byID := map[int64]*Profile{}
	for _, p := range list {
		p := p
		p.GroupID = groupID
		if prev, ok := old[p.Type+"|"+p.Name]; ok {
			p.ID = prev.ID
			p.Upload, p.Download = prev.Upload, prev.Download
			// the server may have moved: keep the country only for an unchanged outbound
			if sameOutbound(prev.Outbound, p.Outbound) {
				p.Country, p.CountryIP, p.CountrySource, p.CountryAt = prev.Country, prev.CountryIP, prev.CountrySource, prev.CountryAt
			}
			delete(old, p.Type+"|"+p.Name)
		} else {
			p.ID = s.nextIDLocked()
		}
		fresh = append(fresh, &p)
		byID[p.ID] = &p
	}
	if keepOrder {
		ordered := make([]*Profile, 0, len(fresh))
		for _, p := range oldOrder {
			if n, ok := byID[p.ID]; ok {
				ordered = append(ordered, n)
				delete(byID, p.ID)
			}
		}
		for _, p := range fresh {
			if _, ok := byID[p.ID]; ok {
				ordered = append(ordered, p)
			}
		}
		fresh = ordered
	} else {
		g.ManualOrder = false
	}
	for _, p := range old {
		if p.ID == s.data.SelectedID {
			s.data.SelectedID = 0
		}
	}
	s.data.Profiles = append(kept, fresh...)
	return s.saveDataLocked()
}

func sameOutbound(a, b map[string]any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

// ReorderProfiles puts the nodes of a group into the order of ids. Nodes of
// the group missing from ids keep their relative order after the listed ones.
func (s *Store) ReorderProfiles(groupID int64, ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.groupLocked(groupID)
	if g == nil {
		return errors.New(i18n.T("分组不存在"))
	}
	var idx []int
	items := map[int64]*Profile{}
	var rest []*Profile
	for i, p := range s.data.Profiles {
		if p.GroupID == groupID {
			idx = append(idx, i)
			items[p.ID] = p
			rest = append(rest, p)
		}
	}
	ordered := make([]*Profile, 0, len(idx))
	for _, id := range ids {
		if p, ok := items[id]; ok {
			ordered = append(ordered, p)
			delete(items, id)
		}
	}
	for _, p := range rest {
		if _, ok := items[p.ID]; ok {
			ordered = append(ordered, p)
		}
	}
	for k, i := range idx {
		s.data.Profiles[i] = ordered[k]
	}
	if g.URL != "" {
		g.ManualOrder = true
	}
	return s.saveDataLocked()
}

// ChainsUsing returns the chain proxies that have any of ids as a member,
// leaving out those in ids themselves.
func (s *Store) ChainsUsing(ids []int64) []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set := map[int64]bool{}
	for _, id := range ids {
		set[id] = true
	}
	out := []Profile{}
	for _, p := range s.data.Profiles {
		if !p.IsChain() || set[p.ID] {
			continue
		}
		for _, m := range p.Chain {
			if set[m] {
				out = append(out, *p)
				break
			}
		}
	}
	return out
}

// Members returns the member nodes of a chain proxy in order; a member that
// no longer exists is returned with ID 0 at its position.
func (s *Store) Members(p Profile) []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Profile, len(p.Chain))
	for i, id := range p.Chain {
		if m := s.profileLocked(id); m != nil {
			out[i] = *m
		}
	}
	return out
}

// SetCountry records where a node is. Like SetLatency it reaches the disk
// with the next save or Flush.
func (s *Store) SetCountry(id int64, code, ip, source string, at int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.profileLocked(id); p != nil {
		p.Country, p.CountryIP, p.CountrySource, p.CountryAt = code, ip, source, at
	}
}

func (s *Store) DeleteProfiles(ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	del := map[int64]bool{}
	for _, id := range ids {
		del[id] = true
	}
	kept := s.data.Profiles[:0]
	for _, p := range s.data.Profiles {
		if !del[p.ID] {
			kept = append(kept, p)
		}
	}
	s.data.Profiles = kept
	if del[s.data.SelectedID] {
		s.data.SelectedID = 0
	}
	return s.saveDataLocked()
}

// SetLatency records a test result; kind is "tcp" or "url".
func (s *Store) SetLatency(id int64, latency int, errMsg string, at int64, kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.profileLocked(id); p != nil {
		p.Latency, p.LatencyError, p.TestedAt, p.LatencyKind = latency, errMsg, at, kind
	}
}

func (s *Store) AddTraffic(id int64, up, down int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.profileLocked(id); p != nil {
		p.Upload += up
		p.Download += down
	}
}

// Flush persists in-memory changes made by SetLatency / AddTraffic.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveDataLocked()
}

func (s *Store) SortGroup(groupID int64, by string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g := s.groupLocked(groupID); g != nil && g.URL != "" {
		g.ManualOrder = true
	}
	var idx []int
	var items []*Profile
	for i, p := range s.data.Profiles {
		if p.GroupID == groupID {
			idx = append(idx, i)
			items = append(items, p)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch by {
		case "latency":
			return latencyKey(a.Latency) < latencyKey(b.Latency)
		case "type":
			return a.Type < b.Type
		default:
			return a.Name < b.Name
		}
	})
	for k, i := range idx {
		s.data.Profiles[i] = items[k]
	}
	return s.saveDataLocked()
}

func latencyKey(l int) int {
	switch {
	case l > 0:
		return l
	case l == 0:
		return 1 << 30
	default:
		return 1<<30 + 1
	}
}

func (s *Store) SelectedID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.SelectedID
}

func (s *Store) SetSelected(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != 0 && s.profileLocked(id) == nil {
		return errors.New(i18n.T("节点不存在"))
	}
	s.data.SelectedID = id
	return s.saveDataLocked()
}

// ---- policy groups ----

func (s *Store) Policies() []Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Policy, 0, len(s.data.Policies))
	for _, p := range s.data.Policies {
		out = append(out, *p)
	}
	return out
}

func (s *Store) policyLocked(id int64) *Policy {
	for _, p := range s.data.Policies {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (s *Store) Policy(id int64) (Policy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p := s.policyLocked(id); p != nil {
		return *p, true
	}
	return Policy{}, false
}

// SavePolicy adds (ID 0) or replaces a policy group.
func (s *Store) SavePolicy(p Policy) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == 0 {
		p.ID = s.nextIDLocked()
		s.data.Policies = append(s.data.Policies, &p)
	} else if old := s.policyLocked(p.ID); old != nil {
		*old = p
	} else {
		return Policy{}, errors.New(i18n.T("分组代理不存在"))
	}
	return p, s.saveDataLocked()
}

func (s *Store) DeletePolicy(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.data.Policies[:0]
	for _, p := range s.data.Policies {
		if p.ID != id {
			kept = append(kept, p)
		}
	}
	s.data.Policies = kept
	return s.saveDataLocked()
}

// SetPolicySelected records the node chosen in a policy group in select mode.
func (s *Store) SetPolicySelected(id, node int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.policyLocked(id)
	if p == nil {
		return errors.New(i18n.T("分组代理不存在"))
	}
	p.Selected = node
	return s.saveDataLocked()
}
