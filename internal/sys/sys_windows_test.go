//go:build windows

package sys

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// The round trip runs against a made-up connection name, so the settings of
// the connection the machine really uses are not changed. WinINet still
// mirrors every write into the legacy registry values for a moment (see
// resyncDefault), which is why the test only runs when asked to:
//
//	NEXTBOX_TEST_SYSPROXY=1 go test ./internal/sys
func TestConnProxyRoundTrip(t *testing.T) {
	if os.Getenv("NEXTBOX_TEST_SYSPROXY") == "" {
		t.Skip("set NEXTBOX_TEST_SYSPROXY=1 to run")
	}
	const name = "NextBox unit test"
	conn, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	legacy := func() (enabled uint64, server string) {
		k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings, registry.QUERY_VALUE)
		if err != nil {
			return 0, ""
		}
		defer k.Close()
		enabled, _, _ = k.GetIntegerValue("ProxyEnable")
		server, _, _ = k.GetStringValue("ProxyServer")
		return enabled, server
	}
	wantEnabled, wantServer := legacy()
	t.Cleanup(func() {
		k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings+`\Connections`, registry.SET_VALUE)
		if err == nil {
			k.DeleteValue(name)
			k.Close()
		}
		if err := resyncDefault(); err != nil {
			t.Errorf("resync: %v", err)
		}
		if enabled, server := legacy(); enabled != wantEnabled || server != wantServer {
			t.Errorf("legacy values not restored: ProxyEnable=%d ProxyServer=%q, want %d %q",
				enabled, server, wantEnabled, wantServer)
		}
	})

	if err := setConnProxy(conn, true, "127.0.0.1:1", "localhost;<local>"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := connServer(conn); got != "127.0.0.1:1" {
		t.Fatalf("server after set = %q", got)
	}
	if flags := connFlags(conn); flags&proxyTypeProxy == 0 || flags&proxyTypeDirect == 0 {
		t.Fatalf("flags after set = %#x", flags)
	}

	if err := setConnProxy(conn, false, "", ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := connServer(conn); got != "" {
		t.Fatalf("server after clear = %q, want it removed", got)
	}
	if flags := connFlags(conn); flags&proxyTypeProxy != 0 {
		t.Fatalf("flags after clear = %#x", flags)
	}
}

// What WinINet reports for the LAN connection has to agree with the legacy
// registry value that ClearSystemProxy compares against.
func TestConnServerMatchesRegistry(t *testing.T) {
	want := ""
	if k, err := registry.OpenKey(registry.CURRENT_USER, internetSettings, registry.QUERY_VALUE); err == nil {
		want, _, _ = k.GetStringValue("ProxyServer")
		k.Close()
	}
	if got := connServer(nil); got != want {
		t.Fatalf("connServer(nil) = %q, registry has %q", got, want)
	}
}

func TestRasConnections(t *testing.T) {
	for _, conn := range rasConnections() {
		t.Logf("dial-up / VPN entry: %s", windows.UTF16PtrToString(conn))
	}
}
