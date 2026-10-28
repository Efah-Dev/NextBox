<script setup lang="ts">
import { t } from '../i18n'
import { computed } from 'vue'
import Modal from './Modal.vue'
import { dialog } from '../util'

const open = computed({
  get: () => dialog.open,
  set: (v) => {
    if (!v) finish(false)
  },
})

function finish(ok: boolean) {
  const resolve = dialog.resolve
  dialog.open = false
  dialog.resolve = undefined
  resolve?.(ok ? (dialog.inputs ?? []).map((i) => i.value) : null)
}
</script>

<template>
  <Modal v-model="open" :title="dialog.title" :width="420">
    <p v-if="dialog.message" class="msg">{{ dialog.message }}</p>
    <label v-for="(f, i) in dialog.inputs" :key="i" class="field">
      <span class="muted">{{ f.label }}</span>
      <input
        v-model="f.value"
        class="input"
        :class="{ warned: f.warn?.(f.value) }"
        :placeholder="f.placeholder"
        @keydown.enter="finish(true)"
      />
      <span v-if="f.warn?.(f.value)" class="warning">⚠ {{ f.warn(f.value) }}</span>
    </label>
    <template #footer>
      <button class="btn" @click="finish(false)">{{ t('取消') }}</button>
      <button class="btn" :class="dialog.danger ? 'danger solid' : 'primary'" @click="finish(true)">
        {{ dialog.confirmText ?? t('确定') }}
      </button>
    </template>
  </Modal>
</template>

<style scoped>
.msg {
  margin: 4px 0 8px;
  line-height: 1.7;
  color: var(--text-2);
  white-space: pre-wrap;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 10px 0;
}
.input.warned {
  color: var(--mid);
  border-color: color-mix(in srgb, var(--mid) 55%, var(--card));
}
.input.warned:focus {
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--mid) 22%, transparent);
}
.warning {
  font-size: 12px;
  line-height: 1.5;
  color: var(--mid);
}
</style>
