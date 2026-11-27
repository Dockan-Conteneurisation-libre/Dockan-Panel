<script setup lang="ts">
// Side panel for forms and details; closes on Escape or backdrop click.
import { onBeforeUnmount, onMounted } from 'vue'
import Icon from './Icon.vue'
import { t } from '../i18n'

defineProps<{ title: string; subtitle?: string; wide?: boolean }>()
const emit = defineEmits<{ close: [] }>()

function onKey(e: KeyboardEvent): void {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-40 bg-[rgb(8_20_14/0.45)]" @click="emit('close')" />
    <aside
      class="fixed inset-y-0 right-0 z-50 flex w-full flex-col border-l border-line bg-paper shadow-2xl"
      :class="wide ? 'max-w-[760px]' : 'max-w-[560px]'"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
    >
      <div class="flex items-start justify-between gap-3 border-b border-line bg-surface px-5 py-4">
        <div class="min-w-0">
          <h2 class="m-0 text-[1.1rem] font-[680] break-words">{{ title }}</h2>
          <p v-if="subtitle" class="mt-0.5 mb-0 text-[0.84rem] text-muted break-words">{{ subtitle }}</p>
        </div>
        <button class="btn btn-ghost btn-icon" :aria-label="t('common.close')" @click="emit('close')">
          <Icon name="close" />
        </button>
      </div>
      <div class="flex-1 overflow-y-auto p-5">
        <slot />
      </div>
      <div v-if="$slots.footer" class="flex flex-wrap justify-end gap-2 border-t border-line bg-surface px-5 py-3">
        <slot name="footer" />
      </div>
    </aside>
  </Teleport>
</template>
