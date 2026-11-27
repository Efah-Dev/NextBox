<script setup lang="ts">
// 这是 Clash 订阅，要一起导入哪些？ The nodes always come; proxy groups,
// rules and DNS settings are converted into NextBox's own settings and
// replaced on every update of the subscription.
import Reveal from '../motion/Reveal.vue'
import { computed, ref } from 'vue'
import { t } from '../i18n'
import Modal from './Modal.vue'
import Icon from './Icon.vue'
import { applyClash, clashDialog } from '../clashImport'

const p = computed(() => clashDialog.info?.preview)
const groups = computed(() => p.value?.groups ?? [])
const skipped = computed(() => p.value?.skipped ?? [])
const showSkipped = ref(false)

// a sing-box subscription brings only its DNS servers
const isClash = computed(() => !!(groups.value.length || p.value?.ruleTotal || p.value?.dns))
const subDns = computed(() => p.value?.subDns)
const subDnsLines = computed(() =>
  [subDns.value?.remote && t('远程 DNS：{0}', subDns.value.remote), subDns.value?.resolver && t('节点域名：{0}', subDns.value.resolver)].filter(Boolean) as string[],
)

const groupNames = computed(() => {
  const g = groups.value
  return g.slice(0, 3).join('、') + (g.length > 3 ? t(' 等') : '')
})
</script>

<template>
  <Modal v-model="clashDialog.open" :title="isClash ? t('这是 Clash 订阅，要一起导入哪些？') : t('这个订阅带有专属 DNS 服务器')" :width="520">
    <p class="lead">{{ t('节点总会导入。其余部分转换成 NextBox 自己的设置，以后随订阅更新；可以在分组菜单的「Clash 订阅内容」里重新选择。') }}</p>
    <div class="rows">
      <label class="row on">
        <input type="checkbox" checked disabled />
        <span class="what"><b>{{ t('节点') }}</b><span class="faint">{{ t('{0} 个', clashDialog.info?.nodes ?? 0) }}</span></span>
        <span class="faint note">{{ t('必选') }}</span>
      </label>
      <template v-if="isClash">
      <label class="row" :class="{ on: clashDialog.opt.groups, off: !groups.length }">
        <input v-model="clashDialog.opt.groups" type="checkbox" :disabled="!groups.length" />
        <span class="what">
          <b>{{ t('分组 → 分组代理') }}</b>
          <span class="faint">{{ groups.length ? t('{0} 个：{1}', groups.length, groupNames) : t('没有含本订阅节点的分组') }}</span>
        </span>
        <span class="faint note">{{ t('自动最快 / 手动') }}</span>
      </label>
      <label class="row" :class="{ on: clashDialog.opt.rules, off: !p?.rules }">
        <input v-model="clashDialog.opt.rules" type="checkbox" :disabled="!p?.rules" />
        <span class="what">
          <b>{{ t('规则 → 规则集') }}</b>
          <span class="faint">{{ t('{0} 条规则，按去向分成 {1} 个规则集；能转换 {2} 条', p?.ruleTotal ?? 0, p?.sets ?? 0, p?.rules ?? 0) }}</span>
        </span>
        <span class="faint note">{{ t('放在你自己的规则之后') }}</span>
      </label>
      <label class="row" :class="{ on: clashDialog.opt.dns, off: !p?.dns }">
        <input v-model="clashDialog.opt.dns" type="checkbox" :disabled="!p?.dns" />
        <span class="what">
          <b>{{ t('DNS 设置 → DNS 复写') }}</b>
          <span class="faint">{{ p?.dns ? t('{0} 条', p.dns) : t('订阅里没有可用的 DNS 设置') }}</span>
        </span>
      </label>
      </template>
      <label v-if="subDns" class="row" :class="{ on: clashDialog.opt.subDns }">
        <input v-model="clashDialog.opt.subDns" type="checkbox" />
        <span class="what">
          <b>{{ t('订阅自带的 DNS → 这个订阅的专属 DNS') }}</b>
          <span v-for="l in subDnsLines" :key="l" class="faint mono ellipsis" :title="l">{{ l }}</span>
        </span>
        <span class="faint note">{{ t('只用于这个订阅的节点') }}</span>
      </label>
    </div>
    <div v-if="skipped.length" class="skipped">
      <button class="link" @click="showSkipped = !showSkipped">
        <Icon name="chevron" :size="12" :class="{ open: showSkipped }" />
        {{ t('{0} 条规则无法转换，导入时会跳过', (p?.ruleTotal ?? 0) - (p?.rules ?? 0)) }}
      </button>
      <Reveal :open="showSkipped">
        <ul class="mono">
          <li v-for="s in skipped" :key="s">{{ s }}</li>
        </ul>
      </Reveal>
    </div>
    <template #footer>
      <button class="btn" @click="applyClash({ groups: false, rules: false, dns: false, subDns: false })">{{ t('只导入节点') }}</button>
      <button class="btn primary" @click="applyClash({ ...clashDialog.opt })">{{ t('导入') }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.lead {
  margin: 0 0 12px;
  color: var(--text-2);
  line-height: 1.6;
}
.rows {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--line);
  border-radius: var(--radius);
  overflow: hidden;
}
.row {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--line);
  cursor: pointer;
}
.row:last-child {
  border-bottom: 0;
}
.row.off {
  cursor: default;
  opacity: 0.55;
}
.row input {
  margin: 0;
  accent-color: var(--accent);
}
.what {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  font-size: 12px;
}
.what b {
  font-size: 13px;
}
.note {
  font-size: 11px;
  white-space: nowrap;
}
.skipped {
  margin-top: 10px;
  font-size: 12px;
}
.link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: 0;
  background: none;
  padding: 0;
  color: var(--text-2);
  font: inherit;
  cursor: pointer;
}
.link .open {
  transform: rotate(90deg);
}
.skipped ul {
  margin: 6px 0 0;
  padding-left: 18px;
  max-height: 140px;
  overflow: auto;
  color: var(--text-3);
  font-size: 11px;
}
</style>
