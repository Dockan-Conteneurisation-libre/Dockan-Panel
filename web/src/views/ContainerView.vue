<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import StatusPill from '../components/StatusPill.vue'
import TerminalPane from '../components/TerminalPane.vue'
import { t, type Key } from '../i18n'
import type { Container } from '../lib/api'
import type { IconName } from '../lib/icons'
import { useLoad } from '../lib/useLoad'
import { run } from '../lib/state'
import { confirmAction } from '../lib/confirm'

const props = defineProps<{ store: string; name: string }>()
interface Detail { container: Container; inspect: [string, string][]; logs: string; composeFile: string }

const storeValue = computed(() => (props.store === '-' ? '' : props.store))
const { data, loading, error, reload } = useLoad<Detail>(
  () => `/api/containers/detail?name=${encodeURIComponent(props.name)}&store=${encodeURIComponent(storeValue.value)}`,
)
const router = useRouter()
type Tab = 'info' | 'terminal' | 'logs' | 'inspect'
const tab = ref<Tab>('info')
const tabs: { id: Tab; label: Key; icon: IconName }[] = [
  { id: 'info', label: 'detail.info', icon: 'box' },
  { id: 'terminal', label: 'detail.terminal', icon: 'terminal' },
  { id: 'logs', label: 'detail.logs', icon: 'logs' },
  { id: 'inspect', label: 'detail.inspect', icon: 'search' },
]
const command = ref('')
const follow = ref(false)
let timer = 0
watch(follow, (on) => {
  window.clearInterval(timer)
  if (on) timer = window.setInterval(() => void reload(), 3000)
})
onBeforeUnmount(() => window.clearInterval(timer))

const c = computed(() => data.value?.container)
const facts = computed(() => {
  const x = c.value
  if (!x) return []
  const list: [Key, string][] = [
    ['containers.image', x.image], ['containers.ports', x.ports || '—'],
    ['containers.store', x.store || '—'], ['containers.pid', x.pid || '—'],
  ]
  return list
})

async function act(action: 'stop' | 'remove' | 'health' | 'start-app' | 'restart-app'): Promise<void> {
  if (action === 'remove' && !(await confirmAction(t('containers.confirmRemove', { name: props.name })))) return
  if (action === 'stop' && !(await confirmAction(t('containers.confirmStop', { name: props.name }), false))) return
  const ok: Record<typeof action, string> = {
    stop: t('containers.stopped'), remove: t('containers.removed'), health: t('containers.healthDone'),
    'start-app': t('containers.started'), 'restart-app': t('containers.redeployed'),
  }
  const out = await run(`${action} ${props.name}`, '/api/containers/action', { name: props.name, store: storeValue.value, action }, ok[action])
  if (out === null) return
  if (action === 'remove') await router.push('/containers')
  else await reload()
}

async function exec(): Promise<void> {
  if (!command.value.trim()) return
  await run(`exec ${props.name}: ${command.value}`, '/api/containers/exec', { name: props.name, store: storeValue.value, command: command.value }, t('detail.execDone'))
}
</script>

<template>
  <RouterLink to="/containers" class="btn btn-ghost btn-sm mb-3"><Icon name="back" /> {{ t('nav.containers') }}</RouterLink>
  <header class="mb-5 flex flex-wrap items-center justify-between gap-3">
    <div class="flex min-w-0 items-center gap-3">
      <span class="grid h-11 w-11 flex-none place-items-center rounded-lg bg-pine text-mint"><Icon name="box" :size="22" /></span>
      <div class="min-w-0">
        <h1 class="m-0 truncate text-[1.4rem] font-[680]">{{ name }}</h1>
        <div class="mt-1 flex flex-wrap items-center gap-2">
          <StatusPill v-if="c" :status="c.status" />
          <span class="mono text-muted">{{ c?.image }}</span>
        </div>
      </div>
    </div>
    <div class="flex flex-wrap gap-2">
      <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
      <template v-if="data?.composeFile">
        <button class="btn" @click="act('start-app')"><Icon name="play" /> {{ t('containers.startApp') }}</button>
        <button class="btn" @click="act('restart-app')"><Icon name="redeploy" /> {{ t('containers.restartApp') }}</button>
      </template>
      <button class="btn" @click="act('health')"><Icon name="heart" /> {{ t('containers.health') }}</button>
      <button class="btn" :disabled="c?.status !== 'running'" @click="act('stop')"><Icon name="stop" /> {{ t('containers.stop') }}</button>
      <button class="btn btn-danger" @click="act('remove')"><Icon name="trash" /> {{ t('containers.remove') }}</button>
    </div>
  </header>

  <p v-if="error" class="error-box">{{ error.includes('not found') ? t('detail.notFound') : error }}</p>

  <div class="tabs" role="tablist">
    <button v-for="x in tabs" :key="x.id" role="tab" :aria-selected="tab === x.id" :class="{ on: tab === x.id }" @click="tab = x.id">
      <Icon :name="x.icon" :size="16" /> {{ t(x.label) }}
    </button>
  </div>

  <div v-if="tab === 'info'" class="grid gap-4 lg:grid-cols-2">
    <section class="card">
      <div class="card-head"><h2>{{ t('detail.info') }}</h2></div>
      <dl class="m-0 grid grid-cols-[max-content_1fr] gap-x-5 gap-y-2 p-4 text-[0.88rem]">
        <template v-for="[k, v] in facts" :key="k">
          <dt class="text-muted">{{ t(k) }}</dt><dd class="mono m-0 break-all">{{ v }}</dd>
        </template>
        <template v-if="data?.composeFile">
          <dt class="text-muted">{{ t('detail.compose') }}</dt><dd class="path m-0">{{ data.composeFile }}</dd>
        </template>
      </dl>
    </section>
    <section class="card">
      <div class="card-head"><h2>{{ t('detail.exec') }}</h2></div>
      <form class="grid gap-3 p-4" @submit.prevent="exec">
        <input v-model="command" class="input mono" placeholder="ls -la /" spellcheck="false" />
        <span class="hint text-[0.8rem] text-muted">{{ t('detail.execHint') }}</span>
        <div><button class="btn btn-primary" :disabled="!command.trim()"><Icon name="play" /> {{ t('common.run') }}</button></div>
      </form>
    </section>
  </div>

  <section v-else-if="tab === 'terminal'" class="card p-4">
    <TerminalPane :name="name" :store="storeValue" />
  </section>

  <section v-else-if="tab === 'logs'" class="card">
    <div class="card-head">
      <h2>{{ t('detail.logs') }}</h2>
      <label class="check"><input v-model="follow" type="checkbox" /> {{ t('detail.follow') }}</label>
    </div>
    <div class="card-body"><pre class="pre logs">{{ data?.logs || t('detail.logsEmpty') }}</pre></div>
  </section>

  <section v-else class="card overflow-hidden">
    <div class="overflow-x-auto">
      <table class="table">
        <tbody>
          <tr v-for="[k, v] in data?.inspect ?? []" :key="k"><td class="mono text-muted">{{ k }}</td><td class="mono break-all">{{ v }}</td></tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.logs { max-height: 65vh; }
</style>
