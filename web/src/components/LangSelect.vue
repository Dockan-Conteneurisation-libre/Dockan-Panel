<script setup lang="ts">
import { lang, langs, setLang, t, type Lang } from '../i18n'

defineProps<{ dark?: boolean }>()
function change(e: Event): void {
  setLang((e.target as HTMLSelectElement).value as Lang)
}
</script>

<template>
  <label class="lang" :class="{ dark }">
    <span class="sr-only">{{ t('nav.language') }}</span>
    <select :value="lang" @change="change">
      <option v-for="l in langs" :key="l.code" :value="l.code">{{ l.code.toUpperCase() }} · {{ l.label }}</option>
    </select>
  </label>
</template>

<style scoped>
.lang select {
  min-height: 34px; padding: 0 0.55rem; border-radius: 7px; cursor: pointer;
  border: 1px solid var(--color-line); background: var(--color-surface); color: var(--color-ink);
  font: 600 0.8rem/1 var(--font-sans);
}
.lang.dark select { background: var(--color-pine-2); color: #e2ebe5; border-color: var(--color-pine-line); }
</style>
