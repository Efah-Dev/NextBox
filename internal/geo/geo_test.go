package geo

import (
	"net"
	"os"
	"testing"
)

// The database is a 3.7 MB download, so this only runs when one is given:
//
//	NEXTBOX_GEOIP_DB=path/to/geoip.db go test ./internal/geo
func TestDB(t *testing.T) {
	path := os.Getenv("NEXTBOX_GEOIP_DB")
	if path == "" {
		t.Skip("set NEXTBOX_GEOIP_DB to a sing-geoip geoip.db")
	}
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for ip, want := range map[string]string{
		"223.5.5.5": "CN", // AliDNS
		"8.8.8.8":   "US", // Google DNS
		"127.0.0.1": "",   // private addresses have no country
	} {
		got := db.Country(net.ParseIP(ip))
		if got != want {
			t.Errorf("%s: got %q, want %q", ip, got, want)
		}
		t.Logf("%s -> %q", ip, got)
	}
}
