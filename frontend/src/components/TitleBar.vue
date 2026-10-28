<script setup lang="ts">
import { t } from '../i18n'
import { onMounted, onUnmounted } from 'vue'
import Icon from './Icon.vue'
import PawLogo from './PawLogo.vue'
import { state, type Page } from '../state'
import { api, rt } from '../api'

const tabs: { id: Page; label: string; icon: string }[] = [
  { id: 'home', label: t('启动'), icon: 'home' },
  { id: 'nodes', label: t('节点'), icon: 'nodes' },
  { id: 'connections', label: t('连接'), icon: 'connections' },
  { id: 'logs', label: t('日志'), icon: 'logs' },
  { id: 'settings', label: t('设置'), icon: 'settings' },
]

// Window dragging and double-click maximise are handled here instead of by
// Wails' built-in drag region. Wails starts a native move loop on the first
// tiny mouse movement after mousedown; on a maximised window that loop eats
// the second click, so "double-click to restore" never arrived. Here the drag
// only starts after the mouse really moves, so both clicks always reach us.
const w = window as any
let lastDown = { t: 0, x: 0, y: 0 }
let press: { x: number; y: number } | null = null

function onMouseDown(e: MouseEvent) {
  if (e.button !== 0 || (e.target as HTMLElement).closest('.tabs, .win')) return
  if (w.wails?.flags?.resizeEdge) return // let Wails resize from the top edge
  e.stopPropagation() // keep Wails' own drag handler out of it
  const now = performance.now()
  const near = Math.abs(e.screenX - lastDown.x) < 6 && Math.abs(e.screenY - lastDown.y) < 6
  if (now - lastDown.t < 450 && near) {
    lastDown.t = 0
    press = null
    rt.toggleMaximise()
    return
  }
  lastDown = { t: now, x: e.screenX, y: e.screenY }
  press = { x: e.screenX, y: e.screenY }
}

function onMouseMove(e: MouseEvent) {
  if (!press) return
  if (!(e.buttons & 1)) {
    press = null
    return
  }
  if (Math.abs(e.screenX - press.x) > 4 || Math.abs(e.screenY - press.y) > 4) {
    press = null
    w.WailsInvoke?.('drag')
  }
}

const release = () => (press = null)
onMounted(() => {
  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', release)
})
onUnmounted(() => {
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', release)
})

const modeLabel = { system: t('系统代理'), tun: t('TUN 模式'), '': t('已连接') }
</script>

<template>
  <header class="titlebar" @mousedown.capture="onMouseDown">
    <div class="brand">
      <span class="logo"><PawLogo :size="22" /></span>
      <span class="name">NextBox</span>
      <span
        class="dot"
        :class="state.app?.running ? 'on ' + state.app.proxyMode : ''"
        :title="state.app?.running ? modeLabel[state.app.proxyMode] : t('未连接')"
      />
    </div>

    <nav class="tabs" style="--wails-draggable: no-drag">
      <button
        v-for="t in tabs"
        :key="t.id"
        class="tab"
        :class="{ active: state.page === t.id }"
        @click="state.page = t.id"
      >
        <Icon :name="t.icon" :size="16" />
        <span>{{ t.label }}</span>
      </button>
    </nav>

    <div class="win" style="--wails-draggable: no-drag">
      <button class="win-btn" :title="t('最小化')" @click="rt.minimise()"><Icon name="minus" :size="16" /></button>
      <button class="win-btn" :title="t('最大化')" @click="rt.toggleMaximise()"><Icon name="maximize" :size="14" /></button>
      <button class="win-btn close" :title="t('关闭')" @click="api.closeWindow()"><Icon name="close" :size="16" /></button>
    </div>
  </header>
</template>

<style scoped>
.titlebar {
  height: var(--titlebar);
  flex-shrink: 0;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  padding: 0 6px 0 12px;
  background: linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--accent) 80%, #ffffff));
  color: #fff;
  box-shadow: 0 2px 10px color-mix(in srgb, var(--accent) 30%, transparent);
  z-index: 10;
}

.brand {
  display: flex;
  align-items: center;
  gap: 9px;
  pointer-events: none;
}
.logo {
  width: 30px;
  height: 30px;
  border-radius: 9px;
  background: #fff;
  display: grid;
  place-items: center;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.12);
}
.name {
  font-size: 16px;
  font-weight: 800;
  letter-spacing: 0.02em;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.35);
  transition: background 0.3s, box-shadow 0.3s;
}
.dot.on {
  background: #6dffb0;
  box-shadow: 0 0 0 3px rgba(109, 255, 176, 0.25);
}
.dot.on.tun {
  background: #ff6b6b;
  box-shadow: 0 0 0 3px rgba(255, 107, 107, 0.3);
}

.tabs {
  display: flex;
  gap: 4px;
}
.tab {
  height: 32px;
  padding: 0 16px;
  border-radius: 16px;
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.92);
  font: inherit;
  font-size: 13px;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  transition: background 0.18s, color 0.18s, transform 0.1s;
}
.tab:hover {
  background: rgba(255, 255, 255, 0.16);
}
.tab:active {
  transform: scale(0.96);
}
.tab.active {
  background: #fff;
  color: var(--accent);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);
}

.win {
  justify-self: end;
  display: flex;
}
.win-btn {
  width: 40px;
  height: 34px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #fff;
  display: grid;
  place-items: center;
  cursor: pointer;
  transition: background 0.15s;
}
.win-btn:hover {
  background: rgba(255, 255, 255, 0.18);
}
.win-btn.close:hover {
  background: #e5484d;
}
</style>
