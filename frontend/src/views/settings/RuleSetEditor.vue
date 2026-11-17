<script setup lang="ts">
// 规则集: a named rule of its own that matches by many things at once —
// domains, IP ranges, programs, site categories, IP regions and sing-box
// rule-sets at an address — and sends all of it to one target. Each kind is
// one row; a row takes several values. "批量添加" reads pasted lines
// ("kind:value", or bare domains, IPs, .exe names and addresses).
import { computed, reactive, ref, watch } from 'vue'
import { t } from '../../i18n'
import Modal from '../../components/Modal.vue'
import Icon from '../../components/Icon.vue'
import MultiPick from '../../components/MultiPick.vue'
import ProcessPicker from './ProcessPicker.vue'
import { api, type ItemKind, type Rule, type RuleItem, type Target } from '../../api'
import { addToItems, geoipOptions, geosites, itemKinds, parseItem, targetClass, targetOptions } from '../../policies'
import Menu from '../../components/Menu.vue'
import { toast, type MenuItem } from '../../util'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ rule?: Rule }>()
const emit = defineEmits<{ save: [rule: Rule] }>()

interface Row {
  kind: ItemKind
  values: string[]
  text: string // what the input shows, for the kinds typed in
}
const draft = reactive({ name: '', target: 'proxy' as Target, rows: [] as Row[] })
const bulkOpen = ref(false)
const bulk = ref('')
const bulkErrors = ref<string[]>([])
const bulkExample = [t('每行一个，例如：'), 'google.com', '1.2.3.0/24', 'chrome.exe', 'geosite:netflix', 'https://example.com/rules.srs'].join('\n')

const listed = (k: ItemKind) => (k === 'geosite' ? geosites : k === 'geoip' ? geoipOptions() : null)
const join = (k: ItemKind, v: string[]) => v.join(k === 'url' ? '\n' : ', ')
const split = (k: ItemKind, text: string) =>
  k === 'regex' ? text.split('\n').map((v) => v.trim()).filter(Boolean) : text.split(k === 'url' ? /[\s,，]+/ : /[,，\s]+/).map((v) => v.trim()).filter(Boolean)
const toRow = (it: RuleItem): Row => ({ kind: it.kind, values: [...it.values], text: join(it.kind, it.values) })

// one row per kind, all of them shown from the start (empty ones are left
// out when saving), in this order: domains, categories, IPs, programs, URLs
const kindOrder: ItemKind[] = ['suffix', 'domain', 'keyword', 'regex', 'geosite', 'cidr', 'geoip', 'process', 'url']
const kindLabel = (k: ItemKind) => itemKinds.find((x) => x.value === k)?.label ?? k
/** All kinds as rows, filled with items. */
function allRows(items: RuleItem[]): Row[] {
  return kindOrder.map((kind) => toRow({ kind, values: items.filter((it) => it.kind === kind).flatMap((it) => it.values) }))
}

watch(open, (v) => {
  if (!v) return
  const r = props.rule
  draft.name = r?.name ?? ''
  draft.target = r?.target || 'proxy'
  draft.rows = allRows(r?.items ?? [])
  bulkOpen.value = false
  bulk.value = ''
  bulkErrors.value = []
  checks.value = {}
})

// 添加: brings back a kind whose row was removed
const addItems = computed<MenuItem[]>(() =>
  kindOrder
    .filter((k) => !draft.rows.some((r) => r.kind === k))
    .map((k) => ({ label: kindLabel(k), action: () => addRow(k) })),
)
function addRow(kind: ItemKind) {
  const row: Row = { kind, values: [], text: '' }
  const at = draft.rows.findIndex((r) => kindOrder.indexOf(r.kind) > kindOrder.indexOf(kind))
  if (at < 0) draft.rows.push(row)
  else draft.rows.splice(at, 0, row)
}
function setText(row: Row, text: string) {
  row.text = text
  row.values = split(row.kind, text)
}
const placeholder = (k: ItemKind) => itemKinds.find((x) => x.value === k)?.placeholder || ''

// ---- programs
const pickerFor = ref<Row | null>(null)
const pickerOpen = computed({ get: () => !!pickerFor.value, set: (v) => !v && (pickerFor.value = null) })
function addPrograms(names: string[]) {
  const row = pickerFor.value
  if (!row) return
  setText(row, join(row.kind, [...row.values, ...names.filter((n) => !row.values.includes(n))]))
}

// ---- rule-sets at an address: downloaded right away to see that they work
const checks = ref<Record<string, 'checking' | 'ok' | string>>({})
async function checkURLs(row: Row) {
  for (const u of row.values) {
    checks.value[u] = 'checking'
    try {
      await api.checkRuleSetURL(u)
      checks.value[u] = 'ok'
    } catch (e) {
      checks.value[u] = (e as Error).message
    }
  }
}

// ---- 批量添加
function applyBulk() {
  const errors: string[] = []
  const items: RuleItem[] = draft.rows.map((r) => ({ kind: r.kind, values: [...r.values] }))
  bulk.value.split('\n').forEach((line, i) => {
    line = line.trim()
    if (!line || line.startsWith('#')) return
    const it = parseItem(line)
    if (!it || !it.value) return errors.push(t('第 {0} 行：不认识“{1}”', i + 1, line))
    addToItems(items, it.kind, [it.value])
  })
  bulkErrors.value = errors
  if (errors.length) return
  draft.rows = allRows(items)
  bulk.value = ''
  bulkOpen.value = false
}

const count = computed(() => draft.rows.reduce((n, r) => n + r.values.length, 0))
const canSave = computed(() => draft.name.trim() !== '' && count.value > 0)

function save() {
  const items: RuleItem[] = []
  for (const r of draft.rows) if (r.values.length) addToItems(items, r.kind, r.values)
  emit('save', {
    kind: 'set',
    values: [],
    name: draft.name.trim(),
    target: draft.target,
    items,
    ...(props.rule?.disabled ? { disabled: true } : {}),
  })
  open.value = false
}
const checkFailed = (u: string) => {
  const c = checks.value[u]
  return c && c !== 'ok' && c !== 'checking' ? c : ''
}
function showCheck(u: string) {
  const c = checkFailed(u)
  if (c) toast(c, 'error')
}
</script>

<template>
  <Modal v-model="open" :title="rule ? t('编辑规则集') : t('新建规则集')" :width="640">
    <p class="faint tip">{{ t('把域名、IP、程序、网站分类等放进同一个规则集，它们命中后都走同一个去向。连接页会显示命中的规则集名称。') }}</p>
    <div class="head">
      <label class="field">
        <span>{{ t('名称') }}</span>
        <input v-model="draft.name" class="input" :placeholder="t('例如：公司内网、游戏加速')" maxlength="40" />
      </label>
      <label class="field to">
        <span>{{ t('去向') }}</span>
        <select v-model="draft.target" class="select" :class="targetClass(draft.target)">
          <option v-for="o in targetOptions()" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </label>
    </div>

    <div class="rows-head">
      <span class="block-title">{{ t('内容') }}</span>
      <span class="faint small">{{ t('任意一项命中即可；多个值用英文或中文逗号、空格分隔') }}</span>
      <span class="spacer" />
      <button class="btn ghost small" @click="bulkOpen = !bulkOpen"><Icon name="edit" :size="13" /> {{ t('批量添加') }}</button>
      <Menu v-if="addItems.length" class="add-menu" :items="addItems" icon="plus" :label="t('添加')" />
    </div>

    <div v-if="bulkOpen" class="bulk">
      <textarea v-model="bulk" class="textarea mono" spellcheck="false" :placeholder="bulkExample" />
      <div v-for="e in bulkErrors" :key="e" class="err"><Icon name="warn" :size="12" /> {{ e }}</div>
      <div class="bulk-foot">
        <span class="faint small">{{ t('不写类型时：网址是规则集网址，IP 是 IP 段，.exe 是程序，其余当作域名') }}</span>
        <button class="btn small primary" :disabled="!bulk.trim()" @click="applyBulk">{{ t('加入') }}</button>
      </div>
    </div>

    <div class="rows">
      <div v-for="(row, i) in draft.rows" :key="row.kind" class="row" :class="{ tall: row.kind === 'url' }">
        <span class="kind" :class="{ filled: row.values.length }">{{ kindLabel(row.kind) }}</span>
        <MultiPick v-if="listed(row.kind)" v-model="row.values" class="value" :options="listed(row.kind)!" :placeholder="row.kind === 'geosite' ? t('选择网站分类') : t('选择地区')" />
        <div v-else-if="row.kind === 'url'" class="value urls">
          <textarea :value="row.text" class="textarea mono mini" rows="2" :placeholder="placeholder(row.kind)" spellcheck="false" @change="setText(row, ($event.target as HTMLTextAreaElement).value)" />
          <div v-if="row.values.length" class="url-checks">
            <span v-for="u in row.values" :key="u" class="url-check" :class="checks[u] === 'ok' ? 'ok' : checkFailed(u) ? 'bad' : ''" :title="checkFailed(u) || u" @click="showCheck(u)">
              <Icon :name="checks[u] === 'ok' ? 'check' : checkFailed(u) ? 'warn' : 'link'" :size="11" />
              <span class="ellipsis mono">{{ u }}</span>
            </span>
          </div>
        </div>
        <input
          v-else
          class="input mini value mono"
          :value="row.text"
          :placeholder="placeholder(row.kind)"
          spellcheck="false"
          @change="setText(row, ($event.target as HTMLInputElement).value)"
        />
        <button v-if="row.kind === 'process'" class="btn small" :title="t('从正在运行的程序里选择')" @click="pickerFor = row"><Icon name="search" :size="13" /> {{ t('选择') }}</button>
        <button
          v-else-if="row.kind === 'url'"
          class="btn small"
          :disabled="!row.values.length || row.values.some((u) => checks[u] === 'checking')"
          :title="t('现在下载一次，确认能用')"
          @click="checkURLs(row)"
        >
          {{ row.values.some((u) => checks[u] === 'checking') ? t('检查中…') : t('检查') }}
        </button>
        <span v-else />
        <button class="btn ghost small icon danger" :title="t('删除')" @click="draft.rows.splice(i, 1)"><Icon name="trash" :size="13" /></button>
      </div>
      <div v-if="!draft.rows.length" class="faint none">{{ t('还没有内容，点「添加」或「批量添加」') }}</div>
    </div>
    <p class="faint small foot-note">
      {{ t('规则集网址只支持 sing-box 的 .srs 或 .json 规则集，Clash 规则不兼容。连接时下载，「更新规则集」时重新下载。') }}
    </p>

    <template #footer>
      <span class="faint small count">{{ t('共 {0} 项', count) }}</span>
      <button class="btn" @click="open = false">{{ t('取消') }}</button>
      <button class="btn primary" :disabled="!canSave" :title="canSave ? '' : t('需要名称和至少一项内容')" @click="save">{{ t('保存') }}</button>
    </template>
  </Modal>
  <ProcessPicker v-model="pickerOpen" :have="pickerFor?.values" @pick="addPrograms" />
</template>

<style scoped>
.tip {
  margin: 0 0 12px;
  font-size: 12px;
  line-height: 1.6;
}
.head {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 200px;
  gap: 10px;
  margin-bottom: 14px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-2);
}
.field .input,
.field .select {
  width: 100%;
}
.rows-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.block-title {
  font-size: 13px;
  font-weight: 700;
}
.rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.row {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 6px;
}
.row.tall {
  align-items: start;
}
.row.tall .kind {
  margin-top: 4px;
}
.kind {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-3);
  white-space: nowrap;
}
.kind.filled {
  color: var(--text);
}
.row > :deep(.pick) {
  width: 100%;
}
.add-menu :deep(> .btn) {
  height: 28px;
  padding: 0 10px;
  font-size: 12px;
}
.select.mini,
.input.mini {
  height: 28px;
  font-size: 12px;
  padding-top: 0;
  padding-bottom: 0;
}
.textarea.mini {
  min-height: 0;
  height: auto;
  font-size: 12px;
  width: 100%;
}
.value {
  min-width: 0;
}
.urls {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.url-checks {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.url-check {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--hover);
  color: var(--text-2);
  cursor: default;
}
.url-check.ok {
  color: var(--good);
}
.url-check.bad {
  color: var(--bad);
  cursor: pointer;
}
.bulk {
  margin-bottom: 10px;
}
.bulk .textarea {
  width: 100%;
  min-height: 110px;
  font-size: 12px;
}
.bulk-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}
.bulk-foot .faint {
  flex: 1;
}
.err {
  color: var(--bad);
  font-size: 12px;
  margin-top: 4px;
}
.none {
  padding: 12px;
  text-align: center;
}
.foot-note {
  margin: 12px 0 0;
  line-height: 1.6;
}
.small {
  font-size: 11.5px;
}
.count {
  margin-right: auto;
}
.spacer {
  flex: 1;
}
</style>
