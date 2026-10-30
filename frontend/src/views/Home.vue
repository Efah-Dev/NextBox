<script setup lang="ts">
import { t } from '../i18n'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Card from '../components/Card.vue'
import Icon from '../components/Icon.vue'
import Switch from '../components/Switch.vue'
import Segmented from '../components/Segmented.vue'
import Sparkline from '../components/Sparkline.vue'
import { api, rt } from '../api'
import { currentGroup, patchSettings, selectedProfile, settings, state } from '../state'
import { formatBytes, formatDuration, formatSpeed, groupName, latencyClass, latencyText, run, serverOf, toast, typeName } from '../util'

const busy = ref(false)
const now = ref(Date.now())
const session = ref({ up: 0, down: 0, count: 0 })

const running = computed(() => !!state.app?.running)
const uptime = computed(() => (running.value ? formatDuration(now.value - (state.app?.startedAt ?? now.value)) : '--:--'))

const routeMode = computed({
  get: () => settings.value?.routeMode ?? 'rule',
  set: (v) => run(() => patchSettings({ routeMode: v as any })),
})
const tun = computed({
  get: () => !!settings.value?.tunEnabled,
  set: (v) => {
    if (v && !state.app?.elevated) {
      toast(t('TUN 模式需要管理员权限，请先点击“提权”以管理员身份重启'), 'error')
      return
    }
    run(() => patchSettings({ tunEnabled: v }))
  },
})

async function toggle() {
  if (busy.value) return
  busy.value = true
  try {
    if (running.value) await api.disconnect()
    else {
      await api.connect()
      toast(t('连接成功'), 'success')
    }
  } catch (e) {
    toast((e as Error).message, 'error')
  } finally {
    busy.value = false
  }
}

async function importClipboard() {
  const text = await rt.clipboardGet()
  if (!text?.trim()) return toast(t('剪贴板是空的'), 'error')
  run(() => api.importText(text, state.app!.currentGroupId), (m) => m)
}

function testCurrentGroup() {
  const ids = state.profiles.map((p) => p.id)
  run(() => api.testLatency(ids, 'url'), t('开始测速：') + groupName(currentGroup.value))
  state.page = 'nodes'
}

// The log card shows exactly as many whole lines as fit its current height:
// a ResizeObserver recomputes the count whenever the window is resized.
const LOG_LINE_PX = 20 // keep in sync with .mini-log line-height
const logBox = ref<HTMLElement>()
const fitLines = ref(6)
const recentLogs = computed(() => state.logs.slice(-fitLines.value))
const resizeObserver = new ResizeObserver(() => {
  if (logBox.value) fitLines.value = Math.max(1, Math.floor(logBox.value.clientHeight / LOG_LINE_PX))
})

const quick = [
  { key: 'allowLan', label: t('允许局域网连接'), icon: 'nodes' },
  { key: 'blockAds', label: t('屏蔽广告'), icon: 'shield' },
  { key: 'autoStart', label: t('开机自启'), icon: 'power' },
  { key: 'logToFile', label: t('记录日志到文件'), icon: 'logs' },
] as const
const quickValue = (key: (typeof quick)[number]['key']) => !!settings.value?.[key]
const setQuick = (key: (typeof quick)[number]['key'], v: boolean) => run(() => patchSettings({ [key]: v }))
// '连 接' only differs from '连接' (the Connections page) to get its own
// translation; the space is not shown.
const connectLabel = t('连 接').replace(/\s+/g, '')
const modeText = computed(() => {
  if (settings.value?.tunEnabled) return t('TUN 模式')
  return settings.value?.systemProxy ? t('系统代理') : t('仅本地端口')
})

let timer = 0
async function poll() {
  now.value = Date.now()
  if (!running.value) return
  if (Math.floor(now.value / 1000) % 2 !== 0) return
  const c = await api.getConnections().catch(() => null)
  if (c) session.value = { up: c.uploadTotal, down: c.downloadTotal, count: c.connections.length }
}
onMounted(() => {
  timer = window.setInterval(poll, 1000)
  if (logBox.value) resizeObserver.observe(logBox.value)
})
onUnmounted(() => {
  clearInterval(timer)
  resizeObserver.disconnect()
})
</script>

<template>
  <div class="page">
    <aside class="side">
      <div>
        <div class="side-title">{{ t('当前节点') }}</div>
        <div class="hero" @click="state.page = 'nodes'">
          <template v-if="selectedProfile">
            <div class="row">
              <span class="badge">{{ typeName(selectedProfile.type) }}</span>
              <span class="lat" :class="latencyClass(selectedProfile.latency)">{{ latencyText(selectedProfile.latency) }}</span>
            </div>
            <div class="hero-name">{{ selectedProfile.name }}</div>
            <div class="hero-server faint mono ellipsis">{{ serverOf(selectedProfile.outbound) }}</div>
          </template>
          <template v-else>
            <div class="hero-name muted">{{ t('尚未选择节点') }}</div>
            <div class="faint">{{ t('点击这里去选择一个节点') }}</div>
          </template>
          <Icon class="hero-go" name="chevron" :size="16" />
        </div>
      </div>

      <div>
        <div class="side-title">{{ t('路由模式') }}</div>
        <Segmented
          v-model="routeMode"
          :options="[
            { value: 'rule', label: t('规则分流') },
            { value: 'global', label: t('全局代理') },
            { value: 'direct', label: t('全部直连') },
          ]"
        />
      </div>

      <div>
        <div class="side-title">{{ t('快捷设置') }}</div>
        <div class="toggles">
          <div class="toggle" @click="tun = !tun">
            <Icon name="tun" :size="16" />
            <span class="toggle-label">{{ t('TUN 模式') }}</span>
            <button v-if="!state.app?.elevated" class="admin-link" :title="t('TUN 模式需要管理员权限')" @click.stop="run(() => api.restartAsAdmin())">
              <Icon name="admin" :size="13" /> {{ t('提权') }}
            </button>
            <Switch v-model="tun" @click.stop />
          </div>
          <div v-for="q in quick" :key="q.key" class="toggle" @click="setQuick(q.key, !quickValue(q.key))">
            <Icon :name="q.icon" :size="16" />
            <span class="toggle-label">{{ q.label }}</span>
            <Switch :model-value="quickValue(q.key)" @update:model-value="setQuick(q.key, $event)" @click.stop />
          </div>
        </div>
      </div>

      <div class="spacer" />

      <button class="launch" :class="{ on: running }" :disabled="busy" @click="toggle">
        <span class="launch-main">{{ busy ? t('请稍候') : running ? t('断开连接') : connectLabel }}</span>
        <span class="launch-sub ellipsis">
          {{ running ? t('{0} · 已运行 {1}', modeText, uptime) : selectedProfile?.name ?? (routeMode === 'direct' ? t('直连模式') : t('请先选择节点')) }}
        </span>
      </button>
    </aside>

    <main class="content home-content">
      <Card v-if="state.app && !state.app.coreFound" :title="t('未找到 sing-box 内核')" :index="0">
        <p class="muted" style="margin: 0 0 12px; line-height: 1.7">
          {{ t('请把 sing-box 1.12 或更新版本的可执行文件放到：') }}<br />
          <span class="mono">{{ state.app.corePath }}</span>
        </p>
        <button class="btn primary" @click="run(() => api.openCoreDir())"><Icon name="folder" /> {{ t('打开内核目录') }}</button>
      </Card>

      <div class="grid-2">
        <Card :title="t('实时速度')" :index="1">
          <div class="speeds">
            <div class="speed">
              <span class="speed-label"><Icon name="down" :size="14" /> {{ t('下载') }}</span>
              <span class="speed-value">{{ formatSpeed(state.traffic.down) }}</span>
            </div>
            <div class="speed up">
              <span class="speed-label"><Icon name="up" :size="14" /> {{ t('上传') }}</span>
              <span class="speed-value">{{ formatSpeed(state.traffic.up) }}</span>
            </div>
          </div>
          <Sparkline :up="state.traffic.history.map((h) => h.up)" :down="state.traffic.history.map((h) => h.down)" />
        </Card>

        <Card :title="t('本次会话')" :index="2">
          <div class="stats">
            <div class="stat"><span class="faint">{{ t('状态') }}</span><b :class="running ? 'ok' : ''">{{ running ? t('已连接') : t('未连接') }}</b></div>
            <div class="stat"><span class="faint">{{ t('运行时长') }}</span><b class="mono">{{ uptime }}</b></div>
            <div class="stat"><span class="faint">{{ t('总下载') }}</span><b>{{ formatBytes(session.down) }}</b></div>
            <div class="stat"><span class="faint">{{ t('总上传') }}</span><b>{{ formatBytes(session.up) }}</b></div>
            <div class="stat"><span class="faint">{{ t('活动连接') }}</span><b>{{ running ? session.count : 0 }}</b></div>
            <div class="stat">
              <span class="faint">{{ t('代理端口') }}</span>
              <b class="mono">{{ settings?.listenAddress }}:{{ settings?.mixedPort }}</b>
            </div>
          </div>
        </Card>
      </div>

      <Card :title="t('快捷操作')" :index="3">
        <div class="actions">
          <button class="action" @click="importClipboard">
            <Icon name="clipboard" :size="20" /><span>{{ t('从剪贴板导入') }}</span>
          </button>
          <button class="action" @click="run(() => api.updateAllSubscriptions(), (m) => m)">
            <Icon name="refresh" :size="20" /><span>{{ t('更新全部订阅') }}</span>
          </button>
          <button class="action" :disabled="!state.profiles.length" @click="testCurrentGroup">
            <Icon name="zap" :size="20" /><span>{{ t('测试当前分组') }}</span>
          </button>
          <button class="action" @click="run(() => api.openDataDir())">
            <Icon name="folder" :size="20" /><span>{{ t('打开数据目录') }}</span>
          </button>
        </div>
      </Card>

      <Card :title="t('最近日志')" :index="4" class="grow">
        <template #actions>
          <button class="btn ghost small" @click="state.page = 'logs'">{{ t('查看全部') }} <Icon name="chevron" :size="12" /></button>
        </template>
        <div ref="logBox" class="mini-log mono">
          <div v-for="(l, i) in recentLogs" :key="i" class="ellipsis">{{ l }}</div>
          <div v-if="!recentLogs.length" class="faint">{{ t('暂无日志') }}</div>
        </div>
      </Card>
    </main>
  </div>
</template>

<style scoped>
.hero {
  position: relative;
  padding: 14px 34px 14px 14px;
  border-radius: var(--radius);
  background: var(--card);
  border: 1px solid var(--card-border);
  box-shadow: var(--shadow);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.hero:hover {
  border-color: var(--accent-line);
  box-shadow: var(--shadow-hover);
}
.hero-name {
  font-size: 15px;
  font-weight: 700;
  line-height: 1.35;
  word-break: break-all;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.hero-server {
  font-size: 11px;
}
.hero-go {
  position: absolute;
  right: 12px;
  top: 50%;
  margin-top: -8px;
  color: var(--text-3);
}
.toggles {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.toggle {
  height: 34px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 6px 0 10px;
  border-radius: var(--radius-sm);
  color: var(--text-2);
  cursor: pointer;
}
.toggle:hover {
  background: var(--hover);
}
.toggle-label {
  flex: 1;
  min-width: 0;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.admin-link {
  flex-shrink: 0;
  white-space: nowrap;
  height: 22px;
  padding: 0 6px;
  border-radius: 11px;
  border: none;
  background: var(--accent-soft);
  color: var(--accent);
  font: inherit;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 3px;
  cursor: pointer;
}
.admin-link:hover {
  text-decoration: underline;
}

.speeds {
  display: flex;
  gap: 24px;
  margin-bottom: 6px;
}
.speed {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.speed-label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--accent);
}
.speed.up .speed-label {
  color: #22b07d;
}
.speed-value {
  font-size: 22px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 16px;
}
.stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.stat span {
  font-size: 12px;
}
.stat b {
  font-size: 15px;
  font-variant-numeric: tabular-nums;
}
.stat b.ok {
  color: var(--good);
}

.actions {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}
.action {
  height: 76px;
  border-radius: var(--radius);
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text-2);
  font: inherit;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.18s;
}
.action:hover {
  border-color: var(--accent-line);
  background: var(--accent-soft);
  color: var(--accent);
  transform: translateY(-2px);
}
.action:disabled {
  opacity: 0.5;
  pointer-events: none;
}
.action span {
  color: var(--text);
  font-size: 12px;
}
.action:hover span {
  color: var(--accent);
}

/* the log card takes the remaining height; newest lines stay at the bottom */
/* Home never scrolls: every card fits the window and the log card takes the
   rest, its bottom edge lining up with the launch button on the left. */
.home-content {
  overflow: hidden;
  padding-bottom: 16px;
}
.grow {
  flex: 1 1 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.grow :deep(.card-body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.mini-log {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  font-size: 11.5px;
  line-height: 20px;
  color: var(--text-2);
}
.mini-log > div {
  height: 20px;
}
</style>
