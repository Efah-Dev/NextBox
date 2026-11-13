<script setup lang="ts">
// 开机自启 with a check of what Windows will really do: an animated green
// tick when NextBox is registered and enabled, a red cross with the reason
// and a fix otherwise.
import { t } from '../../i18n'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Switch from '../../components/Switch.vue'
import { api, type AutoStartStatus } from '../../api'
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
  }
  return null
})

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
    <button v-if="status?.problem === 'tun' && !state.app?.elevated" class="btn small" @click="run(() => api.restartAsAdmin())">{{ t('以管理员身份重启') }}</button>
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
@media (prefers-reduced-motion: reduce) {
  .ring,
  .mark,
  .txt {
    animation-duration: 0.01ms;
    animation-delay: 0ms;
  }
}
</style>
