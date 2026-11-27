<script setup lang="ts">
import { computed } from 'vue'
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusPill from '../components/StatusPill.vue'
import Empty from '../components/Empty.vue'
import { t, type Key } from '../i18n'
import type { Overview } from '../lib/api'
import type { IconName } from '../lib/icons'
import { useLoad } from '../lib/useLoad'

const { data, loading, error, reload } = useLoad<Overview>(() => '/api/overview')

const tiles = computed(() => {
  const d = data.value
  const list: { label: Key; value: number; icon: IconName; to: string }[] = [
    { label: 'overview.images', value: d?.images ?? 0, icon: 'layers', to: '/images' },
    { label: 'overview.volumes', value: d?.volumes ?? 0, icon: 'disk', to: '/volumes' },
    { label: 'overview.networks', value: d?.networks ?? 0, icon: 'network', to: '/networks' },
    { label: 'overview.stacks', value: d?.stacks ?? 0, icon: 'stack', to: '/stacks' },
  ]
  return list
})
const share = computed(() => {
  const d = data.value
  return d && d.containers > 0 ? Math.round((d.running / d.containers) * 100) : 0
})
</script>

<template>
  <PageHeader :title="t('overview.title')" :subtitle="t('overview.subtitle')">
    <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
  </PageHeader>

  <p v-if="error" class="error-box">{{ error }}</p>

  <div class="grid gap-4 lg:grid-cols-[1.25fr_2fr]">
    <RouterLink to="/containers" class="card hero">
      <div class="flex items-center justify-between">
        <span class="text-[0.82rem] font-semibold text-mint">{{ t('overview.containers') }}</span>
        <Icon name="box" :size="20" class="text-mint" />
      </div>
      <div class="mt-3 flex items-end gap-3">
        <strong class="text-[2.6rem] leading-none">{{ data?.running ?? '—' }}</strong>
        <span class="pb-1 text-[0.95rem] text-[#cfe3d8]">/ {{ data?.containers ?? 0 }} · {{ t('overview.running') }}</span>
      </div>
      <div class="bar" role="presentation"><span :style="{ width: share + '%' }" /></div>
      <p class="m-0 text-[0.82rem] text-[#a9c7b8]">{{ t('overview.stopped', { n: (data?.containers ?? 0) - (data?.running ?? 0) }) }}</p>
    </RouterLink>
    <div class="grid grid-cols-2 gap-4">
      <RouterLink v-for="tile in tiles" :key="tile.label" :to="tile.to" class="card tile">
        <span class="grid h-9 w-9 place-items-center rounded-lg bg-code text-teal"><Icon :name="tile.icon" /></span>
        <div>
          <strong class="block text-[1.5rem] leading-tight">{{ data ? tile.value : '—' }}</strong>
          <span class="text-[0.84rem] text-muted">{{ t(tile.label) }}</span>
        </div>
      </RouterLink>
    </div>
  </div>

  <div class="mt-4 flex flex-wrap gap-2">
    <RouterLink to="/containers?new=1" class="btn btn-primary"><Icon name="plus" /> {{ t('overview.newContainer') }}</RouterLink>
    <RouterLink to="/images?pull=1" class="btn"><Icon name="download" /> {{ t('overview.pullImage') }}</RouterLink>
    <RouterLink to="/store" class="btn"><Icon name="store" /> {{ t('overview.openStore') }}</RouterLink>
  </div>

  <div class="mt-6 grid gap-4 xl:grid-cols-[3fr_2fr]">
    <section class="card overflow-hidden">
      <div class="card-head"><h2>{{ t('overview.recent') }}</h2></div>
      <Empty v-if="data && !data.recent.length" icon="box" :text="t('containers.empty')" />
      <div v-else class="overflow-x-auto">
        <table class="table">
          <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('common.status') }}</th><th>{{ t('containers.image') }}</th><th>{{ t('containers.ports') }}</th></tr></thead>
          <tbody>
            <tr v-for="c in data?.recent ?? []" :key="c.store + c.name">
              <td><RouterLink class="link" :to="`/containers/${c.store || '-'}/${c.name}`">{{ c.name }}</RouterLink></td>
              <td><StatusPill :status="c.status" /></td>
              <td class="mono text-muted">{{ c.image }}</td>
              <td class="mono">{{ c.ports || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
    <section class="card">
      <div class="card-head">
        <h2>{{ t('overview.doctor') }}</h2>
        <span class="pill mono">{{ data?.dockanVersion.split('\n')[0] || '—' }}</span>
      </div>
      <div class="card-body">
        <p class="mt-0 mb-2 text-[0.82rem] text-muted">{{ t('overview.doctorHint') }}</p>
        <pre class="pre">{{ data?.doctor || '—' }}</pre>
      </div>
    </section>
  </div>
</template>

<style scoped>
.hero {
  display: grid; gap: 0.6rem; padding: 1.3rem; text-decoration: none; color: #fff;
  background: linear-gradient(140deg, var(--color-pine) 0%, var(--color-pine-3) 100%); border-color: var(--color-pine-line);
}
.bar { height: 6px; border-radius: 3px; background: rgb(255 255 255 / 0.12); overflow: hidden; }
.bar span { display: block; height: 100%; background: var(--color-mint); border-radius: 3px; transition: width 0.3s; }
.tile { display: flex; align-items: center; gap: 0.9rem; padding: 1rem 1.1rem; color: inherit; text-decoration: none; }
.tile:hover, .hero:hover { border-color: var(--color-teal); }
</style>
