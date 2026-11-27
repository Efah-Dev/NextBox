import { t } from './i18n'
import { reactive } from 'vue'

export function formatBytes(n: number): string {
  if (!n || n < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n < 10 && i > 0 ? n.toFixed(2) : n < 100 && i > 0 ? n.toFixed(1) : Math.round(n)} ${units[i]}`
}

export const formatSpeed = (n: number) => formatBytes(n) + '/s'

/** A short date and time: "11/26 14:30" this year, "2025/11/26" before. */
export function shortTime(unixSeconds: number): string {
  const d = new Date(unixSeconds * 1000)
  const pad = (x: number) => String(x).padStart(2, '0')
  if (d.getFullYear() !== new Date().getFullYear()) return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()}`
  return `${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function formatDuration(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const pad = (x: number) => String(x).padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${pad(m)}:${pad(sec)}`
}

export function latencyText(l: number): string {
  if (l > 0) return `${l} ms`
  if (l < 0) return t('超时')
  return t('未测试')
}

export function latencyClass(l: number): string {
  if (l < 0) return 'lat-bad'
  if (l === 0) return 'lat-none'
  if (l < 300) return 'lat-good'
  if (l < 800) return 'lat-mid'
  return 'lat-slow'
}

const typeNames: Record<string, string> = {
  shadowsocks: 'Shadowsocks',
  vmess: 'VMess',
  vless: 'VLESS',
  trojan: 'Trojan',
  hysteria2: 'Hysteria2',
  hysteria: 'Hysteria',
  tuic: 'TUIC',
  anytls: 'AnyTLS',
  socks: 'SOCKS',
  http: 'HTTP',
  wireguard: 'WireGuard',
  ssh: 'SSH',
  shadowtls: 'ShadowTLS',
  naive: 'NaiveProxy',
}

export const typeName = (t: string) => typeNames[t] ?? t

export function serverOf(ob: Record<string, any>): string {
  if (!ob?.server) return ''
  const host = String(ob.server).includes(':') ? `[${ob.server}]` : ob.server
  return ob.server_port ? `${host}:${ob.server_port}` : host
}

export interface MenuItem {
  label: string
  icon?: string
  hint?: string // second, smaller line
  danger?: boolean
  disabled?: boolean
  action: () => void
}

// ---- notices (bottom right, Notices.vue) ----

export interface Toast {
  id: number
  text: string
  kind: 'info' | 'success' | 'error'
  /** a button in the notice, such as 撤销 */
  action?: { label: string; run: () => void }
  /** how long it stays (ms); hovering it pauses the time */
  ms: number
}

export const toasts = reactive<Toast[]>([])
let toastSeq = 0
const MAX_TOASTS = 3

export function closeToast(id: number) {
  const i = toasts.findIndex((t) => t.id === id)
  if (i >= 0) toasts.splice(i, 1)
}

export function toast(text: string, kind: Toast['kind'] = 'info', action?: Toast['action']) {
  const id = ++toastSeq
  // the same message again (a repeated click) replaces the old one
  const same = toasts.findIndex((x) => x.text === text && x.kind === kind)
  if (same >= 0) toasts.splice(same, 1)
  const ms = kind === 'error' || action ? 6000 : text.length > 60 ? 4500 : 3500
  toasts.push({ id, text, kind, ms, action: action && { label: action.label, run: () => (closeToast(id), action.run()) } })
  while (toasts.length > MAX_TOASTS) toasts.shift()
}

/** Runs an async action and reports its result / error as a toast. */
export async function run<T>(fn: () => Promise<T>, success?: string | ((r: T) => string)): Promise<T | undefined> {
  try {
    const r = await fn()
    const msg = typeof success === 'function' ? success(r) : success
    if (msg) toast(msg, 'success')
    return r
  } catch (e) {
    toast((e as Error).message, 'error')
    return undefined
  }
}

// ---- simple confirm / prompt dialogs ----

/** A prompt field; warn returns a warning shown under the field ('' = none). */
export interface PromptInput {
  label: string
  value: string
  placeholder?: string
  warn?: (value: string) => string
}

/** Warns about plain-HTTP subscription URLs, which can be read or altered in transit. */
export const httpWarning = (v: string) =>
  /^http:\/\//i.test(v.trim()) ? t('HTTP 协议不安全：订阅内容以明文传输，可能被窃听或篡改，建议使用 HTTPS 地址') : ''

/** One option of choiceDialog. */
export interface DialogChoice {
  value: string
  label: string
  hint?: string
}

export interface DialogState {
  open: boolean
  title: string
  message: string
  inputs?: PromptInput[]
  choices?: DialogChoice[] // radio options (choiceDialog)
  choice?: string
  note?: string // small print under the options
  hint?: string // second, smaller line
  danger?: boolean
  confirmText?: string
  resolve?: (v: string[] | null) => void
}

export const dialog = reactive<DialogState>({ open: false, title: '', message: '' })

export function confirmDialog(title: string, message: string, danger = false, confirmText = ''): Promise<boolean> {
  return new Promise((resolve) => {
    Object.assign(dialog, {
      open: true, title, message, danger, inputs: undefined, choices: undefined, note: undefined,
      confirmText: confirmText || (danger ? t('删除') : t('确定')),
      resolve: (v: string[] | null) => resolve(v !== null),
    })
  })
}

export function promptDialog(
  title: string,
  inputs: (Omit<PromptInput, 'value'> & { value?: string })[],
  message = '',
): Promise<string[] | null> {
  return new Promise((resolve) => {
    Object.assign(dialog, {
      open: true, title, message, danger: false, confirmText: t('确定'), choices: undefined, note: undefined,
      inputs: inputs.map((i) => ({ ...i, value: i.value ?? '' })),
      resolve,
    })
  })
}

/** Asks to pick one of choices; resolves with the pick, or null when cancelled. */
export function choiceDialog(
  title: string,
  message: string,
  choices: DialogChoice[],
  selected: string,
  opts: { confirmText?: string; note?: string } = {},
): Promise<string | null> {
  return new Promise((resolve) => {
    Object.assign(dialog, {
      open: true, title, message, danger: false, inputs: undefined,
      choices, choice: selected, note: opts.note,
      confirmText: opts.confirmText || t('确定'),
      resolve: (v: string[] | null) => resolve(v ? v[0] : null),
    })
  })
}

/** A group's display name; the automatic default group follows the UI language. */
export const groupName = (g?: { name: string; default?: boolean }) => (g ? (g.default ? t('默认分组') : g.name) : '')
