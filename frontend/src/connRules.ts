// Rules made from a connection (连接页右键): what the connection can be
// matched by, and adding that to a rule set or as a new rule.
import { t } from './i18n'
import { api, type Connection, type Rule, type RuleKind, type Target } from './api'
import { patchSettings, state } from './state'
import { targetLabel } from './policies'
import { toast } from './util'

export interface ConnMatch {
  kind: RuleKind
  value: string
  label: string
}

const processName = (p: string) => p.split(/[\\/]/).pop()?.replace(/ \(.*\)$/, '') ?? ''
const isIP = (s: string) => /^[\d.]+$/.test(s) || s.includes(':')

/**
 * The ways the connection can be matched, the first one being the default:
 * the site with its subdomains, the full name, the IP, the program.
 */
export async function connMatches(c: Connection): Promise<ConnMatch[]> {
  const out: ConnMatch[] = []
  const host = c.metadata.host?.trim()
  if (host && !isIP(host)) {
    const root = (await api.rootDomain(host).catch(() => host)) || host
    out.push({ kind: 'suffix', value: root, label: t('{0} 及子域名', root) })
    if (root !== host) out.push({ kind: 'domain', value: host, label: t('只有 {0}', host) })
  } else {
    const ip = host || c.metadata.destinationIP
    if (ip) out.push({ kind: 'cidr', value: ip, label: t('IP {0}', ip) })
  }
  const proc = processName(c.metadata.processPath ?? '')
  if (proc) out.push({ kind: 'process', value: proc, label: t('程序 {0}', proc) })
  return out
}

const cnIndex = (rules: Rule[]) => rules.findIndex((r) => r.kind === 'scene' && r.values[0] === 'cn')

// New rules and rule sets go before the mainland China bypass, as on the rules page.
function insertAt(rules: Rule[]) {
  const cn = cnIndex(rules)
  return cn < 0 ? rules.length : cn
}

async function save(prev: Rule[], next: Rule[], done: string) {
  await patchSettings({ rules: next })
  toast(done, 'success', { label: t('撤销'), run: () => patchSettings({ rules: prev }) })
}

const copy = (rules: Rule[]): Rule[] => JSON.parse(JSON.stringify(rules))

/** Adds m to the rule set at index (Settings.rules). */
export async function addToSet(index: number, m: ConnMatch) {
  const prev = copy(state.app?.settings.rules ?? [])
  const next = copy(prev)
  const set = next[index]
  if (!set || set.kind !== 'set') return
  set.items ??= []
  let item = set.items.find((i) => i.kind === m.kind)
  if (!item) set.items.push((item = { kind: m.kind, values: [] }))
  if (item.values.includes(m.value)) return toast(t('「{0}」里已经有 {1}', set.name ?? '', m.value))
  item.values.push(m.value)
  await save(prev, next, t('已把 {0} 加入「{1}」', m.value, set.name ?? ''))
}

/** Adds m as a rule of its own that sends it to target. */
export async function addRule(m: ConnMatch, target: Target) {
  const prev = copy(state.app?.settings.rules ?? [])
  const next = copy(prev)
  next.splice(insertAt(next), 0, { kind: m.kind, values: [m.value], target })
  await save(prev, next, t('已新建规则：{0} → {1}', m.value, targetLabel(target)))
}

/** Makes a new rule set with m, named after it, sending it through the current node. */
export async function addNewSet(m: ConnMatch) {
  const prev = copy(state.app?.settings.rules ?? [])
  const next = copy(prev)
  next.splice(insertAt(next), 0, { kind: 'set', name: m.value, values: [], items: [{ kind: m.kind, values: [m.value] }], target: 'proxy' })
  await save(prev, next, t('已新建规则集「{0}」，去向是当前节点，可在设置 → 分流规则里修改', m.value))
}
