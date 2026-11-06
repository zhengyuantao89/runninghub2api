<template>
  <section class="ui-panel page-panel" :class="{ 'page-panel--flush': flush }">
    <slot />
  </section>
</template>

<script setup>
defineProps({
  flush: { type: Boolean, default: false },
})
</script>

<style scoped>
.page-panel--flush {
  overflow: hidden;
  padding: 0 !important;
}
</style>
