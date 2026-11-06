<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, default: '' },
  tone: {
    type: String,
    default: 'neutral',
    validator: (v) => ['neutral', 'success', 'warning', 'error', 'info'].includes(v),
  },
  title: { type: String, default: '' },
})

const toneClass = computed(() => `tone-pill--${props.tone || 'neutral'}`)
</script>

<template>
  <span class="tone-pill" :class="toneClass" :title="title || undefined">{{ label }}</span>
</template>

<style scoped>
.tone-pill {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  padding: 0.2rem 0.55rem;
  border: 1px solid transparent;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.25;
  white-space: nowrap;
}

.tone-pill--neutral {
  border-color: #e4e4e7;
  background: #fafafa;
  color: #71717a;
}

.tone-pill--success {
  border-color: #bbf7d0;
  background: #f0fdf4;
  color: #3f8f6b;
}

.tone-pill--warning {
  border-color: #fde68a;
  background: #fffbeb;
  color: #b45309;
}

.tone-pill--error {
  border-color: #fecdd3;
  background: #fff1f2;
  color: #be123c;
}

.tone-pill--info {
  border-color: #bae6fd;
  background: #f0f9ff;
  color: #0369a1;
}

:global(html[data-theme='dark']) .tone-pill--neutral {
  border-color: #3f3f46;
  background: #18181b;
  color: #a1a1aa;
}

:global(html[data-theme='dark']) .tone-pill--success {
  border-color: #14532d;
  background: #052e1c;
  color: #86efac;
}

:global(html[data-theme='dark']) .tone-pill--warning {
  border-color: #78350f;
  background: #451a03;
  color: #fcd34d;
}

:global(html[data-theme='dark']) .tone-pill--error {
  border-color: #9f1239;
  background: #4c0519;
  color: #fda4af;
}

:global(html[data-theme='dark']) .tone-pill--info {
  border-color: #0c4a6e;
  background: #082f49;
  color: #7dd3fc;
}
</style>
