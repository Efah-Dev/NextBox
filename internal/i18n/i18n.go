// Package i18n translates backend messages (errors, logs, tray menu).
//
// Source strings are Simplified Chinese and double as keys. The dictionaries
// live in frontend/src/i18n and are shared with the UI:
//
//	en.json     Simplified Chinese -> English
//	zh-TW.json  Taiwan vocabulary phrases + simplified -> traditional characters
//
// Format strings keep their fmt verbs, so callers translate the format first:
// fmt.Sprintf(i18n.T("已导入 %d 个节点"), n).
package i18n

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

var (
	mu      sync.RWMutex
	lang    = "zh-CN"
	en      = map[string]string{}
	phrases [][2]string // longest first
	chars   = map[rune]rune{}
	twCache sync.Map
)

// Init loads the dictionaries (the contents of en.json and zh-TW.json).
func Init(enJSON, twJSON []byte) error {
	var dict map[string]string
	if err := json.Unmarshal(enJSON, &dict); err != nil {
		return err
	}
	var tw struct {
		Phrases [][2]string `json:"phrases"`
		Chars   struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"chars"`
	}
	if err := json.Unmarshal(twJSON, &tw); err != nil {
		return err
	}
	sort.SliceStable(tw.Phrases, func(i, j int) bool {
		return len([]rune(tw.Phrases[i][0])) > len([]rune(tw.Phrases[j][0]))
	})
	from, to := []rune(tw.Chars.From), []rune(tw.Chars.To)
	mu.Lock()
	defer mu.Unlock()
	en = dict
	phrases = tw.Phrases
	for i := range from {
		if i < len(to) {
			chars[from[i]] = to[i]
		}
	}
	return nil
}

// SetLang selects zh-CN, zh-TW or en; anything else falls back to zh-CN.
func SetLang(l string) {
	if l != "zh-TW" && l != "en" {
		l = "zh-CN"
	}
	mu.Lock()
	lang = l
	mu.Unlock()
}

func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return lang
}

// T translates a Simplified Chinese source string into the current language.
func T(s string) string {
	mu.RLock()
	l := lang
	tr, ok := en[s]
	mu.RUnlock()
	switch l {
	case "en":
		if ok {
			return tr
		}
		return s
	case "zh-TW":
		return Traditional(s)
	}
	return s
}

// Traditional converts Simplified Chinese to Traditional Chinese (Taiwan).
func Traditional(s string) string {
	if v, ok := twCache.Load(s); ok {
		return v.(string)
	}
	mu.RLock()
	defer mu.RUnlock()
	var b strings.Builder
	rest := s
outer:
	for rest != "" {
		for _, p := range phrases {
			if strings.HasPrefix(rest, p[0]) {
				b.WriteString(p[1])
				rest = rest[len(p[0]):]
				continue outer
			}
		}
		r := []rune(rest[:runeLen(rest)])[0]
		if t, ok := chars[r]; ok {
			b.WriteRune(t)
		} else {
			b.WriteRune(r)
		}
		rest = rest[runeLen(rest):]
	}
	out := b.String()
	twCache.Store(s, out)
	return out
}

func runeLen(s string) int {
	for i := range s {
		if i > 0 {
			return i
		}
	}
	return len(s)
}
