package config

import (
	"fmt"
	"net"

	"nextbox/internal/gost"
	"nextbox/internal/i18n"
)

// TagGostIn is the SOCKS inbound the gost-plugin relays reach out through.
const TagGostIn = "gost-in"

// withGost connects the Shadowsocks nodes that use Clash.Meta's gost-plugin
// (the "gost" field) to local relays, as sing-box has no such transport
// (see package gost). Each such outbound dials its relay on 127.0.0.1; the
// relay comes back through a SOCKS inbound added here, and a route rule per
// relay sends it out the way the node would have gone: direct, or through
// the hop before it in a chain proxy. A config without such nodes is left
// exactly as it is.
func withGost(cfg map[string]any) error {
	outbounds, _ := cfg["outbounds"].([]any)
	var nodes []map[string]any
	for _, ob := range outbounds {
		if m, ok := ob.(map[string]any); ok {
			if _, has := gost.FromOutbound(m); has {
				nodes = append(nodes, m)
			}
		}
	}
	if len(nodes) == 0 {
		return nil
	}
	sess, err := gost.NewSession()
	if err != nil {
		return fmt.Errorf(i18n.T("gost-plugin 本机中转启动失败：%w"), err)
	}
	var users, rules []any
	for _, ob := range nodes {
		o, _ := gost.FromOutbound(ob)
		server, _ := ob["server"].(string)
		target := net.JoinHostPort(server, fmt.Sprint(ob["server_port"]))
		port, user, err := sess.Add(target, o)
		if err != nil {
			sess.Close()
			return fmt.Errorf(i18n.T("gost-plugin 本机中转启动失败：%w"), err)
		}
		users = append(users, map[string]any{"username": user, "password": sess.Password})
		// the relay's way out: the node's DNS for the server name, then the
		// node's detour (a chain proxy's previous hop) or direct
		if res, ok := ob["domain_resolver"].(string); ok && res != "" {
			rules = append(rules, map[string]any{"inbound": []string{TagGostIn}, "auth_user": []string{user}, "action": "resolve", "server": res})
		}
		if d, ok := ob["detour"].(string); ok && d != "" {
			rules = append(rules, map[string]any{"inbound": []string{TagGostIn}, "auth_user": []string{user}, "outbound": d})
		}
		delete(ob, "gost")
		delete(ob, "detour")
		delete(ob, "domain_resolver")
		ob["server"], ob["server_port"] = "127.0.0.1", port
	}
	rules = append(rules, map[string]any{"inbound": []string{TagGostIn}, "outbound": TagDirect})
	inbounds, _ := cfg["inbounds"].([]any)
	cfg["inbounds"] = append(inbounds, map[string]any{
		"type": "socks", "tag": TagGostIn, "listen": "127.0.0.1", "listen_port": sess.ViaPort, "users": users,
	})
	route, _ := cfg["route"].(map[string]any)
	if route == nil {
		route = map[string]any{}
		cfg["route"] = route
	}
	old, _ := route["rules"].([]any)
	route["rules"] = append(rules, old...)
	return nil
}
