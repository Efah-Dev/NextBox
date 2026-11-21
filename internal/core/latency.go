package core

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/geo"
	"nextbox/internal/store"
)

type LatencyResult struct {
	ID      int64  `json:"id"`
	Latency int    `json:"latency"` // -1 = failed
	Error   string `json:"error,omitempty"`
	// Skipped is set when the node cannot be measured this way (a chain
	// proxy in a TCP test); its previous result stands.
	Skipped bool `json:"skipped,omitempty"`
	// Exit is where the node's traffic leaves, when it was looked up.
	Exit *geo.Location `json:"exit,omitempty"`
	// Kind is how it was measured: "tcp" or "url".
	Kind string `json:"kind,omitempty"`
}

// URLOnly reports whether a node can only be measured with a URL test: a
// protocol on top of UDP (QUIC) does not answer a TCP handshake, and a TCP
// test of a chain proxy would only reach its first hop.
func URLOnly(p store.Profile) bool {
	switch p.Type {
	case "hysteria", "hysteria2", "tuic", "wireguard", store.TypeChain:
		return true
	case "naive": // HTTP/3 (QUIC) runs over UDP
		quic, _ := p.Outbound["quic"].(bool)
		return quic
	}
	return false
}

// Progress receives the events of a latency test: Start when a node's
// measurement begins, Result when it ends.
type Progress struct {
	Start  func(id int64)
	Result func(LatencyResult)
	// Member looks up the members of chain proxies (URL test only).
	Member func(id int64) (store.Profile, bool)
	// Locate reports whether to also look up where a node's traffic leaves
	// (URL test only); nil means never.
	Locate func(id int64) bool
	// GroupDNS are the subscriptions' own DNS servers (URL test only).
	GroupDNS map[int64]store.GroupDNS
}

// TestURL measures real latency (HTTP request through the proxy) for nodes
// using a temporary sing-box instance. If the instance refuses to start
// because one node config is invalid, the batch is split in half until the
// broken nodes are isolated.
func TestURL(ctx context.Context, bin, workDir string, s store.Settings, nodes []store.Profile, p Progress) {
	if len(nodes) == 0 || ctx.Err() != nil {
		return
	}
	err := testBatch(ctx, bin, workDir, s, nodes, p)
	if err == nil || ctx.Err() != nil {
		return
	}
	if len(nodes) == 1 {
		p.Result(LatencyResult{ID: nodes[0].ID, Latency: -1, Error: firstLine(err.Error())})
		return
	}
	mid := len(nodes) / 2
	TestURL(ctx, bin, workDir, s, nodes[:mid], p)
	TestURL(ctx, bin, workDir, s, nodes[mid:], p)
}

func testBatch(ctx context.Context, bin, workDir string, s store.Settings, nodes []store.Profile, prog Progress) error {
	apiPort, err := FreePort()
	if err != nil {
		return err
	}
	proxyPort, err := FreePort()
	if err != nil {
		return err
	}
	api := NewAPI("127.0.0.1:"+strconv.Itoa(apiPort), RandomSecret())
	cfg, err := config.BuildTest(s, nodes, prog.Member, prog.GroupDNS, api.Addr, api.Secret, proxyPort)
	if err != nil {
		return err
	}
	proxyAddr := "127.0.0.1:" + strconv.Itoa(proxyPort)
	p, err := Start(ctx, bin, workDir, cfg, api, Options{})
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
			prog.Start(n.ID)
			proxy := &url.URL{Scheme: "http", User: url.UserPassword(config.TestUser(n.ID), api.Secret), Host: proxyAddr}
			timeout := time.Duration(s.TestTimeout) * time.Millisecond
			ms, err := geo.MeasureRTT(ctx, proxy, s.TestURL, timeout)
			r := LatencyResult{ID: n.ID, Latency: ms}
			if err != nil {
				r.Latency, r.Error = -1, err.Error()
			} else if prog.Locate != nil && prog.Locate(n.ID) {
				if loc, err := geo.Locate(ctx, proxy, timeout); err == nil {
					r.Exit = &loc
				}
			}
			prog.Result(r)
		}(n)
	}
	wg.Wait()
	return nil
}

// TCPPing measures the TCP handshake time to the node server, without a core.
func TCPPing(ctx context.Context, s store.Settings, nodes []store.Profile, p Progress) {
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
			p.Start(n.ID)
			if n.IsChain() {
				// only the first hop could be reached directly, which says nothing
				p.Result(LatencyResult{ID: n.ID, Skipped: true})
				return
			}
			d := net.Dialer{Timeout: time.Duration(s.TestTimeout) * time.Millisecond}
			start := time.Now()
			conn, err := d.DialContext(ctx, "tcp", n.Server())
			if err != nil {
				p.Result(LatencyResult{ID: n.ID, Latency: -1, Error: err.Error()})
				return
			}
			conn.Close()
			p.Result(LatencyResult{ID: n.ID, Latency: max(1, int(time.Since(start).Milliseconds()))})
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
