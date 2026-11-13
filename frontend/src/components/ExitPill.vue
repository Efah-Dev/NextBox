<script setup lang="ts">
// Where the connected node's traffic leaves (日志卡片右下角): flag, node,
// exit IP and a status light. Click to check again.
import { t } from '../i18n'
import { computed, ref } from 'vue'
import Flag from './Flag.vue'
import Icon from './Icon.vue'
import { api } from '../api'
import { selectedProfile, state } from '../state'
import { bareName, countryName } from '../country'

const reveal = ref(false)
const exit = computed(() => state.exit)

/** "103.152.84.18" → "103.152.•••.18"; IPv6 keeps its first two groups. */
function mask(ip: string) {
  if (!ip) return ''
  if (ip.includes('.')) {
    const p = ip.split('.')
    return p.length === 4 ? `${p[0]}.${p[1]}.•••.${p[3]}` : ip
  }
  const p = ip.split(':')
  return `${p[0]}:${p[1]}:•••`
}

const light = computed(() => {
  const e = exit.value
  if (!e || e.checking) return 'wait'
  if (e.error || e.latency < 0) return 'bad'
  return e.latency < 300 ? 'ok' : 'mid'
})

const ipText = computed(() => {
  const e = exit.value
  if (!e || e.checking) return t('检测中…')
  if (e.error) return t('检测失败')
  return reveal.value ? e.ip : mask(e.ip)
})

const title = computed(() => {
  const e = exit.value
  if (!e || e.checking) return t('正在通过节点访问 Cloudflare，检测落地 IP…')
  if (e.error) return t('无法通过节点访问 Cloudflare：{0}。点击重新检测', e.error)
  const where = e.country ? countryName(e.country) : t('未知地区')
  const slow = e.latency >= 300 ? t('，延迟较高') : ''
  return t('落地 IP {0}（{1}），延迟 {2} ms{3}。点击重新检测', e.ip, where, e.latency, slow)
})

function refresh() {
  if (exit.value?.checking) return
  api.refreshExit()
}
</script>

<template>
  <button class="pill" :title="title" @click="refresh">
    <Flag :code="exit?.country ?? ''" :size="15" />
    <span class="txt">
      <span class="name ellipsis">{{ selectedProfile ? bareName(selectedProfile.name) : '' }}</span>
      <span class="ip mono">{{ ipText }}</span>
    </span>
    <span
      v-if="exit?.ip && !exit.checking"
      class="eye"
      :title="reveal ? t('隐藏完整 IP') : t('显示完整 IP')"
      @click.stop="reveal = !reveal"
    >
      <Icon :name="reveal ? 'eye-off' : 'eye'" :size="13" />
    </span>
    <span class="dot" :class="light" />
  </button>
</template>

<style scoped>
.pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  max-width: 100%;
  height: 30px;
  padding: 0 10px 0 11px;
  border-radius: 999px;
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--text);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  box-shadow: var(--shadow);
  transition: border-color 0.15s;
}
.pill:hover {
  border-color: var(--accent-line);
}
.txt {
  display: inline-flex;
  align-items: baseline;
  gap: 7px;
  min-width: 0;
  line-height: 1;
}
.name {
  font-weight: 700;
  min-width: 0;
  max-width: 160px;
  line-height: 1.25;
}
.ip {
  font-size: 12px;
  color: var(--text-2);
  white-space: nowrap;
  line-height: 1.25;
}
.eye {
  display: grid;
  place-items: center;
  color: var(--text-3);
  padding: 2px;
  border-radius: 5px;
}
.eye:hover {
  color: var(--accent);
  background: var(--hover);
}
.dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-left: 2px;
}
.dot.ok {
  background: var(--good);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--good) 22%, transparent);
}
.dot.mid {
  background: var(--mid);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--mid) 22%, transparent);
}
.dot.bad {
  background: var(--bad);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--bad) 22%, transparent);
}
.dot.wait {
  background: var(--text-3);
  animation: pulse 1s ease-in-out infinite;
}
@keyframes pulse {
  50% {
    opacity: 0.3;
  }
}
</style>
