//go:build windows

package app

import (
	"encoding/json"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// The tray menu window comes up with its page, reports its size and shows;
// a pick in it reaches onMessage. It opens a real window near the mouse for
// a moment, so it only runs with NEXTBOX_UI_TEST=1.
func TestTrayPopupShows(t *testing.T) {
	if os.Getenv("NEXTBOX_UI_TEST") == "" {
		t.Skip("set NEXTBOX_UI_TEST=1 to open the window")
	}
	var picked atomic.Value
	p := &trayPopup{log: func(s string) { t.Log(s) }, onMessage: func(m string) { picked.Store(m) }}
	v := trayView{
		Theme: "light", Accent: "#F472B6", Parts: map[string]bool{"quick": true, "mode": true, "nodes": true, "policies": true, "latency": true},
		Running: true, Since: time.Now().UnixMilli(), Mode: "rule", Current: "Tokyo 01", Latency: 48, Version: "test",
		Groups: []trayGroup{{ID: 1, Name: "g", Count: 1, Nodes: []trayNode{{ID: 1, Name: "Tokyo 01", Latency: 48}}}}, Labels: map[string]string{},
	}
	b, _ := json.Marshal(v)
	start := time.Now()
	if err := p.Show(string(b)); err != nil {
		t.Fatal(err)
	}
	shown := make(chan bool, 1)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		p.do(func() { shown <- p.shown })
		if <-shown {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	p.do(func() { shown <- p.shown })
	if !<-shown {
		t.Fatal("the menu did not show")
	}
	t.Logf("shown after %v", time.Since(start))
	// a pick from the page reaches the app
	p.do(func() { p.web.Eval(`document.querySelector('[data-a="quit"]').click()`) })
	for i := 0; i < 50 && picked.Load() == nil; i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if m, _ := picked.Load().(string); m != `{"a":"quit"}` {
		t.Errorf("pick: %q", m)
	}
	// a second open is quick (the window is kept)
	p.Hide()
	time.Sleep(400 * time.Millisecond)
	start = time.Now()
	p.Show(string(b))
	for i := 0; i < 50; i++ {
		p.do(func() { shown <- p.shown })
		if <-shown {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("second open after %v", time.Since(start))
	visible := func() bool {
		ch := make(chan bool)
		p.do(func() { r, _, _ := procIsWindowVisible.Call(p.hwnd); ch <- r != 0 })
		return <-ch
	}
	// closing fades the menu out, then the window goes
	p.Hide()
	time.Sleep(50 * time.Millisecond)
	if !visible() {
		t.Error("the window went before the menu faded out")
	}
	time.Sleep(400 * time.Millisecond)
	if visible() {
		t.Error("the window stayed after the menu was closed")
	}
	// opened again before the earlier close's fallback (300 ms) ran: that
	// close does not hide the new menu (within 200 ms, a reopen counts as the
	// second click that closes it)
	p.Show(string(b))
	time.Sleep(400 * time.Millisecond)
	p.Hide()
	time.Sleep(230 * time.Millisecond)
	p.Show(string(b))
	time.Sleep(600 * time.Millisecond)
	if !visible() {
		t.Error("reopened soon after closing, the window was hidden")
	}
	p.Hide()
	time.Sleep(400 * time.Millisecond)
}

var procIsWindowVisible = user32.NewProc("IsWindowVisible")
