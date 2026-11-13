<script setup lang="ts">
// 分流规则: the routing mode; in rule mode the ready-made scenes, the user's
// own rules (each reads like a sentence: 访问 … 走 …) and a tester that says
// where an address would go.
import { t } from '../../i18n'
import { computed, onMounted, ref, watch } from 'vue'
import Card from '../../components/Card.vue'
import Icon from '../../components/Icon.vue'
import Switch from '../../components/Switch.vue'
import Segmented from '../../components/Segmented.vue'
import { api, type Rule, type RouteMatch, type Settings } from '../../api'
import { moveItem, useDragSort } from '../../drag'
import {
  geoips, geosites, policies, refreshPolicies, ruleKinds, rulesToText, targetClass, targetLabel, targetOptions, textToRules,
} from '../../policies'
import { run } from '../../util'

const props = defineProps<{ form: Settings; index: number }>()
const form = props.form

const mirrors = [
  { label: t('本地内置（绕过中国域名和 IP）'), value: 'local' },
  { label: 'jsDelivr（Cloudflare）', value: 'https://testingcf.jsdelivr.net/gh/SagerNet/sing-{kind}@rule-set/{name}.srs' },
  { label: 'jsDelivr', value: 'https://cdn.jsdelivr.net/gh/SagerNet/sing-{kind}@rule-set/{name}.srs' },
  { label: 'GitHub Raw', value: 'https://raw.githubusercontent.com/SagerNet/sing-{kind}/rule-set/{name}.srs' },
]

const scenes = computed(() => [
  { id: 'cn', label: t('绕过中国大陆'), desc: t('国内网站和 IP 直连') },
  { id: 'ai', label: t('AI 服务'), desc: 'ChatGPT · Claude · Gemini' },
  { id: 'media', label: t('流媒体'), desc: 'YouTube · Netflix · Disney+' },
  { id: 'games', label: t('游戏下载直连'), desc: t('Steam 等国内下载节点') },
])
const sceneOf = (id: string) => form.scenes.find((s) => s.id === id)

onMounted(refreshPolicies)

// ---- rules ----
const listEl = ref<HTMLElement>()
const sorter = useDragSort({
  container: listEl,
  item: '.rule',
  count: () => form.rules.length,
  enabled: () => form.rules.length > 1,
  ignore: 'button, input, select',
  onDrop: (from, to) => moveItem(form.rules, from, to),
})

function addRule() {
  form.rules.push({ kind: 'geosite', values: ['google'], target: 'proxy' })
}
function removeRule(i: number) {
  form.rules.splice(i, 1)
}

const valuesText = (r: Rule) => r.values.join(', ')
function setValues(r: Rule, text: string) {
  r.values = r.kind === 'regex' ? [text.trim()].filter(Boolean) : text.split(/[,，\s]+/).map((v) => v.trim()).filter(Boolean)
}
function setKind(r: Rule, kind: Rule['kind']) {
  r.kind = kind
  if (kind === 'geosite') r.values = ['google']
  else if (kind === 'geoip') r.values = ['cn']
  else r.values = []
}
const listed = (r: Rule) => (r.kind === 'geosite' ? geosites : r.kind === 'geoip' ? geoips : null)
const isListed = (r: Rule) => !!listed(r)?.some((x) => x.value === r.values[0]) && r.values.length === 1
const placeholder = (r: Rule) => ruleKinds.find((k) => k.value === r.kind)?.placeholder ?? ''

// ---- text mode ----
const textMode = ref(false)
const text = ref('')
const textErrors = ref<string[]>([])
function openText() {
  text.value = rulesToText(form.rules)
  textErrors.value = []
  textMode.value = true
}
watch(text, (v) => {
  if (!textMode.value) return
  const { rules, errors } = textToRules(v)
  textErrors.value = errors
  if (!errors.length) form.rules = rules
})

// ---- tester ----
const probe = ref('')
const match = ref<RouteMatch | null>(null)
const testing = ref(false)
async function test() {
  if (!probe.value.trim()) return
  testing.value = true
  match.value = (await run(() => api.testRoute(probe.value))) ?? null
  testing.value = false
}
watch(() => [form.rules, form.scenes, form.routeMode], () => (match.value = null), { deep: true })

const verdict = computed(() => {
  const m = match.value
  if (!m) return ''
  const where = m.target === 'final' ? t('当前节点') : targetLabel(m.target)
  switch (m.source) {
    case 'ads':
      return t('命中「屏蔽广告」 → 拦截')
    case 'private':
      return t('局域网 / 本机地址 → 直连')
    case 'rule':
      return t('命中第 {0} 条规则 → {1}', m.index + 1, where)
    case 'scene':
      return t('命中常用场景「{0}」 → {1}', scenes.value.find((s) => s.id === m.scene)?.label ?? m.scene, where)
  }
  return t('没有命中任何规则 → {0}', where)
})
</script>

<template>
  <Card id="sec-route" :title="t('分流规则')" :index="index">
    <div class="form-row">
      <div class="label">{{ t('路由模式') }}</div>
      <div class="control">
        <Segmented
          v-model="form.routeMode"
          style="width: 300px"
          :options="[
            { value: 'global', label: t('全局代理') },
            { value: 'rule', label: t('规则分流') },
            { value: 'direct', label: t('全部直连') },
          ]"
        />
      </div>
    </div>
    <div class="form-row">
      <div class="label">{{ t('屏蔽广告') }}<span class="hint">{{ t('拦截广告域名，所有模式都生效') }}</span></div>
      <div class="control"><Switch v-model="form.blockAds" /></div>
    </div>

    <div v-if="form.routeMode !== 'rule'" class="collapsed">
      <Icon name="route" :size="16" />
      <span>{{ form.routeMode === 'global' ? t('全局代理：所有流量走当前节点，不需要配置，规则不生效。') : t('全部直连：不使用代理，规则不生效。') }}</span>
      <button class="btn small" @click="form.routeMode = 'rule'">{{ t('切换到规则分流') }}</button>
    </div>

    <template v-else>
      <div class="block">
        <div class="block-head">
          <span class="block-title">{{ t('常用场景') }}</span>
          <span class="faint small">{{ t('打开即生效，排在我的规则之后') }}</span>
        </div>
        <div class="scenes">
          <div v-for="sc in scenes" :key="sc.id" class="scene" :class="{ on: sceneOf(sc.id)?.enabled, hit: match?.source === 'scene' && match.scene === sc.id }">
            <div class="scene-top">
              <b>{{ sc.label }}</b>
              <Switch v-if="sceneOf(sc.id)" v-model="sceneOf(sc.id)!.enabled" />
            </div>
            <div class="faint small ellipsis">{{ sc.desc }}</div>
            <div class="scene-to">
              <span class="faint">→</span>
              <span v-if="sc.id === 'cn'" class="to to-direct">{{ t('直连') }}</span>
              <select v-else-if="sceneOf(sc.id)" v-model="sceneOf(sc.id)!.target" class="select mini" :class="targetClass(sceneOf(sc.id)!.target)">
                <option v-for="o in targetOptions(false)" :key="o.value" :value="o.value">{{ o.label }}</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      <div class="block">
        <div class="block-head">
          <span class="block-title">{{ t('我的规则') }}</span>
          <span class="faint small">{{ t('从上往下匹配，拖动调整顺序') }}</span>
          <span class="spacer" />
          <button class="btn ghost small" @click="textMode ? (textMode = false) : openText()">
            <Icon :name="textMode ? 'settings' : 'edit'" :size="13" /> {{ textMode ? t('返回列表') : t('文本模式') }}
          </button>
          <button v-if="!textMode" class="btn small" @click="addRule"><Icon name="plus" :size="13" /> {{ t('添加规则') }}</button>
        </div>

        <template v-if="textMode">
          <textarea v-model="text" class="textarea mono rule-text" spellcheck="false" />
          <div v-if="textErrors.length" class="text-errors">
            <div v-for="e in textErrors" :key="e"><Icon name="warn" :size="12" /> {{ e }}</div>
          </div>
          <p v-else class="faint small">{{ t('修改会自动保存。类型：geosite、suffix（域名）、domain（完整域名）、keyword、regex、geoip、cidr（IP 段）、process（程序）。') }}</p>
        </template>

        <template v-else>
          <div ref="listEl" class="rules" :class="{ 'sort-settling': sorter.drag.settling }">
            <div
              v-for="(r, i) in form.rules"
              :key="i"
              class="rule"
              :class="[{ off: r.disabled, hit: match?.source === 'rule' && match.index === i }, sorter.classes(i)]"
              :style="sorter.style(i)"
              @pointerdown="sorter.start($event, i)"
            >
              <Icon class="grip" name="grip" :size="14" :stroke="3" />
              <span class="num mono">{{ i + 1 }}</span>
              <select :value="r.kind" class="select mini kind" @change="setKind(r, ($event.target as HTMLSelectElement).value as Rule['kind'])">
                <option v-for="k in ruleKinds" :key="k.value" :value="k.value">{{ k.label }}</option>
              </select>
              <select
                v-if="listed(r) && (isListed(r) || !r.values.length)"
                :value="r.values[0]"
                class="select mini value"
                @change="($event.target as HTMLSelectElement).value === '*' ? (r.values = ['']) : (r.values = [($event.target as HTMLSelectElement).value])"
              >
                <option v-for="o in listed(r)!" :key="o.value" :value="o.value">{{ o.label }}</option>
                <option value="*">{{ t('其他（手动输入）…') }}</option>
              </select>
              <input
                v-else
                class="input mini value mono"
                :value="valuesText(r)"
                :placeholder="placeholder(r) || (r.kind === 'geosite' ? 'netflix' : 'jp')"
                spellcheck="false"
                @change="setValues(r, ($event.target as HTMLInputElement).value)"
              />
              <span class="faint">→</span>
              <select v-model="r.target" class="select mini target" :class="targetClass(r.target)">
                <option v-for="o in targetOptions()" :key="o.value" :value="o.value">{{ o.label }}</option>
              </select>
              <Switch :model-value="!r.disabled" :title="r.disabled ? t('已停用') : t('启用中')" @update:model-value="r.disabled = !$event" />
              <button class="btn ghost small icon danger" :title="t('删除')" @click="removeRule(i)"><Icon name="trash" :size="13" /></button>
            </div>
            <div v-if="!form.rules.length" class="empty-rules faint">
              {{ t('还没有规则。常用场景已经能满足大多数需要；想让某个网站或程序单独直连、走代理或走某个分组代理，就添加一条规则。') }}
            </div>
          </div>
          <p v-if="!policies.list.length" class="faint small">
            {{ t('想让不同网站走不同节点（例如 YouTube 走日本、Google 走美国）？先在下面的「分组代理」里建好分组，再在规则里选择它。') }}
          </p>
        </template>
      </div>

      <div class="block">
        <div class="block-head"><span class="block-title">{{ t('匹配测试') }}</span><span class="faint small">{{ t('输入网址或 IP，看看会走哪里') }}</span></div>
        <form class="tester" @submit.prevent="test">
          <Icon name="search" :size="15" class="faint" />
          <input v-model="probe" class="input mono" placeholder="www.youtube.com" spellcheck="false" />
          <button class="btn" type="submit" :disabled="testing || !probe.trim()">{{ testing ? t('测试中…') : t('测试') }}</button>
        </form>
        <div v-if="match" class="verdict">
          <b class="mono">{{ match.input }}</b> · {{ verdict }}
          <div v-if="match.note" class="faint small">{{ match.note }}</div>
        </div>
      </div>

      <div class="form-row">
        <div class="label">{{ t('规则集镜像') }}<span class="hint">{{ t('网站分类和 IP 归属地从这里下载') }}</span></div>
        <div class="control">
          <select v-model="form.ruleSetMirror" class="select">
            <option v-for="m in mirrors" :key="m.value" :value="m.value">{{ m.label }}</option>
            <option v-if="!mirrors.some((m) => m.value === form.ruleSetMirror)" :value="form.ruleSetMirror">{{ t('自定义') }}</option>
          </select>
          <button class="btn" @click="run(() => api.updateRuleSets(), (m) => m)"><Icon name="download" /> {{ t('更新规则集') }}</button>
        </div>
      </div>
    </template>
  </Card>
</template>

<style scoped>
.small {
  font-size: 11.5px;
}
.collapsed {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 10px;
  padding: 12px 14px;
  border-radius: var(--radius-sm);
  background: var(--hover);
  color: var(--text-2);
}
.collapsed span {
  flex: 1 1 260px;
}
.block {
  padding: 14px 0 6px;
  border-bottom: 1px dashed var(--line);
}
.block-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.block-title {
  font-weight: 700;
}
.scenes {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(168px, 1fr));
  gap: 8px;
}
.scene {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 10px 11px;
  border-radius: var(--radius);
  border: 1px solid var(--line);
  background: var(--card);
  transition: border-color 0.15s, background 0.15s, box-shadow 0.15s;
  min-width: 0;
}
.scene.on {
  border-color: var(--accent-line);
  background: var(--accent-soft);
}
.scene.hit,
.rule.hit {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft-2);
}
.scene-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}
.scene-to {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
}
.to {
  font-size: 12px;
  font-weight: 700;
}
.select.mini,
.input.mini {
  height: 28px;
  font-size: 12px;
  padding-top: 0;
  padding-bottom: 0;
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
.scene .select.mini {
  flex: 1;
  min-width: 0;
}
.rules {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.rule {
  display: grid;
  grid-template-columns: 14px 18px 118px minmax(0, 1fr) 12px 150px auto auto;
  align-items: center;
  gap: 8px;
  padding: 6px 8px 6px 6px;
  border-radius: var(--radius);
  border: 1px solid var(--line);
  background: var(--card);
  touch-action: none;
}
.rule.off {
  opacity: 0.55;
}
.grip {
  color: var(--text-3);
  cursor: grab;
}
.num {
  font-size: 11px;
  color: var(--text-3);
  text-align: right;
}
.rule .value {
  min-width: 0;
  width: 100%;
}
.empty-rules {
  padding: 14px;
  border-radius: var(--radius);
  border: 1px dashed var(--line);
  line-height: 1.7;
}
.rule-text {
  width: 100%;
  min-height: 180px;
  font-size: 12px;
  line-height: 1.7;
}
.text-errors {
  margin-top: 6px;
  color: var(--bad);
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.tester {
  display: flex;
  align-items: center;
  gap: 8px;
}
.tester .input {
  flex: 1;
}
.verdict {
  margin-top: 8px;
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  background: var(--hover);
  line-height: 1.6;
}
</style>
