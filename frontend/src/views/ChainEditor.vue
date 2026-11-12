<script setup lang="ts">
// Creates or edits a chain proxy: traffic leaves through several nodes in a
// row, entry first. Members are picked from any group and stay references, so
// a member edited later is used as it is then.
import { t } from '../i18n'
import { computed, ref, watch } from 'vue'
import Modal from '../components/Modal.vue'
import Icon from '../components/Icon.vue'
import Flag from '../components/Flag.vue'
import { api, type LatencyResult, type Profile } from '../api'
import { settings, state } from '../state'
import { bareName, countryMatches, countryTitle, nodeCountry } from '../country'
import { moveItem, useDragSort } from '../drag'
import { groupName, latencyClass, latencyText, run, serverOf, toast, typeName } from '../util'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ profile: Profile | null; groupId: number }>()

const MIN = 2
const MAX = 8

const all = ref<Profile[]>([]) // every node of every group, for lookups and the picker
const byId = computed(() => new Map(all.value.map((p) => [p.id, p])))
const name = ref('')
const nameTouched = ref(false)
const hops = ref<number[]>([])
const saveGroup = ref(0)
const testing = ref(false)
const result = ref<LatencyResult | null>(null)
const saving = ref(false)

const manualGroups = computed(() => (state.app?.groups ?? []).filter((g) => !g.url))

watch(open, async (v) => {
  if (!v) return
  all.value = await api.getProfiles(0).catch(() => [])
  name.value = props.profile?.name ?? ''
  nameTouched.value = !!props.profile
  hops.value = [...(props.profile?.chain ?? [])]
  result.value = null
  const current = state.app?.groups.find((g) => g.id === props.groupId)
  saveGroup.value = props.profile?.groupId ?? (current && !current.url ? current.id : (manualGroups.value[0]?.id ?? -1))
})

// ---- hops ----

const roleOf = (i: number) => (i === 0 ? t('入口') : i === hops.value.length - 1 ? t('落地') : t('中转'))

const rows = computed(() =>
  hops.value.map((id, i) => {
    const p = byId.value.get(id)
    return { id, i, p, role: roleOf(i), group: p ? groupName(state.app?.groups.find((g) => g.id === p.groupId)) : '' }
  }),
)
const missing = computed(() => rows.value.filter((r) => !r.p).map((r) => r.i + 1))

// the default name follows the route until the user types one
watch(hops, () => {
  if (nameTouched.value) return
  const first = byId.value.get(hops.value[0])
  const last = byId.value.get(hops.value[hops.value.length - 1])
  name.value = first && last && hops.value.length >= MIN ? `${bareName(first.name)} → ${bareName(last.name)}` : ''
}, { deep: true })

function remove(i: number) {
  hops.value.splice(i, 1)
  result.value = null
}
function shift(i: number, by: number) {
  moveItem(hops.value, i, i + by)
  result.value = null
}

const hopList = ref<HTMLElement>()
const sorter = useDragSort({
  container: hopList,
  item: '.hop',
  count: () => hops.value.length,
  enabled: () => hops.value.length > 1,
  ignore: 'button',
  onDrop: (from, to) => {
    moveItem(hops.value, from, to)
    result.value = null
  },
})

// ---- checks ----

// protocols that run over UDP, and nodes that cannot pass UDP on
const UDP_TYPES = ['hysteria2', 'hysteria', 'tuic', 'wireguard']
const noUDP = (p: Profile) => p.type === 'http' || (p.type === 'socks' && String(p.outbound?.version ?? '5') !== '5')

const warnings = computed(() => {
  const out: string[] = []
  rows.value.forEach((r, i) => {
    if (!r.p) return
    if (UDP_TYPES.includes(r.p.type)) {
      const blocker = rows.value.slice(0, i).find((b) => b.p && noUDP(b.p))
      if (blocker) {
        out.push(t('第 {0} 跳 {1}（{2}）基于 UDP，但第 {3} 跳 {4} 是 {5} 代理，不能转发 UDP，这条链路会连不上。把它放到 {4} 前面，或换一个节点。',
          i + 1, r.p.name, typeName(r.p.type), blocker.i + 1, blocker.p!.name, typeName(blocker.p!.type)))
      }
    }
    if (r.p.outbound?.detour) out.push(t('“{0}”自己设置了 detour，在链路里会被忽略。', r.p.name))
  })
  return out
})

const problem = computed(() => {
  if (missing.value.length) return t('第 {0} 跳的节点已不存在，请替换或移除', missing.value.join('、'))
  if (hops.value.length < MIN) return t('至少需要 {0} 个节点', MIN)
  if (saveGroup.value === 0) return t('请选择保存到哪个分组')
  return ''
})

async function test() {
  testing.value = true
  result.value = null
  try {
    result.value = await api.testChain([...hops.value])
  } catch (e) {
    toast((e as Error).message, 'error')
  } finally {
    testing.value = false
  }
}

async function save() {
  if (problem.value) return
  saving.value = true
  const saved = await run(
    // a test run in the editor (cleared on every change to the hops) is kept as the result
    () => api.saveChain({
      id: props.profile?.id ?? 0, groupId: saveGroup.value, name: name.value, chain: [...hops.value],
      latency: result.value?.latency ?? 0, exit: result.value?.exit,
    }),
    props.profile ? t('已保存') : t('链式代理已创建'),
  )
  saving.value = false
  if (saved) open.value = false
}

// ---- node picker ----

const pickerOpen = ref(false)
const replaceAt = ref(-1) // >= 0: replace that hop instead of appending
const pickGroup = ref(0)
const pickQuery = ref('')
const picked = ref<number[]>([])

function openPicker(replace = -1) {
  replaceAt.value = replace
  picked.value = []
  pickQuery.value = ''
  const last = byId.value.get(hops.value[hops.value.length - 1])
  pickGroup.value = last?.groupId ?? props.profile?.groupId ?? props.groupId ?? state.app?.groups[0]?.id ?? 0
  if (!state.app?.groups.some((g) => g.id === pickGroup.value)) pickGroup.value = state.app?.groups[0]?.id ?? 0
  pickerOpen.value = true
}

const candidates = computed(() => all.value.filter((p) => p.type !== 'chain'))
const groupCounts = computed(() => {
  const m = new Map<number, number>()
  for (const p of candidates.value) m.set(p.groupId, (m.get(p.groupId) ?? 0) + 1)
  return m
})
const pickList = computed(() => {
  const q = pickQuery.value.trim().toLowerCase()
  if (!q) return candidates.value.filter((p) => p.groupId === pickGroup.value)
  // a search looks through every group
  return candidates.value.filter(
    (p) => p.name.toLowerCase().includes(q) || countryMatches(nodeCountry(p, settings.value).code, q),
  )
})

const inChain = (id: number) => hops.value.includes(id) && hops.value[replaceAt.value] !== id
const room = computed(() => (replaceAt.value >= 0 ? 1 : MAX - hops.value.length))

function togglePick(p: Profile) {
  if (inChain(p.id)) return
  const i = picked.value.indexOf(p.id)
  if (i >= 0) picked.value.splice(i, 1)
  else if (replaceAt.value >= 0) picked.value = [p.id]
  else if (picked.value.length < room.value) picked.value.push(p.id)
  else toast(t('链式代理最多 {0} 个节点', MAX), 'error')
}

function confirmPick() {
  if (!picked.value.length) return
  if (replaceAt.value >= 0) hops.value.splice(replaceAt.value, 1, picked.value[0])
  else hops.value.push(...picked.value)
  result.value = null
  pickerOpen.value = false
}

const groupLabel = (id: number) => groupName(state.app?.groups.find((g) => g.id === id))
</script>

<template>
  <Modal v-model="open" :title="profile ? t('编辑链式代理') : t('新建链式代理')" :width="600">
    <div class="top">
      <label class="field grow">
        <span class="faint">{{ t('名称') }}</span>
        <input v-model="name" class="input" :placeholder="t('例如：机场 → 自建 JP')" @input="nameTouched = true" />
      </label>
      <label class="field">
        <span class="faint">{{ t('保存到分组') }}</span>
        <select v-model.number="saveGroup" class="select">
          <option v-for="g in manualGroups" :key="g.id" :value="g.id">{{ groupName(g) }}</option>
          <option :value="-1">{{ t('新建“链式代理”分组') }}</option>
        </select>
      </label>
    </div>

    <div class="strip">
      <span class="chip end"><small>{{ t('本机') }}</small><b>NextBox</b></span>
      <template v-for="r in rows" :key="r.id + '-' + r.i">
        <span class="arrow">→</span>
        <span class="chip" :class="{ bad: !r.p, exit: r.i === rows.length - 1 }">
          <small>{{ r.role }}</small>
          <b><Flag v-if="r.p" :code="nodeCountry(r.p, settings).code" :size="13" />{{ r.p ? bareName(r.p.name) : t('已失效') }}</b>
        </span>
      </template>
      <template v-if="rows.length < MIN">
        <span class="arrow">→</span>
        <span class="chip ghost"><small>{{ rows.length ? t('落地') : t('入口') }}</small><b>{{ t('待添加') }}</b></span>
      </template>
      <span class="arrow">→</span>
      <span class="chip end"><small>{{ t('目标') }}</small><b>{{ t('网站') }}</b></span>
      <span class="result">
        <template v-if="testing"><span class="faint">{{ t('测试中…') }}</span></template>
        <template v-else-if="result">
          <span class="faint">{{ t('端到端延迟') }}</span>
          <b class="mono" :class="latencyClass(result.latency)" :title="result.error">{{ latencyText(result.latency) }}</b>
          <span v-if="result.exit" class="faint exit-ip"><Flag :code="result.exit.country" :size="12" /> {{ result.exit.ip }}</span>
        </template>
      </span>
    </div>

    <div class="hops-head">
      <span class="faint">{{ t('节点顺序：拖动或用箭头调整，第一个是入口，最后一个是落地') }}</span>
      <span class="faint mono">{{ hops.length }} / {{ MAX }}</span>
    </div>
    <div ref="hopList" class="hops" :class="{ 'sort-settling': sorter.drag.settling }">
      <div
        v-for="r in rows"
        :key="r.id + '-' + r.i"
        class="hop"
        :class="[{ missing: !r.p }, sorter.classes(r.i)]"
        :style="sorter.style(r.i)"
        @pointerdown="sorter.start($event, r.i)"
      >
        <span class="grip" aria-hidden="true"><i /><i /><i /><i /><i /><i /></span>
        <span class="role" :class="{ exit: r.i === rows.length - 1 && rows.length > 1 }">{{ r.role }}</span>
        <template v-if="r.p">
          <Flag :code="nodeCountry(r.p, settings).code" :title="countryTitle(nodeCountry(r.p, settings), settings)" />
          <div class="who">
            <b class="ellipsis" :title="r.p.name">{{ bareName(r.p.name) }}</b>
            <span class="faint ellipsis">{{ r.group }} · {{ serverOf(r.p.outbound) }}</span>
          </div>
          <span class="badge">{{ typeName(r.p.type) }}</span>
          <span class="lat mono" :class="latencyClass(r.p.latency)">{{ latencyText(r.p.latency) }}</span>
        </template>
        <template v-else>
          <Flag />
          <div class="who">
            <b class="bad-text">{{ t('节点已不存在') }}</b>
            <span class="faint ellipsis">{{ t('可能被删除，或订阅更新后改了名字') }}</span>
          </div>
          <button class="btn small" @click="openPicker(r.i)">{{ t('替换') }}</button>
          <span />
        </template>
        <div class="acts">
          <button class="btn ghost small icon" :disabled="r.i === 0" :title="t('上移')" @click="shift(r.i, -1)"><Icon name="up" :size="14" /></button>
          <button class="btn ghost small icon" :disabled="r.i === rows.length - 1" :title="t('下移')" @click="shift(r.i, 1)"><Icon name="down" :size="14" /></button>
          <button class="btn ghost small icon danger" :title="t('移除')" @click="remove(r.i)"><Icon name="close" :size="14" /></button>
        </div>
      </div>
    </div>
    <button class="add" :disabled="hops.length >= MAX" @click="openPicker()">
      <Icon name="plus" :size="14" /> {{ hops.length >= MAX ? t('已达到 {0} 个节点的上限', MAX) : t('添加节点') }}
    </button>

    <div v-for="w in warnings" :key="w" class="alert warn"><Icon name="warn" :size="14" /><span>{{ w }}</span></div>
    <div class="alert info"><Icon name="info" :size="14" /><span>{{ t('网站看到的是落地节点的 IP。每多一跳，延迟会增加这一跳到下一跳的往返时间。') }}</span></div>

    <template #footer>
      <span class="foot-note" :class="{ bad: problem && hops.length }">{{ problem }}</span>
      <button class="btn" :disabled="testing || !!missing.length || hops.length < MIN" @click="test">
        <Icon name="zap" :size="14" /> {{ testing ? t('测试中…') : t('测试链路') }}
      </button>
      <button class="btn" @click="open = false">{{ t('取消') }}</button>
      <button class="btn primary" :disabled="!!problem || saving" @click="save">{{ t('保存') }}</button>
    </template>
  </Modal>

  <Modal v-model="pickerOpen" :title="replaceAt >= 0 ? t('替换第 {0} 跳', replaceAt + 1) : t('添加节点到链路')" :width="640">
    <div class="search wide">
      <Icon name="search" :size="14" />
      <input v-model="pickQuery" class="input" :placeholder="t('搜索所有分组的节点，也可以搜国家，例如“香港”')" />
    </div>
    <div class="picker">
      <div class="nav-list pick-groups" :class="{ dim: pickQuery.trim() }">
        <div
          v-for="g in state.app?.groups"
          :key="g.id"
          class="nav-item"
          :class="{ active: g.id === pickGroup && !pickQuery.trim() }"
          @click="(pickGroup = g.id), (pickQuery = '')"
        >
          <Icon :name="g.url ? 'rss' : 'folder'" :size="15" />
          <span class="ellipsis">{{ groupName(g) }}</span>
          <span class="count">{{ groupCounts.get(g.id) ?? 0 }}</span>
        </div>
      </div>
      <div class="pick-list scroll">
        <div
          v-for="p in pickList"
          :key="p.id"
          class="pick"
          :class="{ on: picked.includes(p.id), off: inChain(p.id) }"
          @click="togglePick(p)"
        >
          <span class="check"><Icon v-if="picked.includes(p.id)" name="check" :size="12" :stroke="3" /></span>
          <Flag :code="nodeCountry(p, settings).code" :title="countryTitle(nodeCountry(p, settings), settings)" />
          <span class="pick-name ellipsis">
            {{ bareName(p.name) }}
            <small v-if="inChain(p.id)" class="faint">{{ t('已在链路中') }}</small>
            <small v-else-if="pickQuery.trim()" class="faint">{{ groupLabel(p.groupId) }}</small>
          </span>
          <span class="badge">{{ typeName(p.type) }}</span>
          <span class="lat mono" :class="latencyClass(p.latency)">{{ latencyText(p.latency) }}</span>
          <span v-if="picked.includes(p.id) && replaceAt < 0" class="order mono">{{ picked.indexOf(p.id) + 1 }}</span>
          <span v-else />
        </div>
        <div v-if="!pickList.length" class="empty small-empty">
          <Icon name="nodes" :size="32" :stroke="1.5" />
          <div>{{ pickQuery.trim() ? t('没有匹配“{0}”的节点', pickQuery) : t('这个分组还没有节点') }}</div>
        </div>
      </div>
    </div>
    <template #footer>
      <span class="foot-note">{{ replaceAt >= 0 ? t('选一个节点替换这一跳') : t('按勾选顺序加到链路末尾') }}</span>
      <button class="btn" @click="pickerOpen = false">{{ t('取消') }}</button>
      <button class="btn primary" :disabled="!picked.length" @click="confirmPick">
        {{ replaceAt >= 0 ? t('替换') : picked.length ? t('添加 {0} 个节点', picked.length) : t('添加') }}
      </button>
    </template>
  </Modal>
</template>

<style scoped>
.top {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 190px;
  gap: 12px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
}

/* ---- route strip ---- */
.strip {
  margin-top: 14px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px;
  border-radius: var(--radius);
  background: var(--accent-soft);
  border: 1px solid var(--accent-line);
}
.chip {
  display: flex;
  flex-direction: column;
  align-items: center;
  max-width: 150px;
  padding: 5px 10px;
  border-radius: 8px;
  background: var(--card);
  border: 1px solid var(--card-border);
  box-shadow: var(--shadow);
}
.chip small {
  font-size: 10px;
  font-weight: 700;
  color: var(--text-3);
  letter-spacing: 0.5px;
}
.chip b {
  max-width: 100%;
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.chip.exit small {
  color: var(--accent);
}
.chip.end {
  background: transparent;
  border-style: dashed;
  box-shadow: none;
  color: var(--text-2);
}
.chip.ghost {
  background: transparent;
  border: 1.5px dashed var(--accent-line);
  box-shadow: none;
  color: var(--text-3);
}
.chip.bad {
  border-color: var(--bad);
  color: var(--bad);
}
.arrow {
  color: var(--accent);
  font-weight: 700;
}
.result {
  margin-left: auto;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  font-size: 11px;
  line-height: 1.35;
}
.result b {
  font-size: 13px;
}
.exit-ip {
  display: flex;
  align-items: center;
  gap: 3px;
}

/* ---- hops ---- */
.hops-head {
  margin: 14px 0 6px;
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}
.hops {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.hop {
  position: relative;
  display: grid;
  grid-template-columns: 12px 40px 20px minmax(0, 1fr) auto 62px auto;
  align-items: center;
  gap: 10px;
  height: 50px;
  padding: 0 6px 0 10px;
  border: 1px solid var(--card-border);
  border-radius: var(--radius);
  background: var(--card);
  box-shadow: var(--shadow);
  cursor: grab;
  touch-action: none;
  transition: border-color 0.15s;
}
.hop:hover {
  border-color: var(--accent-line);
}
.hop.missing {
  border-color: color-mix(in srgb, var(--bad) 45%, var(--card));
}
.grip {
  display: grid;
  grid-template-columns: repeat(2, 3px);
  gap: 3px;
  opacity: 0.45;
}
.grip i {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--text-2);
}
.hop:hover .grip {
  opacity: 0.9;
}
.role {
  font-size: 11px;
  font-weight: 700;
  text-align: center;
  padding: 2px 0;
  border-radius: 6px;
  background: var(--hover);
  color: var(--text-2);
}
.role.exit {
  background: var(--accent-soft-2);
  color: var(--accent);
}
.who {
  min-width: 0;
  display: flex;
  flex-direction: column;
  line-height: 1.35;
}
.who b {
  font-weight: 600;
}
.who span {
  font-size: 11px;
}
.bad-text {
  color: var(--bad);
}
.lat {
  font-size: 12px;
  text-align: right;
}
.acts {
  display: flex;
}
.add {
  margin-top: 6px;
  width: 100%;
  height: 38px;
  border: 1.5px dashed var(--input-border);
  border-radius: var(--radius);
  background: transparent;
  color: var(--text-2);
  font: inherit;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}
.add:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent);
  background: var(--accent-soft);
}
.add:disabled {
  cursor: default;
  opacity: 0.6;
}

.alert {
  margin-top: 10px;
  display: flex;
  gap: 8px;
  padding: 9px 12px;
  border-radius: var(--radius);
  font-size: 12px;
  line-height: 1.55;
}
.alert .icon {
  flex-shrink: 0;
  margin-top: 2px;
}
.alert.warn {
  background: color-mix(in srgb, var(--mid) 12%, var(--card));
  border: 1px solid color-mix(in srgb, var(--mid) 35%, var(--card));
}
.alert.warn .icon {
  color: var(--mid);
}
.alert.info {
  background: var(--hover);
  color: var(--text-2);
}

.foot-note {
  margin-right: auto;
  align-self: center;
  font-size: 12px;
  color: var(--text-3);
}
.foot-note.bad {
  color: var(--bad);
}

/* ---- picker ---- */
.search.wide {
  display: flex;
  align-items: center;
  gap: 8px;
  position: relative;
}
.search.wide .icon {
  position: absolute;
  left: 10px;
  color: var(--text-3);
}
.search.wide .input {
  width: 100%;
  padding-left: 30px;
}
.picker {
  margin-top: 12px;
  display: grid;
  grid-template-columns: 170px minmax(0, 1fr);
  gap: 12px;
  height: 340px;
}
.pick-groups {
  overflow-y: auto;
  padding-right: 10px;
  border-right: 1px solid var(--line);
  transition: opacity 0.15s;
}
.pick-groups.dim {
  opacity: 0.55;
}
.pick-list {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding-right: 4px;
}
.pick {
  flex-shrink: 0;
  display: grid;
  grid-template-columns: 16px 20px minmax(0, 1fr) auto 58px 18px;
  align-items: center;
  gap: 9px;
  height: 40px;
  padding: 0 10px;
  border-radius: var(--radius);
  border: 1px solid var(--card-border);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.pick:hover {
  border-color: var(--accent-line);
}
.pick.on {
  background: var(--accent-soft);
  border-color: var(--accent-line);
}
.pick.off {
  opacity: 0.5;
  cursor: default;
}
.check {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: 2px solid var(--input-border);
  display: grid;
  place-items: center;
  color: #fff;
  transition: background 0.15s, border-color 0.15s;
}
.pick.on .check {
  background: var(--accent);
  border-color: var(--accent);
}
.pick-name {
  font-weight: 600;
}
.pick-name small {
  margin-left: 6px;
  font-weight: 400;
  font-size: 11px;
}
.order {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  display: grid;
  place-items: center;
}
.small-empty {
  padding: 40px 0;
}
</style>
