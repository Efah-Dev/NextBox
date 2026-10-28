package link

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"nextbox/internal/i18n"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// supportedOutbounds lists outbound types accepted from sing-box JSON imports.
var supportedOutbounds = map[string]bool{
	"shadowsocks": true, "vmess": true, "vless": true, "trojan": true,
	"hysteria": true, "hysteria2": true, "tuic": true, "anytls": true,
	"socks": true, "http": true, "wireguard": true, "ssh": true, "shadowtls": true,
}

// ParseContent parses a subscription body or pasted text. It understands
// plain/base64 link lists, Clash YAML and sing-box JSON. It returns the nodes
// it could read and, for each skipped entry, why it was skipped.
func ParseContent(text string) ([]*Node, []string, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, string(rune(0xFEFF)))) // strip UTF-8 BOM
	if text == "" {
		return nil, nil, errors.New(i18n.T("内容为空"))
	}
	if strings.HasPrefix(text, "{") {
		if nodes, skipped, err := parseSingBoxJSON(text); err == nil {
			return nodes, skipped, nil
		}
	}
	if strings.Contains(text, "proxies:") {
		if nodes, skipped, err := parseClash(text); err == nil && len(nodes) > 0 {
			return nodes, skipped, nil
		}
	}
	if !strings.Contains(text, "://") {
		decoded, err := decodeBase64(text)
		if err != nil {
			return nil, nil, errors.New(i18n.T("无法识别的订阅内容"))
		}
		text = decoded
		if strings.Contains(text, "proxies:") {
			if nodes, skipped, err := parseClash(text); err == nil && len(nodes) > 0 {
				return nodes, skipped, nil
			}
		}
	}
	var nodes []*Node
	var skipped []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		n, err := Parse(line)
		if err != nil {
			skipped = append(skipped, linkLabel(line)+"："+err.Error())
			continue
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		if len(skipped) > 0 {
			return nil, skipped, errors.New(i18n.T("没有找到可用的节点。") + skipped[0])
		}
		return nil, skipped, errors.New(i18n.T("没有找到可用的节点"))
	}
	return nodes, skipped, nil
}

// linkLabel names a share link in error messages without echoing secrets:
// the protocol plus the node name when there is one.
func linkLabel(line string) string {
	scheme, rest, _ := strings.Cut(line, "://")
	if _, frag, ok := strings.Cut(rest, "#"); ok && frag != "" {
		if name, err := url.PathUnescape(frag); err == nil {
			frag = name
		}
		return scheme + "「" + frag + "」"
	}
	return scheme + i18n.T(" 链接")
}

func parseSingBoxJSON(text string) ([]*Node, []string, error) {
	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(text), &cfg); err != nil {
		return nil, nil, err
	}
	var nodes []*Node
	var skipped []string
	for _, ob := range cfg.Outbounds {
		typ, _ := ob["type"].(string)
		name, _ := ob["tag"].(string)
		if !supportedOutbounds[typ] {
			switch typ {
			case "direct", "block", "dns", "selector", "urltest":
				// routing helpers, not nodes
			default:
				skipped = append(skipped, name+i18n.T("：不支持的出站类型 ")+typ)
			}
			continue
		}
		delete(ob, "tag")
		delete(ob, "detour")
		nodes = append(nodes, &Node{Name: name, Outbound: ob})
	}
	if len(nodes) == 0 {
		return nil, skipped, errors.New(i18n.T("JSON 中没有代理出站"))
	}
	return nodes, skipped, nil
}

// ---- Clash / Clash.Meta YAML ----

func parseClash(text string) ([]*Node, []string, error) {
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(text), &cfg); err != nil {
		return nil, nil, err
	}
	var nodes []*Node
	var skipped []string
	for _, p := range cfg.Proxies {
		n, err := clashProxy(p)
		if err != nil {
			skipped = append(skipped, cmap(p).s("name")+"："+err.Error())
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes, skipped, nil
}

type cmap map[string]any

func (m cmap) s(k string) string {
	switch v := m[k].(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

func (m cmap) i(k string) int {
	n, _ := strconv.Atoi(strings.Fields(m.s(k) + " 0")[0])
	return n
}

func (m cmap) b(k string) bool {
	v, _ := m[k].(bool)
	return v
}

func (m cmap) m(k string) cmap {
	v, _ := m[k].(map[string]any)
	return v
}

func (m cmap) list(k string) []string {
	switch v := m[k].(type) {
	case []any:
		var out []string
		for _, x := range v {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case string:
		return splitList(v)
	}
	return nil
}

func clashTLS(p cmap, server string, force bool) map[string]any {
	if !force && !p.b("tls") && p.m("reality-opts") == nil {
		return nil
	}
	t := map[string]any{"enabled": true}
	if sni := firstNonEmpty(p.s("servername"), p.s("sni")); sni != "" {
		t["server_name"] = sni
	} else if server != "" {
		t["server_name"] = server
	}
	if p.b("skip-cert-verify") {
		t["insecure"] = true
	}
	if alpn := p.list("alpn"); len(alpn) > 0 {
		t["alpn"] = alpn
	}
	fp := p.s("client-fingerprint")
	if r := p.m("reality-opts"); r != nil {
		reality := map[string]any{"enabled": true, "public_key": r.s("public-key")}
		if sid := r.s("short-id"); sid != "" {
			reality["short_id"] = sid
		}
		t["reality"] = reality
		if fp == "" {
			fp = "chrome"
		}
	}
	if fp != "" {
		t["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
	}
	return t
}

func clashTransport(p cmap) (map[string]any, error) {
	network := p.s("network")
	switch network {
	case "ws":
		o := p.m("ws-opts")
		host := ""
		if h := o.m("headers"); h != nil {
			host = h.s("Host")
		}
		t, err := buildTransport("ws", host, o.s("path"), "", "")
		if err == nil && o.i("max-early-data") > 0 {
			t["max_early_data"] = o.i("max-early-data")
			t["early_data_header_name"] = firstNonEmpty(o.s("early-data-header-name"), "Sec-WebSocket-Protocol")
		}
		return t, err
	case "grpc":
		return buildTransport("grpc", "", "", p.m("grpc-opts").s("grpc-service-name"), "")
	case "h2":
		o := p.m("h2-opts")
		return buildTransport("http", strings.Join(o.list("host"), ","), o.s("path"), "", "")
	case "http":
		o := p.m("http-opts")
		path := ""
		if paths := o.list("path"); len(paths) > 0 {
			path = paths[0]
		}
		var host string
		if h := o.m("headers"); h != nil {
			host = strings.Join(h.list("Host"), ",")
		}
		return buildTransport("tcp", host, path, "", "http")
	case "httpupgrade":
		o := p.m("http-upgrade-opts")
		if o == nil {
			o = p.m("ws-opts")
		}
		host := ""
		if h := o.m("headers"); h != nil {
			host = h.s("Host")
		}
		return buildTransport("httpupgrade", host, o.s("path"), "", "")
	}
	return buildTransport(network, "", "", "", "")
}

func clashProxy(raw map[string]any) (*Node, error) {
	p := cmap(raw)
	server := p.s("server")
	port := p.i("port")
	if server == "" || port <= 0 {
		return nil, errors.New(i18n.T("缺少服务器信息"))
	}
	ob := map[string]any{"server": server, "server_port": port}
	switch p.s("type") {
	case "ss":
		ob["type"] = "shadowsocks"
		ob["method"] = p.s("cipher")
		ob["password"] = p.s("password")
		switch p.s("plugin") {
		case "obfs":
			o := p.m("plugin-opts")
			ob["plugin"] = "obfs-local"
			opts := "obfs=" + o.s("mode")
			if h := o.s("host"); h != "" {
				opts += ";obfs-host=" + h
			}
			ob["plugin_opts"] = opts
		case "v2ray-plugin":
			o := p.m("plugin-opts")
			ob["plugin"] = "v2ray-plugin"
			opts := "mode=" + firstNonEmpty(o.s("mode"), "websocket")
			if o.b("tls") {
				opts += ";tls"
			}
			if h := o.s("host"); h != "" {
				opts += ";host=" + h
			}
			if path := o.s("path"); path != "" {
				opts += ";path=" + path
			}
			ob["plugin_opts"] = opts
		case "":
		default:
			return nil, fmt.Errorf(i18n.T("不支持的 ss 插件 %s"), p.s("plugin"))
		}
	case "vmess":
		ob["type"] = "vmess"
		ob["uuid"] = p.s("uuid")
		ob["security"] = firstNonEmpty(p.s("cipher"), "auto")
		if aid := p.i("alterId"); aid > 0 {
			ob["alter_id"] = aid
		}
	case "vless":
		ob["type"] = "vless"
		ob["uuid"] = p.s("uuid")
		if f := p.s("flow"); f != "" {
			ob["flow"] = f
		}
		ob["packet_encoding"] = "xudp"
	case "trojan":
		ob["type"] = "trojan"
		ob["password"] = p.s("password")
	case "hysteria2":
		ob["type"] = "hysteria2"
		ob["password"] = firstNonEmpty(p.s("password"), p.s("auth"))
		if obfs := p.s("obfs"); obfs != "" {
			ob["obfs"] = map[string]any{"type": obfs, "password": p.s("obfs-password")}
		}
		if ports := p.s("ports"); ports != "" {
			var ranges []string
			for _, r := range splitList(strings.ReplaceAll(ports, "/", ",")) {
				if a, b, ok := strings.Cut(r, "-"); ok {
					ranges = append(ranges, a+":"+b)
				} else {
					ranges = append(ranges, r+":"+r)
				}
			}
			ob["server_ports"] = ranges
		}
		if up := p.i("up"); up > 0 {
			ob["up_mbps"] = up
		}
		if down := p.i("down"); down > 0 {
			ob["down_mbps"] = down
		}
	case "tuic":
		ob["type"] = "tuic"
		ob["uuid"] = p.s("uuid")
		ob["password"] = p.s("password")
		ob["congestion_control"] = firstNonEmpty(p.s("congestion-controller"), "bbr")
		ob["udp_relay_mode"] = firstNonEmpty(p.s("udp-relay-mode"), "native")
	case "anytls":
		ob["type"] = "anytls"
		ob["password"] = p.s("password")
	case "socks5":
		ob["type"] = "socks"
		ob["version"] = "5"
		if u := p.s("username"); u != "" {
			ob["username"], ob["password"] = u, p.s("password")
		}
	case "http":
		ob["type"] = "http"
		if u := p.s("username"); u != "" {
			ob["username"], ob["password"] = u, p.s("password")
		}
	default:
		return nil, fmt.Errorf(i18n.T("不支持的协议 %s"), p.s("type"))
	}

	typ := ob["type"].(string)
	switch typ {
	case "vmess", "vless", "trojan":
		tr, err := clashTransport(p)
		if err != nil {
			return nil, err
		}
		if tr != nil {
			ob["transport"] = tr
		}
	}
	forceTLS := typ == "trojan" || typ == "hysteria2" || typ == "tuic" || typ == "anytls"
	if t := clashTLS(p, server, forceTLS); t != nil {
		if typ == "tuic" {
			if _, ok := t["alpn"]; !ok {
				t["alpn"] = []string{"h3"}
			}
		}
		ob["tls"] = t
	}
	return &Node{Name: p.s("name"), Outbound: ob}, nil
}
