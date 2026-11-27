package sys

import "testing"

func TestParseCombo(t *testing.T) {
	for in, want := range map[string]Combo{
		"Ctrl+Alt+P": {Mods: 3, Key: 'P'}, "Shift+Win+F5": {Mods: 12, Key: 0x74}, "F9": {Key: 0x78},
		"Ctrl+Num3": {Mods: 2, Key: 0x63}, "Alt+Space": {Mods: 1, Key: 0x20},
	} {
		if c, err := ParseCombo(in); err != nil || c != want {
			t.Errorf("%s: %+v, %v", in, c, err)
		}
	}
	for _, bad := range []string{"", "P", "Ctrl+", "Hyper+P", "Ctrl+Foo"} {
		if _, err := ParseCombo(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
