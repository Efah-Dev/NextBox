import { computed, reactive, ref } from 'vue'
import { api, rt, type AppState, type LatencyResult, type LogBatch, type Profile, type Settings } from './api'

export type Page = 'home' | 'nodes' | 'connections' | 'logs' | 'settings'

export const state = reactive({
  page: 'home' as Page,
  app: null as AppState | null,
  profiles: [] as Profile[],
  traffic: { up: 0, down: 0, history: [] as { up: number; down: number }[] },
  logs: [] as string[],
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
  rt.on('traffic', (t: { up: number; down: number }) => {
    state.traffic.up = t.up
    state.traffic.down = t.down
    state.traffic.history.push({ up: t.up, down: t.down })
    if (state.traffic.history.length > 60) state.traffic.history.shift()
  })
  rt.on('latency', (r: LatencyResult) => {
    const p = state.profiles.find((x) => x.id === r.id)
    if (p) {
      p.latency = r.latency
      p.latencyError = r.error
    }
  })
}
