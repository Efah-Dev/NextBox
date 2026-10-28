<script setup lang="ts">
import { t } from '../i18n'
import { ref, watch } from 'vue'
import Icon from './Icon.vue'
import Segmented from './Segmented.vue'

// The three rule lists are stored as text (one rule per line, see
// internal/config parseRuleLines); this component edits them as rows.
const proxy = defineModel<string>('proxy', { required: true })
const direct = defineModel<string>('direct', { required: true })
const block = defineModel<string>('block', { required: true })

type Action = 'proxy' | 'direct' | 'block'
type Kind = 'suffix' | 'domain' | 'keyword' | 'regex' | 'geosite' | 'geoip' | 'cidr' | 'process'
interface Row {
  id: number
  action: Action
  kind: Kind
  value: string
}

const actions: { value: Action; label: string }[] = [
  { value: 'proxy', label: t('走代理') },
  { value: 'direct', label: t('直连') },
  { value: 'block', label: t('拦截') },
]

const kinds: { value: Kind; label: string; placeholder: string }[] = [
  { value: 'geosite', label: t('网站分类'), placeholder: '' },
  { value: 'suffix', label: t('域名（含子域名）'), placeholder: 'google.com' },
  { value: 'domain', label: t('完整域名'), placeholder: 'www.google.com' },
  { value: 'keyword', label: t('域名关键词'), placeholder: 'google' },
  { value: 'geoip', label: t('IP 归属地'), placeholder: '' },
  { value: 'cidr', label: t('IP / IP 段'), placeholder: t('8.8.8.8 或 1.2.3.0/24') },
  { value: 'process', label: t('程序名'), placeholder: 'chrome.exe' },
  { value: 'regex', label: t('域名正则'), placeholder: '^ads?\\.' },
]

const geosites = [
  { value: 'google', label: 'Google' },
  { value: 'youtube', label: 'YouTube' },
  { value: 'openai', label: 'OpenAI / ChatGPT' },
  { value: 'telegram', label: 'Telegram' },
  { value: 'github', label: 'GitHub' },
  { value: 'netflix', label: 'Netflix' },
  { value: 'disney', label: 'Disney+' },
  { value: 'spotify', label: 'Spotify' },
  { value: 'twitter', label: 'Twitter / X' },
  { value: 'facebook', label: 'Facebook' },
  { value: 'instagram', label: 'Instagram' },
  { value: 'tiktok', label: 'TikTok' },
  { value: 'microsoft', label: 'Microsoft' },
  { value: 'apple', label: 'Apple' },
  { value: 'steam', label: 'Steam' },
  { value: 'bilibili', label: t('哔哩哔哩') },
  { value: 'cn', label: t('中国大陆网站') },
  { value: 'geolocation-!cn', label: t('境外网站') },
  { value: 'category-ads-all', label: t('广告域名') },
]

const geoips = [
  { value: 'cn', label: t('中国大陆') },
  { value: 'hk', label: t('中国香港') },
  { value: 'tw', label: t('中国台湾') },
  { value: 'jp', label: t('日本') },
  { value: 'sg', label: t('新加坡') },
  { value: 'kr', label: t('韩国') },
  { value: 'us', label: t('美国') },
]

const presets: { label: string; rules: Omit<Row, 'id'>[] }[] = [
  { label: t('AI 服务走代理'), rules: [{ action: 'proxy', kind: 'geosite', value: 'openai' }] },
  { label: t('Google 走代理'), rules: [{ action: 'proxy', kind: 'geosite', value: 'google' }] },
  { label: t('Telegram 走代理'), rules: [{ action: 'proxy', kind: 'geosite', value: 'telegram' }] },
  { label: t('流媒体走代理'), rules: ['netflix', 'disney', 'youtube'].map((v) => ({ action: 'proxy' as Action, kind: 'geosite' as Kind, value: v })) },
  { label: t('哔哩哔哩直连'), rules: [{ action: 'direct', kind: 'geosite', value: 'bilibili' }] },
  { label: t('Steam 直连'), rules: [{ action: 'direct', kind: 'geosite', value: 'steam' }] },
]

const prefixes: Record<string, Kind> = {
  domain: 'domain', full: 'domain', suffix: 'suffix', domain_suffix: 'suffix', keyword: 'keyword',
  regex: 'regex', regexp: 'regex', geosite: 'geosite', geoip: 'geoip', ip: 'cidr', ip_cidr: 'cidr', process: 'process',
}
const ipLike = /^(\d{1,3}(\.\d{1,3}){3}|[0-9a-f]*:[0-9a-f:]+)(\/\d{1,3})?$/i

let seq = 0
function parse(text: string, action: Action): Row[] {
  const rows: Row[] = []
  for (let line of text.split('\n')) {
    line = line.trim()
    if (!line || line.startsWith('#')) continue
    const i = line.indexOf(':')
    const kind = i > 0 ? prefixes[line.slice(0, i).toLowerCase()] : undefined
    if (kind) rows.push({ id: ++seq, action, kind, value: line.slice(i + 1).trim() })
    else rows.push({ id: ++seq, action, kind: ipLike.test(line) ? 'cidr' : 'suffix', value: line })
  }
  return rows
}

function serialize(list: Row[], action: Action): string {
  return list
    .filter((r) => r.action === action && r.value.trim())
    .map((r) => {
      const v = r.value.trim()
      return r.kind === 'suffix' || r.kind === 'cidr' ? v : `${r.kind}:${v}`
    })
    .join('\n')
}

const rows = ref<Row[]>([])
const mode = ref<'visual' | 'text'>('visual')

function load() {
  rows.value = [...parse(proxy.value, 'proxy'), ...parse(direct.value, 'direct'), ...parse(block.value, 'block')]
}
load()

// rows -> text models
watch(
  rows,
  (list) => {
    const p = serialize(list, 'proxy')
    const d = serialize(list, 'direct')
    const b = serialize(list, 'block')
    if (p !== proxy.value) proxy.value = p
    if (d !== direct.value) direct.value = d
    if (b !== block.value) block.value = b
  },
  { deep: true },
)

// text models -> rows (text mode edits, or settings reloaded from the backend)
watch([proxy, direct, block], () => {
  const list = rows.value
  if (serialize(list, 'proxy') !== proxy.value || serialize(list, 'direct') !== direct.value || serialize(list, 'block') !== block.value) {
    load()
  }
})

function addRow() {
  rows.value.push({ id: ++seq, action: 'proxy', kind: 'geosite', value: 'google' })
}

function removeRow(id: number) {
  rows.value = rows.value.filter((r) => r.id !== id)
}

function changeKind(r: Row, kind: Kind) {
  r.kind = kind
  r.value = kind === 'geosite' ? 'google' : kind === 'geoip' ? 'cn' : ''
}

function applyPreset(p: (typeof presets)[number]) {
  for (const rule of p.rules) {
    const exists = rows.value.some((r) => r.kind === rule.kind && r.value === rule.value)
    if (!exists) rows.value.push({ id: ++seq, ...rule })
  }
}
</script>

<template>
  <div class="rule-editor">
    <div class="head">
      <Segmented
        v-model="mode"
        style="width: 200px"
        :options="[
          { value: 'visual', label: t('可视化') },
          { value: 'text', label: t('文本') },
        ]"
      />
      <span class="faint hint">{{ t('规则从上到下匹配：拦截 → 走代理 → 直连，未命中时按路由模式处理') }}</span>
    </div>

    <template v-if="mode === 'visual'">
      <div v-if="rows.length" class="rows">
        <div v-for="r in rows" :key="r.id" class="rule-row">
          <select v-model="r.action" class="select action" :class="r.action">
            <option v-for="a in actions" :key="a.value" :value="a.value">{{ a.label }}</option>
          </select>
          <select class="select" :value="r.kind" @change="changeKind(r, ($event.target as HTMLSelectElement).value as Kind)">
            <option v-for="k in kinds" :key="k.value" :value="k.value">{{ k.label }}</option>
          </select>
          <select v-if="r.kind === 'geosite'" v-model="r.value" class="select">
            <option v-for="g in geosites" :key="g.value" :value="g.value">{{ g.label }}</option>
            <option v-if="!geosites.some((g) => g.value === r.value)" :value="r.value">{{ r.value }}</option>
          </select>
          <select v-else-if="r.kind === 'geoip'" v-model="r.value" class="select">
            <option v-for="g in geoips" :key="g.value" :value="g.value">{{ g.label }}</option>
            <option v-if="!geoips.some((g) => g.value === r.value)" :value="r.value">{{ r.value }}</option>
          </select>
          <input
            v-else
            v-model.lazy="r.value"
            class="input mono"
            :placeholder="kinds.find((k) => k.value === r.kind)?.placeholder"
          />
          <button class="btn ghost small icon danger" :title="t('删除')" @click="removeRow(r.id)">
            <Icon name="trash" :size="14" />
          </button>
        </div>
      </div>
      <div v-else class="empty-rules faint">{{ t('还没有自定义规则，点击下面的按钮添加，或选择一个常用预设。') }}</div>

      <div class="footer">
        <button class="btn" @click="addRow"><Icon name="plus" /> {{ t('添加规则') }}</button>
        <span class="faint">{{ t('常用：') }}</span>
        <button v-for="p in presets" :key="p.label" class="chip" @click="applyPreset(p)">{{ p.label }}</button>
      </div>
    </template>

    <div v-else class="text-mode">
      <label><span class="muted">{{ t('走代理') }}</span><textarea v-model.lazy="proxy" class="textarea" placeholder="geosite:google&#10;example.com" /></label>
      <label><span class="muted">{{ t('直连') }}</span><textarea v-model.lazy="direct" class="textarea" placeholder="geoip:cn&#10;domain:www.example.cn" /></label>
      <label><span class="muted">{{ t('拦截') }}</span><textarea v-model.lazy="block" class="textarea" placeholder="keyword:ads" /></label>
      <p class="faint syntax">
        {{ t('每行一条：example.com（含子域名）· domain:完整域名 · keyword:关键词 · regex:正则 · geosite:分类 · geoip:国家代码 · 1.2.3.0/24 · process:程序名.exe') }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.rule-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.hint {
  font-size: 12px;
}
.rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.rule-row {
  display: grid;
  grid-template-columns: 92px 150px minmax(0, 1fr) 26px;
  gap: 8px;
  align-items: center;
  animation: card-in 0.25s both;
}
.action {
  font-weight: 700;
}
.action.proxy {
  color: var(--accent);
  background-color: var(--accent-soft);
  border-color: var(--accent-line);
}
.action.direct {
  color: var(--good);
  background-color: color-mix(in srgb, var(--good) 10%, var(--card));
  border-color: color-mix(in srgb, var(--good) 35%, var(--card));
}
.action.block {
  color: var(--bad);
  background-color: color-mix(in srgb, var(--bad) 10%, var(--card));
  border-color: color-mix(in srgb, var(--bad) 35%, var(--card));
}
.empty-rules {
  padding: 14px;
  border: 1px dashed var(--line);
  border-radius: var(--radius-sm);
  text-align: center;
}
.footer {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.chip {
  height: 26px;
  padding: 0 10px;
  border-radius: 13px;
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--text-2);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}
.chip:hover {
  border-color: var(--accent-line);
  background: var(--accent-soft);
  color: var(--accent);
}
.text-mode {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.text-mode label {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.syntax {
  margin: 0;
  font-size: 12px;
  line-height: 1.7;
}
</style>
