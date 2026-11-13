<script setup lang="ts">
// 主题色: preset swatches, recent custom colours and a colour panel of our
// own (saturation / brightness square, hue bar, HEX), instead of the
// system colour dialog that looked out of place.
import { t } from '../../i18n'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '../../components/Icon.vue'

const color = defineModel<string>({ required: true })
const recent = defineModel<string[]>('recent', { default: () => [] })

const presets = ['#F472B6', '#1370F3', '#0EA5E9', '#10B981', '#8B5CF6', '#F97316', '#E5484D', '#475569']
const isPreset = (c: string) => presets.some((p) => p.toLowerCase() === c.toLowerCase())

const open = ref(false)
const root = ref<HTMLElement>()

// ---- colour maths ----
type HSV = [number, number, number]
function hsvToRgb([h, s, v]: HSV): [number, number, number] {
  const f = (n: number) => {
    const k = (n + h / 60) % 6
    return v - v * s * Math.max(0, Math.min(k, 4 - k, 1))
  }
  return [f(5), f(3), f(1)].map((x) => Math.round(x * 255)) as [number, number, number]
}
function rgbToHsv(r: number, g: number, b: number): HSV {
  r /= 255
  g /= 255
  b /= 255
  const mx = Math.max(r, g, b)
  const d = mx - Math.min(r, g, b)
  let h = 0
  if (d) h = mx === r ? ((g - b) / d) % 6 : mx === g ? (b - r) / d + 2 : (r - g) / d + 4
  return [(h * 60 + 360) % 360, mx ? d / mx : 0, mx]
}
const toHex = (c: number[]) => '#' + c.map((x) => x.toString(16).padStart(2, '0')).join('').toUpperCase()
function parseHex(v: string): HSV | null {
  const m = /^#?([0-9a-f]{6})$/i.exec(v.trim())
  if (!m) return null
  const n = m[1]
  return rgbToHsv(parseInt(n.slice(0, 2), 16), parseInt(n.slice(2, 4), 16), parseInt(n.slice(4, 6), 16))
}
function luminance(c: number[]) {
  const a = c.map((x) => {
    x /= 255
    return x <= 0.03928 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4)
  })
  return 0.2126 * a[0] + 0.7152 * a[1] + 0.0722 * a[2]
}

const hsv = ref<HSV>(parseHex(color.value) ?? [330, 0.53, 0.96])
const hexText = ref(color.value.toUpperCase())
const current = computed(() => toHex(hsvToRgb(hsv.value)))
// white text on the colour (buttons, badges) needs a contrast of about 3
const contrast = computed(() => 1.05 / (luminance(hsvToRgb(hsv.value)) + 0.05))

watch(color, (c) => {
  if (c.toLowerCase() !== current.value.toLowerCase()) {
    hsv.value = parseHex(c) ?? hsv.value
    hexText.value = c.toUpperCase()
  }
})

function pick(c: string) {
  hsv.value = parseHex(c) ?? hsv.value
  hexText.value = c.toUpperCase()
  color.value = c
}

// while the panel is open the colour follows live; it is remembered as a
// recent colour when the panel closes
function update() {
  hexText.value = current.value
  color.value = current.value
}

function onHex() {
  const v = parseHex(hexText.value)
  if (v) {
    hsv.value = v
    color.value = current.value
  }
}

function darken() {
  const [h, s] = hsv.value
  let v = hsv.value[2]
  while (v > 0.2 && 1.05 / (luminance(hsvToRgb([h, Math.max(s, 0.55), v])) + 0.05) < 3.2) v -= 0.02
  hsv.value = [h, Math.max(s, 0.55), v]
  update()
}

function drag(el: HTMLElement, fn: (x: number, y: number) => void) {
  return (e: PointerEvent) => {
    el.setPointerCapture(e.pointerId)
    const move = (ev: PointerEvent) => {
      const r = el.getBoundingClientRect()
      fn(Math.min(1, Math.max(0, (ev.clientX - r.left) / r.width)), Math.min(1, Math.max(0, (ev.clientY - r.top) / r.height)))
      update()
    }
    move(e)
    el.onpointermove = move
    el.onpointerup = () => (el.onpointermove = null)
  }
}
const sv = ref<HTMLElement>()
const hue = ref<HTMLElement>()
const onSV = (e: PointerEvent) => drag(sv.value!, (x, y) => (hsv.value = [hsv.value[0], x, 1 - y]))(e)
const onHue = (e: PointerEvent) => drag(hue.value!, (x) => (hsv.value = [x * 359.9, hsv.value[1], hsv.value[2]]))(e)

function svKey(e: KeyboardEvent) {
  const step = { ArrowLeft: [-0.03, 0], ArrowRight: [0.03, 0], ArrowUp: [0, 0.03], ArrowDown: [0, -0.03] }[e.key]
  if (!step) return
  e.preventDefault()
  const [h, s, v] = hsv.value
  hsv.value = [h, Math.min(1, Math.max(0, s + step[0])), Math.min(1, Math.max(0, v + step[1]))]
  update()
}
function hueKey(e: KeyboardEvent) {
  const step = { ArrowLeft: -5, ArrowRight: 5 }[e.key]
  if (!step) return
  e.preventDefault()
  hsv.value = [(hsv.value[0] + step + 360) % 360, hsv.value[1], hsv.value[2]]
  update()
}

function close() {
  if (!open.value) return
  open.value = false
  const c = color.value.toUpperCase()
  if (!isPreset(c)) recent.value = [c, ...(recent.value ?? []).filter((x) => x.toUpperCase() !== c)].slice(0, 4)
}
function onDocDown(e: MouseEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) close()
}
watch(open, (o) => (o ? document.addEventListener('mousedown', onDocDown) : document.removeEventListener('mousedown', onDocDown)))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocDown))
</script>

<template>
  <div ref="root" class="picker-root" :class="{ open }">
    <div class="swatches">
      <button
        v-for="c in presets"
        :key="c"
        class="swatch"
        :class="{ on: color.toLowerCase() === c.toLowerCase() }"
        :style="{ background: c }"
        :title="c"
        @click="pick(c)"
      />
      <template v-if="recent?.length">
        <span class="sep" />
        <button
          v-for="c in recent"
          :key="'r' + c"
          class="swatch"
          :class="{ on: color.toLowerCase() === c.toLowerCase() }"
          :style="{ background: c }"
          :title="t('最近使用') + ' ' + c"
          @click="pick(c)"
        />
      </template>
      <button class="custom" :class="{ on: open || (!isPreset(color) && !recent?.some((r) => r.toLowerCase() === color.toLowerCase())) }" :title="t('自定义颜色')" @click="open ? close() : (open = true)">
        <span class="wheel" />
        {{ t('自定义') }}
      </button>
    </div>

    <Transition name="drop">
      <div v-if="open" class="panel">
        <div class="pad">
          <div
            ref="sv"
            class="sv"
            :style="{ '--hue': `hsl(${hsv[0]} 100% 50%)` }"
            tabindex="0"
            role="slider"
            :aria-label="t('饱和度与亮度')"
            @pointerdown="onSV"
            @keydown="svKey"
          >
            <span class="knob" :style="{ left: hsv[1] * 100 + '%', top: (1 - hsv[2]) * 100 + '%', background: current }" />
          </div>
          <div ref="hue" class="hue" tabindex="0" role="slider" :aria-label="t('色相')" @pointerdown="onHue" @keydown="hueKey">
            <span class="knob" :style="{ left: (hsv[0] / 360) * 100 + '%', background: `hsl(${hsv[0]} 100% 50%)` }" />
          </div>
        </div>
        <div class="pick-side">
          <div class="hex-row">
            <span class="cur" :style="{ background: current }" />
            <input v-model="hexText" class="input mono" maxlength="7" spellcheck="false" :aria-label="t('HEX 颜色值')" @input="onHex" @keydown.enter="close" />
          </div>
          <div class="preview" :style="{ '--accent': current }">
            <button class="btn primary small" tabindex="-1">{{ t('连 接').replace(/\s+/g, '') }}</button>
            <span class="badge">VLESS</span>
            <span class="mini-switch" />
          </div>
          <div v-if="contrast < 3" class="contrast bad">
            <span>{{ t('颜色偏浅，按钮上的白字会看不清') }}</span>
            <button class="btn small" @click="darken">{{ t('自动加深') }}</button>
          </div>
          <div v-else class="contrast"><Icon name="check" :size="12" :stroke="2.6" /> {{ t('文字清晰可读') }}</div>
          <button class="btn small done" @click="close">{{ t('完成') }}</button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.picker-root {
  position: relative;
  flex: 1;
  min-width: 0;
}
.swatches {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.swatch {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 2px solid var(--card);
  box-shadow: 0 0 0 1px var(--line);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
}
.swatch:hover {
  transform: scale(1.12);
}
.swatch.on {
  box-shadow: 0 0 0 2px var(--accent);
}
.sep {
  width: 1px;
  height: 18px;
  background: var(--line);
  margin: 0 2px;
}
.custom {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 26px;
  padding: 0 10px 0 4px;
  border-radius: 13px;
  border: 1px solid var(--input-border);
  background: var(--card);
  color: var(--text-2);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.custom:hover,
.custom.on {
  border-color: var(--accent);
  color: var(--accent);
}
.wheel {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: conic-gradient(#f00, #ff0, #0f0, #0ff, #00f, #f0f, #f00);
  box-shadow: inset 0 0 0 2px var(--card);
}
.panel {
  position: absolute;
  top: calc(100% + 10px);
  left: 0;
  z-index: 20;
  width: 480px;
  max-width: calc(100vw - 80px);
  display: grid;
  grid-template-columns: minmax(0, 1fr) 176px;
  gap: 14px;
  padding: 14px;
  border-radius: 10px;
  background: var(--card);
  border: 1px solid var(--line);
  box-shadow: 0 14px 40px rgba(10, 30, 70, 0.2);
}
.sv {
  position: relative;
  height: 150px;
  border-radius: 8px;
  cursor: crosshair;
  touch-action: none;
  background: linear-gradient(to top, #000, transparent), linear-gradient(to right, #fff, transparent), var(--hue);
}
.hue {
  position: relative;
  height: 12px;
  margin-top: 12px;
  border-radius: 999px;
  cursor: pointer;
  touch-action: none;
  background: linear-gradient(to right, #f00, #ff0, #0f0, #0ff, #00f, #f0f, #f00);
}
.knob {
  position: absolute;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid #fff;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.25), 0 1px 4px rgba(0, 0, 0, 0.3);
  transform: translate(-50%, -50%);
  pointer-events: none;
}
.hue .knob {
  top: 50%;
}
.sv:focus-visible,
.hue:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.pick-side {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.hex-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.cur {
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  border-radius: 8px;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.1);
}
.hex-row .input {
  width: 100%;
  min-width: 0;
  text-transform: uppercase;
}
.preview {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px;
  border-radius: 8px;
  background: var(--hover);
  --accent-soft: color-mix(in srgb, var(--accent) 12%, var(--card));
}
.mini-switch {
  width: 30px;
  height: 17px;
  border-radius: 999px;
  background: var(--accent);
  position: relative;
  margin-left: auto;
}
.mini-switch::after {
  content: '';
  position: absolute;
  top: 2px;
  right: 2px;
  width: 13px;
  height: 13px;
  border-radius: 50%;
  background: #fff;
}
.contrast {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  font-size: 11.5px;
  color: var(--good);
}
.contrast.bad {
  color: var(--mid);
}
.done {
  margin-top: auto;
  justify-content: center;
}
.drop-enter-active,
.drop-leave-active {
  transition: opacity 0.18s, transform 0.18s;
}
.drop-enter-from,
.drop-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
