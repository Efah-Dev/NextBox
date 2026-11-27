<script setup lang="ts">
// The messages in the bottom right corner (toast() in util.ts). Each one
// closes when its time bar runs out; hovering it stops the bar.
import Icon from './Icon.vue'
import { closeToast, toasts } from '../util'

const icon = { info: 'info', success: 'check', error: 'warn' } as const
</script>

<template>
  <div class="notices">
    <TransitionGroup name="nb-notice">
      <div v-for="n in toasts" :key="n.id" class="notice" :class="n.kind" role="status">
        <span class="ic"><Icon :name="icon[n.kind]" :size="13" :stroke="2.4" /></span>
        <span class="text">{{ n.text }}</span>
        <button v-if="n.action" class="act" @click="n.action.run">{{ n.action.label }}</button>
        <button class="x" :aria-label="'×'" @click="closeToast(n.id)"><Icon name="close" :size="11" :stroke="2.4" /></button>
        <span class="life" :style="{ '--life': n.ms + 'ms' }" @animationend="closeToast(n.id)" />
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.notices {
  position: fixed;
  right: 16px;
  bottom: 16px;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  z-index: 100;
  pointer-events: none;
}
.notice {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 9px;
  width: 320px;
  padding: 10px 10px 12px 12px;
  background: var(--card);
  border: 1px solid var(--card-border);
  border-radius: var(--radius);
  box-shadow: 0 10px 30px rgba(10, 30, 70, 0.16), 0 2px 6px rgba(10, 30, 70, 0.06);
  overflow: hidden;
  pointer-events: auto;
}
.ic {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: var(--accent);
  background: var(--accent-soft);
}
.success .ic {
  color: var(--good);
  background: color-mix(in srgb, var(--good) 14%, var(--card));
}
.error .ic {
  color: var(--bad);
  background: color-mix(in srgb, var(--bad) 13%, var(--card));
}
.text {
  flex: 1;
  min-width: 0;
  padding-top: 2px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  user-select: text;
}
.act {
  flex-shrink: 0;
  border: 0;
  border-radius: var(--radius-sm);
  padding: 3px 10px;
  background: var(--accent-soft);
  color: var(--accent);
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}
.act:hover {
  background: var(--accent-soft-2);
}
.x {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--text-3);
  display: grid;
  place-items: center;
  cursor: pointer;
}
.x:hover {
  background: var(--hover);
  color: var(--text);
}
.life {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 2px;
  background: var(--accent);
  transform-origin: left;
  animation: life var(--life) linear forwards;
}
.success .life {
  background: var(--good);
}
.error .life {
  background: var(--bad);
}
.notice:hover .life {
  animation-play-state: paused;
}
/* the bar is the timer: it keeps its length when animations are turned off */
@media (prefers-reduced-motion: reduce) {
  .notice .life {
    animation-duration: var(--life) !important;
  }
}
@keyframes life {
  to {
    transform: scaleX(0);
  }
}
</style>
