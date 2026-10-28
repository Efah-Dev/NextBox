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
  danger?: boolean
  disabled?: boolean
  action: () => void
}

// ---- toasts (PCL-style hint bar, bottom left) ----

export interface Toast {
  id: number
  text: string
  kind: 'info' | 'success' | 'error'
}

export const toasts = reactive<Toast[]>([])
let toastSeq = 0

export function toast(text: string, kind: Toast['kind'] = 'info') {
  const id = ++toastSeq
  toasts.push({ id, text, kind })
  setTimeout(() => {
    const i = toasts.findIndex((t) => t.id === id)
    if (i >= 0) toasts.splice(i, 1)
  }, kind === 'error' ? 6000 : 3000)
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

export interface DialogState {
  open: boolean
  title: string
  message: string
  inputs?: PromptInput[]
  danger?: boolean
  confirmText?: string
  resolve?: (v: string[] | null) => void
}

export const dialog = reactive<DialogState>({ open: false, title: '', message: '' })

export function confirmDialog(title: string, message: string, danger = false): Promise<boolean> {
  return new Promise((resolve) => {
    Object.assign(dialog, {
      open: true, title, message, danger, inputs: undefined,
      confirmText: danger ? t('删除') : t('确定'),
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
      open: true, title, message, danger: false, confirmText: t('确定'),
      inputs: inputs.map((i) => ({ ...i, value: i.value ?? '' })),
      resolve,
    })
  })
}

/** A group's display name; the automatic default group follows the UI language. */
export const groupName = (g?: { name: string; default?: boolean }) => (g ? (g.default ? t('默认分组') : g.name) : '')
