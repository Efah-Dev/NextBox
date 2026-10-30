<script setup lang="ts">
import { languages, t } from '../i18n'
import { onMounted, reactive, ref, watch } from 'vue'
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
        <div class="about">
          <span class="about-logo"><PawLogo :size="44" /></span>
          <div>
            <div class="about-name">NextBox <span class="badge">v{{ state.app?.version }}</span></div>
            <div class="muted">{{ t('基于 NekoBox for PC（nekoray）修改而来的现代化代理客户端，内核采用 sing-box。') }}</div>
          </div>
        </div>
        <div class="core">
          <template v-if="core?.found">
            <div class="core-grid">
              <span class="faint">{{ t('内核版本') }}</span>
              <span>
                sing-box <b>{{ core.version }}</b>
                <span class="badge" :class="{ testing: core.channel === 'testing' }">
                  {{ core.channel === 'testing' ? t('testing 测试分支') : t('stable 稳定分支') }}
                </span>
              </span>
              <span class="faint">{{ t('启用功能') }}</span>
              <span class="mono tags">{{ core.tags.split(',').join(' · ') }}</span>
            </div>
          </template>
          <div v-else class="faint">{{ t('未找到内核：{0}', state.app?.corePath) }}</div>
        </div>
        <div class="credits">
          <div>{{ t('项目地址：') }}<a @click="api.openURL('https://github.com/Efah-Dev/NextBox')">Efah-Dev/NextBox</a></div>
          <div>{{ t('原项目：') }}<a @click="api.openURL('https://github.com/MatsuriDayo/nekoray')">MatsuriDayo/nekoray</a></div>
          <div>{{ t('规则集：SagerNet/sing-geosite、sing-geoip') }}</div>
          <div class="faint">{{ t('本软件以 GPL-3.0-or-later 协议开源。') }}</div>
        </div>
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
.about {
  display: flex;
  align-items: center;
  gap: 14px;
}
.about-logo {
  width: 64px;
  height: 64px;
  flex-shrink: 0;
  border-radius: 18px;
  background: color-mix(in srgb, #ff5c9a 10%, var(--card));
  display: grid;
  place-items: center;
}
.about-name {
  font-size: 17px;
  font-weight: 700;
  margin-bottom: 4px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.core {
  margin-top: 16px;
  padding: 12px 14px;
  border-radius: var(--radius-sm);
  background: var(--hover);
}
.core-grid {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 6px 12px;
  line-height: 1.6;
}
.core-grid a {
  color: var(--accent);
  cursor: pointer;
}
.core-grid a:hover {
  text-decoration: underline;
}
.core .badge {
  margin-left: 6px;
  background: color-mix(in srgb, var(--good) 14%, var(--card));
  color: var(--good);
}
.core .badge.testing {
  background: color-mix(in srgb, var(--mid) 14%, var(--card));
  color: var(--mid);
}
.tags {
  font-size: 11px;
  color: var(--text-2);
  word-break: break-word;
  user-select: text;
}
.credits {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  line-height: 1.6;
}
.credits a {
  color: var(--accent);
  cursor: pointer;
}
.credits a:hover {
  text-decoration: underline;
}
</style>
