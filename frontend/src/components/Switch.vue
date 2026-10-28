<script setup lang="ts">
const model = defineModel<boolean>({ default: false })
defineProps<{ disabled?: boolean }>()
</script>

<template>
  <button
    type="button"
    class="switch"
    :class="{ on: model }"
    :disabled="disabled"
    role="switch"
    :aria-checked="model"
    @click="model = !model"
  >
    <span class="knob" />
  </button>
</template>

<style scoped>
.switch {
  width: 38px;
  height: 22px;
  flex-shrink: 0;
  border-radius: 11px;
  border: none;
  padding: 0;
  background: var(--input-border);
  position: relative;
  cursor: pointer;
  transition: background 0.2s;
}
.switch.on {
  background: var(--accent);
}
.switch:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.knob {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  transition: transform 0.22s cubic-bezier(0.3, 1.4, 0.5, 1);
}
.switch.on .knob {
  transform: translateX(16px);
}
</style>
