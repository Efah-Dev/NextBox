<script setup lang="ts">
// How "bypass mainland China" learns the IP of a domain name
// (Settings.bypassDns): a small button in the step's last column, like the
// delete button of a rule, opening the two choices. The remote DNS is the
// default; the direct one only after a warning, because it can answer
// blocked sites with wrong addresses and the proxy is then sent to those.
import { onBeforeUnmount, ref } from 'vue'
import Icon from '../../components/Icon.vue'
import { t } from '../../i18n'
import { confirmDialog } from '../../util'

const model = defineModel<string | undefined>()

interface ResolveChoice {
  value: 'remote' | 'direct'
  label: string
  hint: string
}

const choices = (): ResolveChoice[] => [
  { value: 'remote', label: t('接管 DNS 解析'), hint: t('经代理用远程 DNS 解析，不会被污染') },
  { value: 'direct', label: t('直连 DNS 解析'), hint: t('能认出国内 CDN，被封锁的网站可能解析出错') },
]
const current = () => (model.value === 'direct' ? 'direct' : 'remote')

const open = ref(false)
const root = ref<HTMLElement>()
function onDoc(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) close()
}
function close() {
  open.value = false
  document.removeEventListener('mousedown', onDoc)
}
// The menu opens towards the side with room: to the left of the button
// unless that leaves the visible part of the page (the cards on the left
// edge), and downwards unless the page ends first. The page is the nearest
// scrolling container, which cuts off what lies outside it.
const MENU_W = 280
const MENU_H = 150
const toRight = ref(false)
const upwards = ref(false)
function clipRect(el: HTMLElement): DOMRect | { left: number; right: number; top: number; bottom: number } {
  for (let p = el.parentElement; p; p = p.parentElement) {
    const s = getComputedStyle(p)
    if (/(auto|scroll|hidden)/.test(s.overflowX + s.overflowY)) return p.getBoundingClientRect()
  }
  return { left: 0, right: window.innerWidth, top: 0, bottom: window.innerHeight }
}
function toggle() {
  if (open.value) return close()
  if (root.value) {
    const r = root.value.getBoundingClientRect()
    const c = clipRect(root.value)
    toRight.value = r.right - MENU_W < c.left + 8 && r.left + MENU_W <= c.right - 8
    upwards.value = r.bottom + MENU_H > c.bottom - 8 && r.top - MENU_H >= c.top + 8
  }
  open.value = true
  document.addEventListener('mousedown', onDoc)
}
async function pick(v: 'remote' | 'direct') {
  close()
  if (v === current()) return
  if (v === 'direct') {
    const ok = await confirmDialog(
      t('改用直连 DNS 解析？'),
      t('直连 DNS 会把一些被封锁的网站解析成错误的地址，代理随后连到这些地址，表现为已连接却打不开网页。已知的境外网站不受影响，冷门网站可能受影响。只有在国内网站被误判走代理、又确实需要直连时再用。'),
      true,
      t('仍然使用'),
    )
    if (!ok) return
  }
  model.value = v
}
onBeforeUnmount(() => document.removeEventListener('mousedown', onDoc))
</script>

<template>
  <div ref="root" class="resolve" @pointerdown.stop>
    <button class="btn ghost small icon" :class="{ on: open }" :title="t('解析方式')" @click.stop="toggle">
      <Icon name="dns" :size="14" />
    </button>
    <Transition name="nb-pop">
      <div v-if="open" class="menu" :class="{ right: toRight, up: upwards }">
        <div class="menu-title">{{ t('解析方式') }}</div>
        <button v-for="c in choices()" :key="c.value" class="menu-item" :class="{ sel: c.value === current() }" @click.stop="pick(c.value)">
          <span class="mark"><Icon v-if="c.value === current()" name="check" :size="14" /></span>
          <span class="menu-text">
            <span>{{ c.label }}</span>
            <span class="menu-hint">{{ c.hint }}</span>
          </span>
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.resolve {
  position: relative;
  display: flex;
  justify-content: center;
}
.btn.on {
  color: var(--accent);
  background: var(--accent-soft);
}
.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  width: 280px;
  padding: 5px;
  background: var(--card);
  border: 1px solid var(--card-border);
  border-radius: 8px;
  box-shadow: 0 10px 30px rgba(10, 30, 70, 0.18);
  z-index: 40;
  text-align: left;
}
.menu.right {
  right: auto;
  left: 0;
}
.menu.up {
  top: auto;
  bottom: calc(100% + 6px);
}
.menu.right.nb-pop-enter-active,
.menu.right.nb-pop-leave-active {
  transform-origin: top left;
}
.menu-title {
  padding: 6px 10px 4px;
  font-size: 11px;
  font-weight: 700;
  color: var(--text-3);
}
.menu-item {
  width: 100%;
  border: none;
  background: transparent;
  border-radius: 5px;
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  align-items: start;
  gap: 8px;
  padding: 7px 10px;
  font: inherit;
  font-size: 12.5px;
  font-weight: 400;
  color: var(--text);
  cursor: pointer;
  text-align: left;
}
.menu-item:hover {
  background: var(--accent-soft);
}
.menu-item.sel {
  color: var(--accent);
  font-weight: 700;
}
.mark {
  height: 18px;
  display: grid;
  place-items: center;
}
.menu-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.menu-hint {
  font-size: 11px;
  font-weight: 400;
  color: var(--text-3);
  white-space: normal;
}
.nb-pop-enter-active,
.nb-pop-leave-active {
  transform-origin: top right;
}
</style>
