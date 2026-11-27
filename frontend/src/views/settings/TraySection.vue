<script setup lang="ts">
// 托盘菜单: which parts the right-click menu of the tray icon shows
// (Settings.trayItems; none = all). The menu itself is drawn by
// internal/app/traymenu.html.
import { t } from '../../i18n'
import Card from '../../components/Card.vue'
import Switch from '../../components/Switch.vue'
import type { Settings } from '../../api'

const props = defineProps<{ form: Settings }>()

// keep in sync with trayParts in internal/app/tray.go
const parts = [
  { id: 'quick', label: t('系统代理与 TUN 开关') },
  { id: 'mode', label: t('路由模式') },
  { id: 'nodes', label: t('订阅与节点'), hint: t('每个分组一行，点开后选择节点') },
  { id: 'policies', label: t('分组代理'), hint: t('选择用哪个分组代理连接') },
  { id: 'latency', label: t('显示延迟') },
]

const shown = (id: string) => !props.form.trayItems || props.form.trayItems.includes(id)
function toggle(id: string, on: boolean) {
  const now = parts.map((p) => p.id).filter(shown)
  const next = on ? [...now, id] : now.filter((x) => x !== id)
  props.form.trayItems = next.length === parts.length ? undefined : next
}
</script>

<template>
  <Card id="sec-tray" :title="t('托盘菜单')">
    <div class="form-row">
      <div class="label">{{ t('连接状态与开关') }}<span class="hint">{{ t('始终显示') }}</span></div>
      <div class="control"><Switch :model-value="true" disabled /></div>
    </div>
    <div v-for="p in parts" :key="p.id" class="form-row">
      <div class="label">{{ p.label }}<span v-if="p.hint" class="hint">{{ p.hint }}</span></div>
      <div class="control"><Switch :model-value="shown(p.id)" @update:model-value="toggle(p.id, $event)" /></div>
    </div>
    <p class="faint tip">{{ t('左键单击托盘图标打开主界面，右键打开这个菜单。') }}</p>
  </Card>
</template>

<style scoped>
.tip {
  margin: 8px 0 0;
  font-size: 12px;
}
</style>
