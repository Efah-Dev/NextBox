<script setup lang="ts">
// The overlay shown while files are dragged over the window; Wails hands the
// dropped files' paths to importFiles.
import { onMounted, onUnmounted, ref } from 'vue'
import { t } from '../i18n'
import Icon from './Icon.vue'
import { rt } from '../api'
import { importFiles } from '../importer'
import { state } from '../state'

const over = ref(false)
let depth = 0 // dragenter / dragleave also fire for every child element

const hasFiles = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
function enter(e: DragEvent) {
  if (!hasFiles(e) || state.app?.locked) return
  depth++
  over.value = true
}
function leave(e: DragEvent) {
  if (!hasFiles(e)) return
  depth = Math.max(0, depth - 1)
  if (!depth) over.value = false
}
function end() {
  depth = 0
  over.value = false
}

onMounted(() => {
  window.addEventListener('dragenter', enter)
  window.addEventListener('dragleave', leave)
  window.addEventListener('drop', end)
  window.addEventListener('dragend', end)
  rt.onFileDrop((paths) => {
    end()
    if (paths.length && !state.app?.locked) importFiles(paths)
  })
})
onUnmounted(() => {
  window.removeEventListener('dragenter', enter)
  window.removeEventListener('dragleave', leave)
  window.removeEventListener('drop', end)
  window.removeEventListener('dragend', end)
})
</script>

<template>
  <Transition name="nb-fade">
    <div v-if="over" class="drop">
      <div class="zone">
        <div class="big"><Icon name="download" :size="24" /></div>
        <b>{{ t('松开即可导入') }}</b>
        <span class="faint">{{ t('根据文件内容自动识别类型') }}</span>
        <div class="kinds">
          <span>{{ t('Clash 配置') }}</span>
          <span>{{ t('sing-box 配置') }}</span>
          <span>{{ t('NextBox 备份') }}</span>
          <span>{{ t('链接列表') }}</span>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.drop {
  position: fixed;
  inset: var(--titlebar) 0 0 0;
  z-index: 90;
  display: grid;
  place-items: center;
  background: color-mix(in srgb, var(--bg) 70%, transparent);
  backdrop-filter: blur(4px);
  pointer-events: none;
}
.zone {
  width: min(560px, 80%);
  padding: 32px 24px;
  border: 2px dashed var(--accent);
  border-radius: 16px;
  background: color-mix(in srgb, var(--card) 90%, transparent);
  box-shadow: 0 18px 50px rgba(10, 30, 70, 0.14);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  text-align: center;
  animation: zone-in var(--t-slow) var(--ease-spring);
}
.big {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  display: grid;
  place-items: center;
  margin-bottom: 6px;
  background: var(--accent-soft-2);
  color: var(--accent);
}
.zone b {
  font-size: 16px;
}
.kinds {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
  margin-top: 10px;
}
.kinds span {
  font-size: 11.5px;
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--card);
  border: 1px solid var(--line);
  color: var(--text-2);
}
@keyframes zone-in {
  from {
    transform: scale(0.96);
  }
}
</style>
