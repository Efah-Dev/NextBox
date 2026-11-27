package gost

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xtaci/smux"
)

// gostServer is a gost-plugin server that echoes what it gets.
func gostServer(t *testing.T, mux bool, gotHost *atomic.Value) *httptest.Server {
	up := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		gotHost.Store(r.Host)
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		var c io.ReadWriteCloser = &wsConn{Conn: ws}
		defer c.Close()
		if mux {
			cfg := smux.DefaultConfig()
			cfg.KeepAliveDisabled = true
			s, err := smux.Server(c, cfg)
			if err != nil {
				return
			}
			defer s.Close()
			st, err := s.AcceptStream()
			if err != nil {
				return
			}
			c = st
		}
		io.Copy(c, c)
	}))
}

// socksServer is a minimal SOCKS5 server with user/password auth; it
// records the users and targets it was asked for.
func socksServer(t *testing.T, password string, seen *atomic.Value) net.Listener {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				b := make([]byte, 262)
				io.ReadFull(c, b[:2])
				io.ReadFull(c, b[:b[1]])
				c.Write([]byte{5, 2}) // user/password
				io.ReadFull(c, b[:2])
				user := make([]byte, b[1])
				io.ReadFull(c, user)
				io.ReadFull(c, b[:1])
				pass := make([]byte, b[0])
				io.ReadFull(c, pass)
				if string(pass) != password {
					c.Write([]byte{1, 1})
					return
				}
				c.Write([]byte{1, 0})
				io.ReadFull(c, b[:4])
				var host string
				switch b[3] {
				case 1:
					io.ReadFull(c, b[:4])
					host = net.IP(b[:4]).String()
				case 3:
					io.ReadFull(c, b[:1])
					h := make([]byte, b[0])
					io.ReadFull(c, h)
					host = string(h)
				}
				io.ReadFull(c, b[:2])
				target := net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(b[:2]))))
				seen.Store(string(user) + "@" + target)
				up, err := net.Dial("tcp", target)
				if err != nil {
					c.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
					return
				}
				defer up.Close()
				c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
				go io.Copy(up, c)
				io.Copy(c, up)
			}()
		}
	}()
	return l
}

func TestRelay(t *testing.T) {
	for _, mux := range []bool{true, false} {
		var host, seen atomic.Value
		srv := gostServer(t, mux, &host)
		sess, err := NewSession()
		if err != nil {
			t.Fatal(err)
		}
		socks := socksServer(t, sess.Password, &seen)
		sess.ViaPort = socks.Addr().(*net.TCPAddr).Port // stand-in for sing-box's inbound

		target := strings.TrimPrefix(srv.URL, "http://")
		port, user, err := sess.Add(target, Options{Host: "cdn.example", Path: "ws", Mux: mux})
		if err != nil {
			t.Fatal(err)
		}
		c, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			t.Fatal(err)
		}
		msg := bytes.Repeat([]byte("nextbox "), 20000) // several WebSocket messages
		go c.Write(msg)
		got := make([]byte, len(msg))
		c.SetReadDeadline(time.Now().Add(10 * time.Second))
		if _, err := io.ReadFull(c, got); err != nil || !bytes.Equal(got, msg) {
			t.Fatalf("mux=%v: echo failed: %v", mux, err)
		}
		c.Close()
		if host.Load() != "cdn.example" {
			t.Errorf("Host header %v", host.Load())
		}
		if seen.Load() != user+"@"+target {
			t.Errorf("went out as %v, want %s@%s", seen.Load(), user, target)
		}

		sess.Close()
		if c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), time.Second); err == nil {
			c.Close()
			t.Error("relay still listening after Close")
		}
		socks.Close()
		srv.Close()
	}
}

func TestFromOutbound(t *testing.T) {
	o, ok := FromOutbound(map[string]any{"gost": map[string]any{
		"host": "h", "path": "/p", "mux": true, "tls": true, "insecure": true,
		"headers": map[string]any{"User-Agent": "x"},
	}})
	if !ok || o.Host != "h" || o.Path != "/p" || !o.Mux || !o.TLS || !o.Insecure || o.Headers["User-Agent"] != "x" {
		t.Errorf("%+v", o)
	}
	if _, ok := FromOutbound(map[string]any{"type": "shadowsocks"}); ok {
		t.Error("plain node read as gost")
	}
}
