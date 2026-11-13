package main

// Running the core: connecting, switching nodes without a restart,
// watching the process and the system proxy.

import (
	"context"
	"encoding/json"
	"errors"
	"nextbox/internal/i18n"
	"strconv"
	"strings"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/core"
	"nextbox/internal/store"
	"nextbox/internal/sys"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// loadedNode is a node the running core was started with: its outbound tag,
// and the outbound it had then, to notice later edits.
type loadedNode struct {
	tag      string
	outbound string
}

// maxSwitchable caps how many nodes of the group are loaded into the core.
const maxSwitchable = 200

// outboundKey identifies what a node connects with, to notice edits. For a
// chain proxy that is the outbounds of its members, in order.
func (a *App) outboundKey(p store.Profile) string {
	v := any(p.Outbound)
	if p.IsChain() {
		hops := make([]any, 0, len(p.Chain))
		for _, m := range a.store.Members(p) {
			hops = append(hops, m.Outbound)
		}
		v = hops
	}
	b, _ := json.Marshal(v) // map keys are sorted, so this is stable
	return string(b)
}

// ---------------------------------------------------------------- core control

func (a *App) Connect() error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	err := a.startLocked()
	a.emitState()
	return err
}

func (a *App) Disconnect() error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.stopLocked()
	a.emitState()
	return nil
}

func (a *App) isRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.proc != nil
}

// restartIfRunning applies new settings / node to a running core.
func (a *App) restartIfRunning() error {
	if !a.isRunning() {
		return nil
	}
	return a.Connect()
}

func (a *App) startLocked() error {
	a.stopLocked()
	s := a.store.Settings()
	if s.TunEnabled && !sys.IsElevated() {
		return errors.New(i18n.T("TUN 模式需要管理员权限，请先“以管理员身份重启”"))
	}
	var node *store.Profile
	if id := a.store.SelectedID(); id != 0 {
		if p, ok := a.store.Profile(id); ok {
			node = &p
		}
	}
	port, err := core.FreePort()
	if err != nil {
		return err
	}
	api := core.NewAPI("127.0.0.1:"+strconv.Itoa(port), core.RandomSecret())
	if missing := a.ensureRuleSets(); len(missing) > 0 {
		a.log(i18n.T("规则集不可用，已暂时跳过相关规则: ") + strings.Join(missing, ", "))
	}
	// The rest of the node's group is loaded too, so that switching to one of
	// them does not need a restart (see switchNode).
	in := config.Input{
		Settings: s, Node: node, Member: a.store.Profile, ClashAPI: api.Addr, Secret: api.Secret,
		RuleSetDir: a.ruleSetDir(), SkipMissingRuleSets: true,
	}
	if node != nil {
		in.Policies = a.policyInputs(s)
	}
	if node != nil {
		for _, p := range a.store.Profiles(node.GroupID) {
			if p.ID != node.ID && len(in.Nodes) < maxSwitchable-1 && a.chainError(p) == nil {
				in.Nodes = append(in.Nodes, p)
			}
		}
	}
	cfg, err := config.Build(in)
	if err != nil {
		return err
	}

	if node != nil {
		a.log(i18n.T("正在连接: ") + node.Name)
	} else {
		a.log(i18n.T("正在启动直连模式"))
	}
	proc, err := core.Start(a.ctx, a.corePath(), a.runDir(), cfg, api, a.logs.Append)
	if err != nil && len(in.Nodes) > 0 {
		// Most likely one of the other nodes has a configuration the core
		// rejects; the selected node must still be usable on its own.
		a.log(i18n.T("内核未能加载分组里的全部节点，改为只加载当前节点（切换节点时会重启内核）"))
		in.Nodes = nil
		if cfg, err = config.Build(in); err != nil {
			return err
		}
		proc, err = core.Start(a.ctx, a.corePath(), a.runDir(), cfg, api, a.logs.Append)
	}
	if err != nil {
		a.log(err.Error())
		return err
	}

	var nodeID int64
	loaded := map[int64]loadedNode{}
	var nodeTags, policyTags map[int64]string
	if node != nil {
		nodeID = node.ID
		_, nodeTags, policyTags = in.Tags()
		for _, p := range in.Switchable() {
			loaded[p.ID] = loadedNode{tag: nodeTags[p.ID], outbound: a.outboundKey(p)}
		}
		// The core remembers the last selection in its cache file and would
		// prefer it over the configured default.
		if err := proc.API.SelectProxy(a.ctx, config.TagProxy, nodeTags[nodeID]); err != nil {
			proc.Stop()
			a.log(err.Error())
			return err
		}
		for _, pi := range in.Policies {
			if pi.Policy.Mode == store.PolicySelect && pi.Policy.Selected != 0 {
				proc.API.SelectProxy(a.ctx, policyTags[pi.Policy.ID], nodeTags[pi.Policy.Selected])
			}
		}
	}
	a.mu.Lock()
	a.proc = proc
	a.runningID = nodeID
	a.loaded = loaded
	a.nodeTags, a.policyTags = nodeTags, policyTags
	a.tunActive = s.TunEnabled
	a.startedAt = time.Now()
	trafficCtx, cancel := context.WithCancel(a.ctx)
	a.stopTraffic = cancel
	a.mu.Unlock()

	if s.SystemProxy && !s.TunEnabled {
		if err := a.applySystemProxy(s, true); err != nil {
			a.log(i18n.T("设置系统代理失败: ") + err.Error())
		}
	}
	go a.trafficLoop(trafficCtx, proc)
	go a.exitLoop(trafficCtx)
	go a.watch(proc)
	a.log(i18n.T("内核已启动，混合代理端口 ") + strconv.Itoa(s.MixedPort))
	return nil
}

func (a *App) stopLocked() {
	a.mu.Lock()
	proc := a.proc
	a.proc = nil
	a.runningID = 0
	a.loaded = nil
	a.nodeTags, a.policyTags = nil, nil
	if a.stopTraffic != nil {
		a.stopTraffic()
		a.stopTraffic = nil
	}
	a.mu.Unlock()
	if proc == nil {
		return
	}
	a.applySystemProxy(store.Settings{}, false)
	proc.Stop()
	a.store.Flush()
	a.log(i18n.T("内核已停止"))
}

// watch reports an unexpected core exit.
func (a *App) watch(proc *core.Process) {
	<-proc.Done()
	a.mu.Lock()
	unexpected := a.proc == proc
	if unexpected {
		a.proc = nil
		a.runningID = 0
		a.loaded = nil
		if a.stopTraffic != nil {
			a.stopTraffic()
			a.stopTraffic = nil
		}
	}
	a.mu.Unlock()
	if !unexpected {
		return
	}
	a.applySystemProxy(store.Settings{}, false)
	msg := i18n.T("内核意外退出")
	if err := proc.ExitErr(); err != nil {
		msg += ": " + err.Error()
	}
	a.log(msg)
	a.emitState()
	wruntime.EventsEmit(a.ctx, "error", msg)
}

func (a *App) trafficLoop(ctx context.Context, proc *core.Process) {
	for ctx.Err() == nil {
		proc.API.StreamTraffic(ctx, func(up, down int64) {
			wruntime.EventsEmit(a.ctx, "traffic", map[string]int64{"up": up, "down": down})
			// the node can change while the core keeps running
			a.mu.Lock()
			nodeID := a.runningID
			if a.proc != proc {
				nodeID = 0
			}
			a.mu.Unlock()
			if nodeID != 0 && up+down > 0 {
				a.store.AddTraffic(nodeID, up, down)
			}
		})
		select {
		case <-ctx.Done():
		case <-proc.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func proxyAddr(s store.Settings) string {
	host := s.ListenAddress
	if host == "" || host == "0.0.0.0" || host == "::" || s.AllowLAN {
		host = "127.0.0.1"
	}
	return host + ":" + strconv.Itoa(s.MixedPort)
}

func (a *App) applySystemProxy(s store.Settings, enable bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !enable {
		if a.sysProxyAddr == "" {
			return nil
		}
		err := sys.ClearSystemProxy(a.sysProxyAddr)
		a.sysProxyAddr = ""
		sys.SetProxyCleanup("")
		return err
	}
	addr := proxyAddr(s)
	if err := sys.SetSystemProxy(addr); err != nil {
		return err
	}
	a.sysProxyAddr = addr
	if err := sys.SetProxyCleanup(addr); err != nil {
		a.logs.Append(time.Now().Format("15:04:05") + " [GUI] " + err.Error())
	}
	return nil
}

func (a *App) SelectProfile(id int64) error {
	if err := a.store.SetSelected(id); err != nil {
		return err
	}
	if a.switchNode(id) {
		a.emitState()
		return nil
	}
	a.emitState()
	return a.restartIfRunning()
}

// switchNode points the running core at another node it has already loaded,
// without a restart: open connections carry on, new ones use the new node.
// It reports false when the core has to be restarted instead (not running,
// node not loaded or edited since, or the core refused).
func (a *App) switchNode(id int64) bool {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	p, ok := a.store.Profile(id)
	if !ok {
		return false
	}
	a.mu.Lock()
	proc, node, current := a.proc, a.loaded[id], a.runningID == id
	a.mu.Unlock()
	if proc == nil || node.tag == "" || node.outbound != a.outboundKey(p) {
		return false
	}
	if current {
		return true
	}
	if err := proc.API.SelectProxy(a.ctx, config.TagProxy, node.tag); err != nil {
		a.log(i18n.T("切换节点失败，将重启内核: ") + err.Error())
		return false
	}
	a.mu.Lock()
	if a.proc == proc {
		a.runningID = id
	}
	a.mu.Unlock()
	a.log(i18n.T("已切换到节点: ") + p.Name)
	go a.checkExit()
	return true
}
