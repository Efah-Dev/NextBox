<p align="center">
  <img src="docs/logo.png" width="120" alt="NextBox logo">
</p>

<h1 align="center">NextBox</h1>

<p align="center">
  基于 sing-box 内核的 Windows 代理客户端，分流规则一目了然
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
  <b>简体中文</b> · <a href="README_EN.md">English</a> · <a href="https://nextbox.efah.dev/">官网</a> · <a href="https://github.com/Efah-Dev/NextBox/releases/latest">下载</a> · <a href="https://github.com/Efah-Dev/NextBox-Android">Android 版</a>
</p>

<p align="center">
  <img src="docs/preview.png" alt="NextBox 启动页与连接页">
</p>

NextBox 是 [sing-box](https://github.com/SagerNet/sing-box) 代理内核的 Windows 图形客户端，延续了已停更的
[NekoBox for PC（nekoray）](https://github.com/MatsuriDayo/nekoray) 的功能，界面重新设计。
它和 [NextBox for Android](https://github.com/Efah-Dev/NextBox-Android) 使用同一套规则，备份可以在两端互相导入。

> NextBox 是独立项目，与 SagerNet / sing-box 以及 MatsuriDayo / nekoray **没有任何隶属或背书关系**。

## ✨ 亮点

| 亮点 | 说明 |
|---|---|
| 🧭 **看得懂的分流** | 路由路径把每个连接的判断顺序画成一条线，可以拖动排序；「试一试」告诉你一个网址在哪一步被接住 |
| 🗂️ **规则集** | 域名、IP、程序、网站分类、IP 归属地和网上的规则集放进一个规则集，统一指定去向；连接页右键一键加入 |
| 📡 **订阅** | Base64、Clash、sing-box 格式；Clash 订阅的分组、规则和 DNS 可以按需一起转换，随订阅更新 |
| 🌍 **DNS** | 绕过中国大陆不怕 DNS 污染；DNS 复写指定域名的解析；订阅自带的专属 DNS 只用于它自己的节点 |
| 📍 **落地信息** | 启动页显示落地 IP、国家和所属网络（ASN）；测速后可以自动按延迟排序 |
| 🛡️ **系统代理 · TUN** | 切换节点不断流；可选服务模式，开 TUN 不再需要管理员身份；托盘快捷操作 |

完整功能列表和常见问题见 **[功能与常见问题](FEATURES.md)**。

## 📦 下载

在 [Releases](https://github.com/Efah-Dev/NextBox/releases/latest) 下载 `NextBox-v<版本>-windows-amd64.zip`。

需要 Windows 10 / 11 64 位，以及 Microsoft Edge WebView2 运行时（Windows 11 自带）。

1. 解压到一个有写入权限的文件夹，例如 `D:\Apps\NextBox`。不要放在 `C:\Program Files`：NextBox 是便携版，数据保存在程序旁边。
2. 运行 `NextBox.exe`。程序没有代码签名，Windows SmartScreen 拦截时点「更多信息 → 仍要运行」。
3. 升级时覆盖旧文件，保留 `config` 文件夹即可。

## 🚀 快速开始

1. 「节点」页 →「添加订阅」，或复制分享链接后点「导入」。
2. 点一个节点选中它，可以先测速。
3. 「启动」页 →「连接」，系统代理会自动设置。
4. 关闭窗口后 NextBox 留在托盘；右键猫爪图标可以快速操作，选「退出」才会真正关闭。

## 🧩 sing-box 内核

发布包里的 `core/sing-box.exe` 由官方 **sing-box 1.14.4**（`stable` 分支）源码**未经修改**编译而成，只编入 NextBox 用到的功能，体积约为官方版的一半；
`libcronet.dll` 是 NaiveProxy 需要的网络库。NextBox 生成 sing-box 1.12 及以上格式的配置，可以换成更新的官方内核。
详见 [功能与常见问题](FEATURES.md#sing-box-内核)。

## 🛠️ 从源码编译

需要 [Go](https://go.dev/dl/) 1.25+、[Node.js](https://nodejs.org/) LTS 和 Wails CLI：

```bash
git clone https://github.com/Efah-Dev/NextBox.git
cd NextBox
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails build                                                              # build/bin/NextBox.exe
powershell -ExecutionPolicy Bypass -File scripts/build-core.ps1          # 内核，读取 ../sing-box-stable 的源码
powershell -ExecutionPolicy Bypass -File scripts/build-service.ps1       # 服务模式用的 NextBoxService.exe
go test ./...
```

开发时可以用 `wails dev` 热重载。每次推送和 Pull Request 都会在 GitHub Actions 上跑测试并编译。

<details>
<summary>目录结构</summary>

```
main.go                  程序入口：启动参数，嵌入界面文件
cmd/genicon/             生成应用图标
cmd/nextbox-service/     NextBoxService.exe（服务模式）
internal/
  app/       窗口、托盘和界面调用的接口；版本号在 version.go
  clash/     Clash 订阅的分组、规则、DNS 转换，订阅专属 DNS 识别
  config/    生成 sing-box 1.12+ 配置
  store/     数据模型与 JSON 持久化
  link/      分享链接解析 / 导出，订阅格式解析
  sub/       订阅下载
  core/      内核进程、Clash API 客户端、测速
  geo/       落地 IP 与 ASN、IP 数据库、根据节点名称推测地区
  vault/     备份与本地数据加密（AES-256-GCM，与 Android 版相同）
  service/   NextBox 服务：命名管道协议、以系统权限运行内核
  sys/       系统代理、管理员权限、开机自启、进程列表、任务栏
  update/ i18n/ icon/ logfile/ ruleset/
frontend/src/            界面（Vue 3）
third_party/systray/     修改过的托盘库（添加图标失败时重试）
scripts/                 编译内核与服务，sync-android.ps1 把共用代码同步到 Android 版
docs/render/             README 主页渲染图的源文件
```

`docs/preview.png` 是真实界面加上模拟数据，用无头 Chrome / Edge 渲染：先在 `frontend` 里 `npm run build`，再运行 `node docs/render/render.mjs`。

</details>

## 🙏 致谢

- [MatsuriDayo/nekoray](https://github.com/MatsuriDayo/nekoray)：NextBox 的原项目
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box)：代理内核；[sing-geosite](https://github.com/SagerNet/sing-geosite)、[sing-geoip](https://github.com/SagerNet/sing-geoip)：规则集
- [Wails](https://wails.io)、[Vue](https://vuejs.org)、[fyne systray](https://github.com/fyne-io/systray)、[go-winio](https://github.com/microsoft/go-winio)、[思源黑体 Noto Sans SC](https://github.com/notofonts/noto-cjk)、[Simple Icons](https://github.com/simple-icons/simple-icons)
- 由 Claude（Anthropic）协助开发

各组件的许可证见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 📄 许可证

[GPL-3.0-or-later](LICENSE)。请在遵守您所在国家或地区法律法规的前提下使用本软件。
