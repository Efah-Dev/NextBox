package main

// Groups, subscriptions, nodes and latency tests.

import (
	"context"
	"errors"
	"fmt"
	"nextbox/internal/i18n"
	"strings"
	"time"

	"nextbox/internal/core"
	"nextbox/internal/link"
	"nextbox/internal/store"
	"nextbox/internal/sub"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------------------------------------------------------------- groups

func (a *App) SetCurrentGroup(id int64) error {
	err := a.store.SetCurrentGroup(id)
	a.emitState()
	return err
}

func (a *App) AddGroup(name string) (store.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Group{}, errors.New(i18n.T("请输入分组名称"))
	}
	g, err := a.store.AddGroup(store.Group{Name: name})
	if err == nil {
		a.store.SetCurrentGroup(g.ID)
	}
	a.emitState()
	return g, err
}

func (a *App) AddSubscription(name, url string) (string, error) {
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", errors.New(i18n.T("订阅地址必须以 http:// 或 https:// 开头"))
	}
	if strings.TrimSpace(name) == "" {
		name = i18n.T("新订阅")
	}
	g, err := a.store.AddGroup(store.Group{Name: strings.TrimSpace(name), URL: url})
	if err != nil {
		return "", err
	}
	a.store.SetCurrentGroup(g.ID)
	a.emitState()
	return a.UpdateSubscription(g.ID)
}

func (a *App) EditGroup(g store.Group) error {
	old, ok := a.store.Group(g.ID)
	if !ok {
		return errors.New(i18n.T("分组不存在"))
	}
	old.Name = strings.TrimSpace(g.Name)
	old.URL = strings.TrimSpace(g.URL)
	old.UserAgent = strings.TrimSpace(g.UserAgent)
	err := a.store.UpdateGroup(old)
	a.emitState()
	return err
}

func (a *App) DeleteGroup(id int64) error {
	err := a.store.DeleteGroup(id)
	a.emitState()
	return err
}

// UpdateSubscription downloads a subscription again. The nodes take the
// order of the subscription; the UI asks first when they were reordered.
func (a *App) UpdateSubscription(groupID int64) (string, error) {
	return a.updateSubscription(groupID, false)
}

// updateSubscription with keepOrder keeps a manual order (background updates).
func (a *App) updateSubscription(groupID int64, keepOrder bool) (string, error) {
	g, ok := a.store.Group(groupID)
	if !ok {
		return "", errors.New(i18n.T("分组不存在"))
	}
	if g.URL == "" {
		return "", errors.New(i18n.T("这个分组不是订阅"))
	}
	s := a.store.Settings()
	ua := g.UserAgent
	if ua == "" {
		ua = s.SubUserAgent
	}
	proxy := ""
	if s.SubUpdateByProxy && a.isRunning() {
		proxy = "http://" + proxyAddr(s)
	}
	a.log(i18n.T("正在更新订阅: ") + g.Name)
	res, err := sub.Fetch(a.ctx, g.URL, ua, proxy)
	if err != nil {
		a.log(err.Error())
		return "", err
	}
	profiles := make([]store.Profile, 0, len(res.Nodes))
	for _, n := range res.Nodes {
		profiles = append(profiles, store.Profile{Name: n.Name, Type: n.Type(), Outbound: n.Outbound})
	}
	if err := a.store.ReplaceGroupProfiles(groupID, profiles, keepOrder && g.ManualOrder); err != nil {
		return "", err
	}
	go a.locateServers(idsOf(a.store.Profiles(groupID)), false)
	g, _ = a.store.Group(groupID)
	g.UpdatedAt = time.Now().Unix()
	if res.Usage != nil {
		g.Usage = res.Usage
	}
	a.store.UpdateGroup(g)
	msg := fmt.Sprintf(i18n.T("“%s”更新完成：%d 个节点"), g.Name, len(profiles))
	msg += a.skipNote(res.Skipped)
	a.log(msg)
	a.emitState()
	a.emitProfiles(groupID)
	return msg, nil
}

func (a *App) UpdateAllSubscriptions() (string, error) {
	var ok, failed int
	for _, g := range a.store.Groups() {
		if g.URL == "" {
			continue
		}
		if _, err := a.UpdateSubscription(g.ID); err != nil {
			failed++
		} else {
			ok++
		}
	}
	if ok+failed == 0 {
		return "", errors.New(i18n.T("还没有添加订阅"))
	}
	return fmt.Sprintf(i18n.T("已更新 %d 个订阅，失败 %d 个"), ok, failed), nil
}

// ---------------------------------------------------------------- profiles

func (a *App) GetProfiles(groupID int64) []ProfileView {
	return a.views(a.store.Profiles(groupID))
}

func (a *App) GetProfile(id int64) (ProfileView, error) {
	p, ok := a.store.Profile(id)
	if !ok {
		return ProfileView{}, errors.New(i18n.T("节点不存在"))
	}
	return a.view(p), nil
}

// ImportText imports share links / subscription content into a group.
func (a *App) ImportText(text string, groupID int64) (string, error) {
	nodes, skipped, err := link.ParseContent(text)
	if err != nil {
		a.skipNote(skipped)
		return "", err
	}
	list := make([]store.Profile, 0, len(nodes))
	for _, n := range nodes {
		list = append(list, store.Profile{Name: n.Name, Type: n.Type(), Outbound: n.Outbound})
	}
	added, err := a.store.AddProfiles(groupID, list)
	if err != nil {
		return "", err
	}
	a.emitProfiles(groupID)
	go a.locateServers(idsOf(added), false)
	msg := fmt.Sprintf(i18n.T("已导入 %d 个节点"), len(list))
	msg += a.skipNote(skipped)
	return msg, nil
}

func (a *App) SaveProfile(p store.Profile) (store.Profile, error) {
	typ, _ := p.Outbound["type"].(string)
	if typ == "" {
		return p, errors.New(i18n.T("出站配置缺少 type 字段"))
	}
	p.Chain = nil // chain proxies are saved by SaveChain
	delete(p.Outbound, "tag")
	p.Type = typ
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = typ
	}
	if p.ID != 0 {
		if old, ok := a.store.Profile(p.ID); ok {
			p.Latency, p.LatencyError, p.TestedAt = old.Latency, old.LatencyError, old.TestedAt
			p.Upload, p.Download = old.Upload, old.Download
			p.Country, p.CountryIP, p.CountrySource, p.CountryAt = old.Country, old.CountryIP, old.CountrySource, old.CountryAt
			if !sameJSON(old.Outbound, p.Outbound) {
				p.Country, p.CountryIP, p.CountrySource, p.CountryAt = "", "", "", 0 // the server may have moved
			}
		}
	}
	saved, err := a.store.SaveProfile(p)
	if err != nil {
		return saved, err
	}
	a.emitProfiles(saved.GroupID)
	go a.locateServers([]int64{saved.ID}, false)
	a.mu.Lock()
	isRunningNode := a.runningID == saved.ID && a.proc != nil
	a.mu.Unlock()
	if isRunningNode {
		go a.restartIfRunning()
	}
	return saved, nil
}

func (a *App) DeleteProfiles(ids []int64) error {
	groups := map[int64]bool{}
	for _, p := range a.store.ProfilesByIDs(ids) {
		groups[p.GroupID] = true
	}
	err := a.store.DeleteProfiles(ids)
	for g := range groups {
		a.emitProfiles(g)
	}
	a.emitState()
	return err
}

// DeleteUnavailable removes nodes whose last latency test failed.
func (a *App) DeleteUnavailable(groupID int64) (int, error) {
	var ids []int64
	for _, p := range a.store.Profiles(groupID) {
		if p.Latency < 0 {
			ids = append(ids, p.ID)
		}
	}
	return len(ids), a.DeleteProfiles(ids)
}

func (a *App) SortGroup(groupID int64, by string) error {
	err := a.store.SortGroup(groupID, by)
	a.emitState() // the group may now have a manual order
	a.emitProfiles(groupID)
	return err
}

// ReorderProfiles saves the order the nodes of a group were dragged into.
func (a *App) ReorderProfiles(groupID int64, ids []int64) error {
	err := a.store.ReorderProfiles(groupID, ids)
	a.emitState()
	a.emitProfiles(groupID)
	return err
}

func (a *App) ExportLinks(ids []int64) (string, error) {
	var lines []string
	for _, p := range a.store.ProfilesByIDs(ids) {
		if p.IsChain() {
			continue // there is no share-link format for chain proxies
		}
		l, err := link.Export(p.Name, p.Outbound)
		if err != nil {
			return "", fmt.Errorf("%s: %w", p.Name, err)
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n"), nil
}

// ---------------------------------------------------------------- latency

// TestBegin announces a latency test, so the UI can show its progress.
type TestBegin struct {
	IDs  []int64 `json:"ids"`
	Mode string  `json:"mode"`
}

// TestLatency tests nodes in the background. It emits "latency-begin", then
// "latency-start" (node id) when a node's measurement begins and "latency"
// with its result, and finally "latency-done". mode is "url" or "tcp".
func (a *App) TestLatency(ids []int64, mode string) error {
	nodes := a.store.ProfilesByIDs(ids)
	if len(nodes) == 0 {
		return errors.New(i18n.T("没有要测试的节点"))
	}
	a.mu.Lock()
	if a.testCancel != nil {
		a.mu.Unlock()
		return errors.New(i18n.T("正在测速，请稍候"))
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.testCancel = cancel
	a.mu.Unlock()
	begin := TestBegin{Mode: mode}
	for _, n := range nodes {
		begin.IDs = append(begin.IDs, n.ID)
	}
	wruntime.EventsEmit(a.ctx, "latency-begin", begin)
	a.emitState()

	s := a.store.Settings()
	go func() {
		defer func() {
			cancel()
			a.mu.Lock()
			a.testCancel = nil
			a.mu.Unlock()
			a.store.Flush()
			wruntime.EventsEmit(a.ctx, "latency-done")
			a.emitState()
		}()
		progress := core.Progress{
			Start: func(id int64) { wruntime.EventsEmit(a.ctx, "latency-start", id) },
			Result: func(r core.LatencyResult) {
				now := time.Now().Unix()
				if !r.Skipped {
					a.store.SetLatency(r.ID, r.Latency, r.Error, now)
				}
				if r.Exit != nil {
					a.store.SetCountry(r.ID, r.Exit.Country, r.Exit.IP, store.CountryExit, now)
				}
				wruntime.EventsEmit(a.ctx, "latency", r)
			},
			Member: a.store.Profile,
			Locate: a.wantsExitLookup,
		}
		go a.locateServers(idsOf(nodes), false)
		if mode == "tcp" {
			core.TCPPing(ctx, s, nodes, progress)
			return
		}
		// a chain proxy with a missing member would fail the whole batch
		usable := nodes[:0:0]
		for _, n := range nodes {
			if err := a.chainError(n); err != nil {
				progress.Start(n.ID)
				progress.Result(core.LatencyResult{ID: n.ID, Latency: -1, Error: err.Error()})
				continue
			}
			usable = append(usable, n)
		}
		core.TestURL(ctx, a.corePath(), a.testDir(), s, usable, progress)
	}()
	return nil
}

func (a *App) StopTest() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.testCancel != nil {
		a.testCancel()
	}
}
