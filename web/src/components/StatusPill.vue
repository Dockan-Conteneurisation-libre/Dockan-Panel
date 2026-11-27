<script setup lang="ts">
import { computed } from 'vue'
import { t, type Key } from '../i18n'

const props = defineProps<{ status: string }>()
const known: Record<string, Key> = {
  running: 'status.running',
  stopped: 'status.stopped',
  exited: 'status.exited',
  files: 'status.files',
}
const label = computed(() => {
  const k = known[props.status]
  return k ? t(k) : props.status || t('status.unknown')
})
const tone = computed(() => (props.status === 'running' ? 'on' : props.status === 'files' ? 'idle' : 'off'))
</script>

<template>
  <span class="pill" :class="`tone-${tone}`">
    <span class="dot" aria-hidden="true" />
    {{ label }}
  </span>
</template>

<style scoped>
.dot { width: 7px; height: 7px; border-radius: 50%; background: currentColor; }
.tone-on { color: var(--color-leaf); border-color: color-mix(in srgb, var(--color-leaf) 35%, transparent); background: color-mix(in srgb, var(--color-leaf) 9%, transparent); }
.tone-on .dot { box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-leaf) 22%, transparent); }
.tone-off { color: var(--color-muted); }
.tone-idle { color: var(--color-signal); border-color: color-mix(in srgb, var(--color-signal) 35%, transparent); }
</style>
