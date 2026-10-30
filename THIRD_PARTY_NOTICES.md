# Third-Party Notices

NextBox is licensed under the GNU General Public License v3.0 or later (see `LICENSE`).
It includes or redistributes the following third-party software.

## Bundled in the release package

| Component | Version | License | Source |
|---|---|---|---|
| sing-box (proxy core, `core/sing-box.exe`) | 1.14.4 (stable) | GPL-3.0-or-later, with an additional naming clause (see below) | https://github.com/SagerNet/sing-box/tree/v1.14.4 |
| Noto Sans SC (UI font, embedded in `NextBox.exe`) | 5.3.1 (@fontsource-variable) | SIL Open Font License 1.1 | https://github.com/notofonts/noto-cjk |

`core/sing-box.exe` is built **without modification** from the official sing-box v1.14.4 source
(`stable` branch) by `scripts/build-core.ps1`. Its complete corresponding source code is available
at the link above. The sing-box license adds: *"no derivative work may use the name or imply
association with this application without prior consent."* NextBox is an independent project and is
**not affiliated with or endorsed by** SagerNet or the sing-box project.

The full license texts are included in the release package under `licenses/`.

## Compiled into NextBox.exe

| Component | License |
|---|---|
| Wails v2 (`github.com/wailsapp/wails/v2`) | MIT |
| go-webview2 (`github.com/wailsapp/go-webview2`) | MIT |
| fyne systray (`fyne.io/systray`) | Apache-2.0 |
| yaml.v3 (`gopkg.in/yaml.v3`) | MIT and Apache-2.0 |
| Go extended libraries (`golang.org/x/sys`, `x/net`, `x/crypto`, `x/text`) | BSD-3-Clause |
| Vue 3 (`vue`) | MIT |
| Other indirect Go modules (see `go.mod` / `go.sum`) | MIT / BSD / Apache-2.0 |

## Rule-set data

| Data | License | Source |
|---|---|---|
| `geosite-cn.srs`, `geoip-cn.srs` (embedded in `NextBox.exe`, unmodified) | See the upstream repositories | `rule-set` branches of https://github.com/SagerNet/sing-geosite and https://github.com/SagerNet/sing-geoip |
| Other geosite / geoip rule-sets (downloaded on demand, not bundled) | See the upstream repositories | same as above |

## Origin

NextBox is based on NekoBox for PC (nekoray, GPL-3.0) by MatsuriDayo:
https://github.com/MatsuriDayo/nekoray
