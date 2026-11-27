<script setup lang="ts">
import Icon from './Icon.vue'
import type { IconName } from '../lib/icons'

defineProps<{ icon: IconName; text: string }>()
</script>

<template>
  <div class="grid place-items-center gap-3 px-6 py-12 text-center text-muted">
    <span class="grid h-12 w-12 place-items-center rounded-full bg-code text-teal">
      <Icon :name="icon" :size="22" />
    </span>
    <p class="m-0 max-w-[420px] text-[0.9rem]">{{ text }}</p>
    <slot />
  </div>
</template>
