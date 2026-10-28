// Package link converts share links and subscription contents into sing-box
// outbound objects, and back.
package link

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"nextbox/internal/i18n"
	"regexp"
	"strconv"
	"strings"
)

// Node is a parsed proxy: a display name plus a sing-box outbound (no tag).
type Node struct {
	Name     string
	Outbound map[string]any
}

func (n *Node) Type() string {
	t, _ := n.Outbound["type"].(string)
	return t
}

// unsupported is translated when returned, so it follows the current language.
func unsupported() error { return errors.New(i18n.T("不支持的链接格式")) }

// Parse parses a single share link.
func Parse(raw string) (*Node, error) {
	raw = cleanHost(strings.TrimSpace(raw))
	scheme, _, ok := strings.Cut(raw, "://")
	if !ok {
		return nil, unsupported()
	}
	var (
		n   *Node
		err error
	)
	switch strings.ToLower(scheme) {
	case "ss":
		n, err = parseShadowsocks(raw)
	case "vmess":
		n, err = parseVMess(raw)
	case "vless":
		n, err = parseVLESS(raw)
	case "trojan":
		n, err = parseTrojan(raw)
	case "hysteria2", "hy2":
		n, err = parseHysteria2(raw)
	case "tuic":
		n, err = parseTUIC(raw)
	case "anytls":
		n, err = parseAnyTLS(raw)
	case "socks", "socks5", "socks4", "socks4a":
		n, err = parseSocks(raw)
	case "http", "https":
		n, err = parseHTTP(raw)
	default:
		return nil, fmt.Errorf("%w: %s", unsupported(), scheme)
	}
	if err != nil {
		return nil, err
	}
	if n.Name == "" {
		n.Name = fmt.Sprintf("%s %v:%v", n.Type(), n.Outbound["server"], n.Outbound["server_port"])
	}
	return n, nil
}

// ---- helpers ----

func decodeBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("\r", "", "\n", "", " ", "").Replace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return string(b), nil
		}
	}
	return "", errors.New(i18n.T("base64 解码失败"))
}

// parseURL parses a share link leniently. The "#name" part is split off
// first: node names often contain spaces, emoji or a stray "%", which make
// url.Parse reject the whole link.
func parseURL(raw string) (*url.URL, string, error) {
	base, frag, _ := strings.Cut(strings.TrimSpace(raw), "#")
	name := frag
	if n, err := url.PathUnescape(frag); err == nil {
		name = n
	}
	base = cleanHost(base)
	u, err := url.Parse(base)
	if err != nil {
		// retry with the query string set aside; it is parsed leniently later
		if b, q, ok := strings.Cut(base, "?"); ok {
			if u, err = url.Parse(b); err == nil {
				u.RawQuery = q
			}
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf(i18n.T("链接格式错误: %w"), err)
	}
	return u, strings.TrimSpace(name), nil
}

// cleanHost removes spaces (plain or "%20") around the server address, e.g.
// "vless://uuid@%20203.0.113.1:443" written by some panels and hand-edited
// subscriptions. url.Parse rejects any space in the host.
func cleanHost(link string) string {
	scheme, rest, ok := strings.Cut(link, "://")
	if !ok {
		return link
	}
	end := strings.IndexAny(rest, "/?")
	if end < 0 {
		end = len(rest)
	}
	authority, tail := rest[:end], rest[end:]
	userinfo, host := "", authority
	if i := strings.LastIndex(authority, "@"); i >= 0 {
		userinfo, host = authority[:i+1], authority[i+1:]
	}
	host = strings.NewReplacer("%20", "", " ", "", "\t", "").Replace(host)
	return scheme + "://" + userinfo + host + tail
}

// markdownLink matches "[text](url)", which chat apps and note editors
// sometimes turn a bare domain like www.apple.com into.
var markdownLink = regexp.MustCompile(`^\[([^\]]+)\]\([^)]*\)$`)

// query reads the first non-empty value among several alias keys
// (share-link generators disagree on spelling, e.g. pbk / publicKey).
func query(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			if m := markdownLink.FindStringSubmatch(v); m != nil {
				v = m[1]
			}
			return v
		}
	}
	return ""
}

func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || p <= 0 || p > 65535 {
		return 0, fmt.Errorf(i18n.T("无效端口: %s"), s)
	}
	return p, nil
}

func hostPort(u *url.URL) (string, int, error) {
	host := u.Hostname()
	if host == "" {
		return "", 0, errors.New(i18n.T("缺少服务器地址"))
	}
	port, err := parsePort(u.Port())
	return host, port, err
}

func truthy(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes"
}

func splitList(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// buildTLS creates a sing-box outbound TLS object from common share-link params.
func buildTLS(q url.Values, server string) map[string]any {
	t := map[string]any{"enabled": true}
	sni := query(q, "sni", "peer", "servername", "serverName", "server_name")
	if sni != "" {
		t["server_name"] = sni
	} else if net.ParseIP(server) == nil {
		t["server_name"] = server
	}
	if truthy(query(q, "allowInsecure", "insecure", "allow_insecure", "skip-cert-verify")) {
		t["insecure"] = true
	}
	if alpn := splitList(q.Get("alpn")); len(alpn) > 0 {
		t["alpn"] = alpn
	}
	fp := query(q, "fp", "fingerprint", "client-fingerprint")
	if pbk := query(q, "pbk", "publicKey", "public-key", "public_key"); pbk != "" {
		reality := map[string]any{"enabled": true, "public_key": pbk}
		if sid := query(q, "sid", "shortId", "short-id", "short_id"); sid != "" {
			reality["short_id"] = sid
		}
		t["reality"] = reality
		if fp == "" {
			fp = "chrome"
		}
	}
	if fp != "" && fp != "none" {
		t["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
	}
	return t
}

// buildTransport maps V2Ray-style transport params to a sing-box transport.
// A nil result means plain TCP.
func buildTransport(network, host, path, serviceName, headerType string) (map[string]any, error) {
	switch strings.ToLower(network) {
	case "", "tcp", "raw":
		if headerType == "http" {
			t := map[string]any{"type": "http", "method": "GET"}
			if hosts := splitList(host); len(hosts) > 0 {
				t["host"] = hosts
			}
			if path != "" {
				t["path"] = path
			}
			return t, nil
		}
		return nil, nil
	case "ws", "websocket":
		t := map[string]any{"type": "ws"}
		if path != "" {
			p, ed := splitEarlyData(path)
			t["path"] = p
			if ed > 0 {
				t["max_early_data"] = ed
				t["early_data_header_name"] = "Sec-WebSocket-Protocol"
			}
		}
		if host != "" {
			t["headers"] = map[string]any{"Host": host}
		}
		return t, nil
	case "grpc":
		t := map[string]any{"type": "grpc"}
		if s := firstNonEmpty(serviceName, path); s != "" {
			t["service_name"] = s
		}
		return t, nil
	case "http", "h2":
		t := map[string]any{"type": "http"}
		if hosts := splitList(host); len(hosts) > 0 {
			t["host"] = hosts
		}
		if path != "" {
			t["path"] = path
		}
		return t, nil
	case "httpupgrade":
		t := map[string]any{"type": "httpupgrade"}
		if host != "" {
			t["host"] = host
		}
		if path != "" {
			t["path"] = path
		}
		return t, nil
	case "quic":
		return map[string]any{"type": "quic"}, nil
	case "xhttp", "splithttp":
		return nil, errors.New(i18n.T("XHTTP 是 Xray 专有的传输方式，sing-box 内核不支持，请在服务端改用 TCP / WS / gRPC / HTTPUpgrade"))
	case "kcp", "mkcp":
		return nil, errors.New(i18n.T("mKCP 是 Xray 专有的传输方式，sing-box 内核不支持"))
	default:
		return nil, fmt.Errorf(i18n.T("sing-box 不支持传输方式 %s"), network)
	}
}

// splitEarlyData handles the Xray convention "/path?ed=2048".
func splitEarlyData(path string) (string, int) {
	p, query, ok := strings.Cut(path, "?")
	if !ok {
		return path, 0
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return path, 0
	}
	ed, _ := strconv.Atoi(q.Get("ed"))
	if ed <= 0 {
		return path, 0
	}
	q.Del("ed")
	if rest := q.Encode(); rest != "" {
		p += "?" + rest
	}
	return p, ed
}

// ---- Shadowsocks ----

func parseShadowsocks(raw string) (*Node, error) {
	body := strings.TrimPrefix(raw[len("ss://"):], "//")
	body, frag, _ := strings.Cut(body, "#")
	name, _ := url.PathUnescape(frag)

	// Legacy format: ss://base64(method:password@host:port)
	if !strings.Contains(body, "@") {
		main, _, _ := strings.Cut(body, "?")
		decoded, err := decodeBase64(main)
		if err != nil {
			return nil, err
		}
		body = decoded
	}
	u, err := url.Parse("ss://" + body)
	if err != nil {
		return nil, err
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	var method, password string
	if u.User != nil {
		if pw, ok := u.User.Password(); ok {
			method, password = u.User.Username(), pw
		} else if decoded, err := decodeBase64(u.User.Username()); err == nil {
			method, password, _ = strings.Cut(decoded, ":")
		} else {
			method, password, _ = strings.Cut(u.User.Username(), ":")
		}
	}
	if method == "" {
		return nil, errors.New(i18n.T("ss 链接缺少加密方式"))
	}
	ob := map[string]any{
		"type":        "shadowsocks",
		"server":      host,
		"server_port": port,
		"method":      method,
		"password":    password,
	}
	if plugin := u.Query().Get("plugin"); plugin != "" {
		pname, opts, _ := strings.Cut(plugin, ";")
		if pname == "simple-obfs" {
			pname = "obfs-local"
		}
		ob["plugin"] = pname
		if opts != "" {
			ob["plugin_opts"] = opts
		}
	}
	return &Node{Name: name, Outbound: ob}, nil
}

// ---- VMess ----

func parseVMess(raw string) (*Node, error) {
	body := raw[len("vmess://"):]
	decoded, err := decodeBase64(body)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(decoded), &v); err != nil {
		return nil, errors.New(i18n.T("vmess 链接内容不是有效 JSON"))
	}
	str := func(k string) string {
		switch x := v[k].(type) {
		case string:
			return strings.TrimSpace(x)
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
		return ""
	}
	port, err := parsePort(str("port"))
	if err != nil {
		return nil, err
	}
	server := str("add")
	if server == "" {
		return nil, errors.New(i18n.T("vmess 链接缺少服务器地址"))
	}
	ob := map[string]any{
		"type":        "vmess",
		"server":      server,
		"server_port": port,
		"uuid":        str("id"),
		"security":    firstNonEmpty(str("scy"), "auto"),
	}
	if aid, _ := strconv.Atoi(str("aid")); aid > 0 {
		ob["alter_id"] = aid
	}
	tr, err := buildTransport(str("net"), str("host"), str("path"), str("path"), str("type"))
	if err != nil {
		return nil, err
	}
	if tr != nil {
		ob["transport"] = tr
	}
	if tls := str("tls"); tls == "tls" || tls == "reality" {
		q := url.Values{}
		q.Set("sni", firstNonEmpty(str("sni"), str("host")))
		q.Set("alpn", str("alpn"))
		q.Set("fp", str("fp"))
		if truthy(str("allowInsecure")) || truthy(str("skip-cert-verify")) {
			q.Set("insecure", "1")
		}
		ob["tls"] = buildTLS(q, server)
	}
	return &Node{Name: str("ps"), Outbound: ob}, nil
}

// ---- VLESS / Trojan (shared query format) ----

func applyStreamQuery(ob map[string]any, q url.Values, server string, defaultTLS bool) error {
	network := query(q, "type", "network", "net")
	tr, err := buildTransport(network, q.Get("host"), q.Get("path"), query(q, "serviceName", "service_name", "service-name"), q.Get("headerType"))
	if err != nil {
		return err
	}
	if tr != nil {
		ob["transport"] = tr
	}
	security := strings.ToLower(q.Get("security"))
	if security == "" && (defaultTLS || query(q, "pbk", "publicKey", "public-key") != "") {
		security = "tls"
	}
	if security == "tls" || security == "reality" || security == "xtls" {
		ob["tls"] = buildTLS(q, server)
	}
	return nil
}

func parseVLESS(raw string) (*Node, error) {
	u, name, err := parseURL(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if enc := q.Get("encryption"); enc != "" && enc != "none" {
		return nil, errors.New(i18n.T("VLESS 加密（encryption=") + enc + i18n.T("）是 Xray 专有功能，sing-box 内核不支持"))
	}
	uuid := ""
	if u.User != nil {
		uuid = u.User.Username()
	}
	if uuid == "" {
		return nil, errors.New(i18n.T("vless 链接缺少 UUID"))
	}
	ob := map[string]any{
		"type":        "vless",
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
	}
	if flow := q.Get("flow"); flow != "" {
		ob["flow"] = strings.TrimSuffix(flow, "-udp443")
	}
	if pe := query(q, "packetEncoding", "packet_encoding"); pe != "" {
		ob["packet_encoding"] = pe
	} else {
		ob["packet_encoding"] = "xudp"
	}
	if err := applyStreamQuery(ob, q, host, false); err != nil {
		return nil, err
	}
	return &Node{Name: name, Outbound: ob}, nil
}

func parseTrojan(raw string) (*Node, error) {
	u, name, err := parseURL(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	password := u.User.Username()
	if pw, ok := u.User.Password(); ok {
		password += ":" + pw
	}
	ob := map[string]any{
		"type":        "trojan",
		"server":      host,
		"server_port": port,
		"password":    password,
	}
	if err := applyStreamQuery(ob, u.Query(), host, true); err != nil {
		return nil, err
	}
	return &Node{Name: name, Outbound: ob}, nil
}

// ---- Hysteria2 ----

func parseHysteria2(raw string) (*Node, error) {
	// Port hopping links ("host:443,20000-30000") are not valid URLs; pull the
	// port section out before parsing.
	rest := raw[strings.Index(raw, "://")+3:]
	authority := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		authority = rest[:i]
	}
	userinfo, hostport := "", authority
	if i := strings.LastIndex(authority, "@"); i >= 0 {
		userinfo, hostport = authority[:i], authority[i+1:]
	}
	var portSpec string
	if strings.HasPrefix(hostport, "[") {
		if i := strings.Index(hostport, "]:"); i >= 0 {
			portSpec = hostport[i+2:]
			hostport = hostport[:i+1]
		}
	} else if i := strings.Index(hostport, ":"); i >= 0 {
		portSpec = hostport[i+1:]
		hostport = hostport[:i]
	}
	if portSpec == "" {
		portSpec = "443"
	}
	firstPort := portSpec
	if i := strings.IndexAny(firstPort, ",-"); i >= 0 {
		firstPort = firstPort[:i]
	}
	u, name, err := parseURL("hysteria2://" + userinfo + "@" + hostport + ":" + firstPort + rest[len(authority):])
	if err != nil {
		return nil, err
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	password := u.User.Username()
	if pw, ok := u.User.Password(); ok {
		password += ":" + pw
	}
	ob := map[string]any{
		"type":        "hysteria2",
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls":         buildTLS(q, host),
	}
	hop := firstNonEmpty(q.Get("mport"), q.Get("ports"))
	if hop == "" && strings.ContainsAny(portSpec, ",-") {
		hop = portSpec
	}
	if hop != "" {
		var ranges []string
		for _, r := range splitList(hop) {
			if a, b, ok := strings.Cut(r, "-"); ok {
				ranges = append(ranges, a+":"+b)
			} else {
				ranges = append(ranges, r+":"+r)
			}
		}
		ob["server_ports"] = ranges
	}
	if obfs := q.Get("obfs"); obfs != "" && obfs != "none" {
		ob["obfs"] = map[string]any{"type": obfs, "password": q.Get("obfs-password")}
	}
	if up, _ := strconv.Atoi(q.Get("upmbps")); up > 0 {
		ob["up_mbps"] = up
	}
	if down, _ := strconv.Atoi(q.Get("downmbps")); down > 0 {
		ob["down_mbps"] = down
	}
	return &Node{Name: name, Outbound: ob}, nil
}

// ---- TUIC ----

func parseTUIC(raw string) (*Node, error) {
	u, name, err := parseURL(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	password, _ := u.User.Password()
	tls := buildTLS(q, host)
	if _, ok := tls["alpn"]; !ok {
		tls["alpn"] = []string{"h3"}
	}
	if truthy(q.Get("disable_sni")) {
		tls["disable_sni"] = true
	}
	ob := map[string]any{
		"type":               "tuic",
		"server":             host,
		"server_port":        port,
		"uuid":               u.User.Username(),
		"password":           password,
		"congestion_control": firstNonEmpty(q.Get("congestion_control"), q.Get("congestion-control"), "bbr"),
		"udp_relay_mode":     firstNonEmpty(q.Get("udp_relay_mode"), q.Get("udp-relay-mode"), "native"),
		"tls":                tls,
	}
	return &Node{Name: name, Outbound: ob}, nil
}

// ---- AnyTLS ----

func parseAnyTLS(raw string) (*Node, error) {
	u, name, err := parseURL(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	ob := map[string]any{
		"type":        "anytls",
		"server":      host,
		"server_port": port,
		"password":    u.User.Username(),
		"tls":         buildTLS(u.Query(), host),
	}
	return &Node{Name: name, Outbound: ob}, nil
}

// ---- SOCKS / HTTP ----

func parseSocks(raw string) (*Node, error) {
	u, name, err := parseURL(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	ob := map[string]any{
		"type":        "socks",
		"server":      host,
		"server_port": port,
		"version":     "5",
	}
	switch u.Scheme {
	case "socks4":
		ob["version"] = "4"
	case "socks4a":
		ob["version"] = "4a"
	}
	if u.User != nil {
		user := u.User.Username()
		pass, hasPass := u.User.Password()
		// v2rayN style: socks://base64(user:pass)@host:port
		if !hasPass {
			if decoded, err := decodeBase64(user); err == nil && strings.Contains(decoded, ":") {
				user, pass, _ = strings.Cut(decoded, ":")
			}
		}
		if user != "" {
			ob["username"] = user
			ob["password"] = pass
		}
	}
	return &Node{Name: name, Outbound: ob}, nil
}

func parseHTTP(raw string) (*Node, error) {
	u, name, err := parseURL(raw)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	if host == "" {
		return nil, errors.New(i18n.T("缺少服务器地址"))
	}
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if u.Port() != "" {
		if port, err = parsePort(u.Port()); err != nil {
			return nil, err
		}
	}
	ob := map[string]any{
		"type":        "http",
		"server":      host,
		"server_port": port,
	}
	if u.User != nil && u.User.Username() != "" {
		ob["username"] = u.User.Username()
		ob["password"], _ = u.User.Password()
	}
	if u.Scheme == "https" {
		ob["tls"] = buildTLS(u.Query(), host)
	}
	return &Node{Name: name, Outbound: ob}, nil
}
