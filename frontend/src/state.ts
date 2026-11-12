import { computed, reactive, ref } from 'vue'
import { api, rt, type AppState, type LatencyResult, type LogBatch, type Profile, type Settings, type UpdateInfo } from './api'
import { t } from './i18n'
import { toast } from './util'

export type Page = 'home' | 'nodes' | 'connections' | 'logs' | 'settings'

export const state = reactive({
  page: 'home' as Page,
  app: null as AppState | null,
  profiles: [] as Profile[],
  traffic: { up: 0, down: 0, history: [] as { up: number; down: number }[] },
  logs: [] as string[],
  /** Settings section to scroll to when the settings page opens. */
  settingsSection: '',
  /** Result of the last update check (设置 → 关于). */
  update: null as UpdateInfo | null,
  /** the one-time explanation of how countries are found (CountryNotice) */
  countryNotice: false,
})

/** Called after nodes were added: the first time, explains where the flags
 *  come from and lets the user pick (落地 IP / 服务器 IP). */
export function nodesAdded() {
  if (state.app && !state.app.settings.countryNoticeShown) state.countryNotice = true
}

/** Progress of the running latency test (mode '' = none). */
export const latencyTest = reactive({
  mode: '' as '' | 'url' | 'tcp',
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
  rt.on('latency-begin', (b: { ids: number[]; mode: 'url' | 'tcp' }) => {
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
    }
    if (r.exit) Object.assign(p, { country: r.exit.country, countryIp: r.exit.ip, countrySource: 'exit', countryAt: Date.now() / 1000 })
  })
  rt.on('latency-done', () => {
    latencyTest.mode = ''
    latencyTest.waiting = new Set()
    latencyTest.running = new Set()
  })
}
