<script setup lang="ts">
// TUN 排除网段: IP ranges that stay out of the TUN interface, e.g. the
// networks of Tailscale or ZeroTier, which only their own adapters reach.
import Reveal from '../../motion/Reveal.vue'
import { t } from '../../i18n'
import { computed, ref, watch } from 'vue'
import Icon from '../../components/Icon.vue'
import { api, type Adapter, type Settings } from '../../api'

const props = defineProps<{ form: Settings }>()
const form = props.form

const bad = ref<number[]>([])
watch(
  () => form.tunExclude,
  async (v) => (bad.value = v?.trim() ? await api.checkCIDRs(v).catch(() => []) : []),
  { immediate: true },
)

const listed = computed(() => new Set((form.tunExclude ?? '').split('\n').map((l) => l.split('#')[0].trim().toLowerCase())))

const adapters = ref<Adapter[] | null>(null)
const open = ref(false)
async function toggle() {
  open.value = !open.value
  if (open.value) adapters.value = await api.listAdapters().catch(() => [])
}
function add(net: string) {
  const text = (form.tunExclude ?? '').replace(/\s+$/, '')
  form.tunExclude = text ? text + '\n' + net : net
}
</script>

<template>
  <div class="form-row top">
    <div class="label">
      {{ t('TUN 排除网段') }}<span class="hint">{{ t('这些网段绕过 TUN，走系统路由') }}</span>
    </div>
    <div class="control col">
      <textarea
        v-model.lazy="form.tunExclude"
        class="textarea mono"
        rows="3"
        spellcheck="false"
        :placeholder="t('每行一个 IP 段或 IP，例如 Tailscale 的 100.64.0.0/10')"
      />
      <div v-if="bad.length" class="bad">
        <Icon name="warn" :size="12" />
        {{ t('第 {0} 行不是有效的 IP 段或 IP，已忽略', bad.join('、')) }}
      </div>
      <div class="row">
        <button class="btn ghost small" @click="toggle">
          <Icon name="plus" :size="12" /> {{ t('从网卡添加') }} <Icon :name="open ? 'up' : 'down'" :size="12" />
        </button>
        <span class="faint small">{{ t('用 Tailscale、ZeroTier 等组网软件时，把它们的网段加进来') }}</span>
      </div>
      <Reveal :open="open">
      <div class="adapters">
        <div v-if="adapters === null" class="faint small pad">{{ t('正在读取网卡…') }}</div>
        <div v-else-if="!adapters.length" class="faint small pad">{{ t('没有找到其他网卡') }}</div>
        <template v-for="a in adapters ?? []" :key="a.name">
          <div v-for="n in a.networks" :key="a.name + n" class="adapter">
            <span class="ellipsis" :title="a.name">{{ a.name }}</span>
            <span class="mono faint">{{ n }}</span>
            <span v-if="listed.has(n.toLowerCase())" class="added"><Icon name="check" :size="12" /> {{ t('已添加') }}</span>
            <button v-else class="btn ghost small" @click="add(n)">{{ t('添加') }}</button>
          </div>
        </template>
      </div>
      </Reveal>
    </div>
  </div>
</template>

<style scoped>
.form-row > .control.col {
  flex-direction: column;
  align-items: stretch;
  gap: 6px;
}
.col .textarea {
  width: 100%;
  min-height: 64px;
  font-size: 12px;
  line-height: 1.6;
}
.row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.small {
  font-size: 11.5px;
}
.bad {
  color: var(--bad);
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.adapters {
  max-width: 520px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  overflow: hidden;
}
.adapter {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto 64px;
  align-items: center;
  gap: 10px;
  padding: 5px 6px 5px 10px;
  font-size: 12px;
}
.adapter + .adapter {
  border-top: 1px solid var(--line);
}
.adapter .btn,
.added {
  justify-self: end;
}
.added {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: var(--good);
  font-weight: 600;
  font-size: 11.5px;
  padding-right: 6px;
}
.pad {
  padding: 8px 10px;
}
</style>
