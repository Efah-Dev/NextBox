// Where requests sent through a node come from: the Cloudflare trace
// (shared by NextBox for Windows and NextBox for Android).

package geo

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// TraceURL answers with the IP and country a request comes from; tests point
// it at a local server.
var TraceURL = "https://www.cloudflare.com/cdn-cgi/trace"

// TraceFallbackURL is asked when TraceURL does not answer: the same trace
// at Cloudflare's DNS address, which needs no name lookup on the node.
var TraceFallbackURL = "https://1.1.1.1/cdn-cgi/trace"

// Location is where a request came from.
type Location struct {
	IP      string `json:"ip"`
	Country string `json:"country"` // ISO 3166 code, upper case
}

// Trace asks TraceURL where the client's requests come from. Send it through
// a node to learn the node's exit IP.
func Trace(ctx context.Context, client *http.Client) (Location, error) {
	return TraceAt(ctx, client, TraceURL)
}

// TraceAt asks the Cloudflare trace at rawURL.
func TraceAt(ctx context.Context, client *http.Client, rawURL string) (Location, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Location{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Location{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Location{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if err != nil {
		return Location{}, err
	}
	loc := ParseTrace(string(body))
	if loc.Country == "" {
		return loc, errors.New("no country in the answer")
	}
	return loc, nil
}

// ParseTrace reads the "key=value" lines of a Cloudflare trace. Codes that
// are not countries (XX = unknown, T1 = Tor) are dropped.
func ParseTrace(body string) Location {
	var loc Location
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok {
			continue
		}
		switch k {
		case "ip":
			if net.ParseIP(v) != nil {
				loc.IP = v
			}
		case "loc":
			loc.Country = Code(v)
		}
	}
	return loc
}

// Code normalises a country code: two letters, upper case, or "" when it is
// not a real country.
func Code(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 2 || s[0] < 'A' || s[0] > 'Z' || s[1] < 'A' || s[1] > 'Z' {
		return ""
	}
	switch s {
	case "XX", "T1", "ZZ", "EU", "AP":
		return ""
	}
	return s
}
