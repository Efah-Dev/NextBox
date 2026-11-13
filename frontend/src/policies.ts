// Policy groups (分组代理) shared by the settings sections, and how rule
// targets are named.
import { reactive } from 'vue'
import { t } from './i18n'
import { api, type PolicyView, type RuleKind, type Target } from './api'

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

export const geosites = [
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

export const geoips = [
  { value: 'cn', label: t('中国大陆') },
  { value: 'hk', label: t('中国香港') },
  { value: 'tw', label: t('中国台湾') },
  { value: 'jp', label: t('日本') },
  { value: 'sg', label: t('新加坡') },
  { value: 'kr', label: t('韩国') },
  { value: 'us', label: t('美国') },
]

export const valueLabel = (kind: RuleKind, v: string) =>
  (kind === 'geosite' ? geosites : kind === 'geoip' ? geoips : []).find((x) => x.value === v)?.label ?? v

// ---- the text editor: one "kind:value, target" per line ----

const kindAliases: Record<string, RuleKind> = {
  geosite: 'geosite', suffix: 'suffix', domain_suffix: 'suffix', domain: 'domain', full: 'domain',
  keyword: 'keyword', domain_keyword: 'keyword', regex: 'regex', regexp: 'regex', geoip: 'geoip',
  cidr: 'cidr', ip: 'cidr', ip_cidr: 'cidr', process: 'process',
}
const ipLike = /^(\d{1,3}(\.\d{1,3}){3}|[0-9a-f]*:[0-9a-f:]+)(\/\d{1,3})?$/i

export interface ParsedRule {
  kind: RuleKind
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

export function rulesToText(rules: ParsedRule[]): string {
  const lines = [t('# 每行一条：类型:值, 去向（proxy 当前节点 / direct 直连 / block 拦截 / policy:分组代理名称）')]
  for (const r of rules) {
    for (const v of r.values) lines.push(`${r.disabled ? '# ' : ''}${r.kind}:${v}, ${targetText(r.target)}`)
  }
  return lines.join('\n')
}

/** Parses the text editor; errors are "line n: reason". */
export function textToRules(text: string): { rules: ParsedRule[]; errors: string[] } {
  const rules: ParsedRule[] = []
  const errors: string[] = []
  text.split('\n').forEach((raw, i) => {
    const line = raw.trim()
    if (!line || line.startsWith('#')) return
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
  return { rules, errors }
}
