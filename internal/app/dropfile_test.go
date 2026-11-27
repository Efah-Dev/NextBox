package app

import (
	"os"
	"path/filepath"
	"testing"
)

// A dropped file is recognised by its content.
func TestInspectFile(t *testing.T) {
	a := &App{}
	dir := t.TempDir()
	for name, c := range map[string]struct{ content, kind string }{
		"backup.json":   {`{"app":"NextBox","version":2,"platform":"android","settings":{}}`, "backup"},
		"profiles.json": {`{"nextId":3,"groups":[],"profiles":[]}`, "backup"},
		"clash.yaml":    {string(rune(0xFEFF)) + "port: 7890\nproxies:\n  - {name: a, type: trojan, server: a.example, port: 443, password: p}\n", "config"},
		"sb.json":       {`{"outbounds":[{"type":"trojan","tag":"x","server":"a.example","server_port":443,"password":"p"},{"type":"direct","tag":"d"}]}`, "config"},
		"sub.txt":       {"https://sub.example.com/s?token=1\n", "subscription"},
		"links.txt":     {"trojan://p@a.example:443#a\ntrojan://p@b.example:443#b\n", "nodes"},
	} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(c.content), 0o644)
		f, err := a.InspectFile(p)
		if err != nil || f.Kind != c.kind {
			t.Errorf("%s: %+v, %v", name, f, err)
		}
	}
	p := filepath.Join(dir, "notes.txt")
	os.WriteFile(p, []byte("hello"), 0o644)
	if _, err := a.InspectFile(p); err == nil {
		t.Error("a text without links was accepted")
	}
}
