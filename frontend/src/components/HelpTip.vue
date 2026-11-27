<script setup lang="ts">
// The circled question mark next to a setting's name: short name and one
// line under it, the details in here (shown on hover or keyboard focus).
// <HelpTip :text="t('…')" />, or put rich content in the slot.
import { nextTick, ref } from 'vue'

defineProps<{ text?: string; title?: string }>()

const btn = ref<HTMLElement>()
const pop = ref<HTMLElement>()
const open = ref(false)
const pos = ref({ left: 0, top: 0, below: true })
let hideTimer = 0
const GAP = 8
const WIDTH = 280

async function show() {
  clearTimeout(hideTimer)
  if (open.value) return
  const r = btn.value?.getBoundingClientRect()
  if (!r) return
  pos.value = { left: Math.max(GAP, Math.min(r.left + r.width / 2 - WIDTH / 2, window.innerWidth - WIDTH - GAP)), top: r.bottom + 6, below: true }
  open.value = true
  await nextTick()
  const h = pop.value?.offsetHeight ?? 0
  if (r.bottom + 6 + h > window.innerHeight - GAP) pos.value = { ...pos.value, top: r.top - 6 - h, below: false }
}
function hide() {
  hideTimer = window.setTimeout(() => (open.value = false), 80)
}
</script>

<template>
  <button
    ref="btn"
    type="button"
    class="help-tip"
    :aria-label="title || text"
    @mouseenter="show"
    @mouseleave="hide"
    @focus="show"
    @blur="hide"
    @click.stop.prevent="show"
  >
    ?
  </button>
  <Teleport to="body">
    <Transition name="nb-pop">
      <div
        v-if="open"
        ref="pop"
        class="help-pop"
        :class="{ above: !pos.below }"
        :style="{ left: pos.left + 'px', top: pos.top + 'px', width: WIDTH + 'px' }"
        role="tooltip"
        @mouseenter="show"
        @mouseleave="hide"
      >
        <b v-if="title">{{ title }}</b>
        <slot>{{ text }}</slot>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.help-tip {
  display: inline-grid;
  place-items: center;
  width: 15px;
  height: 15px;
  margin-left: 5px;
  padding: 0;
  border: 1.3px solid var(--text-3);
  border-radius: 50%;
  background: transparent;
  color: var(--text-3);
  font: 700 10px/1 var(--font);
  vertical-align: 1px;
  cursor: help;
  flex-shrink: 0;
  transition: color var(--t-fast), border-color var(--t-fast);
}
.help-tip:hover,
.help-tip:focus-visible {
  color: var(--accent);
  border-color: var(--accent);
  outline: none;
}
.help-pop {
  position: fixed;
  z-index: 120;
  padding: 10px 12px;
  background: var(--card);
  color: var(--text-2);
  border: 1px solid var(--card-border);
  border-top: 2px solid var(--accent);
  border-radius: var(--radius);
  box-shadow: 0 12px 32px rgba(10, 30, 70, 0.18);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.65;
  white-space: pre-line;
  user-select: text;
}
.help-pop b {
  display: block;
  margin-bottom: 2px;
  color: var(--text);
  font-weight: 700;
}
.help-pop.above {
  border-top: 1px solid var(--card-border);
  border-bottom: 2px solid var(--accent);
}
</style>
