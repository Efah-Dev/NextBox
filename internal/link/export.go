package link

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"nextbox/internal/i18n"
	"strconv"
	"strings"
)

// Export converts a sing-box outbound back to a share link.
func Export(name string, ob map[string]any) (string, error) {
	typ := str(ob, "type")
	server := str(ob, "server")
	port := num(ob, "server_port")
	hostport := net.JoinHostPort(server, strconv.Itoa(port))
	frag := "#" + url.PathEscape(name)

	switch typ {
	case "shadowsocks":
		user := base64.RawURLEncoding.EncodeToString([]byte(str(ob, "method") + ":" + str(ob, "password")))
		q := url.Values{}
		if p := str(ob, "plugin"); p != "" {
			if o := str(ob, "plugin_opts"); o != "" {
				p += ";" + o
			}
			q.Set("plugin", p)
		}
		s := "ss://" + user + "@" + hostport
		if len(q) > 0 {
			s += "/?" + q.Encode()
		}
		return s + frag, nil

	case "vmess":
		v := map[string]any{
			"v": "2", "ps": name, "add": server, "port": strconv.Itoa(port),
			"id": str(ob, "uuid"), "aid": strconv.Itoa(num(ob, "alter_id")),
			"scy": firstNonEmpty(str(ob, "security"), "auto"), "net": "tcp", "type": "none",
		}
		if tr := obj(ob, "transport"); tr != nil {
			network, host, path, _ := exportTransport(tr)
			v["net"], v["host"], v["path"] = network, host, path
			if str(tr, "type") == "http" && obj(ob, "tls") == nil {
				v["net"], v["type"] = "tcp", "http"
			}
		}
		if t := obj(ob, "tls"); t != nil && boolean(t, "enabled") {
			v["tls"] = "tls"
			v["sni"] = str(t, "server_name")
			if alpn := strList(t, "alpn"); len(alpn) > 0 {
				v["alpn"] = strings.Join(alpn, ",")
			}
			if u := obj(t, "utls"); u != nil {
				v["fp"] = str(u, "fingerprint")
			}
		}
		b, _ := json.Marshal(v)
		return "vmess://" + base64.StdEncoding.EncodeToString(b), nil

	case "vless", "trojan":
		q := url.Values{}
		user := str(ob, "uuid")
		if typ == "trojan" {
			user = str(ob, "password")
		} else {
			q.Set("encryption", "none")
			if f := str(ob, "flow"); f != "" {
				q.Set("flow", f)
			}
		}
		exportStream(ob, q)
		return typ + "://" + url.PathEscape(user) + "@" + hostport + "?" + q.Encode() + frag, nil

	case "hysteria2":
		q := url.Values{}
		exportTLS(obj(ob, "tls"), q)
		q.Del("security")
		if o := obj(ob, "obfs"); o != nil {
			q.Set("obfs", str(o, "type"))
			q.Set("obfs-password", str(o, "password"))
		}
		if ports := strList(ob, "server_ports"); len(ports) > 0 {
			for i := range ports {
				ports[i] = strings.Replace(ports[i], ":", "-", 1)
			}
			q.Set("mport", strings.Join(ports, ","))
		}
		return "hysteria2://" + url.PathEscape(str(ob, "password")) + "@" + hostport + "?" + q.Encode() + frag, nil

	case "tuic":
		q := url.Values{}
		exportTLS(obj(ob, "tls"), q)
		q.Del("security")
		q.Set("congestion_control", str(ob, "congestion_control"))
		q.Set("udp_relay_mode", str(ob, "udp_relay_mode"))
		u := url.UserPassword(str(ob, "uuid"), str(ob, "password"))
		return "tuic://" + u.String() + "@" + hostport + "?" + q.Encode() + frag, nil

	case "anytls":
		q := url.Values{}
		exportTLS(obj(ob, "tls"), q)
		q.Del("security")
		return "anytls://" + url.PathEscape(str(ob, "password")) + "@" + hostport + "?" + q.Encode() + frag, nil

	case "socks":
		scheme := "socks5"
		if v := str(ob, "version"); v == "4" || v == "4a" {
			scheme = "socks" + v
		}
		auth := ""
		if user := str(ob, "username"); user != "" {
			auth = url.UserPassword(user, str(ob, "password")).String() + "@"
		}
		return scheme + "://" + auth + hostport + frag, nil

	case "http":
		scheme := "http"
		if t := obj(ob, "tls"); t != nil && boolean(t, "enabled") {
			scheme = "https"
		}
		auth := ""
		if user := str(ob, "username"); user != "" {
			auth = url.UserPassword(user, str(ob, "password")).String() + "@"
		}
		return scheme + "://" + auth + hostport + frag, nil
	}
	return "", fmt.Errorf(i18n.T("暂不支持导出 %s 类型的链接"), typ)
}

func exportStream(ob map[string]any, q url.Values) {
	q.Set("type", "tcp")
	if tr := obj(ob, "transport"); tr != nil {
		network, host, path, service := exportTransport(tr)
		q.Set("type", network)
		if host != "" {
			q.Set("host", host)
		}
		if path != "" {
			q.Set("path", path)
		}
		if service != "" {
			q.Set("serviceName", service)
		}
		if str(tr, "type") == "http" && obj(ob, "tls") == nil {
			q.Set("type", "tcp")
			q.Set("headerType", "http")
		}
	}
	if t := obj(ob, "tls"); t != nil && boolean(t, "enabled") {
		exportTLS(t, q)
	} else {
		q.Set("security", "none")
	}
}

func exportTransport(tr map[string]any) (network, host, path, service string) {
	switch str(tr, "type") {
	case "ws":
		network, path = "ws", str(tr, "path")
		if h := obj(tr, "headers"); h != nil {
			host = str(h, "Host")
		}
		if ed := num(tr, "max_early_data"); ed > 0 {
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			path += sep + "ed=" + strconv.Itoa(ed)
		}
	case "grpc":
		network, service = "grpc", str(tr, "service_name")
		path = service
	case "http":
		network, path = "http", str(tr, "path")
		host = strings.Join(strList(tr, "host"), ",")
	case "httpupgrade":
		network, host, path = "httpupgrade", str(tr, "host"), str(tr, "path")
	case "quic":
		network = "quic"
	default:
		network = "tcp"
	}
	return
}

func exportTLS(t map[string]any, q url.Values) {
	if t == nil || !boolean(t, "enabled") {
		return
	}
	q.Set("security", "tls")
	if sni := str(t, "server_name"); sni != "" {
		q.Set("sni", sni)
	}
	if boolean(t, "insecure") {
		q.Set("allowInsecure", "1")
		q.Set("insecure", "1")
	}
	if alpn := strList(t, "alpn"); len(alpn) > 0 {
		q.Set("alpn", strings.Join(alpn, ","))
	}
	if u := obj(t, "utls"); u != nil && boolean(u, "enabled") {
		q.Set("fp", str(u, "fingerprint"))
	}
	if r := obj(t, "reality"); r != nil && boolean(r, "enabled") {
		q.Set("security", "reality")
		q.Set("pbk", str(r, "public_key"))
		if sid := str(r, "short_id"); sid != "" {
			q.Set("sid", sid)
		}
	}
}

// ---- loose accessors for map[string]any (values may come from JSON) ----

func str(m map[string]any, k string) string {
	switch v := m[k].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	}
	return ""
}

func num(m map[string]any, k string) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func boolean(m map[string]any, k string) bool {
	b, _ := m[k].(bool)
	return b
}

func obj(m map[string]any, k string) map[string]any {
	o, _ := m[k].(map[string]any)
	return o
}

func strList(m map[string]any, k string) []string {
	switch v := m[k].(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}
