// Package sub downloads and decodes subscriptions.
package sub

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"nextbox/internal/i18n"
	"regexp"
	"strconv"
	"strings"
	"time"

	"nextbox/internal/clash"
	"nextbox/internal/link"
	"nextbox/internal/store"
)

type Result struct {
	Nodes   []*link.Node
	Skipped []string // why each skipped entry was rejected
	Usage   *store.SubscriptionInfo
	// Clash is what a Clash subscription has besides its proxies (proxy
	// groups, rules, DNS settings); nil for other formats.
	Clash *clash.Extras
	// Title is the name the server gives the subscription (profile-title,
	// or the file name of Content-Disposition); "" when it gives none.
	Title string
}

// Fetch downloads a subscription. proxy is an optional "http://host:port"
// used when the subscription should be fetched through the running core.
func Fetch(ctx context.Context, rawURL, userAgent, proxy string) (*Result, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		pu, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(pu)
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	body, header, err := get(ctx, client, rawURL, userAgent)
	if err != nil {
		return nil, err
	}
	nodes, skipped, err := link.ParseContent(string(body))
	if providers := link.ClashProviders(string(body)); len(providers) > 0 {
		pn, ps := fetchProviders(ctx, client, userAgent, providers)
		nodes, skipped = append(nodes, pn...), append(skipped, ps...)
		if len(nodes) > 0 {
			err = nil
		} else if len(ps) > 0 {
			err = errors.New(i18n.T("没有找到可用的节点。") + ps[0])
		}
	}
	if err != nil {
		return nil, err
	}
	// a broken rest does not stop the nodes
	ex, _ := clash.Parse(string(body))
	if ex == nil && strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		ex = clash.ParseSingBox(string(body))
	}
	return &Result{
		Nodes:   nodes,
		Skipped: skipped,
		Usage:   parseUserInfo(header.Get("subscription-userinfo")),
		Clash:   ex,
		Title:   Title(header),
	}, nil
}

// get downloads a subscription or proxy-provider body.
func get(ctx context.Context, client *http.Client, rawURL, userAgent string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf(i18n.T("下载订阅失败: %w"), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf(i18n.T("下载订阅失败: HTTP %d"), resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, nil, err
	}
	return body, resp.Header, nil
}

// maxProviders caps how many proxy-providers one subscription may pull in.
const maxProviders = 20

// fetchProviders downloads the proxy-providers of a Clash subscription, as
// Clash would, and keeps the nodes each one's filter lets through. A
// provider that fails is reported in skipped and does not stop the others.
func fetchProviders(ctx context.Context, client *http.Client, userAgent string, providers []link.ClashProvider) (nodes []*link.Node, skipped []string) {
	for i, p := range providers {
		if i == maxProviders {
			skipped = append(skipped, fmt.Sprintf(i18n.T("proxy-providers 超过 %d 个，其余的没有下载"), maxProviders))
			break
		}
		body, _, err := get(ctx, client, p.URL, userAgent)
		if err != nil {
			skipped = append(skipped, p.Name+"："+err.Error())
			continue
		}
		got, sk, err := link.ParseContent(string(body))
		skipped = append(skipped, sk...)
		if err != nil {
			if len(sk) == 0 {
				skipped = append(skipped, p.Name+"："+err.Error())
			}
			continue
		}
		keep, drop := nameFilter(p.Filter), nameFilter(p.Exclude)
		for _, n := range got {
			if (keep == nil || keep.MatchString(n.Name)) && (drop == nil || !drop.MatchString(n.Name)) {
				nodes = append(nodes, n)
			}
		}
	}
	return nodes, skipped
}

// nameFilter compiles a provider's node name filter; nil when there is none
// or Go cannot read it (Clash.Meta allows a richer regexp dialect).
func nameFilter(expr string) *regexp.Regexp {
	if expr == "" {
		return nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil
	}
	return re
}

// Title reads the subscription's name from the response headers:
// "profile-title" (plain or "base64:…"), else the file name in
// Content-Disposition without its extension.
func Title(h http.Header) string {
	if t := strings.TrimSpace(h.Get("profile-title")); t != "" {
		if rest, ok := strings.CutPrefix(t, "base64:"); ok {
			for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
				if b, err := enc.DecodeString(strings.TrimSpace(rest)); err == nil {
					return strings.TrimSpace(string(b))
				}
			}
			return ""
		}
		return t
	}
	if _, params, err := mime.ParseMediaType(h.Get("Content-Disposition")); err == nil {
		name := strings.TrimSpace(params["filename"]) // filename*=UTF-8''… is decoded too
		if dot := strings.LastIndexByte(name, '.'); dot > 0 {
			switch strings.ToLower(name[dot:]) {
			case ".yaml", ".yml", ".txt", ".json", ".conf":
				name = name[:dot]
			}
		}
		return strings.TrimSpace(name)
	}
	return ""
}

// parseUserInfo parses "upload=1; download=2; total=3; expire=4".
func parseUserInfo(h string) *store.SubscriptionInfo {
	if h == "" {
		return nil
	}
	info := &store.SubscriptionInfo{}
	for _, part := range strings.Split(h, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "upload":
			info.Upload = n
		case "download":
			info.Download = n
		case "total":
			info.Total = n
		case "expire":
			info.Expire = n
		}
	}
	return info
}
