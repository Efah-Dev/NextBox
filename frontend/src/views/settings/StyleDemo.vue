<script setup lang="ts">
// A small looping animation of one look of the routing rules page, for the
// guide that helps choosing one (RouteStyleGuide). Plain CSS, no images.
import { t } from '../../i18n'
import ServiceIcon from '../../components/ServiceIcon.vue'
import { serviceOf } from '../../services'
import type { RouteView } from '../../api'

defineProps<{ kind: RouteView }>()
const demoServices = ['youtube', 'chatgpt', 'bilibili'].map((id) => serviceOf(id)!)
</script>

<template>
  <div class="demo" :class="kind" aria-hidden="true">
    <!-- the route: a dot runs down the steps and the matching one lights up -->
    <template v-if="kind === 'path'">
      <div class="rail" />
      <div v-for="n in 4" :key="n" class="step" :class="'s' + n">
        <span class="no">{{ n }}</span>
        <span class="bar" :style="{ width: [62, 48, 70, 54][n - 1] + '%' }" />
        <span class="to" />
      </div>
      <span class="runner" />
    </template>

    <!-- the cards: the scene switches turn on one after the other -->
    <template v-else-if="kind === 'cards'">
      <div v-for="n in 4" :key="n" class="card" :class="'c' + n">
        <span class="title" :style="{ width: [58, 44, 66, 50][n - 1] + '%' }" />
        <span class="sw"><span class="knob" /></span>
        <span class="line" />
      </div>
    </template>

    <!-- the catalog: services get where they go with one pick -->
    <template v-else>
      <div v-for="(sv, n) in demoServices" :key="sv.id" class="row" :class="'r' + (n + 1)">
        <ServiceIcon :service="sv" :size="20" />
        <span class="name">{{ sv.name }}</span>
        <span class="pick">
          <span class="v follow">{{ t('跟随') }}</span>
          <span class="v" :class="n === 2 ? 'direct' : 'proxy'">{{ n === 2 ? t('直连') : t('代理') }}</span>
        </span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.demo {
  position: relative;
  height: 128px;
  padding: 12px 14px;
  border-radius: 10px;
  background: var(--bg);
  border: 1px solid var(--line);
  overflow: hidden;
  --dur: 3.6s;
}

/* ---- path */
.rail {
  position: absolute;
  left: 23px;
  top: 18px;
  bottom: 18px;
  width: 2px;
  background: var(--line);
}
.step {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 22px;
  margin-bottom: 4px;
  padding-right: 6px;
  border-radius: 6px;
  background: var(--card);
  box-shadow: var(--shadow);
}
.step .no {
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  margin-left: 1px;
  border-radius: 50%;
  background: var(--line);
  color: var(--text-3);
  font-size: 10px;
  font-weight: 700;
  z-index: 1;
}
.step .bar {
  height: 6px;
  border-radius: 3px;
  background: var(--line);
}
.step .to {
  margin-left: auto;
  width: 26px;
  height: 8px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--text-3) 40%, transparent);
}
.step.s3 {
  animation: hit var(--dur) infinite;
}
.step.s3 .no {
  animation: hit-no var(--dur) infinite;
}
.step.s3 .to {
  animation: hit-to var(--dur) infinite;
}
.step.s1 .no,
.step.s2 .no {
  animation: pass var(--dur) infinite;
}
.step.s2 .no {
  animation-delay: 0.35s;
}
.runner {
  position: absolute;
  left: 19px;
  top: 16px;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 4px var(--accent-soft-2);
  animation: run var(--dur) infinite;
}
@keyframes run {
  0% {
    transform: translateY(0);
    opacity: 0;
  }
  8% {
    opacity: 1;
  }
  40%,
  78% {
    transform: translateY(52px);
    opacity: 1;
  }
  90%,
  100% {
    transform: translateY(52px);
    opacity: 0;
  }
}
@keyframes pass {
  0%,
  10% {
    background: var(--line);
    color: var(--text-3);
  }
  18%,
  80% {
    background: color-mix(in srgb, var(--accent) 35%, var(--card));
    color: #fff;
  }
  92% {
    background: var(--line);
    color: var(--text-3);
  }
}
@keyframes hit {
  0%,
  38% {
    box-shadow: var(--shadow);
  }
  44%,
  80% {
    box-shadow: 0 0 0 2px var(--accent), 0 4px 12px var(--accent-soft-2);
  }
  92% {
    box-shadow: var(--shadow);
  }
}
@keyframes hit-no {
  0%,
  38% {
    background: var(--line);
    color: var(--text-3);
  }
  44%,
  80% {
    background: var(--accent);
    color: #fff;
  }
  92% {
    background: var(--line);
    color: var(--text-3);
  }
}
@keyframes hit-to {
  0%,
  38% {
    background: color-mix(in srgb, var(--text-3) 40%, transparent);
  }
  44%,
  80% {
    background: var(--accent);
  }
  92% {
    background: color-mix(in srgb, var(--text-3) 40%, transparent);
  }
}

/* ---- cards */
.cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.card {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  padding: 0 10px;
  border-radius: 8px;
  background: var(--card);
  border: 1px solid var(--line);
  animation: card-on var(--dur) infinite;
}
.card .title {
  height: 7px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--text-3) 55%, transparent);
}
.card .line {
  width: 80%;
  height: 5px;
  border-radius: 3px;
  background: var(--line);
}
.sw {
  position: absolute;
  right: 8px;
  top: 8px;
  width: 22px;
  height: 12px;
  border-radius: 6px;
  background: var(--line);
  animation: sw-on var(--dur) infinite;
}
.knob {
  position: absolute;
  left: 2px;
  top: 2px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  animation: knob-on var(--dur) infinite;
}
.c2,
.c2 * {
  animation-delay: 0.45s !important;
}
.c3,
.c3 * {
  animation-delay: 0.9s !important;
}
/* the fourth stays off: not every scene is needed */
.c4,
.c4 * {
  animation: none !important;
}
@keyframes card-on {
  0%,
  15% {
    border-color: var(--line);
    background: var(--card);
  }
  25%,
  80% {
    border-color: var(--accent-line);
    background: var(--accent-soft);
  }
  92% {
    border-color: var(--line);
    background: var(--card);
  }
}
@keyframes sw-on {
  0%,
  15% {
    background: var(--line);
  }
  25%,
  80% {
    background: var(--accent);
  }
  92% {
    background: var(--line);
  }
}
@keyframes knob-on {
  0%,
  15% {
    transform: none;
  }
  25%,
  80% {
    transform: translateX(10px);
  }
  92% {
    transform: none;
  }
}

/* ---- catalog */
.apps {
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.row {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 28px;
  padding: 0 6px;
  border-radius: 7px;
  background: var(--card);
  box-shadow: var(--shadow);
  font-size: 11.5px;
  font-weight: 700;
}
.row .name {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.pick {
  position: relative;
  width: 50px;
  height: 18px;
  border-radius: 5px;
  border: 1px solid var(--input-border);
  overflow: hidden;
  font-size: 10.5px;
}
.v {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
}
.v.follow {
  color: var(--text-3);
  animation: out var(--dur) infinite;
}
.v.proxy,
.v.direct {
  opacity: 0;
  animation: in var(--dur) infinite;
}
.v.proxy {
  color: var(--accent);
}
.v.direct {
  color: var(--good);
}
.r2 .v {
  animation-delay: 0.5s;
}
.r3 .v {
  animation-delay: 1s;
}
@keyframes out {
  0%,
  18% {
    opacity: 1;
    transform: none;
  }
  26%,
  82% {
    opacity: 0;
    transform: translateY(-8px);
  }
  92% {
    opacity: 1;
    transform: none;
  }
}
@keyframes in {
  0%,
  18% {
    opacity: 0;
    transform: translateY(8px);
  }
  26%,
  82% {
    opacity: 1;
    transform: none;
  }
  92%,
  100% {
    opacity: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .demo * {
    animation: none !important;
  }
}
</style>
