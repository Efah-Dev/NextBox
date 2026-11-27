<script setup lang="ts">
// A block that opens and closes smoothly: <Reveal :open="x">…</Reveal>.
// The content is only rendered while open.
defineProps<{ open: boolean }>()
</script>

<template>
  <Transition name="nb-reveal">
    <div v-if="open" class="nb-reveal">
      <div class="nb-reveal-in"><slot /></div>
    </div>
  </Transition>
</template>
