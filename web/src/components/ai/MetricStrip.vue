<template>
  <section class="metric-strip" :class="columnsClass">
    <article
      v-for="item in items"
      :key="item.key || item.label"
      class="metric-strip-card"
      :class="[density === 'compact' ? 'metric-strip-card--compact' : '', item.cardClass || '']"
    >
      <div class="metric-strip-card-body">
        <span class="metric-strip-text">
          <span class="metric-strip-label">{{ item.label }}</span>
          <strong class="metric-strip-value" :class="item.valueClass || item.class || ''">
            {{ item.value }}
          </strong>
          <span v-if="item.meta" class="metric-strip-meta">{{ item.meta }}</span>
        </span>
      </div>
    </article>
  </section>
</template>

<script setup>
defineProps({
  items: { type: Array, required: true },
  columnsClass: { type: String, default: 'grid-cols-2 md:grid-cols-3 xl:grid-cols-4' },
  density: { type: String, default: 'normal' },
})
</script>

<style scoped>
.metric-strip {
  display: grid;
  gap: 12px;
}
.metric-strip-card {
  position: relative;
  min-width: 0;
  min-height: 76px;
  padding: 16px;
  border: 1px solid hsl(var(--border) / 0.82);
  border-radius: 16px;
  background: hsl(var(--card));
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}
.metric-strip-card--compact {
  min-height: 64px;
  padding: 10px 12px;
}
.metric-strip-card-body {
  display: flex;
  min-width: 0;
  height: 100%;
  align-items: center;
  gap: 12px;
}
.metric-strip-text {
  display: block;
  min-width: 0;
  line-height: 1.2;
}
.metric-strip-label {
  display: block;
  overflow: hidden;
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.metric-strip-value {
  display: block;
  overflow: hidden;
  margin-top: 5px;
  font-size: 18px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.metric-strip-card:not(.metric-strip-card--compact) .metric-strip-value {
  margin-top: 8px;
  font-size: 24px;
}
.metric-strip-meta {
  display: block;
  overflow: hidden;
  margin-top: 4px;
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
