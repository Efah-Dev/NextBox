package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const dictDir = "../../frontend/src/i18n"

func load(t *testing.T) map[string]string {
	t.Helper()
	en, err := os.ReadFile(filepath.Join(dictDir, "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	tw, err := os.ReadFile(filepath.Join(dictDir, "zh-TW.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Init(en, tw); err != nil {
		t.Fatal(err)
	}
	var dict map[string]string
	json.Unmarshal(en, &dict)
	return dict
}

var (
	jsKey   = regexp.MustCompile(`\bt\('((?:[^'\\]|\\.)*)'`)
	goKey   = regexp.MustCompile(`i18n\.T\("((?:[^"\\]|\\.)*)"\)`)
	cjk     = regexp.MustCompile(`\p{Han}`)
	holders = regexp.MustCompile(`\{\d+\}|%[a-z]`)
)

// usedKeys collects every source string passed to t() / i18n.T().
func usedKeys(t *testing.T) map[string]string {
	keys := map[string]string{}
	walk := func(root string, exts ...string) {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "dist" || d.Name() == "wailsjs") {
				return filepath.SkipDir
			}
			if d.IsDir() || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			for _, ext := range exts {
				if strings.HasSuffix(path, ext) {
					b, _ := os.ReadFile(path)
					re := jsKey
					if ext == ".go" {
						re = goKey
					}
					for _, m := range re.FindAllStringSubmatch(string(b), -1) {
						key := m[1]
						if ext == ".go" {
							key = strings.ReplaceAll(key, `\n`, "\n")
						}
						keys[key] = path
					}
				}
			}
			return nil
		})
	}
	walk("../../frontend/src", ".vue", ".ts")
	walk("../..", ".go")
	return keys
}

func placeholders(s string) []string {
	h := holders.FindAllString(s, -1)
	sort.Strings(h)
	return h
}

func TestEveryKeyHasEnglish(t *testing.T) {
	dict := load(t)
	keys := usedKeys(t)
	if len(keys) < 300 { // guards against the scan silently finding nothing
		t.Fatalf("only %d keys found", len(keys))
	}
	t.Logf("%d keys checked", len(keys))
	for key, file := range keys {
		if !cjk.MatchString(key) {
			continue
		}
		en, ok := dict[key]
		if !ok {
			t.Errorf("missing English for %q (%s)", key, file)
			continue
		}
		if cjk.MatchString(en) {
			t.Errorf("English for %q still contains Chinese: %q", key, en)
		}
		if a, b := placeholders(key), placeholders(en); strings.Join(a, ",") != strings.Join(b, ",") {
			t.Errorf("placeholders differ for %q: %v vs %v", key, a, b)
		}
	}
}

func TestTraditional(t *testing.T) {
	load(t)
	for in, want := range map[string]string{
		"设置":          "設定",
		"日志文件夹":       "日誌資料夾",
		"允许局域网连接":     "允許區域網路連線",
		"服务器":         "伺服器",
		"关闭时隐藏到托盘":    "關閉時隱藏到系統匣",
		"已删除 {0} 个节点": "已刪除 {0} 個節點",
	} {
		if got := Traditional(in); got != want {
			t.Errorf("Traditional(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLanguageSwitch(t *testing.T) {
	load(t)
	defer SetLang("zh-CN")
	SetLang("en")
	if got := T("设置"); got != "Settings" {
		t.Errorf("en: %q", got)
	}
	SetLang("zh-TW")
	if got := T("设置"); got != "設定" {
		t.Errorf("zh-TW: %q", got)
	}
	SetLang("fr")
	if got := T("设置"); got != "设置" {
		t.Errorf("fallback: %q", got)
	}
}
