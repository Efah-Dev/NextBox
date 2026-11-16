<script setup lang="ts">
// The icon of a service of the catalog: its logo (or first letter) on a tile
// of the brand colour, like an app icon.
import { computed } from 'vue'
import type { Service } from '../services'

const props = withDefaults(defineProps<{ service: Service; size?: number }>(), { size: 30 })

// dark marks on light brand colours (yellow, light green)
const ink = computed(() => {
  const h = props.service.color
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16) / 255)
  return 0.2126 * r + 0.7152 * g + 0.0722 * b > 0.6 ? '#1a1a1a' : '#ffffff'
})
// near black tiles get an outline so that they stand out in the dark theme
const dark = computed(() => {
  const h = props.service.color
  return [1, 3, 5].every((i) => parseInt(h.slice(i, i + 2), 16) < 0x40)
})
</script>

<template>
  <span class="svc-icon" :class="{ outline: dark }" :style="{ width: size + 'px', height: size + 'px', background: service.color, color: ink }">
    <svg v-if="service.glyph" viewBox="0 0 24 24" :width="size * 0.58" :height="size * 0.58" aria-hidden="true">
      <path :d="service.glyph" fill="currentColor" />
    </svg>
    <span v-else class="letter" :style="{ fontSize: size * 0.46 + 'px' }">{{ service.letter }}</span>
  </span>
</template>

<style scoped>
.svc-icon {
  display: inline-grid;
  place-items: center;
  flex-shrink: 0;
  border-radius: 28%;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
}
:global(.dark) .svc-icon.outline {
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.16);
}
.letter {
  font-weight: 800;
  line-height: 1;
}
</style>
