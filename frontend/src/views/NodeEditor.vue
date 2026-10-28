<script setup lang="ts">
import { t } from '../i18n'
import { computed, ref, watch } from 'vue'
import Modal from '../components/Modal.vue'
import Switch from '../components/Switch.vue'
import Segmented from '../components/Segmented.vue'
import { api, type Outbound, type Profile } from '../api'
import { run } from '../util'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ profile: Profile | null; groupId: number }>()

type Kind = 'text' | 'number' | 'select' | 'switch' | 'list'
interface Field {
  key: string
  label: string
  kind?: Kind
  options?: string[]
  placeholder?: string
  show?: (ob: Outbound) => boolean
}

const TLS_TYPES = ['vmess', 'vless', 'trojan', 'http', 'hysteria2', 'tuic', 'anytls', 'shadowtls']
const TRANSPORT_TYPES = ['vmess', 'vless', 'trojan']
const hasTLS = (ob: Outbound) => !!ob.tls?.enabled
const tr = (...t: string[]) => (ob: Outbound) => t.includes(ob.transport?.type ?? '')

const protocolFields: Record<string, Field[]> = {
  shadowsocks: [
    {
      key: 'method', label: t('加密方式'), kind: 'select',
      options: ['2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305',
        'aes-128-gcm', 'aes-256-gcm', 'chacha20-ietf-poly1305', 'xchacha20-ietf-poly1305', 'none'],
    },
    { key: 'password', label: t('密码') },
    { key: 'plugin', label: t('插件'), kind: 'select', options: ['', 'obfs-local', 'v2ray-plugin'] },
    { key: 'plugin_opts', label: t('插件参数'), placeholder: 'obfs=http;obfs-host=example.com', show: (o) => !!o.plugin },
  ],
  vmess: [
    { key: 'uuid', label: 'UUID' },
    { key: 'security', label: t('加密'), kind: 'select', options: ['auto', 'aes-128-gcm', 'chacha20-poly1305', 'none', 'zero'] },
    { key: 'alter_id', label: 'Alter ID', kind: 'number' },
  ],
  vless: [
    { key: 'uuid', label: 'UUID' },
    { key: 'flow', label: 'Flow', kind: 'select', options: ['', 'xtls-rprx-vision'] },
    { key: 'packet_encoding', label: t('UDP 封装'), kind: 'select', options: ['', 'xudp', 'packetaddr'] },
  ],
  trojan: [{ key: 'password', label: t('密码') }],
  hysteria2: [
    { key: 'password', label: t('密码') },
    { key: 'up_mbps', label: t('上行 Mbps'), kind: 'number', placeholder: t('留空使用 BBR') },
    { key: 'down_mbps', label: t('下行 Mbps'), kind: 'number', placeholder: t('留空使用 BBR') },
    { key: 'obfs.type', label: t('混淆'), kind: 'select', options: ['', 'salamander'] },
    { key: 'obfs.password', label: t('混淆密码'), show: (o) => !!o.obfs?.type },
    { key: 'server_ports', label: t('端口跳跃'), kind: 'list', placeholder: '20000:30000, 40000:50000' },
  ],
  tuic: [
    { key: 'uuid', label: 'UUID' },
    { key: 'password', label: t('密码') },
    { key: 'congestion_control', label: t('拥塞控制'), kind: 'select', options: ['bbr', 'cubic', 'new_reno'] },
    { key: 'udp_relay_mode', label: t('UDP 中继'), kind: 'select', options: ['native', 'quic'] },
  ],
  anytls: [{ key: 'password', label: t('密码') }],
  socks: [
    { key: 'version', label: t('版本'), kind: 'select', options: ['5', '4a', '4'] },
    { key: 'username', label: t('用户名') },
    { key: 'password', label: t('密码') },
  ],
  http: [
    { key: 'username', label: t('用户名') },
    { key: 'password', label: t('密码') },
  ],
}

const tlsFields: Field[] = [
  { key: 'tls.enabled', label: t('启用 TLS'), kind: 'switch' },
  { key: 'tls.server_name', label: 'SNI', show: hasTLS },
  { key: 'tls.insecure', label: t('跳过证书验证'), kind: 'switch', show: hasTLS },
  { key: 'tls.alpn', label: 'ALPN', kind: 'list', placeholder: 'h2, http/1.1', show: hasTLS },
  {
    key: 'tls.utls.fingerprint', label: t('uTLS 指纹'), kind: 'select',
    options: ['', 'chrome', 'firefox', 'edge', 'safari', 'ios', 'android', 'random', 'randomized'], show: hasTLS,
  },
  { key: 'tls.reality.public_key', label: t('Reality 公钥'), show: hasTLS },
  { key: 'tls.reality.short_id', label: 'Reality Short ID', show: (o) => hasTLS(o) && !!o.tls?.reality?.public_key },
]

const transportFields: Field[] = [
  { key: 'transport.type', label: t('传输方式'), kind: 'select', options: ['', 'ws', 'grpc', 'http', 'httpupgrade', 'quic'] },
  { key: 'transport.path', label: t('路径'), show: tr('ws', 'http', 'httpupgrade') },
  { key: 'transport.headers.Host', label: 'Host', show: tr('ws') },
  { key: 'transport.host', label: 'Host', show: tr('httpupgrade') },
  { key: 'transport.host', label: t('Host 列表'), kind: 'list', show: tr('http') },
  { key: 'transport.service_name', label: t('gRPC 服务名'), show: tr('grpc') },
  { key: 'transport.max_early_data', label: t('0-RTT 长度'), kind: 'number', show: tr('ws') },
]

const types = Object.keys(protocolFields)

const mode = ref<'form' | 'json'>('form')
const name = ref('')
const ob = ref<Outbound>({})
const jsonText = ref('')
const jsonError = ref('')

watch(open, (v) => {
  if (!v) return
  mode.value = 'form'
  jsonError.value = ''
  name.value = props.profile?.name ?? ''
  ob.value = props.profile
    ? JSON.parse(JSON.stringify(props.profile.outbound))
    : { type: 'vless', server: '', server_port: 443, uuid: '' }
})

const sections = computed(() => {
  const typ = ob.value.type as string
  const list: { title: string; fields: Field[] }[] = [
    { title: t('服务器'), fields: [{ key: 'server', label: t('地址') }, { key: 'server_port', label: t('端口'), kind: 'number' }] },
  ]
  if (protocolFields[typ]) list.push({ title: t('协议'), fields: protocolFields[typ] })
  if (TRANSPORT_TYPES.includes(typ)) list.push({ title: t('传输'), fields: transportFields })
  if (TLS_TYPES.includes(typ)) {
    // hysteria2 / tuic / anytls always use TLS
    const always = ['hysteria2', 'tuic', 'anytls'].includes(typ)
    list.push({ title: 'TLS', fields: always ? tlsFields.slice(1).map((f) => ({ ...f, show: undefined })) : tlsFields })
  }
  return list.map((s) => ({ ...s, fields: s.fields.filter((f) => !f.show || f.show(ob.value)) }))
})

// ---- nested get / set with cleanup of empty values ----

function getPath(o: Outbound, path: string): any {
  return path.split('.').reduce((cur: any, k) => (cur == null ? undefined : cur[k]), o)
}

function setPath(o: Outbound, path: string, value: any) {
  const keys = path.split('.')
  const empty = value === '' || value === undefined || value === null || (Array.isArray(value) && !value.length)
  const parents: any[] = [o]
  let cur: any = o
  for (const k of keys.slice(0, -1)) {
    if (cur[k] == null || typeof cur[k] !== 'object') {
      if (empty) return
      cur[k] = {}
    }
    cur = cur[k]
    parents.push(cur)
  }
  const last = keys[keys.length - 1]
  if (empty) delete cur[last]
  else cur[last] = value
  // drop objects that became empty, e.g. transport: {}
  for (let i = keys.length - 2; i >= 0; i--) {
    const obj = parents[i + 1]
    if (Object.keys(obj).length === 0 || (Object.keys(obj).length === 1 && 'enabled' in obj && i > 0)) {
      delete parents[i][keys[i]]
    } else break
  }
}

function fieldValue(f: Field) {
  const v = getPath(ob.value, f.key)
  if (f.kind === 'list') return Array.isArray(v) ? v.join(', ') : v ?? ''
  if (f.kind === 'switch') return !!v
  return v ?? ''
}

function setField(f: Field, raw: any) {
  let v = raw
  if (f.kind === 'number') v = raw === '' ? '' : Number(raw)
  if (f.kind === 'list') v = String(raw).split(',').map((s) => s.trim()).filter(Boolean)
  if (f.kind === 'switch') v = raw ? true : ''
  const o = ob.value
  setPath(o, f.key, v)
  // keep sing-box "enabled" flags consistent with the fields being set
  if (f.key.startsWith('tls.utls.') && o.tls?.utls) o.tls.utls.enabled = true
  if (f.key.startsWith('tls.reality.') && o.tls?.reality) o.tls.reality.enabled = true
  if (f.key.startsWith('tls.') && f.key !== 'tls.enabled' && o.tls && v !== '') o.tls.enabled = true
  ob.value = { ...o }
}

function changeType(t: string) {
  const keep = { type: t, server: ob.value.server ?? '', server_port: ob.value.server_port ?? 443 } as Outbound
  if (['hysteria2', 'tuic', 'anytls'].includes(t)) keep.tls = { enabled: true, ...(t === 'tuic' ? { alpn: ['h3'] } : {}) }
  if (t === 'trojan') keep.tls = { enabled: true }
  ob.value = keep
}

function switchMode(m: string) {
  if (m === 'json') {
    jsonText.value = JSON.stringify(ob.value, null, 2)
    jsonError.value = ''
    mode.value = 'json'
  } else if (applyJSON()) {
    mode.value = 'form'
  }
}

function applyJSON(): boolean {
  try {
    const v = JSON.parse(jsonText.value)
    if (typeof v !== 'object' || !v || !v.type) throw new Error(t('需要一个包含 type 字段的出站对象'))
    delete v.tag
    ob.value = v
    jsonError.value = ''
    return true
  } catch (e) {
    jsonError.value = (e as Error).message
    return false
  }
}

const modeModel = computed({ get: () => mode.value, set: switchMode })

async function save() {
  if (mode.value === 'json' && !applyJSON()) return
  const p: Profile = props.profile
    ? { ...props.profile, name: name.value, outbound: ob.value }
    : { id: 0, groupId: props.groupId, name: name.value, type: ob.value.type, outbound: ob.value, latency: 0, upload: 0, download: 0 }
  const r = await run(() => api.saveProfile(p), t('已保存'))
  if (r) open.value = false
}
</script>

<template>
  <Modal v-model="open" :title="profile ? t('编辑节点') : t('新建节点')" :width="560">
    <div class="top">
      <label class="field grow">
        <span class="faint">{{ t('名称') }}</span>
        <input v-model="name" class="input" :placeholder="t('节点名称')" />
      </label>
      <label class="field">
        <span class="faint">{{ t('协议') }}</span>
        <select class="select" :value="ob.type" :disabled="mode === 'json'" @change="changeType(($event.target as HTMLSelectElement).value)">
          <option v-for="t in types" :key="t" :value="t">{{ t }}</option>
          <option v-if="ob.type && !types.includes(ob.type)" :value="ob.type">{{ ob.type }}</option>
        </select>
      </label>
    </div>

    <Segmented
      v-model="modeModel"
      class="mode"
      :options="[
        { value: 'form', label: t('表单') },
        { value: 'json', label: t('完整 JSON') },
      ]"
    />

    <template v-if="mode === 'form'">
      <div v-if="!protocolFields[ob.type]" class="faint note">{{ t('此协议没有表单，请使用“完整 JSON”编辑。') }}</div>
      <div v-for="s in sections" :key="s.title" class="section">
        <div class="section-title">{{ s.title }}</div>
        <div v-for="f in s.fields" :key="f.key + f.label" class="form-row">
          <div class="label">{{ f.label }}</div>
          <div class="control">
            <Switch v-if="f.kind === 'switch'" :model-value="fieldValue(f)" @update:model-value="setField(f, $event)" />
            <select
              v-else-if="f.kind === 'select'"
              class="select"
              :value="fieldValue(f)"
              @change="setField(f, ($event.target as HTMLSelectElement).value)"
            >
              <option v-for="o in f.options" :key="o" :value="o">{{ o || t('无') }}</option>
              <option v-if="fieldValue(f) && !f.options?.includes(fieldValue(f))" :value="fieldValue(f)">{{ fieldValue(f) }}</option>
            </select>
            <input
              v-else
              class="input"
              :class="{ mono: f.kind !== 'number' }"
              :type="f.kind === 'number' ? 'number' : 'text'"
              :value="fieldValue(f)"
              :placeholder="f.placeholder"
              @change="setField(f, ($event.target as HTMLInputElement).value)"
            />
          </div>
        </div>
      </div>
    </template>

    <template v-else>
      <p class="faint note">{{ t('这里是 sing-box 出站对象（不含 tag），可使用 sing-box 支持的全部字段。') }}</p>
      <textarea v-model="jsonText" class="textarea json" spellcheck="false" />
      <div v-if="jsonError" class="error">{{ jsonError }}</div>
    </template>

    <template #footer>
      <button class="btn" @click="open = false">{{ t('取消') }}</button>
      <button class="btn primary" @click="save">{{ t('保存') }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.top {
  display: flex;
  gap: 12px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 150px;
}
.field.grow {
  flex: 1;
}
.mode {
  margin: 14px 0 4px;
}
.section {
  margin-top: 10px;
}
.section-title {
  font-weight: 700;
  font-size: 12px;
  color: var(--accent);
  margin: 8px 0 0;
}
.form-row > .label {
  width: 110px;
  color: var(--text-2);
}
.note {
  margin: 12px 0 8px;
  font-size: 12px;
}
.json {
  min-height: 320px;
}
.error {
  margin-top: 8px;
  color: var(--bad);
  font-size: 12px;
}
</style>
