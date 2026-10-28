<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ up: number[]; down: number[] }>()

const W = 300
const H = 80
const N = 60

const max = computed(() => Math.max(1024, ...props.up, ...props.down) * 1.15)

function path(values: number[], close: boolean) {
  const pts = values.slice(-N)
  if (pts.length < 2) return ''
  const step = W / (N - 1)
  const offset = (N - pts.length) * step
  const xy = pts.map((v, i) => [offset + i * step, H - (v / max.value) * H] as const)
  let d = `M${xy[0][0].toFixed(1)},${xy[0][1].toFixed(1)}`
  for (let i = 1; i < xy.length; i++) {
    const [x0, y0] = xy[i - 1]
    const [x1, y1] = xy[i]
    const cx = (x0 + x1) / 2
    d += ` C${cx.toFixed(1)},${y0.toFixed(1)} ${cx.toFixed(1)},${y1.toFixed(1)} ${x1.toFixed(1)},${y1.toFixed(1)}`
  }
  if (close) d += ` L${W},${H} L${offset.toFixed(1)},${H} Z`
  return d
}
</script>

<template>
  <svg class="spark" :viewBox="`0 0 ${W} ${H}`" preserveAspectRatio="none" aria-hidden="true">
    <defs>
      <linearGradient id="sp-down" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="var(--accent)" stop-opacity="0.28" />
        <stop offset="1" stop-color="var(--accent)" stop-opacity="0" />
      </linearGradient>
    </defs>
    <line v-for="i in 3" :key="i" x1="0" :x2="W" :y1="(H / 4) * i" :y2="(H / 4) * i" class="grid" />
    <path :d="path(down, true)" fill="url(#sp-down)" />
    <path :d="path(down, false)" class="line down" />
    <path :d="path(up, false)" class="line up" />
  </svg>
</template>

<style scoped>
.spark {
  width: 100%;
  height: 90px;
  display: block;
}
.grid {
  stroke: var(--line);
  stroke-dasharray: 3 4;
  vector-effect: non-scaling-stroke;
}
.line {
  fill: none;
  stroke-width: 2;
  vector-effect: non-scaling-stroke;
  stroke-linejoin: round;
}
.line.down {
  stroke: var(--accent);
}
.line.up {
  stroke: #22b07d;
  stroke-dasharray: 5 3;
}
</style>
