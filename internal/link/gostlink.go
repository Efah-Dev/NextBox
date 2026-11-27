package link

import (
	"fmt"
	"nextbox/internal/i18n"
	"strings"
)

// gost-plugin in ss:// links, written as SIP002 plugin options:
// "gost-plugin;mode=websocket;host=cdn.example;path=/ws;mux=true;tls".
// mux is on unless "mux=false", as in Clash.Meta.

// gostPluginString writes the "gost" field of an outbound as a plugin value.
func gostPluginString(g map[string]any) string {
	s, _ := g["host"].(string)
	path, _ := g["path"].(string)
	parts := []string{"gost-plugin", "mode=websocket", "host=" + s, "path=" + path}
	mux, _ := g["mux"].(bool)
	parts = append(parts, fmt.Sprintf("mux=%t", mux))
	if tls, _ := g["tls"].(bool); tls {
		parts = append(parts, "tls")
		if sni, _ := g["sni"].(string); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if insecure, _ := g["insecure"].(bool); insecure {
			parts = append(parts, "skip-cert-verify")
		}
	}
	return strings.Join(parts, ";")
}

// gostFromPluginOpts reads the options after "gost-plugin;" into the "gost"
// field of an outbound.
func gostFromPluginOpts(opts string) (map[string]any, error) {
	o := map[string]string{}
	for _, part := range strings.Split(opts, ";") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		if k != "" {
			o[strings.ToLower(k)] = v
		}
	}
	if mode := o["mode"]; mode != "" && mode != "websocket" {
		return nil, fmt.Errorf(i18n.T("不支持的 ss 插件 %s"), "gost-plugin "+mode)
	}
	on := func(k string) bool {
		v, ok := o[k]
		return ok && v != "false" && v != "0"
	}
	g := map[string]any{"host": firstNonEmpty(o["host"], "bing.com"), "path": firstNonEmpty(o["path"], "/")}
	if v, ok := o["mux"]; !ok || (v != "false" && v != "0") {
		g["mux"] = true
	}
	if on("tls") {
		g["tls"] = true
		if sni := o["sni"]; sni != "" {
			g["sni"] = sni
		}
		if on("skip-cert-verify") || on("insecure") {
			g["insecure"] = true
		}
	}
	return g, nil
}
