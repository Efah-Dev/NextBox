<p align="center">
  <img src="docs/logo.png" width="120" alt="NextBox logo">
</p>

<h1 align="center">NextBox</h1>

<p align="center">
  基于 sing-box 内核的简洁、现代的 Windows 代理客户端
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
  <b>简体中文</b> · <a href="README_EN.md">English</a> · <a href="https://nextbox.efah.dev/">官网</a> · <a href="https://github.com/Efah-Dev/NextBox/releases/latest">下载</a> · <a href="https://github.com/Efah-Dev/NextBox-Android">Android 版</a>
</p>

<p align="center">
  <img src="docs/preview.png" alt="NextBox 启动页与节点页">
</p>

NextBox 是 [sing-box](https://github.com/SagerNet/sing-box) 代理内核的桌面图形客户端。它延续了已停更的
[NekoBox for PC（nekoray）](https://github.com/MatsuriDayo/nekoray) 的功能，重新设计了启动器风格的界面，
采用 sing-box 最新的配置格式，并加入了系统托盘。

> NextBox 是独立项目，与 SagerNet / sing-box 以及 MatsuriDayo / nekoray **没有任何隶属或背书关系**。

## ✨ 功能

- 🔌 **协议**：Shadowsocks、VMess、VLESS（含 Reality / Vision）、Trojan、Hysteria2、TUIC、AnyTLS、SOCKS、HTTP
- 📡 **订阅**：Base64 链接列表、Clash / Clash.Meta YAML、sing-box JSON；显示已用流量和到期时间；可按设定的间隔自动更新
- 🗂️ **节点**：表单编辑常用字段，或直接编辑完整的 sing-box 出站 JSON；拖动排序；搜索、排序、批量删除；导出分享链接
- 🚩 **国旗与按国家搜索**：每个节点前显示国旗，默认按落地 IP 判断（URL 测试时顺便检测），也可以改为按服务器 IP；搜“香港”就能找到 HK、🇭🇰、Hong Kong 等节点
- ⛓️ **链式代理**：让流量依次经过 2 到 8 个节点，例如机场节点做入口、自建 VPS 落地；成员可以来自任意分组，可拖动调整顺序，保存前可测试整条链路
- 🔀 **切换节点不断流**：同一分组的节点一次载入内核，切换时不重启内核，已建立的连接（例如正在进行的下载）不会中断
- ⚡ **测速**：URL 测试（与 NekoBox 相同：先建立连接，再测往返时间）和 TCP 测试；每个节点右侧的闪电按钮可单独测速；基于 UDP 的协议（Hysteria2、TUIC 等）和链式代理自动改用 URL 测试；可配置测试地址、超时、并发数
- 🧭 **路由**：全局代理（默认，无需配置）/ 规则分流（默认绕过中国大陆）/ 全部直连。规则分流下有常用场景开关（AI 服务、流媒体、游戏下载直连等）和按顺序匹配的自定义规则（网站分类、域名、IP、程序），每条规则可以指定走当前节点、直连、拦截或某个分组代理；规则可拖动排序，也可用文本模式编辑；「匹配测试」输入网址或 IP 即可看到命中哪条规则、最终走哪里
- 🧩 **分组代理**：按地区、名称关键词或手动挑选把节点分组，自动选最快或手动指定，再让不同网站走不同分组（例如 YouTube 走日本、Google 走美国）；按条件建的分组在订阅更新后自动包含新节点
- 🚫 **屏蔽广告**：在所有路由模式下都生效
- 📍 **落地信息**：连接后在启动页显示当前节点的国旗、落地 IP（默认打码）和状态灯，定期自动复查
- 🖥️ **系统代理**：连接时自动设置，包括宽带拨号（PPPoE）和系统 VPN 连接；断开、退出、内核崩溃时自动还原；Windows 注销 / 关机时优先清除，即使程序被强制结束，下次登录时也会自动清除残留的代理
- 🚀 **开机自启**：设置页实时显示是否真的加入了 Windows 启动项，被禁用、程序被移动等情况会给出原因和一键修复；开启 TUN 模式时改用计划任务以管理员身份启动；可选自启后自动连接上次的节点
- 🛡️ **TUN 模式**：接管全部流量（需要管理员权限，可一键「以管理员身份重启」）；可选 FakeIP，带可编辑的排除列表
- 💾 **备份与加密**：导出 / 导入配置（节点与订阅、分流规则、其他设置可分别选择），用密码以 AES-256 加密，格式与 Android 版相同；本地的节点和订阅也可以加密保存（跟随 Windows 账户，或使用启动密码）；内核配置不再写入磁盘
- 🌍 **DNS**：远程 DNS 走代理，直连 DNS 走本地；支持 UDP / TCP / DoT / DoQ / DoH / DoH3
- 📌 **托盘**：关闭窗口后常驻托盘；右键菜单可启动 / 关闭代理、切换节点、路由模式和 TUN。绿点 = 系统代理，红点 = TUN
- 📜 **连接与日志**：实时连接列表（目标、进程、规则、速度）；日志筛选；可选记录日志文件（单文件大小上限、保留天数）
- 🔄 **更新**：在「设置 → 关于」里检查 NextBox 和 sing-box 内核的新版本，与 GitHub Releases 比对；也可以打开「自动检查更新」（默认关闭）。只提醒，不会自动下载
- 🎨 **外观**：浅色 / 深色 / 跟随系统；浅色亮度、深色深度可无极调节（深色可调到纯黑）；主题色可选预设或用取色面板自定义（默认淡粉色）；思源黑体 Noto Sans SC
- 🌐 **语言**：简体中文、繁體中文、English

## 📦 下载

在 [Releases](https://github.com/Efah-Dev/NextBox/releases/latest) 页面下载 `NextBox-<版本>-windows-amd64.zip`。

**系统要求**：Windows 10 / 11 64 位，需要 Microsoft Edge WebView2 运行时（Windows 11 和已更新的 Windows 10 自带）。

1. 把压缩包解压到一个有写入权限的文件夹，例如 `D:\Apps\NextBox`。
   **不要**放在 `C:\Program Files` 里：NextBox 是便携版，数据保存在程序旁边。
2. 运行 `NextBox.exe`。程序没有代码签名，第一次运行时 Windows SmartScreen 可能会拦截，
   点击「更多信息 → 仍要运行」即可。

> 切换节点不断流、订阅自动更新、检查更新、拨号连接的系统代理从 v1.1.0 开始提供；
> 链式代理、拖动排序、国旗与按国家搜索从 v1.2.0 开始提供；
> 分组代理、新的分流规则、备份与加密、FakeIP、开机自启检测从 v2.0.0 开始提供。

## 🚀 快速上手

1. 「节点」页 →「添加订阅」（或复制分享链接后点「导入」）。
2. 点击选中一个节点，也可以先测速。
3. 「启动」页 → 点击「连接」，系统代理会自动设置。
4. 关闭窗口后 NextBox 会留在托盘里；右键猫爪图标可以快速操作，选「退出」才会真正关闭。

## 🧩 sing-box 内核

发布包里的 `core/sing-box.exe` 由官方源码**未经修改**编译而成：

| | |
|---|---|
| 版本 | **sing-box 1.14.4** |
| 分支 | **`stable` 稳定分支**（官方稳定版，标签 [`v1.14.4`](https://github.com/SagerNet/sing-box/tree/v1.14.4)） |
| 编译环境 | Go 1.27.3，`windows/amd64`，CGO 关闭 |
| 编译标签 | `with_gvisor, with_quic, with_dhcp, with_wireguard, with_utls, with_acme, with_clash_api, with_tailscale, with_ccm, with_ocm, with_cloudflared, with_usbip, with_openvpn, with_openconnect, badlinkname, tfogo_checklinkname0` |

以上与官方 Windows 发布版的编译标签一致，只去掉了需要额外 cronet DLL 的 `with_naive_outbound` / `with_purego`。
内核版本和编译标签也可以在「设置 → 关于」里查看。

NextBox 生成的是 sing-box **1.12 及以上**格式的配置，所以可以把 `core/` 里的 `sing-box.exe` 换成任何更新的官方版本。

## 💾 数据保存位置

所有数据都在 `NextBox.exe` 旁边（便携模式），迁移到另一台电脑时复制整个文件夹即可：

| 路径 | 内容 |
|---|---|
| `config/profiles.json` | 分组、订阅、节点和分组代理（可选加密保存） |
| `config/settings.json` | 设置 |
| `config/rule-set/` | 下载的 geosite / geoip 规则集 |
| `config/logs/` | 日志文件（仅在开启「记录日志到文件」时生成） |
| `config/cache/` | 测速时的临时文件，测完自动清空 |

开启「加密保存节点和订阅」并选择「跟随 Windows 账户」后，`profiles.json` 只能在这台电脑的当前 Windows 账户下打开。
迁移到另一台电脑前，请先在「设置 → 数据与备份」里导出备份。

## ❓ 常见问题

**TUN 模式打不开？** 需要管理员权限。在「启动」页或「设置 → TUN 模式」里点「以管理员身份重启」。

**切换节点会断网吗？** 连接时会把当前节点所在分组的节点（最多 200 个）一起载入内核，在它们之间切换不需要重启：
新连接走新节点，已建立的连接继续走原来的节点直到结束。切换到其他分组的节点、编辑过的节点，或者修改路由模式等设置时，
内核仍然需要重启。

**规则集需要下载吗？** 默认的「本地内置」镜像使用程序自带的规则集（绕过中国域名和中国 IP），无需联网。
其他规则集（如屏蔽广告、geosite:xxx）在第一次使用时下载；下载失败或超时只会在日志里报错，
连接照常进行，只是暂时跳过相关规则。可以在「设置 → 分流规则 → 规则集镜像」里换一个镜像后点「更新规则集」。

**国旗是怎么判断的？** 在「设置 → 订阅与测速 → 节点地区」里选择：
「落地 IP」在 URL 测试时经节点访问一次 Cloudflare，得到网站实际看到的地区，最准确；
「服务器 IP」在本地查询 sing-geoip 的 IP 数据库（首次使用时下载约 4 MB），不经过节点，但中转节点会显示入口所在地区。
检测出结果之前，先根据节点名称推测（悬停国旗可以看到依据）。

**链式代理怎么用？** 「节点」页 → 工具栏的「+」→「新建链式代理」，添加节点并排好顺序：第一个是入口，最后一个是落地。
链式代理只能保存在手动分组里；成员节点被删除或订阅更新后改了名，链式代理会标为失效，编辑时可以替换那一跳。

**手动排好的顺序会被订阅更新冲掉吗？** 手动点「更新」时会先询问，确认后恢复为订阅里的顺序；后台自动更新会保留你的顺序，新节点加在末尾。

**订阅什么时候自动更新？** 在「设置 → 订阅与测速」里打开「自动更新订阅」并选择间隔。只在 NextBox 运行期间更新，
程序没有运行时不会更新。

**导入时有节点被跳过？** 导入提示和日志页会写明原因。XHTTP 传输、VLESS 加密等 Xray 专有功能 sing-box 不支持。

**「屏蔽广告」是什么？** 拦截社区维护的 `geosite-category-ads-all` 广告域名清单，连接和 DNS 查询都会拦截。
和正文使用同一个域名的广告（例如部分视频广告）无法拦截。

**检查更新失败？** 未连接时是直连 GitHub，网络不通就会失败；连接后再检查即可。

**2030 端口被占用？** 在「设置 → 代理端口」里修改混合代理（HTTP + SOCKS5）端口。

**全局代理和规则分流有什么区别？** 全局代理让所有流量走当前节点，不需要任何配置，自定义规则不生效。
规则分流默认绕过中国大陆的网站和 IP（直连），其余走代理；还可以打开常用场景或添加自己的规则。

**怎么让不同网站走不同节点？** 先在「设置 → 分组代理」里新建分组（例如按地区选「日本」、自动选最快），
再在「设置 → 分流规则 → 我的规则」里添加一条规则，例如「网站分类 YouTube → 日本 自动」。分组代理只在规则分流模式下生效。

**备份能在手机上用吗？** 「设置 → 数据与备份 → 导出」生成的文件可以直接在 NextBox Android 版中导入，
Android 版导出的备份也可以在这里导入。来自其他系统的备份只会导入节点、分流规则和通用设置，端口、TUN 等设置保持不变。

**加密保存选哪种？** 「跟随 Windows 账户」不需要密码，开机自启照常工作，推荐大多数人使用；
「启动密码」每次打开都要输入，开机自启时会停在解锁界面，忘记密码无法恢复。

**开机自启没有生效？** 「设置 → 常规 → 开机自启」右边会显示检测结果。被任务管理器禁用、程序被移动、
开启 TUN 模式但没有管理员权限等情况都会写明原因，点修复即可。

**FakeIP 是什么？** 只在 TUN 模式下生效：走代理的域名先拿到一个 198.18.x.x 的假地址，首次连接更快，DNS 也不会泄露。
断开后个别程序可能还记着假地址而连不上，重启该程序即可；遇到兼容问题的域名可以加到排除列表里。

## 🛠️ 从源码编译

需要 [Go](https://go.dev/dl/) 1.25+、[Node.js](https://nodejs.org/) LTS，以及 Wails CLI：

```bash
git clone https://github.com/Efah-Dev/NextBox.git
cd NextBox
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

编译 `build/bin/NextBox.exe`：

```bash
wails build
```

编译 sing-box 内核（默认读取 `../sing-box-stable` 的源码，输出到 `build/bin/core/sing-box.exe`）：

```bash
powershell -ExecutionPolicy Bypass -File scripts/build-core.ps1
```

运行测试（需要先编译前端；配置校验、测速和节点切换的测试会用到上面编译的内核，没有内核时自动跳过）：

```bash
go test ./...
```

开发时可以用 `wails dev` 热重载。每次推送和 Pull Request 都会在 GitHub Actions 上跑测试并编译 `NextBox.exe`。

<details>
<summary>目录结构</summary>

```
main.go、app.go、support.go   Wails 入口与暴露给界面的接口
background.go                 订阅自动更新、检查更新
policy.go、backup.go          分组代理与匹配测试、备份导入导出与加密
exit.go、startup.go           落地信息检测；开机自启检测、关机清理
tray.go                       系统托盘
cmd/genicon/                  生成应用图标
internal/
  store/     数据模型与 JSON 持久化
  link/      分享链接解析 / 导出，订阅格式解析
  sub/       订阅下载
  config/    生成 sing-box 1.12+ 配置
  core/      内核进程、Clash API 客户端、测速
  update/    查询 GitHub 最新版本
  geo/       落地 IP、IP 数据库、根据节点名称推测地区
  vault/     备份与本地数据加密（AES-256-GCM，与 Android 版格式相同）
  icon/      粉色猫爪图标绘制（应用图标与托盘）
  i18n/      后端翻译（与 frontend/src/i18n 共用词典）
  logfile/   日志文件（自动分割与过期清理）
  sys/       系统代理、管理员权限、开机自启（Windows）
frontend/src/
  views/        启动、节点、连接、日志、设置页面（settings/ 下是各设置板块）
  components/   标题栏、卡片、对话框、规则编辑器等
  i18n/         en.json（中文 → 英文）、zh-TW.json（简体 → 繁体）
scripts/build-core.ps1   从源码编译 sing-box
docs/render/             README 主页渲染图的源文件
```

</details>

<details>
<summary>图标与主页渲染图</summary>

应用图标（`build/appicon.png`、`build/windows/icon.ico`）和 `docs/logo.png` 已包含在源码中。
修改 `internal/icon` 后，用 `go run ./cmd/genicon` 重新生成。

`docs/preview.png` 和 `docs/preview-en.png` 是真实的前端界面加上模拟数据，用无头 Chrome / Edge 渲染出来的。
界面有变化后重新生成：

```bash
cd frontend && npm install && npm run build && cd ..
node docs/render/render.mjs
```

</details>

## 🗺️ 后续计划

- 负载均衡 / 自动选择节点组
- 二维码导入导出、全局热键
- Linux / macOS 支持

## 🙏 致谢

- [MatsuriDayo/nekoray](https://github.com/MatsuriDayo/nekoray)：NextBox 的原项目
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box)：代理内核
- [SagerNet/sing-geosite](https://github.com/SagerNet/sing-geosite)、[sing-geoip](https://github.com/SagerNet/sing-geoip)：规则集
- [Wails](https://wails.io)、[Vue](https://vuejs.org)、[fyne systray](https://github.com/fyne-io/systray)、[思源黑体 Noto Sans SC](https://github.com/notofonts/noto-cjk)
- 由 Claude（Anthropic）协助开发

各组件的许可证见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 📄 许可证

[GPL-3.0-or-later](LICENSE)。

请在遵守您所在国家或地区法律法规的前提下使用本软件。
