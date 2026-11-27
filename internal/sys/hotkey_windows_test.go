//go:build windows

package sys

import (
	"os"
	"testing"
	"time"
)

var procKeybdEvent = user32.NewProc("keybd_event")

// A registered combination calls back when it is pressed (the test presses
// Ctrl+Alt+F12 itself, so it only runs with NEXTBOX_UI_TEST=1).
func TestHotkeyPressed(t *testing.T) {
	if os.Getenv("NEXTBOX_UI_TEST") == "" {
		t.Skip("set NEXTBOX_UI_TEST=1 to press keys")
	}
	pressed := make(chan int, 4)
	h := StartHotkeys(func(id int) { pressed <- id })
	c, err := ParseCombo("Ctrl+Alt+F12")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Test(c) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("test: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Test did not return: the hotkey thread did not answer")
	}
	if errs := h.Set(map[int]Combo{1: c}); len(errs) > 0 {
		t.Fatalf("set: %v", errs)
	}
	const keyUp = 2
	for _, k := range []uintptr{0x11, 0x12, 0x7B} { // Ctrl, Alt, F12 down
		procKeybdEvent.Call(k, 0, 0, 0)
	}
	for _, k := range []uintptr{0x7B, 0x12, 0x11} {
		procKeybdEvent.Call(k, 0, keyUp, 0)
	}
	select {
	case id := <-pressed:
		if id != 1 {
			t.Errorf("id %d", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the hotkey was not reported")
	}
	h.Set(nil)
}
