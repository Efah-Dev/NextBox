// Package sub downloads and decodes subscriptions.
package sub

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"nextbox/internal/i18n"
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("下载订阅失败: %w"), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(i18n.T("下载订阅失败: HTTP %d"), resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	nodes, skipped, err := link.ParseContent(string(body))
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
		Usage:   parseUserInfo(resp.Header.Get("subscription-userinfo")),
		Clash:   ex,
		Title:   Title(resp.Header),
	}, nil
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
