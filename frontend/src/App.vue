<script setup lang="ts">
import { onMounted, onUnmounted, ref, watchEffect } from 'vue'
import TitleBar from './components/TitleBar.vue'
import Notices from './components/Notices.vue'
import DropZone from './components/DropZone.vue'
import { importText } from './importer'
import DialogHost from './components/DialogHost.vue'
import ClashImportDialog from './components/ClashImportDialog.vue'
import CountryNotice from './components/CountryNotice.vue'
import LockScreen from './components/LockScreen.vue'
import Home from './views/Home.vue'
import Nodes from './views/Nodes.vue'
import Connections from './views/Connections.vue'
import Logs from './views/Logs.vue'
import Settings from './views/Settings.vue'
import { init, settings, state } from './state'
import { setLang } from './i18n'
import { rt } from './api'
import { toast } from './util'
import { applyLevel } from './theme'

const pages = { home: Home, nodes: Nodes, connections: Connections, logs: Logs, settings: Settings }
const ready = ref(false)
const fatal = ref('')

const darkQuery = window.matchMedia('(prefers-color-scheme: dark)')
const systemDark = ref(darkQuery.matches)
darkQuery.addEventListener('change', (e) => (systemDark.value = e.matches))

// Translations are not reactive: the whole UI is re-rendered (keyed by language)
// when the language changes.
const lang = ref('zh-CN')
watchEffect(() => {
  const l = settings.value?.language ?? 'zh-CN'
  setLang(l)
  lang.value = l
})

watchEffect(() => {
  // while the appearance is being edited it follows the sliders at once,
  // ahead of the (debounced) save
  const s = settings.value && { ...settings.value, ...(state.look ?? {}) }
  const root = document.documentElement
  const dark = s?.theme === 'dark' || (s?.theme !== 'light' && systemDark.value)
  root.classList.toggle('dark', dark)
  if (s?.accentColor) root.style.setProperty('--accent', s.accentColor)
  applyLevel(root, dark, dark ? s?.darkLevel ?? 60 : s?.lightLevel ?? 20)
})

onMounted(async () => {
  try {
    await init()
    rt.on('error', (msg: string) => toast(msg, 'error'))
    rt.on('notice', (n: { text: string; kind: 'info' | 'success' | 'error' }) => toast(n.text, n.kind))
    ready.value = true
  } catch (e) {
    fatal.value = (e as Error).message
  }
})

// Ctrl+V on the home and node pages imports what the clipboard holds; in a
// text field, or with a dialog open, it is an ordinary paste.
async function onKey(e: KeyboardEvent) {
  if (!(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== 'v') return
  if (!ready.value || state.app?.locked || (state.page !== 'home' && state.page !== 'nodes')) return
  const el = document.activeElement as HTMLElement | null
  if (el && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName))) return
  if (document.querySelector('.mask')) return
  e.preventDefault()
  importText(await rt.clipboardGet())
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div :key="lang" class="shell">
    <TitleBar />
    <Transition name="page" mode="out-in">
      <component :is="pages[state.page]" v-if="ready" :key="state.page" />
      <div v-else class="page boot">
        <div class="empty">
          <div v-if="fatal" class="muted">{{ fatal }}</div>
          <div v-else class="loader" />
        </div>
      </div>
    </Transition>
    <Notices />
    <DropZone v-if="ready" />
    <DialogHost />
    <ClashImportDialog />
    <CountryNotice />
    <LockScreen v-if="ready && state.app?.locked" />
  </div>
</template>

<style scoped>
.boot {
  align-items: center;
  justify-content: center;
}
.loader {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  border: 3px solid var(--accent-soft-2);
  border-top-color: var(--accent);
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
