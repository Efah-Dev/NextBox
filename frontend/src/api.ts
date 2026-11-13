import { t } from './i18n'
// Thin typed wrapper over the Wails bindings (window.go.main.App) and runtime.
// Using window.* directly keeps the frontend independent of generated files.

export interface Settings {
  theme: 'auto' | 'light' | 'dark'
  accentColor: string
  language: string
  /** how far the light theme is dimmed, 0-100 (0 = brightest) */
  lightLevel: number
  /** how deep the dark theme goes, 0-100 (60 = classic, 100 = black) */
  darkLevel: number
  recentColors?: string[]
  listenAddress: string
  mixedPort: number
  allowLan: boolean
  systemProxy: boolean
  tunEnabled: boolean
  tunStack: string
  tunStrictRoute: boolean
  tunMtu: number
  tunFakeIp: boolean
  fakeIpExclude: string
  routeMode: 'rule' | 'global' | 'direct'
  blockAds: boolean
  rules: Rule[]
  scenes: Scene[]
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
  subAutoUpdate: boolean
  subUpdateHours: number
  autoCheckUpdate: boolean
  /** what a node's country is judged by: the exit IP or the server IP */
  countrySource: 'exit' | 'server'
  countryNoticeShown: boolean
  ruleTipShown: boolean
  logLevel: string
  logToFile: boolean
  logFileMaxMb: number
  logKeepDays: number
  logViewLimit: number
  connectOnStart: boolean
  autoStart: boolean
  autoStartConnect: boolean
  closeToTray: boolean
  findProcess: boolean
}

export type RuleKind = 'geosite' | 'suffix' | 'domain' | 'keyword' | 'regex' | 'geoip' | 'cidr' | 'process'

/** 'proxy' | 'direct' | 'block' | 'policy:<id>' */
export type Target = string

export interface Rule {
  kind: RuleKind
  values: string[]
  target: Target
  disabled?: boolean
}

export interface Scene {
  id: string
  enabled: boolean
  target: Target
}

export interface Policy {
  id: number
  name: string
  match: 'country' | 'keyword' | 'manual'
  country?: string
  keyword?: string
  members?: number[]
  groupId?: number
  mode: 'auto' | 'select'
  selected?: number
}

export interface PolicyMember {
  id: number
  name: string
  type: string
  latency: number
  country: string
}

export interface PolicyView extends Policy {
  nodes: PolicyMember[]
  /** the member the running core uses (0 = not running / unused) */
  now: number
  rules: number
}

export interface RouteMatch {
  input: string
  source: 'ads' | 'rule' | 'scene' | 'private' | 'final'
  index: number
  scene: string
  target: Target | 'final'
  note?: string
}

export interface ExitInfo {
  nodeId: number
  checking: boolean
  ip: string
  country: string
  latency: number
  error?: string
  at: number
}

export interface AutoStartStatus {
  enabled: boolean
  ok: boolean
  method: '' | 'run' | 'task'
  problem: '' | 'missing' | 'moved' | 'disabled' | 'tun' | 'error'
  detail?: string
}

export interface BackupOptions {
  nodes: boolean
  rules: boolean
  settings: boolean
  password: string
  replace?: boolean
}

export interface BackupInfo {
  encrypted: boolean
  needPassword: boolean
  platform: string
  version: number
  hasNodes: boolean
  hasRules: boolean
  hasSettings: boolean
  summary: { groups: number; subscriptions: number; nodes: number; chains: number; policies: number }
  rules: number
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
  /** the nodes of this subscription were reordered by hand */
  manualOrder?: boolean
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
  /** how the latency was measured */
  latencyKind?: 'tcp' | 'url'
  testedAt?: number
  upload: number
  download: number
  /** member IDs of a chain proxy (type 'chain'), entry first */
  chain?: number[]
  /** the members, resolved by the backend (they may live in other groups) */
  members?: ChainHop[]
  country?: string
  countryIp?: string
  countrySource?: 'exit' | 'server'
  countryAt?: number
}

export interface ChainHop {
  id: number
  name: string
  type: string
  groupId: number
  country?: string
  countrySource?: string
  /** deleted, or renamed by a subscription update */
  missing?: boolean
}

export interface ChainInput {
  id: number
  /** -1 creates a new group for it */
  groupId: number
  name: string
  chain: number[]
  /** result of a test of exactly these hops in the editor, kept on save (0 = none) */
  latency?: number
  exit?: { ip: string; country: string }
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
  /** the node list waits for its start-up password */
  locked: boolean
  encryption: '' | 'windows' | 'password'
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

/** An installed version next to the latest release (latest is '' when unknown). */
export interface UpdateItem {
  current: string
  latest: string
  url: string
  hasUpdate: boolean
}

export interface UpdateInfo {
  app: UpdateItem
  core: UpdateItem
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
  /** not measurable this way (a chain proxy in a TCP test); the old result stands */
  skipped?: boolean
  /** where the node's traffic leaves, when it was looked up */
  exit?: { ip: string; country: string }
  /** how it was measured */
  kind?: 'tcp' | 'url'
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
  reorderProfiles: (groupId: number, ids: number[]) => call<void>('ReorderProfiles', groupId, ids),
  saveChain: (c: ChainInput) => call<Profile>('SaveChain', c),
  chainsUsing: (ids: number[]) => call<string[]>('ChainsUsing', ids),
  testChain: (ids: number[]) => call<LatencyResult>('TestChain', ids),
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
  checkUpdate: () => call<UpdateInfo>('CheckUpdate'),

  updateRuleSets: () => call<string>('UpdateRuleSets'),
  testRoute: (input: string) => call<RouteMatch>('TestRoute', input),
  getPolicies: () => call<PolicyView[]>('GetPolicies'),
  savePolicy: (p: Policy) => call<Policy>('SavePolicy', p),
  deletePolicy: (id: number) => call<void>('DeletePolicy', id),
  selectPolicyNode: (id: number, node: number) => call<void>('SelectPolicyNode', id, node),

  getExitInfo: () => call<ExitInfo>('GetExitInfo'),
  refreshExit: () => call<void>('RefreshExit'),
  getAutoStartStatus: () => call<AutoStartStatus>('GetAutoStartStatus'),
  repairAutoStart: () => call<AutoStartStatus>('RepairAutoStart'),

  exportBackup: (o: BackupOptions) => call<string>('ExportBackup', o),
  chooseBackup: () => call<string>('ChooseBackup'),
  inspectBackup: (path: string, password: string) => call<BackupInfo>('InspectBackup', path, password),
  importBackup: (path: string, o: BackupOptions) => call<string>('ImportBackup', path, o),
  setEncryption: (mode: string, password: string) => call<void>('SetEncryption', mode, password),
  unlock: (password: string) => call<void>('Unlock', password),
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
