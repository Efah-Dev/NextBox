<script setup lang="ts">
// 开机自启 with a check of what Windows will really do: an animated green
// tick when NextBox is registered and enabled, a red cross with the reason
// and a fix otherwise; below, what happened the last times Windows started
// it (or did not).
import { t } from '../../i18n'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Switch from '../../components/Switch.vue'
import { api, type AutoStartRecord, type AutoStartStatus } from '../../api'
import { state } from '../../state'
import { run } from '../../util'

const enabled = defineModel<boolean>({ required: true })
const status = ref<AutoStartStatus>()
const fixing = ref(false)
let seq = 0

async function check() {
  const n = ++seq
  const s = await api.getAutoStartStatus().catch(() => undefined)
  if (n === seq) status.value = s
}

// the setting is saved a moment after the switch moves (auto-save)
watch(enabled, () => {
  status.value = undefined
  setTimeout(check, 900)
})
watch(() => state.app?.settings.tunEnabled, () => setTimeout(check, 900))
onMounted(() => {
  check()
  window.addEventListener('focus', check)
})
onBeforeUnmount(() => window.removeEventListener('focus', check))

const problem = computed(() => {
  switch (status.value?.problem) {
    case 'missing':
      return { title: t('没有加入 Windows 启动项'), text: t('启动项被删除了（安全软件或优化工具常会这样做）。'), fix: t('重新添加') }
    case 'moved':
      return { title: t('启动项指向别的位置'), text: t('NextBox 被移动过，启动项还在启动旧位置的程序：{0}', status.value?.detail ?? ''), fix: t('改为当前位置') }
    case 'disabled':
      return { title: t('已被 Windows 禁用'), text: t('在任务管理器或“设置 → 应用 → 启动”里被关闭了，开机时会跳过 NextBox。'), fix: t('重新启用') }
    case 'tun':
      return {
        title: t('开机后 TUN 模式无法启动'),
        text: state.app?.elevated
          ? t('TUN 模式需要管理员权限，普通启动项拿不到。改用计划任务以管理员身份启动即可。')
          : t('TUN 模式需要管理员权限，普通启动项拿不到。请先“以管理员身份重启”，再点修复。'),
        fix: t('改用计划任务'),
      }
    case 'user':
      return {
        title: t('计划任务属于另一个账户'),
        text: t('它只在 {0} 登录时运行，用现在这个账户登录时不会启动 NextBox。', status.value?.detail ?? ''),
        fix: t('改为当前账户'),
      }
  }
  return null
})

const location = computed(() => {
  switch (status.value?.location) {
    case 'temp':
      return t('NextBox 在临时文件夹里运行（可能是直接从压缩包里打开的），重启后这个文件夹可能就没了。请先解压到一个固定的文件夹，再从那里打开 NextBox。')
    case 'removable':
      return t('NextBox 放在 U 盘或移动硬盘上，开机时没插上就无法启动。建议放到电脑自己的硬盘里。')
  }
  return ''
})

// ---- the last starts by Windows
const records = computed(() => (status.value?.records ?? []).slice(0, 3))
function when(at: number) {
  const d = new Date(at * 1000)
  const hm = d.toTimeString().slice(0, 5)
  const day = new Date()
  day.setHours(0, 0, 0, 0)
  const diff = Math.floor((day.getTime() - new Date(d).setHours(0, 0, 0, 0)) / 86400000)
  if (diff === 0) return t('今天 {0}', hm)
  if (diff === 1) return t('昨天 {0}', hm)
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${hm}`
}
function describe(r: AutoStartRecord): { cls: string; text: string; note?: string } {
  switch (r.result) {
    case 'connected':
      return { cls: 'good', text: t('已自动连接') }
    case 'failed':
      return { cls: 'bad', text: t('自动连接失败'), note: r.detail }
    case 'locked':
      return { cls: 'mid', text: t('等待输入启动密码'), note: t('输入密码后才会连接') }
    case 'noNode':
      return { cls: 'mid', text: t('没有自动连接'), note: t('还没有选择节点') }
    case 'started':
      return { cls: 'good', text: t('已启动'), note: r.detail }
    case 'missed':
      return { cls: 'bad', text: t('这次开机没有启动'), note: t('可能被安全软件拦截了：在 360、火绒、电脑管家等软件的“启动项管理”里允许 NextBox') }
  }
  return { cls: '', text: r.result }
}

async function fix() {
  fixing.value = true
  const s = await run(() => api.repairAutoStart())
  fixing.value = false
  if (s) status.value = s
  else check()
}
</script>

<template>
  <div class="form-row autostart">
    <div class="label">{{ t('开机自启') }}<span class="hint">{{ t('启动 Windows 后自动运行') }}</span></div>
    <div class="control">
      <span class="spacer" />
      <template v-if="enabled && status">
        <span v-if="status.ok" :key="'ok' + seq" class="status ok" :title="status.method === 'task' ? t('通过计划任务以管理员身份启动') : t('通过注册表启动项启动')">
          <svg viewBox="0 0 24 24"><circle class="ring" cx="12" cy="12" r="9" /><path class="mark" d="M7.5 12.5 10.5 15.5 16.5 9" /></svg>
          <span class="txt">{{ t('已加入 Windows 启动项') }}</span>
        </span>
        <span v-else :key="'bad' + seq" class="status bad">
          <svg viewBox="0 0 24 24"><circle class="ring" cx="12" cy="12" r="9" /><path class="mark" d="M9 9l6 6" /><path class="mark m2" d="M15 9l-6 6" /></svg>
          <span class="txt">{{ t('未生效') }}</span>
        </span>
      </template>
      <Switch v-model="enabled" />
    </div>
  </div>
  <div v-if="enabled && problem" class="fail">
    <div>
      <b>{{ problem.title }}</b>
      <span>{{ problem.text }}</span>
    </div>
    <button class="btn small" :disabled="fixing" @click="fix">{{ fixing ? t('正在修复…') : problem.fix }}</button>
    <button v-if="(status?.problem === 'tun' || status?.problem === 'user') && !state.app?.elevated" class="btn small" @click="run(() => api.restartAsAdmin())">{{ t('以管理员身份重启') }}</button>
  </div>
  <div v-if="enabled && location" class="fail warn-box">
    <div>
      <b>{{ t('程序放的位置开机时可能找不到') }}</b>
      <span>{{ location }}</span>
    </div>
  </div>
  <div v-if="enabled && records.length" class="records">
    <div class="records-title">{{ t('最近几次开机') }}</div>
    <div v-for="r in records" :key="r.at + r.result" class="record">
      <span class="at mono">{{ when(r.at) }}</span>
      <span :class="describe(r).cls">{{ describe(r).text }}</span>
      <span v-if="describe(r).note" class="note" :title="describe(r).note">{{ describe(r).note }}</span>
    </div>
  </div>
</template>

<style scoped>
.status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 700;
}
.status svg {
  width: 20px;
  height: 20px;
  fill: none;
  stroke: currentColor;
}
.status.ok {
  color: var(--good);
}
.status.bad {
  color: var(--bad);
}
.ring {
  stroke-width: 2;
  stroke-dasharray: 57;
  stroke-dashoffset: 57;
  animation: draw 0.45s ease-out forwards;
}
.mark {
  stroke-width: 2.4;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-dasharray: 20;
  stroke-dashoffset: 20;
  animation: draw 0.3s 0.35s ease-out forwards;
}
.mark.m2 {
  animation-delay: 0.5s;
}
.txt {
  opacity: 0;
  animation: fade 0.3s 0.45s forwards;
}
@keyframes draw {
  to {
    stroke-dashoffset: 0;
  }
}
@keyframes fade {
  to {
    opacity: 1;
  }
}
.fail {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin: 2px 0 8px;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--bad) 8%, var(--card));
  font-size: 12px;
}
.fail > div {
  flex: 1 1 260px;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.fail b {
  color: var(--bad);
}
.fail span {
  color: var(--text-2);
  line-height: 1.6;
  word-break: break-all;
}
.warn-box {
  background: color-mix(in srgb, var(--mid) 10%, var(--card));
}
.warn-box b {
  color: var(--mid);
}
.records {
  margin: 0 0 10px 236px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  font-size: 12px;
  overflow: hidden;
}
.records-title {
  padding: 6px 10px;
  font-size: 11px;
  font-weight: 700;
  color: var(--text-3);
  background: var(--hover);
}
.record {
  display: grid;
  grid-template-columns: 86px auto minmax(0, 1fr);
  align-items: baseline;
  gap: 10px;
  padding: 6px 10px;
  border-top: 1px solid var(--line);
}
.record .at {
  color: var(--text-3);
  font-size: 11.5px;
}
.record .good {
  color: var(--good);
  font-weight: 700;
}
.record .bad {
  color: var(--bad);
  font-weight: 700;
}
.record .mid {
  color: var(--mid);
  font-weight: 700;
}
.record .note {
  color: var(--text-2);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
@media (max-width: 900px) {
  .records {
    margin-left: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .ring,
  .mark,
  .txt {
    animation-duration: 0.01ms;
    animation-delay: 0ms;
  }
}
</style>
