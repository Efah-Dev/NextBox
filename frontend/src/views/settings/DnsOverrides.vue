<script setup lang="ts">
// DNS 复写 (Settings.dnsOverrides): some domains get their addresses from a
// DNS server of their own (an airport's DNS that unlocks streaming sites) or
// fixed addresses (like a hosts file), before the remote and direct DNS.
import { computed, ref } from 'vue'
import { t } from '../../i18n'
import type { DnsOverride } from '../../api'
import Icon from '../../components/Icon.vue'
import HelpTip from '../../components/HelpTip.vue'
import Modal from '../../components/Modal.vue'
import Segmented from '../../components/Segmented.vue'
import Switch from '../../components/Switch.vue'
import ContextMenu, { type CtxItem } from '../../components/ContextMenu.vue'

const list = defineModel<DnsOverride[] | undefined>({ required: true })

// ---- editor
const editing = ref(-1) // index, list length = a new one
const open = ref(false)
const kind = ref<'server' | 'ip'>('server')
const domains = ref('')
const server = ref('')
const viaProxy = ref(true)
const ips = ref('')

const split = (s: string) => s.split(/[\s,，、;；]+/).map((x) => x.trim()).filter(Boolean)

function edit(i: number) {
  const o = list.value?.[i]
  editing.value = o ? i : (list.value?.length ?? 0)
  kind.value = o?.ips?.length ? 'ip' : 'server'
  domains.value = (o?.domains ?? []).join('\n')
  server.value = o?.server ?? ''
  viaProxy.value = o ? !!o.viaProxy : true
  ips.value = (o?.ips ?? []).join('\n')
  open.value = true
}

const problem = computed(() => {
  if (!split(domains.value).length) return t('至少填一个域名')
  if (kind.value === 'server' && !server.value.trim()) return t('填写 DNS 服务器地址')
  if (kind.value === 'ip' && !split(ips.value).length) return t('至少填一个 IP')
  return ''
})

function save() {
  if (problem.value) return
  const o: DnsOverride =
    kind.value === 'server'
      ? { domains: split(domains.value), server: server.value.trim(), viaProxy: viaProxy.value }
      : { domains: split(domains.value), ips: split(ips.value) }
  const next = [...(list.value ?? [])]
  const old = next[editing.value]
  if (old?.disabled) o.disabled = true
  next.splice(editing.value, old ? 1 : 0, o)
  list.value = next
  open.value = false
}

// ---- the ⋯ menu of a row
const menu = ref<{ x: number; y: number; i: number } | null>(null)
const menuItems = computed<CtxItem[]>(() => {
  const i = menu.value?.i ?? -1
  const o = list.value?.[i]
  if (!o) return []
  return [
    { label: t('编辑'), icon: 'edit', action: () => edit(i) },
    { label: o.disabled ? t('启用') : t('停用'), icon: o.disabled ? 'eye' : 'eye-off', action: () => toggle(i) },
    { sep: true },
    { label: t('删除'), icon: 'trash', danger: true, action: () => remove(i) },
  ]
})
function openMenu(e: MouseEvent, i: number) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  menu.value = { x: r.right - 200, y: r.bottom + 4, i }
}
function toggle(i: number) {
  const next = [...(list.value ?? [])]
  next[i] = { ...next[i], disabled: !next[i].disabled || undefined }
  list.value = next
}
function remove(i: number) {
  const next = [...(list.value ?? [])]
  next.splice(i, 1)
  list.value = next
}
</script>

<template>
  <div class="form-row top">
    <div class="label">
      <span>{{ t('DNS 复写') }}<HelpTip :text="t('指定的域名改用这里填写的 DNS 服务器解析，或直接返回填写的 IP。优先于远程 DNS 和直连 DNS。')" /></span>
      <span class="hint">{{ t('按域名指定 DNS 服务器或 IP') }}</span>
    </div>
    <div class="control">
      <button class="btn" @click="edit(-1)"><Icon name="plus" :size="14" /> {{ t('添加') }}</button>
    </div>
  </div>
  <div v-if="list?.length" class="ov-list">
    <div v-for="(o, i) in list" :key="i" class="ov" :class="{ off: o.disabled }" @dblclick="edit(i)">
      <span class="ov-domains" :title="o.domains.join('\n')">
        <span class="mono ellipsis">{{ o.domains.slice(0, 2).join(', ') }}</span>
        <span v-if="o.domains.length > 2" class="chip plain">{{ t('等 {0} 个', o.domains.length) }}</span>
      </span>
      <Icon name="chevron" :size="12" class="arrow" />
      <span class="ov-to">
        <span class="chip">{{ o.ips?.length ? 'IP' : 'DNS' }}</span>
        <span class="mono ellipsis">{{ o.ips?.length ? o.ips.join(', ') : o.server }}</span>
        <span v-if="!o.ips?.length" class="faint small">{{ o.viaProxy ? t('经代理') : t('直连') }}</span>
        <span v-if="o.disabled" class="faint small">{{ t('已停用') }}</span>
      </span>
      <button class="btn ghost small icon" :title="t('更多')" @click="openMenu($event, i)"><Icon name="more" :size="14" :stroke="3" /></button>
    </div>
  </div>
  <ContextMenu v-if="menu" :items="menuItems" :x="menu.x" :y="menu.y" @close="menu = null" />

  <Modal v-model="open" :title="editing < (list?.length ?? 0) ? t('编辑 DNS 复写') : t('添加 DNS 复写')" :width="460">
    <div class="ed">
      <Segmented
        v-model="kind"
        :options="[
          { value: 'server', label: t('用指定的 DNS 服务器') },
          { value: 'ip', label: t('固定 IP') },
        ]"
      />
      <label class="field">
        <span>{{ kind === 'server' ? t('域名（含子域名）') : t('完整域名') }}</span>
        <textarea v-model="domains" class="input mono" rows="3" spellcheck="false" :placeholder="kind === 'server' ? 'netflix.com\nnflxvideo.net' : 'nas.home.arpa'" />
        <span class="faint small">{{ t('每行一个，也可以用逗号分开') }}</span>
      </label>
      <template v-if="kind === 'server'">
        <label class="field">
          <span>{{ t('DNS 服务器') }}</span>
          <input v-model="server" class="input mono" spellcheck="false" placeholder="10.8.0.53 / https://dns.example.com/dns-query" />
        </label>
        <div class="switch-row">
          <span>{{ t('经代理查询') }}<span class="faint small">{{ t('机场提供的解锁 DNS 一般需要打开') }}</span></span>
          <Switch v-model="viaProxy" />
        </div>
      </template>
      <label v-else class="field">
        <span>IP</span>
        <textarea v-model="ips" class="input mono" rows="2" spellcheck="false" placeholder="192.168.1.20" />
      </label>
    </div>
    <template #footer>
      <span class="warn small">{{ problem }}</span>
      <button class="btn" @click="open = false">{{ t('取消') }}</button>
      <button class="btn primary" :disabled="!!problem" @click="save">{{ t('保存') }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.ov-list {
  margin: 0 0 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius);
  overflow: hidden;
}
.ov {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 12px minmax(0, 1fr) 28px;
  align-items: center;
  gap: 12px;
  padding: 8px 8px 8px 12px;
  border-bottom: 1px solid var(--line);
}
.ov:last-child {
  border-bottom: 0;
}
.ov.off .ov-domains,
.ov.off .ov-to {
  opacity: 0.5;
}
.ov-domains,
.ov-to {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.arrow {
  color: var(--text-3);
}
.chip {
  flex: none;
  height: 20px;
  padding: 0 8px;
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  background: var(--accent-soft);
  color: var(--accent);
}
.chip.plain {
  background: var(--hover);
  color: var(--text-2);
}
.small {
  font-size: 11px;
  white-space: nowrap;
}
.ed {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.field textarea {
  height: auto;
  padding: 8px 10px;
  resize: vertical;
}
.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.switch-row > span {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.warn {
  margin-right: auto;
  color: var(--mid);
}
</style>
