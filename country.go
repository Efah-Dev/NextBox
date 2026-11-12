package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nextbox/internal/geo"
	"nextbox/internal/i18n"
	"nextbox/internal/store"
)

// How long a detected country is trusted before it is looked up again.
const (
	exitCountryTTL   = 24 * time.Hour     // exit IPs of airport nodes rotate
	serverCountryTTL = 7 * 24 * time.Hour // server addresses rarely move
)

func (a *App) geoDBPath() string { return filepath.Join(a.dataDir(), "geoip.db") }

// wantsExitLookup reports whether a URL test should also ask where the
// node's traffic leaves: when the country comes from the exit IP and the
// node has none, or an old one.
func (a *App) wantsExitLookup(id int64) bool {
	if a.store.Settings().CountrySource != store.CountryExit {
		return false
	}
	p, ok := a.store.Profile(id)
	return ok && (p.CountrySource != store.CountryExit || p.Country == "" ||
		time.Since(time.Unix(p.CountryAt, 0)) > exitCountryTTL)
}

// locateServers looks up the countries of the nodes' servers (all nodes when
// ids is nil) in the sing-geoip database, when the country comes from the
// server IP. Unless force is set, nodes with a recent answer are skipped.
// The database is downloaded on first use.
func (a *App) locateServers(ids []int64, force bool) {
	if a.store.Settings().CountrySource != store.CountryServer {
		return
	}
	a.geoMu.Lock()
	defer a.geoMu.Unlock()

	list := a.store.Profiles(0)
	if ids != nil {
		list = a.store.ProfilesByIDs(ids)
	}
	type job struct {
		id   int64
		host string
	}
	var jobs []job
	for _, p := range list {
		if !force && p.CountrySource == store.CountryServer && p.Country != "" &&
			time.Since(time.Unix(p.CountryAt, 0)) < serverCountryTTL {
			continue
		}
		target := p
		if p.IsChain() {
			// a chain proxy is where its last hop is
			members := a.store.Members(p)
			if len(members) == 0 || members[len(members)-1].ID == 0 {
				continue
			}
			target = members[len(members)-1]
		}
		if host, _ := target.Outbound["server"].(string); host != "" {
			jobs = append(jobs, job{p.ID, host})
		}
	}
	if len(jobs) == 0 {
		return
	}

	db, err := a.openGeoDB()
	if err != nil {
		a.log(i18n.T("无法按服务器 IP 判断地区: ") + err.Error())
		return
	}
	defer db.Close()

	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	var mu sync.Mutex
	groups := map[int64]bool{}
	now := time.Now().Unix()
	for _, j := range jobs {
		sem <- struct{}{}
		wg.Add(1)
		go func(j job) {
			defer func() { <-sem; wg.Done() }()
			ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
			defer cancel()
			ip, err := geo.Resolve(ctx, j.host)
			if err != nil {
				return
			}
			code := db.Country(ip)
			if code == "" {
				return
			}
			a.store.SetCountry(j.id, code, ip.String(), store.CountryServer, now)
			if p, ok := a.store.Profile(j.id); ok {
				mu.Lock()
				groups[p.GroupID] = true
				mu.Unlock()
			}
		}(j)
	}
	wg.Wait()
	a.store.Flush()
	for g := range groups {
		a.emitProfiles(g)
	}
}

// openGeoDB opens the sing-geoip database, downloading it when it is missing
// or unreadable.
func (a *App) openGeoDB() (*geo.DB, error) {
	path := a.geoDBPath()
	if db, err := geo.Open(path); err == nil {
		return db, nil
	}
	a.log(i18n.T("正在下载 IP 数据库（sing-geoip，约 4 MB）…"))
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if a.isRunning() {
		if pu, err := url.Parse("http://" + proxyAddr(a.store.Settings())); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 90 * time.Second}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	var errs []error
	for _, u := range geo.DBURLs {
		if err := downloadFile(a.ctx, client, u, path); err != nil {
			errs = append(errs, err)
			continue
		}
		db, err := geo.Open(path)
		if err != nil {
			os.Remove(path)
			errs = append(errs, err)
			continue
		}
		a.log(i18n.T("IP 数据库已下载"))
		return db, nil
	}
	return nil, errors.Join(errs...)
}
