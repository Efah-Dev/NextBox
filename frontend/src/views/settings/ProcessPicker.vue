<script setup lang="ts">
// Picking programs for a program rule: the programs running now, with a
// search box and check boxes, or a program file chosen with the file dialog
// (for one that is not running). Emits the file names ("chrome.exe"), which
// is what program rules match.
import { computed, ref, watch } from 'vue'
import { t } from '../../i18n'
import Modal from '../../components/Modal.vue'
import Icon from '../../components/Icon.vue'
import { api, type RunningProgram } from '../../api'
import { toast } from '../../util'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ have?: string[] }>()
const emit = defineEmits<{ pick: [names: string[]] }>()

const list = ref<RunningProgram[]>([])
const loading = ref(false)
const query = ref('')
const picked = ref<string[]>([])

async function load() {
  loading.value = true
  list.value = await api.runningPrograms().catch((e) => {
    toast((e as Error).message, 'error')
    return []
  })
  loading.value = false
}
watch(open, (v) => {
  if (!v) return
  query.value = ''
  picked.value = []
  load()
})

const haveSet = computed(() => new Set((props.have ?? []).map((v) => v.toLowerCase())))
const shown = computed(() => {
  const q = query.value.trim().toLowerCase()
  return list.value.filter((p) => !q || p.name.toLowerCase().includes(q) || p.path.toLowerCase().includes(q))
})
const isPicked = (name: string) => picked.value.includes(name)
function toggle(name: string) {
  if (haveSet.value.has(name.toLowerCase())) return
  picked.value = isPicked(name) ? picked.value.filter((n) => n !== name) : [...picked.value, name]
}

async function browse() {
  const name = await api.pickProgram().catch((e) => {
    toast((e as Error).message, 'error')
    return ''
  })
  if (name && !isPicked(name) && !haveSet.value.has(name.toLowerCase())) picked.value = [...picked.value, name]
}

function done() {
  emit('pick', picked.value)
  open.value = false
}
</script>

<template>
  <Modal v-model="open" :title="t('选择程序')" :width="520">
    <p class="faint tip">{{ t('勾选正在运行的程序；不在运行的程序可以点「浏览」选择它的 .exe 文件。规则按程序文件名匹配。') }}</p>
    <div class="tools">
      <div class="search">
        <Icon name="search" :size="14" />
        <input v-model="query" class="input" :placeholder="t('搜索程序名或路径')" spellcheck="false" />
      </div>
      <button class="btn small" :disabled="loading" :title="t('重新读取正在运行的程序')" @click="load"><Icon name="refresh" :size="13" /></button>
      <button class="btn small" @click="browse"><Icon name="folder" :size="13" /> {{ t('浏览…') }}</button>
    </div>
    <div class="list scroll">
      <div v-if="loading" class="faint none">{{ t('正在读取…') }}</div>
      <template v-else>
        <label
          v-for="p in shown"
          :key="p.name"
          class="row"
          :class="{ on: isPicked(p.name), have: haveSet.has(p.name.toLowerCase()) }"
          :title="p.path || p.name"
        >
          <input type="checkbox" :checked="isPicked(p.name) || haveSet.has(p.name.toLowerCase())" :disabled="haveSet.has(p.name.toLowerCase())" @change="toggle(p.name)" />
          <b class="mono">{{ p.name }}</b>
          <span class="faint ellipsis path">{{ haveSet.has(p.name.toLowerCase()) ? t('已在规则里') : p.path }}</span>
        </label>
        <div v-if="!shown.length" class="faint none">{{ t('没有找到程序') }}</div>
      </template>
    </div>
    <div v-if="picked.length" class="picked faint">{{ t('已选择：') }}<span class="mono">{{ picked.join('、') }}</span></div>

    <template #footer>
      <button class="btn" @click="open = false">{{ t('取消') }}</button>
      <button class="btn primary" :disabled="!picked.length" @click="done">{{ picked.length ? t('添加 {0} 个', picked.length) : t('添加') }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.tip {
  margin: 0 0 10px;
  font-size: 12px;
  line-height: 1.6;
}
.tools {
  display: flex;
  gap: 6px;
  margin-bottom: 8px;
}
.search {
  flex: 1;
  position: relative;
  display: flex;
  align-items: center;
}
.search :deep(svg) {
  position: absolute;
  left: 10px;
  color: var(--text-3);
}
.search .input {
  width: 100%;
  padding-left: 30px;
}
.list {
  height: 300px;
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 4px;
}
.row {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 12.5px;
}
.row:hover {
  background: var(--hover);
}
.row.on {
  background: var(--accent-soft);
}
.row.have {
  cursor: default;
  opacity: 0.6;
}
.row input {
  accent-color: var(--accent);
}
.path {
  font-size: 11.5px;
}
.none {
  padding: 16px;
  text-align: center;
}
.picked {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.6;
}
</style>
