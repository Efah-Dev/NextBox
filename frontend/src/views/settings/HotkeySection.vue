<script setup lang="ts">
// 快捷键: Ctrl+V (fixed, inside the window) and the system-wide hotkeys,
// which work while NextBox is in the tray. None is set by default.
import { computed, onUnmounted, ref } from 'vue'
import { t } from '../../i18n'
import Card from '../../components/Card.vue'
import HelpTip from '../../components/HelpTip.vue'
import { api, type Settings } from '../../api'

const props = defineProps<{ form: Settings }>()

// keep in sync with hotkeyActions in internal/app/hotkeys.go
const actions = [
  { id: 'toggleProxy', label: t('连接 / 断开') },
  { id: 'systemProxy', label: t('系统代理开关') },
  { id: 'tun', label: t('TUN 模式开关') },
  { id: 'routeMode', label: t('切换路由模式'), hint: t('规则分流 → 全局代理 → 全部直连') },
  { id: 'showWindow', label: t('显示主界面') },
  { id: 'testGroup', label: t('当前分组测延迟') },
]

const keys = computed(() => props.form.hotkeys ?? {})
const recording = ref('')
const errors = ref<Record<string, string>>({})

// KeyboardEvent.code → the names internal/sys/combo.go knows
function keyName(code: string): string {
  if (/^Key[A-Z]$/.test(code)) return code.slice(3)
  if (/^Digit\d$/.test(code)) return code.slice(5)
  if (/^Numpad\d$/.test(code)) return 'Num' + code.slice(6)
  if (/^F\d{1,2}$/.test(code)) return code
  if (code.startsWith('Arrow')) return code.slice(5)
  const same = ['Space', 'PageUp', 'PageDown', 'End', 'Home', 'Insert', 'Delete', 'Backquote', 'Minus', 'Equal', 'BracketLeft', 'BracketRight', 'Backslash', 'Semicolon', 'Quote', 'Comma', 'Period', 'Slash']
  return same.includes(code) ? code : ''
}
const shown = (combo: string) => combo.split('+')

function record(id: string) {
  errors.value = { ...errors.value, [id]: '' }
  recording.value = id
  window.addEventListener('keydown', onKey, true)
}
function stop() {
  recording.value = ''
  window.removeEventListener('keydown', onKey, true)
}
onUnmounted(stop)

function set(id: string, combo: string) {
  const next = { ...keys.value }
  if (combo) next[id] = combo
  else delete next[id]
  props.form.hotkeys = next
}

async function onKey(e: KeyboardEvent) {
  e.preventDefault()
  e.stopPropagation()
  const id = recording.value
  if (!id) return
  if (e.key === 'Escape') return stop()
  if ((e.key === 'Backspace' || e.key === 'Delete') && !e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey) {
    set(id, '')
    return stop()
  }
  const key = keyName(e.code)
  if (!key) return // a modifier alone: wait for the key
  const mods = [e.ctrlKey && 'Ctrl', e.altKey && 'Alt', e.shiftKey && 'Shift', e.metaKey && 'Win'].filter(Boolean) as string[]
  const combo = [...mods, key].join('+')
  stop()
  if (!mods.length && !/^F\d/.test(key)) return (errors.value = { ...errors.value, [id]: t('组合键至少要有 Ctrl、Alt、Shift 或 Win 中的一个') })
  const other = actions.find((a) => a.id !== id && keys.value[a.id] === combo)
  if (other) return (errors.value = { ...errors.value, [id]: t('已用于「{0}」', other.label) })
  try {
    await api.testHotkey(combo)
    set(id, combo)
  } catch (err) {
    errors.value = { ...errors.value, [id]: (err as Error).message }
  }
}
</script>

<template>
  <Card id="sec-hotkeys" :title="t('快捷键')">
    <div class="form-row">
      <div class="label">
        <span>{{ t('粘贴导入') }}<HelpTip :text="t('在「启动」或「节点」页按 Ctrl+V，读取剪贴板，自动判断是订阅链接还是节点链接并导入。光标在输入框里时是普通粘贴。')" /></span>
        <span class="hint">{{ t('仅在窗口内有效，不可修改') }}</span>
      </div>
      <div class="control">
        <span class="kbd"><kbd>Ctrl</kbd><kbd>V</kbd></span>
      </div>
    </div>
    <div v-for="a in actions" :key="a.id" class="form-row">
      <div class="label">
        <span>{{ a.label }}</span>
        <span class="hint" :class="{ err: errors[a.id] }">{{ errors[a.id] || a.hint || t('全局，NextBox 在托盘时也有效') }}</span>
      </div>
      <div class="control">
        <span v-if="recording === a.id" class="recording">{{ t('请按下组合键，Esc 取消') }}</span>
        <span v-else-if="keys[a.id]" class="kbd"><kbd v-for="k in shown(keys[a.id])" :key="k">{{ k }}</kbd></span>
        <span v-else class="faint">{{ t('未设置') }}</span>
        <button v-if="recording === a.id" class="btn small" @click="stop">{{ t('取消') }}</button>
        <template v-else>
          <button class="btn small" @click="record(a.id)">{{ keys[a.id] ? t('修改') : t('设置') }}</button>
          <button v-if="keys[a.id]" class="btn ghost small" @click="set(a.id, '')">{{ t('清除') }}</button>
        </template>
      </div>
    </div>
  </Card>
</template>

<style scoped>
.kbd {
  display: inline-flex;
  gap: 4px;
}
kbd {
  font: 600 11.5px var(--mono);
  min-width: 22px;
  padding: 2px 7px;
  text-align: center;
  border-radius: 5px;
  background: var(--card);
  border: 1px solid var(--input-border);
  border-bottom-width: 2px;
  color: var(--text);
}
.recording {
  padding: 4px 10px;
  border: 1px dashed var(--accent);
  border-radius: var(--radius-sm);
  color: var(--accent);
  font-size: 12px;
  animation: blink 1.2s ease-in-out infinite;
}
@keyframes blink {
  50% {
    opacity: 0.55;
  }
}
.hint.err {
  color: var(--bad);
}
</style>
