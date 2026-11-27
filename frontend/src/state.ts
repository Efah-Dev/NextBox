import { computed, reactive, ref } from 'vue'
import { api, rt, type AppState, type ExitInfo, type LatencyResult, type SpeedResult, type LogBatch, type Profile, type Settings, type TestMode, type UpdateInfo } from './api'
import { t } from './i18n'
import { choiceDialog, toast } from './util'

export type Page = 'home' | 'nodes' | 'connections' | 'logs' | 'settings'

export const state = reactive({
  page: 'home' as Page,
  app: null as AppState | null,
  profiles: [] as Profile[],
  traffic: { up: 0, down: 0, history: [] as { up: number; down: number }[] },
  logs: [] as string[],
  /** Settings section to scroll to when the settings page opens. */
  settingsSection: '',
  /** A backup file to open in the import dialog of the backup section (dropped). */
  backupFile: '',
  /** A site to try in the route tester when the rules section opens (连接页右键). */
  routeProbe: '',
  /** Result of the last update check (设置 → 关于). */
  update: null as UpdateInfo | null,
  /** the one-time explanation of how countries are found (CountryNotice) */
  countryNotice: false,
  /** where the connected node's traffic leaves (home page) */
  exit: null as ExitInfo | null,
  /** appearance being edited on the settings page, shown before it is saved */
  look: null as Pick<Settings, 'theme' | 'accentColor' | 'lightLevel' | 'darkLevel'> | null,
})

/** Called after nodes were added: the first time, explains where the flags
 *  come from and lets the user pick (落地 IP / 服务器 IP). */
export function nodesAdded() {
  if (state.app && !state.app.settings.countryNoticeShown) state.countryNotice = true
}

/** The download speed test: its progress and the results of this session. */
export const speedTest = reactive({
  running: false,
  total: 0,
  done: 0,
  current: 0,
  waiting: new Set<number>(),
  /** results, and the live figure of the node being tested */
  results: new Map<number, SpeedResult>(),
})

/** Progress of the running latency test (mode '' = none). */
export const latencyTest = reactive({
  mode: '' as '' | TestMode,
  total: 0,
  done: 0,
  waiting: new Set<number>(),
  running: new Set<number>(),
  /** The node that started most recently, named in the progress bar. */
  current: 0,
})

export const settings = computed(() => state.app?.settings)

export const currentGroup = computed(() =>
  state.app?.groups.find((g) => g.id === state.app?.currentGroupId),
)

// The selected node can live in another group than the one being browsed.
const selectedCache = ref<Profile | undefined>()

export const selectedProfile = computed(() =>
  state.profiles.find((p) => p.id === state.app?.selectedId) ?? selectedCache.value,
)

export async function refreshState() {
  state.app = await api.getState()
  await refreshSelected()
}

async function refreshSelected() {
  const id = state.app?.selectedId
  if (!id) {
    selectedCache.value = undefined
    return
  }
  if (selectedCache.value?.id === id) return
  selectedCache.value = await api.getProfile(id).catch(() => undefined)
}

export async function loadProfiles() {
  if (!state.app) return
  state.profiles = await api.getProfiles(state.app.currentGroupId)
}

/** The speed test method chosen in the settings. */
export const testMode = (): TestMode => state.app?.settings.testMode ?? 'url'

/**
 * Before the first test of a whole group, asks once how its nodes are
 * ordered afterwards and keeps the answer (Settings.sortAfterTest, changed
 * in the settings later). false when cancelled.
 */
export async function sortChosen(count: number): Promise<boolean> {
  if (count < 2 || state.app?.settings.sortAfterTest) return true
  const v = await choiceDialog(
    t('测延迟'),
    t('测试 {0} 个节点。测完以后节点怎么排？', count),
    [
      { value: 'default', label: t('默认顺序'), hint: t('恢复订阅中的原始顺序') },
      { value: 'latency', label: t('按延迟排序'), hint: t('按延迟从低到高排列') },
      { value: 'none', label: t('保持现在的顺序'), hint: t('保持当前顺序') },
    ],
    'default',
    { confirmText: t('开始测延迟'), note: t('只问这一次。以后要改，前往「设置 → 订阅与测试 → 测延迟后」。') },
  )
  if (!v) return false
  await patchSettings({ sortAfterTest: v as Settings['sortAfterTest'] })
  return true
}

/**
 * Speed tests the given nodes the way the settings say and sorts their group
 * afterwards (Settings.sortAfterTest); a single node is never moved.
 */
export function testNodes(ids: number[], mode = testMode()) {
  const by = state.app?.settings.sortAfterTest || 'default'
  return api.testLatency(ids, mode, ids.length < 2 || by === 'none' ? '' : by)
}

/** Saves a partial settings change; the backend restarts the core if needed. */
export async function patchSettings(patch: Partial<Settings>) {
  if (!state.app) return
  const next = { ...state.app.settings, ...patch }
  state.app.settings = next
  await api.saveSettings(next)
}

// sing-box prefixes each line with its UTC offset ("+0800 2026-10-28 ..."); drop it.
const cleanLog = (line: string) => line.replace(/^[+-]\d{4} /, '')

export function pushLog(line: string) {
  state.logs.push(cleanLog(line))
  trimLogs()
}

// Keeps at most the configured number of lines (设置 → 日志), so a long
// session does not slow the UI down.
export function trimLogs() {
  const limit = state.app?.settings.logViewLimit || 500
  if (state.logs.length > limit) state.logs.splice(0, state.logs.length - limit)
}

export async function init() {
  // Listen for new log lines before fetching the backlog so none are lost, and
  // skip the ones the backlog already holds: the startup lines used to arrive
  // in both and show up twice.
  let shown = -1
  const early: LogBatch[] = []
  const addLogs = (b: LogBatch) => {
    b.lines.slice(Math.max(0, shown - (b.next - b.lines.length))).forEach(pushLog)
    shown = Math.max(shown, b.next)
  }
  rt.on('logs', (b: LogBatch) => (shown < 0 ? early.push(b) : addLogs(b)))

  await refreshState()
  await loadProfiles()
  rt.on('exit', (e: ExitInfo) => (state.exit = e))
  if (state.app?.running) state.exit = await api.getExitInfo().catch(() => null)
  const backlog = await api.getLogs()
  state.logs = backlog.lines.map(cleanLog)
  shown = backlog.next
  trimLogs()
  early.forEach(addLogs)

  let lastGroup = state.app?.currentGroupId
  rt.on('state', async (s: AppState) => {
    state.app = s
    trimLogs()
    await refreshSelected()
    if (s.currentGroupId !== lastGroup) {
      lastGroup = s.currentGroupId
      await loadProfiles()
    }
    if (!s.running) {
      state.traffic.up = 0
      state.traffic.down = 0
      state.exit = null
    }
  })
  rt.on('profiles', async (groupId: number) => {
    if (groupId === state.app?.currentGroupId) await loadProfiles()
    selectedCache.value = undefined
    await refreshSelected()
  })
  rt.on('navigate', (page: Page) => (state.page = page))
  // automatic update check (off by default): only reported when there is news
  rt.on('update', (u: UpdateInfo) => {
    state.update = u
    const name = u.app.hasUpdate ? `NextBox ${u.app.latest}` : `sing-box ${u.core.latest}`
    toast(t('发现新版本 {0}，详见「设置 → 关于」', name))
  })
  rt.on('traffic', (t: { up: number; down: number }) => {
    state.traffic.up = t.up
    state.traffic.down = t.down
    state.traffic.history.push({ up: t.up, down: t.down })
    if (state.traffic.history.length > 60) state.traffic.history.shift()
  })
  rt.on('latency-begin', (b: { ids: number[]; mode: TestMode }) => {
    Object.assign(latencyTest, { mode: b.mode, total: b.ids.length, done: 0, current: 0 })
    latencyTest.waiting = new Set(b.ids)
    latencyTest.running = new Set()
  })
  rt.on('latency-start', (id: number) => {
    latencyTest.waiting.delete(id)
    latencyTest.running.add(id)
    latencyTest.current = id
  })
  rt.on('latency', (r: LatencyResult) => {
    latencyTest.waiting.delete(r.id)
    latencyTest.running.delete(r.id)
    latencyTest.done = Math.min(latencyTest.total, latencyTest.done + 1)
    if (latencyTest.current === r.id) latencyTest.current = [...latencyTest.running].pop() ?? 0
    const p = state.profiles.find((x) => x.id === r.id)
    if (!p) return
    if (!r.skipped) {
      p.latency = r.latency
      p.latencyError = r.error
      p.latencyKind = r.kind ?? (latencyTest.mode || undefined)
    }
    if (r.exit) Object.assign(p, { country: r.exit.country, countryIp: r.exit.ip, countrySource: 'exit', countryAt: Date.now() / 1000 })
  })
  for (const r of (await api.getSpeeds().catch(() => null)) ?? []) speedTest.results.set(r.id, r)
  rt.on('speed-begin', (ids: number[]) => {
    Object.assign(speedTest, { running: true, total: ids.length, done: 0, current: 0 })
    speedTest.waiting = new Set(ids)
    for (const id of ids) speedTest.results.delete(id)
  })
  rt.on('speed-start', (id: number) => {
    speedTest.waiting.delete(id)
    speedTest.current = id
  })
  rt.on('speed-tick', (r: SpeedResult) => speedTest.results.set(r.id, r))
  rt.on('speed', (r: SpeedResult) => {
    speedTest.waiting.delete(r.id)
    speedTest.results.set(r.id, r)
    speedTest.done = Math.min(speedTest.total, speedTest.done + 1)
    if (speedTest.current === r.id) speedTest.current = 0
  })
  rt.on('speed-done', () => {
    speedTest.running = false
    speedTest.current = 0
    speedTest.waiting = new Set()
  })
  rt.on('latency-done', () => {
    latencyTest.mode = ''
    latencyTest.waiting = new Set()
    latencyTest.running = new Set()
  })
}
