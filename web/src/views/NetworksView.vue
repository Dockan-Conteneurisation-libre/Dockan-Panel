<script setup lang="ts">
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import Empty from '../components/Empty.vue'
import { t } from '../i18n'
import type { Network } from '../lib/api'
import { useLoad } from '../lib/useLoad'

const { data, loading, error, reload } = useLoad<Network[]>(() => '/api/networks')
</script>

<template>
  <PageHeader :title="t('networks.title')" :subtitle="t('networks.subtitle')">
    <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
  </PageHeader>
  <p v-if="error" class="error-box">{{ error }}</p>
  <section class="card overflow-hidden">
    <Empty v-if="data && !data.length" icon="network" :text="t('networks.empty')" />
    <div v-else class="overflow-x-auto">
      <table class="table">
        <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('networks.driver') }}</th><th>{{ t('networks.subnet') }}</th><th>{{ t('networks.bridge') }}</th></tr></thead>
        <tbody>
          <tr v-for="n in data ?? []" :key="n.name">
            <td class="font-semibold">{{ n.name }}</td>
            <td><span class="pill">{{ n.driver || '—' }}</span></td>
            <td class="mono">{{ n.subnet || '—' }}</td>
            <td class="mono">{{ n.bridge || '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
