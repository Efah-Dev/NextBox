// Package service is the NextBox service (服务模式, v2.3): a Windows
// service, installed once with administrator rights, that runs the core in
// TUN mode for NextBox, so that NextBox itself never needs to run as
// administrator. It also starts NextBox when its user logs on (开机自启).
//
// The service lives in "%ProgramFiles%\NextBox Service" (NextBoxService.exe
// and a copy of the core), which only administrators can change: the
// service runs as LocalSystem, so a copy in NextBox's own (portable,
// user-writable) folder would let any program of the user replace it.
//
// NextBox talks to it over a named pipe that only the users who installed it
// (and administrators) can open, one JSON message per line:
//
//	→ {"op":"hello"}                    ← Status
//	→ {"op":"start","config":…,"ruleSets":{…}}
//	                                     ← {"started":true}, {"log":"…"}…, {"exit":"…"}
//	→ {"op":"stop"}                      (on the start connection; closing it does the same)
//	→ {"op":"autostart","path":…,"enabled":…} ← Status
//
// A core lives as long as the connection that started it: when NextBox
// exits or crashes, the service stops the core and its TUN routes go away.
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	Name        = "NextBoxService"
	DisplayName = "NextBox Service"
	PipeName    = `\\.\pipe\NextBoxService`
	ExeName     = "NextBoxService.exe"
	DirName     = "NextBox Service" // under %ProgramFiles%
	// Protocol is the version of the messages; NextBox only uses a service
	// that speaks the same one.
	Protocol = 1
)

// Request is a message to the service.
type Request struct {
	Op string `json:"op"` // "hello", "start", "stop", "autostart"
	// start: the core's config, and the rule-set files it names
	// (route.rule_set[].path), by file name.
	Config   json.RawMessage   `json:"config,omitempty"`
	RuleSets map[string][]byte `json:"ruleSets,omitempty"`
	// autostart: start the program at Path when the calling user logs on, or
	// stop doing so.
	Path    string `json:"path,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

// Reply is a message from the service.
type Reply struct {
	Error string `json:"error,omitempty"`

	// hello, autostart
	Status *Status `json:"status,omitempty"`

	// start
	Started bool    `json:"started,omitempty"`
	Log     string  `json:"log,omitempty"`  // a line the core printed
	Exit    *string `json:"exit,omitempty"` // the core exited (why, "" when it was asked to)
}

// Status is what the service tells NextBox about itself.
type Status struct {
	Version  string `json:"version"`  // the NextBox version it was installed from
	Protocol int    `json:"protocol"` // see Protocol
	Dir      string `json:"dir"`      // where it is installed
	// CoreHash is the SHA-256 of its sing-box.exe, so that NextBox can tell
	// whether it runs the same core as NextBox's own folder.
	CoreHash string `json:"coreHash"`
	Running  bool   `json:"running"` // a core is running
	// Autostart is the program started when the calling user logs on, "" =
	// none.
	Autostart string `json:"autostart"`
}

// ---------------------------------------------------------------- config checks

// The core runs as LocalSystem, so the config NextBox sends must not make it
// read or write files of its choosing: it may only name rule-sets (which the
// service writes itself, from the files sent along) and everything that
// writes files is put in the service's own folder or left out.

var ruleSetFile = regexp.MustCompile(`^[A-Za-z0-9@!._-]+\.srs$`)

// allowedTop are the parts of a config the service runs.
var allowedTop = map[string]bool{
	"log": true, "dns": true, "inbounds": true, "outbounds": true, "endpoints": true, "route": true, "experimental": true,
}

// Sanitize checks a config and rewrites it to run in workDir with the
// rule-sets in ruleSetDir. It returns the rule-set file names it needs.
func Sanitize(cfg map[string]any, workDir, ruleSetDir string) (ruleSets []string, err error) {
	for k := range cfg {
		if !allowedTop[k] {
			delete(cfg, k)
		}
	}
	if l, ok := cfg["log"].(map[string]any); ok {
		cfg["log"] = keep(l, "level", "timestamp", "disabled")
	}
	if ex, ok := cfg["experimental"].(map[string]any); ok {
		out := map[string]any{}
		if api, ok := ex["clash_api"].(map[string]any); ok {
			ctl, _ := api["external_controller"].(string)
			if !strings.HasPrefix(ctl, "127.0.0.1:") {
				return nil, errors.New("the clash API must listen on 127.0.0.1")
			}
			out["clash_api"] = keep(api, "external_controller", "secret")
		}
		if cf, ok := ex["cache_file"].(map[string]any); ok {
			cf = keep(cf, "enabled", "store_fakeip", "store_rdrc")
			cf["path"] = filepath.Join(workDir, "cache.db")
			out["cache_file"] = cf
		}
		cfg["experimental"] = out
	}
	if r, ok := cfg["route"].(map[string]any); ok {
		sets, _ := r["rule_set"].([]any)
		for _, s := range sets {
			m, ok := s.(map[string]any)
			if !ok || m["type"] != "local" {
				return nil, errors.New("only local rule-sets can be used")
			}
			p, _ := m["path"].(string)
			name := filepath.Base(p)
			if !ruleSetFile.MatchString(name) {
				return nil, fmt.Errorf("bad rule-set file %q", p)
			}
			m["path"] = filepath.Join(ruleSetDir, name)
			m["format"] = "binary"
			ruleSets = append(ruleSets, name)
		}
	}
	if eps, ok := cfg["endpoints"].([]any); ok {
		for _, e := range eps {
			if m, ok := e.(map[string]any); !ok || m["type"] != "wireguard" {
				return nil, errors.New("only WireGuard endpoints can be used")
			}
		}
	}
	if d, ok := cfg["dns"].(map[string]any); ok {
		servers, _ := d["servers"].([]any)
		for _, s := range servers {
			if m, ok := s.(map[string]any); ok && m["type"] == "hosts" {
				return nil, errors.New("hosts DNS servers cannot be used")
			}
		}
	}
	// anything else that names a file (certificates, keys, ...)
	for _, part := range []string{"inbounds", "outbounds", "endpoints", "dns", "route"} {
		if p := findPath(cfg[part], part); p != "" {
			return nil, fmt.Errorf("files cannot be used (%s)", p)
		}
	}
	return ruleSets, nil
}

func keep(m map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

// findPath returns where in v a field names a file ("…_path", or "path" in
// a rule-set, which Sanitize has already rewritten), "" when none does.
func findPath(v any, at string) string {
	switch t := v.(type) {
	case map[string]any:
		for k, sub := range t {
			if strings.HasSuffix(k, "_path") || (k == "path" && isFileish(at)) {
				return at + "." + k
			}
			if p := findPath(sub, at+"."+k); p != "" {
				return p
			}
		}
	case []any:
		for _, sub := range t {
			if p := findPath(sub, at+"[]"); p != "" {
				return p
			}
		}
	}
	return ""
}

// isFileish tells the objects whose "path" is a file from the ones whose
// "path" is a URL path (transports, DNS over HTTPS).
func isFileish(at string) bool {
	return strings.HasSuffix(at, ".tls") || strings.HasSuffix(at, ".certificate") || strings.HasSuffix(at, ".ech")
}
