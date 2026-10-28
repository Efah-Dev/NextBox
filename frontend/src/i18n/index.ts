// UI translation. Source strings are Simplified Chinese and double as keys:
//   en.json     Simplified Chinese -> English (shared with the Go backend)
//   zh-TW.json  Taiwan vocabulary phrases + simplified -> traditional characters
// Placeholders are written {0}, {1}, … : t('已删除 {0} 个节点', n).
// t() is not reactive; App.vue re-renders everything when the language changes.
import en from './en.json'
import tw from './zh-TW.json'

export type Lang = 'zh-CN' | 'zh-TW' | 'en'

export const languages: { value: Lang; label: string }[] = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
  { value: 'en', label: 'English' },
]

let lang: Lang = 'zh-CN'

export function setLang(l: string | undefined) {
  lang = languages.some((x) => x.value === l) ? (l as Lang) : 'zh-CN'
  document.documentElement.lang = lang
}

const enDict = en as Record<string, string>

// longest phrase first, so 服务器 wins over 服务
const phrases = (tw.phrases as [string, string][]).slice().sort((a, b) => b[0].length - a[0].length)
const chars = new Map<string, string>()
{
  const from = Array.from(tw.chars.from)
  const to = Array.from(tw.chars.to)
  from.forEach((c, i) => chars.set(c, to[i]))
}
const twCache = new Map<string, string>()

export function toTraditional(s: string): string {
  const hit = twCache.get(s)
  if (hit !== undefined) return hit
  let out = ''
  for (let i = 0; i < s.length; ) {
    const p = phrases.find(([from]) => s.startsWith(from, i))
    if (p) {
      out += p[1]
      i += p[0].length
      continue
    }
    const c = String.fromCodePoint(s.codePointAt(i)!)
    out += chars.get(c) ?? c
    i += c.length
  }
  twCache.set(s, out)
  return out
}

export function t(s: string, ...args: unknown[]): string {
  let out = lang === 'en' ? enDict[s] ?? s : lang === 'zh-TW' ? toTraditional(s) : s
  if (args.length) out = out.replace(/\{(\d+)\}/g, (m, i) => (Number(i) < args.length ? String(args[Number(i)]) : m))
  return out
}
