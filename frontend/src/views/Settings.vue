<script setup lang="ts">
import { languages, t } from '../i18n'
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import Card from '../components/Card.vue'
import Icon from '../components/Icon.vue'
import Switch from '../components/Switch.vue'
import Segmented from '../components/Segmented.vue'
import RuleEditor from '../components/RuleEditor.vue'
import PawLogo from '../components/PawLogo.vue'
import { api, type CoreInfo, type Settings } from '../api'
import { state } from '../state'
import { confirmDialog, formatBytes, run, toast } from '../util'

const sections = [
  { id: 'appearance', label: t('外观'), icon: 'palette' },
  { id: 'general', label: t('常规'), icon: 'settings' },
  { id: 'inbound', label: t('入站'), icon: 'inbound' },
  { id: 'route', label: t('路由规则'), icon: 'route' },
  { id: 'dns', label: 'DNS', icon: 'dns' },
  { id: 'tun', label: t('TUN 模式'), icon: 'tun' },
  { id: 'test', label: t('订阅与测速'), icon: 'zap' },
  { id: 'logs', label: t('日志'), icon: 'logs' },
  { id: 'about', label: t('关于'), icon: 'info' },
]
const active = ref('appearance')
const scroller = ref<HTMLElement>()

const accents = ['#F472B6', '#1370F3', '#0EA5E9', '#10B981', '#8B5CF6', '#F97316', '#E5484D', '#475569']
const testUrls = [
  { label: t('Google（推荐）'), value: 'https://www.google.com/generate_204' },
  { label: 'Cloudflare', value: 'https://cp.cloudflare.com/generate_204' },
]
const mirrors = [
  { label: t('本地内置（绕过中国域名和 IP）'), value: 'local' },
  { label: 'jsDelivr（Cloudflare）', value: 'https://testingcf.jsdelivr.net/gh/SagerNet/sing-{kind}@rule-set/{name}.srs' },
  { label: 'jsDelivr', value: 'https://cdn.jsdelivr.net/gh/SagerNet/sing-{kind}@rule-set/{name}.srs' },
  { label: 'GitHub Raw', value: 'https://raw.githubusercontent.com/SagerNet/sing-{kind}/rule-set/{name}.srs' },
]

const form = reactive<Settings>({ ...(state.app!.settings) })
let timer = 0
let pending = false

const sameAsSaved = () => JSON.stringify(form) === JSON.stringify(state.app?.settings)

// Auto-save, like a launcher: every change is persisted shortly after it is made.
watch(form, () => {
  clearTimeout(timer)
  pending = !sameAsSaved()
  if (pending) timer = window.setTimeout(save, 350)
})

// Pick up changes made elsewhere (backend normalisation, other pages).
watch(
  () => state.app?.settings,
  (s) => {
    if (s && !pending && !sameAsSaved()) Object.assign(form, s)
  },
)

async function save() {
  pending = false
  if (sameAsSaved()) return
  try {
    const next = { ...form }
    state.app!.settings = next
    await api.saveSettings(next)
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

const keepOptions = [
  { value: 1, label: t('1 天') },
  { value: 3, label: t('3 天') },
  { value: 7, label: t('7 天') },
  { value: 30, label: t('30 天') },
  { value: 0, label: t('永久保留') },
]

const updateHours = [
  { value: 1, label: t('每 {0} 小时', 1) },
  { value: 6, label: t('每 {0} 小时', 6) },
  { value: 12, label: t('每 {0} 小时', 12) },
  { value: 24, label: t('每天') },
  { value: 72, label: t('每 {0} 天', 3) },
  { value: 168, label: t('每 {0} 天', 7) },
]

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
// one row per component: what is installed and what the last check found
const versionRows = computed(() => {
  const status = (item?: { latest: string; hasUpdate: boolean }) =>
    !item ? '' : item.hasUpdate ? 'new' : item.latest ? 'latest' : 'failed'
  return [
    {
      name: 'NextBox', kind: 'app', current: 'v' + (state.app?.version ?? ''), channel: '',
      status: status(state.update?.app), latest: state.update?.app.latest ?? '', url: state.update?.app.url ?? '',
    },
    {
      name: t('sing-box 内核'), kind: 'core', current: core.value?.found ? core.value.version : '', channel: core.value?.channel ?? '',
      status: core.value?.found && state.update?.core.current ? status(state.update.core) : '',
      latest: state.update?.core.latest ?? '', url: state.update?.core.url ?? '',
    },
  ]
})

const aboutLinks = [
  { label: t('官网'), text: 'nextbox.efah.dev', url: 'https://nextbox.efah.dev/', icon: 'nodes' },
  { label: t('项目地址'), text: 'Efah-Dev/NextBox', url: 'https://github.com/Efah-Dev/NextBox', icon: 'link' },
  { label: t('原项目'), text: 'MatsuriDayo/nekoray', url: 'https://github.com/MatsuriDayo/nekoray', icon: 'link' },
  { label: t('规则集'), text: 'SagerNet/sing-geosite · sing-geoip', url: 'https://github.com/SagerNet/sing-geosite', icon: 'route' },
]

const core = ref<CoreInfo>()
onMounted(async () => (core.value = await api.getCoreInfo().catch(() => undefined)))

const logFiles = ref({ count: 0, bytes: 0 })
const refreshLogFiles = async () => (logFiles.value = await api.getLogFilesInfo().catch(() => ({ count: 0, bytes: 0, dir: '' })))
onMounted(refreshLogFiles)

async function clearLogFiles() {
  if (!(await confirmDialog(t('清除日志文件'), t('确定删除全部 {0} 个日志文件（{1}）吗？', logFiles.value.count, formatBytes(logFiles.value.bytes)), true))) return
  await run(() => api.clearLogFiles(), (n) => t('已删除 {0} 个日志文件', n))
  refreshLogFiles()
}

function jump(id: string) {
  active.value = id
  document.getElementById('sec-' + id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// Opened from another page for a specific section (e.g. Nodes → 测速设置).
onMounted(() => {
  const id = state.settingsSection
  if (!id) return
  state.settingsSection = ''
  active.value = id
  nextTick(() => document.getElementById('sec-' + id)?.scrollIntoView({ block: 'start' }))
})

function onScroll() {
  const el = scroller.value
  if (!el) return
  // the last sections are too short to reach the top, so at the very bottom
  // highlight the last one
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 4) {
    active.value = sections[sections.length - 1].id
    return
  }
  for (const s of [...sections].reverse()) {
    const node = document.getElementById('sec-' + s.id)
    if (node && node.offsetTop - el.offsetTop <= el.scrollTop + 40) {
      active.value = s.id
      return
    }
  }
}

</script>

<template>
  <div class="page">
    <aside class="side">
      <div>
        <div class="side-title">{{ t('设置') }}</div>
        <div class="nav-list">
          <div v-for="s in sections" :key="s.id" class="nav-item" :class="{ active: active === s.id }" @click="jump(s.id)">
            <Icon :name="s.icon" :size="16" />
            <span>{{ s.label }}</span>
          </div>
        </div>
      </div>
      <div class="spacer" />
      <div class="faint saved"><Icon name="check" :size="13" /> {{ t('修改会自动保存并生效') }}</div>
    </aside>

    <main ref="scroller" class="content scroll" @scroll="onScroll">
      <Card id="sec-appearance" :title="t('外观')" :index="0">
        <div class="form-row">
          <div class="label">{{ t('语言') }}<span class="hint">Language / 語言</span></div>
          <div class="control">
            <Segmented v-model="form.language" style="width: 300px" :options="languages" />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('主题') }}</div>
          <div class="control">
            <Segmented
              v-model="form.theme"
              style="width: 240px"
              :options="[
                { value: 'auto', label: t('跟随系统') },
                { value: 'light', label: t('浅色') },
                { value: 'dark', label: t('深色') },
              ]"
            />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('主题色') }}</div>
          <div class="control">
            <button
              v-for="c in accents"
              :key="c"
              class="swatch"
              :class="{ on: form.accentColor.toLowerCase() === c.toLowerCase() }"
              :style="{ background: c }"
              @click="form.accentColor = c"
            />
            <input v-model.lazy="form.accentColor" type="color" class="color" :title="t('自定义颜色')" />
          </div>
        </div>
      </Card>

      <Card id="sec-general" :title="t('常规')" :index="1">
        <div class="form-row">
          <div class="label">{{ t('启动时自动连接') }}</div>
          <div class="control"><Switch v-model="form.connectOnStart" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('开机自启') }}<span class="hint">{{ t('启动 Windows 后自动运行') }}</span></div>
          <div class="control"><Switch v-model="form.autoStart" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('关闭时隐藏到托盘') }}<span class="hint">{{ t('关闭后代理继续在后台运行') }}</span></div>
          <div class="control"><Switch v-model="form.closeToTray" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('识别连接进程') }}<span class="hint">{{ t('连接页显示程序名，略增开销') }}</span></div>
          <div class="control"><Switch v-model="form.findProcess" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('数据目录') }}</div>
          <div class="control">
            <span class="mono faint ellipsis">{{ state.app?.dataDir }}</span>
            <button class="btn small" @click="run(() => api.openDataDir())"><Icon name="folder" :size="13" /> {{ t('打开') }}</button>
          </div>
        </div>
      </Card>

      <Card id="sec-inbound" :title="t('入站')" :index="2">
        <div class="form-row">
          <div class="label">{{ t('连接时设置系统代理') }}<span class="hint">{{ t('开启 TUN 模式时不使用') }}</span></div>
          <div class="control"><Switch v-model="form.systemProxy" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('监听地址') }}</div>
          <div class="control"><input v-model.lazy="form.listenAddress" class="input mono" style="width: 200px" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('混合代理端口') }}<span class="hint">{{ t('HTTP 与 SOCKS5 共用') }}</span></div>
          <div class="control"><input v-model.lazy.number="form.mixedPort" type="number" class="input mono" style="width: 120px" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('允许局域网连接') }}<span class="hint">{{ t('同一网络的设备无需密码即可使用') }}</span></div>
          <div class="control"><Switch v-model="form.allowLan" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('流量嗅探') }}<span class="hint">{{ t('识别 TLS / HTTP 域名') }}</span></div>
          <div class="control"><Switch v-model="form.sniff" /></div>
        </div>
      </Card>

      <Card id="sec-route" :title="t('路由规则')" :index="3">
        <div class="form-row">
          <div class="label">{{ t('路由模式') }}</div>
          <div class="control">
            <Segmented
              v-model="form.routeMode"
              style="width: 300px"
              :options="[
                { value: 'rule', label: t('规则分流') },
                { value: 'global', label: t('全局代理') },
                { value: 'direct', label: t('全部直连') },
              ]"
            />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('屏蔽广告') }}</div>
          <div class="control"><Switch v-model="form.blockAds" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('规则集镜像') }}</div>
          <div class="control">
            <select v-model="form.ruleSetMirror" class="select">
              <option v-for="m in mirrors" :key="m.value" :value="m.value">{{ m.label }}</option>
              <option v-if="!mirrors.some((m) => m.value === form.ruleSetMirror)" :value="form.ruleSetMirror">{{ t('自定义') }}</option>
            </select>
            <button class="btn" @click="run(() => api.updateRuleSets(), (m) => m)"><Icon name="download" /> {{ t('更新规则集') }}</button>
          </div>
        </div>
        <div class="form-row top">
          <div class="label">{{ t('自定义规则') }}<span class="hint">{{ t('规则分流和全局代理模式下生效') }}</span></div>
          <div class="control">
            <RuleEditor v-model:proxy="form.proxyRules" v-model:direct="form.directRules" v-model:block="form.blockRules" />
          </div>
        </div>
      </Card>

      <Card id="sec-dns" title="DNS" :index="4">
        <div class="form-row">
          <div class="label">{{ t('远程 DNS') }}<span class="hint">{{ t('经代理查询') }}</span></div>
          <div class="control"><input v-model.lazy="form.remoteDns" class="input mono" placeholder="https://1.1.1.1/dns-query" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('直连 DNS') }}<span class="hint">{{ t('也用于解析节点域名') }}</span></div>
          <div class="control"><input v-model.lazy="form.directDns" class="input mono" placeholder="223.5.5.5 / local" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('启用 IPv6') }}</div>
          <div class="control"><Switch v-model="form.enableIpv6" /></div>
        </div>
        <p class="faint tip">
          {{ t('支持 223.5.5.5、tcp://、tls://、quic://、https://、h3:// 地址，以及 local（系统 DNS）。') }}
        </p>
      </Card>

      <Card id="sec-tun" :title="t('TUN 模式')" :index="5">
        <div v-if="!state.app?.elevated" class="warn">
          <Icon name="warn" :size="16" />
          <span>{{ t('TUN 模式需要管理员权限。') }}</span>
          <button class="btn small" @click="run(() => api.restartAsAdmin())">{{ t('以管理员身份重启') }}</button>
        </div>
        <div class="form-row">
          <div class="label">{{ t('启用 TUN') }}</div>
          <div class="control"><Switch v-model="form.tunEnabled" :disabled="!state.app?.elevated" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('网络栈') }}</div>
          <div class="control">
            <Segmented
              v-model="form.tunStack"
              style="width: 240px"
              :options="[
                { value: 'mixed', label: 'Mixed' },
                { value: 'system', label: 'System' },
                { value: 'gvisor', label: 'gVisor' },
              ]"
            />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('严格路由') }}<span class="hint">{{ t('防止 DNS 泄露') }}</span></div>
          <div class="control"><Switch v-model="form.tunStrictRoute" /></div>
        </div>
        <div class="form-row">
          <div class="label">MTU</div>
          <div class="control"><input v-model.lazy.number="form.tunMtu" type="number" class="input mono" style="width: 120px" /></div>
        </div>
      </Card>

      <Card id="sec-test" :title="t('订阅与测速')" :index="6">
        <div class="form-row">
          <div class="label">{{ t('测速地址') }}</div>
          <div class="control">
            <select v-model="form.testUrl" class="select" style="width: 260px">
              <option v-for="u in testUrls" :key="u.value" :value="u.value">{{ u.label }}</option>
              <option v-if="!testUrls.some((u) => u.value === form.testUrl)" :value="form.testUrl">{{ form.testUrl }}</option>
            </select>
            <input v-model.lazy="form.testUrl" class="input mono" :placeholder="t('或输入自定义地址')" />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('超时（毫秒）') }}</div>
          <div class="control"><input v-model.lazy.number="form.testTimeout" type="number" class="input mono" style="width: 120px" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('并发数') }}</div>
          <div class="control"><input v-model.lazy.number="form.testConcurrency" type="number" class="input mono" style="width: 120px" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('订阅 User-Agent') }}</div>
          <div class="control"><input v-model.lazy="form.subUserAgent" class="input mono" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('通过代理更新订阅') }}<span class="hint">{{ t('仅在已连接时生效') }}</span></div>
          <div class="control"><Switch v-model="form.subUpdateByProxy" /></div>
        </div>
        <div class="form-row">
          <div class="label">
            {{ t('节点地区') }}
            <span class="hint">{{ form.countrySource === 'server' ? t('按服务器 IP：本地查询，中转节点会显示入口地区') : t('按落地 IP：URL 测试时顺便检测，最准确') }}</span>
          </div>
          <div class="control">
            <Segmented
              v-model="form.countrySource"
              :options="[
                { value: 'exit', label: t('落地 IP') },
                { value: 'server', label: t('服务器 IP') },
              ]"
            />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('自动更新订阅') }}<span class="hint">{{ t('NextBox 运行期间在后台更新') }}</span></div>
          <div class="control">
            <select v-if="form.subAutoUpdate" v-model.number="form.subUpdateHours" class="select" style="width: 160px">
              <option v-for="h in updateHours" :key="h.value" :value="h.value">{{ h.label }}</option>
            </select>
            <Switch v-model="form.subAutoUpdate" />
          </div>
        </div>
      </Card>

      <Card id="sec-logs" :title="t('日志')" :index="7">
        <div class="form-row">
          <div class="label">{{ t('日志级别') }}<span class="hint">{{ t('debug 最详细，warn 只记录问题') }}</span></div>
          <div class="control">
            <select v-model="form.logLevel" class="select" style="width: 160px">
              <option v-for="l in ['debug', 'info', 'warn', 'error']" :key="l" :value="l">{{ l }}</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('日志页最多显示') }}<span class="hint">{{ t('条数越多越占内存') }}</span></div>
          <div class="control">
            <select v-model.number="form.logViewLimit" class="select" style="width: 160px">
              <option v-for="n in [200, 500, 1000, 2000]" :key="n" :value="n">{{ t('{0} 条', n) }}</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('记录日志到文件') }}<span class="hint">{{ t('按启动时间命名，方便排查问题') }}</span></div>
          <div class="control"><Switch v-model="form.logToFile" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('单个文件上限') }}<span class="hint">{{ t('写满后自动新建文件') }}</span></div>
          <div class="control">
            <select v-model.number="form.logFileMaxMb" class="select" style="width: 160px">
              <option v-for="n in [1, 5, 10, 20, 50]" :key="n" :value="n">{{ n }} MB</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('保留时间') }}<span class="hint">{{ t('启动时自动删除更早的文件') }}</span></div>
          <div class="control">
            <select v-model.number="form.logKeepDays" class="select" style="width: 160px">
              <option v-for="d in keepOptions" :key="d.value" :value="d.value">{{ d.label }}</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('日志文件') }}</div>
          <div class="control">
            <span class="faint">{{ t('{0} 个文件，共 {1}', logFiles.count, formatBytes(logFiles.bytes)) }}</span>
            <span class="spacer" />
            <button class="btn small" @click="run(() => api.openLogDir())"><Icon name="folder" :size="13" /> {{ t('打开文件夹') }}</button>
            <button class="btn small danger" :disabled="!logFiles.count" @click="clearLogFiles"><Icon name="trash" :size="13" /> {{ t('全部清除') }}</button>
          </div>
        </div>
      </Card>

      <Card id="sec-about" :title="t('关于')" :index="8">
        <div class="about-hero">
          <span class="about-logo"><PawLogo :size="46" /></span>
          <div class="about-id">
            <div class="about-name">NextBox <span class="badge ver">v{{ state.app?.version }}</span></div>
            <p class="about-desc">{{ t('基于 NekoBox for PC（nekoray）修改而来的现代化代理客户端，内核采用 sing-box。') }}</p>
          </div>
        </div>

        <section class="about-block">
          <div class="block-head">
            <span class="block-title">{{ t('版本与更新') }}</span>
            <button class="btn small" :disabled="checking" @click="checkUpdate">
              <Icon name="refresh" :size="13" :class="{ spin: checking }" /> {{ checking ? t('正在检查…') : t('检查更新') }}
            </button>
          </div>
          <div class="versions">
            <div v-for="v in versionRows" :key="v.name" class="version">
              <span class="version-icon" :class="v.kind"><PawLogo v-if="v.kind === 'app'" :size="20" /><Icon v-else name="route" :size="17" /></span>
              <div class="version-id">
                <b>{{ v.name }}</b>
                <span class="faint">
                  <template v-if="v.current">{{ v.current }}</template>
                  <template v-else>{{ t('未找到内核：{0}', state.app?.corePath) }}</template>
                  <span v-if="v.channel" class="badge channel" :class="{ testing: v.channel === 'testing' }">
                    {{ v.channel === 'testing' ? t('testing 测试分支') : t('stable 稳定分支') }}
                  </span>
                </span>
              </div>
              <span v-if="v.status === 'new'" class="status new">
                {{ t('发现新版本 {0}', v.latest) }}
                <a @click="api.openURL(v.url)">{{ t('查看发布页') }} <Icon name="chevron" :size="12" /></a>
              </span>
              <span v-else-if="v.status === 'latest'" class="status ok"><Icon name="check" :size="13" :stroke="2.6" />{{ t('已是最新版本') }}</span>
              <span v-else-if="v.status === 'failed'" class="status warn">{{ t('未能获取最新版本') }}</span>
              <span v-else-if="v.current" class="status idle">{{ t('尚未检查') }}</span>
            </div>
          </div>
          <div class="auto-check">
            <div>
              <div>{{ t('自动检查更新') }}</div>
              <div class="faint hint">{{ t('启动后检查一次，之后每天一次；只与 GitHub 上的正式版比对，不会自动下载') }}</div>
            </div>
            <Switch v-model="form.autoCheckUpdate" />
          </div>
        </section>

        <section class="about-block">
          <div class="block-head"><span class="block-title">{{ t('链接') }}</span></div>
          <div class="links">
            <a v-for="l in aboutLinks" :key="l.url" class="link" @click="api.openURL(l.url)">
              <span class="link-icon"><Icon :name="l.icon" :size="15" /></span>
              <span class="link-text">
                <span class="faint">{{ l.label }}</span>
                <b>{{ l.text }}</b>
              </span>
              <Icon class="link-go" name="chevron" :size="14" />
            </a>
          </div>
        </section>

        <p class="license faint">{{ t('NextBox 以 GPL-3.0-or-later 协议开源。所用第三方组件的许可证见程序目录里的 THIRD_PARTY_NOTICES.md。') }}</p>
      </Card>
    </main>
  </div>
</template>

<style scoped>
.saved {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  padding: 0 4px;
}
.content {
  scroll-padding-top: 18px;
}
.swatch {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 2px solid var(--card);
  box-shadow: 0 0 0 1px var(--line);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
}
.swatch:hover {
  transform: scale(1.12);
}
.swatch.on {
  box-shadow: 0 0 0 2px var(--accent);
}
.color {
  width: 30px;
  height: 26px;
  border: none;
  background: none;
  padding: 0;
  cursor: pointer;
}
.textarea {
  min-height: 100px;
}
.tip {
  margin: 8px 0 0;
  font-size: 12px;
}
.warn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  margin-bottom: 6px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--mid) 12%, var(--card));
  color: var(--mid);
}
.warn span {
  flex: 1;
}
/* ---- about ---- */
.about-hero {
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 4px 2px 6px;
}
.about-logo {
  width: 72px;
  height: 72px;
  flex-shrink: 0;
  border-radius: 20px;
  background: color-mix(in srgb, #ff5c9a 10%, var(--card));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, #ff5c9a 14%, transparent);
  display: grid;
  place-items: center;
}
.about-id {
  min-width: 0;
}
.about-name {
  font-size: 20px;
  font-weight: 800;
  display: flex;
  align-items: center;
  gap: 10px;
}
.about-desc {
  margin: 6px 0 0;
  color: var(--text-2);
  line-height: 1.7;
}
.badge.ver {
  font-size: 12px;
}

.about-block {
  margin-top: 22px;
}
.block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 28px;
  margin-bottom: 10px;
}
.block-title {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.5px;
  color: var(--text-3);
}

.versions {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--card-border);
  border-radius: var(--radius);
  overflow: hidden;
}
.version {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
}
.version + .version {
  border-top: 1px solid var(--line);
}
.version-icon {
  width: 38px;
  height: 38px;
  flex-shrink: 0;
  border-radius: 11px;
  display: grid;
  place-items: center;
  background: var(--accent-soft);
  color: var(--accent);
}
.version-icon.core {
  background: color-mix(in srgb, #6d5cff 12%, var(--card));
  color: #6d5cff;
}
.version-id {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.version-id b {
  font-size: 14px;
}
.version-id .faint {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
}
.badge.channel {
  background: color-mix(in srgb, var(--good) 14%, var(--card));
  color: var(--good);
}
.badge.channel.testing {
  background: color-mix(in srgb, var(--mid) 14%, var(--card));
  color: var(--mid);
}
.status {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
}
.status.ok {
  color: var(--good);
}
.status.idle {
  color: var(--text-3);
  font-weight: 500;
}
.status.warn {
  color: var(--mid);
}
.status.new {
  padding: 5px 6px 5px 12px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--good) 12%, var(--card));
  color: var(--good);
}
.status.new a {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--card);
  color: var(--accent);
  cursor: pointer;
}
.status.new a:hover {
  text-decoration: underline;
}
.auto-check {
  margin-top: 10px;
  padding: 12px 16px;
  border-radius: var(--radius);
  background: var(--hover);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.auto-check .hint {
  margin-top: 3px;
  font-size: 11px;
  line-height: 1.5;
}

.links {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.link {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: var(--radius);
  border: 1px solid var(--card-border);
  background: var(--card);
  color: inherit;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.link:hover {
  border-color: var(--accent-line);
  background: var(--accent-soft);
}
.link-icon {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 9px;
  display: grid;
  place-items: center;
  background: var(--hover);
  color: var(--accent);
}
.link-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
  font-size: 12px;
}
.link-text b {
  font-size: 13px;
  font-weight: 600;
  color: var(--accent);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.link-go {
  color: var(--text-3);
  transition: transform 0.15s, color 0.15s;
}
.link:hover .link-go {
  color: var(--accent);
  transform: translateX(2px);
}
.spin {
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
.license {
  margin: 22px 0 2px;
  font-size: 12px;
  line-height: 1.6;
}
</style>
