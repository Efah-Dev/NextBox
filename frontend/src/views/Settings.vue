<script setup lang="ts">
import { languages, t } from '../i18n'
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import Card from '../components/Card.vue'
import Icon from '../components/Icon.vue'
import Switch from '../components/Switch.vue'
import Segmented from '../components/Segmented.vue'
import AccentPicker from './settings/AccentPicker.vue'
import DisplayLevels from './settings/DisplayLevels.vue'
import AutoStartRow from './settings/AutoStartRow.vue'
import RouteSection from './settings/RouteSection.vue'
import PolicySection from './settings/PolicySection.vue'
import BackupSection from './settings/BackupSection.vue'
import AboutSection from './settings/AboutSection.vue'
import { api, type Settings } from '../api'
import { state } from '../state'
import { confirmDialog, formatBytes, run, toast } from '../util'

// Sorted by how often people change them: everyday things first, rarely
// touched ones last.
const groups = [
  {
    label: t('基础'),
    items: [
      { id: 'general', label: t('常规'), icon: 'settings' },
      { id: 'appearance', label: t('外观'), icon: 'palette' },
    ],
  },
  {
    label: t('代理'),
    items: [
      { id: 'inbound', label: t('代理端口'), icon: 'inbound' },
      { id: 'tun', label: t('TUN 模式'), icon: 'tun' },
      { id: 'route', label: t('分流规则'), icon: 'route' },
      { id: 'policy', label: t('分组代理'), icon: 'layers' },
      { id: 'dns', label: 'DNS', icon: 'dns' },
    ],
  },
  { label: t('节点'), items: [{ id: 'test', label: t('订阅与测速'), icon: 'zap' }] },
  {
    label: t('数据'),
    items: [
      { id: 'backup', label: t('数据与备份'), icon: 'backup' },
      { id: 'logs', label: t('日志与诊断'), icon: 'logs' },
    ],
  },
  { label: '', items: [{ id: 'about', label: t('关于'), icon: 'info' }] },
]
const sections = groups.flatMap((g) => g.items)
const order = (id: string) => sections.findIndex((s) => s.id === id)
const active = ref('general')
const scroller = ref<HTMLElement>()

const testUrls = [
  { label: t('Google（推荐）'), value: 'https://www.google.com/generate_204' },
  { label: 'Cloudflare', value: 'https://cp.cloudflare.com/generate_204' },
]

// rules and scenes are arrays: the form must never share them with the
// saved settings, or a change would look saved before it is
const clone = (s: Settings): Settings => JSON.parse(JSON.stringify(s))
const form = reactive<Settings>(clone(state.app!.settings))
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
    if (s && !pending && !sameAsSaved()) Object.assign(form, clone(s))
  },
)

async function save() {
  pending = false
  if (sameAsSaved()) return
  try {
    const next = clone(form)
    state.app!.settings = next
    await api.saveSettings(clone(next))
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

// the theme, colour and levels apply while dragging, not only once saved
watch(
  () => [form.theme, form.accentColor, form.lightLevel, form.darkLevel],
  () => (state.look = { theme: form.theme, accentColor: form.accentColor, lightLevel: form.lightLevel, darkLevel: form.darkLevel }),
)
onUnmounted(() => (state.look = null))

const recentColors = computed({
  get: () => form.recentColors ?? [],
  set: (v) => (form.recentColors = v),
})

const fakeIpOpen = ref(false)

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
      <div class="nav scroll">
        <div class="side-title">{{ t('设置') }}</div>
        <template v-for="g in groups" :key="g.label">
          <div v-if="g.label" class="nav-group">{{ g.label }}</div>
          <div v-else class="nav-gap" />
          <div class="nav-list">
            <div v-for="s in g.items" :key="s.id" class="nav-item" :class="{ active: active === s.id }" @click="jump(s.id)">
              <Icon :name="s.icon" :size="16" />
              <span>{{ s.label }}</span>
            </div>
          </div>
        </template>
      </div>
      <div class="faint saved"><Icon name="check" :size="13" /> {{ t('修改会自动保存并生效') }}</div>
    </aside>

    <main ref="scroller" class="content scroll" @scroll="onScroll">
      <Card id="sec-general" :title="t('常规')" :index="order('general')">
        <div class="form-row">
          <div class="label">{{ t('语言') }}<span class="hint">Language / 語言</span></div>
          <div class="control">
            <Segmented v-model="form.language" style="width: 300px" :options="languages" />
          </div>
        </div>
        <AutoStartRow v-model="form.autoStart" />
        <div v-if="form.autoStart" class="form-row sub">
          <div class="label">{{ t('自启后自动连接') }}<span class="hint">{{ t('使用上次的节点和路由模式') }}</span></div>
          <div class="control"><span class="spacer" /><Switch v-model="form.autoStartConnect" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('打开时自动连接') }}<span class="hint">{{ t('手动打开 NextBox 时也自动连接') }}</span></div>
          <div class="control"><span class="spacer" /><Switch v-model="form.connectOnStart" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('关闭时隐藏到托盘') }}<span class="hint">{{ t('关闭后代理继续在后台运行') }}</span></div>
          <div class="control"><span class="spacer" /><Switch v-model="form.closeToTray" /></div>
        </div>
      </Card>

      <Card id="sec-appearance" :title="t('外观')" :index="order('appearance')">
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
        <DisplayLevels v-model:light="form.lightLevel" v-model:dark="form.darkLevel" />
        <div class="form-row">
          <div class="label">{{ t('主题色') }}</div>
          <div class="control">
            <AccentPicker v-model="form.accentColor" v-model:recent="recentColors" />
          </div>
        </div>
      </Card>

      <Card id="sec-inbound" :title="t('代理端口')" :index="order('inbound')">
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

      <Card id="sec-tun" :title="t('TUN 模式')" :index="order('tun')">
        <div v-if="!state.app?.elevated" class="warn">
          <Icon name="warn" :size="16" />
          <span>{{ t('TUN 模式需要管理员权限。') }}</span>
          <button class="btn small" @click="run(() => api.restartAsAdmin())">{{ t('以管理员身份重启') }}</button>
        </div>
        <div class="form-row">
          <div class="label">{{ t('启用 TUN') }}<span class="hint">{{ t('接管所有程序的流量') }}</span></div>
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
        <div class="form-row">
          <div class="label">FakeIP<span class="hint">{{ t('走代理的域名先拿到假地址，首次连接更快、DNS 不泄露') }}</span></div>
          <div class="control">
            <button class="btn ghost small" @click="fakeIpOpen = !fakeIpOpen">{{ t('排除列表') }} <Icon :name="fakeIpOpen ? 'up' : 'down'" :size="12" /></button>
            <span class="spacer" />
            <Switch v-model="form.tunFakeIp" />
          </div>
        </div>
        <p v-if="form.tunFakeIp" class="faint tip">
          {{ t('只在 TUN 模式下生效。断开后个别程序可能还记着 198.18.x.x 的假地址而连不上，重启该程序即可。') }}
        </p>
        <div v-if="fakeIpOpen" class="fakeip-excl">
          <div class="faint tip">{{ t('这些域名（含子域名）始终返回真实 IP，每行一个。Windows 联网检测和局域网名称已默认包含。') }}</div>
          <textarea v-model.lazy="form.fakeIpExclude" class="textarea mono" spellcheck="false" />
        </div>
      </Card>

      <RouteSection :form="form" :index="order('route')" />
      <PolicySection :form="form" :index="order('policy')" />

      <Card id="sec-dns" title="DNS" :index="order('dns')">
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

      <Card id="sec-test" :title="t('订阅与测速')" :index="order('test')">
        <div class="form-row">
          <div class="label">{{ t('自动更新订阅') }}<span class="hint">{{ t('NextBox 运行期间在后台更新') }}</span></div>
          <div class="control">
            <select v-if="form.subAutoUpdate" v-model.number="form.subUpdateHours" class="select" style="width: 160px">
              <option v-for="h in updateHours" :key="h.value" :value="h.value">{{ h.label }}</option>
            </select>
            <Switch v-model="form.subAutoUpdate" />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('通过代理更新订阅') }}<span class="hint">{{ t('仅在已连接时生效') }}</span></div>
          <div class="control"><Switch v-model="form.subUpdateByProxy" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('订阅 User-Agent') }}</div>
          <div class="control"><input v-model.lazy="form.subUserAgent" class="input mono" /></div>
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
      </Card>

      <BackupSection :index="order('backup')" />

      <Card id="sec-logs" :title="t('日志与诊断')" :index="order('logs')">
        <div class="form-row">
          <div class="label">{{ t('日志级别') }}<span class="hint">{{ t('debug 最详细，warn 只记录问题') }}</span></div>
          <div class="control">
            <select v-model="form.logLevel" class="select" style="width: 160px">
              <option v-for="l in ['debug', 'info', 'warn', 'error']" :key="l" :value="l">{{ l }}</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('识别连接进程') }}<span class="hint">{{ t('连接页显示程序名，略增开销') }}</span></div>
          <div class="control"><Switch v-model="form.findProcess" /></div>
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
        <template v-if="form.logToFile || logFiles.count">
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
        </template>
      </Card>

      <AboutSection :form="form" :index="order('about')" />
    </main>
  </div>
</template>

<style scoped>
.nav {
  flex: 1;
  min-height: 0;
  margin: 0 -6px;
  padding: 0 6px;
}
.nav-group {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--text-3);
  margin: 12px 4px 4px;
}
.nav-gap {
  height: 10px;
  margin: 8px 4px 6px;
  border-top: 1px solid var(--line);
}
.nav .nav-item {
  height: 34px;
}
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
.form-row.sub {
  padding-left: 18px;
}
.form-row.sub .label {
  width: 132px;
}
.textarea {
  min-height: 100px;
}
.tip {
  margin: 8px 0 0;
  font-size: 12px;
}
.fakeip-excl .textarea {
  width: 100%;
  margin-top: 6px;
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
</style>
