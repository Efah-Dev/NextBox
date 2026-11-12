package geo

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestParseTrace(t *testing.T) {
	body := "fl=1f1\nh=www.cloudflare.com\nip=203.0.113.7\nts=1.2\nloc=jp\ntls=TLSv1.3\n"
	if got := ParseTrace(body); got != (Location{IP: "203.0.113.7", Country: "JP"}) {
		t.Fatalf("ParseTrace = %+v", got)
	}
	for _, loc := range []string{"XX", "T1", "", "JPN", "1A"} {
		if got := ParseTrace("ip=2001:db8::1\nloc=" + loc); got.Country != "" || got.IP != "2001:db8::1" {
			t.Errorf("loc=%q gave %+v", loc, got)
		}
	}
	if got := ParseTrace("ip=not-an-ip\nloc=US"); got.IP != "" || got.Country != "US" {
		t.Errorf("bad IP kept: %+v", got)
	}
}

func TestTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.Write([]byte("ip=198.51.100.2\nloc=SG\n"))
		case "/unknown":
			w.Write([]byte("ip=198.51.100.2\nloc=XX\n"))
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()
	old := TraceURL
	defer func() { TraceURL = old }()

	TraceURL = srv.URL + "/ok"
	if loc, err := Trace(context.Background(), srv.Client()); err != nil || loc.Country != "SG" {
		t.Fatalf("Trace = %+v, %v", loc, err)
	}
	for _, path := range []string{"/unknown", "/denied"} {
		TraceURL = srv.URL + path
		if _, err := Trace(context.Background(), srv.Client()); err == nil {
			t.Errorf("%s did not fail", path)
		}
	}
}

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
