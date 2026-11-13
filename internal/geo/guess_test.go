package geo

import (
	"os"
	"testing"
)

func TestGuess(t *testing.T) {
	data, err := os.ReadFile("../../frontend/src/places.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := LoadPlaces(data); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"🇯🇵 东京 01":            "JP",
		"香港 IPLC 02":          "HK",
		"Los Angeles 3":       "US",
		"[US] 节点":             "US",
		"UK-London":           "GB",
		"US JP 双栈":            "US",
		"印尼 01":               "ID",
		"Premium relay":       "",
		"HKBN":                "", // codes count only as whole words
		"node-SG":             "SG",
		"Singapore · Netflix": "SG",
	}
	for name, want := range cases {
		if got := Guess(name); got != want {
			t.Errorf("Guess(%q) = %q, want %q", name, got, want)
		}
	}
}
