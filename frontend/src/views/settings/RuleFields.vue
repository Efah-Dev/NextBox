<script setup lang="ts">
// The fields of one routing rule (what it matches, where it goes, on / off,
// delete), shared by both looks of the routing rules page. It renders four
// cells for the parent's grid. A rule can have a name of its own (shown on
// the connections page); a rule set shows its name and contents and opens
// its editor.
import { ref } from 'vue'
import { t } from '../../i18n'
import Icon from '../../components/Icon.vue'
import Switch from '../../components/Switch.vue'
import MultiPick from '../../components/MultiPick.vue'
import ProcessPicker from './ProcessPicker.vue'
import RuleSetEditor from './RuleSetEditor.vue'
import type { Rule } from '../../api'
import { androidKind, geoipOptions, geosites, itemsSummary, ruleKinds, targetClass, targetOptions } from '../../policies'
import { promptDialog } from '../../util'

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
const listed = () => (props.rule.kind === 'geosite' ? geosites : props.rule.kind === 'geoip' ? geoipOptions() : null)
const placeholder = () => [...ruleKinds, androidKind].find((k) => k.value === props.rule.kind)?.placeholder || (props.rule.kind === 'geosite' ? 'netflix' : 'jp')
const android = () => props.rule.kind === 'package'
const isSet = () => props.rule.kind === 'set'

// programs, picked from the running ones
const picking = ref(false)
function addPrograms(names: string[]) {
  props.rule.values = [...props.rule.values, ...names.filter((n) => !props.rule.values.includes(n))]
}

// the rule set editor
const editing = ref(false)
function saveSet(r: Rule) {
  Object.assign(props.rule, { name: r.name, target: r.target, items: r.items })
}

async function rename() {
  const v = await promptDialog(t('规则名称'), [{ label: t('名称'), value: props.rule.name ?? '', placeholder: t('例如：公司 VPN') }], t('连接页会用这个名称显示命中这条规则的连接；留空则按内容显示。'))
  if (v === null) return
  const name = v[0].trim()
  if (name) props.rule.name = name
  else delete props.rule.name
}
</script>

<template>
  <div v-if="isSet()" class="what set" :title="note" @click="editing = true">
    <span class="set-tag"><Icon name="layers" :size="12" /> {{ t('规则集') }}</span>
    <b class="ellipsis">{{ rule.name }}</b>
    <span class="faint small ellipsis">{{ itemsSummary(rule.items) || t('还没有内容') }}</span>
    <button class="btn ghost small icon" :title="t('编辑规则集')" @click.stop="editing = true"><Icon name="edit" :size="13" /></button>
  </div>
  <div v-else class="what" :title="note">
    <button v-if="rule.name" class="name-chip ellipsis" :title="t('规则名称，点击修改')" @click="rename">{{ rule.name }}</button>
    <select :value="rule.kind" class="select mini kind" :disabled="android()" @change="setKind(($event.target as HTMLSelectElement).value as Rule['kind'])">
      <option v-for="k in ruleKinds" :key="k.value" :value="k.value">{{ k.label }}</option>
      <option v-if="android()" :value="androidKind.value">{{ androidKind.label }}</option>
    </select>
    <MultiPick v-if="listed()" v-model="rule.values" class="value" :options="listed()!" :placeholder="rule.kind === 'geosite' ? t('选择网站分类') : t('选择地区')" />
    <input
      v-else
      class="input mini value mono"
      :value="rule.values.join(', ')"
      :placeholder="placeholder()"
      :disabled="android()"
      spellcheck="false"
      @change="setValues(($event.target as HTMLInputElement).value)"
    />
    <button v-if="rule.kind === 'process'" class="btn ghost small icon" :title="t('从正在运行的程序里选择')" @click="picking = true"><Icon name="search" :size="13" /></button>
    <button v-if="!rule.name && !android()" class="btn ghost small icon" :title="t('起个名称（连接页会显示）')" @click="rename"><Icon name="edit" :size="13" /></button>
  </div>
  <select v-model="rule.target" class="select mini target" :class="targetClass(rule.target)" :disabled="android()">
    <option v-for="o in targetOptions()" :key="o.value" :value="o.value">{{ o.label }}</option>
  </select>
  <Switch v-if="!android()" :model-value="!rule.disabled" :title="rule.disabled ? t('已停用') : t('启用中')" @update:model-value="rule.disabled = !$event" />
  <span v-else class="android-tag" :title="t('安卓应用，只在 Android 版生效')">Android</span>
  <button class="btn ghost small icon danger" :title="t('删除')" @click="$emit('remove')"><Icon name="trash" :size="13" /></button>

  <ProcessPicker v-if="rule.kind === 'process'" v-model="picking" :have="rule.values" @pick="addPrograms" />
  <RuleSetEditor v-if="isSet()" v-model="editing" :rule="rule" @save="saveSet" />
</template>

<style scoped>
.what {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.what.set {
  cursor: pointer;
}
.set-tag {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  flex-shrink: 0;
  font-size: 10.5px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 999px;
  color: var(--accent);
  background: var(--accent-soft);
}
.what.set b {
  font-size: 12.5px;
  max-width: 40%;
}
.what.set .faint {
  flex: 1;
  min-width: 0;
}
.small {
  font-size: 11.5px;
}
.name-chip {
  flex-shrink: 0;
  max-width: 110px;
  height: 24px;
  padding: 0 8px;
  border: 1px solid var(--accent-line);
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--text);
  font: inherit;
  font-size: 11.5px;
  font-weight: 700;
  cursor: pointer;
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
