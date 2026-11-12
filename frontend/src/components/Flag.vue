<script setup lang="ts">
import { flagEmoji } from '../country'

// A country flag (emoji, drawn by the bundled flag font), or a faint globe
// when the country is not known.
withDefaults(defineProps<{ code?: string; title?: string; size?: number }>(), { code: '', size: 16 })
</script>

<template>
  <span class="flag" :class="{ unknown: !code }" :title="title" :style="{ fontSize: size + 'px' }">
    <template v-if="code">{{ flagEmoji(code) }}</template>
    <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3c3 3.5 3 14.5 0 18M12 3c-3 3.5-3 14.5 0 18" />
    </svg>
  </span>
</template>

<style scoped>
.flag {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.3em;
  height: 1em;
  line-height: 1;
  font-family: 'Twemoji Country Flags', sans-serif;
  font-weight: 400;
  /* the flags in the font sit a little high */
  transform: translateY(0.04em);
}
.flag.unknown {
  color: var(--text-3);
  opacity: 0.6;
}
.flag svg {
  width: 0.95em;
  height: 0.95em;
}
</style>
