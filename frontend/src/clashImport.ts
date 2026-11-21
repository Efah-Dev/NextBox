// The question which parts of a Clash subscription besides its nodes are
// taken over (ClashImportDialog): asked once after adding or updating a
// subscription that has them, and again from the group menu.
import { reactive } from 'vue'
import { t } from './i18n'
import { api, type ClashImport, type ClashInfo } from './api'
import { state } from './state'
import { refreshPolicies } from './policies'
import { run, toast } from './util'

export const clashDialog = reactive({
  open: false,
  groupId: 0,
  name: '',
  info: null as ClashInfo | null,
  // rules are not taken by default: they would change the user's own routing
  opt: { groups: true, rules: false, dns: true, subDns: true } as ClashImport,
})

/**
 * Opens the dialog for a subscription; fetch downloads it when needed (the
 * group menu); onlyNew skips a subscription that was asked about before.
 */
export async function askClash(groupId: number, fetch = false, onlyNew = false) {
  let info: ClashInfo | null = null
  try {
    info = await api.clashPreview(groupId, fetch)
  } catch (e) {
    if (fetch) toast((e as Error).message, 'error')
    return
  }
  if (!info) {
    if (fetch) toast(t('这个订阅不是 Clash 格式，没有分组和规则可以导入'))
    return
  }
  if (onlyNew && info.current) return
  const g = state.app?.groups.find((x) => x.id === groupId)
  Object.assign(clashDialog, {
    open: true,
    groupId,
    name: g?.name ?? '',
    info,
    // the subscription's own DNS is taken by default: it is only offered
    // when the config names a server that is not a public one
    opt: info.current
      ? { ...info.current }
      : { groups: !!info.preview.groups?.length, rules: false, dns: info.preview.dns > 0, subDns: !!info.preview.subDns },
  })
}

/** After adding or updating: asks about the first subscription that was never asked. */
export async function askNewClash(ids: number[]) {
  for (const id of ids) {
    // a group just added may not be in the state yet
    const g = state.app?.groups.find((x) => x.id === id)
    if (g && (!g.url || g.clash)) continue
    await askClash(id, false, true)
    if (clashDialog.open) return
  }
}

export async function applyClash(opt: ClashImport) {
  clashDialog.open = false
  const msg = await run(() => api.applyClash(clashDialog.groupId, opt), (m) => m || t('只导入节点'))
  if (msg !== undefined) refreshPolicies()
}
