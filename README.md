<p align="center">
  <img src="docs/logo.png" width="128" alt="NextBox logo">
</p>

<h1 align="center">NextBox</h1>

<p align="center">
  A clean, modern Windows proxy client powered by sing-box.<br>
  Based on NekoBox for PC (nekoray).
</p>

<p align="center">
  <b>English</b> · <a href="README_CN.md">简体中文</a>
</p>

---

NextBox is a desktop GUI for the [sing-box](https://github.com/SagerNet/sing-box) proxy core. It continues
the feature set of the no-longer-maintained [NekoBox for PC (nekoray)](https://github.com/MatsuriDayo/nekoray)
with a rewritten launcher-style interface, the current sing-box configuration format, and a system tray.

> NextBox is an independent project. It is **not affiliated with or endorsed by** SagerNet / sing-box
> or MatsuriDayo / nekoray.

## Features

| | |
|---|---|
| **Protocols** | Shadowsocks, VMess, VLESS (incl. Reality / Vision), Trojan, Hysteria2, TUIC, AnyTLS, SOCKS, HTTP |
| **Subscriptions** | Base64 link lists, Clash / Clash.Meta YAML, sing-box JSON; shows used traffic and expiry date |
| **Node editing** | Form editor for common fields, or edit the full sing-box outbound JSON; export share links |
| **Latency test** | URL test (round trip on an established connection, same method as NekoBox) and TCP test |
| **Routing** | Rule-based (bypass mainland China) / Global / Direct; visual rule editor (site categories, domains, IPs, programs); ad blocking |
| **System proxy** | Set automatically on connect; restored on disconnect, exit, core crash and Windows logoff |
| **TUN mode** | Captures all traffic (requires administrator; one-click "restart as administrator") |
| **DNS** | Remote DNS through the proxy, direct DNS locally; UDP / TCP / DoT / DoQ / DoH / DoH3 |
| **Tray** | Keeps running in the tray after closing the window; start / stop, switch node, route mode and TUN from the menu. Green dot = system proxy, red dot = TUN |
| **Connections & logs** | Live connection list (target, process, rule, speed); log viewer with filters; optional log files with size limit and retention |
| **Look & feel** | Light / dark theme, custom accent color (pink by default), Noto Sans SC font |
| **Languages** | 简体中文, 繁體中文 and English, switchable in Settings → Appearance |

## Download

Get `NextBox-v1.0.0-windows-amd64.zip` from the [Releases](../../releases) page.

**Requirements:** Windows 10 / 11, 64-bit, with Microsoft Edge WebView2 Runtime (preinstalled on Windows 11
and on up-to-date Windows 10).

1. Extract the zip to a folder you can write to, e.g. `D:\Apps\NextBox`.
   Do **not** use `C:\Program Files`: NextBox is portable and stores its data next to the program.
2. Run `NextBox.exe`. The release is not code-signed, so Windows SmartScreen may warn on first launch;
   choose *More info → Run anyway*.

## Quick start

1. **Nodes** → *Add subscription* (or copy share links and click *Import*).
2. Click a node to select it, or run a latency test first.
3. **Home** → click **Connect**. The system proxy is set automatically.
4. Closing the window keeps NextBox in the tray; right-click the paw icon for quick actions, or choose *Exit*.

## sing-box core

The release package ships `core/sing-box.exe`, built from unmodified official source:

| | |
|---|---|
| Version | **sing-box 1.14.4** |
| Branch | **`stable`** (official stable release, tag [`v1.14.4`](https://github.com/SagerNet/sing-box/tree/v1.14.4)) |
| Built with | Go 1.27.3, `windows/amd64`, CGO disabled |
| Build tags | `with_gvisor, with_quic, with_dhcp, with_wireguard, with_utls, with_acme, with_clash_api, with_tailscale, with_ccm, with_ocm, with_cloudflared, with_usbip, with_openvpn, with_openconnect, badlinkname, tfogo_checklinkname0` |

These are the official Windows release tags, except `with_naive_outbound` / `with_purego`, which need an
extra cronet DLL. The core version and tags are also shown in **Settings → About**.

NextBox generates configurations in the sing-box **1.12+** format, so you can replace the core with any
newer official release by dropping another `sing-box.exe` into `core/`.

## Where data is stored

Everything lives next to `NextBox.exe` (portable):

| Path | Contents |
|---|---|
| `config/profiles.json` | Groups, subscriptions and nodes |
| `config/settings.json` | Settings |
| `config/rule-set/` | Downloaded geosite / geoip rule-sets |
| `config/logs/` | Log files (only when *Log to file* is enabled) |

To move NextBox to another PC, copy the whole folder.

## FAQ

**TUN mode can't be turned on.** It needs administrator rights. Use *Restart as administrator* (Home page
or Settings → TUN).

**Connecting in rule mode fails with "rule-set download failed".** The geosite / geoip rule-sets are
downloaded on first use. Pick another mirror in Settings → Routing → *Rule-set mirror*.

**A node is skipped on import.** The import message and the Logs page say why. Xray-only features such as
the XHTTP transport and VLESS encryption are not supported by sing-box.

**What does "Block ads" do?** It blocks the domains in the community-maintained `geosite-category-ads-all`
list, for both connections and DNS. Ads served from the same domain as the content cannot be blocked.

**Port 2030 is already in use.** Change the mixed (HTTP + SOCKS5) port in Settings → Inbound.

## Building from source

Requirements: [Go](https://go.dev/dl/) 1.25+, [Node.js](https://nodejs.org/) LTS, and the Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Build:

```bash
go mod tidy
wails build
```

The app icons (`build/appicon.png`, `build/windows/icon.ico`) and `docs/logo.png` are committed. After
changing `internal/icon`, regenerate them with `go run ./cmd/genicon`.

Build the sing-box core (expects the sing-box source in `../sing-box-stable`, output goes to
`build/bin/core/sing-box.exe`):

```bash
powershell -ExecutionPolicy Bypass -File scripts/build-core.ps1
```

Run tests (the config and latency tests use the core built above):

```bash
go test ./internal/...
```

For live development use `wails dev`.

### Project layout

```
main.go, app.go, support.go   Wails entry point and the API exposed to the UI
tray.go                       System tray
cmd/genicon/                  Generates the application icons
internal/
  store/     Data model and JSON persistence
  link/      Share-link parsing / export, subscription formats
  sub/       Subscription download
  config/    sing-box 1.12+ configuration builder
  core/      Core process, Clash API client, latency tests
  icon/      Pink paw icon renderer (app + tray)
  i18n/      Backend translations (shares the dictionaries in frontend/src/i18n)
  logfile/   Log files with rotation and retention
  sys/       System proxy, elevation, autostart (Windows)
frontend/src/
  views/        Home, Nodes, Connections, Logs, Settings
  components/   Title bar, cards, dialogs, rule editor, …
  i18n/         en.json (Chinese → English), zh-TW.json (Simplified → Traditional)
scripts/build-core.ps1   Builds sing-box from source
```

## Roadmap

- Proxy chains, load-balancing / auto-select groups
- QR code import / export, global hotkeys
- Scheduled subscription updates, update checker
- Linux / macOS support

## Credits

- [MatsuriDayo/nekoray](https://github.com/MatsuriDayo/nekoray): the project NextBox is based on
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box): proxy core
- [SagerNet/sing-geosite](https://github.com/SagerNet/sing-geosite), [sing-geoip](https://github.com/SagerNet/sing-geoip): rule-sets
- [Wails](https://wails.io), [Vue](https://vuejs.org), [fyne systray](https://github.com/fyne-io/systray), [Noto Sans SC](https://github.com/notofonts/noto-cjk)
- Developed with the help of Claude Opus 5.5 (Anthropic)

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for licenses.

## License

[GPL-3.0-or-later](LICENSE).

Use this software in accordance with the laws and regulations of your country or region.
