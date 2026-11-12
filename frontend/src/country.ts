// Countries of nodes: flags, display names, guessing from node names, and
// matching search terms. Codes are ISO 3166-1 alpha-2, upper case.
import { currentLang, t, type Lang } from './i18n'
import type { Profile, Settings } from './api'

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
const places: [string, string[], string[]][] = [
  ['HK', ['香港', '港区', '港島', '九龙', '新界', '沙田'], ['hong kong', 'hongkong', 'hkg', 'hkt']],
  ['MO', ['澳门', '澳門'], ['macao', 'macau']],
  ['TW', ['台湾', '台灣', '台北', '新北', '台中', '高雄', '彰化'], ['taiwan', 'taipei', 'hinet', 'twn']],
  ['SG', ['新加坡', '狮城', '獅城'], ['singapore', 'sgp']],
  ['JP', ['日本', '东京', '東京', '大阪', '埼玉', '名古屋', '福冈', '横滨'], ['japan', 'tokyo', 'osaka', 'jpn']],
  ['KR', ['韩国', '韓國', '南韩', '首尔', '首爾', '春川', '釜山'], ['korea', 'seoul', 'chuncheon', 'kor']],
  ['US', ['美国', '美國', '洛杉矶', '洛杉磯', '圣何塞', '聖何塞', '硅谷', '矽谷', '西雅图', '纽约', '紐約', '芝加哥', '达拉斯', '凤凰城', '波特兰', '拉斯维加斯', '迈阿密', '阿什本', '弗吉尼亚', '亚特兰大', '旧金山', '费利蒙', '弗里蒙特'],
    ['united states', 'america', 'usa', 'los angeles', 'san jose', 'silicon valley', 'seattle', 'new york', 'chicago', 'dallas', 'phoenix', 'portland', 'las vegas', 'miami', 'ashburn', 'virginia', 'atlanta', 'san francisco', 'fremont']],
  ['CA', ['加拿大', '多伦多', '温哥华', '蒙特利尔'], ['canada', 'toronto', 'vancouver', 'montreal']],
  ['GB', ['英国', '英國', '伦敦', '倫敦', '曼彻斯特'], ['united kingdom', 'britain', 'england', 'london', 'manchester']],
  ['DE', ['德国', '德國', '法兰克福', '法蘭克福', '柏林', '杜塞尔多夫'], ['germany', 'frankfurt', 'berlin', 'dusseldorf']],
  ['FR', ['法国', '法國', '巴黎', '马赛'], ['france', 'paris', 'marseille']],
  ['NL', ['荷兰', '荷蘭', '阿姆斯特丹'], ['netherlands', 'holland', 'amsterdam']],
  ['RU', ['俄罗斯', '俄羅斯', '莫斯科', '圣彼得堡', '伯力', '新西伯利亚'], ['russia', 'moscow', 'petersburg', 'khabarovsk', 'novosibirsk']],
  ['ID', ['印尼', '印度尼西亚', '印度尼西亞', '雅加达'], ['indonesia', 'jakarta']],
  ['IN', ['印度', '孟买', '孟買', '班加罗尔', '海得拉巴'], ['india', 'mumbai', 'bangalore', 'hyderabad']],
  ['AU', ['澳大利亚', '澳大利亞', '澳洲', '悉尼', '墨尔本'], ['australia', 'sydney', 'melbourne']],
  ['NZ', ['新西兰', '紐西蘭', '奥克兰'], ['new zealand', 'auckland']],
  ['MY', ['马来西亚', '馬來西亞', '吉隆坡'], ['malaysia', 'kuala lumpur']],
  ['TH', ['泰国', '泰國', '曼谷'], ['thailand', 'bangkok']],
  ['VN', ['越南', '胡志明', '河内'], ['vietnam', 'hanoi', 'ho chi minh']],
  ['PH', ['菲律宾', '菲律賓', '马尼拉'], ['philippines', 'manila']],
  ['KH', ['柬埔寨', '金边'], ['cambodia', 'phnom penh']],
  ['TR', ['土耳其', '伊斯坦布尔'], ['turkey', 'turkiye', 'istanbul']],
  ['AE', ['阿联酋', '阿聯酋', '迪拜', '杜拜'], ['emirates', 'dubai', 'uae']],
  ['IL', ['以色列'], ['israel']],
  ['SA', ['沙特'], ['saudi']],
  ['BR', ['巴西', '圣保罗', '聖保羅'], ['brazil', 'sao paulo']],
  ['AR', ['阿根廷'], ['argentina']],
  ['CL', ['智利'], ['chile']],
  ['MX', ['墨西哥'], ['mexico']],
  ['ZA', ['南非', '约翰内斯堡'], ['south africa', 'johannesburg']],
  ['EG', ['埃及'], ['egypt']],
  ['NG', ['尼日利亚'], ['nigeria']],
  ['IT', ['意大利', '義大利', '米兰'], ['italy', 'milan']],
  ['ES', ['西班牙', '马德里'], ['spain', 'madrid']],
  ['CH', ['瑞士', '苏黎世'], ['switzerland', 'zurich']],
  ['SE', ['瑞典', '斯德哥尔摩'], ['sweden', 'stockholm']],
  ['NO', ['挪威'], ['norway']],
  ['FI', ['芬兰', '芬蘭', '赫尔辛基'], ['finland', 'helsinki']],
  ['DK', ['丹麦', '丹麥'], ['denmark']],
  ['PL', ['波兰', '波蘭', '华沙'], ['poland', 'warsaw']],
  ['UA', ['乌克兰', '烏克蘭', '基辅'], ['ukraine', 'kyiv', 'kiev']],
  ['IE', ['爱尔兰', '愛爾蘭', '都柏林'], ['ireland', 'dublin']],
  ['AT', ['奥地利', '奧地利', '维也纳'], ['austria', 'vienna']],
  ['BE', ['比利时', '比利時'], ['belgium']],
  ['PT', ['葡萄牙'], ['portugal']],
  ['RO', ['罗马尼亚'], ['romania']],
  ['HU', ['匈牙利'], ['hungary']],
  ['CZ', ['捷克'], ['czech']],
  ['GR', ['希腊', '希臘'], ['greece']],
  ['LU', ['卢森堡'], ['luxembourg']],
  ['IS', ['冰岛', '冰島'], ['iceland']],
  ['KZ', ['哈萨克斯坦'], ['kazakhstan']],
  ['MN', ['蒙古'], ['mongolia']],
  ['MM', ['缅甸'], ['myanmar']],
  ['BD', ['孟加拉'], ['bangladesh']],
  ['PK', ['巴基斯坦'], ['pakistan']],
  ['NP', ['尼泊尔'], ['nepal']],
  ['LK', ['斯里兰卡'], ['sri lanka']],
  ['QA', ['卡塔尔'], ['qatar']],
  ['IR', ['伊朗'], ['iran']],
  ['CN', ['中国', '中國', '回国', '回國', '大陆', '上海', '北京', '广州', '深圳', '杭州', '江苏', '成都'], ['china', 'mainland', 'shanghai', 'beijing', 'guangzhou', 'shenzhen']],
]

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
