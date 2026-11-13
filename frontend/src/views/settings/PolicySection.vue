<script setup lang="ts">
// 分组代理: groups of nodes that rules can send traffic to, for example
// YouTube → the fastest Japanese node, Google → a fixed US node.
import { t } from '../../i18n'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Card from '../../components/Card.vue'
import Icon from '../../components/Icon.vue'
import Flag from '../../components/Flag.vue'
import PolicyEditor from './PolicyEditor.vue'
import { api, rt, type Policy, type PolicyView, type Settings } from '../../api'
import { bareName, countryName } from '../../country'
import { policies, refreshPolicies } from '../../policies'
import { state } from '../../state'
import { confirmDialog, groupName, latencyClass, latencyText, run } from '../../util'

const props = defineProps<{ form: Settings; index: number }>()

const editorOpen = ref(false)
const editing = ref<Policy | null>(null)
function edit(p: PolicyView | null) {
  editing.value = p
  editorOpen.value = true
}

let off: (() => void) | undefined
onMounted(() => {
  refreshPolicies()
  // the current pick of automatic groups changes while connected
  off = rt.on('state', () => refreshPolicies())
})
onUnmounted(() => off?.())

function describe(p: PolicyView) {
  const scope = p.groupId ? groupName(state.app?.groups.find((g) => g.id === p.groupId)) + ' · ' : ''
  if (p.match === 'country') return scope + t('地区：{0}', countryName(p.country ?? ''))
  if (p.match === 'keyword') return scope + t('名称包含：{0}', p.keyword ?? '')
  return scope + t('手动挑选')
}

const current = (p: PolicyView) => p.nodes.find((n) => n.id === (p.now || p.selected)) ?? (p.mode === 'select' ? p.nodes[0] : undefined)

async function remove(p: PolicyView) {
  const msg = p.rules
    ? t('有 {0} 条规则在使用“{1}”，删除后它们会改走当前节点。确定删除吗？', p.rules, p.name)
    : t('确定删除分组代理“{0}”吗？', p.name)
  if (!(await confirmDialog(t('删除分组代理'), msg, true))) return
  await run(() => api.deletePolicy(p.id))
  refreshPolicies()
}

async function choose(p: PolicyView, id: number) {
  await run(() => api.selectPolicyNode(p.id, id))
  refreshPolicies()
}

const usedHint = computed(() => props.form.routeMode !== 'rule')
</script>

<template>
  <Card id="sec-policy" :title="t('分组代理')" :index="index">
    <template #actions>
      <button class="btn small" @click="edit(null)"><Icon name="plus" :size="13" /> {{ t('新建分组') }}</button>
    </template>
    <p class="intro faint">
      {{ t('把节点分成组，再在分流规则里指定某个网站走哪一组，例如 YouTube 走日本、Google 走美国。按地区或关键词建的组，订阅更新后会自动包含新节点。') }}
    </p>
    <p v-if="usedHint && policies.list.length" class="warn-line"><Icon name="info" :size="13" /> {{ t('分组代理只在「规则分流」模式下生效。') }}</p>

    <div v-if="policies.list.length" class="groups">
      <div v-for="p in policies.list" :key="p.id" class="group">
        <div class="g-head">
          <Flag v-if="p.match === 'country'" :code="p.country" :size="16" />
          <Icon v-else :name="p.match === 'keyword' ? 'search' : 'layers'" :size="15" class="faint" />
          <b class="ellipsis">{{ p.name }}</b>
          <span class="spacer" />
          <button class="btn ghost small icon" :title="t('编辑')" @click="edit(p)"><Icon name="edit" :size="13" /></button>
          <button class="btn ghost small icon danger" :title="t('删除')" @click="remove(p)"><Icon name="trash" :size="13" /></button>
        </div>
        <div class="chips">
          <span class="chip">{{ describe(p) }}</span>
          <span class="chip">{{ p.mode === 'auto' ? t('自动选最快') : t('手动选择') }}</span>
        </div>
        <select v-if="p.mode === 'select' && p.nodes.length" class="select pick" :value="current(p)?.id" @change="choose(p, Number(($event.target as HTMLSelectElement).value))">
          <option v-for="n in p.nodes" :key="n.id" :value="n.id">{{ bareName(n.name) }}{{ n.latency > 0 ? ` · ${n.latency} ms` : '' }}</option>
        </select>
        <div v-else-if="current(p)" class="now">
          <Flag :code="current(p)!.country" :size="14" />
          <span class="ellipsis">{{ bareName(current(p)!.name) }}</span>
          <span class="lat" :class="latencyClass(current(p)!.latency)">{{ latencyText(current(p)!.latency) }}</span>
        </div>
        <div v-else class="now faint">{{ p.nodes.length ? t('连接后自动选择') : t('没有符合条件的节点') }}</div>
        <div class="g-foot faint">
          <span>{{ t('{0} 个节点', p.nodes.length) }}</span>
          <span>·</span>
          <span :class="{ unused: !p.rules }">{{ p.rules ? t('用于 {0} 条规则', p.rules) : t('还没有规则使用') }}</span>
        </div>
      </div>
    </div>
    <div v-else class="empty-groups">
      <Icon name="layers" :size="28" :stroke="1.5" />
      <div>{{ t('还没有分组代理') }}</div>
      <button class="btn primary small" @click="edit(null)"><Icon name="plus" :size="13" /> {{ t('新建分组') }}</button>
    </div>

    <PolicyEditor v-model="editorOpen" :policy="editing" />
  </Card>
</template>

<style scoped>
.intro {
  margin: 0 0 10px;
  font-size: 12px;
  line-height: 1.7;
}
.warn-line {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 10px;
  font-size: 12px;
  color: var(--mid);
}
.groups {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 10px;
}
.group {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 11px 12px;
  border-radius: var(--radius);
  border: 1px solid var(--line);
  background: var(--card);
  min-width: 0;
}
.g-head {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.chip {
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--hover);
  color: var(--text-2);
  border: 1px solid var(--line);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.now {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 7px;
  height: 30px;
  padding: 0 9px;
  border-radius: var(--radius-sm);
  background: var(--hover);
  font-size: 12px;
}
.now.faint {
  display: flex;
}
.pick {
  height: 30px;
  font-size: 12px;
}
.g-foot {
  display: flex;
  gap: 5px;
  font-size: 11px;
}
.unused {
  color: var(--mid);
}
.empty-groups {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 18px;
  border: 1px dashed var(--line);
  border-radius: var(--radius);
  color: var(--text-3);
}
</style>
