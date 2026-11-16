<script setup lang="ts">
// The guide shown the first time the routing rules are opened (once, also to
// users updating from an older version): how well do you know proxy tools?
// The answer picks the look that suits, which can still be changed here and
// later under 规则样式.
import { computed, ref, watch } from 'vue'
import { t } from '../../i18n'
import Modal from '../../components/Modal.vue'
import StyleDemo from './StyleDemo.vue'
import type { RouteView } from '../../api'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ current: RouteView }>()
const emit = defineEmits<{ done: [view: RouteView | null] }>()

const levels = [
  { id: 'new', icon: '🌱', lv: 1, name: t('我是新手'), text: t('第一次用代理工具，想点几下就能用'), view: 'apps' as RouteView },
  { id: 'some', icon: '🧭', lv: 2, name: t('我一知半解'), text: t('知道哪些网站要走代理，也想自己加几条规则'), view: 'cards' as RouteView },
  { id: 'pro', icon: '🦊', lv: 3, name: t('我是老鸟'), text: t('熟悉分流规则，想看清每一步的匹配顺序'), view: 'path' as RouteView },
]
const styles = [
  {
    id: 'apps' as RouteView,
    name: t('服务目录'),
    for: t('适合新手'),
    points: [t('50 多个常用服务，带图标一眼找到'), t('不用懂规则，每个服务点一下选好去向')],
  },
  {
    id: 'cards' as RouteView,
    name: t('卡片'),
    for: t('适合一知半解'),
    points: [t('打开几个常用场景开关就能用'), t('也能自己添加规则，拖动调整顺序')],
  },
  {
    id: 'path' as RouteView,
    name: t('路由路径'),
    for: t('适合老鸟'),
    points: [t('从上到下看清每一步怎么匹配'), t('拖动调整顺序，输入网址测试命中哪一步')],
  },
]

const step = ref<1 | 2>(1)
const level = ref('')
const pick = ref<RouteView>('path')
const recommended = computed(() => levels.find((l) => l.id === level.value)?.view ?? '')

let answered = false
watch(open, (v) => {
  if (v) {
    step.value = 1
    level.value = ''
    pick.value = props.current
    answered = false
  } else if (!answered) {
    emit('done', null) // closed with ✕ or a click outside: keep the current look
  }
})

function choose(id: string) {
  level.value = id
  pick.value = levels.find((l) => l.id === id)!.view
  step.value = 2
}
function done(view: RouteView | null) {
  answered = true
  emit('done', view)
  open.value = false
}
</script>

<template>
  <Modal v-model="open" :title="t('选择分流规则的样式')" :width="760">
    <Transition name="slide" mode="out-in">
      <div v-if="step === 1" key="1" class="pane">
        <div class="hero">
          <div class="hero-title">{{ t('先选一下你的段位') }}</div>
          <div class="faint">{{ t('按你对代理工具的熟悉程度，推荐一种最顺手的样式。三种样式改的是同一份规则，以后随时可以换。') }}</div>
        </div>
        <div class="levels">
          <button v-for="(l, i) in levels" :key="l.id" class="level" :style="{ animationDelay: i * 80 + 'ms' }" @click="choose(l.id)">
            <span class="emoji">{{ l.icon }}</span>
            <span class="lv">
              <span v-for="n in 3" :key="n" class="star" :class="{ on: n <= l.lv }">★</span>
            </span>
            <b>{{ l.name }}</b>
            <span class="faint">{{ l.text }}</span>
          </button>
        </div>
        <div class="skip">
          <button class="btn ghost small" @click="step = 2">{{ t('我想自己挑') }}</button>
        </div>
      </div>

      <div v-else key="2" class="pane">
        <div class="hero">
          <div class="hero-title">{{ recommended ? t('为你推荐「{0}」样式', styles.find((s) => s.id === recommended)!.name) : t('三种样式，挑一个') }}</div>
          <div class="faint">{{ t('看看动画演示，点一下就能选中。') }}</div>
        </div>
        <div class="styles">
          <button v-for="s in styles" :key="s.id" class="style" :class="{ on: pick === s.id }" @click="pick = s.id">
            <span v-if="recommended === s.id" class="ribbon">{{ t('推荐') }}</span>
            <StyleDemo :kind="s.id" />
            <span class="style-name">
              <b>{{ s.name }}</b>
              <span class="for">{{ s.for }}</span>
            </span>
            <ul>
              <li v-for="p in s.points" :key="p">{{ p }}</li>
            </ul>
            <span class="radio" />
          </button>
        </div>
      </div>
    </Transition>
    <template #footer>
      <button v-if="step === 2" class="btn ghost" @click="step = 1">{{ t('上一步') }}</button>
      <span class="spacer" />
      <button class="btn" @click="done(null)">{{ t('以后再说') }}</button>
      <button v-if="step === 2" class="btn primary" @click="done(pick)">{{ t('就用「{0}」', styles.find((s) => s.id === pick)!.name) }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.pane {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.hero {
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12.5px;
}
.hero-title {
  font-size: 18px;
  font-weight: 800;
}
.levels {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.level {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 20px 14px 18px;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: var(--card);
  color: var(--text);
  font: inherit;
  font-size: 12px;
  text-align: center;
  line-height: 1.6;
  cursor: pointer;
  animation: rise 0.35s both;
  transition: transform 0.18s, border-color 0.18s, box-shadow 0.18s;
}
.level:hover {
  transform: translateY(-4px);
  border-color: var(--accent);
  box-shadow: 0 10px 26px var(--accent-soft-2);
}
.level:hover .emoji {
  transform: scale(1.15) rotate(-6deg);
}
.level b {
  font-size: 15px;
}
.emoji {
  font-size: 40px;
  line-height: 1;
  font-family: 'Segoe UI Emoji', 'Noto Color Emoji', sans-serif;
  transition: transform 0.25s cubic-bezier(0.3, 1.6, 0.5, 1);
}
.lv {
  display: flex;
  gap: 2px;
  font-size: 12px;
}
.star {
  color: var(--line);
}
.star.on {
  color: #f5b301;
}
.skip {
  text-align: center;
}
.styles {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.style {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px 10px 12px;
  border: 1px solid var(--line);
  border-radius: 12px;
  background: var(--card);
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s, box-shadow 0.18s;
}
.style:hover {
  border-color: var(--accent-line);
}
.style.on {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft-2);
}
.ribbon {
  position: absolute;
  top: -9px;
  left: 12px;
  z-index: 1;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}
.style-name {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 0 4px;
}
.style-name b {
  font-size: 14px;
}
.for {
  font-size: 11px;
  color: var(--accent);
  font-weight: 700;
}
ul {
  margin: 0;
  padding: 0 4px 0 20px;
  font-size: 12px;
  color: var(--text-2);
  line-height: 1.7;
}
.radio {
  position: absolute;
  right: 12px;
  bottom: 12px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid var(--input-border);
  transition: all 0.15s;
}
.style.on .radio {
  border: 5px solid var(--accent);
}
.spacer {
  flex: 1;
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
}
.slide-enter-active,
.slide-leave-active {
  transition: opacity 0.18s, transform 0.18s;
}
.slide-enter-from {
  opacity: 0;
  transform: translateX(16px);
}
.slide-leave-to {
  opacity: 0;
  transform: translateX(-16px);
}
@media (max-width: 760px) {
  .levels,
  .styles {
    grid-template-columns: 1fr;
  }
}
@media (prefers-reduced-motion: reduce) {
  .level {
    animation: none;
  }
}
</style>
