<script setup lang="ts">
defineProps<{ title?: string; index?: number }>()
</script>

<template>
  <section class="card" :style="{ '--i': index ?? 0 }">
    <div v-if="title || $slots.actions" class="card-head">
      <div class="card-title">{{ title }}</div>
      <slot name="actions" />
    </div>
    <div class="card-body">
      <slot />
    </div>
  </section>
</template>
