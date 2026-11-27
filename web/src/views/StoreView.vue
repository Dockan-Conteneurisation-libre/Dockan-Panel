<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import Empty from '../components/Empty.vue'
import Drawer from '../components/Drawer.vue'
import { t } from '../i18n'
import { api, type StoreApp, type StoreState, type Output } from '../lib/api'
import { useLoad } from '../lib/useLoad'
import { run, state, toast } from '../lib/state'

const { data, loading, error, reload } = useLoad<StoreState>(() => '/api/store')
const query = ref('')
const category = ref('')
const scope = ref<'all' | 'installed' | 'available'>('all')
const current = ref<StoreApp | null>(null)
const target = ref('')
const config = ref('')
const zip = ref<HTMLInputElement | null>(null)
const files = ref<HTMLInputElement | null>(null)
const replace = ref(false)

const categories = computed(() => [...new Set((data.value?.apps ?? []).map((a) => a.category))].sort())
const apps = computed(() => {
  const q = query.value.trim().toLowerCase()
  return (data.value?.apps ?? []).filter((a) =>
    (!q || `${a.name} ${a.summary} ${a.id}`.toLowerCase().includes(q)) &&
    (!category.value || a.category === category.value) &&
    (scope.value === 'all' || (scope.value === 'installed') === a.installed))
})
const counts = computed(() => {
  const list = data.value?.apps ?? []
  return { all: list.length, installed: list.filter((a) => a.installed).length, available: list.filter((a) => !a.installed).length }
})

const hues = [158, 172, 140, 190, 120, 205]
const brokenIcons = ref<Record<string, boolean>>({})
function badge(a: StoreApp): { text: string; bg: string } {
  let h = 0
  for (const ch of a.id) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  const words = a.name.split(/[\s-]+/).filter(Boolean)
  const text = (words.length > 1 ? (words[0]?.[0] ?? '') + (words[1]?.[0] ?? '') : a.name.slice(0, 2)).toUpperCase()
  return { text, bg: `hsl(${hues[h % hues.length]} 45% 32%)` }
}
function onIconError(id: string): void {
  brokenIcons.value = { ...brokenIcons.value, [id]: true }
}

function open(a: StoreApp): void {
  current.value = a
  target.value = ''
  config.value = a.config
  replace.value = false
}

async function act(action: string, ok: string): Promise<void> {
  const a = current.value
  if (!a) return
  const body = { app: a.id, target: target.value, config: config.value }
  const out = await run(`store ${action} ${a.id}`, `/api/store/${action}`, body, ok)
  if (out === null) return
  await reload()
  const fresh = data.value?.apps.find((x) => x.id === a.id)
  if (fresh) { current.value = fresh; config.value = fresh.config }
}

async function updateStore(): Promise<void> {
  if ((await run('store update', '/api/store/update', {}, t('store.updated'))) !== null) await reload()
}

async function upload(): Promise<void> {
  const a = current.value
  if (!a) return
  const fd = new FormData()
  fd.append('app', a.id)
  fd.append('target', target.value)
  if (replace.value) fd.append('replace', '1')
  const z = zip.value?.files?.[0]
  if (z) fd.append('zip', z)
  for (const f of Array.from(files.value?.files ?? [])) fd.append('files', f, f.webkitRelativePath || f.name)
  state.busy++
  try {
    const res = await api.post<Output>('/api/store-upload', fd)
    state.activity.unshift({ id: Date.now(), title: `upload ${a.id}`, ok: true, output: res.output, at: new Date() })
    toast('ok', t('store.siteUploaded'))
    if (zip.value) zip.value.value = ''
    if (files.value) files.value.value = ''
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  } finally {
    state.busy--
  }
}
</script>

<template>
  <PageHeader :title="t('store.title')" :subtitle="t('store.subtitle')">
    <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
    <button class="btn btn-primary" @click="updateStore"><Icon name="download" /> {{ data?.ready === false ? t('store.download') : t('store.update') }}</button>
  </PageHeader>
  <p v-if="error" class="error-box">{{ error }}</p>

  <div v-if="data && !data.ready" class="card mb-4 flex flex-wrap items-center justify-between gap-3 p-4">
    <div><strong>{{ t('store.notReady') }}</strong><p class="m-0 text-[0.86rem] text-muted">{{ t('store.notReadyHint') }}</p></div>
    <button class="btn btn-primary" @click="updateStore"><Icon name="download" /> {{ t('store.download') }}</button>
  </div>

  <div class="mb-4 flex flex-wrap items-center gap-3">
    <label class="relative block min-w-[220px] flex-1 sm:max-w-[340px]">
      <span class="sr-only">{{ t('common.search') }}</span>
      <Icon name="search" :size="16" class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted" />
      <input v-model="query" class="input pl-9" :placeholder="t('store.filter')" />
    </label>
    <div class="seg" role="group">
      <button :class="{ on: scope === 'all' }" @click="scope = 'all'">{{ t('common.all') }} <span>{{ counts.all }}</span></button>
      <button :class="{ on: scope === 'installed' }" @click="scope = 'installed'">{{ t('store.installed') }} <span>{{ counts.installed }}</span></button>
      <button :class="{ on: scope === 'available' }" @click="scope = 'available'">{{ t('store.available') }} <span>{{ counts.available }}</span></button>
    </div>
    <select v-model="category" class="input w-auto" :aria-label="t('common.search')">
      <option value="">{{ t('common.all') }}</option>
      <option v-for="c in categories" :key="c" :value="c">{{ c }}</option>
    </select>
  </div>

  <Empty v-if="data && !apps.length" icon="store" :text="t('store.empty')" />
  <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
    <button v-for="a in apps" :key="a.id" class="card app" @click="open(a)">
      <span class="logo" :style="{ background: badge(a).bg }">
        <img v-if="!brokenIcons[a.id]" :src="`/store-icons/${a.id}.svg`" :alt="a.name" class="logo-img" @error="onIconError(a.id)" />
        <template v-else>{{ badge(a).text }}</template>
      </span>
      <span class="min-w-0 flex-1 text-left">
        <span class="flex items-center gap-2"><strong class="truncate">{{ a.name }}</strong></span>
        <span class="mt-0.5 block text-[0.74rem] tracking-wide text-muted uppercase">{{ a.category }}</span>
        <span class="mt-1.5 line-clamp-2 block text-[0.84rem] text-muted">{{ a.summary }}</span>
        <span class="mt-2 flex flex-wrap gap-1.5">
          <span v-if="a.installed" class="pill on">{{ t('store.installed') }}</span>
          <span v-if="a.images === 'available'" class="pill warn">{{ t('store.imagesNew') }}</span>
          <span v-if="a.autostart" class="pill">{{ t('store.autostart') }}</span>
          <span v-if="a.default_port" class="pill mono">{{ t('store.port', { port: a.default_port }) }}</span>
        </span>
      </span>
    </button>
  </div>
  <p v-if="data" class="mt-4 text-[0.78rem] text-muted">{{ t('store.catalogSource', { source: data.catalog }) }}</p>

  <Drawer v-if="current" :title="current.name" :subtitle="current.summary" wide @close="current = null">
    <div class="grid gap-5">
      <div class="flex flex-wrap gap-1.5">
        <span class="pill">{{ current.category }}</span>
        <span v-if="current.installed" class="pill on">{{ t('store.installed') }}</span>
        <span v-if="current.images === 'available'" class="pill warn">{{ t('store.imagesNew') }}</span>
        <span v-else-if="current.images === 'current'" class="pill">{{ t('store.imagesCurrent') }}</span>
        <span v-if="current.autostart" class="pill">{{ t('store.autostart') }}</span>
      </div>

      <label class="field">{{ t('store.target') }}
        <input v-model="target" class="input mono" :placeholder="current.target" spellcheck="false" />
        <span class="hint">{{ t('store.targetHint', { base: data?.base ?? '' }) }}</span>
      </label>

      <div class="grid gap-2">
        <div class="flex flex-wrap gap-2">
          <template v-if="!current.installed">
            <button class="btn btn-primary" @click="act('deploy', t('store.done'))"><Icon name="play" /> {{ t('store.deploy') }}</button>
            <button class="btn" @click="act('install', t('store.done'))"><Icon name="download" /> {{ t('store.install') }}</button>
            <button class="btn" @click="act('install-autostart', t('store.done'))"><Icon name="power" /> {{ t('store.installBoot') }}</button>
          </template>
          <template v-else>
            <button class="btn btn-primary" @click="act('launch', t('store.done'))"><Icon name="play" /> {{ t('store.launch') }}</button>
            <button class="btn" @click="act('redeploy', t('store.done'))"><Icon name="redeploy" /> {{ t('store.redeploy') }}</button>
            <button class="btn" @click="act('update-images', t('store.done'))"><Icon name="download" /> {{ t('store.updateImages') }}</button>
            <button v-if="!current.autostart" class="btn" @click="act('autostart', t('store.done'))"><Icon name="power" /> {{ t('store.enableBoot') }}</button>
            <button v-else class="btn" @click="act('disable-autostart', t('store.done'))"><Icon name="power" /> {{ t('store.disableBoot') }}</button>
          </template>
        </div>
        <p v-if="!data?.root" class="m-0 text-[0.8rem] text-muted">{{ t('store.needsRoot') }} ({{ t('store.launch') }}, {{ t('store.enableBoot') }})</p>
      </div>

      <div v-if="current.requires.length">
        <p class="mt-0 mb-1.5 text-[0.85rem] font-semibold">{{ t('store.requires') }}</p>
        <ul class="m-0 grid gap-1 p-0">
          <li v-for="img in current.requires" :key="img" class="mono list-none rounded-md bg-code px-2.5 py-1.5 break-all">{{ img }}</li>
        </ul>
      </div>

      <div v-if="current.installed" class="grid gap-2">
        <label class="field">{{ t('store.config') }}
          <textarea v-model="config" class="input code-input" spellcheck="false" />
          <span class="hint">{{ t('store.configHint') }}</span>
        </label>
        <div class="flex flex-wrap justify-end gap-2">
          <button class="btn" @click="act('save-config', t('store.done'))"><Icon name="check" /> {{ t('store.saveConfig') }}</button>
          <button class="btn btn-primary" @click="act('save-redeploy', t('store.done'))"><Icon name="redeploy" /> {{ t('store.saveRedeploy') }}</button>
        </div>
      </div>

      <form v-if="current.id === 'static-site'" class="grid gap-3 rounded-lg border border-line p-4" @submit.prevent="upload">
        <div><strong>{{ t('store.site') }}</strong><p class="m-0 text-[0.84rem] text-muted">{{ t('store.siteHint') }}</p></div>
        <label class="field">{{ t('store.siteZip') }}<input ref="zip" type="file" accept=".zip" class="input" /></label>
        <label class="field">{{ t('store.siteFiles') }}<input ref="files" type="file" multiple class="input" /></label>
        <label class="check"><input v-model="replace" type="checkbox" /> {{ t('store.siteReplace') }}</label>
        <div><button class="btn btn-primary"><Icon name="upload" /> {{ t('store.siteUpload') }}</button></div>
      </form>
    </div>
  </Drawer>
</template>

<style scoped>
.app { display: flex; gap: 0.9rem; padding: 1rem; cursor: pointer; color: inherit; font: inherit; text-align: left; transition: border-color .12s, transform .12s; }
.app:hover { border-color: var(--color-teal); transform: translateY(-1px); }
.logo { display: grid; place-items: center; width: 44px; height: 44px; flex: none; border-radius: 10px; color: #fff; font-weight: 700; font-size: 0.95rem; letter-spacing: 0.02em; }
.logo-img { width: 24px; height: 24px; display: block; object-fit: contain; }
.pill.on { color: var(--color-leaf); border-color: color-mix(in srgb, var(--color-leaf) 35%, transparent); }
.pill.warn { color: var(--color-signal); border-color: color-mix(in srgb, var(--color-signal) 40%, transparent); }
.seg { display: inline-flex; padding: 3px; border-radius: 9px; background: var(--color-surface); border: 1px solid var(--color-line); }
.seg button { display: inline-flex; align-items: center; gap: 0.35rem; min-height: 30px; padding: 0 0.7rem; border: 0; border-radius: 6px; background: none; color: var(--color-muted); font: 600 0.82rem/1 var(--font-sans); cursor: pointer; }
.seg button.on { background: var(--color-pine); color: #fff; }
.seg span { font-size: 0.72rem; opacity: 0.75; }
</style>
