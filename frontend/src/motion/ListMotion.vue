<script setup lang="ts">
// A list whose rows slide in, fade out and move smoothly:
// <ListMotion tag="div" class="rows"><Row v-for … :key …/></ListMotion>.
// Rows need stable keys. While a row is dragged (drag.ts sets body.sorting)
// the move animation is left to the drag code.
withDefaults(defineProps<{ tag?: string }>(), { tag: 'div' })
</script>

<template>
  <TransitionGroup :tag="tag" name="nb-list">
    <slot />
  </TransitionGroup>
</template>

<style>
body.sorting .nb-list-move {
  transition: none;
}
</style>
