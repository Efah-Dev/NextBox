<script setup lang="ts">
// 分组代理 on the node page: use a policy group for the connection instead
// of one node. An automatic group switches to its fastest member by itself;
// in a manual group a member is picked here. Picking a node in any group
// goes back to that node.
import { t } from '../i18n'
import { computed, onMounted, onUnmounted } from 'vue'
import Icon from '../components/Icon.vue'
import Flag from '../components/Flag.vue'
import ListMotion from '../motion/ListMotion.vue'
import { api, rt, type PolicyView } from '../api'
import { bareName, countryName } from '../country'
import { policies, refreshPolicies } from '../policies'
import { state } from '../state'
import { groupName, latencyClass, latencyText, run } from '../util'

const emit = defineEmits<{ edit: [] }>()

let off: (() => void) | undefined
onMounted(() => {
  refreshPolicies()
  off = rt.on('state', () => refreshPolicies())
})
onUnmounted(() => off?.())

const used = computed(() => state.app?.selectedPolicy ?? 0)

function describe(p: PolicyView) {
  const scope = p.groupId ? groupName(state.app?.groups.find((g) => g.id === p.groupId)) + ' · ' : ''
  const from = p.match === 'country' ? t('地区：{0}', countryName(p.country ?? '')) : p.match === 'keyword' ? t('名称包含：{0}', p.keyword ?? '') : t('手动挑选')
  const how = p.mode === 'auto' ? t('自动选最快 · 每 {0} 分钟', p.interval || 10) : t('手动选择')
  return `${scope}${from} · ${how}`
}
const inUse = (p: PolicyView) => p.nodes.find((n) => n.id === (p.now || p.selected)) ?? (p.mode === 'select' ? p.nodes[0] : undefined)

async function use(p: PolicyView) {
  if (!p.nodes.length) return
  await run(() => api.usePolicy(used.value === p.id ? 0 : p.id), used.value === p.id ? t('已改回单个节点') : t('已改用分组代理“{0}”', p.name))
  refreshPolicies()
}
async function pick(p: PolicyView, id: number) {
  if (p.mode !== 'select') return
  await run(() => api.selectPolicyNode(p.id, id))
  refreshPolicies()
}
function manage() {
  state.settingsSection = 'policy'
  state.page = 'settings'
}
</script>

<template>
  <div class="toolbar">
    <div class="title">
      <b>{{ t('分组代理') }}</b>
      <span class="faint">{{ t('用一组节点连接，自动或手动选择其中的节点') }}</span>
    </div>
    <button class="btn" @click="manage"><Icon name="settings" /> {{ t('管理分组代理') }}</button>
    <button class="btn primary" @click="emit('edit')"><Icon name="plus" /> {{ t('新建') }}</button>
  </div>
  <ListMotion class="cards scroll">
    <div v-for="p in policies.list" :key="p.id" class="pcard" :class="{ on: used === p.id, empty: !p.nodes.length }">
      <div class="pc-head" @click="use(p)">
        <span class="radio" />
        <div class="pc-title">
          <b class="ellipsis">{{ p.name }}</b>
          <span class="faint ellipsis">{{ describe(p) }}</span>
        </div>
        <span v-if="!p.nodes.length" class="badge bad">{{ t('没有可用节点') }}</span>
        <span v-else-if="used === p.id" class="badge">{{ t('正在使用') }}</span>
        <span v-else class="use">{{ t('用它连接') }}</span>
      </div>
      <div class="members">
        <button
          v-for="n in p.nodes.slice(0, 24)"
          :key="n.id"
          class="member"
          :class="{ now: inUse(p)?.id === n.id, pickable: p.mode === 'select' }"
          :title="p.mode === 'select' ? t('使用这个节点') : n.name"
          @click="pick(p, n.id)"
        >
          <Flag :code="n.country" :size="13" />
          <span class="ellipsis">{{ bareName(n.name) }}</span>
          <span class="lat mono" :class="latencyClass(n.latency)">{{ latencyText(n.latency) }}</span>
        </button>
        <span v-if="p.nodes.length > 24" class="faint more">{{ t('等 {0} 个', p.nodes.length) }}</span>
      </div>
    </div>
    <div v-if="!policies.list.length" key="empty" class="empty">
      <Icon name="layers" :size="40" :stroke="1.5" />
      <div>{{ t('还没有分组代理') }}</div>
      <button class="btn primary" @click="emit('edit')"><Icon name="plus" /> {{ t('新建分组代理') }}</button>
    </div>
  </ListMotion>
  <p v-if="used" class="faint back-note">{{ t('在任意分组里选择一个节点，即可改回单个节点。') }}</p>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.title {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.title b {
  font-size: 16px;
}
.title span {
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cards {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0 -6px;
  padding: 2px 6px 8px;
}
.pcard {
  flex-shrink: 0;
  background: var(--card);
  border: 1px solid var(--card-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  transition: border-color var(--t-fast), background var(--t-fast);
}
.pcard.on {
  border-color: var(--accent-line);
  background: var(--accent-soft);
}
.pcard.empty {
  opacity: 0.7;
}
.pc-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px 8px;
  cursor: pointer;
}
.radio {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  border-radius: 50%;
  border: 2px solid var(--input-border);
  transition: border var(--t-fast);
}
.on .radio {
  border: 5px solid var(--accent);
}
.pc-title {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.on .pc-title b {
  color: var(--accent);
}
.pc-title .faint {
  font-size: 12px;
}
.use {
  font-size: 12px;
  color: var(--accent);
  opacity: 0;
  transition: opacity var(--t-fast);
}
.pc-head:hover .use {
  opacity: 1;
}
.badge.bad {
  background: color-mix(in srgb, var(--bad) 12%, var(--card));
  color: var(--bad);
}
/* members in columns: flag | name | latency, the latencies right-aligned
   and on the same line as the names */
.members {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 6px;
  padding: 0 14px 12px 42px;
}
.member {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  column-gap: 7px;
  height: 30px;
  padding: 0 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--card);
  color: var(--text-2);
  font: inherit;
  font-size: 12px;
  line-height: 1;
  text-align: left;
  cursor: default;
  transition: border-color var(--t-fast), background var(--t-fast), color var(--t-fast);
}
.member.pickable {
  cursor: pointer;
}
.member.pickable:hover {
  border-color: var(--accent-line);
  color: var(--text);
}
.member.now {
  border-color: var(--accent);
  background: var(--accent-soft-2);
  color: var(--text);
  font-weight: 700;
}
.member .lat {
  font-size: 11px;
  line-height: 1;
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.more {
  align-self: center;
  font-size: 12px;
  padding-left: 4px;
}
.back-note {
  margin: 0;
  font-size: 12px;
  text-align: center;
}
</style>
