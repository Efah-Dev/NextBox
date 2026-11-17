<script setup lang="ts">
// 服务模式: the NextBox service runs the core for TUN mode, so that NextBox
// does not have to run as administrator, and starts NextBox at logon. It is
// optional: installing and removing it take one administrator prompt each,
// and without it TUN mode works by running NextBox as administrator.
import { computed, onMounted, ref } from 'vue'
import { t } from '../../i18n'
import Icon from '../../components/Icon.vue'
import { api, type ServiceInfo } from '../../api'
import { state } from '../../state'
import { confirmDialog, run } from '../../util'

const info = ref<ServiceInfo>()
const busy = ref('')
const refresh = async () => (info.value = await api.getServiceInfo().catch(() => undefined))
onMounted(refresh)

const status = computed(() => {
  const i = info.value
  if (!i) return { cls: '', text: t('正在检查…') }
  if (!i.installed) return { cls: '', text: t('未安装') }
  switch (i.problem) {
    case 'stopped':
      return { cls: 'bad', text: t('已安装，未运行') }
    case 'notAllowed':
      return { cls: 'bad', text: t('由其他 Windows 账户安装') }
    case 'outdated':
      return { cls: 'mid', text: i.usable ? t('运行中，可以更新') : t('需要更新') }
    case 'error':
      return { cls: 'bad', text: t('无法连接') }
  }
  return { cls: 'good', text: t('运行中') + (i.version ? ' · v' + i.version : '') }
})

const installLabel = computed(() => {
  const i = info.value
  if (!i?.installed) return t('安装服务')
  if (i.problem === 'notAllowed') return t('允许当前账户使用')
  if (i.problem === 'outdated') return t('更新服务')
  return t('重新安装')
})

async function install() {
  busy.value = 'install'
  const r = await run(() => api.installService(), t('NextBox 服务已就绪，TUN 模式不再需要管理员身份'))
  busy.value = ''
  if (r) info.value = r
  else refresh()
}

async function uninstall() {
  const ok = await confirmDialog(
    t('卸载 NextBox 服务'),
    t('会停止并删除服务，并删除 {0} 文件夹。之后使用 TUN 模式需要以管理员身份运行 NextBox，开机自启改回普通启动项。', info.value?.dir ?? ''),
    true,
    t('卸载'),
  )
  if (!ok) return
  busy.value = 'uninstall'
  const r = await run(() => api.uninstallService(), t('已卸载 NextBox 服务'))
  busy.value = ''
  if (r) info.value = r
  else refresh()
}
</script>

<template>
  <div class="svc">
    <div class="svc-head">
      <Icon name="shield" :size="16" />
      <b>{{ t('服务模式') }}</b>
      <span class="chip" :class="status.cls">{{ status.text }}</span>
      <span class="spacer" />
      <button class="btn ghost small icon" :title="t('刷新')" @click="refresh"><Icon name="refresh" :size="13" /></button>
    </div>
    <p class="faint text">
      {{ t('可选。安装一个 Windows 服务，由它以系统权限运行 TUN 模式的内核：只在安装和卸载时需要一次管理员授权，之后开启 TUN 模式不用再以管理员身份运行 NextBox，开机自启也改由服务在你登录 Windows 时启动 NextBox，不受启动项管理影响。') }}
    </p>
    <p class="faint text">
      {{ t('安装位置：') }}<span class="mono path">{{ info?.dir || 'C:\\Program Files\\NextBox Service' }}</span>{{ t('（服务程序和一份内核，只有管理员能修改）。不想安装也没关系：以管理员身份运行 NextBox 一样可以使用 TUN 模式。') }}
    </p>
    <p v-if="info && !info.canInstall" class="warn-line"><Icon name="warn" :size="13" /> {{ t('程序目录的 core 文件夹里缺少 NextBoxService.exe，请重新下载完整的 NextBox。') }}</p>
    <p v-if="info?.problem === 'outdated'" class="faint text">{{ t('服务来自另一个版本的 NextBox，或使用的内核和程序目录里的不同。点「更新服务」同步。') }}</p>
    <p v-if="info?.problem === 'error' && info.detail" class="warn-line"><Icon name="warn" :size="13" /> {{ info.detail }}</p>
    <div class="actions">
      <button class="btn small primary" :disabled="!!busy || !info?.canInstall" @click="install">
        <Icon name="admin" :size="13" /> {{ busy === 'install' ? t('请在弹出的窗口中授权…') : installLabel }}
      </button>
      <button v-if="info?.installed" class="btn small danger" :disabled="!!busy" @click="uninstall">
        {{ busy === 'uninstall' ? t('请在弹出的窗口中授权…') : t('卸载服务') }}
      </button>
      <span v-if="state.app?.elevated" class="faint small">{{ t('当前以管理员身份运行，TUN 模式不经过服务') }}</span>
    </div>
  </div>
</template>

<style scoped>
.svc {
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 12px 14px;
  margin: 4px 0 10px;
}
.svc-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.svc-head b {
  font-size: 13px;
}
.chip {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--hover);
  color: var(--text-2);
}
.chip.good {
  color: var(--good);
  background: color-mix(in srgb, var(--good) 12%, var(--card));
}
.chip.bad {
  color: var(--bad);
  background: color-mix(in srgb, var(--bad) 12%, var(--card));
}
.chip.mid {
  color: var(--mid);
  background: color-mix(in srgb, var(--mid) 14%, var(--card));
}
.spacer {
  flex: 1;
}
.text {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 1.7;
}
.path {
  font-size: 11.5px;
  color: var(--text-2);
}
.warn-line {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--bad);
}
.actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
}
.small {
  font-size: 11.5px;
}
</style>
