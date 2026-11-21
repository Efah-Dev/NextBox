<script setup lang="ts">
import { t } from '../i18n'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import ContextMenu, { type CtxItem } from '../components/ContextMenu.vue'
import { addNewSet, addRule, addToSet, connMatches, type ConnMatch } from '../connRules'
import Icon from '../components/Icon.vue'
import { api, rt, type Connection } from '../api'
import { state } from '../state'
import { formatBytes, formatDuration, formatSpeed, run, toast } from '../util'
import Flag from '../components/Flag.vue'
import { bareName, guessCountry } from '../country'
import { policies, refreshPolicies, ruleTitle, sceneInfo, targetLabel, targetOptions } from '../policies'

interface Row extends Connection {
  upSpeed: number
  downSpeed: number
}

const rows = ref<Row[]>([])
const totals = ref({ up: 0, down: 0 })
const query = ref('')
const paused = ref(false)
const sortBy = ref<'start' | 'download' | 'upload' | 'speed'>('start')
const now = ref(Date.now())
let prev = new Map<string, Connection>()

async function refresh() {
  now.value = Date.now()
  if (paused.value) return
  const snap = await api.getConnections().catch(() => null)
  if (!snap) return
  totals.value = { up: snap.uploadTotal, down: snap.downloadTotal }
  const next = new Map<string, Connection>()
  rows.value = snap.connections.map((c) => {
    next.set(c.id, c)
    const p = prev.get(c.id)
    return { ...c, upSpeed: p ? c.upload - p.upload : 0, downSpeed: p ? c.download - p.download : 0 }
  })
  prev = next
}

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  let list = rows.value
  if (q) {
    list = list.filter((c) =>
      [c.metadata.host, c.metadata.destinationIP, c.metadata.processPath, c.rule, matchLabel(c).text, c.chains.join(' ')]
        .some((s) => s?.toLowerCase().includes(q)),
    )
  }
  const key = sortBy.value
  return [...list].sort((a, b) => {
    if (key === 'download') return b.download - a.download
    if (key === 'upload') return b.upload - a.upload
    if (key === 'speed') return b.downSpeed + b.upSpeed - (a.downSpeed + a.upSpeed)
    return Date.parse(b.start) - Date.parse(a.start)
  })
})

const target = (c: Connection) =>
  (c.metadata.host || c.metadata.destinationIP) + ':' + c.metadata.destinationPort
const processName = (p: string) => p.split(/[\\/]/).pop()?.replace(/ \(.*\)$/, '') ?? ''
const ruleText = (r: string) => r.replace(/ => .*$/, '')

/** The routing step the connection took, the way the rules page names it. */
function matchLabel(c: Connection): { text: string; tag?: string } {
  const m = c.match
  switch (m?.source) {
    case 'rule':
      return { text: ruleTitle({ kind: m.kind ?? '', values: m.values, name: m.name, app: m.app, count: m.count }), tag: m.kind === 'set' ? t('规则集') : undefined }
    case 'scene':
      return { text: sceneInfo(m.scene ?? '').label, tag: t('场景') }
    case 'private':
      return { text: t('局域网和本机地址') }
    case 'excluded':
      return { text: t('TUN 排除网段') }
    case 'dns':
      return { text: t('DNS 查询') }
    case 'final':
      return { text: t('其余流量') }
  }
  return { text: ruleText(c.rule) || '—' }
}

/** Where the connection leaves: the node (through a policy group: "group → node"), or direct. */
function exitOf(c: Connection): { text: string; code: string; direct: boolean } {
  const hops = [...(c.chains ?? [])].reverse().filter((h) => !h.startsWith('nb-step:') && h !== 'proxy')
  const last = hops[hops.length - 1] ?? ''
  if (last === 'direct' || !hops.length) return { text: t('直连'), code: '', direct: true }
  // node names often start with a flag emoji: the flag in front shows it
  return { text: hops.map(bareName).join(' → '), code: guessCountry(last), direct: false }
}

// ---- right click: make a rule from a connection
const menu = ref<{ x: number; y: number; c: Connection; matches: ConnMatch[] } | null>(null)
const pickedMatch = ref(0)

async function openMenu(e: MouseEvent, c: Connection) {
  const at = { x: e.clientX, y: e.clientY }
  if (!policies.loaded) refreshPolicies()
  const matches = await connMatches(c)
  pickedMatch.value = 0
  menu.value = { ...at, c, matches }
}

const menuItems = computed<CtxItem[]>(() => {
  const m = menu.value
  if (!m) return []
  const c = m.c
  const name = c.metadata.host || c.metadata.destinationIP
  const match = m.matches[pickedMatch.value]
  const rules = state.app?.settings.rules ?? []
  const sets = rules.map((r, i) => ({ r, i })).filter((x) => x.r.kind === 'set')
  const items: CtxItem[] = [{ title: true, label: name }]
  if (match) {
    items.push({ title: true, label: t('按什么匹配') })
    m.matches.forEach((x, i) => items.push({ label: x.label, checked: i === pickedMatch.value, keep: true, action: () => (pickedMatch.value = i) }))
    items.push({ sep: true })
    items.push({
      label: t('加入规则集'),
      icon: 'layers',
      children: [
        ...(sets.length
          ? sets.map((x) => ({ label: x.r.name || t('未命名规则集'), hint: '→ ' + targetLabel(x.r.target), action: () => addToSet(x.i, match) }))
          : [{ label: t('还没有规则集'), disabled: true }]),
        { sep: true },
        { label: t('新建规则集…'), icon: 'plus', action: () => addNewSet(match) },
      ],
    })
    items.push({
      label: t('新建规则'),
      icon: 'plus',
      children: [{ title: true, label: t('去向') }, ...targetOptions().map((o) => ({ label: o.label, action: () => addRule(match, o.value) }))],
    })
    items.push({ sep: true })
  }
  if (c.metadata.host) {
    items.push({
      label: t('在「试一试」里查看路径'),
      icon: 'route',
      action: () => {
        state.routeProbe = c.metadata.host
        state.settingsSection = 'route'
        state.page = 'settings'
      },
    })
  }
  items.push({
    label: c.metadata.host ? t('复制域名') : t('复制 IP'),
    icon: 'copy',
    action: async () => toast((await rt.clipboardSet(name)) ? t('已复制') : t('复制失败'), 'success'),
  })
  items.push({ label: t('断开这条连接'), icon: 'close', danger: true, action: () => run(() => api.closeConnection(c.id)) })
  return items
})

let timer = 0
onMounted(() => {
  refresh()
  timer = window.setInterval(refresh, 1000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="page">
    <main class="content">
      <div class="toolbar">
        <div class="summary">
          <b>{{ rows.length }}</b><span class="faint">{{ t('个连接') }}</span>
          <span class="sep" />
          <Icon name="down" :size="13" class="c-down" /><span>{{ formatBytes(totals.down) }}</span>
          <Icon name="up" :size="13" class="c-up" /><span>{{ formatBytes(totals.up) }}</span>
        </div>
        <div class="search">
          <Icon name="search" :size="14" />
          <input v-model="query" class="input" :placeholder="t('过滤主机 / 进程 / 规则')" />
        </div>
        <select v-model="sortBy" class="select sort">
          <option value="start">{{ t('最新') }}</option>
          <option value="speed">{{ t('速度') }}</option>
          <option value="download">{{ t('下载量') }}</option>
          <option value="upload">{{ t('上传量') }}</option>
        </select>
        <button class="btn" @click="paused = !paused">{{ paused ? t('继续') : t('暂停') }}</button>
        <button class="btn danger" :disabled="!rows.length" @click="run(() => api.closeConnection(''), t('已关闭全部连接'))">
          <Icon name="close" /> {{ t('全部关闭') }}
        </button>
      </div>

      <div class="card table-card">
        <div class="thead">
          <span>{{ t('目标') }}</span>
          <span>{{ t('进程') }}</span>
          <span>{{ t('命中规则 / 出口') }}</span>
          <span class="num">{{ t('速度') }}</span>
          <span class="num">{{ t('流量') }}</span>
          <span class="num">{{ t('时长') }}</span>
          <span />
        </div>
        <div class="tbody scroll">
          <div v-for="c in filtered" :key="c.id" class="tr" :class="{ picked: menu?.c.id === c.id }" @contextmenu.prevent="openMenu($event, c)">
            <span class="cell-target">
              <span class="net" :class="c.metadata.network">{{ c.metadata.network.toUpperCase() }}</span>
              <span class="ellipsis mono" :title="target(c)">{{ target(c) }}</span>
            </span>
            <span class="ellipsis" :title="c.metadata.processPath">{{ processName(c.metadata.processPath) || '—' }}</span>
            <span class="cell-route">
              <span class="cell-rule" :title="ruleText(c.rule)">
                <span v-if="matchLabel(c).tag" class="tag">{{ matchLabel(c).tag }}</span>
                <span class="ellipsis">{{ matchLabel(c).text }}</span>
              </span>
              <span class="cell-exit" :class="{ direct: exitOf(c).direct }" :title="exitOf(c).text">
                <Flag v-if="!exitOf(c).direct" :code="exitOf(c).code" :size="12" inline />
                <b class="ellipsis">{{ exitOf(c).text }}</b>
              </span>
            </span>
            <span class="num mono small">
              <span class="c-down">{{ formatSpeed(c.downSpeed) }}</span><br />
              <span class="c-up">{{ formatSpeed(c.upSpeed) }}</span>
            </span>
            <span class="num mono small">{{ formatBytes(c.download) }}<br /><span class="faint">{{ formatBytes(c.upload) }}</span></span>
            <span class="num mono small">{{ formatDuration(now - Date.parse(c.start)) }}</span>
            <button class="btn ghost small icon danger" :title="t('关闭连接')" @click="run(() => api.closeConnection(c.id))">
              <Icon name="close" :size="13" />
            </button>
          </div>
          <div v-if="!filtered.length" class="empty">
            <Icon name="connections" :size="40" :stroke="1.5" />
            <div>{{ state.app?.running ? t('暂无连接') : t('连接后这里会显示实时连接') }}</div>
          </div>
        </div>
      </div>
    </main>
    <ContextMenu v-if="menu" :items="menuItems" :x="menu.x" :y="menu.y" @close="menu = null" />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.summary {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 6px;
  font-variant-numeric: tabular-nums;
}
.summary b {
  font-size: 18px;
}
.sep {
  width: 1px;
  height: 14px;
  background: var(--line);
  margin: 0 6px;
}
.c-down {
  color: var(--accent);
}
.c-up {
  color: #22b07d;
}
.toolbar .search {
  width: 220px;
}
.sort {
  width: 92px;
}

.table-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.thead,
.tr {
  display: grid;
  grid-template-columns: minmax(160px, 2fr) minmax(80px, 1fr) minmax(170px, 2fr) 84px 84px 60px 30px;
  gap: 12px;
  align-items: center;
  padding: 0 14px;
}
.thead {
  height: 38px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-3);
  border-bottom: 1px solid var(--line);
  flex-shrink: 0;
}
.tbody {
  flex: 1;
  min-height: 0;
}
.tr {
  min-height: 48px;
  border-bottom: 1px solid var(--line);
  transition: background 0.12s;
}
.tr:hover,
.tr.picked {
  background: var(--hover);
}
.tr.picked {
  box-shadow: inset 2px 0 0 var(--accent);
}
.num {
  text-align: center;
}
.small {
  font-size: 11.5px;
  line-height: 1.5;
}
.cell-target {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.net {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 4px;
  background: var(--accent-soft);
  color: var(--accent);
  flex-shrink: 0;
}
.net.udp {
  background: color-mix(in srgb, #8b5cf6 14%, var(--card));
  color: #8b5cf6;
}
.cell-route {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  font-size: 12.5px;
}
.cell-exit {
  font-size: 12px;
}
.cell-rule,
.cell-exit {
  display: flex;
  align-items: baseline; /* the flag on the baseline of the name */
  gap: 6px;
  min-width: 0;
}
.cell-rule .tag {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 4px;
  background: var(--hover);
  color: var(--text-2);
}
.cell-exit b {
  font-weight: 600;
}
.cell-exit.direct b {
  color: var(--good);
}
.tr button {
  opacity: 0;
}
.tr:hover button {
  opacity: 1;
}
</style>
