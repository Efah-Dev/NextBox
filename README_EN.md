<p align="center">
  <img src="docs/logo.png" width="120" alt="NextBox logo">
</p>

<h1 align="center">NextBox</h1>

<p align="center">
  A Windows proxy client powered by sing-box, with routing you can read
</p>

<p align="center">
  <a href="https://github.com/Efah-Dev/NextBox/releases/latest"><img src="https://img.shields.io/github/v/release/Efah-Dev/NextBox?label=Release&color=f472b6" alt="Release"></a>
  <a href="https://github.com/Efah-Dev/NextBox/releases"><img src="https://img.shields.io/github/downloads/Efah-Dev/NextBox/total?label=Downloads&color=8b8cf8" alt="Downloads"></a>
  <a href="https://github.com/Efah-Dev/NextBox/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/Efah-Dev/NextBox/build.yml?branch=main&label=Build" alt="Build"></a>
  <img src="https://img.shields.io/badge/Platform-Windows%2010%20%2F%2011-0078D4" alt="Platform">
  <img src="https://img.shields.io/badge/sing--box-1.14.4-5B6678" alt="sing-box">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-GPL--3.0-blue" alt="License"></a>
</p>

<p align="center">
  <a href="README.md">简体中文</a> · <b>English</b> · <a href="https://nextbox.efah.dev/">Website</a> · <a href="https://github.com/Efah-Dev/NextBox/releases/latest">Download</a> · <a href="https://github.com/Efah-Dev/NextBox-Android">Android</a>
</p>

<p align="center">
  <img src="docs/preview-en.png" alt="NextBox home and connections pages">
</p>

NextBox is a Windows GUI for the [sing-box](https://github.com/SagerNet/sing-box) proxy core. It continues the feature set of
the no-longer-maintained [NekoBox for PC (nekoray)](https://github.com/MatsuriDayo/nekoray) with a new interface.
It shares its rules with [NextBox for Android](https://github.com/Efah-Dev/NextBox-Android): backups move between the two.

> NextBox is an independent project. It is **not affiliated with or endorsed by** SagerNet / sing-box or MatsuriDayo / nekoray.

## ✨ Highlights

| Highlight | What it means |
|---|---|
| 🧭 **Routing you can read** | The route path draws the order every connection is checked in, and you drag steps into order; the tester tells which step catches a site |
| 🗂️ **Rule sets** | Domains, IPs, programs, site categories, IP regions and online rule-sets in one named set with one target; add a connection to one with a right click |
| 📡 **Subscriptions** | Base64, Clash and sing-box formats; the groups, rules and DNS of a Clash subscription can be converted too, and follow its updates |
| 🌍 **DNS** | Bypassing mainland China without DNS poisoning; DNS overrides for chosen domains; a subscription's own DNS used only for its nodes |
| 📍 **Exit info** | The home page shows the exit IP, country and network (ASN); nodes can be sorted by latency after a test |
| 🛡️ **System proxy · TUN** | Switch nodes without dropping connections; an optional service runs TUN without administrator rights; tray quick actions |

The full feature list and FAQ are in **[Features and FAQ](FEATURES_EN.md)**.

## 📦 Download

Get `NextBox-v<version>-windows-amd64.zip` from [Releases](https://github.com/Efah-Dev/NextBox/releases/latest).

Requires Windows 10 / 11 64-bit and the Microsoft Edge WebView2 runtime (included in Windows 11).

1. Extract it to a writable folder such as `D:\Apps\NextBox`. Not `C:\Program Files`: NextBox is portable and keeps its data next to the program.
2. Run `NextBox.exe`. The build is not code-signed: if SmartScreen warns, choose *More info → Run anyway*.
3. To upgrade, replace the old files and keep the `config` folder.

## 🚀 Quick start

1. *Nodes* → *Add subscription*, or copy share links and click *Import*.
2. Click a node to select it; test the latency first if you like.
3. *Home* → *Connect*. The system proxy is set for you.
4. Closing the window keeps NextBox in the tray; right-click the paw for quick actions, *Quit* to exit.

## 🧩 sing-box core

`core/sing-box.exe` is built **without modification** from the official **sing-box 1.14.4** source (`stable` branch), with only the features NextBox uses,
about half the size of the official build; `libcronet.dll` is the network library NaiveProxy needs. NextBox writes sing-box 1.12+ configs,
so a newer official core can replace it. See [Features and FAQ](FEATURES_EN.md#sing-box-core).

## 🛠️ Build from source

Requires [Go](https://go.dev/dl/) 1.25+, [Node.js](https://nodejs.org/) LTS and the Wails CLI:

```bash
git clone https://github.com/Efah-Dev/NextBox.git
cd NextBox
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails build                                                              # build/bin/NextBox.exe
powershell -ExecutionPolicy Bypass -File scripts/build-core.ps1          # the core, from ../sing-box-stable
powershell -ExecutionPolicy Bypass -File scripts/build-service.ps1       # NextBoxService.exe for service mode
go test ./...
```

`wails dev` gives hot reload. GitHub Actions runs the tests and builds on every push and pull request.

<details>
<summary>Project layout</summary>

```
main.go                  Entry point: start arguments, the embedded UI files
cmd/genicon/             Generates the application icons
cmd/nextbox-service/     NextBoxService.exe (service mode)
internal/
  app/       Window, tray and the API the UI calls; the version is in version.go
  clash/     Clash subscriptions: groups, rules and DNS conversion, subscription DNS
  config/    sing-box 1.12+ configuration builder
  store/     Data model and JSON persistence
  link/      Share-link parsing / export, subscription formats
  sub/       Subscription download
  core/      Core process, Clash API client, latency tests
  geo/       Exit IP and ASN, IP database, guessing the region from node names
  vault/     Encryption of backups and local data (AES-256-GCM, same as Android)
  service/   The NextBox service: named pipe protocol, runs the core with system rights
  sys/       System proxy, elevation, autostart, running programs, taskbar
  update/ i18n/ icon/ logfile/ ruleset/
frontend/src/            The UI (Vue 3)
third_party/systray/     Modified tray library (retries adding the icon)
scripts/                 Building the core and the service; sync-android.ps1 copies the shared code to the Android app
docs/render/             Source of the README images
```

`docs/preview-en.png` is the real UI with made-up data, rendered with headless Chrome / Edge: run `npm run build` in `frontend`, then `node docs/render/render.mjs`.

</details>

## 🙏 Acknowledgements

- [MatsuriDayo/nekoray](https://github.com/MatsuriDayo/nekoray): the project NextBox continues
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box): the proxy core; [sing-geosite](https://github.com/SagerNet/sing-geosite), [sing-geoip](https://github.com/SagerNet/sing-geoip): rule-sets
- [Wails](https://wails.io), [Vue](https://vuejs.org), [fyne systray](https://github.com/fyne-io/systray), [go-winio](https://github.com/microsoft/go-winio), [Noto Sans SC](https://github.com/notofonts/noto-cjk), [Simple Icons](https://github.com/simple-icons/simple-icons)
- Developed with the help of Claude (Anthropic)

Licenses of the components: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## 📄 License

[GPL-3.0-or-later](LICENSE). Use this software in compliance with the laws of your country or region.
