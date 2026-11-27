<script setup lang="ts">
// 服务模式: the NextBox service runs the core for TUN mode, so that NextBox
// does not have to run as administrator, and starts NextBox at logon. It is
// optional: installing and removing it take one administrator prompt each,
// and without it TUN mode works by running NextBox as administrator.
import { computed, onMounted, ref } from 'vue'
import { t } from '../../i18n'
import Icon from '../../components/Icon.vue'
import HelpTip from '../../components/HelpTip.vue'
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
  <div class="form-row svc">
    <div class="label">
      <span>{{ t('服务模式') }}<HelpTip :title="t('服务模式')">{{ t('安装一个 Windows 服务，由它以系统权限运行 TUN 模式的内核。只在安装和卸载时需要一次管理员授权；之后开启 TUN 模式不必以管理员身份运行 NextBox，开机启动也改由服务在登录 Windows 时完成。') }}

{{ t('安装位置：') }}{{ info?.dir || 'C:\Program Files\NextBox Service' }}

{{ t('不安装服务时，以管理员身份运行 NextBox 同样可以使用 TUN 模式。') }}</HelpTip></span>
      <span class="hint">{{ state.app?.elevated ? t('当前以管理员身份运行，不经过服务') : t('无需管理员即可使用 TUN') }}</span>
    </div>
    <div class="control">
      <span class="chip" :class="status.cls">{{ status.text }}</span>
      <button class="btn ghost small icon" :title="t('刷新')" @click="refresh"><Icon name="refresh" :size="13" /></button>
      <button v-if="info?.installed" class="btn small danger" :disabled="!!busy" @click="uninstall">
        {{ busy === 'uninstall' ? t('请在弹出的窗口中授权…') : t('卸载') }}
      </button>
      <button v-if="!info?.installed || info?.problem" class="btn small primary" :disabled="!!busy || !info?.canInstall" @click="install">
        <Icon name="admin" :size="13" /> {{ busy === 'install' ? t('请在弹出的窗口中授权…') : installLabel }}
      </button>
    </div>
  </div>
  <p v-if="info && !info.canInstall" class="warn-line"><Icon name="warn" :size="13" /> {{ t('程序目录的 core 文件夹里缺少 NextBoxService.exe，请重新下载完整的 NextBox。') }}</p>
  <p v-if="info?.problem === 'outdated'" class="warn-line mid"><Icon name="info" :size="13" /> {{ t('服务来自另一个版本的 NextBox，或使用的内核和程序目录里的不同。点「更新服务」同步。') }}</p>
  <p v-if="info?.problem === 'error' && info.detail" class="warn-line"><Icon name="warn" :size="13" /> {{ info.detail }}</p>
</template>

<style scoped>
.chip {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--hover);
  color: var(--text-2);
  white-space: nowrap;
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
.warn-line {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--bad);
}
.warn-line.mid {
  color: var(--mid);
}
</style>
