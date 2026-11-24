# NextBox for Windows: features and FAQ

[Back to README](README_EN.md) · [简体中文](FEATURES.md)

- [Features](#features)
- [sing-box core](#sing-box-core)
- [Where data is kept](#where-data-is-kept)
- [FAQ](#faq)
- [What each version added](#what-each-version-added)

## Features

### Nodes and subscriptions

- **Protocols**: Shadowsocks (incl. ShadowTLS), VMess, VLESS (incl. Reality / Vision), Trojan, Hysteria2, TUIC, AnyTLS, NaiveProxy, SOCKS, HTTP
- **Subscriptions**: Base64 link lists, Clash / Clash.Meta YAML, sing-box JSON; used traffic and expiry date; optional automatic updates. Updates keep your order and add new nodes at the end
- **Clash conversion**: the groups of a subscription become policy groups, its rules become rule sets (one per target), its DNS settings DNS overrides; you tick what to take the first time (rules are off by default), and it follows every update. *Clash subscription content* in the group menu chooses again
- **Subscription DNS**: when a subscription names its provider's own DNS servers, the import offers them; they are used only for that subscription's nodes, without touching other subscriptions or the global DNS. Switch them per subscription in *Settings → DNS → Subscription DNS*
- **Nodes**: edit common fields in a form or the full sing-box outbound JSON; drag to reorder; search, delete in bulk; remove duplicates; export share links
- **Flags and search by country**: a flag in front of every node, from the exit IP by default or the server IP; searching "Hong Kong" finds HK, 🇭🇰 and 香港
- **Chain proxies**: traffic through 2 to 8 nodes in turn, members from any group, the whole chain can be tested before saving
- **Switching without dropping**: the nodes of a group are loaded at once, so switching does not restart the core or break open connections
- **Latency tests**: one click tests the way the settings say, TCP by default or URL; UDP protocols and chains always use the URL test. After testing a whole group the nodes go back to the order of the subscription, are sorted by latency or stay as they are, asked once on the first test and changed later in *Settings → Subscriptions & tests*

### Routing

- **Route modes**: global / rule / direct
- **Three looks** for the same rules:
  - **Service catalog**: 50+ common services with icons, one tap for where each goes
  - **Cards**: switch on scenes (AI services, streaming, game downloads direct, bypass mainland China) and add rules of your own
  - **Route path**: the order every connection is checked in as one line; ad blocking, scenes, rules and rule sets can be dragged into order; steps that are off can be hidden
- **Tester**: type an address or IP and the path shows the step that catches it, and why
- **Rules**: by site category, domain, IP region, IP range or program, or in a text mode; rules can be named
- **Rule sets**: domains, IPs, programs, site categories, IP regions and online sing-box rule-sets or Clash rule providers (YAML / text) in one named set with one target; bulk paste
- **Right click on a connection**: add it to a rule set or make a new rule, by the site with its subdomains, the full name or the program; undo right away
- **Bypass mainland China**: built-in Chinese domain and IP rules, no download. Known foreign sites and sites on the mainland China list are not resolved; the rest are resolved with the remote DNS through the proxy (default, never poisoned) or the direct DNS
- **Policy groups**: group nodes by region, name keywords or by hand, use the fastest or a chosen one, and send different sites through different groups
- **Ad blocking**: in every route mode

### Connection and system

- **System proxy**: set on connect (also for PPPoE and VPN connections); restored on disconnect, quit, a core crash, logoff or shutdown
- **TUN mode**: captures all traffic. The optional service mode needs administrator approval once, then TUN works without running NextBox elevated; *TUN exclusions* keep Tailscale, ZeroTier and similar networks out of TUN; optional FakeIP
- **DNS**: remote DNS through the proxy, direct DNS locally; UDP / TCP / DoT / DoQ / DoH / DoH3. **DNS overrides** send chosen domains to another DNS server or give them fixed IPs
- **Exit info**: the home page shows the exit IP (masked), country, network (ASN) and a status light; a failed check is retried after 30 seconds
- **Start with Windows**: shows whether it really works, with the reason and a fix when it does not; with service mode the service starts NextBox at logon
- **Tray**: stays in the tray when closed; the menu starts or stops the proxy, switches nodes, route modes and TUN
- **Connections and logs**: live connections with the matched rule and exit node; log filter; optional log files

### Data and look

- **Backups and encryption**: export / import nodes and subscriptions, routing rules, appearance and other settings (each optional), AES-256 with a password, the same format as Android; nodes and subscriptions can also be stored encrypted
- **Updates**: check NextBox and the sing-box core in *Settings → About*; optional automatic check (off by default), only a notice
- **Appearance**: light / dark / system, adjustable levels, custom accent colour; Noto Sans SC
- **Settings search**: top left of the settings page (Ctrl+F)
- **Languages**: Simplified Chinese, Traditional Chinese, English

## sing-box core

`core/sing-box.exe` is built **without modification** from the official source, with only what NextBox uses:

| | |
|---|---|
| Version | **sing-box 1.14.4**, `stable` branch (tag [`v1.14.4`](https://github.com/SagerNet/sing-box/tree/v1.14.4)) |
| Toolchain | Go 1.27.3, `windows/amd64`, CGO off |
| Build tags | `with_gvisor, with_quic, with_dhcp, with_wireguard, with_utls, with_clash_api, with_naive_outbound, with_purego, badlinkname, tfogo_checklinkname0` |

Compared with the official Windows build, Tailscale, cloudflared, OpenVPN / OpenConnect, USB/IP, ACME and the CCM / OCM services are left out; every protocol NextBox supports is in.
`libcronet.dll` comes from [cronet-go](https://github.com/SagerNet/cronet-go) at the version the sing-box source pins.

NextBox writes sing-box **1.12+** configs, so the core in `core/` can be replaced with a newer official one (together with `libcronet.dll` for NaiveProxy).
With service mode installed, click *Update service* in *Settings → TUN mode* afterwards.

## Where data is kept

Everything sits next to `NextBox.exe` (portable); copy the folder to move it:

| Path | Contents |
|---|---|
| `config/profiles.json` | Groups, subscriptions, nodes and policy groups (optionally encrypted) |
| `config/settings.json` | Settings |
| `config/rule-set/` | Rule-sets |
| `config/logs/` | Log files (with *Log to file* on) |
| `config/autostart.json` | Records of starts by Windows |
| `config/android.json` | Phone-only settings from an Android backup, exported again |
| `C:\Program Files\NextBox Service\` | Only with service mode; removed with the service |

Encrypted "with the Windows account", `profiles.json` opens only in this account on this PC: export a backup in *Settings → Data & backup* before moving.

## FAQ

**TUN mode does not start?** It needs administrator rights: install service mode in *Settings → TUN mode* (approve once), or *Restart as administrator*.

**Does the service need updating after updating NextBox?** Yes. *Settings → TUN mode* shows *Update service* when the versions differ.

**A Chinese site still goes through the proxy with the bypass on?** Sites on the mainland China list go direct; others are resolved first and judged by the region of their IP.
The default remote DNS is never poisoned, but a few Chinese sites on foreign CDNs may look foreign. Add a "domain → direct" rule, or switch the resolving to the direct DNS on the home page (it warns first).

**What is subscription DNS?** Some providers name their own DNS servers in Clash or sing-box subscriptions. NextBox offers them on import (ticked by default);
while one of that subscription's nodes is connected they replace the remote DNS, and they look up its nodes' addresses. Other subscriptions are not affected. *Settings → DNS* switches it off or sets it by hand.

**DNS overrides or subscription DNS?** DNS overrides work by domain (say, streaming sites through an unlocking DNS, or `nas.home.arpa` at a fixed IP) whatever node is connected;
subscription DNS works by subscription, only while one of its nodes is connected.

**Do the rules of a Clash subscription replace mine?** No. They are not imported by default; when ticked they become rule sets after your own rules, and updates replace only those.

**Which rule-set addresses work?** sing-box `.srs` / `.json` rule-sets and Clash rule providers (a YAML payload or a plain list). The binary Clash `mrs` format does not.

**Does switching nodes drop connections?** Nodes of one group (up to 200) are loaded together: new connections use the new node, open ones finish on the old one.
Switching to another group, editing a node or changing settings such as the route mode restarts the core.

**Does a subscription update undo my order?** No. Your order stays, new nodes go to the end, nodes the subscription dropped are removed.

**How are flags decided?** *Settings → Subscriptions & tests → Node region*: the exit IP (one request to Cloudflare through the node during a URL test, most accurate) or the server IP (a local IP database).

**How do I send different sites through different nodes?** Make a policy group in *Settings → Policy groups* (say "Japan", fastest), then a rule such as "site category YouTube → Japan".

**How do I change the order of rules?** In the route path look, drag the handle left of each step. The first step (LAN) and the last (everything else) stay in place.

**Do backups work on the phone?** Yes, in both directions; from the other system only settings that mean the same are taken, ports and TUN stay as they are.

**Start with Windows does not work?** *Settings → General → Start with Windows* shows the check and the reason, with a fix. "Did not start" usually means a security tool's startup manager blocked it.

**Tailscale / ZeroTier devices unreachable in TUN mode?** Add their ranges in *Settings → TUN mode → TUN exclusions* (Tailscale: `100.64.0.0/10`) or *Add from adapter*.

**Nodes skipped on import?** The message and the log say why. Xray-only features such as XHTTP or VLESS encryption are not supported by sing-box.

**Port 2030 is taken?** Change the mixed proxy port in *Settings → Proxy ports*.

## What each version added

- **2.4.0**: Clash conversion, subscription DNS, DNS overrides, rules from the connections page, sorting by latency after a test, updates keep the order, appearance in backups
- **2.3.0**: rule sets, service mode, matched rules on the connections page, picking programs, settings search
- **2.2.0**: service catalog, several site categories at once, removing duplicates, ASN of the exit IP
- **2.1.0**: NaiveProxy, route path, TUN exclusions
- **2.0.0**: policy groups, new routing rules, backups and encryption, FakeIP, autostart check
- **1.2.0**: chain proxies, drag to reorder, flags and search by country
- **1.1.0**: switching without dropping, automatic subscription updates, update check
