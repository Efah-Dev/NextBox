// Package geo finds out which country a node is in: either from the exit IP,
// asked from Cloudflare through the node, or from the server IP, looked up in
// the sing-geoip database.
package geo

import (
	"context"
	"fmt"
	"net"

	"github.com/oschwald/maxminddb-golang"
)

// DBURLs are the places the sing-geoip database is downloaded from, tried in
// order (mirrors first: GitHub is not reachable everywhere).
var DBURLs = []string{
	"https://testingcf.jsdelivr.net/gh/SagerNet/sing-geoip@release/geoip.db",
	"https://cdn.jsdelivr.net/gh/SagerNet/sing-geoip@release/geoip.db",
	"https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db",
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
