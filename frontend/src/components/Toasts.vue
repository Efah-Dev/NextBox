<script setup lang="ts">
import { toasts } from '../util'
</script>

<template>
  <div class="toasts">
    <TransitionGroup name="toast">
      <div v-for="t in toasts" :key="t.id" class="toast" :class="t.kind">
        <span class="bar" />
        <span class="text">{{ t.text }}</span>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toasts {
  position: fixed;
  left: 16px;
  bottom: 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  z-index: 100;
  pointer-events: none;
}
.toast {
  display: flex;
  align-items: stretch;
  max-width: 420px;
  background: var(--card);
  border-radius: 6px;
  box-shadow: 0 6px 20px rgba(10, 30, 70, 0.16);
  overflow: hidden;
  pointer-events: auto;
}
.bar {
  width: 5px;
  flex-shrink: 0;
  background: var(--accent);
}
.toast.success .bar {
  background: var(--good);
}
.toast.error .bar {
  background: var(--bad);
}
.text {
  padding: 10px 14px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  user-select: text;
}
.toast-enter-active,
.toast-leave-active {
  transition: transform 0.28s cubic-bezier(0.2, 0.9, 0.3, 1.1), opacity 0.2s;
}
.toast-enter-from,
.toast-leave-to {
  transform: translateX(-110%);
  opacity: 0;
}
.toast-move {
  transition: transform 0.25s;
}
</style>
