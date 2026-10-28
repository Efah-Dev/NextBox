<script setup lang="ts">
const open = defineModel<boolean>({ required: true })
withDefaults(defineProps<{ title: string; width?: number }>(), { width: 440 })
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="open" class="mask" @mousedown.self="open = false">
        <div class="modal" :style="{ width: width + 'px' }">
          <div class="modal-head">
            <span class="modal-title">{{ title }}</span>
            <button class="btn ghost small icon" @click="open = false">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M6 6l12 12M18 6 6 18" /></svg>
            </button>
          </div>
          <div class="modal-body scroll">
            <slot />
          </div>
          <div v-if="$slots.footer" class="modal-foot">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: var(--titlebar) 0 0 0;
  background: rgba(16, 28, 48, 0.28);
  backdrop-filter: blur(2px);
  display: grid;
  place-items: center;
  z-index: 50;
}
.modal {
  max-width: calc(100vw - 48px);
  max-height: calc(100vh - var(--titlebar) - 48px);
  display: flex;
  flex-direction: column;
  background: var(--card);
  border-radius: 10px;
  box-shadow: 0 18px 50px rgba(10, 30, 70, 0.25);
  border-top: 3px solid var(--accent);
}
.modal-head {
  display: flex;
  align-items: center;
  padding: 14px 14px 6px 20px;
}
.modal-title {
  flex: 1;
  font-size: 15px;
  font-weight: 700;
}
.modal-body {
  padding: 8px 20px 12px;
  min-height: 0;
}
.modal-foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 10px 20px 16px;
}
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.18s;
}
.modal-enter-active .modal,
.modal-leave-active .modal {
  transition: transform 0.22s cubic-bezier(0.2, 0.9, 0.3, 1.2);
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from .modal {
  transform: scale(0.94) translateY(8px);
}
.modal-leave-to .modal {
  transform: scale(0.97);
}
</style>
