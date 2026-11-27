<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import Empty from '../components/Empty.vue'
import { t } from '../i18n'
import type { Stack } from '../lib/api'
import { useLoad } from '../lib/useLoad'
import { run } from '../lib/state'
import { confirmAction } from '../lib/confirm'

const { data, loading, error, reload } = useLoad<{ stacks: Stack[]; template: string }>(() => '/api/stacks')
const selected = ref<string | null>(null)
const form = reactive({ name: '', yaml: '', requiredImages: '', registryDir: '' })
const isNew = computed(() => !data.value?.stacks.some((s) => s.name === selected.value))
const compose = reactive({ file: '', action: 'up' })

function open(s: Stack | null): void {
  selected.value = s?.name ?? ''
  Object.assign(form, s
    ? { name: s.name, yaml: s.yaml, requiredImages: s.requiredImages.join('\n'), registryDir: s.registryDir }
    : { name: '', yaml: data.value?.template ?? '', requiredImages: '', registryDir: '' })
}
watch(data, (d) => {
  if (selected.value === null && d?.stacks[0]) open(d.stacks[0])
})

async function save(): Promise<void> {
  if ((await run(`stack save ${form.name}`, '/api/stacks/save', { ...form }, t('stacks.saved'))) !== null) {
    selected.value = form.name
    await reload()
  }
}
async function action(a: 'up' | 'down' | 'redeploy' | 'health' | 'import'): Promise<void> {
  await run(`stack ${a} ${form.name}`, `/api/stacks/${a}`, { name: form.name }, t('stacks.actionDone'))
}
async function remove(): Promise<void> {
  if (!(await confirmAction(t('stacks.confirmDelete', { name: form.name })))) return
  if ((await run(`stack delete ${form.name}`, '/api/stacks/delete', { name: form.name }, t('stacks.deleted'))) !== null) {
    selected.value = null
    await reload()
  }
}
async function runCompose(): Promise<void> {
  await run(`compose ${compose.action} ${compose.file}`, '/api/compose', { ...compose }, t('stacks.actionDone'))
}
</script>

<template>
  <PageHeader :title="t('stacks.title')" :subtitle="t('stacks.subtitle')">
    <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
    <button class="btn btn-primary" @click="open(null)"><Icon name="plus" /> {{ t('stacks.new') }}</button>
  </PageHeader>
  <p v-if="error" class="error-box">{{ error }}</p>

  <div class="grid gap-4 lg:grid-cols-[260px_1fr]">
    <aside class="card self-start overflow-hidden">
      <Empty v-if="data && !data.stacks.length && selected === null" icon="stack" :text="t('stacks.empty')" />
      <nav v-else class="grid p-2">
        <button v-for="s in data?.stacks ?? []" :key="s.name" class="item" :class="{ on: selected === s.name }" @click="open(s)">
          <Icon name="stack" :size="16" /> {{ s.name }}
        </button>
        <span v-if="selected !== null && isNew" class="item on"><Icon name="plus" :size="16" /> {{ form.name || t('stacks.new') }}</span>
      </nav>
    </aside>

    <section v-if="selected !== null" class="card">
      <div class="card-head flex-wrap">
        <h2>{{ form.name || t('stacks.new') }}</h2>
        <div v-if="!isNew" class="flex flex-wrap gap-1">
          <button class="btn btn-sm" @click="action('up')"><Icon name="play" /> {{ t('stacks.up') }}</button>
          <button class="btn btn-sm" @click="action('down')"><Icon name="stop" /> {{ t('stacks.down') }}</button>
          <button class="btn btn-sm" @click="action('redeploy')"><Icon name="redeploy" /> {{ t('stacks.redeploy') }}</button>
          <button class="btn btn-sm" @click="action('health')"><Icon name="heart" /> {{ t('stacks.health') }}</button>
          <button class="btn btn-sm" @click="action('import')"><Icon name="download" /> {{ t('stacks.import') }}</button>
        </div>
      </div>
      <form class="grid gap-4 p-4" @submit.prevent="save">
        <label class="field">{{ t('common.name') }}<input v-model="form.name" class="input" required :readonly="!isNew" spellcheck="false" /></label>
        <label class="field">{{ t('stacks.yaml') }}<textarea v-model="form.yaml" class="input code-input" spellcheck="false" /></label>
        <div class="form-grid">
          <label class="field">{{ t('stacks.required') }}<textarea v-model="form.requiredImages" class="input" rows="3" spellcheck="false" /><span class="hint">{{ t('stacks.requiredHint') }}</span></label>
          <label class="field">{{ t('stacks.registry') }}<input v-model="form.registryDir" class="input mono" spellcheck="false" /><span class="hint">{{ t('stacks.registryHint') }}</span></label>
        </div>
        <div class="flex flex-wrap justify-between gap-2">
          <button v-if="!isNew" type="button" class="btn btn-danger" @click="remove"><Icon name="trash" /> {{ t('common.delete') }}</button>
          <span v-else />
          <button class="btn btn-primary"><Icon name="check" /> {{ t('common.save') }}</button>
        </div>
      </form>
    </section>
  </div>

  <section class="card mt-6">
    <div class="card-head"><div><h2>{{ t('stacks.compose') }}</h2><p class="m-0 text-[0.8rem] text-muted">{{ t('stacks.composeHint') }}</p></div></div>
    <form class="flex flex-wrap items-end gap-3 p-4" @submit.prevent="runCompose">
      <label class="field min-w-[260px] flex-1">{{ t('stacks.file') }}<input v-model="compose.file" class="input mono" required placeholder="/srv/dockan-apps/forgejo/dockan.yml" /></label>
      <label class="field">{{ t('common.actions') }}
        <select v-model="compose.action" class="input">
          <option value="up">{{ t('stacks.up') }}</option>
          <option value="down">{{ t('stacks.down') }}</option>
          <option value="redeploy">{{ t('stacks.redeploy') }}</option>
          <option value="health">{{ t('stacks.health') }}</option>
        </select>
      </label>
      <button class="btn btn-primary"><Icon name="play" /> {{ t('common.run') }}</button>
    </form>
  </section>
</template>

<style scoped>
.item {
  display: flex; align-items: center; gap: 0.55rem; min-height: 36px; padding: 0 0.7rem; border: 0; border-radius: 7px;
  background: none; color: var(--color-ink); font: 550 0.88rem/1 var(--font-sans); cursor: pointer; text-align: left;
}
.item:hover { background: var(--color-surface-2); }
.item.on { background: var(--color-code); color: var(--color-teal); }
</style>
