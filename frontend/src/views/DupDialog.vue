<script setup lang="ts">
// 删除重复节点: lists the nodes of all groups that connect the same way as
// another one (same server, port, password and so on, any name), which one
// of each is kept, and removes the copies once confirmed.
import { computed, ref, watch } from 'vue'
import { t } from '../i18n'
import Modal from '../components/Modal.vue'
import { api, type DupSet } from '../api'
import { state } from '../state'
import { groupName, run, typeName } from '../util'

const open = defineModel<boolean>({ required: true })
const sets = ref<DupSet[] | null>(null)
const busy = ref(false)

watch(open, async (v) => {
  if (!v) return
  sets.value = null
  sets.value = (await run(() => api.findDuplicates())) ?? []
})

const dropCount = computed(() => (sets.value ?? []).reduce((n, d) => n + d.drop.length, 0))
const groupOf = (id: number) => groupName(state.app?.groups.find((g) => g.id === id))
const fromSub = (id: number) => !!state.app?.groups.find((g) => g.id === id)?.url
// copies in a subscription come back with its next update when the node is
// kept in another group
const comeBack = computed(() => (sets.value ?? []).some((d) => d.drop.some((p) => fromSub(p.groupId) && p.groupId !== d.keep.groupId)))

async function remove() {
  busy.value = true
  const n = await run(() => api.deleteDuplicates(), (n) => t('已删除 {0} 个重复节点', n))
  busy.value = false
  if (n !== undefined) open.value = false
}
</script>

<template>
  <Modal v-model="open" :title="t('删除重复节点')" :width="560">
    <div v-if="!sets" class="faint">{{ t('正在查找…') }}</div>
    <div v-else-if="!sets.length" class="none">
      <b>{{ t('没有重复的节点') }}</b>
      <span class="faint">{{ t('所有分组里的节点都各不相同。') }}</span>
    </div>
    <template v-else>
      <p class="lead">
        {{ t('找到 {0} 个重复节点：服务器、端口、密码等连接参数完全一样，只是名字不同。每组保留一个，其余删除。', dropCount) }}
      </p>
      <div class="sets">
        <div v-for="d in sets" :key="d.keep.id" class="set">
          <div class="row keep">
            <span class="badge">{{ d.running ? t('正在使用') : t('保留') }}</span>
            <span class="name ellipsis" :title="d.keep.name">{{ d.keep.name }}</span>
            <span class="meta faint ellipsis">{{ typeName(d.keep.type) }} · {{ groupOf(d.keep.groupId) }}</span>
          </div>
          <div v-for="p in d.drop" :key="p.id" class="row drop">
            <span class="badge del">{{ t('删除') }}</span>
            <span class="name ellipsis" :title="p.name">{{ p.name }}</span>
            <span class="meta faint ellipsis">{{ groupOf(p.groupId) }}</span>
          </div>
        </div>
      </div>
      <p class="faint small">{{ t('正在使用的节点不会被删除；分组代理和链式代理里用到被删节点的，会自动改用保留的那个。') }}</p>
      <p v-if="comeBack" class="faint small warn">{{ t('订阅里的节点在下次更新订阅时会重新出现。同一个订阅里的重复节点，更新时会自动去掉。') }}</p>
    </template>
    <template #footer>
      <button class="btn" @click="open = false">{{ sets?.length ? t('取消') : t('关闭') }}</button>
      <button v-if="sets?.length" class="btn danger solid" :disabled="busy" @click="remove">{{ t('删除 {0} 个', dropCount) }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.none {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 0;
}
.lead {
  margin: 0 0 10px;
  line-height: 1.6;
}
.sets {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 340px;
  overflow-y: auto;
}
.set {
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  overflow: hidden;
}
.row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) minmax(0, auto);
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  font-size: 12.5px;
}
.row + .row {
  border-top: 1px solid var(--line);
}
.drop .name {
  color: var(--text-3);
  text-decoration: line-through;
}
.meta {
  font-size: 11.5px;
  text-align: right;
}
.badge {
  font-size: 10.5px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
  color: var(--good);
  background: color-mix(in srgb, var(--good) 14%, var(--card));
}
.badge.del {
  color: var(--bad);
  background: color-mix(in srgb, var(--bad) 12%, var(--card));
}
.small {
  font-size: 11.5px;
  margin: 10px 0 0;
  line-height: 1.6;
}
.warn {
  color: var(--mid);
}
</style>
