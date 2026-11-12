<script setup lang="ts">
// Shown once, the first time nodes are added: explains where the flags come
// from and lets the user choose (Settings → 订阅与测速 changes it later).
import { t } from '../i18n'
import { computed, ref, watch } from 'vue'
import Modal from './Modal.vue'
import { patchSettings, state } from '../state'
import { run } from '../util'

const open = computed({
  get: () => state.countryNotice,
  set: (v) => {
    if (!v) done(state.app?.settings.countrySource ?? 'exit')
  },
})
const choice = ref<'exit' | 'server'>('exit')
watch(open, (v) => v && (choice.value = state.app?.settings.countrySource ?? 'exit'))

function done(source: 'exit' | 'server') {
  state.countryNotice = false
  run(() => patchSettings({ countrySource: source, countryNoticeShown: true }))
}

const options = [
  {
    value: 'exit' as const,
    title: t('按落地 IP（推荐）'),
    lines: [t('显示网站实际看到的地区，中转节点也准确'), t('在 URL 测试时顺便检测，每个节点多一次请求（经节点访问 Cloudflare）')],
  },
  {
    value: 'server' as const,
    title: t('按服务器 IP'),
    lines: [t('显示节点服务器所在地区，不经过节点，添加后马上就有'), t('首次使用时下载约 4 MB 的 IP 数据库；中转节点会显示入口地区')],
  },
]
</script>

<template>
  <Modal v-model="open" :title="t('节点前的国旗是怎么来的')" :width="520">
    <p class="lead">
      {{ t('NextBox 会在每个节点名称前显示国旗，并支持按国家搜索，例如搜“香港”。检测出结果之前，先根据节点名称推测。请选择以哪个 IP 为准：') }}
    </p>
    <div class="options">
      <button
        v-for="o in options"
        :key="o.value"
        class="option"
        :class="{ on: choice === o.value }"
        @click="choice = o.value"
      >
        <span class="radio" />
        <span class="body">
          <b>{{ o.title }}</b>
          <span v-for="l in o.lines" :key="l" class="line">{{ l }}</span>
        </span>
      </button>
    </div>
    <p class="faint note">{{ t('以后可以在「设置 → 订阅与测速」里更改。') }}</p>
    <template #footer>
      <button class="btn primary" @click="done(choice)">{{ t('好的') }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.lead {
  margin: 2px 0 12px;
  line-height: 1.7;
  color: var(--text-2);
}
.options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.option {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  text-align: left;
  padding: 12px 14px;
  border-radius: var(--radius);
  border: 1px solid var(--card-border);
  background: var(--card);
  color: inherit;
  font: inherit;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.option:hover {
  border-color: var(--accent-line);
}
.option.on {
  border-color: var(--accent);
  background: var(--accent-soft);
}
.radio {
  flex-shrink: 0;
  margin-top: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid var(--input-border);
  transition: border 0.15s;
}
.option.on .radio {
  border: 5px solid var(--accent);
}
.body {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.body b {
  font-size: 14px;
}
.line {
  font-size: 12px;
  color: var(--text-2);
  line-height: 1.55;
}
.note {
  margin: 12px 0 0;
  font-size: 12px;
}
</style>
