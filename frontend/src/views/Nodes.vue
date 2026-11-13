<script setup lang="ts">
import { t } from '../i18n'
import { computed, ref } from 'vue'
import Icon from '../components/Icon.vue'
import Menu from '../components/Menu.vue'
import Flag from '../components/Flag.vue'
import NodeEditor from './NodeEditor.vue'
import ChainEditor from './ChainEditor.vue'
import { api, rt, type Profile } from '../api'
import { currentGroup, latencyTest, nodesAdded, settings, state } from '../state'
import { bareName, countryMatches, countryTitle, nodeCountry } from '../country'
import { moveItem, useDragSort } from '../drag'
import {
  confirmDialog, formatBytes, groupName, httpWarning, latencyClass, latencyText, promptDialog, run, serverOf, toast, typeName,
  type MenuItem,
} from '../util'

const query = ref('')
const updating = ref(false)
const editorOpen = ref(false)
const editing = ref<Profile | null>(null)
const chainOpen = ref(false)
const editingChain = ref<Profile | null>(null)

const groupId = computed(() => state.app?.currentGroupId ?? 0)
const testing = computed(() => !!state.app?.testing)

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return state.profiles
  return state.profiles.filter(
    (p) =>
      p.name.toLowerCase().includes(q) ||
      p.type.includes(q) ||
      serverOf(p.outbound).toLowerCase().includes(q) ||
      // 香港 finds HK-01, 🇭🇰 01 and nodes detected in Hong Kong
      countryMatches(nodeCountry(p, settings.value).code, q) ||
      !!p.members?.some((m) => m.name.toLowerCase().includes(q)),
  )
})

/** A chain proxy that cannot be used: which hops are gone. */
const brokenHops = (p: Profile) => (p.members ?? []).flatMap((m, i) => (m.missing ? [i + 1] : []))

// ---- drag to reorder (not while searching: the list is then partial) ----
const listEl = ref<HTMLElement>()
const sorter = useDragSort({
  container: listEl,
  item: '.node',
  count: () => state.profiles.length,
  enabled: () => !query.value.trim() && state.profiles.length > 1,
  ignore: '.node-actions, button',
  onDrop: (from, to) => {
    moveItem(state.profiles, from, to)
    run(() => api.reorderProfiles(groupId.value, state.profiles.map((p) => p.id)))
  },
})

const usage = computed(() => {
  const u = currentGroup.value?.usage
  if (!u) return null
  const used = u.upload + u.download
  return {
    used,
    total: u.total,
    percent: u.total > 0 ? Math.min(100, (used / u.total) * 100) : 0,
    expire: u.expire > 0 ? new Date(u.expire * 1000).toLocaleDateString() : '',
  }
})

function selectGroup(id: number) {
  query.value = ''
  run(() => api.setCurrentGroup(id))
}

async function addGroup() {
  const r = await promptDialog(t('新建分组'), [{ label: t('分组名称'), placeholder: t('例如：自建节点') }])
  if (r) run(() => api.addGroup(r[0]), t('分组已创建'))
}

async function addSubscription() {
  const r = await promptDialog(t('添加订阅'), [
    { label: t('名称'), placeholder: t('节点名称') },
    { label: t('订阅地址'), placeholder: 'https://...', warn: httpWarning },
  ], t('支持 Base64 链接列表、Clash / Clash.Meta YAML 与 sing-box JSON 格式。'))
  if (!r) return
  updating.value = true
  const ok = await run(() => api.addSubscription(r[0], r[1]), (m) => m)
  updating.value = false
  if (ok) nodesAdded()
}

async function updateSubscription() {
  if (currentGroup.value?.manualOrder) {
    const go = await confirmDialog(
      t('节点顺序已调整过'),
      t('这个订阅的节点顺序被手动调整过。更新后会恢复为订阅里的顺序，确定更新吗？'),
      false,
      t('更新'),
    )
    if (!go) return
  }
  updating.value = true
  await run(() => api.updateSubscription(groupId.value), (m) => m)
  updating.value = false
}

async function editGroup() {
  const g = currentGroup.value
  if (!g) return
  const fields: { label: string; value: string; warn?: (v: string) => string }[] = [{ label: t('分组名称'), value: groupName(g) }]
  if (g.url) fields.push({ label: t('订阅地址'), value: g.url, warn: httpWarning }, { label: t('User-Agent（留空用默认）'), value: g.userAgent ?? '' })
  const r = await promptDialog(t('编辑分组'), fields)
  if (r) run(() => api.editGroup({ ...g, name: r[0], url: r[1] ?? '', userAgent: r[2] ?? '' }), t('已保存'))
}

async function deleteGroup() {
  const g = currentGroup.value
  if (!g) return
  let msg = t('确定删除“{0}”及其中的 {1} 个节点吗？', groupName(g), state.profiles.length)
  msg += await chainNote(state.profiles.map((p) => p.id))
  if (await confirmDialog(t('删除分组'), msg, true)) {
    run(() => api.deleteGroup(g.id), t('分组已删除'))
  }
}

/** A warning about the chain proxies that deleting these nodes would break. */
async function chainNote(ids: number[]) {
  const names = await api.chainsUsing(ids).catch(() => [] as string[])
  if (!names.length) return ''
  const list = names.map((n) => `“${bareName(n)}”`).join('、')
  return '\n\n' + (ids.length === 1
    ? t('⚠ 它是 {0} 个链式代理的成员：{1}。删除后这些链式代理会失效。', names.length, list)
    : t('⚠ 其中有节点是 {0} 个链式代理的成员：{1}。删除后这些链式代理会失效。', names.length, list))
}

async function importClipboard() {
  const text = await rt.clipboardGet()
  if (!text?.trim()) return toast(t('剪贴板是空的'), 'error')
  if (await run(() => api.importText(text, groupId.value), (m) => m)) nodesAdded()
}

function newProfile() {
  editing.value = null
  editorOpen.value = true
}

function newChain() {
  editingChain.value = null
  chainOpen.value = true
}

function editProfile(p: Profile) {
  if (p.type === 'chain') {
    editingChain.value = p
    chainOpen.value = true
    return
  }
  editing.value = p
  editorOpen.value = true
}

const addItems = computed<MenuItem[]>(() => [
  { label: t('新建节点'), hint: t('手动填写一个节点'), icon: 'edit', action: newProfile },
  { label: t('新建链式代理'), hint: t('让流量依次经过多个节点'), icon: 'route', action: newChain },
])

async function shareProfile(p: Profile) {
  const link = await run(() => api.exportLinks([p.id]))
  if (link) {
    await rt.clipboardSet(link)
    toast(t('分享链接已复制到剪贴板'), 'success')
  }
}

async function deleteProfile(p: Profile) {
  const msg = t('确定删除“{0}”吗？', p.name) + (await chainNote([p.id]))
  if (await confirmDialog(t('删除节点'), msg, true)) run(() => api.deleteProfiles([p.id]))
}

/** Explains why a chain proxy cannot be used; true when it can. */
function usable(p: Profile) {
  const gone = brokenHops(p)
  if (gone.length) toast(t('链式代理“{0}”的第 {1} 跳已不存在，请编辑后再使用', p.name, gone.join('、')), 'error')
  return !gone.length
}

function select(p: Profile) {
  if (sorter.wasDragged() || !usable(p)) return
  if (p.id !== state.app?.selectedId) run(() => api.selectProfile(p.id))
}

async function selectAndConnect(p: Profile) {
  if (sorter.wasDragged() || !usable(p)) return
  await run(() => api.selectProfile(p.id))
  if (!state.app?.running) run(() => api.connect(), t('连接成功'))
}

function test(mode: 'url' | 'tcp') {
  const ids = filtered.value.map((p) => p.id)
  if (!ids.length) return
  run(() => api.testLatency(ids, mode))
}

async function copyAll() {
  const nodes = filtered.value.filter((p) => p.type !== 'chain')
  const chains = filtered.value.length - nodes.length
  if (!nodes.length) return toast(t('链式代理没有分享链接'), 'error')
  const text = await run(() => api.exportLinks(nodes.map((p) => p.id)))
  if (text) {
    await rt.clipboardSet(text)
    let msg = t('已复制 {0} 个节点的链接', nodes.length)
    if (chains) msg += t('（跳过 {0} 个链式代理，它们没有分享链接）', chains)
    toast(msg, 'success')
  }
}

async function deleteUnavailable() {
  if (await confirmDialog(t('删除失效节点'), t('将删除本分组中最近一次测速失败的节点。'), true)) {
    run(() => api.deleteUnavailable(groupId.value), (n) => t('已删除 {0} 个节点', n))
  }
}

function openTestSettings() {
  state.settingsSection = 'test'
  state.page = 'settings'
}

const testItems = computed<MenuItem[]>(() => [
  { label: t('URL 测试'), hint: t('经节点访问测速地址，测真实延迟'), icon: 'zap', action: () => test('url') },
  { label: t('TCP 测试'), hint: t('只测连上服务器的时间，更快但不经过节点；基于 UDP 的协议和链式代理自动改用 URL 测试'), icon: 'gauge', action: () => test('tcp') },
  { label: t('测速设置…'), icon: 'settings', action: openTestSettings },
])

// ---- progress of the running test ----
const testPercent = computed(() => (latencyTest.total ? (latencyTest.done / latencyTest.total) * 100 : 0))
const testNow = computed(() => {
  const name = state.profiles.find((p) => p.id === latencyTest.current)?.name
  if (!name) return latencyTest.mode === 'url' && !latencyTest.done ? t('正在启动测速内核…') : ''
  const n = latencyTest.running.size
  return n > 1 ? t('正在测试：{0} 等 {1} 个节点', name, n) : t('正在测试：{0}', name)
})
const testConfig = computed(() => {
  const s = settings.value
  if (!s) return ''
  const sec = +(s.testTimeout / 1000).toFixed(1)
  if (latencyTest.mode === 'tcp') return t('直连节点服务器 · 超时 {0} 秒 · 并发 {1}', sec, s.testConcurrency)
  let host = s.testUrl
  try {
    host = new URL(s.testUrl).host
  } catch {}
  return t('测速地址 {0} · 超时 {1} 秒 · 并发 {2}', host, sec, s.testConcurrency)
})

const moreItems = computed<MenuItem[]>(() => [
  { label: t('按延迟排序'), icon: 'sort', action: () => run(() => api.sortGroup(groupId.value, 'latency')) },
  { label: t('按名称排序'), icon: 'sort', action: () => run(() => api.sortGroup(groupId.value, 'name')) },
  { label: t('按协议排序'), icon: 'sort', action: () => run(() => api.sortGroup(groupId.value, 'type')) },
  { label: t('复制全部分享链接'), icon: 'copy', action: copyAll },
  { label: t('编辑分组'), icon: 'edit', action: editGroup },
  { label: t('删除失效节点'), icon: 'trash', danger: true, action: deleteUnavailable },
  { label: t('删除分组'), icon: 'trash', danger: true, action: deleteGroup },
])

// ---- single node test (the bolt on each row) ----

// Protocols on top of UDP (QUIC) cannot be reached by a TCP handshake, and a
// TCP test of a chain proxy would only reach its first hop: these always use
// a URL test.
// (keep in sync with core.URLOnly, which does the same for batch TCP tests)
const urlOnlyTypes = new Set(['hysteria', 'hysteria2', 'tuic', 'wireguard', 'chain'])
const urlOnly = (p: Profile) => urlOnlyTypes.has(p.type)

function testOne(p: Profile, e: MouseEvent) {
  if (latencyTest.mode) return toast(t('正在测速，请稍候'), 'error')
  const mode = e.shiftKey || urlOnly(p) ? 'url' : 'tcp'
  run(() => api.testLatency([p.id], mode))
}

function boltTitle(p: Profile) {
  if (urlOnly(p)) return p.type === 'chain' ? t('URL 测试（链式代理要测整条链路）') : t('URL 测试（基于 UDP 的协议不能用 TCP 测速）')
  return t('TCP 测速：只测服务器端口能否连上。按住 Shift 点击改用 URL 测试，检验节点是否真的可用')
}
</script>

<template>
  <div class="page">
    <aside class="side">
      <div class="groups-wrap">
        <div class="side-title">{{ t('分组') }}</div>
        <div class="nav-list scroll groups">
          <div
            v-for="g in state.app?.groups"
            :key="g.id"
            class="nav-item"
            :class="{ active: g.id === groupId }"
            @click="selectGroup(g.id)"
          >
            <Icon :name="g.url ? 'rss' : 'folder'" :size="16" />
            <span class="ellipsis">{{ groupName(g) }}</span>
            <span v-if="g.id === groupId" class="count">{{ state.profiles.length }}</span>
          </div>
        </div>
      </div>

      <div v-if="usage" class="usage">
        <div class="row">
          <span class="faint">{{ t('已用流量') }}</span>
          <span class="spacer" />
          <b>{{ formatBytes(usage.used) }}</b>
          <span class="faint">/ {{ usage.total ? formatBytes(usage.total) : '∞' }}</span>
        </div>
        <div class="bar"><span :style="{ width: usage.percent + '%' }" /></div>
        <div v-if="usage.expire" class="faint" style="font-size: 12px">{{ t('到期：{0}', usage.expire) }}</div>
      </div>

      <div class="side-actions">
        <button class="btn" @click="addGroup"><Icon name="plus" /> {{ t('新建分组') }}</button>
        <button class="btn primary" :disabled="updating" @click="addSubscription"><Icon name="rss" /> {{ t('添加订阅') }}</button>
      </div>
    </aside>

    <main class="content">
      <div class="toolbar">
        <div class="title">
          <b>{{ groupName(currentGroup) }}</b>
          <span v-if="currentGroup?.updatedAt" class="faint">
            {{ t('更新于 {0}', new Date(currentGroup.updatedAt * 1000).toLocaleString()) }}
          </span>
        </div>
        <div class="search">
          <Icon name="search" :size="14" />
          <input v-model="query" class="input" :placeholder="t('搜索节点')" />
        </div>
        <button v-if="currentGroup?.url" class="btn" :disabled="updating" @click="updateSubscription">
          <Icon name="refresh" :class="{ spin: updating }" /> {{ t('更新') }}
        </button>
        <button class="btn" :title="t('从剪贴板导入')" @click="importClipboard"><Icon name="clipboard" /> {{ t('导入') }}</button>
        <Menu :items="addItems" icon="plus" />
        <button v-if="testing" class="btn danger" @click="run(() => api.stopTest())"><Icon name="close" /> {{ t('停止测速') }}</button>
        <Menu v-else :items="testItems" icon="zap" :label="t('测速')" />
        <Menu :items="moreItems" />
      </div>

      <div v-if="latencyTest.mode" class="test-bar">
        <span class="test-ping"><Icon :name="latencyTest.mode === 'tcp' ? 'gauge' : 'zap'" :size="14" /></span>
        <div class="test-info">
          <div class="test-line">
            <b>{{ latencyTest.mode === 'tcp' ? t('TCP 测试') : t('URL 测试') }}</b>
            <span class="mono test-count">{{ latencyTest.done }} / {{ latencyTest.total }}</span>
            <span class="ellipsis test-now">{{ testNow }}</span>
          </div>
          <div class="faint ellipsis test-config">{{ testConfig }}</div>
        </div>
        <button class="btn ghost small icon" :title="t('测速设置…')" @click="openTestSettings"><Icon name="settings" :size="14" /></button>
        <div class="test-progress"><span :style="{ width: testPercent + '%' }" /></div>
      </div>

      <div v-if="filtered.length" class="list-head-wrap">
        <div class="list-head">
          <span />
          <span>{{ t('名称') }}</span>
          <span class="center">{{ t('协议') }}</span>
          <span>{{ t('服务器') }}</span>
          <span class="center">{{ t('延迟') }}</span>
          <span />
        </div>
      </div>
      <div ref="listEl" class="list scroll" :class="{ 'sort-settling': sorter.drag.settling, sortable: !query.trim() }">
        <div
          v-for="(p, i) in filtered"
          :key="p.id"
          class="node"
          :class="[{
            selected: p.id === state.app?.selectedId,
            running: p.id === state.app?.runningId,
            probing: latencyTest.running.has(p.id),
            broken: brokenHops(p).length > 0,
          }, sorter.classes(i)]"
          :style="[{ '--i': Math.min(i, 12) }, sorter.style(i)]"
          @pointerdown="sorter.start($event, i)"
          @click="select(p)"
          @dblclick="selectAndConnect(p)"
        >
          <span class="radio" />
          <div class="node-name">
            <Flag :code="nodeCountry(p, settings).code" :title="countryTitle(nodeCountry(p, settings), settings)" />
            <span class="ellipsis" :title="p.name">{{ bareName(p.name) }}</span>
            <span v-if="p.upload + p.download > 0" class="faint traffic">{{ formatBytes(p.upload + p.download) }}</span>
          </div>
          <span class="node-type">
            <span v-if="brokenHops(p).length" class="badge bad"><Icon name="warn" :size="11" />{{ t('失效') }}</span>
            <span v-else-if="p.type === 'chain'" class="badge chain"><Icon name="route" :size="11" />{{ t('链式') }}</span>
            <span v-else class="badge">{{ typeName(p.type) }}</span>
          </span>
          <span v-if="brokenHops(p).length" class="ellipsis node-route bad">{{ t('第 {0} 跳的节点已不存在', brokenHops(p).join('、')) }}</span>
          <span v-else-if="p.type === 'chain'" class="ellipsis node-route" :title="p.members?.map((m) => m.name).join(' → ')">
            <template v-for="(m, k) in p.members" :key="k"><span v-if="k" class="sep">→</span>{{ bareName(m.name) }}</template>
          </span>
          <span v-else class="faint mono ellipsis node-server">{{ serverOf(p.outbound) }}</span>
          <span v-if="latencyTest.running.has(p.id)" class="lat" :title="t('测速中')"><span class="probe-bars"><i /><i /><i /><i /></span></span>
          <span v-else-if="latencyTest.waiting.has(p.id)" class="lat lat-none waiting">{{ t('等待中') }}</span>
          <span v-else class="lat" :class="latencyClass(p.latency)" :title="p.latencyError">
            <span v-if="p.latencyKind && p.latency" class="lat-kind" :title="p.latencyKind === 'tcp' ? t('TCP 测速：只说明服务器端口能连上') : t('URL 测试：通过节点访问测速地址')">{{ p.latencyKind.toUpperCase() }}</span>{{ latencyText(p.latency) }}
          </span>
          <div class="node-actions" @click.stop @dblclick.stop>
            <button class="btn ghost small icon bolt" :class="{ busy: latencyTest.running.has(p.id) }" :title="boltTitle(p)" @click="testOne(p, $event)"><Icon name="zap" :size="14" /></button>
            <button class="btn ghost small icon" :title="t('编辑')" @click="editProfile(p)"><Icon name="edit" :size="14" /></button>
            <button class="btn ghost small icon" :disabled="p.type === 'chain'" :title="p.type === 'chain' ? t('链式代理没有分享链接') : t('复制分享链接')" @click="shareProfile(p)"><Icon name="link" :size="14" /></button>
            <button class="btn ghost small icon danger" :title="t('删除')" @click="deleteProfile(p)"><Icon name="trash" :size="14" /></button>
          </div>
        </div>

        <div v-if="!filtered.length" class="empty">
          <Icon name="nodes" :size="40" :stroke="1.5" />
          <div v-if="query">{{ t('没有匹配“{0}”的节点', query) }}</div>
          <template v-else>
            <div>{{ t('这个分组还没有节点') }}</div>
            <div class="row">
              <button class="btn" @click="importClipboard"><Icon name="clipboard" /> {{ t('从剪贴板导入') }}</button>
              <button class="btn primary" @click="addSubscription"><Icon name="rss" /> {{ t('添加订阅') }}</button>
            </div>
          </template>
        </div>
      </div>
    </main>

    <NodeEditor v-model="editorOpen" :profile="editing" :group-id="groupId" />
    <ChainEditor v-model="chainOpen" :profile="editingChain" :group-id="groupId" />
  </div>
</template>

<style scoped>
.groups-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.groups {
  flex: 1;
  min-height: 0;
}
/* side by side when both labels fit, otherwise one button per row */
.side-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.side-actions .btn {
  flex: 1 1 auto;
  padding: 0 10px;
}
.usage {
  padding: 12px;
  border-radius: var(--radius);
  background: var(--card);
  border: 1px solid var(--card-border);
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
}
.bar {
  height: 6px;
  border-radius: 3px;
  background: var(--accent-soft-2);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  border-radius: 3px;
  background: var(--accent);
  transition: width 0.4s;
}

.content {
  padding-bottom: 12px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.title {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.title b {
  font-size: 16px;
}
.title span {
  font-size: 11px;
}
.toolbar .search {
  width: 200px;
}

/* The header sits in a box with exactly the same horizontal geometry as the
   list, including an always-present (empty) scrollbar, so both get the same
   width whatever the WebView's scrollbar width or DPI is, and the columns
   line up. */
.list,
.list-head-wrap {
  margin: 0 -6px;
  padding-left: 6px;
  padding-right: 6px;
  overflow-x: hidden;
  overflow-y: scroll;
}
.list-head-wrap {
  flex-shrink: 0;
  padding-top: 2px;
  margin-bottom: -6px;
}
.list {
  flex: 1;
  min-height: 0;
  padding-top: 2px;
  padding-bottom: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
/* name | protocol | server | latency | actions, shared by the header row */
.list-head,
.node {
  display: grid;
  grid-template-columns: 16px minmax(0, 2fr) 104px minmax(0, 1.3fr) 92px 122px;
  align-items: center;
  gap: 14px;
  padding: 0 10px 0 14px;
}
.list-head {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-3);
  /* same side borders as a row */
  border-left: 1px solid transparent;
  border-right: 1px solid transparent;
}
/* protocol pills and latencies vary in width, so those columns are centred,
   header included: they line up on the column's centre line */
.list-head .center,
.node-type,
.lat {
  text-align: center;
}
.node {
  flex-shrink: 0;
  height: 52px;
  background: var(--card);
  border: 1px solid var(--card-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  cursor: pointer;
  position: relative;
  transition: border-color 0.15s, background 0.15s, box-shadow 0.15s;
  animation: card-in 0.28s cubic-bezier(0.2, 0.8, 0.3, 1) both;
  animation-delay: calc(var(--i) * 18ms);
}
.node:hover {
  border-color: var(--accent-line);
  box-shadow: var(--shadow-hover);
}
.node.selected {
  background: var(--accent-soft);
  border-color: var(--accent-line);
}
.radio {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  border-radius: 50%;
  border: 2px solid var(--input-border);
  transition: border 0.15s;
}
.node.selected .radio {
  border: 5px solid var(--accent);
}
.node.running .radio {
  box-shadow: 0 0 0 4px var(--accent-soft-2);
}
.node-name {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}
.node-name .ellipsis {
  min-width: 0;
}
.node.selected .node-name {
  color: var(--accent);
}
.traffic {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 400;
}
.node-type .badge {
  min-width: 0;
  max-width: 100%;
}
.node-server {
  font-size: 12px;
}
.node-route {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-2);
}
.node-route .sep {
  margin: 0 5px;
  color: var(--accent);
  font-weight: 700;
}
.node-route.bad {
  color: var(--bad);
}
.badge.chain {
  gap: 3px;
  background: var(--accent);
  color: #fff;
}
.badge.bad {
  gap: 3px;
  background: color-mix(in srgb, var(--bad) 14%, var(--card));
  color: var(--bad);
}
.node.broken {
  border-color: color-mix(in srgb, var(--bad) 35%, var(--card));
}
.node.broken .node-name {
  color: var(--text-2);
}
/* rows can be dragged to reorder (drag.ts) */
.list.sortable .node {
  touch-action: none;
}
.node-actions {
  display: flex;
  justify-content: flex-end;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.15s;
}
.node:hover .node-actions {
  opacity: 1;
}
/* ---- latency test progress ---- */
.test-bar {
  position: relative;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 10px 11px 14px;
  border-radius: var(--radius);
  background: var(--card);
  border: 1px solid var(--accent-line);
  box-shadow: var(--shadow);
  overflow: hidden;
  animation: card-in 0.25s cubic-bezier(0.2, 0.8, 0.3, 1) both;
}
/* icon with a radar-like ring going out of it */
.test-ping {
  position: relative;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: var(--accent-soft-2);
  color: var(--accent);
}
.test-ping::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: 50%;
  border: 2px solid var(--accent);
  animation: ping 1.4s cubic-bezier(0, 0, 0.2, 1) infinite;
}
@keyframes ping {
  from {
    transform: scale(0.9);
    opacity: 0.7;
  }
  to {
    transform: scale(1.9);
    opacity: 0;
  }
}
.test-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.test-line {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}
.test-line b {
  flex-shrink: 0;
}
.test-count {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--accent);
  font-weight: 600;
}
.test-now {
  min-width: 0;
  color: var(--text-2);
}
.test-config {
  font-size: 11px;
}
.test-progress {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 3px;
  background: var(--accent-soft);
}
.test-progress span {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--accent) 60%, #ffffff), var(--accent));
  background-size: 200% 100%;
  animation: flow 1.2s linear infinite;
  transition: width 0.3s ease;
}
@keyframes flow {
  to {
    background-position: -200% 0;
  }
}

/* a light sweeping across the node being tested */
.node.probing {
  overflow: hidden;
  border-color: var(--accent-line);
}
.node.probing::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(100deg, transparent 20%, color-mix(in srgb, var(--accent) 14%, transparent) 50%, transparent 80%);
  animation: sweep 1.3s ease-in-out infinite;
}
@keyframes sweep {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(100%);
  }
}
/* signal bars rising one after another */
.probe-bars {
  display: inline-flex;
  align-items: flex-end;
  gap: 2px;
  height: 14px;
  vertical-align: middle;
}
.probe-bars i {
  width: 3px;
  height: 100%;
  border-radius: 2px;
  background: var(--accent);
  transform-origin: bottom;
  animation: bars 0.9s ease-in-out infinite;
}
.probe-bars i:nth-child(2) {
  animation-delay: 0.15s;
}
.probe-bars i:nth-child(3) {
  animation-delay: 0.3s;
}
.probe-bars i:nth-child(4) {
  animation-delay: 0.45s;
}
@keyframes bars {
  0%,
  100% {
    transform: scaleY(0.25);
    opacity: 0.5;
  }
  50% {
    transform: scaleY(1);
    opacity: 1;
  }
}
.lat.waiting {
  font-family: var(--font);
  font-size: 11px;
  animation: blink 1.6s ease-in-out infinite;
}
@keyframes blink {
  50% {
    opacity: 0.45;
  }
}

.spin {
  animation: spin 0.9s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
.lat {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
}
.lat-kind {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.3px;
  padding: 0 4px;
  border-radius: 3px;
  line-height: 14px;
  color: var(--text-3);
  background: var(--hover);
  box-shadow: inset 0 0 0 1px var(--line);
}
.bolt:hover {
  color: var(--mid);
}
.bolt.busy :deep(svg) {
  color: var(--mid);
  animation: zap 0.6s ease-in-out infinite;
}
@keyframes zap {
  50% {
    transform: scale(0.75);
    opacity: 0.5;
  }
}
</style>
