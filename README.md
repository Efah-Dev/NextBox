<p align="center">
  <img src="docs/logo.png" width="120" alt="NextBox logo">
</p>

<h1 align="center">NextBox</h1>

<p align="center">
  基于 sing-box 内核的轻量、简洁、现代的 Windows 代理客户端
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

- 🔌 **协议**：Shadowsocks（含 ShadowTLS）、VMess、VLESS（含 Reality / Vision）、Trojan、Hysteria2、TUIC、AnyTLS、NaiveProxy、SOCKS、HTTP
- 📡 **订阅**：Base64 链接列表、Clash / Clash.Meta YAML、sing-box JSON；显示已用流量和到期时间；可按设定的间隔自动更新
- 🗂️ **节点**：表单编辑常用字段，或直接编辑完整的 sing-box 出站 JSON；拖动排序；搜索、排序、批量删除；一键删除重复节点（所有分组里连接参数相同的只留一个）；导出分享链接
- 🚩 **国旗与按国家搜索**：每个节点前显示国旗，默认按落地 IP 判断（URL 测试时顺便检测），也可以改为按服务器 IP；搜“香港”就能找到 HK、🇭🇰、Hong Kong 等节点
- ⛓️ **链式代理**：让流量依次经过 2 到 8 个节点，例如机场节点做入口、自建 VPS 落地；成员可以来自任意分组，可拖动调整顺序，保存前可测试整条链路
- 🔀 **切换节点不断流**：同一分组的节点一次载入内核，切换时不重启内核，已建立的连接（例如正在进行的下载）不会中断
- ⚡ **测速**：URL 测试（与 NekoBox 相同：先建立连接，再测往返时间）和 TCP 测试；每个节点右侧的闪电按钮可单独测速；基于 UDP 的协议（Hysteria2、TUIC 等）和链式代理自动改用 URL 测试；可配置测试地址、超时、并发数
- 🧭 **路由**：全局代理（默认，无需配置）/ 规则分流（默认绕过中国大陆）/ 全部直连。规则分流有三种样式，改的是同一份规则，第一次打开时会按你对代理工具的熟悉程度推荐一种：
  - **服务目录**（适合新手）：50 多个常用服务（AI、影音、社交、工具、游戏、国内）带图标列出，每个服务点一下选好走当前节点、直连、拦截还是某个分组代理
  - **卡片**（适合一知半解）：打开常用场景开关（AI 服务、流媒体、游戏下载直连、绕过中国大陆），再添加自己的规则
  - **路由路径**（适合老鸟）：把每个连接的判断顺序画成一条线，屏蔽广告、常用场景和自己的规则都可以任意拖动调整先后；可以隐藏未开启的步骤

  规则可以按网站分类（可多选）、域名、IP 归属地（可多选）、IP 段、程序匹配，也可用文本模式编辑；输入网址或 IP 测试会显示命中哪一条。三种样式都有「恢复默认」
  - **规则集**：把域名、IP、程序、网站分类、IP 归属地，以及网上的 sing-box 规则集（.srs / .json 网址）放进一个有名称的规则集，统一指定去向，一次加好一批规则；支持批量粘贴
  - 规则可以起名称，连接页会显示命中的规则名称；程序规则可以直接从正在运行的程序里勾选
  - 系统代理模式下也会按 IP 归属地判断：域名先用直连 DNS 解析，国内网站即使不在域名列表里也能正确直连
  - 服务目录把 Steam 分成「商店 / 社区」和「下载」：下载走国内 CDN 直连，商店和社区可以单独走代理
- 🧩 **分组代理**：按地区、名称关键词或手动挑选把节点分组，自动选最快或手动指定，再让不同网站走不同分组（例如 YouTube 走日本、Google 走美国）；按条件建的分组在订阅更新后自动包含新节点
- 🚫 **屏蔽广告**：在所有路由模式下都生效
- 📍 **落地信息**：连接后在启动页显示当前节点的国旗、落地 IP（默认打码）、所属网络（ASN，例如 `AS13335 Cloudflare`）和状态灯，可一键复制落地 IP，定期自动复查
- 🖥️ **系统代理**：连接时自动设置，包括宽带拨号（PPPoE）和系统 VPN 连接；断开、退出、内核崩溃时自动还原；Windows 注销 / 关机时优先清除，即使程序被强制结束，下次登录时也会自动清除残留的代理
- 🚀 **开机自启**：设置页实时显示是否真的加入了 Windows 启动项，被禁用、程序被移动等情况会给出原因和一键修复；安装了 NextBox 服务时由服务在你登录 Windows 时启动 NextBox，不依赖注册表启动项，任务管理器也关不掉；没有安装服务、开启 TUN 模式时改用计划任务以管理员身份启动；可选自启后自动连接上次的节点，连接失败时直接弹出主窗口；显示上次开机的结果，能发现被安全软件拦截的情况
- 🛡️ **TUN 模式**：接管全部流量。可选的「服务模式」只需在安装时授权一次管理员，之后开启 TUN 不再需要以管理员身份运行 NextBox；不想安装服务也可以「以管理员身份重启」；「TUN 排除网段」让 Tailscale、ZeroTier 等组网软件的网段不进 TUN，可从网卡一键添加；可选 FakeIP，带可编辑的排除列表
- 💾 **备份与加密**：导出 / 导入配置（节点与订阅、分流规则、其他设置可分别选择），用密码以 AES-256 加密，格式与 Android 版相同，Android 专用的规则和设置会原样保留；本地的节点和订阅也可以加密保存（跟随 Windows 账户，或使用启动密码）；内核配置不再写入磁盘
- 🌍 **DNS**：远程 DNS 走代理，直连 DNS 走本地；支持 UDP / TCP / DoT / DoQ / DoH / DoH3
- 📌 **托盘**：关闭窗口后常驻托盘；右键菜单可启动 / 关闭代理、切换节点、路由模式和 TUN。绿点 = 系统代理，红点 = TUN
- 📜 **连接与日志**：实时连接列表（目标、进程、命中的规则名称、出口节点、速度）；日志筛选；可选记录日志文件（单文件大小上限、保留天数）
- 🔄 **更新**：在「设置 → 关于」里检查 NextBox 和 sing-box 内核的新版本，与 GitHub Releases 比对；也可以打开「自动检查更新」（默认关闭）。只提醒，不会自动下载
- 🎨 **外观**：浅色 / 深色 / 跟随系统；浅色亮度、深色深度可无极调节（深色可调到纯黑）；主题色可选预设或用取色面板自定义（默认淡粉色）；思源黑体 Noto Sans SC
- 🔍 **设置搜索**：设置页左上角可以按名称和说明搜索选项（Ctrl+F），点结果直接跳过去
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
> 分组代理、新的分流规则、备份与加密、FakeIP、开机自启检测从 v2.0.0 开始提供；
> NaiveProxy、路由路径（可调整顺序）、TUN 排除网段从 v2.1.0 开始提供；
> 服务目录、多选网站分类、删除重复节点、落地 IP 的 ASN 从 v2.2.0 开始提供；
> 规则集、服务模式（TUN 免管理员、更稳的开机自启）、连接页显示命中的规则、选择进程、设置搜索从 v2.3.0 开始提供。

## 🚀 快速上手

1. 「节点」页 →「添加订阅」（或复制分享链接后点「导入」）。
2. 点击选中一个节点，也可以先测速。
3. 「启动」页 → 点击「连接」，系统代理会自动设置。
4. 关闭窗口后 NextBox 会留在托盘里；右键猫爪图标可以快速操作，选「退出」才会真正关闭。

## 🧩 sing-box 内核

发布包里的 `core/sing-box.exe` 由官方源码**未经修改**编译而成，只编入 NextBox 用得到的功能（体积约为官方版的一半），旁边的 `libcronet.dll` 是 NaiveProxy 需要的网络库（来自 sing-box 源码锁定版本的 [cronet-go](https://github.com/SagerNet/cronet-go)，与官方发布包相同）：

| | |
|---|---|
| 版本 | **sing-box 1.14.4** |
| 分支 | **`stable` 稳定分支**（官方稳定版，标签 [`v1.14.4`](https://github.com/SagerNet/sing-box/tree/v1.14.4)） |
| 编译环境 | Go 1.27.3，`windows/amd64`，CGO 关闭 |
| 编译标签 | `with_gvisor, with_quic, with_dhcp, with_wireguard, with_utls, with_clash_api, with_naive_outbound, with_purego, badlinkname, tfogo_checklinkname0` |

和官方 Windows 发布版相比，去掉了 NextBox 不会用到的 Tailscale、cloudflared、OpenVPN / OpenConnect、USB/IP、ACME 和 CCM / OCM 服务；所有支持的协议（包括 NaiveProxy、Hysteria2、TUIC、WireGuard）都在。
内核版本和编译标签也可以在「设置 → 关于」里查看。

NextBox 生成的是 sing-box **1.12 及以上**格式的配置，所以可以把 `core/` 里的 `sing-box.exe` 换成任何更新的官方版本（使用 NaiveProxy 时连同 `libcronet.dll` 一起替换）。
装了服务模式时，换完内核后到「设置 → TUN 模式」点「更新服务」，服务里的那份内核才会跟着更新。

## 💾 数据保存位置

所有数据都在 `NextBox.exe` 旁边（便携模式），迁移到另一台电脑时复制整个文件夹即可：

| 路径 | 内容 |
|---|---|
| `config/profiles.json` | 分组、订阅、节点和分组代理（可选加密保存） |
| `config/settings.json` | 设置 |
| `config/rule-set/` | 下载的 geosite / geoip 规则集 |
| `config/logs/` | 日志文件（仅在开启「记录日志到文件」时生成） |
| `config/cache/` | 测速时的临时文件，测完自动清空 |
| `config/autostart.json` | 开机自启的结果记录 |
| `config/android.json` | 从 Android 备份导入的手机专用设置，再次导出时带上 |
| `C:\Program Files\NextBox Service\` | 只在安装了服务模式时存在：服务程序、一份内核、`service.json`（允许使用的账户和开机启动的程序）、`service.log`。在「设置 → TUN 模式」卸载服务时一起删除 |

开启「加密保存节点和订阅」并选择「跟随 Windows 账户」后，`profiles.json` 只能在这台电脑的当前 Windows 账户下打开。
迁移到另一台电脑前，请先在「设置 → 数据与备份」里导出备份。

## ❓ 常见问题

**TUN 模式打不开？** 需要管理员权限，有两种方式，在「设置 → TUN 模式」里任选：
安装「服务模式」（只在安装时授权一次，之后 NextBox 以普通身份运行就能开 TUN，开机自启也更稳），
或者每次「以管理员身份重启」。服务装在 `C:\Program Files\NextBox Service`，不想用了随时可以在同一处卸载。

**更新 NextBox 后服务要更新吗？** 要。版本不一致时「设置 → TUN 模式」会显示「更新服务」，点一下（授权一次）即可。

**国内网站被分流到了代理？** 规则分流会先看域名列表（geosite-cn），不在列表里的再用直连 DNS 解析出 IP，按 IP 归属地判断。
如果网站在国内的 IP 也查不到（例如用了海外 CDN），可以加一条「域名 → 直连」规则，或放进自己的规则集。

**规则集是什么？** 「添加规则集」可以把多种规则（域名、IP、程序、网站分类、网上的 sing-box 规则集）放在一起、起一个名字、统一指定去向，
例如「公司内网 → 直连」。连接页会显示连接命中的是哪个规则集。规则集网址需要是 sing-box 的 `.srs` 或 `.json` 规则集，Clash 规则不能直接使用。

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
开启 TUN 模式但没有管理员权限等情况都会写明原因，点修复即可。安装了服务模式后，开机自启改由服务负责，不再受启动项管理的影响。同一行中间会显示上次开机的结果：
「没有启动」通常是被 360、火绒等安全软件的启动项管理拦截了，需要在那里允许 NextBox。
自启后自动连接失败时会直接弹出主窗口并写明原因。

**开着 Tailscale / ZeroTier 时 TUN 模式连不上组网设备？** 在「设置 → TUN 模式 → TUN 排除网段」里加上它们的网段
（Tailscale 是 `100.64.0.0/10`，也可以点「从网卡添加」），这些地址就不再经过 NextBox。

**三种规则样式选哪个？** 不想研究规则就用「服务目录」，给常用服务点一下选好去向即可；想自己加几条规则用「卡片」；
想看清并调整每一步的先后用「路由路径」。随时可以在「规则样式」里切换，旁边的「帮我选」会再打开推荐引导。

**规则的先后顺序怎么调？** 在「路由路径」样式里，拖动每一步左边的把手即可。
第一步「局域网」和最后一步「其余流量」固定不动，其余（屏蔽广告、常用场景、自己的规则）都可以拖动。
在服务目录里单独指定的服务会排在最前面，比常用场景优先。规则改乱了，点底部的「恢复默认」即可从头开始。

**重复节点怎么删？** 「节点」页 → 右上角「⋯」→「删除重复节点」。会检查所有分组，服务器、端口、密码等连接参数完全相同的只留一个，
删除前列出保留和删除的节点；正在使用的节点不会被删，分组代理和链式代理会自动改用保留的那个。
同一个订阅里的重复节点在更新订阅时会自动去掉。

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

编译 sing-box 内核（默认读取 `../sing-box-stable` 的源码，输出到 `build/bin/core/sing-box.exe`，并复制 NaiveProxy 需要的 `libcronet.dll`）：

```bash
powershell -ExecutionPolicy Bypass -File scripts/build-core.ps1
```

编译服务模式用的 `build/bin/core/NextBoxService.exe`（版本号取自 `main.go`）：

```bash
powershell -ExecutionPolicy Bypass -File scripts/build-service.ps1
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
exit.go、startup.go           落地信息（IP、ASN）检测；开机自启检测、关机清理
autostart_log.go              每次开机自启的结果记录
tunexclude.go                 TUN 排除网段的检查与网卡列表
tray.go                       系统托盘
servicemode.go                服务模式的安装、卸载和状态
cmd/genicon/                  生成应用图标
cmd/nextbox-service/          NextBoxService.exe（服务模式）
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
  sys/       系统代理、管理员权限、开机自启、进程列表（Windows）
  service/   NextBox 服务：命名管道协议、以系统权限运行内核、登录时启动 NextBox
frontend/src/
  views/        启动、节点、连接、日志、设置页面（settings/ 下是各设置板块）
  components/   标题栏、卡片、对话框、多选框、服务图标等
  services.ts   服务目录的服务与图标（Simple Icons）
  i18n/         en.json（中文 → 英文）、zh-TW.json（简体 → 繁体）
scripts/build-core.ps1   从源码编译 sing-box
scripts/build-service.ps1 编译 NextBoxService.exe
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
- [Wails](https://wails.io)、[Vue](https://vuejs.org)、[fyne systray](https://github.com/fyne-io/systray)、[go-winio](https://github.com/microsoft/go-winio)、[思源黑体 Noto Sans SC](https://github.com/notofonts/noto-cjk)
- [Simple Icons](https://github.com/simple-icons/simple-icons)：服务目录的图标
- 由 Claude（Anthropic）协助开发

各组件的许可证见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 📄 许可证

[GPL-3.0-or-later](LICENSE)。

请在遵守您所在国家或地区法律法规的前提下使用本软件。
