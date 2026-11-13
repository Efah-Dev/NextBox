<script setup lang="ts">
// Creates or edits a policy group: which nodes it holds (by country, by
// name keywords, or picked by hand) and how it chooses among them.
import { t } from '../../i18n'
import { computed, ref, watch } from 'vue'
import Modal from '../../components/Modal.vue'
import Segmented from '../../components/Segmented.vue'
import Flag from '../../components/Flag.vue'
import Icon from '../../components/Icon.vue'
import { api, type Policy, type Profile } from '../../api'
import { bareName, countryName, nodeCountry } from '../../country'
import { settings, state } from '../../state'
import { groupName, latencyClass, latencyText, toast } from '../../util'
import { refreshPolicies } from '../../policies'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ policy: Policy | null }>()

const blank = (): Policy => ({ id: 0, name: '', match: 'country', country: '', keyword: '', members: [], groupId: 0, mode: 'auto' })
const form = ref<Policy>(blank())
const all = ref<Profile[]>([])
const search = ref('')
const saving = ref(false)

watch(open, async (o) => {
  if (!o) return
  form.value = props.policy ? { ...blank(), ...props.policy, members: [...(props.policy.members ?? [])] } : blank()
  search.value = ''
  all.value = await api.getProfiles(0).catch(() => [])
  if (!form.value.country && countries.value.length) form.value.country = countries.value[0].code
})

const inScope = computed(() => all.value.filter((p) => !form.value.groupId || p.groupId === form.value.groupId))
const codeOf = (p: Profile) => nodeCountry(p, settings.value).code

// countries present among the nodes, most nodes first
const countries = computed(() => {
  const n = new Map<string, number>()
  for (const p of inScope.value) {
    const c = codeOf(p)
    if (c) n.set(c, (n.get(c) ?? 0) + 1)
  }
  return [...n.entries()].sort((a, b) => b[1] - a[1]).map(([code, count]) => ({ code, count }))
})

const keywords = computed(() => (form.value.keyword ?? '').toLowerCase().split(/[\s,，|]+/).filter(Boolean))

const matched = computed(() => {
  const f = form.value
  if (f.match === 'manual') {
    const byId = new Map(all.value.map((p) => [p.id, p]))
    return (f.members ?? []).map((id) => byId.get(id)).filter(Boolean) as Profile[]
  }
  return inScope.value.filter((p) =>
    f.match === 'country' ? !!f.country && codeOf(p) === f.country : keywords.value.some((k) => p.name.toLowerCase().includes(k)),
  )
})

const pickable = computed(() => {
  const q = search.value.trim().toLowerCase()
  return inScope.value.filter((p) => !q || p.name.toLowerCase().includes(q))
})
const isMember = (id: number) => form.value.members?.includes(id)
function toggleMember(id: number) {
  const m = form.value.members ?? []
  form.value.members = m.includes(id) ? m.filter((x) => x !== id) : [...m, id]
}

// a sensible default name
watch(
  () => [form.value.match, form.value.country, form.value.keyword, form.value.mode],
  (_, old) => {
    if (form.value.id || (form.value.name && form.value.name !== suggested(old as any))) return
    form.value.name = suggested()
  },
)
function suggested(o?: [string, string, string, string]) {
  const [match, country, keyword, mode] = o ?? [form.value.match, form.value.country, form.value.keyword, form.value.mode]
  const how = mode === 'auto' ? t('自动') : t('手选')
  if (match === 'country' && country) return `${countryName(country)} ${how}`
  if (match === 'keyword' && keyword) return `${keyword.trim()} ${how}`
  return ''
}

async function save() {
  saving.value = true
  try {
    const p = { ...form.value }
    if (p.match !== 'manual') p.members = []
    if (p.match !== 'country') p.country = ''
    if (p.match !== 'keyword') p.keyword = ''
    await api.savePolicy(p)
    await refreshPolicies()
    toast(t('已保存分组代理“{0}”', p.name), 'success')
    open.value = false
  } catch (e) {
    toast((e as Error).message, 'error')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Modal v-model="open" :title="policy ? t('编辑分组代理') : t('新建分组代理')" :width="560">
    <div class="form-row">
      <div class="label">{{ t('名称') }}</div>
      <div class="control"><input v-model="form.name" class="input" :placeholder="t('例如：日本 自动')" /></div>
    </div>
    <div class="form-row">
      <div class="label">{{ t('选择节点') }}</div>
      <div class="control">
        <Segmented
          v-model="form.match"
          style="width: 300px"
          :options="[
            { value: 'country', label: t('按地区') },
            { value: 'keyword', label: t('按关键词') },
            { value: 'manual', label: t('手动挑选') },
          ]"
        />
      </div>
    </div>
    <div class="form-row">
      <div class="label">{{ t('范围') }}</div>
      <div class="control">
        <select v-model.number="form.groupId" class="select">
          <option :value="0">{{ t('全部分组') }}</option>
          <option v-for="g in state.app?.groups ?? []" :key="g.id" :value="g.id">{{ groupName(g) }}</option>
        </select>
      </div>
    </div>
    <div v-if="form.match === 'country'" class="form-row">
      <div class="label">{{ t('地区') }}<span class="hint">{{ t('订阅更新后自动包含新节点') }}</span></div>
      <div class="control">
        <select v-model="form.country" class="select">
          <option v-for="c in countries" :key="c.code" :value="c.code">{{ countryName(c.code) }}（{{ c.count }}）</option>
        </select>
        <Flag :code="form.country" :size="18" />
      </div>
    </div>
    <div v-else-if="form.match === 'keyword'" class="form-row">
      <div class="label">{{ t('关键词') }}<span class="hint">{{ t('名称包含任意一个即可，空格分隔') }}</span></div>
      <div class="control"><input v-model="form.keyword" class="input" :placeholder="t('例如：IPLC 洛杉矶')" /></div>
    </div>
    <div class="form-row">
      <div class="label">{{ t('选用方式') }}</div>
      <div class="control">
        <Segmented
          v-model="form.mode"
          style="width: 220px"
          :options="[
            { value: 'auto', label: t('自动选最快') },
            { value: 'select', label: t('手动选择') },
          ]"
        />
      </div>
    </div>
    <p class="faint mode-hint">
      {{ form.mode === 'auto' ? t('每 10 分钟测一次延迟，自动用最快的节点；当前节点不可用时会换到别的。') : t('使用你选定的节点，可以在分组卡片或托盘里随时切换。') }}
    </p>

    <div class="members">
      <div class="members-head">
        <b>{{ form.match === 'manual' ? t('挑选节点') : t('包含的节点') }}</b>
        <span class="faint">{{ t('{0} 个', matched.length) }}</span>
        <span class="spacer" />
        <input v-if="form.match === 'manual'" v-model="search" class="input mini" :placeholder="t('搜索')" />
      </div>
      <div class="member-list scroll">
        <template v-if="form.match === 'manual'">
          <label v-for="p in pickable" :key="p.id" class="member pick">
            <input type="checkbox" :checked="isMember(p.id)" @change="toggleMember(p.id)" />
            <Flag :code="codeOf(p)" :size="14" />
            <span class="ellipsis">{{ bareName(p.name) }}</span>
            <span class="lat" :class="latencyClass(p.latency)">{{ latencyText(p.latency) }}</span>
          </label>
        </template>
        <template v-else>
          <div v-for="p in matched.slice(0, 100)" :key="p.id" class="member">
            <Flag :code="codeOf(p)" :size="14" />
            <span class="ellipsis">{{ bareName(p.name) }}</span>
            <span class="lat" :class="latencyClass(p.latency)">{{ latencyText(p.latency) }}</span>
          </div>
          <div v-if="!matched.length" class="faint none"><Icon name="info" :size="14" /> {{ t('没有符合条件的节点') }}</div>
        </template>
      </div>
      <p v-if="matched.length > 100" class="faint small">{{ t('最多使用前 100 个节点') }}</p>
    </div>

    <template #footer>
      <button class="btn" @click="open = false">{{ t('取消') }}</button>
      <button class="btn primary" :disabled="saving || !form.name.trim() || !matched.length" @click="save">{{ t('保存') }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.mode-hint {
  margin: 0 0 8px;
  font-size: 12px;
  line-height: 1.6;
}
.members {
  margin-top: 6px;
}
.members-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.input.mini {
  height: 28px;
  width: 150px;
  font-size: 12px;
}
.member-list {
  max-height: 210px;
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 4px;
}
.member {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: var(--radius-sm);
}
.member.pick {
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  cursor: pointer;
}
.member:hover {
  background: var(--hover);
}
.member input {
  accent-color: var(--accent);
}
.none {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 12px;
}
.small {
  font-size: 11.5px;
  margin: 6px 0 0;
}
</style>
