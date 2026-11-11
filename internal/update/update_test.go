package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.0.4", "1.0.3", true},
		{"v1.1.0", "1.0.9", true},
		{"1.10.0", "1.9.9", true},
		{"2.0", "1.99.99", true},
		{"1.0.3", "1.0.3", false},
		{"1.0.3", "v1.0.3", false},
		{"1.0.2", "1.0.3", false},
		{"1.14.4", "1.15.0-beta.2", false},
		{"1.15.0", "1.15.0-beta.2", true},
		{"1.15.0-beta.3", "1.15.0", false},
		{"", "1.0.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/app/releases/latest":
			w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://example.com/releases/v1.2.3"}`))
		case "/repos/owner/limited/releases/latest":
			w.WriteHeader(http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := API
	API = srv.URL
	defer func() { API = old }()

	rel, err := Latest(context.Background(), srv.Client(), "owner/app")
	if err != nil || rel.Version != "1.2.3" || rel.URL != "https://example.com/releases/v1.2.3" {
		t.Fatalf("Latest = %+v, %v", rel, err)
	}
	for _, repo := range []string{"owner/limited", "owner/missing"} {
		if _, err := Latest(context.Background(), srv.Client(), repo); err == nil {
			t.Errorf("Latest(%s) did not fail", repo)
		}
	}
}
