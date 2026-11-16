// Policy groups (分组代理) shared by the settings sections, and how rule
// targets are named.
import { reactive } from 'vue'
import { t } from './i18n'
import { api, type PolicyView, type RuleKind, type StepKind, type Target } from './api'
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

export const ruleKinds: { value: RuleKind; label: string; placeholder: string }[] = [
  { value: 'geosite', label: t('网站分类'), placeholder: '' },
  { value: 'suffix', label: t('域名'), placeholder: 'google.com, youtube.com' },
  { value: 'domain', label: t('完整域名'), placeholder: 'www.google.com' },
  { value: 'keyword', label: t('域名关键词'), placeholder: 'google' },
  { value: 'regex', label: t('域名正则'), placeholder: '^ads?\\.' },
  { value: 'geoip', label: t('IP 归属地'), placeholder: '' },
  { value: 'cidr', label: t('IP / IP 段'), placeholder: '8.8.8.8, 1.2.3.0/24' },
  { value: 'process', label: t('程序'), placeholder: 'chrome.exe' },
]

/** Android apps: kept in the rules, only NextBox for Android uses them. */
export const androidKind = { value: 'package' as RuleKind, label: t('安卓应用'), placeholder: 'com.example.app' }

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

const baseGeosites = [
  { value: 'google', label: 'Google' },
  { value: 'youtube', label: 'YouTube' },
  { value: 'openai', label: 'OpenAI / ChatGPT' },
  { value: 'anthropic', label: 'Claude (Anthropic)' },
  { value: 'google-gemini', label: 'Gemini' },
  { value: 'category-ai-!cn', label: t('AI 服务（境外）') },
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
  { value: 'bilibili', label: t('哔哩哔哩') },
  { value: 'cn', label: t('中国大陆网站') },
  { value: 'geolocation-!cn', label: t('境外网站') },
  { value: 'category-ads-all', label: t('广告域名') },
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

export const geoips = [
  { value: 'cn', label: t('中国大陆') },
  { value: 'hk', label: t('中国香港') },
  { value: 'tw', label: t('中国台湾') },
  { value: 'jp', label: t('日本') },
  { value: 'sg', label: t('新加坡') },
  { value: 'kr', label: t('韩国') },
  { value: 'us', label: t('美国') },
]

// ---- the text editor: one "kind:value, target" per line ----

const kindAliases: Record<string, RuleKind> = {
  geosite: 'geosite', suffix: 'suffix', domain_suffix: 'suffix', domain: 'domain', full: 'domain',
  keyword: 'keyword', domain_keyword: 'keyword', regex: 'regex', regexp: 'regex', geoip: 'geoip',
  cidr: 'cidr', ip: 'cidr', ip_cidr: 'cidr', process: 'process', package: 'package', package_name: 'package', app: 'package',
}
const ipLike = /^(\d{1,3}(\.\d{1,3}){3}|[0-9a-f]*:[0-9a-f:]+)(\/\d{1,3})?$/i

export interface ParsedRule {
  kind: RuleKind | StepKind
  values: string[]
  target: Target
  disabled?: boolean
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
  ]
  for (const r of rules) {
    if (r.kind === 'ads') lines.push('ads')
    else if (r.kind === 'scene') {
      const id = r.values[0]
      lines.push(id === 'cn' ? 'scene:cn' : `scene:${id}, ${targetText(sceneTargets[id] ?? 'proxy')}`)
    } else for (const v of r.values) lines.push(`${r.disabled ? '# ' : ''}${r.kind}:${v}, ${targetText(r.target)}`)
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
  text.split('\n').forEach((raw, i) => {
    const line = raw.trim()
    if (!line || line.startsWith('#')) return
    // the places of ad blocking and of the scenes
    if (line.toLowerCase() === 'ads') return rules.push({ kind: 'ads', values: [], target: '' })
    if (line.toLowerCase().startsWith('scene:')) {
      const [idPart, targetPart] = line.slice(6).split(',')
      const id = idPart.trim().toLowerCase()
      if (!sceneIds.includes(id)) return errors.push(t('第 {0} 行：没有叫“{1}”的常用场景', i + 1, idPart.trim()))
      if (targetPart?.trim() && id !== 'cn') {
        const tg = parseTarget(targetPart.trim())
        if (Array.isArray(tg))
          return errors.push(tg[0] === 'policy' ? t('第 {0} 行：没有叫“{1}”的分组代理', i + 1, tg[1]) : t('第 {0} 行：不认识的去向“{1}”', i + 1, tg[1]))
        if (tg === 'block') return errors.push(t('第 {0} 行：常用场景不能设为拦截', i + 1))
        sceneTargets[id] = tg
      }
      return rules.push({ kind: 'scene', values: [id], target: '' })
    }
    const comma = line.lastIndexOf(',')
    if (comma < 0) return errors.push(t('第 {0} 行：缺少去向，例如 “{1}, direct”', i + 1, line))
    const left = line.slice(0, comma).trim()
    const right = line.slice(comma + 1).trim()
    let target: Target
    const lower = right.toLowerCase()
    if (lower === 'proxy' || lower === 'direct' || lower === 'block') target = lower
    else if (lower.startsWith('policy:')) {
      const name = right.slice(7).trim()
      const p = policies.list.find((x) => x.name === name)
      if (!p) return errors.push(t('第 {0} 行：没有叫“{1}”的分组代理', i + 1, name))
      target = 'policy:' + p.id
    } else return errors.push(t('第 {0} 行：不认识的去向“{1}”', i + 1, right))
    const c = left.indexOf(':')
    let kind: RuleKind | undefined = c > 0 ? kindAliases[left.slice(0, c).toLowerCase()] : undefined
    let value = kind ? left.slice(c + 1).trim() : left
    if (!kind) {
      if (c > 0 && !ipLike.test(left) && !left.slice(0, c).includes('.'))
        return errors.push(t('第 {0} 行：不认识的类型“{1}”', i + 1, left.slice(0, c)))
      kind = ipLike.test(left) ? 'cidr' : 'suffix'
      value = left
    }
    if (!value) return errors.push(t('第 {0} 行：缺少值', i + 1))
    const last = rules[rules.length - 1]
    if (last && last.kind === kind && last.target === target) last.values.push(value)
    else rules.push({ kind, values: [value], target })
  })
  return { rules, errors, sceneTargets }
}
