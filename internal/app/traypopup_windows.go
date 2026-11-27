//go:build windows

package app

// The tray menu in NextBox's own look: a small borderless window with a
// WebView2 of its own (traymenu.html), shown above the tray icon on a right
// click and hidden when it loses the focus. It shares the browser process of
// the main window (same data folder and arguments as Wails). If WebView2
// cannot be set up for it, the tray keeps the plain Windows menu.

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"path"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

//go:embed traymenu.html
var trayMenuHTML string

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostThreadMessage   = user32.NewProc("PostThreadMessageW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procMonitorFromPoint    = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfo      = user32.NewProc("GetMonitorInfoW")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procLoadCursor          = user32.NewProc("LoadCursorW")
)

const (
	popupWidth = 288 // the menu card, CSS pixels
	// The window is larger than the card and see-through around it: the
	// card's shadow is in popupMargin, and the window is popupHeight high
	// whatever the card's height (see place).
	popupMargin = 12
	popupHeight = 620
	wmRun       = 0x8000 + 0x4E43
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }

type winMsg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type wndClassEx struct {
	size, style                        uint32
	wndProc                            uintptr
	clsExtra, wndExtra                 int32
	instance, icon, cursor, background windows.Handle
	menuName, className                *uint16
	iconSm                             windows.Handle
}

// trayPopup is the window; its methods may be called from any goroutine.
type trayPopup struct {
	onMessage func(msg string) // a message from the page (an action)
	files     fs.FS            // the built UI (frontend/dist), for its fonts
	log       func(string)     // problems go to the log page

	loaded bool   // the page is there (window's thread only)
	state  string // a state that came before the page did

	once    sync.Once
	failed  bool
	tid     uint32
	hwnd    uintptr
	web     *edge.Chromium
	mu      sync.Mutex
	calls   []func()
	anchor  point // where the click was, in screen pixels
	pending bool  // waits for the page's size before showing
	shown   bool
	edge    string // the edge that stays put: "bottom", "top" (see edgeAt)
	hidAt   time.Time
	hiding  int         // counts hides, see hideNow
	open    atomic.Bool // shown, for other goroutines
}

var popupProc = windows.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
	if p := thePopup; p != nil && hwnd == p.hwnd {
		switch msg {
		case 0x0006: // WM_ACTIVATE
			if wParam&0xFFFF == 0 { // WA_INACTIVE
				p.hideNow()
			}
		case 0x0005: // WM_SIZE
			if p.web != nil {
				p.web.Resize()
			}
		case 0x0010: // WM_CLOSE
			p.hideNow()
			return 0
		}
	}
	r, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return r
})

// thePopup is the one popup of the process (the window procedure needs it).
var thePopup *trayPopup

// start creates the window and its WebView2 on a thread of their own, once;
// it reports whether that worked. It gives up after a while: setting up
// WebView2 can hang (its environment never completes), and the tray must not.
func (p *trayPopup) start() bool {
	p.once.Do(func() {
		thePopup = p
		ready := make(chan bool, 1)
		go p.loop(ready)
		select {
		case ok := <-ready:
			p.failed = !ok
		case <-time.After(10 * time.Second):
			p.failed = true
			p.logf("WebView2 did not start within 10 s")
		}
	})
	return !p.failed
}

func (p *trayPopup) logf(msg string) {
	if p.log != nil {
		p.log("[tray menu] " + msg)
	}
}

func (p *trayPopup) loop(ready chan bool) {
	runtime.LockOSThread()
	windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	p.tid = windows.GetCurrentThreadId()
	var inst windows.Handle
	windows.GetModuleHandleEx(0, nil, &inst)
	cls, _ := windows.UTF16PtrFromString("NextBoxTrayMenu")
	cursor, _, _ := procLoadCursor.Call(0, 32512) // IDC_ARROW
	// no class shadow: the window is see-through, the page draws the card and its shadow
	wc := wndClassEx{wndProc: popupProc, instance: inst, cursor: windows.Handle(cursor), className: cls}
	wc.size = uint32(unsafe.Sizeof(wc))
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	// WS_EX_NOREDIRECTIONBITMAP: the window has no surface of its own, only
	// the WebView's, so what the page leaves transparent shows what is behind.
	const wsPopup, wsExToolWindow, wsExTopmost, wsExNoRedirectionBitmap = 0x80000000, 0x00000080, 0x00000008, 0x00200000
	hwnd, _, _ := procCreateWindowEx.Call(wsExToolWindow|wsExTopmost|wsExNoRedirectionBitmap, uintptr(unsafe.Pointer(cls)), 0, wsPopup,
		0, 0, popupWidth, 300, 0, 0, uintptr(inst), 0)
	if hwnd == 0 {
		p.logf("CreateWindowEx failed")
		ready <- false
		return
	}
	p.hwnd = hwnd
	web := edge.NewChromium()
	// the same as Wails' window, so that both use one browser process
	web.AdditionalBrowserArgs = []string{"--disable-features=msSmartScreenProtection"}
	// most errors reported here are not fatal (an optional interface that
	// is missing): they are logged, and only a missing controller fails
	web.SetErrorCallback(func(err error) { p.logf(err.Error()) })
	web.MessageCallback = func(msg string, _ *edge.ICoreWebView2, _ *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		p.received(msg)
	}
	web.NavigationCompletedCallback = func(_ *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		p.loaded = true
		if s := p.state; s != "" {
			p.state = ""
			p.web.Eval("window.nb && nb.render(" + s + ", true, '" + p.edge + "')")
		}
	}
	if !web.Embed(hwnd) || web.GetController() == nil {
		p.logf("WebView2 could not be set up")
		ready <- false
		return
	}
	web.SetBackgroundColour(0, 0, 0, 0) // see-through; the page draws the card
	if s, err := web.GetSettings(); err == nil {
		s.PutAreDefaultContextMenusEnabled(false)
		s.PutAreDevToolsEnabled(false)
		s.PutIsStatusBarEnabled(false)
		s.PutIsZoomControlEnabled(false)
	}
	web.Resize()
	p.web = web
	web.WebResourceRequestedCallback = p.serve
	web.AddWebResourceRequestedFilter(trayOrigin+"/*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL)
	web.Navigate(trayOrigin + "/menu")
	ready <- true

	var m winMsg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		if m.hwnd == 0 && m.message == wmRun {
			p.mu.Lock()
			calls := p.calls
			p.calls = nil
			p.mu.Unlock()
			for _, f := range calls {
				f()
			}
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// trayOrigin is the made-up address the menu page and its fonts come from.
const trayOrigin = "http://nextbox.tray"

// serve answers the page's requests: the page itself, and files of the UI.
func (p *trayPopup) serve(req *edge.ICoreWebView2WebResourceRequest, args *edge.ICoreWebView2WebResourceRequestedEventArgs) {
	uri, err := req.GetUri()
	if err != nil {
		return
	}
	name := strings.TrimPrefix(strings.SplitN(strings.TrimPrefix(uri, trayOrigin), "?", 2)[0], "/")
	var body []byte
	typ := mime.TypeByExtension(path.Ext(name))
	status, reason := 200, "OK"
	if name == "menu" {
		body, typ = []byte(strings.Replace(trayMenuHTML, "/*FONTS*/", p.fontCSS(), 1)), "text/html; charset=utf-8"
	} else if p.files != nil {
		if body, err = fs.ReadFile(p.files, name); err != nil {
			status, reason = 404, "Not Found"
		}
	}
	if typ == "" {
		typ = "application/octet-stream"
	}
	resp, err := webResponse(p.web.Environment(), body, status, reason, "Content-Type: "+typ)
	if err != nil {
		p.logf(err.Error())
		return
	}
	defer resp.Release()
	args.PutResponse(resp)
}

var (
	shlwapi               = windows.NewLazySystemDLL("shlwapi.dll")
	procSHCreateMemStream = shlwapi.NewProc("SHCreateMemStream")
)

// webResponse makes a response of WebView2. go-webview2's own
// CreateWebResourceResponse (v1.0.22) always sends an empty body: it declares
// the content stream again inside an if, so the stream it passes is nil.
// This calls ICoreWebView2Environment::CreateWebResourceResponse (the fifth
// method of its vtable: IUnknown's three, CreateCoreWebView2Controller, then
// this one) with a stream of its own.
func webResponse(env *edge.ICoreWebView2Environment, body []byte, status int, reason, headers string) (*edge.ICoreWebView2WebResourceResponse, error) {
	var stream uintptr
	if len(body) > 0 {
		stream, _, _ = procSHCreateMemStream.Call(uintptr(unsafe.Pointer(&body[0])), uintptr(len(body)))
		if stream == 0 {
			return nil, errors.New("SHCreateMemStream failed")
		}
		// the response holds its own reference
		defer syscall.SyscallN(comMethod(*(*unsafe.Pointer)(unsafe.Pointer(&stream)), 2), stream) // IStream::Release
	}
	r, _ := windows.UTF16PtrFromString(reason)
	h, _ := windows.UTF16PtrFromString(headers)
	var resp *edge.ICoreWebView2WebResourceResponse
	hr, _, _ := syscall.SyscallN(comMethod(unsafe.Pointer(env), 4), uintptr(unsafe.Pointer(env)),
		stream, uintptr(status), uintptr(unsafe.Pointer(r)), uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&resp)))
	if int32(hr) < 0 || resp == nil {
		return nil, fmt.Errorf("CreateWebResourceResponse: %#x", uint32(hr))
	}
	return resp, nil
}

// comMethod is the i-th method of a COM object's vtable.
func comMethod(obj unsafe.Pointer, i int) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	return *(*uintptr)(unsafe.Add(vtbl, i*int(unsafe.Sizeof(uintptr(0)))))
}

var fontFaceRe = regexp.MustCompile(`@font-face{[^}]*}`)

// fontCSS is the @font-face rules of the UI's stylesheet (Noto Sans SC) and
// the flag font, so that the menu looks like the window.
func (p *trayPopup) fontCSS() string {
	if p.files == nil {
		return ""
	}
	var css strings.Builder
	list, _ := fs.ReadDir(p.files, "assets")
	for _, e := range list {
		switch {
		case strings.HasPrefix(e.Name(), "index-") && strings.HasSuffix(e.Name(), ".css"):
			b, _ := fs.ReadFile(p.files, "assets/"+e.Name())
			for _, rule := range fontFaceRe.FindAll(b, -1) {
				css.Write(rule)
				css.WriteByte('\n')
			}
		case strings.HasPrefix(e.Name(), "TwemojiCountryFlags") && strings.HasSuffix(e.Name(), ".woff2"):
			css.WriteString("@font-face{font-family:'Twemoji Country Flags';unicode-range:U+1F1E6-1F1FF,U+1F3F4,U+E0062-E0063,U+E0065,U+E0067,U+E006C,U+E006E,U+E0073-E0074,U+E0077,U+E007F;src:url(/assets/" + e.Name() + ") format('woff2');font-display:block}\n")
		}
	}
	return css.String()
}

// received is a message from the page: its height, or an action.
func (p *trayPopup) received(msg string) {
	var m struct {
		A string `json:"a"`
		H int    `json:"h"`
	}
	if json.Unmarshal([]byte(msg), &m) != nil {
		return
	}
	switch m.A {
	case "ready": // the page has drawn the menu: show it
		if p.pending {
			p.place()
		}
	case "hide":
		p.hideNow()
	case "hidden": // the page is blank now
		p.hideWindow(p.hiding)
	default:
		if p.onMessage != nil {
			go p.onMessage(msg)
		}
	}
}

// do runs f on the window's thread.
func (p *trayPopup) do(f func()) {
	p.mu.Lock()
	p.calls = append(p.calls, f)
	p.mu.Unlock()
	procPostThreadMessage.Call(uintptr(p.tid), wmRun, 0, 0)
}

// Show renders the state (JSON for nb.render) and shows the window at the
// mouse once the page has told its height.
func (p *trayPopup) Show(state string) error {
	if !p.start() {
		return errors.New("tray popup unavailable")
	}
	var at point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&at)))
	p.do(func() {
		// a second click on the icon closes the menu it opened
		if p.shown || time.Since(p.hidAt) < 200*time.Millisecond {
			p.hideNow()
			return
		}
		p.anchor, p.pending = at, true
		p.edge = edgeAt(at)
		if !p.loaded {
			p.state = state // rendered once the page is there
		} else {
			p.web.Eval("window.nb && nb.render(" + state + ", true, '" + p.edge + "')")
		}
		go p.watchShow()
	})
	return nil
}

// watchShow logs when the menu was asked for but did not appear, and loads
// the page again for the next click.
func (p *trayPopup) watchShow() {
	time.Sleep(3 * time.Second)
	p.do(func() {
		if p.pending && !p.shown {
			p.pending = false
			p.logf("the menu page did not answer; reloading it")
			p.loaded = false
			p.web.Navigate(trayOrigin + "/menu")
		}
	})
}

// Update renders a new state into the open menu.
func (p *trayPopup) Update(state string) {
	if p.failed || p.web == nil {
		return
	}
	p.do(func() {
		if p.shown {
			p.web.Eval("window.nb && nb.render(" + state + ", false)")
		}
	})
}

// Hide hides the window.
func (p *trayPopup) Hide() {
	if p.web != nil {
		p.do(p.hideNow)
	}
}

func (p *trayPopup) hideNow() {
	if !p.shown {
		return
	}
	p.shown, p.pending = false, false
	p.open.Store(false)
	p.hidAt = time.Now()
	// The page blanks itself first and says so; the window goes then (or
	// after a moment, should the page not answer). Hidden at once, the window
	// would show the menu as it was for an instant when it opens next.
	p.hiding++
	n := p.hiding
	p.web.Eval("window.nb && nb.hidden()")
	time.AfterFunc(300*time.Millisecond, func() { p.do(func() { p.hideWindow(n) }) })
}

// hideWindow hides the window for hide n, unless the menu was opened again
// since.
func (p *trayPopup) hideWindow(n int) {
	if n == p.hiding && !p.shown && !p.pending {
		procShowWindow.Call(p.hwnd, 0) // SW_HIDE
	}
}

// edgeAt is the edge of the window the menu card keeps to (and grows away
// from), as with Windows' own menus: in the lower half of the screen the
// card ends at the mouse and grows upwards (the taskbar at the bottom, the
// hidden icons above it), in the upper half it starts at the mouse.
func edgeAt(at point) string {
	if work := workArea(at); at.Y > (work.Top+work.Bottom)/2 {
		return "bottom"
	}
	return "top"
}

// place puts the window next to where the menu was asked for and shows it.
//
// The window has one size, whatever the menu shows: a window that changes
// size while it is shown flashes (Windows shows the window's empty surface,
// and the WebView's last frame at its old place, until the WebView has drawn
// again). So the window stays as it is, see-through, and the page moves and
// sizes the card in it.
func (p *trayPopup) place() {
	dpi, _, _ := procGetDpiForWindow.Call(p.hwnd)
	if dpi == 0 {
		dpi = 96
	}
	px := func(css int) int32 { return int32(css * int(dpi) / 96) }
	work := workArea(p.anchor)
	w := px(popupWidth + 2*popupMargin)
	m := px(popupMargin)
	// the card to the right of the mouse, like Windows' own menus; to the
	// left only as far as the screen edge needs
	x := p.anchor.X - m
	x = max(work.Left, min(x, work.Right-w))
	// the card's edge at the mouse (on the taskbar itself: at the taskbar),
	// the window as high as fits on that side of it
	var y, h int32
	if p.edge == "bottom" {
		bottom := min(work.Bottom, p.anchor.Y+m)
		h = min(px(popupHeight), bottom-work.Top)
		y = bottom - h
	} else {
		top := max(work.Top, p.anchor.Y-m)
		h = min(px(popupHeight), work.Bottom-top)
		y = top
	}
	const swpShow = 0x0040
	flags := uintptr(0)
	if !p.shown {
		flags = swpShow
	}
	procSetWindowPos.Call(p.hwnd, ^uintptr(0) /* HWND_TOPMOST */, uintptr(x), uintptr(y), uintptr(w), uintptr(h), flags)
	// The WebView was made while its window was hidden: without these it
	// keeps its old bounds and does not paint (an empty, see-through window).
	p.web.Resize()
	p.web.Show()
	p.web.NotifyParentWindowPositionChanged()
	if !p.shown {
		p.shown, p.pending = true, false
		p.open.Store(true)
		procSetForegroundWindow.Call(p.hwnd)
		p.web.Focus()
		p.web.Eval("window.nb && nb.opened()")
	}
}

// workArea is the work area (the screen without the taskbar) at pt.
func workArea(pt point) rect {
	mon, _, _ := procMonitorFromPoint.Call(uintptr(*(*uint64)(unsafe.Pointer(&pt))), 2 /* MONITOR_DEFAULTTONEAREST */)
	info := struct {
		size          uint32
		monitor, work rect
		flags         uint32
	}{}
	info.size = uint32(unsafe.Sizeof(info))
	procGetMonitorInfo.Call(mon, uintptr(unsafe.Pointer(&info)))
	return info.work
}
