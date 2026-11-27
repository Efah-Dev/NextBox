package sys

// System-wide hotkeys: RegisterHotKey on a thread of their own, whose message
// loop receives WM_HOTKEY. Registrations belong to that thread, so changes
// are handed to it through a channel and a wake-up message.

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"

	"nextbox/internal/i18n"
)

var (
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procPostThreadMessage = user32.NewProc("PostThreadMessageW")
	procPeekMessage       = user32.NewProc("PeekMessageW")
)

const (
	wmHotkey = 0x0312
	wmWake   = 0x8000 + 0x4E42 // WM_APP + a NextBox number
	testID   = 0xBFFE          // a registration that is only tried
)

// ErrHotkeyTaken: another program (or Windows) already uses the combination.
var ErrHotkeyTaken = errors.New("hotkey taken")

type hotkeyReq struct {
	set   map[int]Combo // replace all registrations with these
	test  *Combo        // only try this one
	reply chan map[int]error
}

// Hotkeys owns the registered combinations of this process.
type Hotkeys struct {
	tid  uint32
	reqs chan hotkeyReq
}

// StartHotkeys starts the hotkey thread; on is called (on that thread) with
// the ID of a combination that was pressed.
func StartHotkeys(on func(id int)) *Hotkeys {
	h := &Hotkeys{reqs: make(chan hotkeyReq, 4)}
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		h.tid = windows.GetCurrentThreadId()
		var m msg
		// creates the thread's message queue, so that it can be woken up
		procPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 0)
		close(ready)
		registered := map[int]bool{}
		for {
			r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			switch m.message {
			case wmHotkey:
				if id := int(m.wParam); registered[id] {
					on(id)
				}
			case wmWake:
				for {
					select {
					case req := <-h.reqs:
						req.reply <- h.handle(req, registered)
						continue
					default:
					}
					break
				}
			}
		}
	}()
	<-ready
	return h
}

func (h *Hotkeys) handle(req hotkeyReq, registered map[int]bool) map[int]error {
	errs := map[int]error{}
	if req.test != nil {
		if err := register(testID, *req.test); err != nil {
			errs[testID] = err
		} else {
			procUnregisterHotKey.Call(0, testID)
		}
		return errs
	}
	for id := range registered {
		procUnregisterHotKey.Call(0, uintptr(id))
		delete(registered, id)
	}
	for id, c := range req.set {
		if err := register(id, c); err != nil {
			errs[id] = err
			continue
		}
		registered[id] = true
	}
	return errs
}

func register(id int, c Combo) error {
	const modNoRepeat = 0x4000
	ok, _, _ := procRegisterHotKey.Call(0, uintptr(id), uintptr(c.Mods|modNoRepeat), uintptr(c.Key))
	if ok == 0 {
		return ErrHotkeyTaken
	}
	return nil
}

func (h *Hotkeys) ask(req hotkeyReq) map[int]error {
	req.reply = make(chan map[int]error, 1)
	h.reqs <- req
	procPostThreadMessage.Call(uintptr(h.tid), wmWake, 0, 0)
	return <-req.reply
}

// Set replaces the registered combinations (by ID; IDs must be 1–0xBFFD).
// It returns the ones that could not be registered.
func (h *Hotkeys) Set(list map[int]Combo) map[int]error {
	return h.ask(hotkeyReq{set: list})
}

// Test reports whether c is free: ErrHotkeyTaken when another program (or
// one of NextBox's own registrations) has it.
func (h *Hotkeys) Test(c Combo) error {
	if err := h.ask(hotkeyReq{test: &c})[testID]; err != nil {
		return errors.New(i18n.T("这个组合键已被其他程序占用"))
	}
	return nil
}
