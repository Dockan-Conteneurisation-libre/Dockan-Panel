<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <header class="mb-6 flex flex-wrap items-end justify-between gap-4">
    <div class="min-w-0">
      <h1 class="m-0 text-[1.45rem] leading-tight font-[680] tracking-[-0.01em]">{{ title }}</h1>
      <p v-if="subtitle" class="mt-1 mb-0 text-[0.9rem] text-muted">{{ subtitle }}</p>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <slot />
    </div>
  </header>
</template>
