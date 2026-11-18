<script setup lang="ts">
// 分流规则: the routing mode, and in rule mode the rules, in one of three
// looks the user picks (simplest first):
// - 服务目录 (apps): well-known services with icons, each sent somewhere with
//   one pick (ServiceCatalog); a pick is an ordinary rule at the top.
// - 卡片 (cards): the look of v2.0, scene cards and a list of the user's own
//   rules. The places of ad blocking and the scenes stay where they are;
//   dragging only reorders the user's rules among themselves.
// - 路由路径 (path): the route every connection takes, from top to bottom.
//   LAN addresses come first and "everything else" last; in between, ad
//   blocking, the scenes and the user's own rules are steps that can be
//   dragged into any order. A test lights the path up to the step that
//   matches. The steps that are off can be hidden.
// All share the data (Settings.Rules with the step places), the tester and a
// reset to the defaults; cards and path also the text mode. The first time
// the section is seen, a guide helps choosing the look (RouteStyleGuide).
import { t } from '../../i18n'
import ResolveDns from './ResolveDns.vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import Card from '../../components/Card.vue'
import Icon from '../../components/Icon.vue'
import Switch from '../../components/Switch.vue'
import Segmented from '../../components/Segmented.vue'
import RuleFields from './RuleFields.vue'
import RuleSetEditor from './RuleSetEditor.vue'
import ServiceCatalog from './ServiceCatalog.vue'
import RouteStyleGuide from './RouteStyleGuide.vue'
import { api, type Rule, type RouteMatch, type RouteView, type Settings } from '../../api'
import { serviceOf } from '../../services'
import { useDragSort } from '../../drag'
import {
  defaultRoute, finalLabel as finalOf, isStep, keepLabels, refreshPolicies, ruleFilled, rulesToText, ruleTitle,
  sceneCards, sceneInfo, targetClass, targetLabel, targetOptions, textToRules,
} from '../../policies'
import { confirmDialog, run, toast } from '../../util'

const props = defineProps<{ form: Settings; index: number }>()
const form = props.form

const mirrors = [
  { label: t('本地内置（绕过中国域名和 IP）'), value: 'local' },
  { label: 'jsDelivr（Cloudflare）', value: 'https://testingcf.jsdelivr.net/gh/SagerNet/sing-{kind}@rule-set/{name}.srs' },
  { label: 'jsDelivr', value: 'https://cdn.jsdelivr.net/gh/SagerNet/sing-{kind}@rule-set/{name}.srs' },
  { label: 'GitHub Raw', value: 'https://raw.githubusercontent.com/SagerNet/sing-{kind}/rule-set/{name}.srs' },
]

const sceneOf = (id: string) => form.scenes.find((s) => s.id === id)
const ruleMode = computed(() => form.routeMode === 'rule')
// how "bypass mainland China" resolves names (see ResolveDns.vue)
const resolveLabel = computed(() => (form.bypassDns === 'direct' ? t('直连 DNS 解析') : t('接管 DNS 解析')))

const view = computed<RouteView>({
  get: () => (form.routeView === 'cards' || form.routeView === 'apps' ? form.routeView : 'path'),
  set: (v) => (form.routeView = v),
})
const hideOff = computed({ get: () => !!form.routeHideOff, set: (v: boolean) => (form.routeHideOff = v) })
// what the switch hides: ad blocking and the scenes while they are off
const stepHidden = (r: Rule) =>
  hideOff.value && ((r.kind === 'scene' && !sceneOf(r.values[0])?.enabled) || (r.kind === 'ads' && !form.blockAds))

// ---- the guide, once: when the top of the section has come up past the
// lower part of the window (a share of the card would never be reached by a
// long list of rules)
const guideOpen = ref(false)
let seen: IntersectionObserver | undefined
onMounted(() => {
  refreshPolicies()
  const el = document.getElementById('sec-route')
  if (form.routeViewAsked || !el || !('IntersectionObserver' in window)) return
  seen = new IntersectionObserver(
    (entries) => {
      if (!entries.some((e) => e.isIntersecting) || form.routeViewAsked) return
      seen?.disconnect()
      guideOpen.value = true
    },
    { rootMargin: '0px 0px -40% 0px' },
  )
  seen.observe(el)
})
onUnmounted(() => seen?.disconnect())
function guideDone(v: RouteView | null) {
  form.routeViewAsked = true
  if (v) view.value = v
}

// ---- the path: position 0 = LAN, 1..n = form.rules, n + 1 = everything else
const last = computed(() => form.rules.length + 1)

/** The steps shown on the path (i = index in form.rules). */
const pathItems = computed(() => form.rules.map((r, i) => ({ r, i })).filter((x) => !stepHidden(x.r)))
const hiddenCount = computed(() => form.rules.filter(stepHidden).length)

/** Whether a step takes part with the current settings. */
function stepActive(r: Rule) {
  if (r.kind === 'ads') return form.blockAds
  if (!ruleMode.value) return false
  if (r.kind === 'scene') return !!sceneOf(r.values[0])?.enabled
  return !r.disabled && r.kind !== 'package' && ruleFilled(r)
}
const activeAt = (pos: number) => pos === 0 || pos === last.value || stepActive(form.rules[pos - 1])

/** Why a step is skipped, shown under its name. */
function skipNote(r: Rule) {
  if (r.kind === 'ads') return form.blockAds ? '' : t('已关闭')
  if (form.routeMode === 'global') return t('全局代理下跳过')
  if (form.routeMode === 'direct') return t('全部直连下跳过')
  if (r.kind === 'package') return t('安卓应用，只在 Android 版生效')
  if (r.kind === 'scene') return sceneOf(r.values[0])?.enabled ? '' : t('已关闭')
  if (r.disabled) return t('已停用')
  if (!ruleFilled(r)) return t('还没填内容')
  return ''
}

const finalLabel = computed(() => finalOf(form.routeMode))

function stepTitle(pos: number) {
  if (pos === 0) return t('局域网和本机地址')
  if (pos === last.value) return t('其余流量')
  const r = form.rules[pos - 1]
  if (r.kind === 'ads') return t('屏蔽广告')
  if (r.kind === 'scene') return sceneInfo(r.values[0]).label
  return ruleTitle(r)
}

// the excluded ranges, shown at the entrance of the path
const excluded = computed(() =>
  (form.tunExclude ?? '')
    .split('\n')
    .map((l) => l.split('#')[0].trim())
    .filter(Boolean),
)

// ---- the cards: scenes in a fixed order, the user's own rules as a list
const cardScenes = sceneCards()
const ownItems = computed(() => form.rules.map((r, i) => ({ r, i })).filter((x) => !isStep(x.r)))
const ownCount = computed(() => ownItems.value.length)

// ---- drag to reorder: the shown items, mapped back onto form.rules
/** Moves the item shown at from to where the one shown at to is. */
function moveShown(shown: number[], from: number, to: number) {
  if (from === to) return
  const item = form.rules[shown[from]]
  const target = form.rules[shown[to]]
  const rest = form.rules.filter((_, i) => i !== shown[from])
  const at = rest.indexOf(target) + (to > from ? 1 : 0)
  rest.splice(at, 0, item)
  form.rules = rest
}
const pathEl = ref<HTMLElement>()
const pathSorter = useDragSort({
  container: pathEl,
  item: '.stn',
  count: () => pathItems.value.length,
  enabled: () => pathItems.value.length > 1,
  ignore: 'button, input, select, textarea, .switch',
  onDrop: (from, to) => moveShown(pathItems.value.map((x) => x.i), from, to),
})
const listEl = ref<HTMLElement>()
const listSorter = useDragSort({
  container: listEl,
  item: '.rule',
  count: () => ownItems.value.length,
  enabled: () => ownItems.value.length > 1,
  ignore: 'button, input, select, textarea, .switch',
  onDrop: (from, to) => moveShown(ownItems.value.map((x) => x.i), from, to),
})

// ---- rules
async function addRule() {
  // new rules go before the mainland China bypass, so that they apply
  const cn = form.rules.findIndex((r) => r.kind === 'scene' && r.values[0] === 'cn')
  const at = cn < 0 ? form.rules.length : cn
  form.rules.splice(at, 0, { kind: 'suffix', values: [], target: 'proxy' })
  await nextTick()
  const rows = view.value === 'path' ? pathEl.value?.querySelectorAll<HTMLElement>('.stn') : listEl.value?.querySelectorAll<HTMLElement>('.rule')
  const shown = view.value === 'path' ? pathItems.value : ownItems.value
  const row = rows?.[shown.findIndex((x) => x.i === at)]
  row?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  row?.querySelector<HTMLInputElement>('input.value')?.focus()
}
const removeRule = (i: number) => form.rules.splice(i, 1)

// ---- rule sets: made in their editor, then placed like a new rule
const newSetOpen = ref(false)
async function addSet(r: Rule) {
  const cn = form.rules.findIndex((x) => x.kind === 'scene' && x.values[0] === 'cn')
  const at = cn < 0 ? form.rules.length : cn
  form.rules.splice(at, 0, r)
  await nextTick()
  const rows = view.value === 'path' ? pathEl.value?.querySelectorAll<HTMLElement>('.stn') : listEl.value?.querySelectorAll<HTMLElement>('.rule')
  const shown = view.value === 'path' ? pathItems.value : ownItems.value
  rows?.[shown.findIndex((x) => x.i === at)]?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
}

async function resetRules() {
  const ok = await confirmDialog(
    t('恢复默认分流规则'),
    t('会删除你添加的所有规则，常用场景和步骤顺序恢复成刚安装时的样子（只开启「绕过中国大陆」）。路由模式、屏蔽广告开关和分组代理不受影响。'),
    true,
    t('恢复默认'),
  )
  if (!ok) return
  textMode.value = false
  const d = defaultRoute()
  form.rules = d.rules as Rule[]
  form.scenes = d.scenes
  toast(t('已恢复默认分流规则'), 'success')
}

// ---- text mode
const textMode = ref(false)
watch(view, () => (textMode.value = false))
const text = ref('')
const textErrors = ref<string[]>([])
function openText() {
  const targets = Object.fromEntries(form.scenes.map((s) => [s.id, s.target]))
  text.value = rulesToText(form.rules, targets)
  textErrors.value = []
  textMode.value = true
}
watch(text, (v) => {
  if (!textMode.value) return
  const { rules, errors, sceneTargets } = textToRules(v)
  textErrors.value = errors
  if (errors.length) return
  keepLabels(rules, form.rules)
  form.rules = rules as Rule[]
  for (const [id, target] of Object.entries(sceneTargets)) {
    const sc = sceneOf(id)
    if (sc) sc.target = target
  }
})

// ---- tester
const probe = ref('')
const match = ref<RouteMatch | null>(null)
const testing = ref(false)
const hitPos = ref(-2) // -1 = the excluded ranges
const shown = ref(-2) // the path is lit up to here
let timer = 0
const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

async function test() {
  if (!probe.value.trim()) return
  testing.value = true
  const m = (await run(() => api.testRoute(probe.value))) ?? null
  testing.value = false
  match.value = m
  clearInterval(timer)
  if (!m) return (hitPos.value = shown.value = -2)
  let pos: number
  switch (m.source) {
    case 'excluded':
      pos = -1
      break
    case 'private':
      pos = 0
      break
    case 'final':
      pos = last.value
      break
    default:
      pos = m.index >= 0 ? m.index + 1 : form.rules.findIndex((r) => r.kind === 'scene' && r.values[0] === m.scene) + 1
  }
  hitPos.value = pos
  if (reduceMotion || pos <= 0 || view.value !== 'path') {
    shown.value = pos
    return
  }
  shown.value = 0
  timer = window.setInterval(() => {
    shown.value++
    if (shown.value >= pos) clearInterval(timer)
  }, 70)
}
onUnmounted(() => clearInterval(timer))
watch(
  () => [form.rules, form.scenes, form.routeMode, form.blockAds, form.tunExclude],
  () => {
    match.value = null
    hitPos.value = shown.value = -2
    clearInterval(timer)
  },
  { deep: true },
)

function stnClass(pos: number) {
  const tested = hitPos.value > -2
  return {
    skip: !activeAt(pos),
    pass: tested && pos < shown.value && pos < hitPos.value && activeAt(pos),
    hit: tested && pos === hitPos.value && shown.value >= pos,
    after: tested && shown.value >= hitPos.value && pos > hitPos.value,
  }
}
const hitRule = (i: number) => match.value?.source === 'rule' && match.value.index === i
const hitScene = (id: string) => match.value?.source === 'scene' && match.value.scene === id

const verdict = computed(() => {
  const m = match.value
  if (!m) return ''
  const where = m.target === 'final' ? finalLabel.value : targetLabel(m.target)
  if (m.source === 'excluded') return t('在 TUN 排除网段里，不经过分流 → {0}', form.tunEnabled ? t('Windows 路由') : t('直连'))
  if (view.value !== 'path') {
    const app = m.source === 'rule' ? serviceOf(form.rules[m.index]?.app ?? '') : undefined
    switch (m.source) {
      case 'ads':
        return t('命中「屏蔽广告」 → 拦截')
      case 'private':
        return t('局域网 / 本机地址 → 直连')
      case 'rule':
        if (app) return t('命中单独指定的服务「{0}」 → {1}', app.name, where)
        if (form.rules[m.index]?.name) return t('命中「{0}」 → {1}', ruleTitle(form.rules[m.index]), where)
        return t('命中第 {0} 条规则 → {1}', ownItems.value.findIndex((x) => x.i === m.index) + 1, where)
      case 'scene':
        return t('命中常用场景「{0}」 → {1}', sceneInfo(m.scene).label, where)
    }
    return t('没有命中任何规则 → {0}', where)
  }
  if (m.source === 'final') return t('前面都没有命中，走最后一步「其余流量」 → {0}', where)
  // the number as shown, with hidden scenes left out
  const n = hitPos.value === 0 ? 1 : pathItems.value.findIndex((x) => x.i === hitPos.value - 1) + 2
  return t('在第 {0} 步「{1}」命中 → {2}', n, stepTitle(hitPos.value), where)
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
      <div class="label">{{ t('规则样式') }}<span class="hint">{{ t('三种样式改的是同一份规则') }}</span></div>
      <div class="control">
        <Segmented
          v-model="view"
          style="width: 330px"
          :options="[
            { value: 'apps', label: t('服务目录') },
            { value: 'cards', label: t('卡片') },
            { value: 'path', label: t('路由路径') },
          ]"
        />
        <button class="btn ghost small" :title="t('按熟悉程度推荐样式，带动画演示')" @click="guideOpen = true">{{ t('帮我选') }}</button>
      </div>
    </div>
    <div v-if="view !== 'path'" class="form-row">
      <div class="label">{{ t('屏蔽广告') }}<span class="hint">{{ t('拦截广告域名，所有模式都生效') }}</span></div>
      <div class="control"><Switch v-model="form.blockAds" /></div>
    </div>

    <div class="mode-note" :class="{ off: !ruleMode }">
      <Icon name="route" :size="16" />
      <span v-if="form.routeMode === 'global'">{{ t('全局代理：除了局域网和屏蔽广告，所有流量都走当前节点，下面的规则和场景暂不生效。') }}</span>
      <span v-else-if="form.routeMode === 'direct'">{{ t('全部直连：不使用代理，下面的规则和场景暂不生效。') }}</span>
      <span v-else-if="view === 'path'">{{ t('每个连接从上往下依次判断，在哪一步命中就从哪一步出去。拖动左边的把手调整先后顺序。') }}</span>
      <span v-else-if="view === 'apps'">{{ t('给常用服务单独选择走当前节点、直连还是某个分组；没有单独指定的服务，跟随常用场景和「其余流量」。') }}</span>
      <span v-else>{{ t('打开需要的常用场景，再添加自己的规则。规则从上往下匹配，拖动调整顺序；要调整场景和规则之间的先后，请切换到「路由路径」样式。') }}</span>
      <button v-if="!ruleMode" class="btn small" @click="form.routeMode = 'rule'">{{ t('切换到规则分流') }}</button>
    </div>

    <form class="tester" @submit.prevent="test">
      <Icon name="search" :size="15" class="faint" />
      <input v-model="probe" class="input mono" :placeholder="t('试一试：输入网址或 IP，例如 www.youtube.com')" spellcheck="false" />
      <button class="btn" type="submit" :disabled="testing || !probe.trim()">{{ testing ? t('测试中…') : t('测试') }}</button>
    </form>
    <div v-if="match" class="verdict">
      <b class="mono">{{ match.input }}</b> · {{ verdict }}
      <div v-if="match.resolved" class="faint small note">{{ t('按 IP 判断的步骤用直连 DNS 解析出的 {0}', match.resolved) }}</div>
      <div v-if="match.note" class="faint small note">{{ match.note }}</div>
    </div>

    <!-- ================================================================ path -->
    <template v-if="view === 'path'">
      <div class="path-head">
        <span class="block-title">{{ t('路由路径') }}</span>
        <span class="faint small">{{ t('共 {0} 步', pathItems.length + 2) }}</span>
        <span class="spacer" />
        <button class="btn ghost small" @click="textMode ? (textMode = false) : openText()">
          <Icon :name="textMode ? 'route' : 'edit'" :size="13" /> {{ textMode ? t('返回路径') : t('文本模式') }}
        </button>
        <button v-if="!textMode" class="btn small" :title="t('把域名、IP、程序等放进一个有名称的规则集，统一指定去向')" @click="newSetOpen = true"><Icon name="layers" :size="13" /> {{ t('添加规则集') }}</button>
        <button v-if="!textMode" class="btn small primary-soft" @click="addRule"><Icon name="plus" :size="13" /> {{ t('添加规则') }}</button>
      </div>

      <div v-if="!textMode" class="path">
        <div v-if="excluded.length" class="stn entry" :class="{ hit: hitPos === -1 }">
          <span class="no"><Icon name="tun" :size="12" /></span>
          <div class="box">
            <span />
            <div class="what">
              <b>{{ t('TUN 排除网段') }}</b>
              <span class="sub mono">{{ excluded.slice(0, 3).join('  ') }}{{ excluded.length > 3 ? t(' 等 {0} 个', excluded.length) : '' }}</span>
            </div>
            <span class="to to-sys">{{ form.tunEnabled ? t('Windows 路由') : t('直连') }}</span>
          </div>
        </div>

        <div class="stn fixed" :class="stnClass(0)">
          <span class="no">1</span>
          <div class="box">
            <Icon class="lock" name="lock" :size="12" :title="t('固定在第一步')" />
            <div class="what">
              <b>{{ t('局域网和本机地址') }}</b>
              <span class="sub">{{ t('192.168.x.x、10.x.x.x、127.0.0.1 等') }}</span>
            </div>
            <span class="to to-direct">{{ t('直连') }}</span>
          </div>
        </div>

        <div ref="pathEl" class="steps" :class="{ 'sort-settling': pathSorter.drag.settling }">
          <div
            v-for="(x, k) in pathItems"
            :key="x.i"
            class="stn"
            :class="[stnClass(x.i + 1), pathSorter.classes(k), { android: x.r.kind === 'package' }]"
            :style="pathSorter.style(k)"
            @pointerdown="pathSorter.start($event, k)"
          >
            <span class="no">{{ k + 2 }}</span>
            <div class="box">
              <Icon class="grip" name="grip" :size="14" :stroke="3" :title="t('拖动调整顺序')" />

              <template v-if="x.r.kind === 'ads'">
                <div class="what">
                  <b><span class="tag">{{ t('内置') }}</span>{{ t('屏蔽广告') }}</b>
                  <span class="sub">{{ skipNote(x.r) || t('常见的广告和跟踪域名') }}</span>
                </div>
                <span class="to to-block">{{ t('拦截') }}</span>
                <Switch v-model="form.blockAds" />
                <span />
              </template>

              <template v-else-if="x.r.kind === 'scene'">
                <div class="what">
                  <b><span class="tag">{{ t('场景') }}</span>{{ sceneInfo(x.r.values[0]).label }}</b>
                  <span class="sub">{{ skipNote(x.r) || sceneInfo(x.r.values[0]).desc }}<template v-if="x.r.values[0] === 'cn' && sceneOf('cn')?.enabled && !skipNote(x.r)"> · {{ resolveLabel }}</template></span>
                </div>
                <span v-if="x.r.values[0] === 'cn'" class="to to-direct">{{ t('直连') }}</span>
                <select v-else-if="sceneOf(x.r.values[0])" v-model="sceneOf(x.r.values[0])!.target" class="select mini target" :class="targetClass(sceneOf(x.r.values[0])!.target)">
                  <option v-for="o in targetOptions(false)" :key="o.value" :value="o.value">{{ o.label }}</option>
                </select>
                <Switch v-if="sceneOf(x.r.values[0])" v-model="sceneOf(x.r.values[0])!.enabled" />
                <ResolveDns v-if="x.r.values[0] === 'cn' && sceneOf('cn')?.enabled" v-model="form.bypassDns" />
                <span v-else />
              </template>

              <RuleFields v-else :rule="x.r" :note="skipNote(x.r)" @remove="removeRule(x.i)" />
            </div>
          </div>
        </div>

        <div class="stn fixed" :class="stnClass(last)">
          <span class="no">{{ pathItems.length + 2 }}</span>
          <div class="box">
            <Icon class="lock" name="lock" :size="12" :title="t('固定在最后一步')" />
            <div class="what">
              <b>{{ t('其余流量') }}</b>
              <span class="sub">{{ t('上面都没有命中的') }}</span>
            </div>
            <span class="to" :class="form.routeMode === 'direct' ? 'to-direct' : 'to-proxy'">{{ finalLabel }}</span>
          </div>
        </div>
      </div>
    </template>

    <!-- ============================================================= catalog -->
    <ServiceCatalog v-else-if="view === 'apps'" :form="form" :match="match" @view="view = $event" />

    <!-- =============================================================== cards -->
    <template v-else>
      <div v-if="!textMode" class="block">
        <div class="block-head">
          <span class="block-title">{{ t('常用场景') }}</span>
          <span class="faint small">{{ t('打开即生效') }}</span>
        </div>
        <div class="scenes">
          <div v-for="sc in cardScenes" :key="sc.id" class="scene" :class="{ on: sceneOf(sc.id)?.enabled, hit: hitScene(sc.id) }">
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
              <ResolveDns v-if="sc.id === 'cn' && sceneOf('cn')?.enabled" v-model="form.bypassDns" class="end" />
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
          <button v-if="!textMode" class="btn small" :title="t('把域名、IP、程序等放进一个有名称的规则集，统一指定去向')" @click="newSetOpen = true"><Icon name="layers" :size="13" /> {{ t('添加规则集') }}</button>
          <button v-if="!textMode" class="btn small primary-soft" @click="addRule"><Icon name="plus" :size="13" /> {{ t('添加规则') }}</button>
        </div>
        <div v-if="!textMode" ref="listEl" class="rules" :class="{ 'sort-settling': listSorter.drag.settling }">
          <div
            v-for="(x, k) in ownItems"
            :key="x.i"
            class="rule"
            :class="[{ off: x.r.disabled || x.r.kind === 'package', hit: hitRule(x.i) }, listSorter.classes(k)]"
            :style="listSorter.style(k)"
            @pointerdown="listSorter.start($event, k)"
          >
            <Icon class="grip" name="grip" :size="14" :stroke="3" />
            <span class="num mono">{{ k + 1 }}</span>
            <RuleFields :rule="x.r" :note="skipNote(x.r)" @remove="removeRule(x.i)" />
          </div>
          <div v-if="!ownItems.length" class="empty-rules faint">
            {{ t('还没有规则。常用场景已经能满足大多数需要；想让某个网站或程序单独直连、走代理或走某个分组代理，就添加一条规则。') }}
          </div>
        </div>
      </div>
    </template>

    <template v-if="textMode">
      <textarea v-model="text" class="textarea mono rule-text" spellcheck="false" />
      <div v-if="textErrors.length" class="text-errors">
        <div v-for="e in textErrors" :key="e"><Icon name="warn" :size="12" /> {{ e }}</div>
      </div>
      <p v-else class="faint small">
        {{ t('修改会自动保存。类型：geosite、suffix（域名）、domain（完整域名）、keyword、regex、geoip、cidr（IP 段）、process（程序）；规则集里还可以用 url（规则集网址）。') }}
      </p>
    </template>

    <RouteStyleGuide v-model="guideOpen" :current="view" @done="guideDone" />
    <RuleSetEditor v-model="newSetOpen" @save="addSet" />

    <!-- shared by all looks -->
    <!-- left: the hide switch and the hints; right: 恢复默认, centred on them -->
    <div class="rules-foot">
      <div class="foot-left">
        <label v-if="view === 'path'" class="hide-off">
          <Switch v-model="hideOff" />
          <span>{{ t('隐藏未开启的步骤') }}</span>
          <span v-if="hideOff && hiddenCount" class="faint small">{{ t('已隐藏 {0} 个', hiddenCount) }}</span>
        </label>
        <p v-if="!textMode && ruleMode && view === 'path' && !ownCount" class="faint small hint-line">
          <span>{{ t('还没有自己的规则。') }}</span>
          <span>{{ t('想让某个网站或程序单独直连、走代理或走某个分组代理，点「添加规则」。') }}</span>
        </p>
      </div>
      <button class="btn ghost small reset" :title="t('删除自己的规则，场景和顺序恢复成刚安装时的样子')" @click="resetRules">
        <Icon name="refresh" :size="13" /> {{ t('恢复默认') }}
      </button>
    </div>

    <div class="form-row mirror">
      <div class="label">{{ t('规则集镜像') }}<span class="hint">{{ t('网站分类和 IP 归属地从这里下载') }}</span></div>
      <div class="control">
        <select v-model="form.ruleSetMirror" class="select">
          <option v-for="m in mirrors" :key="m.value" :value="m.value">{{ m.label }}</option>
          <option v-if="!mirrors.some((m) => m.value === form.ruleSetMirror)" :value="form.ruleSetMirror">{{ t('自定义') }}</option>
        </select>
        <button class="btn" @click="run(() => api.updateRuleSets(), (m) => m)"><Icon name="download" /> {{ t('更新规则集') }}</button>
      </div>
    </div>
  </Card>
</template>

<style scoped>
.small {
  font-size: 11.5px;
}
.mode-note {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--accent-soft);
  color: var(--text-2);
  font-size: 12.5px;
}
.mode-note > svg {
  color: var(--accent);
  flex-shrink: 0;
}
.mode-note.off {
  background: var(--hover);
}
.mode-note.off > svg {
  color: var(--text-3);
}
.mode-note span {
  flex: 1 1 260px;
}
.tester {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
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
.verdict .note {
  white-space: pre-line;
}
.path-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 16px 0 8px;
}
.block-title {
  font-weight: 700;
}
.primary-soft {
  color: var(--accent);
  border-color: var(--accent-line);
  background: var(--accent-soft);
}

/* ---- the path: a rail of numbered stops on the left */
.path {
  display: flex;
  flex-direction: column;
}
.stn {
  --h: 50px;
  height: var(--h);
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 8px;
  position: relative;
  touch-action: none;
}
.stn::before {
  content: '';
  position: absolute;
  left: 13px;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--accent-line);
}
.path > .stn:first-child::before {
  top: 50%;
}
.path > .stn:last-child::before {
  bottom: 50%;
}
.no {
  position: relative;
  z-index: 1;
  align-self: center;
  width: 24px;
  height: 24px;
  margin-left: 2px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 11px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  background: var(--card);
  border: 2px solid var(--accent-line);
  color: var(--text-2);
  transition: background 0.15s, border-color 0.15s, color 0.15s, box-shadow 0.15s;
}
.box {
  align-self: center;
  height: calc(var(--h) - 6px);
  display: grid;
  grid-template-columns: 14px minmax(0, 1fr) 148px 36px 28px;
  align-items: center;
  gap: 8px;
  padding: 0 6px 0 8px;
  border-radius: var(--radius);
  border: 1px solid var(--line);
  background: var(--card);
  transition: border-color 0.15s, box-shadow 0.15s, opacity 0.15s;
  min-width: 0;
}
/* the fixed steps line their destination up with the others' */
.fixed .box,
.entry .box {
  grid-template-columns: 14px minmax(0, 1fr) 148px 72px;
  background: var(--hover);
}
.entry .box {
  border-style: dashed;
}
.entry .no {
  color: var(--text-3);
}
.box > .what {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.35;
}
.what b {
  font-size: 12.5px;
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.what .sub {
  font-size: 11px;
  color: var(--text-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.scene-to .end {
  margin-left: auto;
}
.tag {
  font-size: 10px;
  font-weight: 700;
  padding: 0 5px;
  border-radius: 4px;
  background: var(--hover);
  border: 1px solid var(--line);
  color: var(--text-3);
  flex-shrink: 0;
}
.grip {
  color: var(--text-3);
  cursor: grab;
}
.lock {
  color: var(--text-3);
  opacity: 0.7;
}
.to {
  justify-self: start;
  font-size: 11.5px;
  font-weight: 700;
  padding: 3px 9px;
  border-radius: 7px;
  white-space: nowrap;
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
span.to-direct {
  background: color-mix(in srgb, var(--good) 12%, transparent);
}
span.to-block {
  background: color-mix(in srgb, var(--bad) 11%, transparent);
}
span.to-proxy {
  background: var(--accent-soft);
}
span.to-sys {
  color: var(--text-2);
  background: var(--card);
  border: 1px solid var(--line);
}
select.to-direct,
select.to-block,
select.to-proxy {
  font-weight: 700;
}
.select.mini {
  height: 28px;
  font-size: 12px;
  padding-top: 0;
  padding-bottom: 0;
}
.select.mini.target {
  width: 100%;
  min-width: 0;
}

/* skipped, passed and hit while testing */
.stn.skip .box {
  opacity: 0.5;
}
.stn.skip .no {
  border-color: var(--line);
  color: var(--text-3);
}
.stn.pass .no {
  background: var(--hover);
  border-color: var(--text-3);
  color: var(--text-3);
}
.stn.pass .box {
  opacity: 0.6;
}
.stn.hit .no {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
  box-shadow: 0 0 0 4px var(--accent-soft-2);
}
.stn.hit .box {
  opacity: 1;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft-2);
}
.stn.after .box {
  opacity: 0.45;
}
.stn.android .box > :deep(.what) {
  opacity: 0.7;
}

/* ---- the cards (the look of v2.0) */
.block {
  padding: 14px 0 6px;
}
.block + .block {
  border-top: 1px dashed var(--line);
}
.block-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
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
  grid-template-columns: 14px 18px minmax(0, 1fr) 150px 36px 28px;
  align-items: center;
  gap: 8px;
  padding: 6px 8px 6px 6px;
  border-radius: var(--radius);
  border: 1px solid var(--line);
  background: var(--card);
  touch-action: none;
}
.rule.off > :deep(.what) {
  opacity: 0.55;
}
.num {
  font-size: 11px;
  color: var(--text-3);
  text-align: right;
}
.empty-rules {
  padding: 14px;
  border-radius: var(--radius);
  border: 1px dashed var(--line);
  line-height: 1.7;
}

/* ---- shared */
.rules-foot {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed var(--line);
}
.hide-off {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  color: var(--text-2);
  cursor: pointer;
}
.foot-left {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}
.foot-left:empty {
  display: none;
}
.reset {
  flex-shrink: 0;
  margin-left: auto;
}
/* a question or a fact, then what to do: one line each */
.hint-line {
  margin: 0;
  line-height: 1.6;
}
.hint-line span {
  display: block;
}
.rule-text {
  width: 100%;
  min-height: 200px;
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
.mirror {
  margin-top: 10px;
  border-top: 1px dashed var(--line);
}
</style>
