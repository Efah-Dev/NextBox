package config

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xtaci/smux"

	"nextbox/internal/link"
	"nextbox/internal/store"
)

const gostClash = `mixed-port: 7890
proxies:
  - { name: 'US 02', type: ss, server: gost.example, port: 80, cipher: 2022-blake3-aes-128-gcm, password: 'MTIzNDU2Nzg5MDEyMzQ1Ng==', plugin: gost-plugin, plugin-opts: { mode: websocket, host: cdn.example, path: /ws, tls: false, mux: true }, udp: true, udp-over-tcp: true, udp-over-tcp-version: 2 }
  - { name: plain, type: trojan, server: t.example, port: 443, password: p }
`

func gostProfiles(t *testing.T) []store.Profile {
	nodes, skipped, err := link.ParseContent(gostClash)
	if err != nil || len(nodes) != 2 || len(skipped) != 0 {
		t.Fatalf("parse: %v %d %v", err, len(nodes), skipped)
	}
	var list []store.Profile
	for i, n := range nodes {
		list = append(list, store.Profile{ID: int64(i + 1), GroupID: 1, Name: n.Name, Type: n.Type(), Outbound: n.Outbound})
	}
	return list
}

func findOutbound(cfg map[string]any, tag string) map[string]any {
	for _, ob := range cfg["outbounds"].([]any) {
		if m := ob.(map[string]any); m["tag"] == tag {
			return m
		}
	}
	return nil
}

// A gost-plugin node dials a local relay, which comes back through the
// gost-in SOCKS inbound and goes out direct; other nodes are untouched.
func TestGostNode(t *testing.T) {
	list := gostProfiles(t)
	in := Input{Settings: store.DefaultSettings(), Node: &list[0], Nodes: list, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: t.TempDir(), SkipMissingRuleSets: true}
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	ss := findOutbound(cfg, "US 02")
	if ss["server"] != "127.0.0.1" || ss["server_port"] == 80 || ss["gost"] != nil || ss["type"] != "shadowsocks" {
		t.Errorf("gost node: %v", ss)
	}
	if uot, _ := ss["udp_over_tcp"].(map[string]any); uot["version"] != 2 {
		t.Errorf("udp over tcp: %v", ss["udp_over_tcp"])
	}
	if tr := findOutbound(cfg, "plain"); tr["server"] != "t.example" {
		t.Errorf("plain node changed: %v", tr)
	}
	var via map[string]any
	for _, ib := range cfg["inbounds"].([]any) {
		if m := ib.(map[string]any); m["tag"] == TagGostIn {
			via = m
		}
	}
	if via == nil || via["listen"] != "127.0.0.1" || len(via["users"].([]any)) != 1 {
		t.Fatalf("via inbound: %v", via)
	}
	first := cfg["route"].(map[string]any)["rules"].([]any)[0].(map[string]any)
	if first["outbound"] != TagDirect || first["inbound"].([]string)[0] != TagGostIn {
		t.Errorf("first rule: %v", first)
	}
	check(t, singBox(t), cfg)

	// the latency test config gets the same
	test, err := BuildTest(store.DefaultSettings(), list, nil, nil, "127.0.0.1:9090", "x", 20000)
	if err != nil {
		t.Fatal(err)
	}
	if ob := findOutbound(test, TestTag(1)); ob["server"] != "127.0.0.1" {
		t.Errorf("test config gost node: %v", ob)
	}
	check(t, singBox(t), test)
}

// As the second hop of a chain proxy, the relay goes out through the first.
func TestGostInChain(t *testing.T) {
	list := gostProfiles(t)
	chain := store.Profile{ID: 9, Name: "chain", Type: store.TypeChain, Chain: []int64{2, 1}}
	member := func(id int64) (store.Profile, bool) {
		for _, p := range list {
			if p.ID == id {
				return p, true
			}
		}
		return store.Profile{}, false
	}
	in := Input{Settings: store.DefaultSettings(), Node: &chain, Nodes: []store.Profile{chain}, Member: member, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: t.TempDir(), SkipMissingRuleSets: true}
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	exit := findOutbound(cfg, "chain")
	if exit["server"] != "127.0.0.1" || exit["detour"] != nil {
		t.Errorf("exit hop: %v", exit)
	}
	b, _ := json.Marshal(cfg["route"].(map[string]any)["rules"].([]any)[:2])
	if !strings.Contains(string(b), `"auth_user":["gost0"],"inbound":["gost-in"],"outbound":"chain / 1"`) {
		t.Errorf("relay rules: %s", b)
	}
	check(t, singBox(t), cfg)
}

// The whole way, with real sing-box on both ends: client core → relay →
// back into the core's gost-in inbound → direct → a gost-plugin server
// (WebSocket + smux) → a sing-box Shadowsocks 2022 server → a web server.
func TestGostEndToEnd(t *testing.T) {
	bin := singBox(t)
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello through gost")
	}))
	defer web.Close()

	const key = "MTIzNDU2Nzg5MDEyMzQ1Ng=="
	ssPort := freePort(t)
	stopSS := runCore(t, bin, map[string]any{
		"log": map[string]any{"level": "error"},
		"inbounds": []any{map[string]any{"type": "shadowsocks", "listen": "127.0.0.1", "listen_port": ssPort,
			"method": "2022-blake3-aes-128-gcm", "password": key}},
		"outbounds": []any{map[string]any{"type": "direct"}},
	}, ssPort)
	defer stopSS()

	var hosts atomic.Value
	up := websocket.Upgrader{}
	gostSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hosts.Store(r.Host + r.URL.Path)
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c := &websocketStream{Conn: ws}
		cfg := smux.DefaultConfig()
		cfg.KeepAliveDisabled = true
		sess, err := smux.Server(c, cfg)
		if err != nil {
			return
		}
		defer sess.Close()
		st, err := sess.AcceptStream()
		if err != nil {
			return
		}
		ss, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(ssPort))
		if err != nil {
			return
		}
		go io.Copy(ss, st)
		io.Copy(st, ss)
		ss.Close()
	}))
	defer gostSrv.Close()
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(gostSrv.URL, "http://"))

	nodes, _, err := link.ParseContent("mixed-port: 7890\nproxies:\n  - { name: g, type: ss, server: " + host + ", port: " + port +
		", cipher: 2022-blake3-aes-128-gcm, password: '" + key + "', plugin: gost-plugin, plugin-opts: { mode: websocket, host: cdn.example, path: /ws, mux: true } }\n")
	if err != nil {
		t.Fatal(err)
	}
	p := store.Profile{ID: 1, Name: "g", Type: nodes[0].Type(), Outbound: nodes[0].Outbound}
	testPort := freePort(t)
	cfg, err := BuildTest(store.DefaultSettings(), []store.Profile{p}, nil, nil, "127.0.0.1:"+strconv.Itoa(freePort(t)), "secret", testPort)
	if err != nil {
		t.Fatal(err)
	}
	stop := runCore(t, bin, cfg, testPort)
	defer stop()

	proxy := &url.URL{Scheme: "http", User: url.UserPassword(TestUser(1), "secret"), Host: "127.0.0.1:" + strconv.Itoa(testPort)}
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
	for i := 0; i < 3; i++ { // a new relay connection each time
		resp, err := client.Get(web.URL)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(b) != "hello through gost" {
			t.Fatalf("got %q", b)
		}
		client.CloseIdleConnections()
	}
	if hosts.Load() != "cdn.example/ws" {
		t.Errorf("gost server saw %v", hosts.Load())
	}
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// runCore runs sing-box with cfg until the returned function is called,
// once it listens on port.
func runCore(t *testing.T, bin string, cfg map[string]any, port int) func() {
	t.Helper()
	b, _ := json.Marshal(cfg)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "c.json"), b, 0o644)
	cmd := exec.Command(bin, "run", "-c", "c.json")
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if c, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port)); err == nil {
			c.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return func() { cmd.Process.Kill(); cmd.Wait() }
}

// websocketStream reads and writes a WebSocket as a byte stream.
type websocketStream struct {
	*websocket.Conn
	r io.Reader
}

func (c *websocketStream) Read(b []byte) (int, error) {
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
			if n == 0 {
				continue
			}
			err = nil
		}
		return n, err
	}
}

func (c *websocketStream) Write(b []byte) (int, error) {
	return len(b), c.WriteMessage(websocket.BinaryMessage, b)
}

// Without gost-plugin nodes nothing is added.
func TestNoGostNoRelay(t *testing.T) {
	list := gostProfiles(t)[1:]
	in := Input{Settings: store.DefaultSettings(), Node: &list[0], Nodes: list, ClashAPI: "127.0.0.1:9090", Secret: "x", RuleSetDir: t.TempDir(), SkipMissingRuleSets: true}
	cfg, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(cfg)
	if strings.Contains(string(b), TagGostIn) {
		t.Errorf("relay inbound in a config without gost nodes")
	}
}
