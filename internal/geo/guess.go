package geo

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

// The words that point to a country in node names come from the UI
// (frontend/src/places.json): [code, Chinese words, Latin words], checked in
// order. main.go hands them over with LoadPlaces.
type place struct {
	code  string
	zh    []string
	latin *regexp.Regexp
}

var (
	placesMu sync.RWMutex
	places   []place
	codes    = map[string]bool{}
)

var codeAliases = map[string]string{"UK": "GB"}

// LoadPlaces reads the places table (see above).
func LoadPlaces(data []byte) error {
	var raw [][]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	list := make([]place, 0, len(raw))
	set := map[string]bool{}
	for _, r := range raw {
		if len(r) != 3 {
			continue
		}
		var p place
		var latin []string
		if json.Unmarshal(r[0], &p.code) != nil || json.Unmarshal(r[1], &p.zh) != nil || json.Unmarshal(r[2], &latin) != nil {
			continue
		}
		quoted := make([]string, len(latin))
		for i, w := range latin {
			quoted[i] = regexp.QuoteMeta(w)
		}
		if len(quoted) > 0 {
			p.latin = regexp.MustCompile(`(^|[^a-z])(` + strings.Join(quoted, "|") + `)($|[^a-z])`)
		}
		list = append(list, p)
		set[p.code] = true
	}
	placesMu.Lock()
	places, codes = list, set
	placesMu.Unlock()
	return nil
}

var bareCode = regexp.MustCompile(`(?:^|[^A-Za-z])([A-Z]{2})(?:$|[^A-Za-z])`)

// Guess reads the country from a node name like the UI does: a flag emoji,
// a place name, or an upper case country code. "" when nothing points to one.
func Guess(name string) string {
	if code := flagCode(name); code != "" {
		return code
	}
	placesMu.RLock()
	defer placesMu.RUnlock()
	for _, p := range places {
		for _, w := range p.zh {
			if strings.Contains(name, w) {
				return p.code
			}
		}
	}
	lower := strings.ToLower(name)
	for _, p := range places {
		if p.latin != nil && p.latin.MatchString(lower) {
			return p.code
		}
	}
	// the expression consumes the character after a code, so walk manually
	for i := 0; i < len(name); {
		m := bareCode.FindStringSubmatchIndex(name[i:])
		if m == nil {
			break
		}
		code := name[i+m[2] : i+m[3]]
		if a, ok := codeAliases[code]; ok {
			code = a
		}
		if codes[code] {
			return code
		}
		i += m[3]
	}
	return ""
}

// flagCode decodes the first flag emoji (a pair of regional indicators).
func flagCode(name string) string {
	var prev rune
	for _, r := range name {
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			if prev != 0 {
				return string([]rune{prev - 0x1F1E6 + 'A', r - 0x1F1E6 + 'A'})
			}
			prev = r
			continue
		}
		prev = 0
	}
	return ""
}
