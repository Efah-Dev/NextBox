<script setup lang="ts">
const props = withDefaults(defineProps<{ name: string; size?: number; stroke?: number }>(), {
  size: 16,
  stroke: 2,
})

const p = (d: string) => `<path d="${d}"/>`
const c = (cx: number, cy: number, r: number) => `<circle cx="${cx}" cy="${cy}" r="${r}"/>`

const icons: Record<string, string> = {
  home: p('M3 11.5 12 4l9 7.5') + p('M5.5 10v10h5v-6h3v6h5V10'),
  nodes: c(12, 12, 9) + p('M3 12h18') + p('M12 3c3 3.5 3 14.5 0 18') + p('M12 3c-3 3.5-3 14.5 0 18'),
  connections: p('M4 8h14l-3.5-3.5') + p('M20 16H6l3.5 3.5'),
  logs: p('M6 3h9l4 4v14H6z') + p('M14 3v5h5') + p('M9 12.5h7') + p('M9 16.5h5'),
  settings: p('M4 7h9') + p('M18 7h2') + c(15.5, 7, 2.5) + p('M4 17h3') + p('M12 17h8') + c(9.5, 17, 2.5),
  minus: p('M6 12h12'),
  close: p('M6 6l12 12') + p('M18 6 6 18'),
  maximize: '<rect x="5.5" y="5.5" width="13" height="13" rx="1.5"/>',
  plus: p('M12 5v14') + p('M5 12h14'),
  refresh: p('M20 12a8 8 0 1 1-2.34-5.66') + p('M20 4v5h-5'),
  zap: p('M13 3 5 14h6l-1 7 8-11h-6z'),
  trash: p('M4 7h16') + p('M10 11v6') + p('M14 11v6') + p('M6 7l1 13h10l1-13') + p('M9 7V4h6v3'),
  edit: p('M4 20h4L19 9l-4-4L4 16z') + p('M13.5 6.5l4 4'),
  link: p('M10 14a4 4 0 0 0 5.66 0l3-3a4 4 0 0 0-5.66-5.66l-1 1') + p('M14 10a4 4 0 0 0-5.66 0l-3 3a4 4 0 0 0 5.66 5.66l1-1'),
  clipboard: '<rect x="8.5" y="3" width="7" height="4" rx="1"/>' + p('M8.5 5H6v16h12V5h-2.5'),
  search: c(11, 11, 6.5) + p('M20 20l-4.3-4.3'),
  power: p('M12 3v8') + p('M7 6.3a7.5 7.5 0 1 0 10 0'),
  shield: p('M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z'),
  sort: p('M7 4v16') + p('M4 17l3 3 3-3') + p('M17 20V4') + p('M14 7l3-3 3 3'),
  folder: p('M3 6.5A1.5 1.5 0 0 1 4.5 5H9l2 2h8.5A1.5 1.5 0 0 1 21 8.5v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5z'),
  download: p('M12 4v11') + p('M7 10l5 5 5-5') + p('M5 20h14'),
  check: p('M5 12.5l4.5 4.5L19 7'),
  chevron: p('M9 6l6 6-6 6'),
  up: p('M12 19V5') + p('M6 11l6-6 6 6'),
  down: p('M12 5v14') + p('M6 13l6 6 6-6'),
  rss: p('M5 5a14 14 0 0 1 14 14') + p('M5 11a8 8 0 0 1 8 8') + c(6, 18, 1.2),
  more: p('M5 12h.01') + p('M12 12h.01') + p('M19 12h.01'),
  copy: '<rect x="8" y="8" width="12" height="12" rx="1.5"/>' + p('M16 8V5.5A1.5 1.5 0 0 0 14.5 4h-9A1.5 1.5 0 0 0 4 5.5v9A1.5 1.5 0 0 0 5.5 16H8'),
  info: c(12, 12, 9) + p('M12 11v6') + p('M12 7.5h.01'),
  warn: p('M12 4 2.5 20h19z') + p('M12 10v4.5') + p('M12 17.5h.01'),
  route: c(6, 6, 2.5) + c(18, 18, 2.5) + p('M8.5 6H15a3 3 0 0 1 0 6H9a3 3 0 0 0 0 6h6.5'),
  dns: '<rect x="3.5" y="4" width="17" height="6" rx="1.5"/><rect x="3.5" y="14" width="17" height="6" rx="1.5"/>' + p('M7 7h.01') + p('M7 17h.01'),
  tun: p('M4 12a8 8 0 0 1 16 0') + p('M7.5 12a4.5 4.5 0 0 1 9 0') + c(12, 12, 1.2) + p('M12 13.5V20'),
  gauge: p('M4.5 17a8.5 8.5 0 1 1 15 0') + p('M12 13l4-4.5'),
  palette: p('M12 3a9 9 0 1 0 0 18c1.5 0 2-1 2-2s-1-1.5-1-2.5 1-1.5 2-1.5h2a4 4 0 0 0 4-4c0-4.5-4-8-9-8z') + c(7.5, 11, 1) + c(10, 7, 1) + c(15, 7.5, 1),
  inbound: p('M3 12h11') + p('M10 8l4 4-4 4') + p('M15 4h4.5A1.5 1.5 0 0 1 21 5.5v13a1.5 1.5 0 0 1-1.5 1.5H15'),
  admin: p('M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z') + p('M9 12l2 2 4-4'),
}
</script>

<template>
  <svg
    class="icon"
    :width="props.size"
    :height="props.size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="props.stroke"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    v-html="icons[props.name] ?? ''"
  />
</template>

<style scoped>
.icon {
  flex-shrink: 0;
  display: block;
}
</style>
