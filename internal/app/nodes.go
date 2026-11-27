package app

// Groups, subscriptions, nodes and latency tests.

import (
	"context"
	"errors"
	"fmt"
	"nextbox/internal/i18n"
	"slices"
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
	// downloaded a moment ago by AddSubscriptionAuto, which asked for the name
	a.mu.Lock()
	pending := a.pendingSub
	a.pendingSub = nil
	a.mu.Unlock()
	if pending != nil && (pending.url != url || time.Since(pending.at) > 5*time.Minute) {
		pending = nil
	}
	g, err := a.store.AddGroup(store.Group{Name: strings.TrimSpace(name), URL: url})
	if err != nil {
		return "", err
	}
	a.store.SetCurrentGroup(g.ID)
	a.emitState()
	if pending != nil {
		return a.addedSubscription(g, pending.res)
	}
	return a.UpdateSubscription(g.ID)
}

// SubscriptionAdd is what AddSubscriptionAuto did.
type SubscriptionAdd struct {
	Message string `json:"message,omitempty"`
	// NeedName: the subscription names itself nowhere; it was downloaded and
	// AddSubscription(name, url) adds it without downloading it again.
	NeedName bool   `json:"needName,omitempty"`
	Suggest  string `json:"suggest,omitempty"` // the host, to start the name with
}

// pendingSubscription is a subscription downloaded while its name is asked.
type pendingSubscription struct {
	url string
	res *sub.Result
	at  time.Time
}

// AddSubscriptionAuto adds a subscription found in the clipboard, a dropped
// file or the import button. Its name is the one given (from the link), else
// the one its server sends; with neither, the user is asked (NeedName).
func (a *App) AddSubscriptionAuto(url, name string) (SubscriptionAdd, error) {
	url = strings.TrimSpace(url)
	if strings.TrimSpace(name) != "" {
		msg, err := a.AddSubscription(name, url)
		return SubscriptionAdd{Message: msg}, err
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return SubscriptionAdd{}, errors.New(i18n.T("订阅地址必须以 http:// 或 https:// 开头"))
	}
	a.log(i18n.T("正在读取订阅: ") + url)
	res, err := a.fetchSubscription(url, "")
	if err != nil {
		a.log(err.Error())
		return SubscriptionAdd{}, err
	}
	if res.Title == "" {
		a.mu.Lock()
		a.pendingSub = &pendingSubscription{url: url, res: res, at: time.Now()}
		a.mu.Unlock()
		return SubscriptionAdd{NeedName: true, Suggest: link.Classify(url).Name}, nil
	}
	g, err := a.store.AddGroup(store.Group{Name: res.Title, URL: url})
	if err != nil {
		return SubscriptionAdd{}, err
	}
	a.store.SetCurrentGroup(g.ID)
	a.emitState()
	msg, err := a.addedSubscription(g, res)
	return SubscriptionAdd{Message: msg}, err
}

// addedSubscription fills a new subscription group with what was downloaded
// already; a failure leaves the group, which can then be updated again.
func (a *App) addedSubscription(g store.Group, res *sub.Result) (string, error) {
	a.log(i18n.T("正在更新订阅: ") + g.Name)
	return a.applySubscription(g.ID, res)
}

// ClassifyText tells what pasted or dropped text holds (see link.Classify).
func (a *App) ClassifyText(text string) link.Classified { return link.Classify(text) }

func (a *App) EditGroup(g store.Group) error {
	old, ok := a.store.Group(g.ID)
	if !ok {
		return errors.New(i18n.T("分组不存在"))
	}
	old.Name = strings.TrimSpace(g.Name)
	old.URL = strings.TrimSpace(g.URL)
	old.UserAgent = strings.TrimSpace(g.UserAgent)
	err := a.store.UpdateGroup(old) // its DNS is set in Settings → DNS (SetGroupDNS)
	a.emitState()
	return err
}

func (a *App) DeleteGroup(id int64) error {
	err := a.store.DeleteGroup(id)
	a.emitState()
	return err
}

// UpdateSubscription downloads a subscription again. The nodes keep the
// order they have (sorted, dragged or the subscription's own); new nodes
// are added at the end in the order of the subscription, and nodes it no
// longer has are removed.
func (a *App) UpdateSubscription(groupID int64) (string, error) {
	g, ok := a.store.Group(groupID)
	if !ok {
		return "", errors.New(i18n.T("分组不存在"))
	}
	if g.URL == "" {
		return "", errors.New(i18n.T("这个分组不是订阅"))
	}
	a.log(i18n.T("正在更新订阅: ") + g.Name)
	res, err := a.fetchSubscription(g.URL, g.UserAgent)
	if err != nil {
		a.log(err.Error())
		return "", err
	}
	return a.applySubscription(groupID, res)
}

// fetchSubscription downloads a subscription with the group's User-Agent
// (or the default one), through the core when the settings say so.
func (a *App) fetchSubscription(url, userAgent string) (*sub.Result, error) {
	s := a.store.Settings()
	if userAgent == "" {
		userAgent = s.SubUserAgent
	}
	proxy := ""
	if s.SubUpdateByProxy && a.isRunning() {
		proxy = "http://" + proxyAddr(s)
	}
	return sub.Fetch(a.ctx, url, userAgent, proxy)
}

// applySubscription puts a downloaded subscription into its group.
func (a *App) applySubscription(groupID int64, res *sub.Result) (string, error) {
	profiles := make([]store.Profile, 0, len(res.Nodes))
	for _, n := range res.Nodes {
		profiles = append(profiles, store.Profile{Name: n.Name, Type: n.Type(), Outbound: n.Outbound})
	}
	all := len(profiles)
	profiles = store.DedupeList(profiles)
	before := map[string]bool{}
	for _, p := range a.store.Profiles(groupID) {
		before[store.DupKey(&p)] = true
	}
	hadNodes := len(before) > 0
	if err := a.store.ReplaceGroupProfiles(groupID, profiles, true); err != nil {
		return "", err
	}
	added := 0
	for i := range profiles {
		k := store.DupKey(&profiles[i])
		if !before[k] {
			added++
		}
		delete(before, k)
	}
	removed := len(before)
	go a.locateServers(idsOf(a.store.Profiles(groupID)), false)
	g, _ := a.store.Group(groupID)
	g.UpdatedAt = time.Now().Unix()
	if res.Usage != nil {
		g.Usage = res.Usage
	}
	// the DNS the subscription names, offered in Settings → DNS
	g.SuggestedDNS = nil
	if res.Clash != nil {
		g.SuggestedDNS = res.Clash.SubDNS
	}
	a.store.UpdateGroup(g)
	msg := fmt.Sprintf(i18n.T("“%s”更新完成：%d 个节点"), g.Name, len(profiles))
	if hadNodes && added+removed > 0 {
		msg += fmt.Sprintf(i18n.T("，新增 %d · 移除 %d"), added, removed)
	}
	if n := all - len(profiles); n > 0 {
		msg += fmt.Sprintf(i18n.T("（去掉了 %d 个重复节点）"), n)
	}
	msg += a.skipNote(res.Skipped)
	if res.Clash != nil {
		a.setClashExtras(groupID, res.Clash)
		if g.Clash != nil {
			if sum, err := a.applyClash(g, res.Clash, *g.Clash); err != nil {
				a.log(err.Error())
			} else {
				msg += "；" + sum.String()
			}
		}
	}
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

// FindDuplicates lists the nodes of all groups that connect the same way as
// another one (see store.FindDuplicates), for the confirmation.
func (a *App) FindDuplicates() []store.DupSet {
	return a.store.FindDuplicates(a.runningNode())
}

// DeleteDuplicates removes them and returns how many went.
func (a *App) DeleteDuplicates() (int, error) {
	n, err := a.store.DeleteDuplicates(a.runningNode())
	if n > 0 {
		for _, g := range a.store.Groups() {
			a.emitProfiles(g.ID)
		}
		a.emitState()
	}
	return n, err
}

// runningNode is the node the core runs, 0 when it is stopped.
func (a *App) runningNode() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proc == nil {
		return 0
	}
	return a.runningID
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

// groupsOf lists the groups of nodes, each once.
func groupsOf(nodes []store.Profile) []int64 {
	var out []int64
	for _, n := range nodes {
		if !slices.Contains(out, n.GroupID) {
			out = append(out, n.GroupID)
		}
	}
	return out
}

// TestBegin announces a latency test, so the UI can show its progress.
type TestBegin struct {
	IDs  []int64 `json:"ids"`
	Mode string  `json:"mode"`
}

// TestLatency tests nodes in the background. It emits "latency-begin", then
// "latency-start" (node id) when a node's measurement begins and "latency"
// with its result, and finally "latency-done". mode is store.TestURL or
// store.TestTCP. When sortBy is not empty, the groups of the tested nodes
// are sorted afterwards (SortGroup), unless the test was stopped.
func (a *App) TestLatency(ids []int64, mode string, sortBy string) error {
	nodes := a.store.ProfilesByIDs(ids)
	if len(nodes) == 0 {
		return errors.New(i18n.T("没有要测试的节点"))
	}
	a.mu.Lock()
	if a.testCancel != nil {
		a.mu.Unlock()
		return errors.New(i18n.T("正在测试，请稍候"))
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
	groups := groupsOf(nodes) // before nodes is narrowed down below
	go func() {
		defer func() {
			finished := ctx.Err() == nil
			cancel()
			a.mu.Lock()
			a.testCancel = nil
			a.mu.Unlock()
			if sortBy != "" && finished {
				for _, gid := range groups {
					a.store.SortGroup(gid, sortBy)
					a.emitProfiles(gid)
				}
			}
			a.store.Flush()
			wruntime.EventsEmit(a.ctx, "latency-done")
			a.emitState()
		}()
		progress := core.Progress{
			Start: func(id int64) { wruntime.EventsEmit(a.ctx, "latency-start", id) },
			Result: func(r core.LatencyResult) {
				now := time.Now().Unix()
				if !r.Skipped {
					a.store.SetLatency(r.ID, r.Latency, r.Error, now, r.Kind)
				}
				if r.Exit != nil {
					a.store.SetCountry(r.ID, r.Exit.Country, r.Exit.IP, store.CountryExit, now)
				}
				wruntime.EventsEmit(a.ctx, "latency", r)
				a.trayLive()
			},
			Member:   a.store.Profile,
			Locate:   a.wantsExitLookup,
			GroupDNS: a.store.GroupDNS(),
		}
		go a.locateServers(idsOf(nodes), false)
		base := progress // the wrappers below must not call themselves
		withKind := func(kind string) core.Progress {
			p := base
			p.Result = func(r core.LatencyResult) {
				r.Kind = kind
				base.Result(r)
			}
			return p
		}
		if mode == "tcp" {
			// nodes a TCP handshake cannot measure get a URL test instead
			var tcp, url []store.Profile
			for _, n := range nodes {
				if core.URLOnly(n) {
					url = append(url, n)
				} else {
					tcp = append(tcp, n)
				}
			}
			core.TCPPing(ctx, s, tcp, withKind("tcp"))
			nodes = url
			if len(nodes) == 0 {
				return
			}
		}
		progress = withKind("url")
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
		core.TestURL(ctx, a.corePath(), a.cacheDir(), s, usable, progress)
		a.clearCache()
	}()
	return nil
}

// testGroup tests a whole group the way the settings say, from outside the
// window (hotkey, tray menu). Its order afterwards follows
// Settings.SortAfterTest; while that was never chosen (the window asks on
// the first test) the order stays as it is.
func (a *App) testGroup(groupID int64) error {
	s := a.store.Settings()
	var ids []int64
	for _, p := range a.store.Profiles(groupID) {
		ids = append(ids, p.ID)
	}
	by := s.SortAfterTest
	if by == store.SortNone || len(ids) < 2 {
		by = ""
	}
	return a.TestLatency(ids, s.TestMode, by)
}

func (a *App) StopTest() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.testCancel != nil {
		a.testCancel()
	}
}

// TestSpeed downloads through the given nodes one after another (see
// core.TestSpeed). Events: "speed-begin" (the IDs), "speed-start" (an ID),
// "speed-tick" and "speed" (core.SpeedResult), "speed-done". The results
// stay in memory (GetSpeeds) until NextBox quits. StopTest stops it.
func (a *App) TestSpeed(ids []int64) error {
	nodes := a.store.ProfilesByIDs(ids)
	if len(nodes) == 0 {
		return errors.New(i18n.T("没有要测试的节点"))
	}
	a.mu.Lock()
	if a.testCancel != nil {
		a.mu.Unlock()
		return errors.New(i18n.T("正在测试，请稍候"))
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.testCancel = cancel
	a.mu.Unlock()
	wruntime.EventsEmit(a.ctx, "speed-begin", idsOf(nodes))
	a.emitState()
	s := a.store.Settings()
	go func() {
		defer func() {
			cancel()
			a.mu.Lock()
			a.testCancel = nil
			a.mu.Unlock()
			wruntime.EventsEmit(a.ctx, "speed-done")
			a.emitState()
		}()
		usable := nodes[:0:0]
		for _, n := range nodes {
			if err := a.chainError(n); err != nil {
				a.setSpeed(core.SpeedResult{ID: n.ID, Error: err.Error()})
				continue
			}
			usable = append(usable, n)
		}
		core.TestSpeed(ctx, a.corePath(), a.cacheDir(), s, usable, core.SpeedProgress{
			Start:    func(id int64) { wruntime.EventsEmit(a.ctx, "speed-start", id) },
			Tick:     func(r core.SpeedResult) { wruntime.EventsEmit(a.ctx, "speed-tick", r) },
			Result:   a.setSpeed,
			Member:   a.store.Profile,
			GroupDNS: a.store.GroupDNS(),
		})
		a.clearCache()
	}()
	return nil
}

func (a *App) setSpeed(r core.SpeedResult) {
	a.mu.Lock()
	if a.speeds == nil {
		a.speeds = map[int64]core.SpeedResult{}
	}
	a.speeds[r.ID] = r
	a.mu.Unlock()
	wruntime.EventsEmit(a.ctx, "speed", r)
}

// GetSpeeds returns the download speed results of this session.
func (a *App) GetSpeeds() []core.SpeedResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]core.SpeedResult, 0, len(a.speeds))
	for _, r := range a.speeds {
		out = append(out, r)
	}
	return out
}
