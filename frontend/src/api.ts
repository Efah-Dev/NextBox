import { t } from './i18n'
// Thin typed wrapper over the Wails bindings (window.go.main.App) and runtime.
// Using window.* directly keeps the frontend independent of generated files.

export interface Settings {
  theme: 'auto' | 'light' | 'dark'
  accentColor: string
  language: string
  listenAddress: string
  mixedPort: number
  allowLan: boolean
  systemProxy: boolean
  tunEnabled: boolean
  tunStack: string
  tunStrictRoute: boolean
  tunMtu: number
  routeMode: 'rule' | 'global' | 'direct'
  blockAds: boolean
  directRules: string
  proxyRules: string
  blockRules: string
  ruleSetMirror: string
  remoteDns: string
  directDns: string
  enableIpv6: boolean
  sniff: boolean
  testUrl: string
  testTimeout: number
  testConcurrency: number
  subUserAgent: string
  subUpdateByProxy: boolean
  logLevel: string
  logToFile: boolean
  logFileMaxMb: number
  logKeepDays: number
  logViewLimit: number
  connectOnStart: boolean
  autoStart: boolean
  closeToTray: boolean
  findProcess: boolean
}

export interface SubscriptionInfo {
  upload: number
  download: number
  total: number
  expire: number
}

export interface Group {
  id: number
  name: string
  url: string
  userAgent?: string
  updatedAt?: number
  usage?: SubscriptionInfo
  default?: boolean
}

export type Outbound = Record<string, any>

export interface Profile {
  id: number
  groupId: number
  name: string
  type: string
  outbound: Outbound
  latency: number
  latencyError?: string
  testedAt?: number
  upload: number
  download: number
}

export interface AppState {
  version: string
  running: boolean
  runningId: number
  selectedId: number
  currentGroupId: number
  startedAt: number
  testing: boolean
  proxyMode: '' | 'system' | 'tun'
  elevated: boolean
  coreFound: boolean
  corePath: string
  dataDir: string
  settings: Settings
  groups: Group[]
}

export interface Connection {
  id: string
  metadata: {
    network: string
    type: string
    sourceIP: string
    destinationIP: string
    sourcePort: string
    destinationPort: string
    host: string
    processPath: string
  }
  upload: number
  download: number
  start: string
  chains: string[]
  rule: string
}

export interface Connections {
  downloadTotal: number
  uploadTotal: number
  memory: number
  connections: Connection[]
}

export interface CoreInfo {
  found: boolean
  version: string
  channel: 'stable' | 'testing' | ''
  environment: string
  tags: string
  revision: string
  cgo: string
}

/** Consecutive log lines; next = lines logged so far, up to the last one. */
export interface LogBatch {
  lines: string[]
  next: number
}

export interface LatencyResult {
  id: number
  latency: number
  error?: string
}

const w = window as any

function call<T>(name: string, ...args: unknown[]): Promise<T> {
  const app = w.go?.main?.App
  if (!app) return Promise.reject(new Error(t('后端未就绪：请使用 wails dev / wails build 运行')))
  return app[name](...args).catch((e: unknown) => {
    throw new Error(typeof e === 'string' ? e : (e as any)?.message ?? String(e))
  })
}

export const api = {
  getState: () => call<AppState>('GetState'),
  connect: () => call<void>('Connect'),
  disconnect: () => call<void>('Disconnect'),
  saveSettings: (s: Settings) => call<void>('SaveSettings', s),

  setCurrentGroup: (id: number) => call<void>('SetCurrentGroup', id),
  addGroup: (name: string) => call<Group>('AddGroup', name),
  addSubscription: (name: string, url: string) => call<string>('AddSubscription', name, url),
  editGroup: (g: Group) => call<void>('EditGroup', g),
  deleteGroup: (id: number) => call<void>('DeleteGroup', id),
  updateSubscription: (id: number) => call<string>('UpdateSubscription', id),
  updateAllSubscriptions: () => call<string>('UpdateAllSubscriptions'),

  getProfiles: (groupId: number) => call<Profile[]>('GetProfiles', groupId),
  getProfile: (id: number) => call<Profile>('GetProfile', id),
  importText: (text: string, groupId: number) => call<string>('ImportText', text, groupId),
  saveProfile: (p: Profile) => call<Profile>('SaveProfile', p),
  deleteProfiles: (ids: number[]) => call<void>('DeleteProfiles', ids),
  deleteUnavailable: (groupId: number) => call<number>('DeleteUnavailable', groupId),
  sortGroup: (groupId: number, by: string) => call<void>('SortGroup', groupId, by),
  exportLinks: (ids: number[]) => call<string>('ExportLinks', ids),
  selectProfile: (id: number) => call<void>('SelectProfile', id),

  testLatency: (ids: number[], mode: 'url' | 'tcp') => call<void>('TestLatency', ids, mode),
  stopTest: () => call<void>('StopTest'),

  getConnections: () => call<Connections>('GetConnections'),
  closeConnection: (id: string) => call<void>('CloseConnection', id),
  getLogs: () => call<LogBatch>('GetLogs'),
  clearLogs: () => call<void>('ClearLogs'),
  getLogFilesInfo: () => call<{ count: number; bytes: number; dir: string }>('GetLogFilesInfo'),
  clearLogFiles: () => call<number>('ClearLogFiles'),
  openLogDir: () => call<void>('OpenLogDir'),
  getCoreInfo: () => call<CoreInfo>('GetCoreInfo'),

  updateRuleSets: () => call<string>('UpdateRuleSets'),
  restartAsAdmin: () => call<void>('RestartAsAdmin'),
  openDataDir: () => call<void>('OpenDataDir'),
  openCoreDir: () => call<void>('OpenCoreDir'),
  openURL: (url: string) => call<void>('OpenURL', url),
  quit: () => call<void>('Quit'),
  closeWindow: () => call<void>('CloseWindow'),
}

export const rt = {
  on(name: string, cb: (...data: any[]) => void): () => void {
    return w.runtime?.EventsOn?.(name, cb) ?? (() => {})
  },
  minimise: () => w.runtime?.WindowMinimise(),
  toggleMaximise: () => w.runtime?.WindowToggleMaximise(),
  clipboardGet: (): Promise<string> => w.runtime?.ClipboardGetText() ?? Promise.resolve(''),
  clipboardSet: (text: string): Promise<boolean> => w.runtime?.ClipboardSetText(text) ?? Promise.resolve(false),
}
