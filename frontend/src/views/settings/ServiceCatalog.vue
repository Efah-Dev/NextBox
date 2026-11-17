<script setup lang="ts">
// 服务目录, the simplest look of the routing rules (the one suggested to
// beginners): well-known services with their icons, grouped by kind, each
// with where it goes. A service follows
// the scenes and the rest of the route until it is given a place of its own;
// then it is an ordinary rule (site category → target, labelled with the
// service in Rule.app) at the top of the route, so it comes before the scenes
// and shows in the other two looks too. A rule with the service's category
// and nothing else that was added by hand counts as the service's as well.
import { computed, ref } from 'vue'
import { t } from '../../i18n'
import Icon from '../../components/Icon.vue'
import Switch from '../../components/Switch.vue'
import ServiceIcon from '../../components/ServiceIcon.vue'
import type { RouteMatch, Settings, Target } from '../../api'
import { finalLabel, isStep, sceneCards, sceneInfo, targetClass, targetLabel, targetOptions } from '../../policies'
import { serviceCategories, services, type Service, type ServiceCategory } from '../../services'

const props = defineProps<{ form: Settings; match: RouteMatch | null }>()
defineEmits<{ view: ['path' | 'cards'] }>()
const form = props.form

const sceneOf = (id: string) => form.scenes.find((s) => s.id === id)
const sameSet = (a: string[], b: string[]) => a.length === b.length && a.every((v) => b.includes(v))

/** For each service with a place of its own: the index of its rule. */
const places = computed(() => {
  const m = new Map<string, number>()
  for (const sv of services) {
    let i = form.rules.findIndex((r) => r.app === sv.id)
    if (i < 0) i = form.rules.findIndex((r) => !r.app && !r.disabled && (sv.suffix ? r.kind === 'suffix' && sameSet(r.values, sv.suffix) : r.kind === 'geosite' && sameSet(r.values, sv.sets)))
    if (i >= 0) m.set(sv.id, i)
  }
  return m
})
/** The rule that gives the service its place, or -1. */
const ruleAt = (sv: Service) => places.value.get(sv.id) ?? -1
/** Where the service goes when it has a place of its own, '' = it follows. */
function choice(sv: Service): Target | '' {
  const i = ruleAt(sv)
  return i >= 0 && !form.rules[i].disabled ? form.rules[i].target : ''
}
function choose(sv: Service, v: Target | '') {
  const i = ruleAt(sv)
  if (!v) {
    form.rules = form.rules.filter((r, k) => k !== i && r.app !== sv.id)
    return
  }
  if (i >= 0) {
    Object.assign(form.rules[i], { target: v, disabled: false, app: sv.id })
    return
  }
  // at the top of the route, only after ad blocking when that comes first
  const at = form.rules[0]?.kind === 'ads' ? 1 : 0
  form.rules.splice(at, 0, sv.suffix ? { kind: 'suffix', values: [...sv.suffix], target: v, app: sv.id } : { kind: 'geosite', values: [...sv.sets], target: v, app: sv.id })
}

/** What a service that has no place of its own does. */
function follows(sv: Service) {
  const sc = sv.scene ? sceneOf(sv.scene) : undefined
  if (sc?.enabled) {
    const to = sv.scene === 'cn' ? t('直连') : targetLabel(sc.target)
    return t('跟随「{0}」→ {1}', sceneInfo(sv.scene).label, to)
  }
  return t('跟随「其余流量」→ {0}', finalLabel(form.routeMode))
}

// ---- filters
const cat = ref<ServiceCategory | 'all' | 'own'>('all')
const query = ref('')
const own = computed(() => services.filter((sv) => choice(sv)))
const shown = computed(() => {
  const q = query.value.trim().toLowerCase()
  return services.filter((sv) => {
    if (cat.value === 'own' ? !choice(sv) : cat.value !== 'all' && sv.cat !== cat.value) return false
    return !q || sv.name.toLowerCase().includes(q) || sv.id.includes(q) || sv.sets.some((s) => s.includes(q)) || !!sv.suffix?.some((s) => s.includes(q))
  })
})
const catLabel = (c: ServiceCategory) => serviceCategories.find((x) => x.id === c)?.label ?? c
const counts = computed(() => {
  const n = { proxy: 0, direct: 0, block: 0, policy: 0 }
  for (const sv of own.value) {
    const c = choice(sv)
    if (c === 'proxy' || c === 'direct' || c === 'block') n[c]++
    else n.policy++
  }
  return n
})

// rules of the user's own that the catalog does not show
const otherRules = computed(() => {
  const taken = new Set(places.value.values())
  return form.rules.filter((r, i) => !isStep(r) && !taken.has(i)).length
})

const hit = (sv: Service) => props.match?.source === 'rule' && ruleAt(sv) >= 0 && props.match.index === ruleAt(sv)
const cardScenes = sceneCards()
</script>

<template>
  <div class="block">
    <div class="block-head">
      <span class="block-title">{{ t('常用场景') }}</span>
      <span class="faint small">{{ t('没有单独指定的服务跟随这里') }}</span>
    </div>
    <div class="scene-strip">
      <div v-for="sc in cardScenes" :key="sc.id" class="chip-scene" :class="{ on: sceneOf(sc.id)?.enabled }">
        <span class="ellipsis" :title="sc.desc">{{ sc.label }}</span>
        <span v-if="sc.id === 'cn'" class="to to-direct">{{ t('直连') }}</span>
        <select v-else-if="sceneOf(sc.id)" v-model="sceneOf(sc.id)!.target" class="select mini" :class="targetClass(sceneOf(sc.id)!.target)">
          <option v-for="o in targetOptions(false)" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
        <Switch v-if="sceneOf(sc.id)" v-model="sceneOf(sc.id)!.enabled" />
      </div>
    </div>
  </div>

  <div class="block">
    <div class="block-head">
      <span class="block-title">{{ t('服务目录') }}</span>
      <span class="faint small">
        {{ t('共 {0} 个服务，单独指定 {1} 个', services.length, own.length) }}
        <template v-if="own.length">
          （<span v-if="counts.proxy" class="to-proxy">{{ t('代理 {0}', counts.proxy) }}</span
          ><span v-if="counts.direct" class="to-direct">{{ t('直连 {0}', counts.direct) }}</span
          ><span v-if="counts.block" class="to-block">{{ t('拦截 {0}', counts.block) }}</span
          ><span v-if="counts.policy" class="to-proxy">{{ t('分组 {0}', counts.policy) }}</span>）
        </template>
      </span>
    </div>
    <div class="tools">
      <div class="search">
        <Icon name="search" :size="14" />
        <input v-model="query" class="input" :placeholder="t('搜索服务，例如 YouTube')" spellcheck="false" />
      </div>
      <div class="cats">
        <button class="cat" :class="{ on: cat === 'all' }" @click="cat = 'all'">{{ t('全部') }}</button>
        <button v-for="c in serviceCategories" :key="c.id" class="cat" :class="{ on: cat === c.id }" @click="cat = c.id">{{ c.label }}</button>
        <button class="cat" :class="{ on: cat === 'own' }" @click="cat = 'own'">
          {{ t('已指定') }}<span v-if="own.length" class="n">{{ own.length }}</span>
        </button>
      </div>
    </div>

    <TransitionGroup v-if="shown.length" tag="div" name="svc" class="svcs">
      <div v-for="sv in shown" :key="sv.id" class="svc" :class="{ set: !!choice(sv), hit: hit(sv) }">
        <ServiceIcon :service="sv" />
        <div class="svc-text">
          <b class="ellipsis">{{ sv.name }}</b>
          <span class="sub ellipsis" :title="sv.suffix ? sv.suffix.join(', ') : sv.sets.map((s) => 'geosite:' + s).join(', ')">
            <template v-if="choice(sv)">{{ t('单独指定') }}</template>
            <template v-else>{{ follows(sv) }}</template>
          </span>
        </div>
        <select
          class="select mini"
          :class="choice(sv) ? targetClass(choice(sv) as Target) : 'follow'"
          :value="choice(sv)"
          :title="catLabel(sv.cat)"
          @change="choose(sv, ($event.target as HTMLSelectElement).value as Target | '')"
        >
          <option value="">{{ t('跟随默认') }}</option>
          <option v-for="o in targetOptions()" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </div>
    </TransitionGroup>
    <div v-else class="none faint">
      {{ cat === 'own' && !query ? t('还没有单独指定的服务。在「全部」里给某个服务选择「当前节点」「直连」或某个分组即可。') : t('没有找到这个服务。可以在「路由路径」或「卡片」样式里，用「网站分类」或「域名」规则添加。') }}
    </div>

    <p class="foot faint small">
      {{ t('单独指定的服务会排在路由最前面，比常用场景优先。') }}
      <template v-if="otherRules">
        {{ t('另外还有 {0} 条自己的规则，', otherRules) }}<a href="#" @click.prevent="$emit('view', 'path')">{{ t('在「路由路径」里查看') }}</a>
      </template>
    </p>
  </div>
</template>

<style scoped>
.small {
  font-size: 11.5px;
}
.block {
  margin-top: 16px;
}
.block-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 8px;
}
.block-title {
  font-weight: 700;
  font-size: 13px;
}
.scene-strip {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px;
}
.chip-scene {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 8px;
  height: 40px;
  padding: 0 8px 0 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  color: var(--text-2);
  transition: border-color 0.15s, background 0.15s;
}
.chip-scene.on {
  color: var(--text);
  font-weight: 700;
  border-color: var(--accent-line);
  background: var(--accent-soft);
}
.chip-scene .select.mini {
  width: 96px;
}
.select.mini {
  height: 28px;
  font-size: 12px;
  padding-top: 0;
  padding-bottom: 0;
}
.to {
  font-size: 12px;
  font-weight: 700;
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
select.follow {
  color: var(--text-3);
}
.small span + span::before {
  content: ' · ';
  color: var(--text-3);
}
.tools {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}
.search {
  position: relative;
  flex: 0 1 220px;
  min-width: 160px;
}
.search svg {
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-3);
}
.search .input {
  padding-left: 30px;
}
.cats {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.cat {
  height: 28px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--text-2);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  transition: all 0.15s;
}
.cat:hover {
  border-color: var(--accent-line);
  color: var(--text);
}
.cat.on {
  border-color: var(--accent);
  background: var(--accent-soft);
  color: var(--accent);
}
.cat .n {
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  background: var(--accent);
  color: #fff;
  font-size: 10.5px;
  line-height: 16px;
  text-align: center;
}
.svcs {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(270px, 1fr));
  gap: 8px;
}
.svc {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) 112px;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--card);
  transition: border-color 0.15s, box-shadow 0.2s, background 0.2s;
}
.svc:hover {
  border-color: var(--accent-line);
}
.svc.set {
  border-color: var(--accent-line);
  background: color-mix(in srgb, var(--accent) 4%, var(--card));
}
.svc.hit {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft-2);
}
.svc-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.svc-text b {
  font-size: 13px;
}
.sub {
  font-size: 11px;
  color: var(--text-3);
}
.svc.set .sub {
  color: var(--accent);
  font-weight: 600;
}
.none {
  padding: 18px 12px;
  border: 1px dashed var(--line);
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  text-align: center;
  line-height: 1.7;
}
.foot {
  margin: 10px 0 0;
  line-height: 1.7;
}
.foot a {
  color: var(--accent);
}
.svc-move,
.svc-enter-active,
.svc-leave-active {
  transition: opacity 0.18s, transform 0.18s;
}
.svc-enter-from,
.svc-leave-to {
  opacity: 0;
  transform: scale(0.97);
}
.svc-leave-active {
  position: absolute;
  visibility: hidden;
}
@media (prefers-reduced-motion: reduce) {
  .svc-move,
  .svc-enter-active,
  .svc-leave-active {
    transition: none;
  }
}
</style>
