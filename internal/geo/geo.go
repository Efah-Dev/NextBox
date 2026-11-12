// Package geo finds out which country a node is in: either from the exit IP,
// asked from Cloudflare through the node, or from the server IP, looked up in
// the sing-geoip database.
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

	"github.com/oschwald/maxminddb-golang"
)

// TraceURL answers with the IP and country a request comes from; tests point
// it at a local server.
var TraceURL = "https://www.cloudflare.com/cdn-cgi/trace"

// DBURLs are the places the sing-geoip database is downloaded from, tried in
// order (mirrors first: GitHub is not reachable everywhere).
var DBURLs = []string{
	"https://testingcf.jsdelivr.net/gh/SagerNet/sing-geoip@release/geoip.db",
	"https://cdn.jsdelivr.net/gh/SagerNet/sing-geoip@release/geoip.db",
	"https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db",
}

// Location is where a request came from.
type Location struct {
	IP      string `json:"ip"`
	Country string `json:"country"` // ISO 3166 code, upper case
}

// Trace asks TraceURL where the client's requests come from. Send it through
// a node to learn the node's exit IP.
func Trace(ctx context.Context, client *http.Client) (Location, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, TraceURL, nil)
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

// DB is the sing-geoip country database (MaxMind DB format, IP -> code).
type DB struct {
	r *maxminddb.Reader
}

func Open(path string) (*DB, error) {
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	if r.Metadata.DatabaseType != "sing-geoip" {
		r.Close()
		return nil, fmt.Errorf("unexpected database type %q", r.Metadata.DatabaseType)
	}
	return &DB{r: r}, nil
}

// Country returns the code of the country ip belongs to, or "".
func (d *DB) Country(ip net.IP) string {
	var code string
	if err := d.r.Lookup(ip, &code); err != nil {
		return ""
	}
	return Code(code)
}

func (d *DB) Close() error { return d.r.Close() }

// Resolve returns the first IP of host (host itself when it is an IP),
// preferring IPv4 as the core does by default.
func Resolve(ctx context.Context, host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		if v4 := a.IP.To4(); v4 != nil {
			return v4, nil
		}
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no address for %s", host)
	}
	return addrs[0].IP, nil
}
