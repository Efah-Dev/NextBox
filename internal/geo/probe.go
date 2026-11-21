package geo

// Measuring a request through a proxy and asking where it leaves (the exit
// IP and its network): the home page's exit check on both systems, and the
// URL test of NextBox for Windows.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nextbox/internal/i18n"
)

// MeasureRTT sends two requests over one kept-alive connection through the
// proxy and times the second one. The first request pays for the TCP and
// TLS / Reality handshakes; the second shows the real round trip a user feels
// on an open connection (the same method NekoBox uses for "URL test").
func MeasureRTT(ctx context.Context, proxy *url.URL, testURL string, timeout time.Duration) (int, error) {
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
		return -1, SimplifyErr(err)
	}
	start := time.Now()
	if err := get(); err != nil {
		return -1, SimplifyErr(err)
	}
	return max(1, int(time.Since(start).Milliseconds())), nil
}

// ProbeExit measures a request through proxy (see MeasureRTT) and asks where
// it leaves from. ms is -1 when the request failed.
func ProbeExit(ctx context.Context, proxy *url.URL, testURL string, timeout time.Duration) (int, Location, error) {
	ms, err := MeasureRTT(ctx, proxy, testURL, timeout)
	if err != nil {
		return -1, Location{}, &ProbeError{ProbeStepTest, err}
	}
	loc, err := Locate(ctx, proxy, timeout)
	if err != nil {
		return ms, Location{}, &ProbeError{ProbeStepTrace, SimplifyErr(err)}
	}
	return ms, loc, nil
}

// Locate asks where requests sent through the proxy come from. Each try is
// a new connection through the node; when the first does not answer (a slow
// handshake, a lost packet) the trace is asked once more at Cloudflare's DNS
// address, and the first error is reported if that fails too.
func Locate(ctx context.Context, proxy *url.URL, timeout time.Duration) (Location, error) {
	tr := &http.Transport{Proxy: http.ProxyURL(proxy), ForceAttemptHTTP2: true, TLSHandshakeTimeout: timeout}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: timeout}
	loc, err := Trace(ctx, client)
	if err == nil || TraceFallbackURL == "" || ctx.Err() != nil {
		return loc, err
	}
	if l, err2 := TraceAt(ctx, client, TraceFallbackURL); err2 == nil {
		return l, nil
	}
	return loc, err
}

// Which request of ProbeExit failed.
const (
	ProbeStepTest  = "test"  // the test URL (Settings.TestURL)
	ProbeStepTrace = "trace" // the Cloudflare trace
)

// ProbeError is a failed ProbeExit: Step says which request it was.
type ProbeError struct {
	Step string
	Err  error
}

func (e *ProbeError) Error() string { return e.Err.Error() }
func (e *ProbeError) Unwrap() error { return e.Err }

// ExitNetwork asks, through the proxy, which network ip belongs to.
func ExitNetwork(ctx context.Context, proxy *url.URL, ip string, timeout time.Duration) (Network, error) {
	tr := &http.Transport{Proxy: http.ProxyURL(proxy), ForceAttemptHTTP2: true, TLSHandshakeTimeout: timeout}
	defer tr.CloseIdleConnections()
	n, err := LookupASN(ctx, &http.Client{Transport: tr, Timeout: timeout}, ip)
	if err != nil {
		return n, SimplifyErr(err)
	}
	return n, nil
}

// SimplifyErr strips Go's "Get \"url\": proxyconnect ..." wrapping.
func SimplifyErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout") {
		return errors.New(i18n.T("超时"))
	}
	return err
}
