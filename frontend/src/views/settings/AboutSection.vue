<script setup lang="ts">
// 关于: the product with the update check, the versions of NextBox and its
// core, where NextBox keeps its files, and the components it is built from.
import { t } from '../../i18n'
import { computed, onMounted, ref } from 'vue'
import Card from '../../components/Card.vue'
import Icon from '../../components/Icon.vue'
import Switch from '../../components/Switch.vue'
import PawLogo from '../../components/PawLogo.vue'
import { api, type CoreInfo, type Settings } from '../../api'
import { state } from '../../state'
import { toast } from '../../util'

const props = defineProps<{ form: Settings }>()
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
      name: 'NextBox', what: t('客户端'), current: 'v' + (state.app?.version ?? ''), channel: '',
      status: status(state.update?.app), latest: state.update?.app.latest ?? '', url: state.update?.app.url ?? '',
    },
    {
      name: 'sing-box', what: t('代理内核'), current: core.value?.found ? core.value.version : '', channel: core.value?.channel ?? '',
      status: core.value?.found && state.update?.core.current ? status(state.update.core) : '',
      latest: state.update?.core.latest ?? '', url: state.update?.core.url ?? '',
    },
  ]
})
// the build tags of the core: what it can do
const coreTags = computed(() => (core.value?.tags ?? '').split(',').map((s) => s.trim()).filter(Boolean))

const runtime = computed(() => [
  { label: t('数据目录'), value: state.app?.dataDir ?? '', open: () => api.openDataDir() },
  { label: t('内核位置'), value: state.app?.corePath ?? '', open: () => api.openCoreDir() },
  { label: t('运行权限'), value: state.app?.elevated ? t('管理员') : t('普通用户') },
  { label: t('服务模式'), value: state.app?.serviceReady ? t('已安装，可用于 TUN 模式') : t('未安装') },
])

const links = [
  { label: t('官网'), url: 'https://nextbox.efah.dev/', icon: 'nodes' },
  { label: 'GitHub', url: 'https://github.com/Efah-Dev/NextBox', icon: 'link' },
  { label: t('Telegram 频道'), url: 'https://t.me/nextboxofficial', icon: 'send' },
  { label: t('问题反馈'), url: 'https://github.com/Efah-Dev/NextBox/issues', icon: 'info' },
  { label: t('更新日志'), url: 'https://github.com/Efah-Dev/NextBox/releases', icon: 'logs' },
]

// keep in sync with THIRD_PARTY_NOTICES.md
const parts = [
  { name: 'sing-box', what: t('代理内核'), license: 'GPL-3.0', url: 'https://github.com/SagerNet/sing-box' },
  { name: 'NekoBox for PC', what: t('原项目'), license: 'GPL-3.0', url: 'https://github.com/MatsuriDayo/nekoray' },
  { name: 'Wails', what: t('桌面框架'), license: 'MIT', url: 'https://github.com/wailsapp/wails' },
  { name: 'Vue', what: t('界面'), license: 'MIT', url: 'https://github.com/vuejs/core' },
  { name: 'sing-geosite / sing-geoip', what: t('规则集'), license: t('见上游仓库'), url: 'https://github.com/SagerNet/sing-geosite' },
  { name: 'Noto Sans SC', what: t('界面字体'), license: 'OFL-1.1', url: 'https://github.com/notofonts/noto-cjk' },
  { name: 'Twemoji Country Flags', what: t('国旗字体'), license: 'CC-BY-4.0', url: 'https://github.com/talkjs/country-flag-emoji-polyfill' },
  { name: 'fyne systray', what: t('托盘图标'), license: 'Apache-2.0', url: 'https://github.com/fyne-io/systray' },
]
</script>

<template>
  <div class="about">
    <Card id="sec-about">
      <div class="hero">
        <span class="logo"><PawLogo :size="44" /></span>
        <div class="id">
          <div class="name">NextBox <span class="badge">v{{ state.app?.version }}</span></div>
          <p class="faint">{{ t('基于 NekoBox for PC（nekoray）修改而来的代理客户端，内核采用 sing-box。') }}</p>
          <div class="links">
            <a v-for="l in links" :key="l.url" class="chip" :title="l.url" @click="api.openURL(l.url)"><Icon :name="l.icon" :size="13" />{{ l.label }}</a>
          </div>
        </div>
        <button class="btn primary check" :disabled="checking" @click="checkUpdate">
          <Icon name="refresh" :size="14" :class="{ spin: checking }" /> {{ checking ? t('正在检查…') : t('检查更新') }}
        </button>
      </div>
      <div class="auto">
        <span>{{ t('自动检查更新') }}<span class="faint">{{ t('每天检查一次是否有新版本') }}</span></span>
        <Switch v-model="form.autoCheckUpdate" />
      </div>
    </Card>

    <div class="grid">
      <Card :title="t('版本')">
        <div v-for="v in versionRows" :key="v.name" class="kv">
          <div class="k">
            <b>{{ v.name }}</b>
            <span class="faint">{{ v.what }}</span>
          </div>
          <div class="v">
            <span class="mono cur">{{ v.current || t('未找到') }}</span>
            <span v-if="v.channel" class="chan" :class="{ testing: v.channel === 'testing' }">{{ v.channel }}</span>
            <a v-if="v.status === 'new'" class="new" @click="api.openURL(v.url)">{{ t('发现新版本 {0}', v.latest) }} <Icon name="chevron" :size="11" /></a>
            <span v-else-if="v.status === 'latest'" class="ok"><Icon name="check" :size="12" :stroke="2.6" />{{ t('已是最新版本') }}</span>
            <span v-else-if="v.status === 'failed'" class="warn">{{ t('未能获取最新版本') }}</span>
            <span v-else class="faint">{{ v.current ? t('尚未检查') : '' }}</span>
          </div>
        </div>
        <div v-if="core?.environment" class="kv">
          <div class="k"><b>{{ t('构建环境') }}</b></div>
          <div class="v"><span class="mono faint">{{ core.environment }}</span></div>
        </div>
        <div v-if="coreTags.length" class="tags">
          <span v-for="tag in coreTags" :key="tag" class="tag mono">{{ tag }}</span>
        </div>
      </Card>

      <Card :title="t('运行环境')">
        <div v-for="r in runtime" :key="r.label" class="kv">
          <div class="k"><b>{{ r.label }}</b></div>
          <div class="v">
            <span class="ellipsis" :class="{ mono: !!r.open }" :title="r.value">{{ r.value }}</span>
            <button v-if="r.open" class="btn ghost small icon" :title="t('打开文件夹')" @click="r.open"><Icon name="folder" :size="14" /></button>
          </div>
        </div>
      </Card>
    </div>

    <Card :title="t('组件与许可')">
      <div class="parts">
        <a v-for="p in parts" :key="p.name" class="part" :title="p.url" @click="api.openURL(p.url)">
          <span class="p-name">{{ p.name }}</span>
          <span class="faint p-what">{{ p.what }}</span>
          <span class="lic">{{ p.license }}</span>
        </a>
      </div>
      <p class="license faint">{{ t('NextBox 以 GPL-3.0-or-later 协议开源。所用第三方组件的许可证见程序目录里的 THIRD_PARTY_NOTICES.md。') }}</p>
    </Card>
  </div>
</template>

<style scoped>
.about {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.hero {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 6px 2px 2px;
}
.logo {
  width: 72px;
  height: 72px;
  flex-shrink: 0;
  border-radius: 20px;
  background: color-mix(in srgb, #ff5c9a 10%, var(--card));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, #ff5c9a 14%, transparent);
  display: grid;
  place-items: center;
}
.id {
  flex: 1;
  min-width: 0;
}
.name {
  font-size: 20px;
  font-weight: 800;
  display: flex;
  align-items: center;
  gap: 8px;
}
.id p {
  margin: 2px 0 8px;
  font-size: 12.5px;
}
.check {
  flex-shrink: 0;
  align-self: flex-start;
}
.links {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid var(--line);
  color: var(--text-2);
  font-size: 12px;
  cursor: pointer;
  transition: border-color var(--t-fast), color var(--t-fast), background var(--t-fast);
}
.chip:hover {
  border-color: var(--accent-line);
  color: var(--accent);
  background: var(--accent-soft);
}
.auto {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px dashed var(--line);
}
.auto > span .faint {
  margin-left: 8px;
  font-size: 12px;
}

.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}
.grid > :deep(.card) {
  min-width: 0;
}
.kv {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 40px;
  padding: 6px 0;
  border-bottom: 1px dashed var(--line);
}
.kv:last-child {
  border-bottom: 0;
}
.k {
  width: 92px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
}
.k .faint {
  font-size: 11px;
}
.v {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  font-size: 12.5px;
}
.v .mono {
  font-size: 12px;
}
.cur {
  font-weight: 700;
  color: var(--text);
}
.chan {
  font-size: 11px;
  font-weight: 700;
  padding: 1px 7px;
  border-radius: 999px;
  color: var(--good);
  background: color-mix(in srgb, var(--good) 12%, var(--card));
}
.chan.testing {
  color: var(--mid);
  background: color-mix(in srgb, var(--mid) 14%, var(--card));
}
.ok {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: var(--good);
}
.warn {
  color: var(--mid);
}
.new {
  color: var(--accent);
  font-weight: 700;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
}
.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding-top: 8px;
}
.tag {
  font-size: 10.5px;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--hover);
  color: var(--text-2);
}

.parts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px 14px;
}
.part {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  grid-template-rows: auto auto;
  column-gap: 10px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line);
  cursor: pointer;
  transition: border-color var(--t-fast), background var(--t-fast);
}
.part:hover {
  border-color: var(--accent-line);
  background: var(--accent-soft);
}
.p-name {
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.p-what {
  grid-row: 2;
  font-size: 11.5px;
}
.lic {
  grid-row: 1 / 3;
  grid-column: 2;
  align-self: center;
  font: 600 11px var(--mono);
  color: var(--text-2);
}
.license {
  margin: 12px 0 0;
  font-size: 12px;
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
