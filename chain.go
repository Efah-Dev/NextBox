package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/core"
	"nextbox/internal/geo"
	"nextbox/internal/i18n"
	"nextbox/internal/store"
)

// ChainHop describes one member of a chain proxy for the UI.
type ChainHop struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	GroupID       int64  `json:"groupId"`
	Country       string `json:"country,omitempty"`
	CountrySource string `json:"countrySource,omitempty"`
	Missing       bool   `json:"missing,omitempty"` // deleted, or renamed by a subscription update
}

// ProfileView is a node as the UI sees it: a chain proxy also carries its
// members, which may live in other groups.
type ProfileView struct {
	store.Profile
	Members []ChainHop `json:"members,omitempty"`
}

func (a *App) view(p store.Profile) ProfileView {
	v := ProfileView{Profile: p}
	if !p.IsChain() {
		return v
	}
	for i, m := range a.store.Members(p) {
		if m.ID == 0 {
			v.Members = append(v.Members, ChainHop{ID: p.Chain[i], Missing: true})
			continue
		}
		v.Members = append(v.Members, ChainHop{
			ID: m.ID, Name: m.Name, Type: m.Type, GroupID: m.GroupID,
			Country: m.Country, CountrySource: m.CountrySource,
		})
	}
	return v
}

func (a *App) views(list []store.Profile) []ProfileView {
	out := make([]ProfileView, 0, len(list))
	for _, p := range list {
		out = append(out, a.view(p))
	}
	return out
}

// chainError explains why a chain proxy cannot be used, or returns nil (also
// for plain nodes).
func (a *App) chainError(p store.Profile) error {
	if !p.IsChain() {
		return nil
	}
	_, err := config.Expand(p, "check", a.store.Profile, nil)
	return err
}

// ChainInput is a chain proxy as the editor sends it. GroupID -1 asks for a
// new group to hold it.
type ChainInput struct {
	ID      int64   `json:"id"`
	GroupID int64   `json:"groupId"`
	Name    string  `json:"name"`
	Chain   []int64 `json:"chain"`
	// The result of testing exactly these hops in the editor (TestChain),
	// kept so the list does not show "untested" right after saving.
	Latency int           `json:"latency,omitempty"`
	Exit    *geo.Location `json:"exit,omitempty"`
}

// checkMembers validates the member list of a chain proxy.
func (a *App) checkMembers(ids []int64) error {
	if len(ids) < store.MinChainLength || len(ids) > store.MaxChainLength {
		return fmt.Errorf(i18n.T("链式代理需要 %d 到 %d 个节点"), store.MinChainLength, store.MaxChainLength)
	}
	seen := map[int64]bool{}
	for i, id := range ids {
		m, ok := a.store.Profile(id)
		switch {
		case !ok:
			return fmt.Errorf(i18n.T("第 %d 跳的节点已不存在，请替换"), i+1)
		case m.IsChain():
			return fmt.Errorf(i18n.T("第 %d 跳不能是另一个链式代理"), i+1)
		case seen[id]:
			return fmt.Errorf(i18n.T("“%s”在链路里出现了两次"), m.Name)
		}
		seen[id] = true
	}
	return nil
}

// SaveChain creates or updates a chain proxy. It only lives in a manual
// group: a subscription update replaces the whole group.
func (a *App) SaveChain(in ChainInput) (ProfileView, error) {
	if err := a.checkMembers(in.Chain); err != nil {
		return ProfileView{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = i18n.T("链式代理")
	}
	groupID := in.GroupID
	if groupID == -1 {
		g, err := a.store.AddGroup(store.Group{Name: i18n.T("链式代理")})
		if err != nil {
			return ProfileView{}, err
		}
		groupID = g.ID
		a.emitState()
	}
	g, ok := a.store.Group(groupID)
	if !ok {
		return ProfileView{}, errors.New(i18n.T("分组不存在"))
	}
	if g.URL != "" {
		return ProfileView{}, errors.New(i18n.T("链式代理只能保存在手动分组里"))
	}
	p := store.Profile{ID: in.ID, GroupID: groupID, Name: name, Type: store.TypeChain, Outbound: map[string]any{}, Chain: in.Chain}
	if in.ID != 0 {
		old, ok := a.store.Profile(in.ID)
		if !ok || !old.IsChain() {
			return ProfileView{}, errors.New(i18n.T("节点不存在"))
		}
		p.Upload, p.Download = old.Upload, old.Download
		if sameJSON(old.Chain, in.Chain) {
			p.Latency, p.LatencyError, p.TestedAt = old.Latency, old.LatencyError, old.TestedAt
			p.Country, p.CountryIP, p.CountrySource, p.CountryAt = old.Country, old.CountryIP, old.CountrySource, old.CountryAt
		}
		if old.GroupID != groupID {
			defer a.emitProfiles(old.GroupID)
		}
	}
	now := time.Now().Unix()
	if in.Latency != 0 {
		p.Latency, p.LatencyError, p.TestedAt = in.Latency, "", now
	}
	if in.Exit != nil && in.Exit.Country != "" {
		p.Country, p.CountryIP, p.CountrySource, p.CountryAt = in.Exit.Country, in.Exit.IP, store.CountryExit, now
	}
	saved, err := a.store.SaveProfile(p)
	if err != nil {
		return ProfileView{}, err
	}
	a.emitProfiles(groupID)
	a.mu.Lock()
	isRunningNode := a.runningID == saved.ID && a.proc != nil
	a.mu.Unlock()
	if isRunningNode {
		go a.restartIfRunning()
	}
	go a.locateServers([]int64{saved.ID}, false)
	return a.view(saved), nil
}

// ChainsUsing returns the names of the chain proxies that would stop working
// if the nodes ids were deleted.
func (a *App) ChainsUsing(ids []int64) []string {
	names := []string{}
	for _, p := range a.store.ChainsUsing(ids) {
		names = append(names, p.Name)
	}
	return names
}

// TestChain measures a chain proxy that is still being edited, through a
// temporary core of its own.
func (a *App) TestChain(ids []int64) (core.LatencyResult, error) {
	if err := a.checkMembers(ids); err != nil {
		return core.LatencyResult{}, err
	}
	s := a.store.Settings()
	const probeID = 1 << 40 // not a real node
	probe := store.Profile{ID: probeID, Name: i18n.T("链式代理"), Type: store.TypeChain, Chain: ids}
	ctx, cancel := context.WithTimeout(a.ctx, time.Duration(s.TestTimeout)*time.Millisecond*3+15*time.Second)
	defer cancel()
	result := core.LatencyResult{ID: probeID, Latency: -1, Error: i18n.T("超时")}
	core.TestURL(ctx, a.corePath(), filepath.Join(a.cacheDir(), "chain"), s, []store.Profile{probe}, core.Progress{
		Start:  func(int64) {},
		Result: func(r core.LatencyResult) { result = r },
		Member: a.store.Profile,
		Locate: func(int64) bool { return s.CountrySource == store.CountryExit },
	})
	a.clearCache()
	return result, nil
}

func sameJSON(a, b any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

func idsOf(list []store.Profile) []int64 {
	ids := make([]int64, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	return ids
}
