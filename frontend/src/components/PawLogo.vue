<script setup lang="ts">
// Same geometry as internal/icon (Go), which renders the app and tray icons:
// an upright paw tilted 45° clockwise, filled with a vertical pink gradient.
withDefaults(defineProps<{ size?: number }>(), { size: 24 })
const id = `paw-${Math.random().toString(36).slice(2, 8)}`
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 100 100" aria-hidden="true">
    <defs>
      <linearGradient :id="id + '-g'" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="#FFA3C8" />
        <stop offset="1" stop-color="#FF5C9A" />
      </linearGradient>
      <mask :id="id + '-m'">
        <g fill="#fff" transform="translate(50 50) scale(0.94) rotate(45) translate(-50 -50)">
          <circle cx="50" cy="70" r="19" />
          <circle cx="38.5" cy="64" r="14.5" />
          <circle cx="61.5" cy="64" r="14.5" />
          <ellipse cx="50" cy="67.5" rx="25" ry="17" />
          <ellipse cx="20" cy="40.5" rx="8.5" ry="11.5" transform="rotate(-28 20 40.5)" />
          <ellipse cx="37.5" cy="23" rx="9.2" ry="12.5" transform="rotate(-10 37.5 23)" />
          <ellipse cx="62.5" cy="23" rx="9.2" ry="12.5" transform="rotate(10 62.5 23)" />
          <ellipse cx="80" cy="40.5" rx="8.5" ry="11.5" transform="rotate(28 80 40.5)" />
        </g>
      </mask>
    </defs>
    <rect width="100" height="100" :fill="`url(#${id}-g)`" :mask="`url(#${id}-m)`" />
  </svg>
</template>
