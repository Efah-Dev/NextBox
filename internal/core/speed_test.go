package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nextbox/internal/store"
)

func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		chunk := []byte(strings.Repeat("x", 32<<10))
		for i := 0; i < 64; i++ { // 2 MB
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	ticks := 0
	// capped by the amount
	r := download(context.Background(), nil, srv.URL, 10*time.Second, 1<<20, time.Second, func(SpeedResult) { ticks++ })
	if r.Error != "" || r.Bytes < 1<<20 || r.Bytes > 1<<20+64<<10 || r.Speed <= 0 {
		t.Errorf("capped download: %+v", r)
	}
	// the whole file when it is smaller
	if r = download(context.Background(), nil, srv.URL, 10*time.Second, 50<<20, time.Second, func(SpeedResult) {}); r.Bytes != 2<<20 {
		t.Errorf("whole file: %+v", r)
	}
	if r = download(context.Background(), nil, srv.URL+"/missing", time.Second, 1<<20, time.Second, func(SpeedResult) {}); r.Error == "" {
		t.Errorf("404 accepted: %+v", r)
	}
}

func TestSpeedLimits(t *testing.T) {
	d, b, u := SpeedLimits(store.Settings{})
	if d != 10*time.Second || b != 50<<20 || u != speedURLBase+"52428800" {
		t.Errorf("defaults: %v %d %s", d, b, u)
	}
	d, b, u = SpeedLimits(store.Settings{SpeedTestSeconds: 999, SpeedTestMB: 5, SpeedTestURL: "https://x.example/f"})
	if d != 60*time.Second || b != 5<<20 || u != "https://x.example/f" {
		t.Errorf("custom: %v %d %s", d, b, u)
	}
}
