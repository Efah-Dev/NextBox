<script setup lang="ts">
// 数据与备份: where the data lives, encrypting the node list on disk, and
// password-protected backups that NextBox for Android can open too.
import { t } from '../../i18n'
import { computed, reactive, ref } from 'vue'
import Card from '../../components/Card.vue'
import Icon from '../../components/Icon.vue'
import Switch from '../../components/Switch.vue'
import Modal from '../../components/Modal.vue'
import Segmented from '../../components/Segmented.vue'
import { api, type BackupInfo } from '../../api'
import { loadProfiles, state } from '../../state'
import { confirmDialog, promptDialog, run, toast } from '../../util'
import { refreshPolicies } from '../../policies'

defineProps<{ index: number }>()

// ---- encryption at rest ----
const encryption = computed(() => state.app?.encryption ?? '')

async function askPassword(title: string): Promise<string | null> {
  const r = await promptDialog(title, [
    { label: t('启动密码'), placeholder: t('至少 6 位'), warn: (v) => (v && [...v].length < 6 ? t('至少 6 位') : '') },
    { label: t('再次输入') },
  ], t('每次打开 NextBox 都要输入这个密码。忘记后无法恢复节点。'))
  if (!r) return null
  if (r[0] !== r[1]) {
    toast(t('两次输入的密码不一致'), 'error')
    return null
  }
  if ([...r[0]].length < 6) {
    toast(t('密码至少 6 位'), 'error')
    return null
  }
  return r[0]
}

async function setMode(mode: '' | 'windows' | 'password') {
  if (mode === encryption.value && mode !== 'password') return
  let password = ''
  if (mode === 'password') {
    const p = await askPassword(encryption.value === 'password' ? t('修改启动密码') : t('设置启动密码'))
    if (p === null) return
    password = p
  }
  if (mode === '' && !(await confirmDialog(t('改为明文保存'), t('节点密码和订阅链接将以明文保存在 profiles.json 里，确定吗？')))) return
  await run(() => api.setEncryption(mode, password), mode ? t('已加密保存') : t('已改为明文保存'))
}

const encOn = computed({
  get: () => !!encryption.value,
  set: (v) => setMode(v ? 'windows' : ''),
})

// ---- export ----
const exportOpen = ref(false)
const exp = reactive({ nodes: true, rules: true, settings: false, encrypt: true, password: '', password2: '' })
const strength = computed(() => {
  const p = exp.password
  let s = 0
  if (p.length >= 8) s++
  if (p.length >= 12) s++
  if (/[a-z]/.test(p) && /[A-Z]/.test(p)) s++
  if (/\d/.test(p) && /[^A-Za-z0-9]/.test(p)) s++
  return p ? Math.max(1, s) : 0
})
const exportProblem = computed(() => {
  if (!exp.nodes && !exp.rules && !exp.settings) return t('请至少选择一项内容')
  if (!exp.encrypt) return ''
  if ([...exp.password].length < 6) return t('密码至少 6 位')
  if (exp.password !== exp.password2) return t('两次输入的密码不一致')
  return ''
})
function openExport() {
  Object.assign(exp, { password: '', password2: '', encrypt: true })
  exportOpen.value = true
}
async function doExport() {
  const path = await run(() => api.exportBackup({ nodes: exp.nodes, rules: exp.rules, settings: exp.settings, password: exp.encrypt ? exp.password : '' }))
  if (path) {
    exportOpen.value = false
    toast(t('已导出到 {0}', path), 'success')
  }
}

// ---- import ----
const importOpen = ref(false)
const imp = reactive({ path: '', password: '', info: null as BackupInfo | null, error: '', busy: false, nodes: true, replace: false, rules: true, settings: false })
async function openImport() {
  const path = await run(() => api.chooseBackup())
  if (!path) return
  Object.assign(imp, { path, password: '', info: null, error: '', busy: false, replace: false })
  importOpen.value = true
  inspect()
}
async function inspect() {
  imp.busy = true
  imp.error = ''
  try {
    imp.info = await api.inspectBackup(imp.path, imp.password)
    if (!imp.info.needPassword) {
      imp.nodes = imp.info.hasNodes
      imp.rules = imp.info.hasRules
      imp.settings = false
    }
  } catch (e) {
    imp.error = (e as Error).message
  } finally {
    imp.busy = false
  }
}
const platformName = (p: string) => (p === 'android' ? 'Android' : p === 'windows' ? 'Windows' : t('未知'))
const foreign = computed(() => imp.info && imp.info.platform !== 'windows')
async function doImport() {
  if (imp.nodes && imp.replace && !(await confirmDialog(t('替换现有节点'), t('现有的分组、节点和分组代理都会被备份里的内容替换，确定吗？'), true, t('替换')))) return
  imp.busy = true
  const msg = await run(() => api.importBackup(imp.path, { nodes: imp.nodes, rules: imp.rules, settings: imp.settings, password: imp.password, replace: imp.replace }), (m) => m)
  imp.busy = false
  if (msg) {
    importOpen.value = false
    loadProfiles()
    refreshPolicies()
  }
}
const fileName = computed(() => imp.path.split(/[\\/]/).pop())
</script>

<template>
  <Card id="sec-backup" :title="t('数据与备份')" :index="index">
    <div class="form-row">
      <div class="label">{{ t('数据目录') }}</div>
      <div class="control">
        <span class="mono faint ellipsis">{{ state.app?.dataDir }}</span>
        <button class="btn small" @click="run(() => api.openDataDir())"><Icon name="folder" :size="13" /> {{ t('打开') }}</button>
      </div>
    </div>
    <div class="form-row">
      <div class="label">{{ t('备份') }}<span class="hint">{{ t('加密后的备份可以在 NextBox Android 版中导入') }}</span></div>
      <div class="control">
        <button class="btn" @click="openImport"><Icon name="download" :size="14" /> {{ t('导入…') }}</button>
        <button class="btn" @click="openExport"><Icon name="upload" :size="14" /> {{ t('导出…') }}</button>
      </div>
    </div>
    <div class="form-row">
      <div class="label">{{ t('加密保存节点和订阅') }}<span class="hint">{{ t('防止 profiles.json 被直接读取') }}</span></div>
      <div class="control"><span class="spacer" /><Switch v-model="encOn" /></div>
    </div>
    <div v-if="encryption" class="modes">
      <button class="mode" :class="{ on: encryption === 'windows' }" @click="setMode('windows')">
        <span class="radio" />
        <span>
          <b>{{ t('跟随 Windows 账户') }}<span class="rec">{{ t('推荐') }}</span></b>
          <span class="faint">{{ t('无需密码，开机自启照常工作。只有这台电脑的当前账户能打开；换电脑前先导出备份。') }}</span>
        </span>
      </button>
      <button class="mode" :class="{ on: encryption === 'password' }" @click="encryption !== 'password' && setMode('password')">
        <span class="radio" />
        <span>
          <b>{{ t('启动密码') }}</b>
          <span class="faint">{{ t('每次打开 NextBox 都要输入密码，开机自启时会停在解锁界面。忘记密码无法恢复。') }}</span>
          <a v-if="encryption === 'password'" class="link" @click.stop="setMode('password')">{{ t('修改密码') }}</a>
        </span>
      </button>
      <p class="faint note">
        <Icon name="info" :size="12" />
        {{ t('加密能防止别人拷走文件夹、网盘同步或误发文件时泄露节点；防不了已在这台电脑上运行的恶意软件。内核配置不再写入磁盘。') }}
      </p>
    </div>

    <!-- export -->
    <Modal v-model="exportOpen" :title="t('导出配置')" :width="460">
      <label class="check"><input v-model="exp.nodes" type="checkbox" /> {{ t('节点与订阅') }}<span class="faint">{{ t('含链式代理和分组代理') }}</span></label>
      <label class="check"><input v-model="exp.rules" type="checkbox" /> {{ t('分流规则') }}<span class="faint">{{ t('{0} 条规则', state.app?.settings.rules.length ?? 0) }}</span></label>
      <label class="check"><input v-model="exp.settings" type="checkbox" /> {{ t('其他设置') }}<span class="faint">{{ t('端口、TUN 等只在 Windows 上导入') }}</span></label>
      <div class="enc-row">
        <span><Icon name="lock" :size="14" /> {{ t('用密码加密') }}</span>
        <Switch v-model="exp.encrypt" />
      </div>
      <template v-if="exp.encrypt">
        <input v-model="exp.password" type="password" class="input full" :placeholder="t('密码（至少 6 位）')" autocomplete="new-password" />
        <div class="meter"><i v-for="n in 4" :key="n" :class="{ on: n <= strength, weak: strength <= 1 }" /></div>
        <input v-model="exp.password2" type="password" class="input full" :placeholder="t('再次输入')" autocomplete="new-password" />
        <p class="faint small">{{ t('AES-256 加密，和 Android 版的备份格式相同。忘记密码将无法恢复。') }}</p>
      </template>
      <p v-else class="danger-note small"><Icon name="warn" :size="13" /> {{ t('不加密的备份里，节点密码和订阅链接都是明文，请勿发给别人或上传到网盘。') }}</p>
      <template #footer>
        <span class="faint small problem">{{ exportProblem }}</span>
        <button class="btn" @click="exportOpen = false">{{ t('取消') }}</button>
        <button class="btn primary" :disabled="!!exportProblem" @click="doExport"><Icon :name="exp.encrypt ? 'lock' : 'upload'" :size="14" /> {{ exp.encrypt ? t('加密导出') : t('导出') }}</button>
      </template>
    </Modal>

    <!-- import -->
    <Modal v-model="importOpen" :title="t('导入配置')" :width="460">
      <div class="file"><Icon name="backup" :size="16" /> <span class="ellipsis mono">{{ fileName }}</span></div>
      <form v-if="imp.info?.needPassword" class="pw" @submit.prevent="inspect">
        <p class="small">{{ t('这个备份已加密，请输入导出时设置的密码。') }}</p>
        <div class="row">
          <input v-model="imp.password" type="password" class="input full" :placeholder="t('备份密码')" autofocus />
          <button class="btn primary" type="submit" :disabled="!imp.password || imp.busy">{{ t('打开') }}</button>
        </div>
      </form>
      <p v-if="imp.error" class="danger-note small"><Icon name="warn" :size="13" /> {{ imp.error }}</p>
      <template v-if="imp.info && !imp.info.needPassword">
        <div class="summary">
          <span class="badge">{{ t('来自 {0}', platformName(imp.info.platform)) }}</span>
          <span v-if="imp.info.encrypted" class="badge"><Icon name="lock" :size="11" /> {{ t('已加密') }}</span>
        </div>
        <label v-if="imp.info.hasNodes" class="check">
          <input v-model="imp.nodes" type="checkbox" /> {{ t('节点与订阅') }}
          <span class="faint">{{ t('{0} 个订阅、{1} 个节点', imp.info.summary.subscriptions, imp.info.summary.nodes + imp.info.summary.chains) }}<template v-if="imp.info.summary.policies">{{ t('、{0} 个分组代理', imp.info.summary.policies) }}</template></span>
        </label>
        <div v-if="imp.info.hasNodes && imp.nodes" class="sub">
          <Segmented
            :model-value="imp.replace ? 'replace' : 'merge'"
            style="width: 260px"
            :options="[
              { value: 'merge', label: t('合并到现有节点') },
              { value: 'replace', label: t('替换现有节点') },
            ]"
            @update:model-value="imp.replace = $event === 'replace'"
          />
          <p class="faint small">{{ imp.replace ? t('现有的分组和节点会被替换。') : t('已有的订阅不会重复添加。') }}</p>
        </div>
        <label v-if="imp.info.hasRules" class="check"><input v-model="imp.rules" type="checkbox" /> {{ t('分流规则') }}<span class="faint">{{ t('{0} 条，会替换现有规则', imp.info.rules) }}</span></label>
        <label v-if="imp.info.hasSettings" class="check">
          <input v-model="imp.settings" type="checkbox" /> {{ t('其他设置') }}
          <span class="faint">{{ foreign ? t('来自其他系统：只导入主题、DNS、测速等通用设置') : t('包括端口、TUN 等') }}</span>
        </label>
      </template>
      <template #footer>
        <button class="btn" @click="importOpen = false">{{ t('取消') }}</button>
        <button class="btn primary" :disabled="!imp.info || imp.info.needPassword || imp.busy || (!imp.nodes && !imp.rules && !imp.settings)" @click="doImport">
          <Icon name="download" :size="14" /> {{ t('导入') }}
        </button>
      </template>
    </Modal>
  </Card>
</template>

<style scoped>
.modes {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 4px 0 10px;
}
.mode {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border-radius: var(--radius);
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.mode > span:last-child {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 12px;
  line-height: 1.6;
}
.mode b {
  font-size: 13px;
}
.mode.on {
  border-color: var(--accent);
  background: var(--accent-soft);
}
.radio {
  width: 15px;
  height: 15px;
  margin-top: 3px;
  flex-shrink: 0;
  border-radius: 50%;
  border: 1.5px solid var(--input-border);
}
.mode.on .radio {
  border: 4.5px solid var(--accent);
}
.rec {
  margin-left: 6px;
  font-size: 10.5px;
  padding: 0 6px;
  border-radius: 4px;
  background: var(--accent);
  color: #fff;
}
.link {
  color: var(--accent);
  cursor: pointer;
  align-self: flex-start;
}
.note {
  display: flex;
  gap: 6px;
  margin: 2px 0 0;
  font-size: 11.5px;
  line-height: 1.6;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 0;
  cursor: pointer;
}
.check input {
  accent-color: var(--accent);
}
.check .faint {
  margin-left: auto;
  font-size: 12px;
}
.enc-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 8px 0 10px;
  padding-top: 10px;
  border-top: 1px dashed var(--line);
}
.enc-row span {
  display: flex;
  align-items: center;
  gap: 6px;
}
.full {
  width: 100%;
}
.meter {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 4px;
  margin: 6px 0 8px;
}
.meter i {
  height: 4px;
  border-radius: 2px;
  background: var(--line);
}
.meter i.on {
  background: var(--good);
}
.meter i.on.weak {
  background: var(--mid);
}
.small {
  font-size: 12px;
  line-height: 1.6;
}
.danger-note {
  display: flex;
  gap: 6px;
  color: var(--bad);
}
.problem {
  margin-right: auto;
}
.file {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  background: var(--hover);
  margin-bottom: 8px;
  min-width: 0;
}
.pw .row {
  gap: 8px;
}
.summary {
  display: flex;
  gap: 6px;
  margin: 4px 0 6px;
}
.sub {
  padding: 0 0 6px 24px;
}
.sub p {
  margin: 6px 0 0;
}
</style>
