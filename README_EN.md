<p align="center">
  <img src="docs/logo.png" width="120" alt="NextBox logo">
</p>

<h1 align="center">NextBox</h1>

<p align="center">
  A lightweight, clean and modern Windows proxy client powered by sing-box
</p>

<p align="center">
  <a href="https://github.com/Efah-Dev/NextBox/releases/latest"><img src="https://img.shields.io/github/v/release/Efah-Dev/NextBox?label=Release&color=f472b6" alt="Release"></a>
  <a href="https://github.com/Efah-Dev/NextBox/releases"><img src="https://img.shields.io/github/downloads/Efah-Dev/NextBox/total?label=Downloads&color=8b8cf8" alt="Downloads"></a>
  <a href="https://github.com/Efah-Dev/NextBox/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/Efah-Dev/NextBox/build.yml?branch=main&label=Build" alt="Build"></a>
  <img src="https://img.shields.io/badge/Platform-Windows%2010%20%2F%2011-0078D4" alt="Platform">
  <img src="https://img.shields.io/badge/Go-Wails-00ADD8?logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white" alt="Vue">
  <img src="https://img.shields.io/badge/sing--box-1.14.4-5B6678" alt="sing-box">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-GPL--3.0-blue" alt="License"></a>
</p>

<p align="center">
  <a href="README.md">简体中文</a> · <b>English</b> · <a href="https://nextbox.efah.dev/">Website</a> · <a href="https://github.com/Efah-Dev/NextBox/releases/latest">Download</a> · <a href="https://github.com/Efah-Dev/NextBox-Android">Android</a>
</p>

<p align="center">
  <img src="docs/preview-en.png" alt="NextBox home and nodes pages">
</p>

NextBox is a desktop GUI for the [sing-box](https://github.com/SagerNet/sing-box) proxy core. It continues
the feature set of the no-longer-maintained [NekoBox for PC (nekoray)](https://github.com/MatsuriDayo/nekoray)
with a rewritten launcher-style interface, the current sing-box configuration format, and a system tray.

> NextBox is an independent project. It is **not affiliated with or endorsed by** SagerNet / sing-box
> or MatsuriDayo / nekoray.

## ✨ Features

- 🔌 **Protocols**: Shadowsocks (incl. ShadowTLS), VMess, VLESS (incl. Reality / Vision), Trojan, Hysteria2, TUIC, AnyTLS, NaiveProxy, SOCKS, HTTP
- 📡 **Subscriptions**: Base64 link lists, Clash / Clash.Meta YAML, sing-box JSON; shows used traffic and expiry date; optional automatic updates at an interval you choose
- 🗂️ **Nodes**: form editor for common fields, or edit the full sing-box outbound JSON; drag to reorder; search, sort, bulk delete; delete duplicates in one go (nodes of all groups that connect the same way, one is kept); export share links
- 🚩 **Flags and country search**: a flag in front of every node, by exit IP by default (checked during URL tests) or by server IP; searching "Hong Kong" or "香港" finds HK, 🇭🇰 and Hong Kong nodes alike
- ⛓️ **Chain proxies**: send traffic through 2 to 8 nodes in a row, e.g. an airport node as the entry and your own VPS as the exit; members come from any group, can be reordered by dragging, and the whole chain can be tested before saving
- 🔀 **Switch nodes without dropping connections**: the nodes of a group are loaded into the core together, so switching does not restart it and open connections (a running download, for example) are not cut
- ⚡ **Latency test**: URL test (round trip on an established connection, same method as NekoBox) and TCP test; a bolt button on every node tests just that node; protocols over UDP (Hysteria2, TUIC …) and chain proxies always use a URL test; configurable URL, timeout and concurrency
- 🧭 **Routing**: global (the default, nothing to set up) / rule-based (bypasses mainland China by default) / direct. Rule-based mode has three styles that edit the same rules; the first time, NextBox suggests one by how well you know proxy tools:
  - **Services** (for beginners): 50+ common services (AI, video & music, social, tools, games, China) with their icons; one click sends a service through the current node, direct, blocked or through a policy group
  - **Cards** (for the curious): turn on the scenarios (AI services, streaming, game downloads direct, bypass mainland China), then add rules of your own
  - **Route** (for experts): the order every connection is checked in as one line; ad blocking, the scenarios and your own rules can be dragged into any order; steps that are off can be hidden

  Rules match site categories (several at once), domains, IP regions (several at once), IP ranges and programs, and can be edited as text; testing a website or IP shows which rule matches. All three styles have *Reset*
  - **Rule sets**: put domains, IPs, programs, site categories, IP regions and sing-box rule-sets from the web (.srs / .json URLs) into one named rule set with one target, and add many rules at once; supports pasting in bulk
  - Rules can have names, which the connections page shows for the connections they matched; program rules can be picked from the running programs
  - IP regions work with the system proxy too: domains are resolved with the direct DNS first, so Chinese websites that are not in the domain list still go direct
  - The service catalog splits Steam into *store / community* and *downloads*: downloads go direct through the CDNs in China, the store and community can use the proxy
- 🧩 **Policy groups**: group nodes by region, name keywords or by hand, use the fastest automatically or pick one, and send different sites through different groups (YouTube via Japan, Google via the US); groups by region or keyword pick up new nodes after subscription updates
- 🚫 **Ad blocking**: works in every routing mode
- 📍 **Exit info**: once connected, the home page shows the node's flag, exit IP (masked by default), its network (ASN, e.g. `AS13335 Cloudflare`) and a status light, with a button to copy the exit IP; checked again regularly
- 🖥️ **System proxy**: set automatically on connect, including dial-up (PPPoE) and Windows VPN connections; restored on disconnect, exit and core crash; cleared first thing at Windows logoff / shutdown, and cleared at the next logon if NextBox was killed before it could
- 🚀 **Autostart**: Settings shows whether Windows will really start NextBox; a disabled entry, a moved program and similar problems are explained with a one-click fix; with the NextBox service installed, the service starts NextBox when you log on to Windows, independent of the Run key and of Task Manager; without it, TUN mode starts NextBox through a scheduled task as administrator; optionally connects to the last node after starting with Windows and shows the window when that fails; shows how the last boot went, so a start blocked by security software shows up
- 🛡️ **TUN mode**: captures all traffic. The optional *service mode* needs administrator approval once, when it is installed; after that TUN mode works without running NextBox as administrator. Without the service, use "Restart as administrator"; *TUN exclusions* keep the networks of Tailscale, ZeroTier and the like out of TUN, added in one click from an adapter; optional FakeIP with an editable exclusion list
- 💾 **Backup & encryption**: export / import nodes and subscriptions, routing rules and other settings separately, encrypted with a password (AES-256, the same format as the Android version; Android-only rules and preferences are kept); the node list on disk can be encrypted too (tied to your Windows account, or with a start-up password); the core configuration is no longer written to disk
- 🌍 **DNS**: remote DNS through the proxy, direct DNS locally; UDP / TCP / DoT / DoQ / DoH / DoH3
- 📌 **Tray**: keeps running in the tray after closing the window; start / stop, switch node, route mode and TUN from the menu. Green dot = system proxy, red dot = TUN
- 📜 **Connections & logs**: live connection list (target, process, the name of the rule it matched, exit node, speed); log viewer with filters; optional log files with size limit and retention
- 🔄 **Updates**: check for new versions of NextBox and the sing-box core in Settings → About, compared with GitHub Releases; optional automatic check (off by default). It only notifies, nothing is downloaded
- 🎨 **Look & feel**: light / dark / follow system; adjustable brightness of the light theme and depth of the dark theme (down to black); preset accent colors or a color panel (pink by default); Noto Sans SC font
- 🔍 **Search in settings**: find an option by its name or description (Ctrl+F) and jump straight to it
- 🌐 **Languages**: 简体中文, 繁體中文, English

## 📦 Download

Get `NextBox-<version>-windows-amd64.zip` from the [Releases](https://github.com/Efah-Dev/NextBox/releases/latest) page.

**Requirements**: Windows 10 / 11, 64-bit, with Microsoft Edge WebView2 Runtime (preinstalled on Windows 11
and on up-to-date Windows 10).

1. Extract the zip to a folder you can write to, e.g. `D:\Apps\NextBox`.
   Do **not** use `C:\Program Files`: NextBox is portable and stores its data next to the program.
2. Run `NextBox.exe`. The release is not code-signed, so Windows SmartScreen may warn on first launch;
   choose *More info → Run anyway*.

> Switching nodes without dropping connections, automatic subscription updates, the update check and the
> system proxy for dial-up connections are available from v1.1.0 on; chain proxies, drag to reorder and
> flags with country search from v1.2.0 on; policy groups, the new routing rules, backup & encryption, FakeIP
> and the autostart check from v2.0.0 on; NaiveProxy, the reorderable route and TUN exclusions from v2.1.0 on;
> the service catalog, multiple site categories per rule, deleting duplicate nodes and the ASN of the exit IP
> from v2.2.0 on; rule sets, service mode (TUN without administrator, sturdier autostart), rule names on the connections page,
> picking programs and search in settings from v2.3.0 on.

## 🚀 Quick start

1. **Nodes** → *Add subscription* (or copy share links and click *Import*).
2. Click a node to select it, or run a latency test first.
3. **Home** → click **Connect**. The system proxy is set automatically.
4. Closing the window keeps NextBox in the tray; right-click the paw icon for quick actions, or choose *Exit*.

## 🧩 sing-box core

The release package ships `core/sing-box.exe`, built from **unmodified** official source with only the features NextBox uses (about half the size of the official build):

| | |
|---|---|
| Version | **sing-box 1.14.4** |
| Branch | **`stable`** (official stable release, tag [`v1.14.4`](https://github.com/SagerNet/sing-box/tree/v1.14.4)) |
| Built with | Go 1.27.3, `windows/amd64`, CGO disabled |
| Build tags | `with_gvisor, with_quic, with_dhcp, with_wireguard, with_utls, with_clash_api, with_naive_outbound, with_purego, badlinkname, tfogo_checklinkname0` |

Compared with the official Windows release, Tailscale, cloudflared, OpenVPN / OpenConnect, USB/IP, ACME and the CCM / OCM
services are left out, as NextBox never uses them; every supported protocol (NaiveProxy, Hysteria2, TUIC, WireGuard ...)
is there. `libcronet.dll` next to it is the network library NaiveProxy
needs ([cronet-go](https://github.com/SagerNet/cronet-go), the version the sing-box source pins, as in the official
package). The core version and tags are also shown in **Settings → About**.

NextBox generates configurations in the sing-box **1.12+** format, so you can replace the core with any
newer official release by dropping another `sing-box.exe` into `core/` (with its `libcronet.dll` for NaiveProxy).
With service mode installed, click *Update service* in Settings → TUN mode afterwards so that the service gets the new core too.

## 💾 Where data is stored

Everything lives next to `NextBox.exe` (portable); to move NextBox to another PC, copy the whole folder:

| Path | Contents |
|---|---|
| `config/profiles.json` | Groups, subscriptions, nodes and policy groups (optionally encrypted) |
| `config/settings.json` | Settings |
| `config/rule-set/` | Downloaded geosite / geoip rule-sets |
| `config/logs/` | Log files (only when *Log to file* is enabled) |
| `config/cache/` | Temporary files of latency tests, emptied afterwards |
| `config/autostart.json` | What happened when Windows started NextBox |
| `config/android.json` | Android preferences from an imported backup, exported again with the settings |
| `C:\Program Files\NextBox Service\` | Only with service mode installed: the service, a copy of the core, `service.json` (the accounts that may use it and what to start at logon) and `service.log`. Removed when you uninstall the service in Settings → TUN mode |

With *Encrypt nodes and subscriptions* set to *Tied to your Windows account*, `profiles.json` only opens under
this Windows account on this PC. Export a backup in Settings → Data & backup before moving to another PC.

## ❓ FAQ

**TUN mode can't be turned on.** It needs administrator rights. Use *Restart as administrator* (Home page
or Settings → TUN).

**Does switching nodes interrupt the network?** On connect, the nodes of the current node's group (up to 200)
are loaded into the core together, and switching between them needs no restart: new connections use the new
node, connections that are already open stay on the old one until they end. Switching to a node of another
group or to an edited node, or changing settings such as the route mode, still restarts the core.

**Do rule-sets need a download?** The default *Local built-in* mirror uses the rule-sets shipped inside
the program (bypass China domains and China IPs), so no network is needed. Other rule-sets (ad blocking,
geosite:xxx) are downloaded on first use; a failure or timeout is only reported in the log, the connection
still starts and just skips those rules. To retry, pick a mirror in Settings → Routing rules → *Rule-set mirror*
and click *Update rule-sets*.

**How are the flags decided?** Choose in Settings → Subscriptions & tests → *Node location*. *Exit IP* asks
Cloudflare once through the node during a URL test and shows what websites see, the most accurate option.
*Server IP* looks the server up in the sing-geoip database (a 4 MB download on first use) without going through
the node, but relayed nodes then show where their entry is. Until a node is checked, the flag is guessed from
its name (hover the flag to see why).

**How do I use a chain proxy?** Nodes → the *+* button in the toolbar → *New chain proxy*, then add nodes and
put them in order: the first is the entry, the last the exit. Chain proxies live in manual groups. When a member
is deleted, or renamed by a subscription update, the chain is marked broken and the editor lets you replace
that hop.

**Does a subscription update undo my order?** A manual *Update* asks first and then restores the order of the
subscription; background updates keep your order and append new nodes.

**When are subscriptions updated automatically?** Turn on *Update subscriptions automatically* in
Settings → Subscriptions & tests and pick an interval. Updates only happen while NextBox is running.

**A node is skipped on import.** The import message and the Logs page say why. Xray-only features such as
the XHTTP transport and VLESS encryption are not supported by sing-box.

**What does "Block ads" do?** It blocks the domains in the community-maintained `geosite-category-ads-all`
list, for both connections and DNS. Ads served from the same domain as the content cannot be blocked.

**The update check fails.** While disconnected it reaches GitHub directly, which fails where GitHub is not
reachable; connect first and check again.

**Port 2030 is already in use.** Change the mixed (HTTP + SOCKS5) port in Settings → Proxy port.

**Global or rule-based?** Global sends all traffic through the current node; there is nothing to set up and
your own rules do not apply. Rule-based sends mainland China sites and IPs direct and the rest through the
proxy; turn on scenarios or add rules of your own on top.

**How do I send different sites through different nodes?** Create a group in Settings → Policy groups (e.g.
by region "Japan", fastest automatically), then add a rule in Settings → Routing rules → *My rules*, e.g. site
category YouTube → Japan auto. Policy groups only apply in rule-based mode.

**Can I use a backup on my phone?** A backup from Settings → Data & backup → *Export* imports directly in
NextBox for Android, and Android backups import here. A backup from another system brings its nodes, routing
rules and shared settings; ports, TUN and the like stay as they are.

**Which encryption should I pick?** *Tied to your Windows account* needs no password and autostart keeps
working, the right choice for most people. *Start-up password* asks for the password every time NextBox
opens (when started with Windows it waits on the unlock screen); a forgotten password cannot be recovered.

**TUN mode needs administrator rights.** Settings → TUN mode offers two ways: install *service mode* (one approval;
NextBox then turns TUN on without running as administrator, and autostart gets sturdier), or "Restart as administrator"
each time. The service lives in `C:\Program Files\NextBox Service` and can be uninstalled in the same place. After
updating NextBox, click *Update service* there when it says the versions differ.

**A Chinese website goes through the proxy.** Rule-based routing checks the domain list (geosite-cn) first; other
domains are resolved with the direct DNS and judged by the region of their IP. When a site has no IP in China either
(a CDN abroad, say), add a "domain → direct" rule or put it into a rule set of your own.

**What is a rule set?** *Add rule set* puts rules of several kinds (domains, IPs, programs, site categories, sing-box
rule-sets from the web) together under one name and one target, e.g. "Office network → direct". The connections page
shows which rule set a connection matched. Rule-set URLs must point to sing-box `.srs` or `.json` rule-sets; Clash
rules cannot be used directly.

**Autostart does not work.** Settings → General → *Start with Windows* shows the result of a check. A
disabled entry, a moved program or TUN mode without administrator rights are explained; click the fix button.
The middle of that row shows how the last boot went: "Did not start" usually means security software
blocked the startup entry. When connecting after a start with Windows fails, the window
opens with the reason.

**TUN mode breaks Tailscale / ZeroTier.** Add their networks under Settings → TUN mode → *TUN exclusions*
(Tailscale uses `100.64.0.0/10`, or click *Add from an adapter*); those addresses then bypass NextBox.

**Which of the three rule styles?** *Services* if you'd rather not deal with rules: pick where each common
service goes. *Cards* to add a few rules of your own. *Route* to see and change the order of every step. Switch
any time under *Style*; *Help me choose* next to it opens the suggestion again.

**How do I change the order of the rules?** In the *Route* style, drag the handle of a step. The first step
(LAN) and the last one (everything else) are fixed; everything in between can be moved. Services chosen in the
catalog go to the top, before the scenarios. Rules in a mess? *Reset* at the bottom starts over.

**How do I delete duplicate nodes?** Nodes → *⋯* at the top right → *Delete duplicate nodes*. All groups are
checked; of the nodes whose server, port, password and other connection settings are the same, one is kept. The
list of what is kept and deleted is shown first; the node in use is never deleted, and policy groups and chain
proxies use the kept one instead. Duplicates within one subscription are left out whenever it updates.

**What is FakeIP?** TUN mode only: proxied domains first get a placeholder 198.18.x.x address, which makes
first connections faster and keeps DNS from leaking. After disconnecting, a few programs may keep the
placeholder and fail until restarted; domains with problems can be added to the exclusion list.

## 🛠️ Building from source

Requirements: [Go](https://go.dev/dl/) 1.25+, [Node.js](https://nodejs.org/) LTS, and the Wails CLI:

```bash
git clone https://github.com/Efah-Dev/NextBox.git
cd NextBox
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Build `build/bin/NextBox.exe`:

```bash
wails build
```

Build the sing-box core (expects the sing-box source in `../sing-box-stable`, output goes to
`build/bin/core/sing-box.exe`, together with the `libcronet.dll` NaiveProxy needs):

```bash
powershell -ExecutionPolicy Bypass -File scripts/build-core.ps1
```

Build `build/bin/core/NextBoxService.exe` for service mode (its version comes from `main.go`):

```bash
powershell -ExecutionPolicy Bypass -File scripts/build-service.ps1
```

Run the tests (build the frontend first; the config, latency and node-switching tests use the core built
above and skip themselves without it):

```bash
go test ./...
```

For live development use `wails dev`. Every push and pull request is tested and built on GitHub Actions.

<details>
<summary>Project layout</summary>

```
main.go, app.go, support.go   Wails entry point and the API exposed to the UI
background.go                 Automatic subscription updates, update check
policy.go, backup.go          Policy groups and the rule tester, backups and encryption
exit.go, startup.go           Exit IP and ASN checks; autostart check, cleanup at shutdown
autostart_log.go              Records of every start by Windows
tunexclude.go                 TUN exclusions: checks and the adapter list
tray.go                       System tray
servicemode.go                Installing, removing and checking service mode
cmd/genicon/                  Generates the application icons
cmd/nextbox-service/          NextBoxService.exe (service mode)
internal/
  store/     Data model and JSON persistence
  link/      Share-link parsing / export, subscription formats
  sub/       Subscription download
  config/    sing-box 1.12+ configuration builder
  core/      Core process, Clash API client, latency tests
  update/    Looks up the latest GitHub release
  geo/       Exit IP, IP database, guessing the region from node names
  vault/     Encryption of backups and local data (AES-256-GCM, same format as Android)
  icon/      Pink paw icon renderer (app + tray)
  i18n/      Backend translations (shares the dictionaries in frontend/src/i18n)
  logfile/   Log files with rotation and retention
  sys/       System proxy, elevation, autostart, running programs (Windows)
  service/   The NextBox service: named pipe protocol, runs the core with system rights, starts NextBox at logon
frontend/src/
  views/        Home, Nodes, Connections, Logs, Settings (settings/ holds the sections)
  components/   Title bar, cards, dialogs, multi-select, service icons, …
  services.ts   The services of the catalog and their icons (Simple Icons)
  i18n/         en.json (Chinese → English), zh-TW.json (Simplified → Traditional)
scripts/build-core.ps1   Builds sing-box from source
scripts/build-service.ps1 Builds NextBoxService.exe
docs/render/             Source of the README hero images
```

</details>

<details>
<summary>Icons and hero images</summary>

The app icons (`build/appicon.png`, `build/windows/icon.ico`) and `docs/logo.png` are committed. After
changing `internal/icon`, regenerate them with `go run ./cmd/genicon`.

`docs/preview.png` and `docs/preview-en.png` are the real frontend running on made-up data, rendered with
headless Chrome / Edge. Regenerate them after UI changes:

```bash
cd frontend && npm install && npm run build && cd ..
node docs/render/render.mjs
```

</details>

## 🗺️ Roadmap

- Load-balancing / auto-select groups
- QR code import / export, global hotkeys
- Linux / macOS support

## 🙏 Credits

- [MatsuriDayo/nekoray](https://github.com/MatsuriDayo/nekoray): the project NextBox is based on
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box): proxy core
- [SagerNet/sing-geosite](https://github.com/SagerNet/sing-geosite), [sing-geoip](https://github.com/SagerNet/sing-geoip): rule-sets
- [Wails](https://wails.io), [Vue](https://vuejs.org), [fyne systray](https://github.com/fyne-io/systray), [go-winio](https://github.com/microsoft/go-winio), [Noto Sans SC](https://github.com/notofonts/noto-cjk)
- [Simple Icons](https://github.com/simple-icons/simple-icons): icons of the service catalog
- Developed with the help of Claude (Anthropic)

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for licenses.

## 📄 License

[GPL-3.0-or-later](LICENSE).

Use this software in accordance with the laws and regulations of your country or region.
