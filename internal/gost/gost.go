// Package gost carries Shadowsocks over the transport of Clash.Meta's
// gost-plugin, which sing-box does not have: a WebSocket (optionally over
// TLS) with, by default, one smux stream inside per connection.
//
// A Relay listens on 127.0.0.1; the node's shadowsocks outbound connects to
// it instead of the server, and the Shadowsocks encryption stays end to end
// in sing-box. The relay does not dial the server itself: it goes through a
// SOCKS inbound of the same sing-box instance ("via"), so that sing-box's own
// outbound (direct, or the hop before it in a chain) carries the connection,
// with its DNS, its interface binding and, on Android, the VPN protection.
package gost

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xtaci/smux"
	"golang.org/x/net/proxy"
)

// Options are the gost-plugin settings of a node, kept in its outbound as
// the "gost" field (see FromOutbound).
type Options struct {
	Host     string            // Host header and default TLS server name
	Path     string            // WebSocket path
	Mux      bool              // one smux stream inside the WebSocket
	TLS      bool              // wss
	SNI      string            // TLS server name, when not Host
	Insecure bool              // skip certificate verification
	Headers  map[string]string // extra request headers
}

// FromOutbound reads the "gost" field of an outbound; ok is false when it
// has none.
func FromOutbound(ob map[string]any) (o Options, ok bool) {
	m, ok := ob["gost"].(map[string]any)
	if !ok {
		return o, false
	}
	str := func(k string) string { s, _ := m[k].(string); return s }
	flag := func(k string) bool { b, _ := m[k].(bool); return b }
	o = Options{Host: str("host"), Path: str("path"), Mux: flag("mux"), TLS: flag("tls"), SNI: str("sni"), Insecure: flag("insecure")}
	if h, ok := m["headers"].(map[string]any); ok {
		o.Headers = map[string]string{}
		for k, v := range h {
			if s, ok := v.(string); ok {
				o.Headers[k] = s
			}
		}
	}
	return o, true
}

// Session is the set of relays of one sing-box config. Its relays reach
// out through the config's SOCKS inbound on 127.0.0.1:ViaPort, each with
// its own user so that a route rule can pick its way out.
type Session struct {
	ViaPort  int
	Password string

	mu      sync.Mutex
	relays  []*relay
	closed  bool
	created time.Time
	misses  int // janitor checks in a row that found the via port closed
}

// NewSession picks the via port and password for a new config.
func NewSession() (*Session, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	b := make([]byte, 16)
	rand.Read(b)
	s := &Session{ViaPort: port, Password: hex.EncodeToString(b), created: time.Now()}
	track(s)
	return s, nil
}

// User is the SOCKS username of the session's n-th relay (from 0).
func User(n int) string { return "gost" + strconv.Itoa(n) }

// Add starts a relay to target ("host:port") and returns its local port and
// SOCKS user.
func (s *Session) Add(target string, o Options) (port int, user string, err error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		l.Close()
		return 0, "", errors.New("gost: session closed")
	}
	user = User(len(s.relays))
	via := "127.0.0.1:" + strconv.Itoa(s.ViaPort)
	r := &relay{ln: l, target: target, opts: o, via: via, auth: &proxy.Auth{User: user, Password: s.Password}, conns: map[net.Conn]bool{}}
	s.relays = append(s.relays, r)
	go r.serve()
	return l.Addr().(*net.TCPAddr).Port, user, nil
}

// Close stops the relays and their connections.
func (s *Session) Close() {
	s.mu.Lock()
	relays := s.relays
	s.relays, s.closed = nil, true
	s.mu.Unlock()
	for _, r := range relays {
		r.close()
	}
}

// ---- cleanup ----

// A session lives as long as its sing-box instance: nothing has to close
// it. Once its via port has stopped answering for a while, the instance is
// gone and the janitor closes the session.
var (
	sessionsMu sync.Mutex
	sessions   []*Session
	janitorOn  bool
)

const (
	janitorEvery = 20 * time.Second
	startupGrace = 90 * time.Second // the core may start well after the config is built
)

func track(s *Session) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	sessions = append(sessions, s)
	if !janitorOn {
		janitorOn = true
		go janitor()
	}
}

func janitor() {
	for {
		time.Sleep(janitorEvery)
		sessionsMu.Lock()
		list := append([]*Session(nil), sessions...)
		sessionsMu.Unlock()
		var gone []*Session
		for _, s := range list {
			if time.Since(s.created) < startupGrace {
				continue
			}
			c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(s.ViaPort), time.Second)
			if err == nil {
				c.Close()
				s.misses = 0
				continue
			}
			if s.misses++; s.misses >= 2 {
				s.Close()
				gone = append(gone, s)
			}
		}
		sessionsMu.Lock()
		kept := sessions[:0]
		for _, s := range sessions {
			if !contains(gone, s) {
				kept = append(kept, s)
			}
		}
		sessions = kept
		sessionsMu.Unlock()
	}
}

func contains(list []*Session, s *Session) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ---- relay ----

type relay struct {
	ln     net.Listener
	target string
	opts   Options
	via    string
	auth   *proxy.Auth

	mu    sync.Mutex
	conns map[net.Conn]bool
	done  bool
}

func (r *relay) serve() {
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		if !r.hold(c) {
			c.Close()
			return
		}
		go func() {
			defer r.release(c)
			r.handle(c)
		}()
	}
}

func (r *relay) hold(c net.Conn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done {
		return false
	}
	r.conns[c] = true
	return true
}

func (r *relay) release(c net.Conn) {
	c.Close()
	r.mu.Lock()
	delete(r.conns, c)
	r.mu.Unlock()
}

func (r *relay) close() {
	r.ln.Close()
	r.mu.Lock()
	r.done = true
	for c := range r.conns {
		c.Close()
	}
	r.mu.Unlock()
}

const handshakeTimeout = 15 * time.Second

func (r *relay) handle(local net.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	up, err := Dial(ctx, r.via, r.auth, r.target, r.opts)
	cancel()
	if err != nil {
		return
	}
	defer up.Close()
	pipe(local, up)
}

// Dial opens one connection to target through the gost-plugin transport,
// reaching target through the SOCKS5 proxy at via (direct when via is "").
func Dial(ctx context.Context, via string, auth *proxy.Auth, target string, o Options) (net.Conn, error) {
	netDial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		if via == "" {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", target)
		}
		d, err := proxy.SOCKS5("tcp", via, auth, &net.Dialer{})
		if err != nil {
			return nil, err
		}
		return d.(proxy.ContextDialer).DialContext(ctx, "tcp", target)
	}
	host := o.Host
	if host == "" {
		host, _, _ = net.SplitHostPort(target)
	}
	path := o.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: "ws", Host: host, Path: path}
	header := http.Header{}
	for k, v := range o.Headers {
		header.Set(k, v) // a "Host" header replaces the Host
	}
	d := websocket.Dialer{NetDialContext: netDial, HandshakeTimeout: handshakeTimeout, ReadBufferSize: 32 << 10, WriteBufferSize: 32 << 10}
	if o.TLS {
		u.Scheme = "wss"
		sni := o.SNI
		if sni == "" {
			sni = host
			if h := header.Get("Host"); h != "" {
				sni = h
			}
		}
		d.TLSClientConfig = &tls.Config{ServerName: sni, InsecureSkipVerify: o.Insecure, NextProtos: []string{"http/1.1"}}
	}
	ws, _, err := d.DialContext(ctx, u.String(), header)
	if err != nil {
		return nil, err
	}
	conn := &wsConn{Conn: ws}
	if !o.Mux {
		return conn, nil
	}
	cfg := smux.DefaultConfig()
	cfg.KeepAliveDisabled = true
	sess, err := smux.Client(conn, cfg)
	if err != nil {
		conn.Close()
		return nil, err
	}
	st, err := sess.OpenStream()
	if err != nil {
		sess.Close()
		return nil, err
	}
	return &muxConn{Stream: st, sess: sess}, nil
}

// pipe copies both ways until one side ends, then closes both.
func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		io.Copy(dst, src)
		done <- struct{}{}
	}
	go cp(a, b)
	go cp(b, a)
	<-done
	a.Close()
	b.Close()
	<-done
}

// wsConn is a WebSocket as a byte stream of binary messages.
type wsConn struct {
	*websocket.Conn
	r  io.Reader
	wm sync.Mutex
}

func (c *wsConn) Read(b []byte) (int, error) {
	for {
		if c.r == nil {
			_, r, err := c.NextReader()
			if err != nil {
				return 0, err
			}
			c.r = r
		}
		n, err := c.r.Read(b)
		if err == io.EOF {
			c.r = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *wsConn) Write(b []byte) (int, error) {
	c.wm.Lock()
	defer c.wm.Unlock()
	if err := c.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wsConn) SetDeadline(t time.Time) error {
	c.SetReadDeadline(t)
	return c.SetWriteDeadline(t)
}

// muxConn is the one smux stream of a connection; closing it closes the
// session (and so the WebSocket) too.
type muxConn struct {
	*smux.Stream
	sess *smux.Session
}

func (c *muxConn) Close() error {
	err := c.Stream.Close()
	c.sess.Close()
	return err
}
