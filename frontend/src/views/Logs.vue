<script setup lang="ts">
import { t } from '../i18n'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import Icon from '../components/Icon.vue'
import Switch from '../components/Switch.vue'
import Segmented from '../components/Segmented.vue'
import { api, rt } from '../api'
import { patchSettings, state, trimLogs } from '../state'
import { run, toast } from '../util'

const level = ref('all')
const query = ref('')
const follow = ref(true)
const box = ref<HTMLElement>()

function levelOf(line: string): string {
  if (/\b(FATAL|ERROR|PANIC)\b/.test(line)) return 'error'
  if (/\bWARN(ING)?\b/.test(line)) return 'warn'
  if (/\bDEBUG|TRACE\b/.test(line)) return 'debug'
  if (line.includes('[GUI]')) return 'gui'
  return 'info'
}

// how many lines the page keeps (also caps memory, see trimLogs)
const limit = computed({
  get: () => state.app?.settings.logViewLimit || 500,
  set: (v: number) =>
    run(async () => {
      await patchSettings({ logViewLimit: v })
      trimLogs()
    }),
})

const lines = computed(() => {
  const q = query.value.trim().toLowerCase()
  return state.logs
    .map((text, i) => ({ text, i, lv: levelOf(text) }))
    .filter((l) => {
      if (level.value === 'warn' && l.lv !== 'warn' && l.lv !== 'error') return false
      if (level.value === 'error' && l.lv !== 'error') return false
      return !q || l.text.toLowerCase().includes(q)
    })
})

function scrollToEnd() {
  if (follow.value && box.value) box.value.scrollTop = box.value.scrollHeight
}

watch(() => lines.value.length, () => nextTick(scrollToEnd))
onMounted(() => nextTick(scrollToEnd))

async function copy() {
  await rt.clipboardSet(lines.value.map((l) => l.text).join('\n'))
  toast(t('日志已复制'), 'success')
}

async function clear() {
  await api.clearLogs()
  state.logs = []
}
</script>

<template>
  <div class="page">
    <main class="content">
      <div class="toolbar">
        <Segmented
          v-model="level"
          class="levels"
          :options="[
            { value: 'all', label: t('全部') },
            { value: 'warn', label: t('警告') },
            { value: 'error', label: t('错误') },
          ]"
        />
        <div class="search">
          <Icon name="search" :size="14" />
          <input v-model="query" class="input" :placeholder="t('搜索日志')" />
        </div>
        <span class="spacer" />
        <select v-model.number="limit" class="select limit" :title="t('日志页最多显示的条数')">
          <option v-for="n in [200, 500, 1000, 2000]" :key="n" :value="n">{{ t('最多 {0} 条', n) }}</option>
        </select>
        <label class="row follow"><span class="muted">{{ t('自动滚动') }}</span><Switch v-model="follow" /></label>
        <button class="btn" @click="copy"><Icon name="copy" /> {{ t('复制') }}</button>
        <button class="btn danger" @click="clear"><Icon name="trash" /> {{ t('清空') }}</button>
      </div>

      <div ref="box" class="card log scroll mono">
        <div v-for="l in lines" :key="l.i" class="line" :class="l.lv">{{ l.text }}</div>
        <div v-if="!lines.length" class="empty"><Icon name="logs" :size="40" :stroke="1.5" />{{ t('暂无日志') }}</div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
}
.limit {
  width: 118px;
}
.levels {
  width: 210px;
}
.toolbar .search {
  width: 240px;
}
.follow {
  cursor: pointer;
  margin-right: 6px;
}
.log {
  flex: 1;
  min-height: 0;
  padding: 12px 14px;
  font-size: 12px;
  line-height: 1.7;
  user-select: text;
  animation: none;
}
.line {
  white-space: pre-wrap;
  word-break: break-all;
  padding: 0 6px;
  border-radius: 3px;
}
.line:hover {
  background: var(--hover);
}
.line.error {
  color: var(--bad);
}
.line.warn {
  color: var(--mid);
}
.line.debug {
  color: var(--text-3);
}
.line.gui {
  color: var(--accent);
}
</style>
