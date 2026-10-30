// Package ruleset ships the rule-sets used by rule-based routing inside the
// executable, so bypassing China domains / IPs works without any download.
//
// data/*.srs are copied unmodified from the rule-set branches of
// SagerNet/sing-geosite and SagerNet/sing-geoip.
package ruleset

import (
	"embed"
	"os"
)

//go:embed data/*.srs
var bundled embed.FS

// Has reports whether name (e.g. "geosite-cn") is bundled.
func Has(name string) bool {
	_, err := bundled.Open("data/" + name + ".srs")
	return err == nil
}

// Install writes the bundled copy of name to dest.
func Install(name, dest string) error {
	b, err := bundled.ReadFile("data/" + name + ".srs")
	if err != nil {
		return err
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
