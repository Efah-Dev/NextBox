<script setup lang="ts">
// 订阅专属 DNS: the DNS a subscription names (found when it is added or
// updated) is used for its own nodes. Each subscription has a switch; a DNS
// of one's own can be typed in with 编辑, which few people need.
import { computed } from 'vue'
import { t } from '../../i18n'
import { api, type Group, type GroupDNS } from '../../api'
import { state } from '../../state'
import { groupName, promptDialog, run } from '../../util'
import Switch from '../../components/Switch.vue'

const subs = computed(() => (state.app?.groups ?? []).filter((g) => g.url))

const filled = (d?: GroupDNS | null) => !!(d && (d.remote || d.resolver))
const addrs = (d: GroupDNS) => [d.remote && t('远程 {0}', d.remote), d.resolver && t('节点域名 {0}', d.resolver)].filter(Boolean) as string[]

function status(g: Group): { text: string; lines: string[] } {
  if (filled(g.dns)) return { text: g.clash?.subDns ? t('使用订阅自带的 DNS') : t('手动设置'), lines: addrs(g.dns!) }
  if (filled(g.suggestedDns)) return { text: t('订阅提供了 DNS，未使用'), lines: addrs(g.suggestedDns!) }
  return { text: t('订阅没有提供，使用全局 DNS'), lines: [] }
}

async function toggle(g: Group, on: boolean) {
  if (on && !filled(g.suggestedDns)) return edit(g)
  await run(() => api.useSubscriptionDNS(g.id, on))
}

async function edit(g: Group) {
  const r = await promptDialog(
    t('订阅专属 DNS：{0}', groupName(g)),
    [
      { label: t('专属远程 DNS（留空用全局的远程 DNS）'), value: g.dns?.remote ?? '', placeholder: 'https://dns.example.com/dns-query' },
      { label: t('节点域名解析 DNS（留空用直连 DNS）'), value: g.dns?.resolver ?? '', placeholder: '10.0.0.53' },
    ],
    t('只用于这个订阅的节点：连接它的节点时代替远程 DNS，并用来解析它的节点地址。手动设置后，更新订阅不会再覆盖。'),
  )
  if (r) run(() => api.setGroupDNS(g.id, r[0] ?? '', r[1] ?? ''), t('已保存'))
}
</script>

<template>
  <div class="form-row top">
    <div class="label">
      {{ t('订阅专属 DNS') }}
      <span class="hint">{{ t('按订阅里的配置，只用于它的节点；添加或更新订阅时自动识别') }}</span>
    </div>
    <div class="control">
      <span v-if="!subs.length" class="faint">{{ t('还没有添加订阅') }}</span>
    </div>
  </div>
  <div v-if="subs.length" class="sd-list">
    <div v-for="g in subs" :key="g.id" class="sd">
      <span class="sd-name ellipsis" :title="groupName(g)">{{ groupName(g) }}</span>
      <span class="sd-info">
        <span :class="filled(g.dns) ? 'on' : 'faint'">{{ status(g).text }}</span>
        <span v-for="l in status(g).lines" :key="l" class="mono faint ellipsis" :title="l">{{ l }}</span>
      </span>
      <button class="btn ghost small" @click="edit(g)">{{ t('编辑') }}</button>
      <Switch v-if="filled(g.dns) || filled(g.suggestedDns)" :model-value="filled(g.dns)" @update:model-value="(v: boolean) => toggle(g, v)" />
      <span v-else />
    </div>
  </div>
</template>

<style scoped>
.sd-list {
  margin: 0 0 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius);
  overflow: hidden;
}
.sd {
  display: grid;
  grid-template-columns: minmax(0, 0.8fr) minmax(0, 2fr) auto 40px;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--line);
}
.sd:last-child {
  border-bottom: 0;
}
.sd-name {
  font-weight: 700;
}
.sd-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
  font-size: 12px;
}
.sd-info .on {
  color: var(--accent);
  font-weight: 700;
}
.sd-info .mono {
  font-size: 11px;
}
</style>
