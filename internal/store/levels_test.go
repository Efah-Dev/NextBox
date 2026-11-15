package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLevelsMigrateOnce(t *testing.T) {
	dir := t.TempDir()
	// a v2.0 settings file: classic light theme, a softer dark theme
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"lightLevel":0,"darkLevel":29}`), 0o600)
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Settings()
	if got.LightLevel != DefaultLightLevel || got.DarkLevel != 40 || got.ThemeScale != ThemeScale {
		t.Fatalf("levels = %d / %d (scale %d)", got.LightLevel, got.DarkLevel, got.ThemeScale)
	}
	// saved and opened again: unchanged
	if err := s.SaveSettings(got); err != nil {
		t.Fatal(err)
	}
	s2, _ := Open(dir)
	if g := s2.Settings(); g.LightLevel != got.LightLevel || g.DarkLevel != got.DarkLevel {
		t.Fatalf("converted twice: %d / %d", g.LightLevel, g.DarkLevel)
	}
	// a new installation starts at the defaults
	s3, _ := Open(t.TempDir())
	if g := s3.Settings(); g.LightLevel != DefaultLightLevel || g.DarkLevel != DefaultDarkLevel {
		t.Fatalf("defaults: %d / %d", g.LightLevel, g.DarkLevel)
	}
	// the old darkest and dimmest ends
	old := Settings{LightLevel: 100, DarkLevel: 100}
	old.fillDefaults()
	if old.LightLevel != 72 || old.DarkLevel != 100 {
		t.Fatalf("ends: %d / %d", old.LightLevel, old.DarkLevel)
	}
}
