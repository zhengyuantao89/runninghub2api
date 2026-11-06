<template>
  <div
    class="filter-toolbar"
    :class="[
      bordered ? 'filter-toolbar--bordered' : '',
      `filter-toolbar--gap-${gap}`,
      `filter-toolbar--mobile-${mobileMode}`,
      align === 'start' ? 'filter-toolbar--align-start' : '',
    ]"
  >
    <slot />
  </div>
</template>

<script setup>
defineProps({
  bordered: { type: Boolean, default: true },
  gap: { type: String, default: 'normal' },
  mobileMode: { type: String, default: 'wrap' },
  align: { type: String, default: 'center' },
})
</script>

<style scoped>
.filter-toolbar {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
}
.filter-toolbar--align-start {
  align-items: flex-start;
}
.filter-toolbar--bordered {
  padding-top: 14px;
  border-top: 1px solid hsl(var(--border) / 0.82);
}
.filter-toolbar--gap-tight { gap: 8px; }
.filter-toolbar--gap-normal { gap: 10px; }
.filter-toolbar--gap-loose { gap: 12px; }
@media (max-width: 640px) {
  .filter-toolbar--mobile-wrap { width: 100%; }
  .filter-toolbar--mobile-stack { flex-direction: column; align-items: stretch; }
}
</style>
