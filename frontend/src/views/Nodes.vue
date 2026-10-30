<script setup lang="ts">
import { t } from '../i18n'
import { computed, ref } from 'vue'
import Icon from '../components/Icon.vue'
import Menu from '../components/Menu.vue'
import NodeEditor from './NodeEditor.vue'
import { api, rt, type Profile } from '../api'
import { currentGroup, state } from '../state'
import {
  confirmDialog, formatBytes, groupName, httpWarning, latencyClass, latencyText, promptDialog, run, serverOf, toast, typeName,
  type MenuItem,
} from '../util'

const query = ref('')
const updating = ref(false)
const editorOpen = ref(false)
const editing = ref<Profile | null>(null)

const groupId = computed(() => state.app?.currentGroupId ?? 0)
const testing = computed(() => !!state.app?.testing)

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return state.profiles
  return state.profiles.filter(
    (p) => p.name.toLowerCase().includes(q) || p.type.includes(q) || serverOf(p.outbound).toLowerCase().includes(q),
  )
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
  await run(() => api.addSubscription(r[0], r[1]), (m) => m)
  updating.value = false
}

async function updateSubscription() {
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
  if (await confirmDialog(t('删除分组'), t('确定删除“{0}”及其中的 {1} 个节点吗？', groupName(g), state.profiles.length), true)) {
    run(() => api.deleteGroup(g.id), t('分组已删除'))
  }
}

async function importClipboard() {
  const text = await rt.clipboardGet()
  if (!text?.trim()) return toast(t('剪贴板是空的'), 'error')
  run(() => api.importText(text, groupId.value), (m) => m)
}

function newProfile() {
  editing.value = null
  editorOpen.value = true
}

function editProfile(p: Profile) {
  editing.value = p
  editorOpen.value = true
}

async function shareProfile(p: Profile) {
  const link = await run(() => api.exportLinks([p.id]))
  if (link) {
    await rt.clipboardSet(link)
    toast(t('分享链接已复制到剪贴板'), 'success')
  }
}

async function deleteProfile(p: Profile) {
  if (await confirmDialog(t('删除节点'), t('确定删除“{0}”吗？', p.name), true)) run(() => api.deleteProfiles([p.id]))
}

function select(p: Profile) {
  if (p.id !== state.app?.selectedId) run(() => api.selectProfile(p.id))
}

async function selectAndConnect(p: Profile) {
  await run(() => api.selectProfile(p.id))
  if (!state.app?.running) run(() => api.connect(), t('连接成功'))
}

function test(mode: 'url' | 'tcp') {
  const ids = filtered.value.map((p) => p.id)
  if (!ids.length) return
  run(() => api.testLatency(ids, mode))
}

async function copyAll() {
  const text = await run(() => api.exportLinks(filtered.value.map((p) => p.id)))
  if (text) {
    await rt.clipboardSet(text)
    toast(t('已复制 {0} 个节点的链接', filtered.value.length), 'success')
  }
}

async function deleteUnavailable() {
  if (await confirmDialog(t('删除失效节点'), t('将删除本分组中最近一次测速失败的节点。'), true)) {
    run(() => api.deleteUnavailable(groupId.value), (n) => t('已删除 {0} 个节点', n))
  }
}

const testItems = computed<MenuItem[]>(() => [
  { label: t('URL 测试'), icon: 'zap', action: () => test('url') },
  { label: t('TCP 测试'), icon: 'gauge', action: () => test('tcp') },
])

const moreItems = computed<MenuItem[]>(() => [
  { label: t('按延迟排序'), icon: 'sort', action: () => run(() => api.sortGroup(groupId.value, 'latency')) },
  { label: t('按名称排序'), icon: 'sort', action: () => run(() => api.sortGroup(groupId.value, 'name')) },
  { label: t('按协议排序'), icon: 'sort', action: () => run(() => api.sortGroup(groupId.value, 'type')) },
  { label: t('复制全部分享链接'), icon: 'copy', action: copyAll },
  { label: t('编辑分组'), icon: 'edit', action: editGroup },
  { label: t('删除失效节点'), icon: 'trash', danger: true, action: deleteUnavailable },
  { label: t('删除分组'), icon: 'trash', danger: true, action: deleteGroup },
])
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
        <button class="btn icon" :title="t('新建节点')" @click="newProfile"><Icon name="plus" /></button>
        <button v-if="testing" class="btn danger" @click="run(() => api.stopTest())"><Icon name="close" /> {{ t('停止测速') }}</button>
        <Menu v-else :items="testItems" icon="zap" :label="t('测速')" />
        <Menu :items="moreItems" />
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
      <div class="list scroll">
        <div
          v-for="(p, i) in filtered"
          :key="p.id"
          class="node"
          :class="{ selected: p.id === state.app?.selectedId, running: p.id === state.app?.runningId }"
          :style="{ '--i': Math.min(i, 12) }"
          @click="select(p)"
          @dblclick="selectAndConnect(p)"
        >
          <span class="radio" />
          <div class="node-name">
            <span class="ellipsis" :title="p.name">{{ p.name }}</span>
            <span v-if="p.upload + p.download > 0" class="faint traffic">{{ formatBytes(p.upload + p.download) }}</span>
          </div>
          <span class="node-type"><span class="badge">{{ typeName(p.type) }}</span></span>
          <span class="faint mono ellipsis node-server">{{ serverOf(p.outbound) }}</span>
          <span class="lat" :class="latencyClass(p.latency)" :title="p.latencyError">{{ latencyText(p.latency) }}</span>
          <div class="node-actions" @click.stop @dblclick.stop>
            <button class="btn ghost small icon" :title="t('编辑')" @click="editProfile(p)"><Icon name="edit" :size="14" /></button>
            <button class="btn ghost small icon" :title="t('复制分享链接')" @click="shareProfile(p)"><Icon name="link" :size="14" /></button>
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
  grid-template-columns: 16px minmax(0, 2fr) 104px minmax(0, 1.3fr) 70px 92px;
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
  align-items: baseline;
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
.spin {
  animation: spin 0.9s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
