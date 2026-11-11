package sub

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const links = "ss://YWVzLTI1Ni1nY206cGFzcw@1.2.3.4:8388#first\n" +
	"trojan://pass@example.com:443#second\n"

func TestFetch(t *testing.T) {
	var gotUA atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA.Store(r.UserAgent())
		switch r.URL.Path {
		case "/base64":
			w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=1000; expire=1767225600")
			w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(links))))
		case "/plain":
			w.Write([]byte(links))
		case "/gone":
			http.NotFound(w, r)
		default:
			w.Write([]byte("<html>not a subscription</html>"))
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	res, err := Fetch(ctx, "  "+srv.URL+"/base64\n", "TestAgent/1.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 2 || res.Nodes[0].Name != "first" || res.Nodes[1].Type() != "trojan" {
		t.Fatalf("nodes: %+v", res.Nodes)
	}
	if u := res.Usage; u == nil || u.Upload != 1 || u.Download != 2 || u.Total != 1000 || u.Expire != 1767225600 {
		t.Errorf("usage: %+v", res.Usage)
	}
	if gotUA.Load() != "TestAgent/1.0" {
		t.Errorf("User-Agent sent: %v", gotUA.Load())
	}

	res, err = Fetch(ctx, srv.URL+"/plain", "x", "")
	if err != nil || len(res.Nodes) != 2 || res.Usage != nil {
		t.Errorf("plain list: %+v, %v", res, err)
	}

	if _, err := Fetch(ctx, srv.URL+"/gone", "x", ""); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("HTTP 404 was not reported: %v", err)
	}
	if _, err := Fetch(ctx, srv.URL+"/html", "x", ""); err == nil {
		t.Error("content without nodes did not fail")
	}
}

// With a proxy given, the request has to go through it.
func TestFetchThroughProxy(t *testing.T) {
	var viaProxy atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// a plain-HTTP proxy receives the absolute URL of the target
		viaProxy.Store(r.URL.Host == "subscription.invalid")
		w.Write([]byte(links))
	}))
	defer proxy.Close()
	res, err := Fetch(context.Background(), "http://subscription.invalid/sub", "x", proxy.URL)
	if err != nil || len(res.Nodes) != 2 {
		t.Fatalf("fetch through proxy: %+v, %v", res, err)
	}
	if !viaProxy.Load() {
		t.Error("the request did not go through the proxy")
	}
}

func TestParseUserInfo(t *testing.T) {
	if parseUserInfo("") != nil {
		t.Error("an absent header must give nil")
	}
	// spacing, case and unknown or malformed fields are tolerated
	u := parseUserInfo("Upload=10;download = 20 ; total=30;junk; expire=;other=1")
	if u == nil || u.Upload != 10 || u.Download != 20 || u.Total != 30 || u.Expire != 0 {
		t.Errorf("parsed: %+v", u)
	}
}
