package app

// Files dragged into the window: a backup, a Clash or sing-box config, or a
// text file of share links or a subscription address. What a file is comes
// from its content, not its name.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nextbox/internal/clash"
	"nextbox/internal/i18n"
	"nextbox/internal/link"
	"nextbox/internal/store"
	"nextbox/internal/vault"
)

// maxDropSize is the largest file taken; bigger ones are not configs.
const maxDropSize = 20 << 20

// DroppedFile is what a dropped file holds.
type DroppedFile struct {
	// Kind is "backup" (a NextBox backup, also an encrypted one), "config"
	// (a Clash / sing-box config with nodes), "subscription" (an address,
	// see Link), "nodes" (share links) or "none".
	Kind string `json:"kind"`
	Path string `json:"path"`
	Name string `json:"name"` // the file name without its extension
	// Link is what a text file holds (Kind subscription or nodes).
	Link  link.Classified `json:"link"`
	Count int             `json:"count,omitempty"` // nodes of a config
}

// InspectFile tells what a dropped file is, without changing anything.
func (a *App) InspectFile(path string) (DroppedFile, error) {
	base := filepath.Base(path)
	out := DroppedFile{Kind: "none", Path: path, Name: strings.TrimSuffix(base, filepath.Ext(base))}
	st, err := os.Stat(path)
	if err != nil {
		return out, err
	}
	if st.IsDir() {
		return out, fmt.Errorf(i18n.T("“%s”是文件夹，请拖入文件"), base)
	}
	if st.Size() > maxDropSize {
		return out, fmt.Errorf(i18n.T("“%s”超过 20 MB，不是 NextBox 能导入的文件"), base)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	if _, ok := vault.Parse(b); ok {
		out.Kind = "backup" // encrypted: the import dialog asks for the password
		return out, nil
	}
	text := strings.TrimSpace(strings.TrimPrefix(string(b), string(rune(0xFEFF))))
	if isBackup(text) {
		out.Kind = "backup"
		return out, nil
	}
	if isConfig(text) {
		nodes, _, err := link.ParseContent(text)
		if err != nil || len(nodes) == 0 {
			return out, fmt.Errorf(i18n.T("“%s”里没有可以导入的节点"), base)
		}
		out.Kind, out.Count = "config", len(nodes)
		return out, nil
	}
	out.Link = link.Classify(text)
	switch out.Link.Kind {
	case "subscription", "nodes":
		out.Kind = out.Link.Kind
	default:
		return out, fmt.Errorf(i18n.T("“%s”不是 NextBox 能导入的文件"), base)
	}
	return out, nil
}

// isBackup: NextBox's backup document (or a bare profiles.json).
func isBackup(text string) bool {
	if !strings.HasPrefix(text, "{") {
		return false
	}
	var doc struct {
		App      string          `json:"app"`
		Profiles json.RawMessage `json:"profiles"`
		Settings json.RawMessage `json:"settings"`
		NextID   *int64          `json:"nextId"`
	}
	if json.Unmarshal([]byte(text), &doc) != nil {
		return false
	}
	return strings.EqualFold(doc.App, "NextBox") || doc.NextID != nil
}

// isConfig: a Clash config (proxies) or a sing-box config (outbounds).
func isConfig(text string) bool {
	if strings.HasPrefix(text, "{") {
		var doc struct {
			Outbounds []json.RawMessage `json:"outbounds"`
			Endpoints []json.RawMessage `json:"endpoints"`
		}
		return json.Unmarshal([]byte(text), &doc) == nil && len(doc.Outbounds)+len(doc.Endpoints) > 0
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "proxies:") {
			return true
		}
	}
	return false
}

// ImportConfigFile makes a new group, named after the file, of the nodes of
// a dropped Clash / sing-box config, and keeps the config's proxy groups,
// rules and DNS for the Clash import dialog. The new group becomes the
// current one.
func (a *App) ImportConfigFile(path string) (string, error) {
	f, err := a.InspectFile(path)
	if err != nil {
		return "", err
	}
	if f.Kind != "config" {
		return "", errors.New(i18n.T("不是 Clash 或 sing-box 配置文件"))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(b)
	nodes, skipped, err := link.ParseContent(text)
	if err != nil {
		return "", err
	}
	list := make([]store.Profile, 0, len(nodes))
	for _, n := range nodes {
		list = append(list, store.Profile{Name: n.Name, Type: n.Type(), Outbound: n.Outbound})
	}
	all := len(list)
	list = store.DedupeList(list)
	g, err := a.store.AddGroup(store.Group{Name: f.Name})
	if err != nil {
		return "", err
	}
	added, err := a.store.AddProfiles(g.ID, list)
	if err != nil {
		return "", err
	}
	ex, _ := clash.Parse(text)
	if ex == nil && strings.HasPrefix(strings.TrimSpace(text), "{") {
		ex = clash.ParseSingBox(text)
	}
	if ex != nil {
		a.setClashExtras(g.ID, ex)
	}
	a.store.SetCurrentGroup(g.ID)
	a.emitState()
	a.emitProfiles(g.ID)
	go a.locateServers(idsOf(added), false)
	msg := fmt.Sprintf(i18n.T("已从“%s”导入 %d 个节点"), filepath.Base(path), len(added))
	if n := all - len(list); n > 0 {
		msg += fmt.Sprintf(i18n.T("（去掉了 %d 个重复节点）"), n)
	}
	msg += a.skipNote(skipped)
	a.log(msg)
	return msg, nil
}

// ImportLinksFile imports the share links of a dropped text file into a group.
func (a *App) ImportLinksFile(path string, groupID int64) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return a.ImportText(string(b), groupID)
}
