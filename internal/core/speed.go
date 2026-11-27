package core

// The download speed test: a file is downloaded through each node in turn,
// for at most a time and an amount, using the temporary core of the URL test
// (config.BuildTest: one proxy user per node). One node at a time, so that
// the nodes do not share the line.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"nextbox/internal/config"
	"nextbox/internal/i18n"
	"nextbox/internal/store"
)

// Defaults of the download speed test (store.Settings.SpeedTest*).
const (
	DefaultSpeedSeconds = 10
	DefaultSpeedMB      = 50
	speedURLBase        = "https://speed.cloudflare.com/__down?bytes="
)

// SpeedResult is the result for one node.
type SpeedResult struct {
	ID    int64  `json:"id"`
	Speed int64  `json:"speed"` // bytes per second; 0 when it failed
	Bytes int64  `json:"bytes"` // downloaded
	Error string `json:"error,omitempty"`
}

// SpeedProgress receives the events of a speed test.
type SpeedProgress struct {
	Start  func(id int64)
	Tick   func(r SpeedResult) // a few times per second while downloading
	Result func(r SpeedResult)
	// Member and GroupDNS: as for the URL test (Progress).
	Member   func(id int64) (store.Profile, bool)
	GroupDNS map[int64]store.GroupDNS
}

// SpeedLimits are the time and amount per node, and the file.
func SpeedLimits(s store.Settings) (time.Duration, int64, string) {
	secs, mb := s.SpeedTestSeconds, s.SpeedTestMB
	if secs <= 0 {
		secs = DefaultSpeedSeconds
	}
	if mb <= 0 {
		mb = DefaultSpeedMB
	}
	secs, mb = min(secs, 60), min(mb, 1024)
	bytes := int64(mb) << 20
	u := s.SpeedTestURL
	if u == "" {
		u = speedURLBase + strconv.FormatInt(bytes, 10)
	}
	return time.Duration(secs) * time.Second, bytes, u
}

// TestSpeed tests the nodes one after another.
func TestSpeed(ctx context.Context, bin, workDir string, s store.Settings, nodes []store.Profile, p SpeedProgress) {
	if len(nodes) == 0 || ctx.Err() != nil {
		return
	}
	err := speedBatch(ctx, bin, workDir, s, nodes, p)
	if err == nil || ctx.Err() != nil {
		return
	}
	// the core refused a node: find it by halves, as the URL test does
	if len(nodes) == 1 {
		p.Start(nodes[0].ID)
		p.Result(SpeedResult{ID: nodes[0].ID, Error: firstLine(err.Error())})
		return
	}
	mid := len(nodes) / 2
	TestSpeed(ctx, bin, workDir, s, nodes[:mid], p)
	TestSpeed(ctx, bin, workDir, s, nodes[mid:], p)
}

func speedBatch(ctx context.Context, bin, workDir string, s store.Settings, nodes []store.Profile, prog SpeedProgress) error {
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
	p, err := Start(ctx, bin, workDir, cfg, api, Options{})
	if err != nil {
		return err
	}
	defer p.Stop()
	limit, maxBytes, file := SpeedLimits(s)
	for _, n := range nodes {
		if ctx.Err() != nil {
			return nil
		}
		prog.Start(n.ID)
		proxy := &url.URL{Scheme: "http", User: url.UserPassword(config.TestUser(n.ID), api.Secret), Host: "127.0.0.1:" + strconv.Itoa(proxyPort)}
		r := download(ctx, proxy, file, limit, maxBytes, time.Duration(s.TestTimeout)*time.Millisecond, func(r SpeedResult) {
			r.ID = n.ID
			prog.Tick(r)
		})
		if ctx.Err() != nil {
			return nil // stopped: the node half done is not reported
		}
		r.ID = n.ID
		prog.Result(r)
	}
	return nil
}

// download fetches file through proxy until limit or maxBytes; the speed is
// measured from the first byte, so the connection set-up does not count.
func download(ctx context.Context, proxy *url.URL, file string, limit time.Duration, maxBytes int64, connect time.Duration, tick func(SpeedResult)) SpeedResult {
	if connect <= 0 {
		connect = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, connect+limit)
	defer cancel()
	tr := &http.Transport{Proxy: http.ProxyURL(proxy), ResponseHeaderTimeout: connect, DisableCompression: true}
	defer tr.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file, nil)
	if err != nil {
		return SpeedResult{Error: err.Error()}
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return SpeedResult{Error: firstLine(err.Error())}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SpeedResult{Error: fmt.Sprintf(i18n.T("测速文件返回 HTTP %d"), resp.StatusCode)}
	}
	buf := make([]byte, 64<<10)
	var got int64
	var start, last time.Time
	for got < maxBytes {
		n, err := resp.Body.Read(buf)
		now := time.Now()
		if n > 0 {
			if start.IsZero() {
				start, last = now, now
			}
			got += int64(n)
			if now.Sub(last) >= 250*time.Millisecond {
				last = now
				tick(SpeedResult{Bytes: got, Speed: rate(got, now.Sub(start))})
			}
			if now.Sub(start) >= limit {
				break
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
				break
			}
			if got == 0 {
				return SpeedResult{Error: firstLine(err.Error())}
			}
			break
		}
	}
	if got == 0 {
		return SpeedResult{Error: i18n.T("没有下载到数据")}
	}
	return SpeedResult{Bytes: got, Speed: rate(got, time.Since(start))}
}

func rate(bytes int64, d time.Duration) int64 {
	if d < 50*time.Millisecond {
		d = 50 * time.Millisecond
	}
	return int64(float64(bytes) / d.Seconds())
}
