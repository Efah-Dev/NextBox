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
  if (!ok) return resolve?.(null)
  if (dialog.choices) return resolve?.([dialog.choice ?? '', dialog.remember ? '1' : ''])
  resolve?.((dialog.inputs ?? []).map((i) => i.value))
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
    <div v-if="dialog.choices" class="choices" role="radiogroup">
      <label v-for="c in dialog.choices" :key="c.value" class="choice" :class="{ on: dialog.choice === c.value }">
        <input v-model="dialog.choice" type="radio" :value="c.value" />
        <span class="choice-text">
          <b>{{ c.label }}</b>
          <span v-if="c.hint" class="faint">{{ c.hint }}</span>
        </span>
      </label>
    </div>
    <label v-if="dialog.remember !== undefined" class="remember">
      <input v-model="dialog.remember" type="checkbox" />
      <span>{{ dialog.rememberText }}</span>
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
.choices {
  display: grid;
  gap: 8px;
  margin: 4px 0 12px;
}
.choice {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  gap: 8px;
  align-items: start;
  padding: 8px 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  cursor: pointer;
}
.choice.on {
  border-color: var(--accent-line);
  background: var(--accent-soft);
}
.choice input {
  margin: 3px 0 0;
  accent-color: var(--accent);
}
.choice-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}
.choice-text b {
  font-size: 13px;
}
.remember {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-2);
  cursor: pointer;
}
.remember input {
  accent-color: var(--accent);
}
.warning {
  font-size: 12px;
  line-height: 1.5;
  color: var(--mid);
}
</style>
