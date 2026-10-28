<script setup lang="ts">
import { computed } from 'vue'

const model = defineModel<string>({ required: true })
const props = defineProps<{ options: { value: string; label: string }[] }>()

const index = computed(() => Math.max(0, props.options.findIndex((o) => o.value === model.value)))
</script>

<template>
  <div class="seg" :style="{ '--n': options.length, '--idx': index }">
    <span class="thumb" />
    <button
      v-for="o in options"
      :key="o.value"
      type="button"
      class="seg-btn"
      :class="{ active: o.value === model }"
      @click="model = o.value"
    >
      {{ o.label }}
    </button>
  </div>
</template>

<style scoped>
.seg {
  position: relative;
  display: grid;
  grid-template-columns: repeat(var(--n), 1fr);
  background: var(--hover);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 3px;
}
.thumb {
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 3px;
  width: calc((100% - 6px) / var(--n));
  transform: translateX(calc(var(--idx) * 100%));
  background: var(--card);
  border-radius: 6px;
  box-shadow: 0 1px 4px rgba(17, 52, 110, 0.12);
  transition: transform 0.25s cubic-bezier(0.3, 1.2, 0.5, 1);
}
.seg-btn {
  position: relative;
  height: 28px;
  padding: 0 10px;
  white-space: nowrap;
  border: none;
  background: transparent;
  font: inherit;
  color: var(--text-2);
  cursor: pointer;
  transition: color 0.2s;
}
.seg-btn.active {
  color: var(--accent);
  font-weight: 700;
}
</style>
