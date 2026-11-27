<script setup lang="ts">
// A drop-down with check boxes for picking several values from a list (site
// categories, IP regions), with a search box and room for values that are not
// in the list. Looks like a small select when closed; the list is placed on
// the page itself so that scrolling or clipped parents do not cut it off.
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { t } from '../i18n'
import { pinyinInitials } from '../pinyin'

export interface PickOption {
  value: string
  label: string
  /** more words the search finds it by (other languages, aliases), lower case */
  keywords?: string
}

defineOptions({ inheritAttrs: false })
const props = defineProps<{ options: PickOption[]; placeholder?: string; disabled?: boolean }>()
const model = defineModel<string[]>({ required: true })

const open = ref(false)
const query = ref('')
const trigger = ref<HTMLElement>()
const pop = ref<HTMLElement>()
const searchBox = ref<HTMLInputElement>()
const place = ref({ left: 0, top: 0, width: 260, up: false })

const labelOf = (v: string) => props.options.find((o) => o.value === v)?.label ?? v

const summary = computed(() => {
  const v = model.value.filter(Boolean)
  if (!v.length) return ''
  const names = v.map(labelOf)
  return names.length > 2 ? t('{0} 等 {1} 项', names.slice(0, 2).join('、'), names.length) : names.join('、')
})

// values that are not in the list come first, so they can be unticked too
const rows = computed(() => {
  const own = model.value.filter((v) => v && !props.options.some((o) => o.value === v)).map((v) => ({ value: v, label: v }))
  const all = [...own, ...props.options]
  const q = query.value.trim().toLowerCase()
  if (!q) return all
  // Latin letters also find Chinese names by their pinyin initials (日本 → rb)
  // the closest matches first: the code or the start of the name (or of its
  // initials), then anywhere in the name, then other words for it
  const latin = /^[a-z]+$/.test(q)
  const score = (o: PickOption) => {
    const label = o.label.toLowerCase()
    if (o.value.toLowerCase() === q || label.startsWith(q) || (latin && pinyinInitials(o.label).startsWith(q))) return 0
    if (label.includes(q) || o.value.toLowerCase().includes(q)) return 1
    if (o.keywords?.split(' ').some((w) => w.startsWith(q))) return 2
    // inside a word only for longer text: "us" would find Russia and Australia
    return q.length > 3 && o.keywords?.includes(q) ? 3 : -1
  }
  return all
    .map((o, i) => ({ o, i, s: score(o) }))
    .filter((x) => x.s >= 0)
    .sort((a, b) => a.s - b.s || a.i - b.i)
    .map((x) => x.o)
})
// what was typed can be added as a value of its own
const extra = computed(() => {
  const q = query.value.trim().toLowerCase()
  // only when nothing in the list matches: "rb" is 日本, not a value of its own
  if (!q || /[\s,，]/.test(q) || rows.value.length) return ''
  return [...props.options.map((o) => o.value), ...model.value].includes(q) ? '' : q
})

function toggle(v: string) {
  model.value = model.value.includes(v) ? model.value.filter((x) => x !== v) : [...model.value.filter(Boolean), v]
}
function add() {
  if (!extra.value) return
  toggle(extra.value)
  query.value = ''
}

function measure() {
  const r = trigger.value?.getBoundingClientRect()
  if (!r) return
  const width = Math.max(r.width, 260)
  const below = window.innerHeight - r.bottom
  const up = below < 320 && r.top > below
  place.value = { left: Math.min(r.left, window.innerWidth - width - 8), top: up ? r.top - 6 : r.bottom + 6, width, up }
}
function onDoc(e: MouseEvent) {
  const n = e.target as Node
  if (!pop.value?.contains(n) && !trigger.value?.contains(n)) close()
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}
function onScroll(e: Event) {
  if (!pop.value?.contains(e.target as Node)) close()
}
// while open: a click elsewhere, Esc, scrolling or resizing closes the list
function listen(on: boolean) {
  if (on) {
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    document.addEventListener('scroll', onScroll, true)
    window.addEventListener('resize', close)
  } else {
    document.removeEventListener('mousedown', onDoc)
    document.removeEventListener('keydown', onKey)
    document.removeEventListener('scroll', onScroll, true)
    window.removeEventListener('resize', close)
  }
}
async function show() {
  if (props.disabled) return
  if (open.value) return close()
  measure()
  open.value = true
  query.value = ''
  listen(true)
  await nextTick()
  searchBox.value?.focus()
}
function close() {
  open.value = false
  listen(false)
}
onBeforeUnmount(() => listen(false))
</script>

<template>
  <button ref="trigger" v-bind="$attrs" type="button" class="select pick" :class="{ open, blank: !summary }" :disabled="disabled" :title="summary" @click="show">
    <span class="pick-text">{{ summary || placeholder || t('请选择') }}</span>
  </button>
  <Teleport to="body">
    <Transition name="nb-pop">
      <div
        v-if="open"
        ref="pop"
        class="pick-pop"
        :class="{ up: place.up }"
        :style="{ left: place.left + 'px', top: place.top + 'px', width: place.width + 'px' }"
      >
        <input
          ref="searchBox"
          v-model="query"
          class="input mini"
          :placeholder="t('搜索或输入名称')"
          spellcheck="false"
          @keydown.enter.prevent="add"
        />
        <div class="pick-list">
          <label v-for="o in rows" :key="o.value" class="pick-row" :class="{ on: model.includes(o.value) }">
            <input type="checkbox" :checked="model.includes(o.value)" @change="toggle(o.value)" />
            <span class="name">{{ o.label }}</span>
            <span v-if="o.label !== o.value" class="code mono">{{ o.value }}</span>
          </label>
          <button v-if="extra" type="button" class="pick-row add" @click="add">
            <span class="plus">+</span>
            <span class="name">{{ t('添加「{0}」', extra) }}</span>
          </button>
          <div v-if="!rows.length && !extra" class="none">{{ t('没有找到') }}</div>
        </div>
        <div class="pick-foot">
          <span>{{ t('已选 {0} 项', model.filter(Boolean).length) }}</span>
          <span class="spacer" />
          <button v-if="model.length" type="button" class="btn ghost small" @click="model = []">{{ t('清空') }}</button>
          <button type="button" class="btn small primary" @click="close">{{ t('完成') }}</button>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.pick {
  flex: 1;
  display: flex;
  align-items: center;
  height: 28px;
  min-width: 0;
  padding-top: 0;
  padding-bottom: 0;
  font-size: 12px;
  line-height: 1;
  text-align: left;
  cursor: pointer;
}
/* centred in the box, cut with … when long */
.pick-text {
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.3;
}
.pick.blank {
  color: var(--text-3);
}
.pick.open {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft-2);
}
.pick-pop {
  position: fixed;
  z-index: 200;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  background: var(--card);
  border: 1px solid var(--card-border);
  border-radius: 10px;
  box-shadow: 0 12px 32px rgba(10, 30, 70, 0.2);
}
.pick-pop.up {
  transform: translateY(-100%);
}
.input.mini {
  height: 28px;
  font-size: 12px;
}
.pick-list {
  max-height: 240px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}
.pick-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 30px;
  padding: 0 8px;
  border: none;
  background: transparent;
  border-radius: 6px;
  font: inherit;
  font-size: 12.5px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}
.pick-row:hover {
  background: var(--hover);
}
.pick-row.on .name {
  font-weight: 700;
}
.pick-row input {
  accent-color: var(--accent);
  margin: 0;
}
.name {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.code {
  font-size: 10.5px;
  color: var(--text-3);
}
.add {
  color: var(--accent);
}
.plus {
  width: 13px;
  text-align: center;
  font-weight: 700;
}
.none {
  padding: 10px 8px;
  font-size: 12px;
  color: var(--text-3);
}
.pick-foot {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 6px;
  border-top: 1px solid var(--line);
  font-size: 11.5px;
  color: var(--text-3);
}
.spacer {
  flex: 1;
}
</style>
