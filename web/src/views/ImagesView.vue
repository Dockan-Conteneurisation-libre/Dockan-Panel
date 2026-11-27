<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import Empty from '../components/Empty.vue'
import Drawer from '../components/Drawer.vue'
import { t } from '../i18n'
import type { Image } from '../lib/api'
import { useLoad } from '../lib/useLoad'
import { run } from '../lib/state'
import { confirmAction } from '../lib/confirm'

const { data, loading, error, reload } = useLoad<Image[]>(() => '/api/images')
const route = useRoute()
const router = useRouter()
const showPull = ref(false)
const ref_ = ref('')
const filter = ref('')

onMounted(() => {
  if (route.query.pull === '1') {
    showPull.value = true
    void router.replace({ query: {} })
  }
})

const rows = computed(() => {
  const q = filter.value.trim().toLowerCase()
  return (data.value ?? []).filter((i) => !q || i.tag.toLowerCase().includes(q))
})

async function pull(): Promise<void> {
  if ((await run(`pull ${ref_.value}`, '/api/images/pull', { ref: ref_.value }, t('images.pulled'))) !== null) {
    ref_.value = ''
    showPull.value = false
    await reload()
  }
}

async function remove(i: Image): Promise<void> {
  if (!(await confirmAction(t('images.confirmRemove', { name: i.tag })))) return
  if ((await run(`rmi ${i.tag}`, '/api/images/remove', { ref: i.tag }, t('images.removed'))) !== null) await reload()
}
</script>

<template>
  <PageHeader :title="t('images.title')" :subtitle="t('images.subtitle')">
    <button class="btn" :disabled="loading" @click="reload"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
    <button class="btn btn-primary" @click="showPull = true"><Icon name="download" /> {{ t('images.pull') }}</button>
  </PageHeader>
  <p v-if="error" class="error-box">{{ error }}</p>

  <section class="card overflow-hidden">
    <div class="card-head">
      <input v-model="filter" class="input max-w-[340px]" :placeholder="t('common.search')" :aria-label="t('common.search')" />
      <span class="pill">{{ rows.length }}</span>
    </div>
    <Empty v-if="data && !rows.length" icon="layers" :text="t('images.empty')" />
    <div v-else class="overflow-x-auto">
      <table class="table">
        <thead><tr><th>{{ t('images.tag') }}</th><th>{{ t('common.path') }}</th><th class="text-right">{{ t('common.actions') }}</th></tr></thead>
        <tbody>
          <tr v-for="i in rows" :key="i.tag">
            <td class="mono font-semibold">{{ i.tag }}</td>
            <td class="path">{{ i.path }}</td>
            <td class="text-right">
              <button class="btn btn-ghost btn-icon btn-sm btn-danger" :title="t('common.delete')" :aria-label="t('common.delete')" @click="remove(i)"><Icon name="trash" /></button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <Drawer v-if="showPull" :title="t('images.pull')" @close="showPull = false">
    <form id="pull-form" class="grid gap-4" @submit.prevent="pull">
      <label class="field">{{ t('images.ref') }}
        <input v-model="ref_" class="input mono" required spellcheck="false" placeholder="docker.io/library/nginx:alpine" />
        <span class="hint">{{ t('images.refHint') }}</span>
        <span class="hint">{{ t('run.imageHint') }}</span>
      </label>
    </form>
    <template #footer>
      <button class="btn" @click="showPull = false">{{ t('common.cancel') }}</button>
      <button class="btn btn-primary" form="pull-form"><Icon name="download" /> {{ t('images.pullSubmit') }}</button>
    </template>
  </Drawer>
</template>
