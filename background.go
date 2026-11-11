package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"nextbox/internal/i18n"
	"nextbox/internal/update"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	appRepo  = "Efah-Dev/NextBox"
	coreRepo = "SagerNet/sing-box"
)

// backgroundLoop runs the periodic jobs: automatic subscription updates and
// the optional update check.
func (a *App) backgroundLoop(ctx context.Context) {
	const tick = 10 * time.Minute
	failed := map[int64]time.Time{} // subscriptions whose last automatic update failed
	var checked time.Time
	timer := time.NewTimer(30 * time.Second) // let the app (and an auto-connect) settle first
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		a.autoUpdateSubscriptions(failed)
		if a.store.Settings().AutoCheckUpdate && time.Since(checked) >= 24*time.Hour {
			checked = time.Now()
			if info, err := a.CheckUpdate(); err != nil {
				a.log(i18n.T("检查更新失败: ") + err.Error())
			} else if info.App.HasUpdate || info.Core.HasUpdate {
				wruntime.EventsEmit(a.ctx, "update", info)
			}
		}
		timer.Reset(tick)
	}
}

// autoUpdateSubscriptions refreshes the subscriptions that are older than the
// configured interval. One that fails is not retried for an hour.
func (a *App) autoUpdateSubscriptions(failed map[int64]time.Time) {
	s := a.store.Settings()
	if !s.SubAutoUpdate {
		return
	}
	interval := time.Duration(s.SubUpdateHours) * time.Hour
	now := time.Now()
	for _, g := range a.store.Groups() {
		if g.URL == "" || now.Sub(time.Unix(g.UpdatedAt, 0)) < interval {
			continue
		}
		if last, ok := failed[g.ID]; ok && now.Sub(last) < time.Hour {
			continue
		}
		if _, err := a.UpdateSubscription(g.ID); err != nil {
			failed[g.ID] = now
		} else {
			delete(failed, g.ID)
		}
	}
}

// UpdateItem compares an installed version with the latest release.
type UpdateItem struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"` // empty when it could not be looked up
	URL       string `json:"url"`    // release page
	HasUpdate bool   `json:"hasUpdate"`
}

type UpdateInfo struct {
	App  UpdateItem `json:"app"`
	Core UpdateItem `json:"core"`
}

// CheckUpdate compares NextBox and the sing-box core with their latest GitHub
// releases. Nothing is downloaded or installed. The request goes through the
// proxy while connected, as GitHub is not reachable everywhere.
func (a *App) CheckUpdate() (UpdateInfo, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if a.isRunning() {
		if pu, err := url.Parse("http://" + proxyAddr(a.store.Settings())); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}

	info := UpdateInfo{App: UpdateItem{Current: Version}}
	rel, err := update.Latest(a.ctx, client, appRepo)
	if err != nil {
		return info, fmt.Errorf(i18n.T("检查更新失败: %w"), err)
	}
	info.App.Latest, info.App.URL = rel.Version, rel.URL
	info.App.HasUpdate = update.Newer(rel.Version, Version)

	// The core is optional here: a failure only leaves its latest version empty.
	if core := a.GetCoreInfo(); core.Found && core.Version != "" {
		info.Core.Current = core.Version
		if rel, err := update.Latest(a.ctx, client, coreRepo); err == nil {
			info.Core.Latest, info.Core.URL = rel.Version, rel.URL
			info.Core.HasUpdate = update.Newer(rel.Version, core.Version)
		}
	}
	if info.App.HasUpdate {
		a.log(fmt.Sprintf(i18n.T("发现新版本 NextBox %s（当前 %s）"), info.App.Latest, Version))
	}
	if info.Core.HasUpdate {
		a.log(fmt.Sprintf(i18n.T("发现新版本 sing-box %s（当前 %s）"), info.Core.Latest, info.Core.Current))
	}
	return info, nil
}
