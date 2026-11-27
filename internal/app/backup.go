package app

// Backups (导入 / 导出配置) and encryption of the node list on disk.
//
// A backup is the document NextBox for Android makes (see its nbcore.go
// Backup): {"app":"NextBox","version":…,"profiles":<profiles.json>,
// "settings":<settings.json>}, usually sealed with a password in the
// envelope of internal/vault. Version 2 adds "platform", so that settings
// that only make sense on one system (ports, TUN, autostart) stay there.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"nextbox/internal/i18n"
	"nextbox/internal/store"
	"nextbox/internal/vault"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const backupPlatform = "windows"

type backupDoc struct {
	App      string          `json:"app"`
	Version  int             `json:"version"`
	Platform string          `json:"platform,omitempty"`
	Profiles json.RawMessage `json:"profiles,omitempty"`
	Settings json.RawMessage `json:"settings,omitempty"`
	// Android holds preferences of NextBox for Android (the apps that use
	// the VPN and the like). They mean nothing here but are kept, so that a
	// backup that went through this computer still has them.
	Android json.RawMessage `json:"android,omitempty"`
}

// androidPrefsPath is where the Android preferences of an imported backup
// are kept until the next export.
func (a *App) androidPrefsPath() string { return filepath.Join(a.dataDir(), "android.json") }

// The look of the app (外观), a part of its own: a backup takes it along by
// default, also to and from the other system.
var appearanceSettings = []string{
	"theme", "accentColor", "language", "lightLevel", "darkLevel", "themeScale", "recentColors",
	"nodeView", "trayItems", // Windows only
}

// Other settings that mean the same on every platform: everything else
// (ports, TUN, system proxy, autostart, window behaviour, logs) stays local
// when a backup comes from another system.
var portableSettings = []string{
	"remoteDns", "directDns", "dnsOverrides", "enableIpv6", "sniff",
	"testUrl", "testTimeout", "testConcurrency", "testMode",
	"subUserAgent", "subUpdateByProxy", "subAutoUpdate", "subUpdateHours",
	"countrySource", "autoCheckUpdate", "sortAfterTest",
	"speedTestUrl", "speedTestSeconds", "speedTestMb",
}

// Settings that make up the routing rules (分流规则).
var ruleSettings = []string{
	"routeMode", "blockAds", "rules", "scenes", "ruleSetMirror", "bypassDns",
	"routeView", "routeHideOff", // how the page looks
	"directRules", "proxyRules", "blockRules", // v1.2 and Android
}

// BackupOptions says what goes into a backup or is taken from one.
type BackupOptions struct {
	Nodes      bool   `json:"nodes"`      // groups, nodes, chain proxies, policy groups
	Rules      bool   `json:"rules"`      // routing rules
	Appearance bool   `json:"appearance"` // theme, colours, language (appearanceSettings)
	Settings   bool   `json:"settings"`   // the other settings
	Password   string `json:"password"`   // "" = not encrypted (export) / not given yet (import)
	// Import only: replace the current node list instead of adding to it.
	Replace bool `json:"replace"`
}

// ExportBackup asks where to save and writes the backup. It returns the file
// path, or "" when the dialog was cancelled.
func (a *App) ExportBackup(o BackupOptions) (string, error) {
	if !o.Nodes && !o.Rules && !o.Appearance && !o.Settings {
		return "", errors.New(i18n.T("请至少选择一项内容"))
	}
	doc := backupDoc{App: "NextBox", Version: 2, Platform: backupPlatform}
	if o.Nodes {
		a.store.Flush()
		raw, err := a.store.ExportData()
		if err != nil {
			return "", err
		}
		doc.Profiles = raw
	}
	if o.Rules || o.Appearance || o.Settings {
		raw, err := json.Marshal(a.store.Settings())
		if err != nil {
			return "", err
		}
		if raw, err = keepParts(raw, o); err != nil {
			return "", err
		}
		doc.Settings = raw
	}
	if o.Appearance || o.Settings {
		if b, err := os.ReadFile(a.androidPrefsPath()); err == nil && json.Valid(b) {
			doc.Android = b
		}
	}
	plain, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	out := plain
	if o.Password != "" {
		if out, err = vault.Encrypt(plain, o.Password); err != nil {
			return "", err
		}
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           i18n.T("导出配置"),
		DefaultFilename: "NextBox-" + i18n.T("备份") + "-" + time.Now().Format("2006-01-02") + ".json",
		Filters:         []wruntime.FileFilter{{DisplayName: "JSON (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".json") {
		path += ".json"
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return "", err
	}
	a.log(i18n.T("已导出配置: ") + path)
	return path, nil
}

// ChooseBackup asks for a backup file; "" when cancelled.
func (a *App) ChooseBackup() (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: i18n.T("导入配置"),
		// a backup sent through a chat app often loses its extension
		Filters: []wruntime.FileFilter{
			{DisplayName: "JSON (*.json)", Pattern: "*.json"},
			{DisplayName: i18n.T("所有文件 (*.*)"), Pattern: "*.*"},
		},
	})
}

// BackupInfo describes a backup file before it is imported.
type BackupInfo struct {
	Encrypted    bool   `json:"encrypted"`
	NeedPassword bool   `json:"needPassword"` // encrypted and no password given yet
	Platform     string `json:"platform"`     // "windows", "android" or "" (unknown)
	Version      int    `json:"version"`
	HasNodes     bool   `json:"hasNodes"`
	HasRules     bool   `json:"hasRules"`
	HasSettings  bool   `json:"hasSettings"`
	// HasAppearance: the backup has the look of the app (backups made before
	// it was a part of its own have it with the other settings).
	HasAppearance bool              `json:"hasAppearance"`
	Summary       store.DataSummary `json:"summary"`
	Rules         int               `json:"rules"`
	// AndroidRules counts the rules by app, which only NextBox for Android
	// can use; they are kept anyway.
	AndroidRules int `json:"androidRules"`
}

// readBackup opens a backup (or a plain profiles.json).
func readBackup(path, password string) (backupDoc, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return backupDoc{}, false, err
	}
	encrypted := false
	if env, ok := vault.Parse(b); ok {
		encrypted = true
		if env.KDF == vault.KDFWindows {
			return backupDoc{}, true, errors.New(i18n.T("这是本机加密保存的 profiles.json，不是备份文件：请在 NextBox 里用「导出配置」生成备份"))
		}
		if password == "" {
			return backupDoc{}, true, nil
		}
		plain, _, err := vault.Decrypt(env, password)
		if errors.Is(err, vault.ErrPassword) {
			return backupDoc{}, true, errors.New(i18n.T("密码错误"))
		}
		if err != nil {
			return backupDoc{}, true, err
		}
		b = plain
	}
	var doc backupDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return doc, encrypted, errors.New(i18n.T("不是有效的备份文件"))
	}
	if len(doc.Profiles) == 0 && len(doc.Settings) == 0 {
		// a bare profiles.json
		var probe struct {
			Profiles []json.RawMessage `json:"profiles"`
		}
		if json.Unmarshal(b, &probe) != nil || probe.Profiles == nil {
			return doc, encrypted, errors.New(i18n.T("不是有效的备份文件"))
		}
		doc = backupDoc{App: "NextBox", Profiles: b}
	}
	if doc.Platform == "" && doc.Version == 1 {
		doc.Platform = "android" // only the Android version wrote version 1
	}
	return doc, encrypted, nil
}

func (a *App) InspectBackup(path, password string) (BackupInfo, error) {
	doc, encrypted, err := readBackup(path, password)
	info := BackupInfo{Encrypted: encrypted}
	if err != nil {
		return info, err
	}
	if encrypted && password == "" {
		info.NeedPassword = true
		return info, nil
	}
	info.Platform, info.Version = doc.Platform, doc.Version
	if len(doc.Profiles) > 0 {
		sum, err := store.SummarizeData(doc.Profiles)
		if err != nil {
			return info, errors.New(i18n.T("备份里的节点数据已损坏"))
		}
		info.HasNodes, info.Summary = true, sum
	}
	if len(doc.Settings) > 0 {
		var keys map[string]json.RawMessage
		json.Unmarshal(doc.Settings, &keys)
		for k := range keys {
			switch settingsPart(k) {
			case "rules":
				info.HasRules = true
			case "appearance":
				info.HasAppearance = true
			default:
				info.HasSettings = true
			}
		}
		s := store.DefaultSettings()
		s.Rules = nil
		if json.Unmarshal(doc.Settings, &s) == nil {
			s.Normalize()
			for _, r := range s.Rules {
				switch {
				case r.IsStep():
				case r.Kind == "package":
					info.AndroidRules++
					info.Rules++
				default:
					info.Rules++
				}
			}
		}
	}
	return info, nil
}

func (a *App) ImportBackup(path string, o BackupOptions) (string, error) {
	doc, encrypted, err := readBackup(path, o.Password)
	if err != nil {
		return "", err
	}
	if encrypted && o.Password == "" {
		return "", errors.New(i18n.T("请输入备份密码"))
	}
	var done []string
	policyMap := map[int64]int64{}
	if o.Nodes && len(doc.Profiles) > 0 {
		m, err := a.store.ImportData(doc.Profiles, o.Replace)
		if err != nil {
			return "", fmt.Errorf(i18n.T("导入节点失败: %w"), err)
		}
		policyMap = m
		sum, _ := store.SummarizeData(doc.Profiles)
		done = append(done, fmt.Sprintf(i18n.T("%d 个节点"), sum.Nodes+sum.Chains))
	}
	if (o.Rules || o.Appearance || o.Settings) && len(doc.Settings) > 0 {
		keys := importKeys(doc, o)
		if (o.Appearance || o.Settings) && len(doc.Android) > 0 && json.Valid(doc.Android) {
			os.WriteFile(a.androidPrefsPath(), doc.Android, 0o600)
		}
		next, err := mergeSettings(a.store.Settings(), doc.Settings, keys, o.Rules)
		if err != nil {
			return "", err
		}
		remapPolicies(&next, policyMap)
		if o.Settings {
			legacyTestMode(&next, doc)
		}
		if err := a.SaveSettings(next); err != nil {
			return "", err
		}
		if o.Rules {
			done = append(done, i18n.T("分流规则"))
		}
		if o.Appearance {
			done = append(done, i18n.T("外观"))
		}
		if o.Settings {
			done = append(done, i18n.T("设置"))
		}
	}
	if len(done) == 0 {
		return "", errors.New(i18n.T("备份里没有选中的内容"))
	}
	a.store.Flush()
	a.emitState()
	a.emitProfiles(a.store.CurrentGroupID())
	go a.restartIfRunning()
	msg := i18n.T("已导入：") + strings.Join(done, i18n.T("、"))
	a.log(msg)
	return msg, nil
}

// mergeSettings applies the given keys of an imported settings document (all
// of them for keys == nil, except the rules unless withRules).
func mergeSettings(cur store.Settings, imported json.RawMessage, keys []string, withRules bool) (store.Settings, error) {
	var in map[string]json.RawMessage
	if err := json.Unmarshal(imported, &in); err != nil {
		return cur, errors.New(i18n.T("备份里的设置已损坏"))
	}
	b, _ := json.Marshal(cur)
	var out map[string]json.RawMessage
	json.Unmarshal(b, &out)
	for k, v := range in {
		if keys != nil && !slices.Contains(keys, k) {
			continue
		}
		if keys == nil && !withRules && slices.Contains(ruleSettings, k) {
			continue
		}
		out[k] = v
	}
	// levels of a backup made before the v2.1 scales are converted (see
	// store.Settings.ThemeScale)
	_, light := in["lightLevel"]
	_, dark := in["darkLevel"]
	if _, scale := in["themeScale"]; (light || dark) && !scale && (keys == nil || slices.Contains(keys, "lightLevel")) {
		out["themeScale"] = json.RawMessage("0")
	}
	if withRules {
		// rules from the backup replace the current ones
		if _, ok := in["rules"]; !ok {
			out["rules"] = json.RawMessage("[]")
		}
	}
	b, _ = json.Marshal(out)
	next := store.DefaultSettings()
	if err := json.Unmarshal(b, &next); err != nil {
		return cur, errors.New(i18n.T("备份里的设置已损坏"))
	}
	return next, nil
}

// legacyTestMode takes the speed test method of an Android backup made
// before it was a shared setting (the "latencyMode" of its preferences).
func legacyTestMode(s *store.Settings, doc backupDoc) {
	var in map[string]json.RawMessage
	json.Unmarshal(doc.Settings, &in)
	if _, ok := in["testMode"]; ok {
		return
	}
	var prefs struct {
		LatencyMode string `json:"latencyMode"`
	}
	if json.Unmarshal(doc.Android, &prefs) == nil && (prefs.LatencyMode == store.TestTCP || prefs.LatencyMode == store.TestURL) {
		s.TestMode = prefs.LatencyMode
	}
}

// remapPolicies points imported rules at the policy groups as they were
// imported; rules naming a group that did not come along use the node.
func remapPolicies(s *store.Settings, m map[int64]int64) {
	fix := func(t string) string {
		id := store.TargetPolicy(t)
		if id == 0 {
			return t
		}
		if n, ok := m[id]; ok {
			return store.PolicyTarget(n)
		}
		return store.TargetProxy
	}
	for i := range s.Rules {
		s.Rules[i].Target = fix(s.Rules[i].Target)
	}
	for i := range s.Scenes {
		s.Scenes[i].Target = fix(s.Scenes[i].Target)
	}
}

// ---------------------------------------------------------------- at rest

// SetEncryption changes how the node list is kept on disk: "" (plain),
// "windows" (bound to this Windows account) or "password".
func (a *App) SetEncryption(mode, password string) error {
	if err := a.store.SetEncryption(mode, password); err != nil {
		return err
	}
	switch mode {
	case store.EncryptWindows:
		a.log(i18n.T("节点和订阅已改为加密保存（跟随 Windows 账户）"))
	case store.EncryptPassword:
		a.log(i18n.T("节点和订阅已改为加密保存（启动密码）"))
	default:
		a.log(i18n.T("节点和订阅已改为明文保存"))
	}
	a.emitState()
	return nil
}

// Unlock opens a node list kept with a start-up password.
func (a *App) Unlock(password string) error {
	if err := a.store.Unlock(password); err != nil {
		return err
	}
	a.log(i18n.T("已解锁"))
	a.emitState()
	a.emitProfiles(a.store.CurrentGroupID())
	s := a.store.Settings()
	switch {
	case a.autoStarted && s.AutoStartConnect && a.store.SelectedID() == 0:
		a.recordAutoStart("noNode", "")
	case a.autoStarted && s.AutoStartConnect:
		a.recordAutoStart("started", "")
		go a.autoConnect()
	case s.ConnectOnStart && a.store.SelectedID() != 0:
		go a.autoConnect()
	}
	return nil
}

// settingsPart names the part of a backup a settings key belongs to:
// "rules", "appearance" or "settings" (the other settings).
func settingsPart(key string) string {
	switch {
	case slices.Contains(ruleSettings, key):
		return "rules"
	case slices.Contains(appearanceSettings, key):
		return "appearance"
	}
	return "settings"
}

// keepParts keeps the settings keys of the parts o asks for.
func keepParts(raw []byte, o BackupOptions) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	want := map[string]bool{"rules": o.Rules, "appearance": o.Appearance, "settings": o.Settings}
	for k := range m {
		if !want[settingsPart(k)] {
			delete(m, k)
		}
	}
	return json.Marshal(m)
}

// importKeys lists the settings keys of a backup to apply for the parts o
// asks for. From the other system only what means the same here is taken,
// and the routing mode stays the home page's choice.
func importKeys(doc backupDoc, o BackupOptions) []string {
	var in map[string]json.RawMessage
	json.Unmarshal(doc.Settings, &in)
	same := doc.Platform == backupPlatform
	keys := []string{} // not nil: mergeSettings takes nil for "everything"
	for k := range in {
		switch settingsPart(k) {
		case "rules":
			if o.Rules && (same || k != "routeMode") {
				keys = append(keys, k)
			}
		case "appearance":
			if o.Appearance {
				keys = append(keys, k)
			}
		default:
			if o.Settings && (same || slices.Contains(portableSettings, k)) {
				keys = append(keys, k)
			}
		}
	}
	return keys
}
