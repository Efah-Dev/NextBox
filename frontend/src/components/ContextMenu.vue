<script setup lang="ts">
// The one menu for right clicks (and anything else that opens at a point):
// section titles, separators, check marks, submenus that open on hover.
// It stays inside the window: near the right or bottom edge it opens to the
// left or upwards, and so do its submenus.
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'

export interface CtxItem {
  label?: string
  icon?: string
  /** small text on the right: a shortcut or what the item points to */
  hint?: string
  /** a section title, not clickable */
  title?: boolean
  sep?: boolean
  /** shows a check mark (one choice of several) */
  checked?: boolean
  /** picking it leaves the menu open (choosing an option in it) */
  keep?: boolean
  danger?: boolean
  disabled?: boolean
  children?: CtxItem[]
  action?: () => void
}

const props = defineProps<{ items: CtxItem[]; x: number; y: number }>()
const emit = defineEmits<{ close: [] }>()

const root = ref<HTMLElement>()
const pos = ref({ left: props.x, top: props.y })
const sub = ref(-1) // index of the item whose submenu is open
const subStyle = ref<Record<string, string>>({})
const subEl = ref<HTMLElement>()
const GAP = 8

onMounted(() => {
  const el = root.value
  if (!el) return
  const r = el.getBoundingClientRect()
  pos.value = {
    left: Math.max(GAP, Math.min(props.x, window.innerWidth - r.width - GAP)),
    top: Math.max(GAP, Math.min(props.y, window.innerHeight - r.height - GAP)),
  }
  document.addEventListener('mousedown', onDoc, true)
  document.addEventListener('keydown', onKey)
  window.addEventListener('blur', close)
  window.addEventListener('resize', close)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDoc, true)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('blur', close)
  window.removeEventListener('resize', close)
})

function close() {
  emit('close')
}
function onDoc(e: MouseEvent) {
  if (!root.value?.contains(e.target as Node)) close()
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

async function openSub(i: number, e: MouseEvent) {
  if (sub.value === i) return
  sub.value = props.items[i].children ? i : -1
  if (sub.value < 0) return
  // the submenu sits in the row it belongs to: offsets are from that row
  const row = (e.currentTarget as HTMLElement).getBoundingClientRect()
  subStyle.value = { visibility: 'hidden', top: '-5px', left: '100%' }
  await nextTick()
  const s = subEl.value?.getBoundingClientRect()
  if (!s) return
  const toLeft = row.right + s.width + GAP > window.innerWidth
  let top = -5
  const overflow = row.top + top + s.height - (window.innerHeight - GAP)
  if (overflow > 0) top -= overflow
  subStyle.value = toLeft ? { top: top + 'px', right: '100%' } : { top: top + 'px', left: '100%' }
}

function pick(item: CtxItem) {
  if (item.disabled || item.title || item.sep || item.children) return
  item.action?.()
  if (!item.keep) close()
}
</script>

<template>
  <Teleport to="body">
    <Transition name="nb-pop" appear>
    <div ref="root" class="ctx" :style="{ left: pos.left + 'px', top: pos.top + 'px' }" role="menu" @contextmenu.prevent>
      <template v-for="(it, i) in items" :key="i">
        <div v-if="it.sep" class="sep" />
        <div v-else-if="it.title" class="title">{{ it.label }}</div>
        <div
          v-else
          class="item"
          role="menuitem"
          :class="{ on: sub === i, danger: it.danger, disabled: it.disabled }"
          @mouseenter="openSub(i, $event)"
          @click="pick(it)"
        >
          <span class="ic">
            <Icon v-if="it.checked" name="check" :size="14" :stroke="2.6" class="mark" />
            <Icon v-else-if="it.icon" :name="it.icon" :size="14" />
          </span>
          <span class="label">{{ it.label }}</span>
          <span v-if="it.children" class="hint"><Icon name="chevron" :size="12" /></span>
          <span v-else-if="it.hint" class="hint">{{ it.hint }}</span>
          <div v-if="it.children && sub === i" :ref="(el) => (subEl = el as HTMLElement)" class="ctx sub" :style="subStyle" role="menu">
            <template v-for="(c, j) in it.children" :key="j">
              <div v-if="c.sep" class="sep" />
              <div v-else-if="c.title" class="title">{{ c.label }}</div>
              <div v-else class="item" role="menuitem" :class="{ danger: c.danger, disabled: c.disabled }" @click.stop="pick(c)">
                <span class="ic">
                  <Icon v-if="c.checked" name="check" :size="14" :stroke="2.6" class="mark" />
                  <Icon v-else-if="c.icon" :name="c.icon" :size="14" />
                </span>
                <span class="label">{{ c.label }}</span>
                <span v-if="c.hint" class="hint">{{ c.hint }}</span>
              </div>
            </template>
          </div>
        </div>
      </template>
    </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.ctx {
  position: fixed;
  z-index: 60;
  min-width: 200px;
  max-width: 320px;
  padding: 4px;
  background: var(--card);
  border: 1px solid var(--card-border);
  border-radius: var(--radius);
  box-shadow: 0 10px 30px rgba(10, 30, 70, 0.18);
  font-size: 12px;
  user-select: none;
}
.ctx.sub {
  position: absolute;
  margin: 0 4px;
  animation: sub-in var(--t-base) var(--ease-out);
}
@keyframes sub-in {
  from {
    opacity: 0;
    transform: translateX(-4px);
  }
}
.title {
  padding: 6px 10px 4px;
  font-size: 11px;
  font-weight: 700;
  color: var(--text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sep {
  height: 1px;
  margin: 4px 6px;
  background: var(--line);
}
.item {
  position: relative;
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  color: var(--text);
  cursor: pointer;
}
.item:hover,
.item.on {
  background: var(--hover);
}
.item.danger {
  color: var(--bad);
}
.item.disabled {
  color: var(--text-3);
  cursor: default;
}
.item.disabled:hover {
  background: none;
}
.ic {
  display: grid;
  place-items: center;
  color: var(--text-2);
}
.ic .mark {
  color: var(--accent);
}
.label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hint {
  display: flex;
  align-items: center;
  font-size: 11px;
  color: var(--text-3);
  white-space: nowrap;
}
</style>
