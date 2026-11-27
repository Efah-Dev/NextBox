//go:build windows

package app

import (
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

var procGetWindowRect = user32.NewProc("GetWindowRect")

// Going into a group and back gives the menu card its first height again
// while the window keeps its size, the nodes of a group show their flags and
// its test button works. Opens a real window (NEXTBOX_UI_TEST=1).
func TestTrayPopupBackAndFlags(t *testing.T) {
	if os.Getenv("NEXTBOX_UI_TEST") == "" {
		t.Skip("set NEXTBOX_UI_TEST=1 to open the window")
	}
	var got atomic.Value
	p := &trayPopup{log: func(s string) { t.Log(s) }, onMessage: func(m string) { got.Store(m) }}
	v := trayView{Theme: "dark", Dark: true, Parts: map[string]bool{"quick": true, "mode": true, "nodes": true, "policies": true, "latency": true}, Mode: "rule", Version: "t",
		Groups: []trayGroup{{ID: 1, Name: "g", Count: 14}}, Labels: map[string]string{}}
	for i := 0; i < 14; i++ {
		v.Groups[0].Nodes = append(v.Groups[0].Nodes, trayNode{ID: int64(i + 1), Name: "Node", Latency: 100, Country: "JP"})
	}
	b, _ := json.Marshal(v)
	if err := p.Show(string(b)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	window := func() int32 {
		var r rect
		ch := make(chan rect)
		p.do(func() { procGetWindowRect.Call(p.hwnd, uintptr(unsafe.Pointer(&r))); ch <- r })
		r = <-ch
		return r.Bottom - r.Top
	}
	// the card's height, and whether it lies inside the window
	card := func() (h int, inside bool) {
		got.Store("")
		p.do(func() {
			p.web.Eval(`(() => { const r = document.getElementById('root').getBoundingClientRect(); window.chrome.webview.postMessage(JSON.stringify({a: 'probe', v: Math.round(r.height) + '/' + (r.top >= 0 && r.bottom <= innerHeight)})) })()`)
		})
		for i := 0; i < 20; i++ {
			time.Sleep(50 * time.Millisecond)
			if m, _ := got.Load().(string); m != "" {
				var p struct{ V string }
				json.Unmarshal([]byte(m), &p)
				fmt.Sscanf(p.V, "%d/%t", &h, &inside)
				return
			}
		}
		return 0, false
	}
	win := window()
	first, ok := card()
	if !ok {
		t.Error("the card is not inside the window")
	}
	p.do(func() {
		p.web.Eval(`window.chrome.webview.postMessage(JSON.stringify({a: 'probe', v: document.documentElement.classList.contains('dark') + ''}))`)
	})
	time.Sleep(500 * time.Millisecond)
	if m, _ := got.Load().(string); m != `{"a":"probe","v":"true"}` {
		t.Errorf("dark: %s", m)
	}
	p.do(func() { p.web.Eval(`document.querySelector('[data-go="group:1"]').click()`) })
	time.Sleep(time.Second)
	inGroup, ok := card()
	if !ok {
		t.Error("the group's card is not inside the window")
	}
	p.do(func() {
		p.web.Eval(`window.chrome.webview.postMessage(JSON.stringify({a: 'probe', v: document.querySelectorAll('.flag').length + ''}))`)
	})
	time.Sleep(500 * time.Millisecond)
	if m, _ := got.Load().(string); m != `{"a":"probe","v":"14"}` {
		t.Errorf("flags in the group: %s", m)
	}
	// the subscription's test button asks NextBox to test it
	p.do(func() { p.web.Eval(`document.querySelector('[data-a="testGroup"]').click()`) })
	time.Sleep(300 * time.Millisecond)
	if m, _ := got.Load().(string); m != `{"a":"testGroup","id":1}` {
		t.Errorf("test button: %s", m)
	}
	p.do(func() { p.web.Eval(`document.querySelector('[data-go="main"]').click()`) })
	time.Sleep(time.Second)
	back, _ := card()
	t.Logf("card heights: %d, in group %d, back %d", first, inGroup, back)
	if first < 150 || back != first || inGroup <= first {
		t.Errorf("card heights: %d, in group %d, back %d", first, inGroup, back)
	}
	if w := window(); w != win {
		t.Errorf("the window changed its height: %d, then %d", win, w)
	}
	p.Hide()
	time.Sleep(300 * time.Millisecond)
}

var procSetCursorPos = user32.NewProc("SetCursorPos")

// The menu card opens at the mouse: on the taskbar it sits on the taskbar,
// over the hidden icons (above the taskbar) it ends at the mouse, near the
// top of the screen it starts there. Moves the mouse (NEXTBOX_UI_TEST=1).
func TestTrayPopupPlace(t *testing.T) {
	if os.Getenv("NEXTBOX_UI_TEST") == "" {
		t.Skip("set NEXTBOX_UI_TEST=1 to open the window")
	}
	var was point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&was)))
	defer procSetCursorPos.Call(uintptr(was.X), uintptr(was.Y))
	var got atomic.Value
	p := &trayPopup{log: func(s string) { t.Log(s) }, onMessage: func(m string) { got.Store(m) }}
	v := trayView{Parts: map[string]bool{"quick": true, "mode": true, "nodes": true}, Mode: "rule", Version: "t",
		Groups: []trayGroup{{ID: 1, Name: "g", Count: 1, Nodes: []trayNode{{ID: 1, Name: "n"}}}}, Labels: map[string]string{}}
	b, _ := json.Marshal(v)
	work := workArea(was)
	// the card's top and bottom on the screen
	card := func() (top, bottom int32) {
		var r rect
		ch := make(chan rect)
		p.do(func() { procGetWindowRect.Call(p.hwnd, uintptr(unsafe.Pointer(&r))); ch <- r })
		r = <-ch
		dpi := make(chan uintptr)
		p.do(func() { d, _, _ := procGetDpiForWindow.Call(p.hwnd); dpi <- d })
		scale := float64(<-dpi) / 96
		got.Store("")
		p.do(func() {
			p.web.Eval(`(() => { const r = document.getElementById('root').getBoundingClientRect(); window.chrome.webview.postMessage(JSON.stringify({a: 'probe', v: Math.round(r.top) + '/' + Math.round(r.bottom)})) })()`)
		})
		for i := 0; i < 20; i++ {
			time.Sleep(50 * time.Millisecond)
			if m, _ := got.Load().(string); m != "" {
				var pr struct{ V string }
				json.Unmarshal([]byte(m), &pr)
				var t0, b0 int
				fmt.Sscanf(pr.V, "%d/%d", &t0, &b0)
				return r.Top + int32(float64(t0)*scale), r.Top + int32(float64(b0)*scale)
			}
		}
		return 0, 0
	}
	x := (work.Left + work.Right) / 2
	cases := []struct {
		name  string
		mouse int32
		check func(top, bottom int32) bool
	}{
		{"on the taskbar", work.Bottom + 10, func(top, bottom int32) bool { return bottom <= work.Bottom && bottom > work.Bottom-40 }},
		{"over the hidden icons", work.Bottom - 120, func(top, bottom int32) bool { return bottom >= work.Bottom-150 && bottom <= work.Bottom-100 }},
		{"near the top", work.Top + 150, func(top, bottom int32) bool { return top >= work.Top+120 && top <= work.Top+180 }},
	}
	for _, c := range cases {
		procSetCursorPos.Call(uintptr(x), uintptr(c.mouse))
		if err := p.Show(string(b)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1500 * time.Millisecond)
		top, bottom := card()
		t.Logf("%s (mouse at %d): card %d–%d", c.name, c.mouse, top, bottom)
		if !c.check(top, bottom) {
			t.Errorf("%s: the card is at %d–%d, the mouse at %d (work area %d–%d)", c.name, top, bottom, c.mouse, work.Top, work.Bottom)
		}
		p.Hide()
		time.Sleep(500 * time.Millisecond)
	}
}
