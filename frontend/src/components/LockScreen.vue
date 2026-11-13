<script setup lang="ts">
// Shown at start-up when the node list is kept with a start-up password
// (设置 → 数据与备份). Nothing else is usable until it is unlocked.
import { t } from '../i18n'
import { nextTick, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import PawLogo from './PawLogo.vue'
import { api } from '../api'

const password = ref('')
const error = ref('')
const busy = ref(false)
const input = ref<HTMLInputElement>()

onMounted(() => nextTick(() => input.value?.focus()))

async function unlock() {
  if (!password.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await api.unlock(password.value)
  } catch (e) {
    error.value = (e as Error).message
    password.value = ''
    nextTick(() => input.value?.focus())
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="lock">
    <form class="box" @submit.prevent="unlock">
      <span class="logo"><PawLogo :size="40" /></span>
      <div class="title">{{ t('NextBox 已锁定') }}</div>
      <p class="faint">{{ t('节点和订阅已加密保存，请输入启动密码') }}</p>
      <div class="field" :class="{ bad: error }">
        <Icon name="lock" :size="15" />
        <input ref="input" v-model="password" type="password" class="input" :placeholder="t('启动密码')" autocomplete="current-password" />
      </div>
      <div v-if="error" class="err">{{ error }}</div>
      <button class="btn primary" type="submit" :disabled="!password || busy">
        {{ busy ? t('正在解锁…') : t('解锁') }}
      </button>
      <p class="tip faint">{{ t('忘记密码时无法恢复节点。可以删除数据目录里的 profiles.json 重新开始，设置不受影响。') }}</p>
    </form>
  </div>
</template>

<style scoped>
.lock {
  position: fixed;
  inset: var(--titlebar) 0 0 0;
  z-index: 90;
  display: grid;
  place-items: center;
  background: var(--bg);
}
.box {
  width: 340px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  text-align: center;
  animation: card-in 0.35s cubic-bezier(0.2, 0.8, 0.3, 1) both;
}
.logo {
  width: 72px;
  height: 72px;
  border-radius: 20px;
  background: var(--accent-soft);
  display: grid;
  place-items: center;
  margin-bottom: 6px;
}
.title {
  font-size: 18px;
  font-weight: 800;
}
.box p {
  margin: 0;
}
.field {
  width: 100%;
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 0 0 12px;
  border: 1px solid var(--input-border);
  border-radius: var(--radius);
  background: var(--input-bg);
  color: var(--text-3);
}
.field:focus-within {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}
.field.bad {
  border-color: var(--bad);
}
.field .input {
  flex: 1;
  border: 0;
  box-shadow: none;
  background: transparent;
  height: 38px;
}
.err {
  color: var(--bad);
  font-size: 12px;
}
.btn {
  width: 100%;
  height: 38px;
  justify-content: center;
}
.tip {
  font-size: 11.5px;
  line-height: 1.6;
  margin-top: 8px !important;
}
</style>
