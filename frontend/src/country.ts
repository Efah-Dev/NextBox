// Countries of nodes: flags, display names, guessing from node names, and
// matching search terms. Codes are ISO 3166-1 alpha-2, upper case.
import { currentLang, t, type Lang } from './i18n'
import type { Profile, Settings } from './api'
import placesData from './places.json'

// Flags are emoji (regional indicator pairs). Windows has no flag glyphs, so
// main.ts loads a small flag-only font (Twemoji Country Flags).
export function flagEmoji(code: string): string {
  if (!/^[A-Z]{2}$/.test(code)) return ''
  return String.fromCodePoint(...[...code].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65))
}

/**
 * Words that point to a country in node names, checked in this order: a
 * more specific place comes before one it contains (香港 before 中国, 印尼
 * before 印度). Chinese entries match anywhere; Latin ones only as whole words.
 */
// shared with the backend (internal/geo), which matches policy groups by country
const places = placesData as [string, string[], string[]][]

// Bare codes (HK, JP-01, [US]) count only when written in upper case.
const codeAliases: Record<string, string> = { UK: 'GB' }
const codes = new Set(places.map((p) => p[0]))

const latin = places.map(([code, , words]) => [code, new RegExp(`(^|[^a-z])(${words.map(escape).join('|')})($|[^a-z])`)] as const)

function escape(s: string) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

const guessCache = new Map<string, string>()

/** Guesses the country from a node name; '' when nothing points to one. */
export function guessCountry(name: string): string {
  let code = guessCache.get(name)
  if (code === undefined) {
    code = guess(name)
    guessCache.set(name, code)
  }
  return code
}

function guess(name: string): string {
  // a flag emoji says it best
  const flag = /[\u{1F1E6}-\u{1F1FF}]{2}/u.exec(name)
  if (flag) return [...flag[0]].map((c) => String.fromCharCode(c.codePointAt(0)! - 0x1f1e6 + 65)).join('')
  for (const [code, zh] of places) if (zh.some((w) => name.includes(w))) return code
  const lower = name.toLowerCase()
  for (const [code, re] of latin) if (re.test(lower)) return code
  for (const m of name.matchAll(/(?:^|[^A-Za-z])([A-Z]{2})(?=$|[^A-Za-z])/g)) {
    const code = codeAliases[m[1]] ?? m[1]
    if (codes.has(code)) return code
  }
  return ''
}

// ---- names ----

// Intl names for these are long ("中国香港特别行政区") or not what users say.
const shortNames: Record<string, Record<string, string>> = {
  HK: { 'zh-CN': '香港', 'zh-TW': '香港', en: 'Hong Kong' },
  MO: { 'zh-CN': '澳门', 'zh-TW': '澳門', en: 'Macao' },
  AE: { 'zh-CN': '阿联酋', 'zh-TW': '阿聯酋', en: 'UAE' },
}
const intl = new Map<string, Intl.DisplayNames>()
function regionNames(lang: string) {
  if (!intl.has(lang)) {
    const tag = lang === 'zh-CN' ? 'zh-Hans' : lang === 'zh-TW' ? 'zh-Hant' : 'en'
    intl.set(lang, new Intl.DisplayNames([tag], { type: 'region' }))
  }
  return intl.get(lang)!
}

/** The country's name in the given (default: current) UI language. */
export function countryName(code: string, lang = currentLang()): string {
  if (!code) return ''
  try {
    return shortNames[code]?.[lang] ?? regionNames(lang).of(code) ?? code
  } catch {
    return code
  }
}

const termCache = new Map<string, string[]>()

/** Everything a search for this country may be typed as, lower case. */
export function countryTerms(code: string): string[] {
  if (!code) return []
  let terms = termCache.get(code)
  if (!terms) {
    const place = places.find((p) => p[0] === code)
    terms = [
      code.toLowerCase(),
      ...(['zh-CN', 'zh-TW', 'en'] as Lang[]).map((l) => countryName(code, l).toLowerCase()),
      ...(place ? [...place[1], ...place[2]] : []),
    ]
    termCache.set(code, [...new Set(terms)])
    terms = termCache.get(code)!
  }
  return terms
}

/** Whether the search text names this country (香港, hong kong, hk, 港 …). */
export function countryMatches(code: string, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (!code || !q) return false
  // a single Latin letter would match half the world
  if (/^[a-z]$/.test(q)) return false
  return countryTerms(code).some((term) => term.includes(q))
}

// ---- a node's country ----

export interface NodeCountry {
  code: string
  /** detected = looked up from an IP; guessed = read from the name */
  how: 'detected' | 'guessed' | ''
  ip?: string
  at?: number
}

type Located = Pick<Profile, 'name' | 'country' | 'countryIp' | 'countrySource' | 'countryAt'> & {
  members?: { name: string; country?: string; countrySource?: string; missing?: boolean }[]
}

/** Where a node is, as shown in the UI: the detection that matches the
 *  chosen source (Settings.countrySource), otherwise a guess from the name.
 *  A chain proxy without its own answer falls back to its last hop. */
export function nodeCountry(p: Located, s: Pick<Settings, 'countrySource'> | undefined): NodeCountry {
  const source = s?.countrySource ?? 'exit'
  if (p.country && p.countrySource === source) {
    return { code: p.country, how: 'detected', ip: p.countryIp, at: p.countryAt }
  }
  const exit = p.members?.length ? p.members[p.members.length - 1] : undefined
  if (exit && !exit.missing && exit.country && exit.countrySource === source) return { code: exit.country, how: 'detected' }
  const code = guessCountry(p.name) || (exit && !exit.missing ? guessCountry(exit.name) : '')
  return { code, how: code ? 'guessed' : '' }
}

/** Tooltip for a flag. */
export function countryTitle(c: NodeCountry, s: Pick<Settings, 'countrySource'> | undefined): string {
  if (!c.code) return t('地区未知：URL 测试后显示')
  const name = countryName(c.code)
  if (c.how === 'guessed') return t('{0}（根据名称推测）', name)
  const what = (s?.countrySource ?? 'exit') === 'server' ? t('服务器 IP') : t('落地 IP')
  return c.ip ? t('{0} · {1} {2}', name, what, c.ip) : t('{0}（按{1}判断）', name, what)
}

/** A node name without its leading flag emoji, which the flag in front of
 *  it already shows ("🇭🇰 香港 01" → "香港 01"). */
export function bareName(name: string): string {
  return name.replace(/^\s*(?:[\u{1F1E6}-\u{1F1FF}]{2}\s*)+/u, '').trim() || name
}
