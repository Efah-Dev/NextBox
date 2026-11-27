package app

// Connecting through a policy group (分组代理) instead of one node: the main
// selector of the core ("proxy") points at the group, which picks a member
// (the fastest, or the one chosen). While it is used, the member in use is
// followed, so that traffic is counted for the node that carried it and the
// exit address is checked again when the group switches.

import (
	"context"
	"fmt"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/i18n"
	"nextbox/internal/store"
)

// policyPoll is how often the member in use is read while a group is used.
const policyPoll = 5 * time.Second

// UsePolicy makes the connection use a policy group; 0 goes back to the
// selected node. A running core is switched without a restart when it has
// the group loaded.
func (a *App) UsePolicy(id int64) error {
	if err := a.store.SetSelectedPolicy(id); err != nil {
		return err
	}
	defer a.emitState()
	if !a.isRunning() {
		return nil
	}
	if id == 0 {
		if a.switchNode(a.store.SelectedID()) {
			return nil
		}
		return a.restartIfRunning()
	}
	if a.switchPolicy(id) {
		return nil
	}
	return a.restartIfRunning()
}

// switchPolicy points the running core's main selector at a policy group it
// has loaded; false when it has to be restarted instead.
func (a *App) switchPolicy(id int64) bool {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	a.mu.Lock()
	proc, tag := a.proc, a.policyTags[id]
	a.mu.Unlock()
	if proc == nil || tag == "" {
		return false
	}
	if err := proc.API.SelectProxy(a.ctx, config.TagProxy, tag); err != nil {
		a.log(i18n.T("切换节点失败，将重启内核: ") + err.Error())
		return false
	}
	a.mu.Lock()
	if a.proc == proc {
		a.runningPolicy = id
	}
	a.mu.Unlock()
	if p, ok := a.store.Policy(id); ok {
		a.log(i18n.T("已切换到分组代理: ") + p.Name)
	}
	a.followPolicy()
	return true
}

// policyForConnect is the policy group a connection starts with and its
// members; an error when it has none that can be used.
func (a *App) policyForConnect() (store.Policy, []store.Profile, error) {
	id := a.store.SelectedPolicy()
	if id == 0 {
		return store.Policy{}, nil, nil
	}
	p, _ := a.store.Policy(id)
	members := a.policyMembers(p)
	if len(members) == 0 {
		return p, nil, fmt.Errorf(i18n.T("分组代理“%s”没有可用节点，请换一个分组代理或节点"), p.Name)
	}
	return p, members, nil
}

// followPolicy reads which member the policy group in use has picked and
// takes it as the running node (traffic, exit address, tray).
func (a *App) followPolicy() {
	a.mu.Lock()
	id, tag := a.runningPolicy, a.policyTags[a.runningPolicy]
	p := a.proc
	byTag := map[string]int64{}
	for nid, t := range a.nodeTags {
		byTag[t] = nid
	}
	a.mu.Unlock()
	if id == 0 || tag == "" || p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
	now, err := p.API.SelectedProxy(ctx, tag)
	cancel()
	node := byTag[now]
	if err != nil || node == 0 {
		return
	}
	a.mu.Lock()
	changed := a.proc == p && a.runningPolicy == id && a.runningID != node
	if changed {
		a.runningID = node
	}
	a.mu.Unlock()
	if !changed {
		return
	}
	if n, ok := a.store.Profile(node); ok {
		a.log(fmt.Sprintf(i18n.T("分组代理正在使用节点: %s"), n.Name))
	}
	go a.checkExit()
	a.emitState()
}

// policyLoop follows the policy group in use while the core runs.
func (a *App) policyLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(policyPoll):
		}
		a.followPolicy()
	}
}

// policyNowName is the member a policy group uses in the running core, or "".
func (a *App) policyNowName(id int64) string {
	a.mu.Lock()
	proc, tag := a.proc, a.policyTags[id]
	byTag := map[string]int64{}
	for nid, t := range a.nodeTags {
		byTag[t] = nid
	}
	a.mu.Unlock()
	if proc == nil || tag == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
	defer cancel()
	now, err := proc.API.SelectedProxy(ctx, tag)
	if err != nil {
		return ""
	}
	if n, ok := a.store.Profile(byTag[now]); ok {
		return n.Name
	}
	return ""
}
