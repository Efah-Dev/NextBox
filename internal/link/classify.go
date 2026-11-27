package link

import (
	"encoding/base64"
	"net/url"
	"strings"
)

// Classified is what a pasted, dropped, scanned or shared text holds.
type Classified struct {
	// Kind is "subscription" (URL holds it), "nodes" (Count share links or
	// a subscription's content) or "none".
	Kind  string `json:"kind"`
	URL   string `json:"url,omitempty"`
	Name  string `json:"name,omitempty"` // the link's name, else the host
	Count int    `json:"count,omitempty"`
	// Named is set when the link itself names the subscription (a #name,
	// name=…), so Name is not just the host.
	Named bool `json:"named,omitempty"`
	// LikelyProxy marks an http(s) address that looks more like an HTTP
	// proxy ("http://user:pass@1.2.3.4:8080") than a subscription; the apps
	// still offer both.
	LikelyProxy bool `json:"likelyProxy,omitempty"`
}

// Classify tells what a text holds: a subscription address, nodes, or nothing
// NextBox can use. Subscription addresses are also read from one-click import
// links (clash://install-config?url=…, sing-box://import-remote-profile?url=…,
// sub://<base64>). A single http(s) line is a subscription, not an HTTP proxy
// node; LikelyProxy tells when it may be meant as one.
func Classify(text string) Classified {
	text = strings.TrimSpace(strings.TrimPrefix(text, string(rune(0xFEFF))))
	res := Classified{Kind: "none"}
	if text == "" {
		return res
	}
	if !strings.ContainsAny(text, "\n\r") {
		if u, name, named, ok := subscriptionURL(text); ok {
			res.Kind, res.URL, res.Name, res.Named = "subscription", u, name, named
			res.LikelyProxy = likelyHTTPProxy(u)
			return res
		}
	}
	if nodes, _, err := ParseContent(text); err == nil && len(nodes) > 0 {
		res.Kind, res.Count = "nodes", len(nodes)
	}
	return res
}

// subscriptionURL reads a subscription address and its name (the host when
// the link has none; named tells which).
func subscriptionURL(s string) (target, name string, named, ok bool) {
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
		u, err := url.Parse(s)
		if err != nil || u.Host == "" {
			return "", "", false, false
		}
		// "https://host/sub#Name": the fragment is never sent to the server
		frag := u.Fragment
		if frag != "" {
			u.Fragment, u.RawFragment = "", ""
			s = u.String()
		}
		name, named = nameFromURL(u, frag)
		return s, name, named, true
	case strings.HasPrefix(lower, "clash://") || strings.HasPrefix(lower, "clashmeta://") ||
		strings.HasPrefix(lower, "sing-box://") || strings.HasPrefix(lower, "hiddify://") ||
		strings.HasPrefix(lower, "mihomo://") || strings.HasPrefix(lower, "stash://"):
		u, err := url.Parse(s)
		if err != nil {
			return "", "", false, false
		}
		target = u.Query().Get("url")
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			return "", "", false, false
		}
		n := u.Query().Get("name")
		if n == "" {
			n = u.Fragment
		}
		tu, err := url.Parse(target)
		if err != nil || tu.Host == "" {
			return "", "", false, false
		}
		name, named = nameFromURL(tu, n)
		return target, name, named, true
	case strings.HasPrefix(lower, "sub://"):
		// Shadowrocket: sub://base64(url)#name
		body, frag, _ := strings.Cut(s[len("sub://"):], "#")
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			if b, err := enc.DecodeString(body); err == nil {
				target = strings.TrimSpace(string(b))
				if tu, err := url.Parse(target); err == nil && tu.Host != "" && strings.HasPrefix(tu.Scheme, "http") {
					n, _ := url.PathUnescape(frag)
					name, named = nameFromURL(tu, n)
					return target, name, named, true
				}
			}
		}
	}
	return "", "", false, false
}

func nameFromURL(u *url.URL, name string) (string, bool) {
	if name = strings.TrimSpace(name); name != "" {
		return name, true
	}
	return strings.TrimPrefix(u.Hostname(), "www."), false
}

// likelyHTTPProxy: credentials or a port, and nothing after it.
func likelyHTTPProxy(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	bare := (u.Path == "" || u.Path == "/") && u.RawQuery == ""
	return bare && (u.User != nil || u.Port() != "")
}
