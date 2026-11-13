<script setup lang="ts">
// 浅色亮度 / 深色深度: one slider per theme, with a live preview of each.
import { t } from '../../i18n'
import { computed } from 'vue'
import Icon from '../../components/Icon.vue'
import { DARK_DEFAULT, palette, paletteVars } from '../../theme'

const light = defineModel<number>('light', { required: true })
const dark = defineModel<number>('dark', { required: true })

const lightVars = computed(() => {
  const p = palette(false, light.value)
  return p ? paletteVars(p, false) : {}
})
const darkVars = computed(() => paletteVars(palette(true, dark.value === DARK_DEFAULT ? DARK_DEFAULT - 0.001 : dark.value)!, true))

// the slider shows brightness (right = brighter); the setting stores dimming
const brightness = computed({ get: () => 100 - light.value, set: (v: number) => (light.value = 100 - v) })
const lightLabel = computed(() => `${100 - Math.round(light.value * 0.25)}%`)
const darkLabel = computed(() =>
  dark.value <= 20 ? t('柔和') : dark.value < DARK_DEFAULT - 5 ? t('偏浅') : dark.value <= DARK_DEFAULT + 5 ? t('标准') : dark.value < 92 ? t('更深') : t('纯黑'),
)
</script>

<template>
  <div class="levels">
    <div class="level">
      <div class="level-head">
        <span>{{ t('浅色亮度') }}<span class="hint">{{ t('夜里嫌太亮可以调低') }}</span></span>
        <button v-if="light !== 0" class="btn ghost small" @click="light = 0">{{ t('恢复默认') }}</button>
      </div>
      <div class="slider">
        <Icon name="sun" :size="12" />
        <input v-model.number="brightness" type="range" min="0" max="100" step="1" :aria-label="t('浅色亮度')" />
        <Icon name="sun" :size="16" />
        <span class="value mono">{{ lightLabel }}</span>
      </div>
    </div>
    <div class="level">
      <div class="level-head">
        <span>{{ t('深色深度') }}<span class="hint">{{ t('往右越接近纯黑，适合 OLED 屏幕') }}</span></span>
        <button v-if="dark !== DARK_DEFAULT" class="btn ghost small" @click="dark = DARK_DEFAULT">{{ t('恢复默认') }}</button>
      </div>
      <div class="slider">
        <Icon name="moon" :size="12" />
        <input v-model.number="dark" type="range" min="0" max="100" step="1" :aria-label="t('深色深度')" />
        <Icon name="moon" :size="16" />
        <span class="value">{{ darkLabel }}</span>
      </div>
    </div>
    <div class="previews">
      <div class="pv" :style="lightVars">
        <span class="pv-label">{{ t('浅色') }}</span>
        <div class="pv-bar" />
        <div class="pv-card"><b>日本 02</b><span class="pv-acc">68 ms</span><div class="pv-sub">VLESS · Reality</div></div>
        <div class="pv-card"><b>美国 02</b><div class="pv-sub">Hysteria2</div></div>
      </div>
      <div class="pv dark-pv" :style="darkVars">
        <span class="pv-label">{{ t('深色') }}</span>
        <div class="pv-bar" />
        <div class="pv-card"><b>日本 02</b><span class="pv-acc">68 ms</span><div class="pv-sub">VLESS · Reality</div></div>
        <div class="pv-card"><b>美国 02</b><div class="pv-sub">Hysteria2</div></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.levels {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 270px;
  grid-template-rows: auto auto;
  column-gap: 20px;
  row-gap: 10px;
  padding: 10px 0;
  border-bottom: 1px dashed var(--line);
}
.level {
  grid-column: 1;
  min-width: 0;
}
.level-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 26px;
}
.hint {
  display: block;
  font-size: 11px;
  color: var(--text-3);
  margin-top: 2px;
}
.slider {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) 16px 44px;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  color: var(--text-3);
}
.slider input {
  width: 100%;
  accent-color: var(--accent);
}
.value {
  font-size: 12px;
  text-align: right;
  color: var(--text-2);
}
.previews {
  grid-column: 2;
  grid-row: 1 / span 2;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  align-self: center;
}
.pv {
  position: relative;
  padding: 26px 8px 8px;
  border-radius: 9px;
  background: var(--bg);
  color: var(--text);
  box-shadow: inset 0 0 0 1px var(--line);
  font-size: 11px;
}
.dark-pv {
  --accent-soft: color-mix(in srgb, var(--accent) 16%, var(--card));
}
.pv-label {
  position: absolute;
  top: 7px;
  left: 9px;
  font-size: 10.5px;
  font-weight: 700;
  color: var(--text-3);
}
.pv-bar {
  position: absolute;
  top: 10px;
  right: 9px;
  width: 40%;
  height: 6px;
  border-radius: 3px;
  background: var(--card);
}
.pv-card {
  padding: 6px 8px;
  border-radius: 6px;
  background: var(--card);
  box-shadow: inset 0 0 0 1px var(--line);
}
.pv-card + .pv-card {
  margin-top: 5px;
}
.pv-acc {
  float: right;
  font-size: 10px;
  font-weight: 700;
  color: var(--good);
}
.pv-sub {
  color: var(--text-3);
  font-size: 10px;
}
</style>
