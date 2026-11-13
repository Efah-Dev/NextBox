<script setup lang="ts">
// 关于, compact: identity, both versions side by side with the update check,
// the links as a row of chips, and the licence line.
import { t } from '../../i18n'
import { computed, onMounted, ref } from 'vue'
import Card from '../../components/Card.vue'
import Icon from '../../components/Icon.vue'
import Switch from '../../components/Switch.vue'
import PawLogo from '../../components/PawLogo.vue'
import { api, type CoreInfo, type Settings } from '../../api'
import { state } from '../../state'
import { toast } from '../../util'

const props = defineProps<{ form: Settings; index: number }>()
const form = props.form

const core = ref<CoreInfo>()
onMounted(async () => (core.value = await api.getCoreInfo().catch(() => undefined)))

const checking = ref(false)
async function checkUpdate() {
  checking.value = true
  try {
    state.update = await api.checkUpdate()
  } catch (e) {
    toast((e as Error).message, 'error')
  } finally {
    checking.value = false
  }
}

const versionRows = computed(() => {
  const status = (item?: { latest: string; hasUpdate: boolean }) =>
    !item ? '' : item.hasUpdate ? 'new' : item.latest ? 'latest' : 'failed'
  return [
    {
      name: 'NextBox', current: 'v' + (state.app?.version ?? ''), channel: '',
      status: status(state.update?.app), latest: state.update?.app.latest ?? '', url: state.update?.app.url ?? '',
    },
    {
      name: 'sing-box', current: core.value?.found ? core.value.version : '', channel: core.value?.channel ?? '',
      status: core.value?.found && state.update?.core.current ? status(state.update.core) : '',
      latest: state.update?.core.latest ?? '', url: state.update?.core.url ?? '',
    },
  ]
})

const links = [
  { label: t('官网'), url: 'https://nextbox.efah.dev/', icon: 'nodes' },
  { label: t('项目地址'), url: 'https://github.com/Efah-Dev/NextBox', icon: 'link' },
  { label: t('原项目'), url: 'https://github.com/MatsuriDayo/nekoray', icon: 'link' },
  { label: t('规则集'), url: 'https://github.com/SagerNet/sing-geosite', icon: 'route' },
]
</script>

<template>
  <Card id="sec-about" :title="t('关于')" :index="index">
    <template #actions>
      <button class="btn small" :disabled="checking" @click="checkUpdate">
        <Icon name="refresh" :size="13" :class="{ spin: checking }" /> {{ checking ? t('正在检查…') : t('检查更新') }}
      </button>
    </template>
    <div class="hero">
      <span class="logo"><PawLogo :size="30" /></span>
      <div class="id">
        <div class="name">NextBox <span class="badge">v{{ state.app?.version }}</span></div>
        <p class="faint">{{ t('基于 NekoBox for PC（nekoray）修改而来的现代化代理客户端，内核采用 sing-box。') }}</p>
      </div>
    </div>

    <div class="versions">
      <div v-for="v in versionRows" :key="v.name" class="version">
        <b>{{ v.name }} <span class="mono cur">{{ v.current || t('未找到') }}</span></b>
        <span class="state">
          <span v-if="v.channel" class="chan" :class="{ testing: v.channel === 'testing' }">{{ v.channel }}</span>
          <a v-if="v.status === 'new'" class="new" @click="api.openURL(v.url)">{{ t('发现新版本 {0}', v.latest) }} <Icon name="chevron" :size="11" /></a>
          <span v-else-if="v.status === 'latest'" class="ok"><Icon name="check" :size="12" :stroke="2.6" />{{ t('已是最新版本') }}</span>
          <span v-else-if="v.status === 'failed'" class="warn">{{ t('未能获取最新版本') }}</span>
          <span v-else-if="v.current" class="faint">{{ t('尚未检查') }}</span>
          <span v-else class="faint mono ellipsis" :title="state.app?.corePath">{{ state.app?.corePath }}</span>
        </span>
      </div>
    </div>

    <div class="auto" :title="t('启动后检查一次，之后每天一次；只与 GitHub 上的正式版比对，不会自动下载')">
      <span>{{ t('自动检查更新') }}<span class="faint">{{ t('每天一次，不会自动下载') }}</span></span>
      <Switch v-model="form.autoCheckUpdate" />
    </div>

    <div class="links">
      <a v-for="l in links" :key="l.url" class="chip" :title="l.url" @click="api.openURL(l.url)"><Icon :name="l.icon" :size="13" />{{ l.label }}</a>
    </div>
    <p class="license faint">{{ t('NextBox 以 GPL-3.0-or-later 协议开源。所用第三方组件的许可证见程序目录里的 THIRD_PARTY_NOTICES.md。') }}</p>
  </Card>
</template>

<style scoped>
.hero {
  display: flex;
  align-items: center;
  gap: 12px;
}
.logo {
  width: 46px;
  height: 46px;
  flex-shrink: 0;
  border-radius: 13px;
  background: color-mix(in srgb, #ff5c9a 10%, var(--card));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, #ff5c9a 14%, transparent);
  display: grid;
  place-items: center;
}
.id {
  min-width: 0;
}
.name {
  font-size: 16px;
  font-weight: 800;
  display: flex;
  align-items: center;
  gap: 8px;
}
.id p {
  margin: 2px 0 0;
  font-size: 12px;
}
.versions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  margin-top: 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius);
}
.version {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 9px 12px;
  min-width: 0;
}
.version + .version {
  border-left: 1px solid var(--line);
}
.cur {
  font-weight: 500;
  color: var(--text-2);
  margin-left: 4px;
}
.state {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  min-width: 0;
}
.chan {
  padding: 0 6px;
  border-radius: 4px;
  font-weight: 700;
  background: color-mix(in srgb, var(--good) 14%, var(--card));
  color: var(--good);
}
.chan.testing {
  background: color-mix(in srgb, var(--mid) 14%, var(--card));
  color: var(--mid);
}
.ok {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--good);
  font-weight: 600;
}
.warn {
  color: var(--mid);
}
.new {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: var(--good);
  font-weight: 700;
  cursor: pointer;
}
.new:hover {
  text-decoration: underline;
}
.auto {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
}
.auto > span .faint {
  margin-left: 8px;
  font-size: 11.5px;
}
.links {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 12px;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 26px;
  padding: 0 11px;
  border-radius: 999px;
  border: 1px solid var(--line);
  color: var(--accent);
  font-size: 12px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.chip:hover {
  border-color: var(--accent-line);
  background: var(--accent-soft);
}
.license {
  margin: 10px 0 0;
  font-size: 11.5px;
  line-height: 1.6;
}
.spin {
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
