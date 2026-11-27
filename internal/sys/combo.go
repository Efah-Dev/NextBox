package sys

import (
	"errors"
	"strings"

	"nextbox/internal/i18n"
)

// Combo is a key combination: modifier flags (MOD_ALT 1, MOD_CONTROL 2,
// MOD_SHIFT 4, MOD_WIN 8) and a Windows virtual-key code.
type Combo struct {
	Mods uint32
	Key  uint32
}

var modNames = []struct {
	name string
	flag uint32
}{{"Ctrl", 2}, {"Alt", 1}, {"Shift", 4}, {"Win", 8}}

// keyCodes maps the key names the UI writes (KeyboardEvent.code, shortened)
// to virtual-key codes.
var keyCodes = func() map[string]uint32 {
	m := map[string]uint32{
		"Space": 0x20, "PageUp": 0x21, "PageDown": 0x22, "End": 0x23, "Home": 0x24,
		"Left": 0x25, "Up": 0x26, "Right": 0x27, "Down": 0x28, "Insert": 0x2D, "Delete": 0x2E,
		"Backquote": 0xC0, "Minus": 0xBD, "Equal": 0xBB, "BracketLeft": 0xDB, "BracketRight": 0xDD,
		"Backslash": 0xDC, "Semicolon": 0xBA, "Quote": 0xDE, "Comma": 0xBC, "Period": 0xBE, "Slash": 0xBF,
	}
	for c := 'A'; c <= 'Z'; c++ {
		m[string(c)] = uint32(c)
	}
	for c := '0'; c <= '9'; c++ {
		m[string(c)] = uint32(c)
		m["Num"+string(c)] = 0x60 + uint32(c-'0')
	}
	for i := 1; i <= 24; i++ {
		m["F"+itoa(i)] = 0x6F + uint32(i)
	}
	return m
}()

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// ParseCombo reads "Ctrl+Alt+P". A combination needs a modifier, except the
// function keys F1–F24.
func ParseCombo(s string) (Combo, error) {
	var c Combo
	parts := strings.Split(strings.TrimSpace(s), "+")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if i < len(parts)-1 {
			found := false
			for _, m := range modNames {
				if strings.EqualFold(p, m.name) {
					c.Mods |= m.flag
					found = true
				}
			}
			if !found {
				return c, errors.New(i18n.T("无法识别的组合键"))
			}
			continue
		}
		k, ok := keyCodes[p]
		if !ok {
			return c, errors.New(i18n.T("无法识别的组合键"))
		}
		c.Key = k
	}
	if c.Key == 0 || (c.Mods == 0 && (c.Key < 0x70 || c.Key > 0x87)) {
		return c, errors.New(i18n.T("组合键至少要有 Ctrl、Alt、Shift 或 Win 中的一个"))
	}
	return c, nil
}
