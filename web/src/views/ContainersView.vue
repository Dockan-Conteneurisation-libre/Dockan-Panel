<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusPill from '../components/StatusPill.vue'
import Empty from '../components/Empty.vue'
import Drawer from '../components/Drawer.vue'
import { t } from '../i18n'
import type { Container, Image, StoreAppRow } from '../lib/api'
import { useLoad } from '../lib/useLoad'
import { run } from '../lib/state'
import { confirmAction } from '../lib/confirm'

interface List { containers: Container[]; apps: StoreAppRow[]; images: Image[] }
const { data, loading, error, reload } = useLoad<List>(() => '/api/containers')
const route = useRoute()
const router = useRouter()
const filter = ref('')
const showRun = ref(false)
const advanced = ref(false)

onMounted(() => {
  if (route.query.new === '1') {
    showRun.value = true
    void router.replace({ query: {} })
  }
})

const rows = computed(() => {
  const q = filter.value.trim().toLowerCase()
  const list = data.value?.containers ?? []
  return q ? list.filter((c) => c.name.toLowerCase().includes(q) || c.image.toLowerCase().includes(q)) : list
})

const blank = () => ({
  name: '', image: '', ports: '', env: '', volumes: '', aliases: '', network: '', entrypoint: '',
  restart: '', healthcheck: '', memory: '', cpus: '', isolation: 'auto', command: '', gui: false,
})
const form = reactive(blank())

function link(c: Container): string {
  return `/containers/${c.store || '-'}/${encodeURIComponent(c.name)}`
}

async function act(c: Container, action: 'stop' | 'remove' | 'health'): Promise<void> {
  if (action === 'remove' && !(await confirmAction(t('containers.confirmRemove', { name: c.name })))) return
  if (action === 'stop' && !(await confirmAction(t('containers.confirmStop', { name: c.name }), false))) return
  const ok = { stop: t('containers.stopped'), remove: t('containers.removed'), health: t('containers.healthDone') }[action]
  if ((await run(`${action} ${c.name}`, '/api/containers/action', { name: c.name, store: c.store, action }, ok)) !== null) await reload()
}

async function appAct(name: string, action: 'start-app' | 'restart-app'): Promise<void> {
  const ok = action === 'start-app' ? t('containers.started') : t('containers.redeployed')
  if ((await run(`${action} ${name}`, '/api/containers/action', { name, action }, ok)) !== null) await reload()
}

async function submitRun(): Promise<void> {
  const out = await run(`run ${form.name}`, '/api/containers/run', { ...form }, t('run.started'))
  if (out !== null) {
    Object.assign(form, blank())
    showRun.value = false
    await reload()
  }
}
</script>

<template>
  <PageHeader :title="t('containers.title')" :subtitle="t('containers.subtitle')">
    <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
    <button class="btn btn-primary" @click="showRun = true"><Icon name="plus" /> {{ t('containers.new') }}</button>
  </PageHeader>
  <p v-if="error" class="error-box">{{ error }}</p>

  <section class="card overflow-hidden">
    <div class="card-head">
      <label class="relative block w-full max-w-[340px]">
        <span class="sr-only">{{ t('common.search') }}</span>
        <Icon name="search" :size="16" class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted" />
        <input v-model="filter" class="input pl-9" :placeholder="t('containers.filter')" />
      </label>
      <span class="pill">{{ rows.length }}</span>
    </div>
    <Empty v-if="data && !rows.length" icon="box" :text="t('containers.empty')" />
    <div v-else class="overflow-x-auto">
      <table class="table">
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th><th>{{ t('common.status') }}</th><th>{{ t('containers.store') }}</th>
            <th>{{ t('containers.image') }}</th><th>{{ t('containers.ports') }}</th><th class="text-right">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in rows" :key="c.store + c.name">
            <td><RouterLink class="link" :to="link(c)">{{ c.name }}</RouterLink><div class="mono text-muted">{{ c.pid ? 'PID ' + c.pid : '' }}</div></td>
            <td><StatusPill :status="c.status" /></td>
            <td><span class="pill">{{ c.store || '—' }}</span></td>
            <td class="mono max-w-[280px] truncate" :title="c.image">{{ c.image }}</td>
            <td class="mono">{{ c.ports || '—' }}</td>
            <td>
              <div class="flex justify-end gap-1">
                <RouterLink class="btn btn-ghost btn-icon btn-sm" :to="link(c)" :title="t('detail.terminal')" :aria-label="t('detail.terminal')"><Icon name="terminal" /></RouterLink>
                <button class="btn btn-ghost btn-icon btn-sm" :title="t('containers.health')" :aria-label="t('containers.health')" @click="act(c, 'health')"><Icon name="heart" /></button>
                <button class="btn btn-ghost btn-icon btn-sm" :disabled="c.status !== 'running'" :title="t('containers.stop')" :aria-label="t('containers.stop')" @click="act(c, 'stop')"><Icon name="stop" /></button>
                <button class="btn btn-ghost btn-icon btn-sm btn-danger" :title="t('containers.remove')" :aria-label="t('containers.remove')" @click="act(c, 'remove')"><Icon name="trash" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <section v-if="data?.apps.length" class="card mt-6 overflow-hidden">
    <div class="card-head">
      <div><h2>{{ t('containers.apps') }}</h2><p class="m-0 text-[0.8rem] text-muted">{{ t('containers.appsHint') }}</p></div>
      <RouterLink to="/store" class="btn btn-sm"><Icon name="store" /> {{ t('nav.store') }}</RouterLink>
    </div>
    <div class="grid gap-3 p-4 sm:grid-cols-2 xl:grid-cols-3">
      <article v-for="a in data.apps" :key="a.id" class="rounded-lg border border-line bg-surface-2 p-4">
        <div class="flex items-start justify-between gap-2">
          <strong>{{ a.name }}</strong>
          <StatusPill :status="a.status" />
        </div>
        <p class="path mt-1 mb-3">{{ a.target }}</p>
        <p class="m-0 mb-3 text-[0.82rem] text-muted">{{ a.containers.length ? a.containers.join(', ') : t('containers.noContainers') }}</p>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-sm" @click="appAct(a.containers[0] ?? a.id, 'start-app')"><Icon name="play" /> {{ t('containers.startApp') }}</button>
          <button class="btn btn-sm" @click="appAct(a.containers[0] ?? a.id, 'restart-app')"><Icon name="redeploy" /> {{ t('containers.restartApp') }}</button>
        </div>
      </article>
    </div>
  </section>

  <Drawer v-if="showRun" :title="t('run.title')" :subtitle="t('run.subtitle')" wide @close="showRun = false">
    <form id="run-form" class="grid gap-4" @submit.prevent="submitRun">
      <div class="form-grid">
        <label class="field">{{ t('run.name') }}<input v-model="form.name" class="input" required spellcheck="false" /></label>
        <label class="field">{{ t('run.image') }}
          <input v-model="form.image" class="input mono" list="image-list" required spellcheck="false" />
          <span class="hint">{{ t('run.imageHint') }}</span>
        </label>
      </div>
      <datalist id="image-list"><option v-for="i in data?.images ?? []" :key="i.tag" :value="i.tag" /></datalist>
      <div class="form-grid">
        <label class="field">{{ t('run.ports') }}<textarea v-model="form.ports" class="input" rows="3" placeholder="8080:80" /><span class="hint">{{ t('run.portsHint') }}</span></label>
        <label class="field">{{ t('run.volumes') }}<textarea v-model="form.volumes" class="input" rows="3" placeholder="data:/var/lib/app" /><span class="hint">{{ t('run.volumesHint') }}</span></label>
      </div>
      <label class="field">{{ t('run.env') }}<textarea v-model="form.env" class="input" rows="3" placeholder="TZ=Europe/Paris" /><span class="hint">{{ t('run.envHint') }}</span></label>
      <label class="field">{{ t('run.command') }} <span class="hint">({{ t('common.optional') }})</span><input v-model="form.command" class="input mono" spellcheck="false" /></label>
      <label class="field">{{ t('run.isolation') }}
        <select v-model="form.isolation" class="input">
          <option value="auto">{{ t('run.isolationAuto') }}</option>
          <option v-for="m in ['bubblewrap', 'firejail', 'systemd-nspawn', 'chroot', 'none']" :key="m" :value="m">{{ m }}</option>
        </select>
      </label>
      <button type="button" class="btn btn-ghost justify-start" @click="advanced = !advanced">
        <Icon name="chevron" :class="{ 'rotate-90': advanced }" class="transition-transform" /> {{ t('run.advanced') }}
      </button>
      <div v-if="advanced" class="grid gap-4">
        <div class="form-grid">
          <label class="field">{{ t('run.network') }}<input v-model="form.network" class="input" /></label>
          <label class="field">{{ t('run.aliases') }}<textarea v-model="form.aliases" class="input" rows="2" /></label>
          <label class="field">{{ t('run.entrypoint') }}<input v-model="form.entrypoint" class="input mono" /></label>
          <label class="field">{{ t('run.restart') }}
            <select v-model="form.restart" class="input">
              <option value="">—</option>
              <option v-for="p in ['no', 'on-failure', 'always', 'unless-stopped']" :key="p" :value="p">{{ p }}</option>
            </select>
          </label>
          <label class="field">{{ t('run.memory') }}<input v-model="form.memory" class="input" placeholder="512m" /></label>
          <label class="field">{{ t('run.cpus') }}<input v-model="form.cpus" class="input" placeholder="1.5" /></label>
        </div>
        <label class="field">{{ t('run.healthcheck') }}<input v-model="form.healthcheck" class="input mono" placeholder="curl -fsS http://127.0.0.1/" /></label>
        <label class="check"><input v-model="form.gui" type="checkbox" /> {{ t('run.gui') }}</label>
      </div>
    </form>
    <template #footer>
      <button class="btn" @click="showRun = false">{{ t('common.cancel') }}</button>
      <button class="btn btn-primary" form="run-form"><Icon name="play" /> {{ t('run.submit') }}</button>
    </template>
  </Drawer>
</template>
