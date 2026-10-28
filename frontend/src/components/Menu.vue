<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import Icon from './Icon.vue'
import type { MenuItem } from '../util'

defineProps<{ items: MenuItem[]; icon?: string; label?: string }>()

const open = ref(false)
const root = ref<HTMLElement>()

function onDoc(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false
}
function toggle() {
  open.value = !open.value
  if (open.value) document.addEventListener('mousedown', onDoc)
  else document.removeEventListener('mousedown', onDoc)
}
function pick(item: MenuItem) {
  if (item.disabled) return
  toggle()
  item.action()
}
onBeforeUnmount(() => document.removeEventListener('mousedown', onDoc))
</script>

<template>
  <div ref="root" class="menu-wrap">
    <button class="btn" :class="{ icon: !label }" @click="toggle">
      <Icon :name="icon ?? 'more'" :size="16" :stroke="label ? 2 : 3" />
      <span v-if="label">{{ label }}</span>
    </button>
    <Transition name="pop">
      <div v-if="open" class="menu">
        <button
          v-for="(it, i) in items"
          :key="i"
          class="menu-item"
          :class="{ danger: it.danger }"
          :disabled="it.disabled"
          @click="pick(it)"
        >
          <Icon v-if="it.icon" :name="it.icon" :size="15" />
          <span>{{ it.label }}</span>
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.menu-wrap {
  position: relative;
}
.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  min-width: 168px;
  padding: 5px;
  background: var(--card);
  border: 1px solid var(--card-border);
  border-radius: 8px;
  box-shadow: 0 10px 30px rgba(10, 30, 70, 0.18);
  z-index: 40;
}
.menu-item {
  width: 100%;
  height: 32px;
  white-space: nowrap;
  border: none;
  background: transparent;
  border-radius: 5px;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 0 10px;
  font: inherit;
  color: var(--text);
  cursor: pointer;
  text-align: left;
}
.menu-item:hover {
  background: var(--accent-soft);
  color: var(--accent);
}
.menu-item.danger:hover {
  background: color-mix(in srgb, var(--bad) 10%, var(--card));
  color: var(--bad);
}
.menu-item:disabled {
  opacity: 0.45;
  pointer-events: none;
}
.pop-enter-active,
.pop-leave-active {
  transition: opacity 0.14s, transform 0.16s;
  transform-origin: top right;
}
.pop-enter-from,
.pop-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
