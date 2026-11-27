package app

// The parts of a Clash subscription besides its nodes (proxy groups, rules,
// DNS settings), converted by internal/clash when the user chose to.

import (
	"errors"
	"nextbox/internal/i18n"

	"nextbox/internal/clash"
	"nextbox/internal/store"
)

// ClashInfo is what the dialog that asks shows: what an import would make,
// and the parts chosen before (nil when never asked).
type ClashInfo struct {
	Nodes   int                `json:"nodes"`
	Preview clash.Preview      `json:"preview"`
	Current *store.ClashImport `json:"current"`
}

func (a *App) setClashExtras(groupID int64, ex *clash.Extras) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.clashExtras == nil {
		a.clashExtras = map[int64]*clash.Extras{}
	}
	a.clashExtras[groupID] = ex
}

// clashExtrasOf returns the Clash parts of a subscription from its last
// update, or with fetch downloads it again (without touching its nodes).
func (a *App) clashExtrasOf(g store.Group, fetch bool) (*clash.Extras, error) {
	a.mu.Lock()
	ex := a.clashExtras[g.ID]
	a.mu.Unlock()
	if ex != nil || g.URL == "" || !fetch {
		return ex, nil
	}
	res, err := a.fetchSubscription(g.URL, g.UserAgent)
	if err != nil {
		return nil, err
	}
	if res.Clash != nil {
		a.setClashExtras(g.ID, res.Clash)
	}
	return res.Clash, nil
}

// ClashPreview says what the Clash parts of a subscription would make; nil
// when it has none. Without fetch only the last update is looked at (the
// question after adding or updating), with fetch the subscription is
// downloaded when needed (the group menu).
func (a *App) ClashPreview(groupID int64, fetch bool) (*ClashInfo, error) {
	g, ok := a.store.Group(groupID)
	if !ok {
		return nil, errors.New(i18n.T("分组不存在"))
	}
	ex, err := a.clashExtrasOf(g, fetch)
	if err != nil || ex == nil {
		return nil, err
	}
	nodes := a.store.Profiles(groupID)
	return &ClashInfo{Nodes: len(nodes), Preview: ex.Preview(nodes), Current: g.Clash}, nil
}

// ApplyClash takes over the chosen parts of a Clash subscription, now and on
// every update (all false: only its nodes).
func (a *App) ApplyClash(groupID int64, opt store.ClashImport) (string, error) {
	g, ok := a.store.Group(groupID)
	if !ok {
		return "", errors.New(i18n.T("分组不存在"))
	}
	ex, err := a.clashExtrasOf(g, true)
	if err != nil {
		return "", err
	}
	// unticking the subscription's DNS stops using it
	if g.Clash != nil && g.Clash.SubDNS && !opt.SubDNS {
		g.DNS = nil
	}
	g.Clash = &opt
	if err := a.store.UpdateGroup(g); err != nil {
		return "", err
	}
	if ex == nil {
		return "", nil
	}
	sum, err := a.applyClash(g, ex, opt)
	if err != nil {
		return "", err
	}
	a.emitState()
	return sum.String(), nil
}

// SetGroupDNS sets a subscription's own DNS by hand (Settings → DNS →
// 订阅专属 DNS); empty values use the global DNS.
func (a *App) SetGroupDNS(id int64, remote, resolver string) error {
	err := a.store.SetGroupDNS(id, &store.GroupDNS{Remote: remote, Resolver: resolver})
	a.emitState()
	if err == nil {
		err = a.restartIfRunning()
	}
	return err
}

// UseSubscriptionDNS switches a subscription between the DNS it names and
// the global one.
func (a *App) UseSubscriptionDNS(id int64, on bool) error {
	err := a.store.UseSubscriptionDNS(id, on)
	a.emitState()
	if err == nil {
		err = a.restartIfRunning()
	}
	return err
}

// applyClash applies and restarts the core when the routing changed.
func (a *App) applyClash(g store.Group, ex *clash.Extras, opt store.ClashImport) (clash.Summary, error) {
	before := a.routingState()
	sum, err := clash.Apply(a.store, ex, g, opt)
	if err != nil {
		return sum, err
	}
	if !sameJSON(before, a.routingState()) {
		a.log(sum.String())
		if err := a.restartIfRunning(); err != nil {
			a.log(err.Error())
		}
	}
	return sum, nil
}

// routingState is what an import of Clash parts can change.
func (a *App) routingState() any {
	s := a.store.Settings()
	return []any{s.Rules, s.DNSOverrides, a.store.Policies(), a.store.GroupDNS()}
}
