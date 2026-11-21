// Policy groups (分组代理) shared by the settings sections, and how rule
// targets are named.
import { reactive } from 'vue'
import { currentLang, t } from './i18n'
import { countryName, countryTerms } from './country'
import { pinyinInitials } from './pinyin'
import type { PickOption } from './components/MultiPick.vue'
import { api, type ItemKind, type PolicyView, type RuleItem, type RuleKind, type StepKind, type Target } from './api'
import { services } from './services'

export const policies = reactive({ list: [] as PolicyView[], loaded: false })

export async function refreshPolicies() {
  policies.list = await api.getPolicies().catch(() => [])
  policies.loaded = true
}

export const policyId = (target: Target) => (target.startsWith('policy:') ? Number(target.slice(7)) : 0)

/** The choices for where traffic goes. */
export function targetOptions(withBlock = true) {
  return [
    { value: 'proxy', label: t('当前节点'), group: 'basic' },
    { value: 'direct', label: t('直连'), group: 'basic' },
    ...(withBlock ? [{ value: 'block', label: t('拦截'), group: 'basic' }] : []),
    ...policies.list.map((p) => ({ value: 'policy:' + p.id, label: p.name, group: 'policy' })),
  ]
}

export function targetLabel(target: Target): string {
  switch (target) {
    case 'proxy':
      return t('当前节点')
    case 'direct':
      return t('直连')
    case 'block':
      return t('拦截')
  }
  const p = policies.list.find((x) => x.id === policyId(target))
  return p ? p.name : t('已删除的分组代理')
}

export const targetClass = (target: Target) => (target === 'direct' ? 'to-direct' : target === 'block' ? 'to-block' : 'to-proxy')

// ---- rules ----

/**
 * A kind of rule with its name, translated when it is read: these lists are
 * made when the module loads, before the language of the settings is set.
 */
function kind<K extends string>(value: K, zh: string, placeholder: string) {
  return {
    value,
    placeholder,
    get label() {
      return t(zh)
    },
  }
}

export const ruleKinds: { value: RuleKind; label: string; placeholder: string }[] = [
  kind<RuleKind>('geosite', '网站分类', ''),
  kind<RuleKind>('suffix', '域名', 'google.com, youtube.com'),
  kind<RuleKind>('domain', '完整域名', 'www.google.com, www.youtube.com'),
  kind<RuleKind>('keyword', '域名关键词', 'google, youtube'),
  kind<RuleKind>('regex', '域名正则', '^ads?\\.'),
  kind<RuleKind>('geoip', 'IP 归属地', ''),
  kind<RuleKind>('cidr', 'IP / IP 段', '8.8.8.8, 1.2.3.0/24'),
  kind<RuleKind>('process', '程序', 'chrome.exe, Telegram.exe'),
]

/** Android apps: kept in the rules, only NextBox for Android uses them. */
export const androidKind = kind<RuleKind>('package', '安卓应用', 'com.example.app')

/** A sing-box rule-set (.srs or .json) at an address: only inside rule sets. */
export const urlKind = kind<ItemKind>('url', '规则集网址', 'https://example.com/rules.srs')

/** What an item of a rule set can be. */
export const itemKinds: { value: ItemKind; label: string; placeholder: string }[] = [...ruleKinds, urlKind]

export const kindLabel = (k: string) => [...itemKinds, androidKind].find((x) => x.value === k)?.label ?? k

/** A rule set's contents in a few words, e.g. "域名 3 · 程序 1". */
export function itemsSummary(items: RuleItem[] = []) {
  return items
    .filter((it) => it.values.length)
    .map((it) => `${kindLabel(it.kind)} ${it.values.length}`)
    .join(' · ')
}

/** How a rule is named everywhere: its own name, its service, or what it matches. */
export function ruleTitle(r: { kind: string; values?: string[]; name?: string; app?: string; count?: number }) {
  if (r.name) return r.name
  const sv = r.app ? services.find((s) => s.id === r.app) : undefined
  if (sv) return sv.name
  if (r.kind === 'set') return t('规则集')
  const values = r.values ?? []
  const more = (r.count ?? values.length) - Math.min(values.length, 2)
  return `${kindLabel(r.kind)} ${values.slice(0, 2).join(', ')}${more > 0 ? t(' 等 {0} 个', (r.count ?? values.length)) : ''}`
}

/** Whether a rule matches anything yet: values, or a rule set's items. */
export const ruleFilled = (r: { kind: string; values: string[]; items?: RuleItem[] }) =>
  r.kind === 'set' ? (r.items ?? []).some((it) => it.values.length > 0) : r.values.length > 0

/** The scenes, as the route shows them. */
export const sceneInfo = (id: string) =>
  ({
    cn: { label: t('绕过中国大陆'), desc: t('国内网站和国内 IP') },
    ai: { label: t('AI 服务'), desc: 'ChatGPT · Claude · Gemini' },
    media: { label: t('流媒体'), desc: 'YouTube · Netflix · Disney+' },
    games: { label: t('游戏下载直连'), desc: t('Steam 等国内下载节点') },
  })[id] ?? { label: id, desc: '' }

export const sceneIds = ['ai', 'media', 'games', 'cn']

/** The scenes in the order the cards show them. */
export const sceneCards = () => ['cn', 'ai', 'media', 'games'].map((id) => ({ id, ...sceneInfo(id) }))

/** Ad blocking and the scenes: they have a place among the rules but are not rules of their own. */
export const isStep = (r: { kind: string }) => r.kind === 'ads' || r.kind === 'scene'

/** Where a connection that no rule matched goes. */
export const finalLabel = (routeMode: string) => (routeMode === 'direct' ? t('直连') : t('当前节点'))

/** The routing rules of a new installation (store.DefaultScenes and
 * store.placeSteps): no rules of your own, ad blocking first, the scenes
 * last with only the mainland China bypass on. */
export function defaultRoute(): { rules: ParsedRule[]; scenes: { id: string; enabled: boolean; target: Target }[] } {
  return {
    rules: [{ kind: 'ads', values: [], target: '' }, ...sceneIds.map((id) => ({ kind: 'scene' as const, values: [id], target: '' }))],
    scenes: [
      { id: 'cn', enabled: true, target: 'direct' },
      { id: 'ai', enabled: false, target: 'proxy' },
      { id: 'media', enabled: false, target: 'proxy' },
      { id: 'games', enabled: false, target: 'direct' },
    ],
  }
}

// translated when read: the list is made before the language is set
const geo = (value: string, zh: string) => ({
  value,
  get label() {
    return t(zh)
  },
})

const baseGeosites = [
  { value: 'google', label: 'Google' },
  { value: 'youtube', label: 'YouTube' },
  { value: 'openai', label: 'OpenAI / ChatGPT' },
  { value: 'anthropic', label: 'Claude (Anthropic)' },
  { value: 'google-gemini', label: 'Gemini' },
  geo('category-ai-!cn', 'AI 服务（境外）'),
  { value: 'telegram', label: 'Telegram' },
  { value: 'github', label: 'GitHub' },
  { value: 'microsoft', label: 'Microsoft' },
  { value: 'azure', label: 'Azure' },
  { value: 'netflix', label: 'Netflix' },
  { value: 'disney', label: 'Disney+' },
  { value: 'spotify', label: 'Spotify' },
  { value: 'twitter', label: 'Twitter / X' },
  { value: 'facebook', label: 'Facebook' },
  { value: 'instagram', label: 'Instagram' },
  { value: 'tiktok', label: 'TikTok' },
  { value: 'apple', label: 'Apple' },
  { value: 'steam', label: 'Steam' },
  geo('bilibili', '哔哩哔哩'),
  geo('cn', '中国大陆网站'),
  geo('geolocation-!cn', '境外网站'),
  geo('category-ads-all', '广告域名'),
]

/** The site categories offered in the rules: the common ones first, then
 * the rest of the service catalog. */
export const geosites = (() => {
  const out = [...baseGeosites]
  for (const sv of services) {
    for (const value of sv.sets) {
      if (!out.some((o) => o.value === value)) out.push({ value, label: sv.name })
    }
  }
  return out
})()

// IP regions: every country and region sing-geoip has a rule-set for
// (ISO 3166-1), the ones people pick most first, the rest by name. A search
// finds them by name in any language, by code, and by pinyin initials (日本 → rb).
const commonRegions = ['cn', 'hk', 'tw', 'mo', 'jp', 'sg', 'kr', 'us', 'gb', 'de', 'fr', 'nl', 'ca', 'au', 'ru', 'in', 'th', 'my', 'vn', 'ph', 'id', 'tr', 'ae']
const allRegions =
  'ad ae af ag ai al am ao aq ar as at au aw ax az ba bb bd be bf bg bh bi bj bl bm bn bo bq br bs bt bw by bz ca cc cd cf cg ch ci ck cl cm cn co cr cu cv cw cx cy cz de dj dk dm do dz ec ee eg eh er es et fi fj fk fm fo fr ga gb gd ge gf gg gh gi gl gm gn gp gq gr gs gt gu gw gy hk hn hr ht hu id ie il im in io iq ir is it je jm jo jp ke kg kh ki km kn kp kr kw ky kz la lb lc li lk lr ls lt lu lv ly ma mc md me mf mg mh mk ml mm mn mo mp mq mr ms mt mu mv mw mx my mz na nc ne nf ng ni nl no np nr nu nz om pa pe pf pg ph pk pl pm pn pr ps pt pw py qa re ro rs ru rw sa sb sc sd se sg sh si sj sk sl sm sn so sr ss st sv sx sy sz tc td tf tg th tj tk tl tm tn to tr tt tv tw tz ua ug um us uy uz va vc ve vg vi vn vu wf ws ye yt za zm zw'.split(' ')
const regionLabels: Record<string, () => string> = { cn: () => t('中国大陆'), hk: () => t('中国香港'), tw: () => t('中国台湾'), mo: () => t('中国澳门') }
const geoipCache = new Map<string, PickOption[]>()

/** The IP regions to pick from, named in the current language. */
export function geoipOptions(): PickOption[] {
  const lang = currentLang()
  let list = geoipCache.get(lang)
  if (!list) {
    const option = (code: string): PickOption => {
      const label = regionLabels[code]?.() ?? countryName(code.toUpperCase())
      const zh = countryName(code.toUpperCase(), 'zh-CN')
      const words = [...countryTerms(code.toUpperCase()), label.toLowerCase(), pinyinInitials(regionLabels[code]?.() ?? zh)]
      return { value: code, label, keywords: [...new Set(words)].join(' ') }
    }
    const zhOrder = new Intl.Collator('zh-Hans-CN')
    const rest = allRegions
      .filter((c) => !commonRegions.includes(c))
      .map(option)
      .sort((a, b) => (lang === 'en' ? a.label.localeCompare(b.label) : zhOrder.compare(a.label, b.label)))
    list = [...commonRegions.map(option), ...rest]
    geoipCache.set(lang, list)
  }
  return list
}

// ---- the text editor: one "kind:value, target" per line ----
// A rule set is a line "set:name, target" followed by its items, indented
// ("  kind:value"). A "!" in front of a rule or a rule set switches it off.

const kindAliases: Record<string, ItemKind> = {
  geosite: 'geosite', suffix: 'suffix', domain_suffix: 'suffix', domain: 'domain', full: 'domain',
  keyword: 'keyword', domain_keyword: 'keyword', regex: 'regex', regexp: 'regex', geoip: 'geoip',
  cidr: 'cidr', ip: 'cidr', ip_cidr: 'cidr', process: 'process', process_name: 'process',
  package: 'package', package_name: 'package', app: 'package', url: 'url',
}
const ipLike = /^(\d{1,3}(\.\d{1,3}){3}|[0-9a-f]*:[0-9a-f:]+)(\/\d{1,3})?$/i

export interface ParsedRule {
  kind: RuleKind | StepKind | 'set'
  values: string[]
  target: Target
  disabled?: boolean
  name?: string
  app?: string
  items?: RuleItem[]
}

/** Reads "kind:value", or a bare domain / IP / program / address, as one
 * value of a kind; null for an unknown kind. */
export function parseItem(text: string): { kind: ItemKind; value: string } | null {
  const left = text.trim()
  if (/^https?:\/\//i.test(left)) return { kind: 'url', value: left }
  const c = left.indexOf(':')
  const kind = c > 0 ? kindAliases[left.slice(0, c).toLowerCase()] : undefined
  if (kind) return { kind, value: left.slice(c + 1).trim() }
  if (c > 0 && !ipLike.test(left) && !left.slice(0, c).includes('.')) return null
  if (/\.exe$/i.test(left)) return { kind: 'process', value: left }
  return { kind: ipLike.test(left) ? 'cidr' : 'suffix', value: left }
}

/** Adds values to the item of their kind, or as a new item. */
export function addToItems(items: RuleItem[], kind: ItemKind, values: string[]) {
  const it = items.find((x) => x.kind === kind)
  if (it) {
    for (const v of values) if (!it.values.includes(v)) it.values.push(v)
  } else items.push({ kind, values: [...values] })
}

function targetText(target: Target) {
  const id = policyId(target)
  if (!id) return target
  const p = policies.list.find((x) => x.id === id)
  return p ? 'policy:' + p.name : 'proxy'
}

export function rulesToText(rules: ParsedRule[], sceneTargets: Record<string, Target> = {}): string {
  const lines = [
    t('# 每行一条：类型:值, 去向（proxy 当前节点 / direct 直连 / block 拦截 / policy:分组代理名称）'),
    t('# ads 是屏蔽广告的位置，scene:… 是常用场景的位置，可以上下移动'),
    t('# 规则集：set:名称, 去向，下面缩进的行是它的内容；行首加 ! 表示停用'),
  ]
  for (const r of rules) {
    const off = r.disabled ? '!' : ''
    if (r.kind === 'ads') lines.push('ads')
    else if (r.kind === 'scene') {
      const id = r.values[0]
      lines.push(id === 'cn' ? 'scene:cn' : `scene:${id}, ${targetText(sceneTargets[id] ?? 'proxy')}`)
    } else if (r.kind === 'set') {
      lines.push(`${off}set:${r.name ?? ''}, ${targetText(r.target)}`)
      for (const it of r.items ?? []) for (const v of it.values) lines.push(`  ${it.kind}:${v}`)
    } else for (const v of r.values) lines.push(`${off}${r.kind}:${v}, ${targetText(r.target)}`)
  }
  return lines.join('\n')
}

function parseTarget(right: string): Target | string[] {
  const lower = right.toLowerCase()
  if (lower === 'proxy' || lower === 'direct' || lower === 'block') return lower
  if (lower.startsWith('policy:')) {
    const name = right.slice(7).trim()
    const p = policies.list.find((x) => x.name === name)
    return p ? 'policy:' + p.id : ['policy', name]
  }
  return ['target', right]
}

/** Parses the text editor; errors are "line n: reason". */
export function textToRules(text: string): { rules: ParsedRule[]; errors: string[]; sceneTargets: Record<string, Target> } {
  const rules: ParsedRule[] = []
  const errors: string[] = []
  const sceneTargets: Record<string, Target> = {}
  let set: ParsedRule | null = null // the rule set whose items follow
  const targetError = (n: number, tg: string[]) =>
    tg[0] === 'policy' ? t('第 {0} 行：没有叫“{1}”的分组代理', n, tg[1]) : t('第 {0} 行：不认识的去向“{1}”', n, tg[1])
  text.split('\n').forEach((raw, i) => {
    const indented = /^\s/.test(raw)
    let line = raw.trim()
    if (!line || line.startsWith('#')) return
    if (indented && set) {
      const item = parseItem(line)
      if (!item) return errors.push(t('第 {0} 行：不认识的类型“{1}”', i + 1, line.slice(0, line.indexOf(':'))))
      if (!item.value) return errors.push(t('第 {0} 行：缺少值', i + 1))
      addToItems(set.items!, item.kind, [item.value])
      return
    }
    set = null
    const disabled = line.startsWith('!')
    if (disabled) line = line.slice(1).trim()
    // the places of ad blocking and of the scenes
    if (line.toLowerCase() === 'ads') return rules.push({ kind: 'ads', values: [], target: '' })
    if (line.toLowerCase().startsWith('scene:')) {
      const [idPart, targetPart] = line.slice(6).split(',')
      const id = idPart.trim().toLowerCase()
      if (!sceneIds.includes(id)) return errors.push(t('第 {0} 行：没有叫“{1}”的常用场景', i + 1, idPart.trim()))
      if (targetPart?.trim() && id !== 'cn') {
        const tg = parseTarget(targetPart.trim())
        if (Array.isArray(tg)) return errors.push(targetError(i + 1, tg))
        if (tg === 'block') return errors.push(t('第 {0} 行：常用场景不能设为拦截', i + 1))
        sceneTargets[id] = tg
      }
      return rules.push({ kind: 'scene', values: [id], target: '' })
    }
    const comma = line.lastIndexOf(',')
    if (comma < 0) return errors.push(t('第 {0} 行：缺少去向，例如 “{1}, direct”', i + 1, line))
    const left = line.slice(0, comma).trim()
    const target = parseTarget(line.slice(comma + 1).trim())
    if (Array.isArray(target)) return errors.push(targetError(i + 1, target))
    if (/^set:/i.test(left)) {
      const name = left.slice(4).trim()
      if (!name) return errors.push(t('第 {0} 行：规则集需要名称', i + 1))
      set = { kind: 'set', values: [], target, name, items: [], ...(disabled ? { disabled } : {}) }
      return rules.push(set)
    }
    const item = parseItem(left)
    if (!item) return errors.push(t('第 {0} 行：不认识的类型“{1}”', i + 1, left.slice(0, left.indexOf(':'))))
    if (item.kind === 'url') return errors.push(t('第 {0} 行：规则集网址只能写在规则集里', i + 1))
    if (!item.value) return errors.push(t('第 {0} 行：缺少值', i + 1))
    const kind = item.kind as RuleKind
    const last = rules[rules.length - 1]
    if (last && last.kind === kind && last.target === target && !!last.disabled === disabled) last.values.push(item.value)
    else rules.push({ kind, values: [item.value], target, ...(disabled ? { disabled } : {}) })
  })
  return { rules, errors, sceneTargets }
}

/** Gives rules read back from the text the names and services the same
 * rules had before (the text only says what rules match). */
export function keepLabels(parsed: ParsedRule[], before: ParsedRule[]) {
  const key = (r: ParsedRule) => `${r.kind}|${r.target}|${[...r.values].sort().join(',')}`
  const labels = new Map(before.filter((r) => r.kind !== 'set' && (r.name || r.app)).map((r) => [key(r), r]))
  for (const r of parsed) {
    const old = r.kind === 'set' ? undefined : labels.get(key(r))
    if (old?.name) r.name = old.name
    if (old?.app) r.app = old.app
  }
}
