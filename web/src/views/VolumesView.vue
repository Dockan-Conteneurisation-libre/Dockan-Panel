<script setup lang="ts">
import { ref } from 'vue'
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import Empty from '../components/Empty.vue'
import { t, lang } from '../i18n'
import type { Backup, Volume } from '../lib/api'
import { useLoad } from '../lib/useLoad'
import { run } from '../lib/state'
import { confirmAction } from '../lib/confirm'

const { data, loading, error, reload } = useLoad<{ volumes: Volume[]; backups: Backup[] }>(() => '/api/volumes')
const newName = ref('')
const target = ref<Record<string, string>>({})

function size(n: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
  return `${n.toFixed(i ? 1 : 0)} ${units[i]}`
}
function date(s: string): string {
  return new Date(s).toLocaleString(lang.value)
}
function guess(b: Backup): string {
  return target.value[b.name] ?? b.name.replace(/-\d{8}-\d{6}\.tar\.gz$/, '')
}

async function create(): Promise<void> {
  if ((await run(`volume create ${newName.value}`, '/api/volumes/create', { name: newName.value }, t('volumes.created'))) !== null) {
    newName.value = ''
    await reload()
  }
}
async function remove(v: Volume): Promise<void> {
  if (!(await confirmAction(t('volumes.confirmRemove', { name: v.name })))) return
  if ((await run(`volume rm ${v.name}`, '/api/volumes/remove', { name: v.name }, t('volumes.removed'))) !== null) await reload()
}
async function backup(v: Volume): Promise<void> {
  if ((await run(`volume backup ${v.name}`, '/api/volumes/backup', { name: v.name }, t('volumes.backedUp'))) !== null) await reload()
}
async function restore(b: Backup): Promise<void> {
  const name = guess(b)
  if (!(await confirmAction(t('volumes.confirmRestore', { backup: b.name, name })))) return
  await run(`volume restore ${name}`, '/api/volumes/restore', { name, backup: b.name }, t('volumes.restored'))
}
</script>

<template>
  <PageHeader :title="t('volumes.title')" :subtitle="t('volumes.subtitle')">
    <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
  </PageHeader>
  <p v-if="error" class="error-box">{{ error }}</p>

  <form class="card mb-4 flex flex-wrap items-end gap-3 p-4" @submit.prevent="create">
    <label class="field min-w-[220px] flex-1">{{ t('volumes.create') }}<input v-model="newName" class="input" required spellcheck="false" :placeholder="t('common.name')" /></label>
    <button class="btn btn-primary"><Icon name="plus" /> {{ t('volumes.create') }}</button>
  </form>

  <section class="card overflow-hidden">
    <Empty v-if="data && !data.volumes.length" icon="disk" :text="t('volumes.empty')" />
    <div v-else class="overflow-x-auto">
      <table class="table">
        <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('common.path') }}</th><th class="text-right">{{ t('common.actions') }}</th></tr></thead>
        <tbody>
          <tr v-for="v in data?.volumes ?? []" :key="v.name">
            <td class="font-semibold">{{ v.name }}</td>
            <td class="path">{{ v.path }}</td>
            <td>
              <div class="flex justify-end gap-1">
                <button class="btn btn-sm" @click="backup(v)"><Icon name="download" /> {{ t('volumes.backup') }}</button>
                <button class="btn btn-ghost btn-icon btn-sm btn-danger" :title="t('common.delete')" :aria-label="t('common.delete')" @click="remove(v)"><Icon name="trash" /></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <section class="card mt-6 overflow-hidden">
    <div class="card-head"><div><h2>{{ t('volumes.backups') }}</h2><p class="m-0 text-[0.8rem] text-muted">{{ t('volumes.backupsHint') }}</p></div></div>
    <Empty v-if="data && !data.backups.length" icon="download" :text="t('volumes.noBackup')" />
    <div v-else class="overflow-x-auto">
      <table class="table">
        <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('volumes.size') }}</th><th>{{ t('volumes.date') }}</th><th>{{ t('volumes.target') }}</th><th class="text-right">{{ t('common.actions') }}</th></tr></thead>
        <tbody>
          <tr v-for="b in data?.backups ?? []" :key="b.name">
            <td class="mono">{{ b.name }}</td>
            <td>{{ size(b.size) }}</td>
            <td class="whitespace-nowrap">{{ date(b.created) }}</td>
            <td>
              <select class="input min-w-[160px]" :value="guess(b)" @change="target[b.name] = ($event.target as HTMLSelectElement).value">
                <option v-for="v in data?.volumes ?? []" :key="v.name" :value="v.name">{{ v.name }}</option>
                <option v-if="!(data?.volumes ?? []).some((v) => v.name === guess(b))" :value="guess(b)">{{ guess(b) }}</option>
              </select>
            </td>
            <td class="text-right"><button class="btn btn-sm" @click="restore(b)"><Icon name="upload" /> {{ t('volumes.restore') }}</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
