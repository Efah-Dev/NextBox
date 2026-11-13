package main

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
}

// Settings that mean the same on every platform: everything else (ports,
// TUN, system proxy, autostart, window behaviour, logs) stays local when a
// backup comes from another system.
var portableSettings = []string{
	"theme", "accentColor", "language", "lightLevel", "darkLevel", "recentColors",
	"remoteDns", "directDns", "enableIpv6", "sniff",
	"testUrl", "testTimeout", "testConcurrency",
	"subUserAgent", "subUpdateByProxy", "subAutoUpdate", "subUpdateHours",
	"countrySource", "autoCheckUpdate",
}

// Settings that make up the routing rules (分流规则).
var ruleSettings = []string{
	"routeMode", "blockAds", "rules", "scenes", "ruleSetMirror",
	"directRules", "proxyRules", "blockRules", // v1.2 and Android
}

// BackupOptions says what goes into a backup or is taken from one.
type BackupOptions struct {
	Nodes    bool   `json:"nodes"`    // groups, nodes, chain proxies, policy groups
	Rules    bool   `json:"rules"`    // routing rules
	Settings bool   `json:"settings"` // the other settings
	Password string `json:"password"` // "" = not encrypted (export) / not given yet (import)
	// Import only: replace the current node list instead of adding to it.
	Replace bool `json:"replace"`
}

// ExportBackup asks where to save and writes the backup. It returns the file
// path, or "" when the dialog was cancelled.
func (a *App) ExportBackup(o BackupOptions) (string, error) {
	if !o.Nodes && !o.Rules && !o.Settings {
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
	if o.Rules || o.Settings {
		raw, err := json.Marshal(a.store.Settings())
		if err != nil {
			return "", err
		}
		if !o.Settings {
			if raw, err = pickKeys(raw, ruleSettings); err != nil {
				return "", err
			}
		} else if !o.Rules {
			if raw, err = dropKeys(raw, ruleSettings); err != nil {
				return "", err
			}
		}
		doc.Settings = raw
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
		Title:   i18n.T("导入配置"),
		Filters: []wruntime.FileFilter{{DisplayName: "JSON (*.json)", Pattern: "*.json"}},
	})
}

// BackupInfo describes a backup file before it is imported.
type BackupInfo struct {
	Encrypted    bool              `json:"encrypted"`
	NeedPassword bool              `json:"needPassword"` // encrypted and no password given yet
	Platform     string            `json:"platform"`     // "windows", "android" or "" (unknown)
	Version      int               `json:"version"`
	HasNodes     bool              `json:"hasNodes"`
	HasRules     bool              `json:"hasRules"`
	HasSettings  bool              `json:"hasSettings"`
	Summary      store.DataSummary `json:"summary"`
	Rules        int               `json:"rules"`
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
		for _, k := range ruleSettings {
			if _, ok := keys[k]; ok {
				info.HasRules = true
			}
		}
		for k := range keys {
			if !contains(ruleSettings, k) {
				info.HasSettings = true
				break
			}
		}
		s := store.DefaultSettings()
		s.Rules = nil
		if json.Unmarshal(doc.Settings, &s) == nil {
			s.Normalize()
			info.Rules = len(s.Rules)
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
	if (o.Rules || o.Settings) && len(doc.Settings) > 0 {
		var keys []string
		if o.Rules {
			keys = append(keys, ruleSettings...)
		}
		if o.Settings {
			if doc.Platform == backupPlatform {
				keys = nil // everything
			} else {
				keys = append(keys, portableSettings...)
			}
		}
		next, err := mergeSettings(a.store.Settings(), doc.Settings, keys, o.Rules)
		if err != nil {
			return "", err
		}
		remapPolicies(&next, policyMap)
		if err := a.SaveSettings(next); err != nil {
			return "", err
		}
		if o.Rules {
			done = append(done, i18n.T("分流规则"))
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
		if keys != nil && !contains(keys, k) {
			continue
		}
		if keys == nil && !withRules && contains(ruleSettings, k) {
			continue
		}
		out[k] = v
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

func pickKeys(raw []byte, keys []string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	out := map[string]json.RawMessage{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return json.Marshal(out)
}

func dropKeys(raw []byte, keys []string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	for _, k := range keys {
		delete(m, k)
	}
	return json.Marshal(m)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
	if (s.ConnectOnStart || (a.autoStarted && s.AutoStartConnect)) && a.store.SelectedID() != 0 {
		go func() {
			if err := a.Connect(); err != nil {
				a.log(i18n.T("自动连接失败: ") + err.Error())
			}
		}()
	}
	return nil
}
