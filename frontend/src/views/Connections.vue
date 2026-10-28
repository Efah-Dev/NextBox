<script setup lang="ts">
import { t } from '../i18n'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Icon from '../components/Icon.vue'
import { api, type Connection } from '../api'
import { state } from '../state'
import { formatBytes, formatDuration, formatSpeed, run } from '../util'

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
      [c.metadata.host, c.metadata.destinationIP, c.metadata.processPath, c.rule, c.chains.join(' ')]
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
const chain = (c: Connection) => [...(c.chains ?? [])].reverse().join(' → ')
const ruleText = (r: string) => r.replace(/ => .*$/, '')

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
          <span>{{ t('规则 / 出站') }}</span>
          <span class="num">{{ t('速度') }}</span>
          <span class="num">{{ t('流量') }}</span>
          <span class="num">{{ t('时长') }}</span>
          <span />
        </div>
        <div class="tbody scroll">
          <div v-for="c in filtered" :key="c.id" class="tr">
            <span class="cell-target">
              <span class="net" :class="c.metadata.network">{{ c.metadata.network.toUpperCase() }}</span>
              <span class="ellipsis mono" :title="target(c)">{{ target(c) }}</span>
            </span>
            <span class="ellipsis" :title="c.metadata.processPath">{{ processName(c.metadata.processPath) || '—' }}</span>
            <span class="ellipsis" :title="c.rule">
              <span class="faint">{{ ruleText(c.rule) }}</span> · <b>{{ chain(c) }}</b>
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
  grid-template-columns: minmax(180px, 2.2fr) minmax(80px, 1fr) minmax(130px, 1.6fr) 92px 84px 64px 30px;
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
.tr:hover {
  background: var(--hover);
}
.num {
  text-align: right;
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
.tr button {
  opacity: 0;
}
.tr:hover button {
  opacity: 1;
}
</style>
