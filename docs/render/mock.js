// A stand-in for the Go backend, used only to render the README hero image
// (see render.mjs). It is injected in front of the real frontend bundle and
// provides window.go.main.App and window.runtime with made-up data.
//
// URL parameters: lang=en, theme=dark, page=nodes|connections|logs|settings
(() => {
  const q = new URLSearchParams(location.search)
  const en = q.get('lang') === 'en'
  const theme = q.get('theme') === 'dark' ? 'dark' : 'light'
  const page = q.get('page') || 'home'
  const tr = (zh, english) => (en ? english : zh)
  const now = Date.now()
  const GB = 1024 ** 3

  const settings = {
    theme, accentColor: '#F472B6', language: en ? 'en' : 'zh-CN',
    listenAddress: '127.0.0.1', mixedPort: 2030, allowLan: false, systemProxy: true,
    tunEnabled: false, tunStack: 'mixed', tunStrictRoute: true, tunMtu: 9000,
    routeMode: 'rule', blockAds: true, directRules: '', proxyRules: '', blockRules: '', ruleSetMirror: 'local',
    remoteDns: 'https://1.1.1.1/dns-query', directDns: '223.5.5.5', enableIpv6: false, sniff: true,
    testUrl: 'https://www.google.com/generate_204', testTimeout: 5000, testConcurrency: 16,
    subUserAgent: 'NextBox', subUpdateByProxy: false, subAutoUpdate: true, subUpdateHours: 12, autoCheckUpdate: false,
    logLevel: 'info', logToFile: false, logFileMaxMb: 10, logKeepDays: 7, logViewLimit: 500,
    connectOnStart: false, autoStart: true, closeToTray: true, findProcess: false,
  }

  const nodes = [
    ['Tokyo 01', 'vless', 'tyo1', 48], ['Tokyo 02', 'hysteria2', 'tyo2', 62], ['Osaka 01', 'trojan', 'osa1', 71],
    ['Hong Kong 01', 'tuic', 'hkg1', 39], ['Hong Kong 02', 'shadowsocks', 'hkg2', 45], ['Singapore 01', 'vless', 'sin1', 76],
    ['Singapore 02', 'hysteria2', 'sin2', 84], ['Seoul 01', 'vmess', 'sel1', 95], ['Taipei 01', 'anytls', 'tpe1', 58],
    ['Los Angeles 01', 'vless', 'lax1', 148], ['San Jose 01', 'trojan', 'sjc1', 156], ['Frankfurt 01', 'vless', 'fra1', 212],
    ['London 01', 'shadowsocks', 'lon1', 231], ['Sydney 01', 'trojan', 'syd1', -1],
  ]
  const profiles = nodes.map(([name, type, host, latency], i) => ({
    id: i + 1, groupId: 1, name, type, latency, testedAt: Math.floor(now / 1000) - 600,
    latencyError: latency < 0 ? 'timeout' : undefined,
    outbound: { type, server: `${host}.example.net`, server_port: 443 },
    upload: Math.round((14 - i) ** 2 * 0.011 * GB), download: Math.round((14 - i) ** 2 * 0.19 * GB),
  }))

  const state = {
    version: '1.0.3', running: true, runningId: 1, selectedId: 1, currentGroupId: 1,
    startedAt: now - (42 * 60 + 18) * 1000, testing: false, proxyMode: 'system', elevated: false,
    coreFound: true, corePath: 'core\\sing-box.exe', dataDir: 'config', settings,
    groups: [
      {
        id: 1, name: tr('示例订阅', 'Example'), url: 'https://example.com/sub',
        updatedAt: Math.floor(now / 1000) - 3 * 3600,
        usage: { upload: 3.2 * GB, download: 41.5 * GB, total: 200 * GB, expire: Math.floor(now / 1000) + 86 * 86400 },
      },
      { id: 2, name: tr('自建节点', 'My servers'), url: '' },
    ],
  }

  const logs = [
    '21:30:02 [GUI] ' + tr('已加载内置规则集: ', 'Loaded built-in rule-set: ') + 'geoip-cn',
    '21:30:02 [GUI] ' + tr('已加载内置规则集: ', 'Loaded built-in rule-set: ') + 'geosite-cn',
    '21:30:05 [GUI] ' + tr('正在连接: ', 'Connecting: ') + 'Tokyo 01',
    '21:30:05 INFO inbound/mixed[mixed-in]: tcp server started at 127.0.0.1:2030',
    '21:30:05 INFO sing-box started (0.12s)',
    '21:30:05 [GUI] ' + tr('内核已启动，混合代理端口 ', 'Core started, mixed proxy port ') + '2030',
    '21:30:11 INFO outbound/vless[Tokyo 01]: outbound connection to www.example.com:443',
    '21:30:12 INFO outbound/direct[direct]: outbound connection to www.example.cn:443',
  ]

  const connection = (i) => ({
    id: String(i), upload: 1200 * (i + 1), download: 86000 * (i + 1), start: new Date(now - i * 9000).toISOString(),
    chains: i % 4 ? ['Tokyo 01', 'proxy'] : ['direct'], rule: i % 4 ? 'final' : 'rule_set=geosite-cn => route(direct)',
    metadata: {
      network: 'tcp', type: 'mixed', sourceIP: '127.0.0.1', sourcePort: String(50000 + i),
      destinationIP: '', destinationPort: '443', host: i % 4 ? `cdn${i}.example.com` : `www${i}.example.cn`, processPath: '',
    },
  })

  const app = {
    GetState: () => state,
    GetProfiles: (groupId) => profiles.filter((p) => p.groupId === groupId),
    GetProfile: (id) => profiles.find((p) => p.id === id),
    GetLogs: () => ({ lines: logs, next: logs.length }),
    GetConnections: () => ({
      downloadTotal: 1.86 * GB, uploadTotal: 0.092 * GB, memory: 38 << 20,
      connections: Array.from({ length: 23 }, (_, i) => connection(i)),
    }),
    GetCoreInfo: () => ({
      found: true, version: '1.14.4', channel: 'stable', environment: 'go1.27.3 windows/amd64',
      tags: 'with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_clash_api', revision: '', cgo: 'disabled',
    }),
    GetLogFilesInfo: () => ({ count: 0, bytes: 0, dir: 'config\\logs' }),
  }
  // every other call succeeds and does nothing
  const handler = { get: (target, name) => (...args) => Promise.resolve(target[name]?.(...args)) }
  window.go = { main: { App: new Proxy(app, handler) } }

  const listeners = {}
  const emit = (name, data) => (listeners[name] ?? []).forEach((cb) => cb(data))
  window.runtime = {
    EventsOn(name, cb) {
      ;(listeners[name] ??= []).push(cb)
      if (name === 'navigate' && page !== 'home') setTimeout(() => emit('navigate', page), 50)
      return () => {}
    },
    WindowIsMaximised: () => Promise.resolve(false),
    WindowMinimise() {},
    WindowToggleMaximise() {},
    ClipboardGetText: () => Promise.resolve(''),
    ClipboardSetText: () => Promise.resolve(true),
  }

  // one minute of traffic for the speed chart, replayed quickly
  let tick = 0
  const timer = setInterval(() => {
    if (!listeners.traffic) return
    const wave = 0.58 + 0.27 * Math.sin(tick / 6.5) + 0.12 * Math.sin(tick / 1.9 + 1)
    emit('traffic', { up: Math.round(182e3 * (0.7 + 0.5 * wave)), down: Math.round(8.6e6 * wave) })
    if (++tick >= 60) clearInterval(timer)
  }, 15)
})()
