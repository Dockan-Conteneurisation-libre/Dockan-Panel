<script setup lang="ts">
import { computed, reactive } from 'vue'
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import { t, type Key } from '../i18n'
import type { SystemInfo } from '../lib/api'
import { useLoad } from '../lib/useLoad'
import { run } from '../lib/state'

const { data, loading, error, reload } = useLoad<{ info: SystemInfo; profiles: string[]; runtimes: string[] }>(() => '/api/system')
type Mode = 'preview' | 'install' | 'command'
const deps = reactive({ kind: 'profile', profile: 'core', runtime: 'node:20', packages: '', mode: 'preview' as Mode })
const dockan = reactive({ version: 'latest', system: true, mode: 'preview' as Mode })
const panel = reactive({ ref: 'latest', mode: 'command' as Mode })

const info = computed(() => data.value?.info)
const automation = computed<Key>(() => ({ root: 'system.automationRoot', sudo: 'system.automationSudo', disabled: 'system.automationDisabled' } as const)[info.value?.automation ?? 'disabled'])
const facts = computed(() => {
  const i = info.value
  if (!i) return []
  const list: [Key, string][] = [
    ['system.panel', i.panelVersion], ['system.dockan', i.dockanVersion.split('\n')[0] ?? ''], ['system.binary', i.dockanPath],
    ['system.user', i.user], ['system.arch', i.arch], ['system.storage', i.storage], ['system.updateDir', i.updateDir],
  ]
  return list
})
const modes: { id: Mode; label: Key }[] = [
  { id: 'preview', label: 'system.modePreview' }, { id: 'install', label: 'system.modeInstall' }, { id: 'command', label: 'system.modeCommand' },
]

async function runDeps(): Promise<void> {
  const value = deps.kind === 'profile' ? deps.profile : deps.kind === 'runtime' ? deps.runtime : deps.packages
  await run(`deps ${deps.kind} ${value}`, '/api/system/deps', { kind: deps.kind, value, mode: deps.mode }, t('system.done'))
}
async function updateDockan(): Promise<void> {
  if ((await run(`dockan update ${dockan.version}`, '/api/system/update-dockan', { ...dockan }, t('system.done'))) !== null && dockan.mode === 'install') await reload()
}
async function updatePanel(): Promise<void> {
  await run(`panel update ${panel.ref}`, '/api/system/update-panel', { ...panel }, t('system.done'))
}
</script>

<template>
  <PageHeader :title="t('system.title')" :subtitle="t('system.subtitle')">
    <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
  </PageHeader>
  <p v-if="error" class="error-box">{{ error }}</p>

  <div class="grid gap-4 xl:grid-cols-2">
    <section class="card">
      <div class="card-head"><h2>{{ t('system.versions') }}</h2></div>
      <dl class="m-0 grid grid-cols-[max-content_1fr] gap-x-5 gap-y-2 p-4 text-[0.88rem]">
        <template v-for="[k, v] in facts" :key="k"><dt class="text-muted">{{ t(k) }}</dt><dd class="mono m-0 break-all">{{ v || '—' }}</dd></template>
        <dt class="text-muted">{{ t('system.automation') }}</dt>
        <dd class="m-0"><span class="pill" :class="info?.automation === 'disabled' ? 'warn' : 'on'">{{ t(automation) }}</span></dd>
      </dl>
    </section>

    <section class="card">
      <div class="card-head"><div><h2>{{ t('system.deps') }}</h2><p class="m-0 text-[0.8rem] text-muted">{{ t('system.depsHint') }}</p></div></div>
      <form class="grid gap-4 p-4" @submit.prevent="runDeps">
        <div class="seg" role="group">
          <button type="button" :class="{ on: deps.kind === 'profile' }" @click="deps.kind = 'profile'">{{ t('system.profile') }}</button>
          <button type="button" :class="{ on: deps.kind === 'runtime' }" @click="deps.kind = 'runtime'">{{ t('system.runtime') }}</button>
          <button type="button" :class="{ on: deps.kind === 'packages' }" @click="deps.kind = 'packages'">{{ t('system.packages') }}</button>
        </div>
        <label v-if="deps.kind === 'profile'" class="field">{{ t('system.profile') }}
          <select v-model="deps.profile" class="input"><option v-for="p in data?.profiles ?? []" :key="p" :value="p">{{ p }}</option></select>
        </label>
        <label v-else-if="deps.kind === 'runtime'" class="field">{{ t('system.runtime') }}
          <select v-model="deps.runtime" class="input"><option v-for="r in data?.runtimes ?? []" :key="r" :value="r">{{ r }}</option></select>
        </label>
        <label v-else class="field">{{ t('system.packages') }}<input v-model="deps.packages" class="input mono" required /><span class="hint">{{ t('system.packagesHint') }}</span></label>
        <label class="field">{{ t('system.mode') }}
          <select v-model="deps.mode" class="input"><option v-for="m in modes" :key="m.id" :value="m.id">{{ t(m.label) }}</option></select>
        </label>
        <div><button class="btn btn-primary"><Icon name="play" /> {{ t('common.run') }}</button></div>
      </form>
    </section>

    <section class="card">
      <div class="card-head"><div><h2>{{ t('system.updateDockan') }}</h2><p class="m-0 text-[0.8rem] text-muted">{{ t('system.updateDockanHint') }}</p></div></div>
      <form class="grid gap-4 p-4" @submit.prevent="updateDockan">
        <div class="form-grid">
          <label class="field">{{ t('system.version') }}<input v-model="dockan.version" class="input mono" required /><span class="hint">{{ t('system.versionHint') }}</span></label>
          <label class="field">{{ t('system.mode') }}
            <select v-model="dockan.mode" class="input"><option v-for="m in modes" :key="m.id" :value="m.id">{{ t(m.label) }}</option></select>
          </label>
        </div>
        <label class="check"><input v-model="dockan.system" type="checkbox" /> {{ t('system.systemWide') }}</label>
        <div><button class="btn btn-primary"><Icon name="download" /> {{ t('system.updateDockan') }}</button></div>
      </form>
    </section>

    <section class="card">
      <div class="card-head"><div><h2>{{ t('system.updatePanel') }}</h2><p class="m-0 text-[0.8rem] text-muted">{{ t('system.updatePanelHint') }}</p></div></div>
      <form class="grid gap-4 p-4" @submit.prevent="updatePanel">
        <div class="form-grid">
          <label class="field">{{ t('system.version') }}<input v-model="panel.ref" class="input mono" required /><span class="hint">{{ t('system.versionHint') }}</span></label>
          <label class="field">{{ t('system.mode') }}
            <select v-model="panel.mode" class="input"><option v-for="m in modes" :key="m.id" :value="m.id">{{ t(m.label) }}</option></select>
          </label>
        </div>
        <div><button class="btn btn-primary"><Icon name="download" /> {{ t('system.updatePanel') }}</button></div>
      </form>
    </section>
  </div>

  <section class="card mt-4">
    <div class="card-head"><h2>{{ t('system.doctor') }}</h2></div>
    <div class="card-body"><pre class="pre">{{ info?.doctor || '—' }}</pre></div>
  </section>
</template>

<style scoped>
.seg { display: inline-flex; width: fit-content; padding: 3px; border-radius: 9px; background: var(--color-surface-2); border: 1px solid var(--color-line); }
.seg button { min-height: 30px; padding: 0 0.75rem; border: 0; border-radius: 6px; background: none; color: var(--color-muted); font: 600 0.82rem/1 var(--font-sans); cursor: pointer; }
.seg button.on { background: var(--color-pine); color: #fff; }
.pill.on { color: var(--color-leaf); }
.pill.warn { color: var(--color-signal); }
</style>
