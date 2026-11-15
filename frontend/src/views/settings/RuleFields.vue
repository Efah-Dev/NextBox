<script setup lang="ts">
// The fields of one routing rule (what it matches, where it goes, on / off,
// delete), shared by both looks of the routing rules page. It renders four
// cells for the parent's grid.
import { t } from '../../i18n'
import Icon from '../../components/Icon.vue'
import Switch from '../../components/Switch.vue'
import type { Rule } from '../../api'
import { androidKind, geoips, geosites, ruleKinds, targetClass, targetOptions } from '../../policies'

const props = defineProps<{ rule: Rule; note?: string }>()
defineEmits<{ remove: [] }>()

function setValues(text: string) {
  props.rule.values = props.rule.kind === 'regex' ? [text.trim()].filter(Boolean) : text.split(/[,，\s]+/).map((v) => v.trim()).filter(Boolean)
}
function setKind(kind: Rule['kind']) {
  props.rule.kind = kind
  if (kind === 'geosite') props.rule.values = ['google']
  else if (kind === 'geoip') props.rule.values = ['cn']
  else props.rule.values = []
}
const listed = () => (props.rule.kind === 'geosite' ? geosites : props.rule.kind === 'geoip' ? geoips : null)
const isListed = () => !!listed()?.some((x) => x.value === props.rule.values[0]) && props.rule.values.length === 1
const placeholder = () => [...ruleKinds, androidKind].find((k) => k.value === props.rule.kind)?.placeholder || (props.rule.kind === 'geosite' ? 'netflix' : 'jp')
const android = () => props.rule.kind === 'package'
</script>

<template>
  <div class="what" :title="note">
    <select :value="rule.kind" class="select mini kind" :disabled="android()" @change="setKind(($event.target as HTMLSelectElement).value as Rule['kind'])">
      <option v-for="k in ruleKinds" :key="k.value" :value="k.value">{{ k.label }}</option>
      <option v-if="android()" :value="androidKind.value">{{ androidKind.label }}</option>
    </select>
    <select
      v-if="listed() && (isListed() || !rule.values.length)"
      :value="rule.values[0]"
      class="select mini value"
      @change="($event.target as HTMLSelectElement).value === '*' ? (rule.values = ['']) : (rule.values = [($event.target as HTMLSelectElement).value])"
    >
      <option v-for="o in listed()!" :key="o.value" :value="o.value">{{ o.label }}</option>
      <option value="*">{{ t('其他（手动输入）…') }}</option>
    </select>
    <input
      v-else
      class="input mini value mono"
      :value="rule.values.join(', ')"
      :placeholder="placeholder()"
      :disabled="android()"
      spellcheck="false"
      @change="setValues(($event.target as HTMLInputElement).value)"
    />
  </div>
  <select v-model="rule.target" class="select mini target" :class="targetClass(rule.target)" :disabled="android()">
    <option v-for="o in targetOptions()" :key="o.value" :value="o.value">{{ o.label }}</option>
  </select>
  <Switch v-if="!android()" :model-value="!rule.disabled" :title="rule.disabled ? t('已停用') : t('启用中')" @update:model-value="rule.disabled = !$event" />
  <span v-else class="android-tag" :title="t('安卓应用，只在 Android 版生效')">Android</span>
  <button class="btn ghost small icon danger" :title="t('删除')" @click="$emit('remove')"><Icon name="trash" :size="13" /></button>
</template>

<style scoped>
.what {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.select.mini,
.input.mini {
  height: 28px;
  font-size: 12px;
  padding-top: 0;
  padding-bottom: 0;
}
.kind {
  width: 108px;
  flex-shrink: 0;
}
.value {
  flex: 1;
  min-width: 0;
}
.select.mini.target {
  width: 100%;
  min-width: 0;
}
.to-direct {
  color: var(--good);
}
.to-block {
  color: var(--bad);
}
.to-proxy {
  color: var(--accent);
}
select.to-direct,
select.to-block,
select.to-proxy {
  font-weight: 700;
}
.android-tag {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-3);
  justify-self: center;
}
</style>
