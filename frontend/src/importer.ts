// One way in for everything the user hands NextBox: the import button,
// Ctrl+V on the home and node pages, and files dropped on the window. What
// it is comes from the content (link.Classify / InspectFile), never from
// where it came from.
import { t } from './i18n'
import { api, type Classified } from './api'
import { askNewClash } from './clashImport'
import { nodesAdded, state } from './state'
import { choiceDialog, httpWarning, promptDialog, run, toast } from './util'

let busy = false

/** Imports pasted text: a subscription address or share links. */
export async function importText(text: string) {
  if (!text.trim()) return toast(t('剪贴板是空的'), 'error')
  if (busy) return
  busy = true
  try {
    const c = await api.classifyText(text).catch(() => ({ kind: 'none' }) as Classified)
    if (c.kind === 'subscription') await addSubscription(c, text)
    else if (c.kind === 'nodes') await importNodes(text)
    else toast(t('没有找到可以导入的节点或订阅'), 'error')
  } finally {
    busy = false
  }
}

async function importNodes(text: string) {
  const group = state.app?.currentGroupId ?? 0
  if (await run(() => api.importText(text, group), (m) => m)) nodesAdded()
}

/**
 * Adds a subscription: named by the link, else by its server, else by the
 * user. An address that looks like an HTTP proxy asks first which it is.
 */
async function addSubscription(c: Classified, text: string) {
  const url = c.url ?? ''
  if (c.likelyProxy) {
    const pick = await choiceDialog(
      t('这是订阅还是代理节点？'),
      url,
      [
        { value: 'sub', label: t('订阅地址'), hint: t('下载其中的节点，新建一个分组') },
        { value: 'node', label: t('HTTP 代理节点'), hint: t('作为一个节点加入当前分组') },
      ],
      'sub',
    )
    if (!pick) return
    if (pick === 'node') return importNodes(text.includes('://') ? text : 'http://' + text)
  }
  const warn = httpWarning(url)
  if (warn) toast(warn)
  if (c.named) {
    const ok = await run(() => api.addSubscriptionAuto(url, c.name ?? ''), (r) => r.message ?? '')
    if (ok) added()
    return
  }
  toast(t('正在读取订阅…'))
  const r = await run(() => api.addSubscriptionAuto(url, ''))
  if (!r) return
  if (!r.needName) {
    toast(r.message ?? '', 'success')
    return added()
  }
  const name = await promptDialog(t('添加订阅'), [{ label: t('分组名称'), value: r.suggest ?? '', placeholder: t('例如：机场 A') }], t('订阅没有提供名称，请为它的分组起个名字。'))
  if (!name) return
  if (await run(() => api.addSubscription(name[0], url), (m) => m)) added()
}

function added() {
  nodesAdded()
  askNewClash([state.app?.currentGroupId ?? 0])
}

/** Imports files dropped on the window, one after another. */
export async function importFiles(paths: string[]) {
  for (const path of paths) {
    const f = await run(() => api.inspectFile(path))
    if (!f) continue
    switch (f.kind) {
      case 'backup':
        state.settingsSection = 'backup'
        state.backupFile = path
        state.page = 'settings'
        return // the import dialog takes over
      case 'config': {
        const ok = await run(() => api.importConfigFile(path), (m) => m)
        if (ok) {
          state.page = 'nodes'
          added()
        }
        break
      }
      case 'subscription':
        await addSubscription(f.link, f.link.url ?? '')
        break
      case 'nodes':
        if (await run(() => api.importLinksFile(path, state.app?.currentGroupId ?? 0), (m) => m)) nodesAdded()
        break
    }
  }
}
