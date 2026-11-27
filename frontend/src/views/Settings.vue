<script lang="ts">
let lastSection = 'general'
</script>

<script setup lang="ts">
import { languages, t } from '../i18n'
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import Card from '../components/Card.vue'
import Icon from '../components/Icon.vue'
import Switch from '../components/Switch.vue'
import Segmented from '../components/Segmented.vue'
import HelpTip from '../components/HelpTip.vue'
import Reveal from '../motion/Reveal.vue'
import AccentPicker from './settings/AccentPicker.vue'
import DisplayLevels from './settings/DisplayLevels.vue'
import AutoStartRow from './settings/AutoStartRow.vue'
import RouteSection from './settings/RouteSection.vue'
import DnsOverrides from './settings/DnsOverrides.vue'
import SubscriptionDns from './settings/SubscriptionDns.vue'
import TunExcludeRow from './settings/TunExcludeRow.vue'
import TunServiceRow from './settings/TunServiceRow.vue'
import PolicySection from './settings/PolicySection.vue'
import BackupSection from './settings/BackupSection.vue'
import AboutSection from './settings/AboutSection.vue'
import HotkeySection from './settings/HotkeySection.vue'
import TraySection from './settings/TraySection.vue'
import { api, type Settings } from '../api'
import { state } from '../state'
import { confirmDialog, formatBytes, run, toast } from '../util'
import { isStep, ruleFilled } from '../policies'
import { DARK_DEFAULT, LIGHT_DEFAULT } from '../theme'

// Sorted by how often people change them: everyday things first, rarely
// touched ones last.
const groups = [
  {
    label: t('基础'),
    items: [
      { id: 'general', label: t('常规'), icon: 'settings' },
      { id: 'appearance', label: t('外观'), icon: 'palette' },
      { id: 'hotkeys', label: t('快捷键'), icon: 'key' },
      { id: 'tray', label: t('托盘菜单'), icon: 'tray' },
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
  { label: t('节点'), items: [{ id: 'test', label: t('订阅与测试'), icon: 'zap' }] },
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
// One section is shown at a time; the page remembers the last one while
// NextBox runs.
const active = ref(lastSection)
watch(active, (id) => {
  lastSection = id
  scroller.value?.scrollTo({ top: 0 })
})
const scroller = ref<HTMLElement>()

const testUrls = [
  { label: t('Google（推荐）'), value: 'https://www.google.com/generate_204' },
  { label: 'Cloudflare', value: 'https://cp.cloudflare.com/generate_204' },
]

// rules and scenes are arrays: the form must never share them with the
// saved settings, or a change would look saved before it is
const clone = <T,>(v: T): T => JSON.parse(JSON.stringify(v))
type Key = keyof Settings
const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)

// The form is compared key by key with base, the settings as they were when
// it last took them over: only the keys changed here are saved, and changes
// made elsewhere (an imported backup, the backend's normalisation, other
// pages) are taken over for every other key. A comparison of the whole
// object would depend on the order of its keys, and a stale form would be
// saved over the newer settings.
let base = clone(state.app!.settings)
const form = reactive<Settings>(clone(base))
const edited = () => [...new Set([...Object.keys(form), ...Object.keys(base)])].filter((k) => !same(form[k as Key], base[k as Key])) as Key[]
// left-overs of earlier visits (see the clean-up when the page is left)
if (form.rules.some((r) => !isStep(r) && !ruleFilled(r))) form.rules = form.rules.filter((r) => isStep(r) || ruleFilled(r))
let timer = 0

// Auto-save, like a launcher: every change is persisted shortly after it is made.
watch(form, () => {
  clearTimeout(timer)
  if (edited().length) timer = window.setTimeout(save, 350)
})

watch(
  () => state.app?.settings,
  (s) => {
    if (!s) return
    const mine = edited()
    const next = clone(s)
    base = clone(s)
    const f = form as Record<string, unknown>
    for (const k of Object.keys(f)) if (!(k in next) && !mine.includes(k as Key)) delete f[k]
    for (const [k, v] of Object.entries(next)) if (!mine.includes(k as Key) && !same(f[k], v)) f[k] = v
  },
)

async function save() {
  clearTimeout(timer)
  const keys = edited()
  if (!keys.length || !state.app) return
  const next = clone(state.app.settings) as Record<string, unknown>
  for (const k of keys) {
    if (!(k in form)) {
      delete next[k]
      continue
    }
    next[k] = clone(form[k])
    // an emptied number field means the default
    if ((k === 'speedTestSeconds' || k === 'speedTestMb') && typeof next[k] !== 'number') next[k] = Number(next[k]) || 0
  }
  try {
    state.app.settings = next as unknown as Settings
    await api.saveSettings(clone(next as unknown as Settings))
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
const levelsOpen = ref(false)
const levelsChanged = computed(() => (form.lightLevel ?? LIGHT_DEFAULT) !== LIGHT_DEFAULT || (form.darkLevel ?? DARK_DEFAULT) !== DARK_DEFAULT)

const logFiles = ref({ count: 0, bytes: 0 })
const refreshLogFiles = async () => (logFiles.value = await api.getLogFilesInfo().catch(() => ({ count: 0, bytes: 0, dir: '' })))
onMounted(refreshLogFiles)

async function clearLogFiles() {
  if (!(await confirmDialog(t('清除日志文件'), t('确定删除全部 {0} 个日志文件（{1}）吗？', logFiles.value.count, formatBytes(logFiles.value.bytes)), true))) return
  await run(() => api.clearLogFiles(), (n) => t('已删除 {0} 个日志文件', n))
  refreshLogFiles()
}

const jump = (id: string) => (active.value = id)

// ---- search: the options on the page as they are shown (in the current
// language), found by their names and the hints under them
interface Hit {
  key: number
  id: string // the section
  section: string
  label: string
  hint: string
  el: HTMLElement
}
const query = ref('')
const hits = ref<Hit[]>([])
const searchBox = ref<HTMLInputElement>()
function search() {
  const q = query.value.trim().toLowerCase()
  if (!q) return (hits.value = [])
  const out: Hit[] = []
  for (const s of sections) {
    const card = document.getElementById('sec-' + s.id)
    if (!card) continue
    const before = out.length
    card.querySelectorAll<HTMLElement>('.form-row > .label, .block-title, .svc-head b').forEach((labelEl) => {
      const hintText = labelEl.querySelector('.hint')?.textContent?.trim() ?? ''
      // the help tip's "?" is not part of the name
      const label = (labelEl.textContent ?? '').replace(hintText, '').replace(/\?+\s*$/, '').trim()
      if (!label || !(label + ' ' + hintText).toLowerCase().includes(q)) return
      const row = (labelEl.closest('.form-row') as HTMLElement) ?? labelEl
      if (!out.some((h) => h.el === row)) out.push({ key: out.length, id: s.id, section: s.label, label, hint: hintText, el: row })
    })
    if (out.length === before && s.label.toLowerCase().includes(q)) out.push({ key: out.length, id: s.id, section: s.label, label: s.label, hint: '', el: card })
  }
  hits.value = out.slice(0, 40)
}
watch(query, search)
async function goTo(h: Hit) {
  active.value = h.id
  await nextTick()
  h.el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  h.el.classList.remove('search-flash')
  void h.el.offsetWidth // restart the animation
  h.el.classList.add('search-flash')
  window.setTimeout(() => h.el.classList.remove('search-flash'), 1800)
}
function onKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') {
    e.preventDefault()
    searchBox.value?.focus()
    searchBox.value?.select()
  }
}
onMounted(() => window.addEventListener('keydown', onKey))

// A rule added but never filled in (or a rule set left empty) matches
// nothing: it goes when the page is left, instead of waiting there forever.
onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  const filled = form.rules.filter((r) => isStep(r) || ruleFilled(r))
  if (filled.length !== form.rules.length) {
    form.rules = filled
    save()
  }
})

// Opened from another page for a specific section (e.g. Nodes → 测试设置).
onMounted(() => {
  const id = state.settingsSection
  if (!id) return
  state.settingsSection = ''
  if (sections.some((s) => s.id === id)) active.value = id
})
</script>

<template>
  <div class="page">
    <aside class="side">
      <div class="nav scroll">
        <div class="side-title">{{ t('设置') }}</div>
        <div class="side-search">
          <Icon name="search" :size="13" />
          <input ref="searchBox" v-model="query" class="input" :placeholder="t('搜索设置')" spellcheck="false" @keydown.esc="query = ''" @keydown.enter="hits[0] && goTo(hits[0])" />
          <button v-if="query" class="clear" :title="t('清除')" @click="query = ''"><Icon name="close" :size="11" /></button>
        </div>
        <div v-if="query.trim()" class="results">
          <div v-for="h in hits" :key="h.key" class="result" :title="h.hint" @click="goTo(h)">
            <span class="r-label ellipsis">{{ h.label }}</span>
            <span class="r-section faint ellipsis">{{ h.section }}</span>
          </div>
          <div v-if="!hits.length" class="faint none">{{ t('没有找到相关设置') }}</div>
        </div>
        <template v-for="g in groups" v-else :key="g.label">
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

    <main ref="scroller" class="content scroll settings-content">
      <Card v-show="active === 'general'" id="sec-general" :title="t('常规')">
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
          <div class="label">{{ t('打开时自动连接') }}<span class="hint">{{ t('手动启动 NextBox 时自动连接') }}</span></div>
          <div class="control"><span class="spacer" /><Switch v-model="form.connectOnStart" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('关闭时隐藏到托盘') }}<span class="hint">{{ t('关闭窗口后代理保持运行') }}</span></div>
          <div class="control"><span class="spacer" /><Switch v-model="form.closeToTray" /></div>
        </div>
      </Card>

      <Card v-show="active === 'appearance'" id="sec-appearance" :title="t('外观')">
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
            <AccentPicker v-model="form.accentColor" v-model:recent="recentColors" />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('亮度与深度') }}<span class="hint">{{ levelsChanged ? t('已调整') : t('标准') }}</span></div>
          <div class="control">
            <button class="btn ghost small" @click="levelsOpen = !levelsOpen">{{ levelsOpen ? t('收起') : t('调整') }} <Icon :name="levelsOpen ? 'up' : 'down'" :size="12" /></button>
          </div>
        </div>
        <Reveal :open="levelsOpen">
          <DisplayLevels v-model:light="form.lightLevel" v-model:dark="form.darkLevel" />
        </Reveal>
      </Card>

      <HotkeySection v-show="active === 'hotkeys'" :form="form" />
      <TraySection v-show="active === 'tray'" :form="form" />

      <Card v-show="active === 'inbound'" id="sec-inbound" :title="t('代理端口')">
        <div class="form-row">
          <div class="label">{{ t('连接时设置系统代理') }}<span class="hint">{{ t('TUN 模式下不设置') }}</span></div>
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
          <div class="label"><span>{{ t('允许局域网连接') }}<HelpTip :text="t('监听地址改为 0.0.0.0，同一局域网的设备可以把本机作为代理服务器。代理端口没有密码保护，在公共网络中请关闭。')" /></span><span class="hint">{{ t('局域网设备可连接代理端口') }}</span></div>
          <div class="control"><Switch v-model="form.allowLan" /></div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('流量嗅探') }}<span class="hint">{{ t('从 TLS SNI 与 HTTP Host 识别域名') }}</span></div>
          <div class="control"><Switch v-model="form.sniff" /></div>
        </div>
      </Card>

      <Card v-show="active === 'tun'" id="sec-tun" :title="t('TUN 模式')">
        <div v-if="!state.app?.elevated && !state.app?.serviceReady" class="warn">
          <Icon name="warn" :size="16" />
          <span>{{ t('TUN 模式需要管理员权限：请安装服务模式，或以管理员身份运行。') }}</span>
          <button class="btn small" @click="run(() => api.restartAsAdmin())">{{ t('以管理员身份重启') }}</button>
        </div>
        <div class="form-row">
          <div class="label">{{ t('启用 TUN') }}<span class="hint">{{ t('接管所有程序的流量') }}</span></div>
          <div class="control"><Switch v-model="form.tunEnabled" :disabled="!state.app?.elevated && !state.app?.serviceReady && !form.tunEnabled" /></div>
        </div>
        <TunServiceRow />
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
        <TunExcludeRow :form="form" />
        <div class="form-row">
          <div class="label"><span>FakeIP<HelpTip :text="t('仅在 TUN 模式下生效。需要走代理的域名先得到 198.18.0.0/15 内的保留地址，连接时由内核还原为域名，可省去一次远程解析并避免本地 DNS 泄露。断开后个别程序可能仍缓存该地址，重启该程序即可。')" /></span><span class="hint">{{ t('为代理域名分配保留地址') }}</span></div>
          <div class="control">
            <button class="btn ghost small" @click="fakeIpOpen = !fakeIpOpen">{{ t('排除列表') }} <Icon :name="fakeIpOpen ? 'up' : 'down'" :size="12" /></button>
            <span class="spacer" />
            <Switch v-model="form.tunFakeIp" />
          </div>
        </div>
        <Reveal :open="fakeIpOpen">
          <div class="fakeip-excl">
            <div class="faint tip">{{ t('这些域名（含子域名）始终返回真实 IP，每行一个。Windows 联网检测和局域网名称已默认包含。') }}</div>
            <textarea v-model.lazy="form.fakeIpExclude" class="textarea mono" spellcheck="false" />
          </div>
        </Reveal>
      </Card>

      <RouteSection v-show="active === 'route'" :form="form" />
      <PolicySection v-show="active === 'policy'" :form="form" />

      <Card v-show="active === 'dns'" id="sec-dns" title="DNS">
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
        <DnsOverrides v-model="form.dnsOverrides" />
        <SubscriptionDns />
        <p class="faint tip">
          {{ t('支持 223.5.5.5、tcp://、tls://、quic://、https://、h3:// 地址，以及 local（系统 DNS）。') }}
        </p>
      </Card>

      <Card v-show="active === 'test'" id="sec-test" :title="t('订阅与测试')">
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
            <span>{{ t('节点地区') }}<HelpTip :text="t('落地 IP：URL 测延迟时经节点查询出口地址，结果最准确。服务器 IP：用本地数据库查询节点服务器的地址，无需联网，但中转节点会显示入口所在地区。')" /></span>
            <span class="hint">{{ form.countrySource === 'server' ? t('依据服务器 IP（本地查询）') : t('依据落地 IP（URL 测试时检测）') }}</span>
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
          <div class="label">
            <span>{{ t('测延迟方式') }}<HelpTip :text="t('TCP：只测与服务器端口建立连接的时间，速度快，但不能说明节点可用。URL：经节点请求测试地址，结果即实际可用的延迟。基于 UDP 的协议（Hysteria2、TUIC 等）和链式代理始终使用 URL。')" /></span>
            <span class="hint">{{ form.testMode === 'tcp' ? t('只测到服务器端口的握手时间') : t('经节点请求测试地址，反映实际可用性') }}</span>
          </div>
          <div class="control">
            <Segmented
              v-model="form.testMode"
              :options="[
                { value: 'tcp', label: 'TCP' },
                { value: 'url', label: 'URL' },
              ]"
            />
          </div>
        </div>
        <div class="form-row">
          <div class="label">
            {{ t('测延迟后') }}
            <span class="hint">{{ form.sortAfterTest === 'default' ? t('恢复订阅中的原始顺序') : form.sortAfterTest === 'latency' ? t('按延迟从低到高排列') : form.sortAfterTest === 'none' ? t('保持当前顺序') : t('尚未选择，首次测试时询问') }}</span>
          </div>
          <div class="control">
            <Segmented
              v-model="form.sortAfterTest"
              :options="[
                { value: 'default', label: t('默认顺序') },
                { value: 'latency', label: t('按延迟') },
                { value: 'none', label: t('不动') },
              ]"
            />
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('测试地址') }}</div>
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
        <div class="sub-title">{{ t('下载测速') }}</div>
        <div class="form-row">
          <div class="label">
            <span>{{ t('单个节点时长') }}<HelpTip :text="t('每个节点下载到这个时间就停止，按下载期间的平均速度计。时间越长结果越稳定，也越费流量。')" /></span>
            <span class="hint">{{ t('1–60 秒，默认 10 秒') }}</span>
          </div>
          <div class="control"><input v-model.lazy.number="form.speedTestSeconds" type="number" min="1" max="60" class="input mono" style="width: 120px" placeholder="10" /></div>
        </div>
        <div class="form-row">
          <div class="label">
            <span>{{ t('单个节点流量上限') }}<HelpTip :text="t('每个节点下载到这个数量就停止，先到时长或流量上限的一方为准。流量少的套餐可以调低。')" /></span>
            <span class="hint">{{ t('MB，默认 50') }}</span>
          </div>
          <div class="control"><input v-model.lazy.number="form.speedTestMb" type="number" min="1" max="1024" class="input mono" style="width: 120px" placeholder="50" /></div>
        </div>
        <div class="form-row">
          <div class="label">
            <span>{{ t('测速文件') }}<HelpTip :text="t('留空使用 Cloudflare 的测速文件（speed.cloudflare.com），大小按流量上限生成。自定义地址需要能直接下载，且文件足够大。')" /></span>
            <span class="hint">{{ t('留空使用 Cloudflare') }}</span>
          </div>
          <div class="control"><input v-model.lazy.trim="form.speedTestUrl" class="input mono" placeholder="https://speed.cloudflare.com/__down" /></div>
        </div>
      </Card>

      <BackupSection v-show="active === 'backup'" />

      <Card v-show="active === 'logs'" id="sec-logs" :title="t('日志与诊断')">
        <div class="form-row">
          <div class="label">{{ t('日志级别') }}<span class="hint">{{ t('debug 最详细，warn 仅记录异常') }}</span></div>
          <div class="control">
            <select v-model="form.logLevel" class="select" style="width: 160px">
              <option v-for="l in ['debug', 'info', 'warn', 'error']" :key="l" :value="l">{{ l }}</option>
            </select>
          </div>
        </div>
        <div class="form-row">
          <div class="label">{{ t('识别连接进程') }}<span class="hint">{{ t('在连接页显示发起连接的程序') }}</span></div>
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
          <div class="label">{{ t('记录日志到文件') }}<span class="hint">{{ t('每次启动新建一个日志文件') }}</span></div>
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
            <div class="label">{{ t('保留时间') }}<span class="hint">{{ t('启动时清理超期的日志文件') }}</span></div>
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

      <AboutSection v-show="active === 'about'" :form="form" />
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
.side-search {
  position: relative;
  display: flex;
  align-items: center;
  margin: 0 0 6px;
}
.side-search > :deep(svg) {
  position: absolute;
  left: 9px;
  color: var(--text-3);
  pointer-events: none;
}
.side-search .input {
  width: 100%;
  height: 30px;
  font-size: 12.5px;
  padding-left: 28px;
  padding-right: 24px;
}
.side-search .clear {
  position: absolute;
  right: 6px;
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  border: 0;
  border-radius: 50%;
  background: var(--hover);
  color: var(--text-2);
  cursor: pointer;
}
.results {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.result {
  display: flex;
  flex-direction: column;
  padding: 6px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  min-width: 0;
}
.result:hover {
  background: var(--hover);
}
.r-label {
  font-size: 12.5px;
  font-weight: 600;
}
.r-section {
  font-size: 11px;
}
.results .none {
  padding: 10px 8px;
  font-size: 12px;
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
/* one section per page: rows of one height, the name on the left and its
   controls on the right */
/* one column of the same width, centred in a wide window */
.settings-content > * {
  flex-shrink: 0;
  width: 100%;
  max-width: 980px;
  margin-left: auto;
  margin-right: auto;
}
.settings-content :deep(.form-row) {
  min-height: 52px;
  padding: 8px 0;
}
.settings-content :deep(.form-row > .control) {
  justify-content: flex-end;
}
.settings-content :deep(.form-row > .label) {
  width: 260px;
}
.settings-content :deep(.form-row > .label .hint) {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.form-row.sub {
  padding-left: 18px;
}
.settings-content .form-row.sub > .label {
  width: 242px;
}
.textarea {
  min-height: 100px;
}
.sub-title {
  margin: 14px 0 2px;
  padding-top: 12px;
  border-top: 1px solid var(--line);
  font-size: 12px;
  font-weight: 700;
  color: var(--text-2);
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
