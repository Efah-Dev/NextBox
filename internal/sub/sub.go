// Package sub downloads and decodes subscriptions.
package sub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nextbox/internal/i18n"
	"strconv"
	"strings"
	"time"

	"nextbox/internal/link"
	"nextbox/internal/store"
)

type Result struct {
	Nodes   []*link.Node
	Skipped []string // why each skipped entry was rejected
	Usage   *store.SubscriptionInfo
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
	return &Result{
		Nodes:   nodes,
		Skipped: skipped,
		Usage:   parseUserInfo(resp.Header.Get("subscription-userinfo")),
	}, nil
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
		switch strings.ToLower(k) {
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
