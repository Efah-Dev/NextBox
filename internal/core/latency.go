package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"nextbox/internal/i18n"
	"strconv"
	"strings"
	"sync"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/store"
)

type LatencyResult struct {
	ID      int64  `json:"id"`
	Latency int    `json:"latency"` // -1 = failed
	Error   string `json:"error,omitempty"`
}

// TestURL measures real latency (HTTP request through the proxy) for nodes
// using a temporary sing-box instance. If the instance refuses to start
// because one node config is invalid, the batch is split in half until the
// broken nodes are isolated.
func TestURL(ctx context.Context, bin, workDir string, s store.Settings, nodes []store.Profile, onResult func(LatencyResult)) {
	if len(nodes) == 0 || ctx.Err() != nil {
		return
	}
	err := testBatch(ctx, bin, workDir, s, nodes, onResult)
	if err == nil || ctx.Err() != nil {
		return
	}
	if len(nodes) == 1 {
		onResult(LatencyResult{ID: nodes[0].ID, Latency: -1, Error: firstLine(err.Error())})
		return
	}
	mid := len(nodes) / 2
	TestURL(ctx, bin, workDir, s, nodes[:mid], onResult)
	TestURL(ctx, bin, workDir, s, nodes[mid:], onResult)
}

func testBatch(ctx context.Context, bin, workDir string, s store.Settings, nodes []store.Profile, onResult func(LatencyResult)) error {
	apiPort, err := FreePort()
	if err != nil {
		return err
	}
	proxyPort, err := FreePort()
	if err != nil {
		return err
	}
	api := NewAPI("127.0.0.1:"+strconv.Itoa(apiPort), RandomSecret())
	cfg, err := config.BuildTest(s, nodes, api.Addr, api.Secret, proxyPort)
	if err != nil {
		return err
	}
	proxyAddr := "127.0.0.1:" + strconv.Itoa(proxyPort)
	p, err := Start(ctx, bin, workDir, "test.json", cfg, api, nil)
	if err != nil {
		return err
	}
	defer p.Stop()

	sem := make(chan struct{}, max(1, s.TestConcurrency))
	var wg sync.WaitGroup
	for _, n := range nodes {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(n store.Profile) {
			defer func() { <-sem; wg.Done() }()
			proxy := &url.URL{Scheme: "http", User: url.UserPassword(config.TestUser(n.ID), api.Secret), Host: proxyAddr}
			ms, err := measureRTT(ctx, proxy, s.TestURL, time.Duration(s.TestTimeout)*time.Millisecond)
			r := LatencyResult{ID: n.ID, Latency: ms}
			if err != nil {
				r.Latency, r.Error = -1, err.Error()
			}
			onResult(r)
		}(n)
	}
	wg.Wait()
	return nil
}

// measureRTT sends two requests over one kept-alive connection through the
// proxy and times the second one. The first request pays for the TCP and
// TLS / Reality handshakes; the second shows the real round trip a user feels
// on an open connection (the same method NekoBox uses for "URL test").
func measureRTT(ctx context.Context, proxy *url.URL, testURL string, timeout time.Duration) (int, error) {
	tr := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 1,
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: timeout,
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: timeout}

	get := func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return nil
	}
	if err := get(); err != nil {
		return -1, simplifyErr(err)
	}
	start := time.Now()
	if err := get(); err != nil {
		return -1, simplifyErr(err)
	}
	return max(1, int(time.Since(start).Milliseconds())), nil
}

// simplifyErr strips Go's "Get \"url\": proxyconnect ..." wrapping.
func simplifyErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout") {
		return errors.New(i18n.T("超时"))
	}
	return err
}

// TCPPing measures the TCP handshake time to the node server, without a core.
func TCPPing(ctx context.Context, s store.Settings, nodes []store.Profile, onResult func(LatencyResult)) {
	sem := make(chan struct{}, max(1, s.TestConcurrency))
	var wg sync.WaitGroup
	for _, n := range nodes {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(n store.Profile) {
			defer func() { <-sem; wg.Done() }()
			d := net.Dialer{Timeout: time.Duration(s.TestTimeout) * time.Millisecond}
			start := time.Now()
			conn, err := d.DialContext(ctx, "tcp", n.Server())
			if err != nil {
				onResult(LatencyResult{ID: n.ID, Latency: -1, Error: err.Error()})
				return
			}
			conn.Close()
			onResult(LatencyResult{ID: n.ID, Latency: max(1, int(time.Since(start).Milliseconds()))})
		}(n)
	}
	wg.Wait()
}

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			rest := s[i+1:]
			// the first line is our own prefix; prefer the core's message
			for j, c2 := range rest {
				if c2 == '\n' {
					return rest[:j]
				}
			}
			return rest
		}
	}
	return s
}
