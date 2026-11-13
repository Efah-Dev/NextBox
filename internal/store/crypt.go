package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nextbox/internal/i18n"
	"nextbox/internal/vault"
)

// profiles.json can be kept encrypted (设置 → 数据与备份): bound to the
// Windows account, which needs no password, or with a start-up password.
// settings.json holds nothing secret and stays readable.
const (
	EncryptNone     = ""
	EncryptWindows  = "windows"
	EncryptPassword = "password"
)

func (s *Store) profilesPath() string { return filepath.Join(s.Dir, "profiles.json") }

// loadProfiles reads profiles.json. A password-protected file leaves the
// store locked until Unlock.
func (s *Store) loadProfiles() error {
	b, err := os.ReadFile(s.profilesPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	env, encrypted := vault.Parse(b)
	switch {
	case !encrypted:
		return json.Unmarshal(b, &s.data)
	case env.KDF == vault.KDFWindows:
		plain, err := vault.OpenWindows(env)
		if err != nil {
			return err
		}
		s.encryption = EncryptWindows
		return json.Unmarshal(plain, &s.data)
	default:
		s.encryption = EncryptPassword
		s.locked = true
		s.sealed = env
		return nil
	}
}

// encodeData returns profiles.json as it is written to disk.
func (s *Store) encodeDataLocked() ([]byte, error) {
	plain, err := json.MarshalIndent(&s.data, "", "  ")
	if err != nil {
		return nil, err
	}
	switch s.encryption {
	case EncryptWindows:
		return vault.SealWindows(plain)
	case EncryptPassword:
		if s.key == nil {
			return nil, errors.New(i18n.T("配置已锁定"))
		}
		return s.key.Seal(plain)
	}
	return plain, nil
}

func writeFileAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Encryption reports how profiles.json is stored (Encrypt*).
func (s *Store) Encryption() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.encryption
}

// Locked reports whether the node list waits for its password.
func (s *Store) Locked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.locked
}

// Unlock opens a password-protected node list.
func (s *Store) Unlock(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.locked {
		return nil
	}
	plain, key, err := vault.Decrypt(s.sealed, password)
	if errors.Is(err, vault.ErrPassword) {
		return errors.New(i18n.T("密码错误"))
	}
	if err != nil {
		return err
	}
	var d data
	if err := json.Unmarshal(plain, &d); err != nil {
		return err
	}
	s.data, s.key, s.locked, s.sealed = d, key, false, vault.Envelope{}
	s.ensureGroupLocked()
	return nil
}

// SetEncryption changes how profiles.json is stored and rewrites it.
// password is needed for EncryptPassword (also to change it).
func (s *Store) SetEncryption(mode, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return errors.New(i18n.T("配置已锁定"))
	}
	switch mode {
	case EncryptNone, EncryptWindows:
		s.key = nil
	case EncryptPassword:
		if len([]rune(password)) < 6 {
			return errors.New(i18n.T("密码至少 6 位"))
		}
		key, err := vault.NewPasswordKey(password)
		if err != nil {
			return err
		}
		s.key = key
	default:
		return errors.New(i18n.T("未知的加密方式"))
	}
	old := s.encryption
	s.encryption = mode
	if err := s.saveDataLocked(); err != nil {
		s.encryption = old
		return err
	}
	return nil
}

// ---- backup ----

// ExportData returns the node list (groups, nodes, policy groups) as plain
// JSON, the "profiles" part of a backup.
func (s *Store) ExportData() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.locked {
		return nil, errors.New(i18n.T("配置已锁定"))
	}
	return json.Marshal(&s.data)
}

// DataSummary counts what a node list holds.
type DataSummary struct {
	Groups        int `json:"groups"`
	Subscriptions int `json:"subscriptions"`
	Nodes         int `json:"nodes"`
	Chains        int `json:"chains"`
	Policies      int `json:"policies"`
}

// SummarizeData reads a node list from a backup.
func SummarizeData(raw []byte) (DataSummary, error) {
	var d data
	if err := json.Unmarshal(raw, &d); err != nil {
		return DataSummary{}, err
	}
	sum := DataSummary{Groups: len(d.Groups), Policies: len(d.Policies)}
	for _, g := range d.Groups {
		if g.URL != "" {
			sum.Subscriptions++
		}
	}
	for _, p := range d.Profiles {
		if p.IsChain() {
			sum.Chains++
		} else {
			sum.Nodes++
		}
	}
	return sum, nil
}

// ImportData adds the node list of a backup. With replace the current list
// is dropped first and the backup's IDs are kept; otherwise everything gets
// new IDs, and a subscription that is already there is not added twice (its
// nodes are matched by name). It returns how policy group IDs were mapped,
// for the rules of the backup's settings.
func (s *Store) ImportData(raw []byte, replace bool) (map[int64]int64, error) {
	var in data
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return nil, errors.New(i18n.T("配置已锁定"))
	}
	policyMap := map[int64]int64{}
	if replace {
		maxID := in.NextID
		for _, g := range in.Groups {
			maxID = max(maxID, g.ID)
		}
		for _, p := range in.Profiles {
			maxID = max(maxID, p.ID)
		}
		for _, p := range in.Policies {
			maxID = max(maxID, p.ID)
			policyMap[p.ID] = p.ID
		}
		in.NextID = maxID
		s.data = in
		s.ensureGroupLocked()
		return policyMap, s.saveDataLocked()
	}

	groupMap := map[int64]int64{}
	profileMap := map[int64]int64{}
	existing := map[string]*Group{} // subscriptions by URL
	for _, g := range s.data.Groups {
		if g.URL != "" {
			existing[strings.TrimSpace(g.URL)] = g
		}
	}
	matched := map[int64]bool{} // imported groups that are already here
	for _, g := range in.Groups {
		if have, ok := existing[strings.TrimSpace(g.URL)]; ok && g.URL != "" {
			groupMap[g.ID] = have.ID
			matched[g.ID] = true
			continue
		}
		ng := *g
		ng.ID = s.nextIDLocked()
		ng.Default = false
		groupMap[g.ID] = ng.ID
		s.data.Groups = append(s.data.Groups, &ng)
	}
	byName := map[string]int64{} // nodes of matched subscriptions
	for _, p := range s.data.Profiles {
		byName[groupKey(p.GroupID, p.Type, p.Name)] = p.ID
	}
	var added []*Profile
	for _, p := range in.Profiles {
		gid, ok := groupMap[p.GroupID]
		if !ok {
			continue
		}
		if matched[p.GroupID] {
			if id, ok := byName[groupKey(gid, p.Type, p.Name)]; ok {
				profileMap[p.ID] = id
			}
			continue
		}
		np := *p
		np.ID = s.nextIDLocked()
		np.GroupID = gid
		profileMap[p.ID] = np.ID
		added = append(added, &np)
	}
	for _, p := range added {
		for i, m := range p.Chain {
			p.Chain[i] = profileMap[m] // 0 when lost: the chain shows as broken
		}
		s.data.Profiles = append(s.data.Profiles, p)
	}
	for _, p := range in.Policies {
		np := *p
		np.ID = s.nextIDLocked()
		np.GroupID = groupMap[p.GroupID]
		np.Selected = profileMap[p.Selected]
		np.Members = nil
		for _, m := range p.Members {
			if id := profileMap[m]; id != 0 {
				np.Members = append(np.Members, id)
			}
		}
		policyMap[p.ID] = np.ID
		s.data.Policies = append(s.data.Policies, &np)
	}
	s.dropEmptyDefaultLocked()
	s.ensureGroupLocked()
	return policyMap, s.saveDataLocked()
}

func groupKey(group int64, typ, name string) string {
	return strconv.FormatInt(group, 10) + "|" + typ + "|" + name
}
