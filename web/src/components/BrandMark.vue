<template>
  <span class="brand-mark" :class="[`brand-mark--${size}`]" :aria-hidden="decorative ? 'true' : undefined" :aria-label="decorative ? undefined : 'runninghub2api'">
    <svg class="brand-mark__svg" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg">
      <g class="logo-dog-wrapper">
        <path d="M10 22 C6 18 4 28 7 40 C9 48 16 50 18 42 C19 34 16 26 10 22Z" fill="#2c3e50" />
        <path d="M54 22 C58 18 60 28 57 40 C55 48 48 50 46 42 C45 34 48 26 54 22Z" fill="#2c3e50" />
        <path d="M11 28 C9 32 10 40 12 44 C14 42 15 34 13 30Z" fill="#4a657a" />
        <path d="M53 28 C55 32 54 40 52 44 C50 42 49 34 51 30Z" fill="#4a657a" />
        <path
          d="M18 18 H46 A8 8 0 0 1 54 26 V40 A16 16 0 0 1 38 56 H26 A16 16 0 0 1 10 40 V26 A8 8 0 0 1 18 18 Z"
          fill="#ffffff"
          stroke="#2c3e50"
          stroke-width="2.2"
        />
        <rect class="logo-eye" x="20" y="32" width="8" height="2.5" rx="1" />
        <rect class="logo-eye" x="36" y="32" width="8" height="2.5" rx="1" />
        <ellipse cx="32" cy="42" rx="3.2" ry="2.4" fill="#2c3e50" />
        <path d="M26 45 Q32 51 38 45" fill="none" stroke="#2c3e50" stroke-width="2" stroke-linecap="round" />
      </g>
    </svg>
  </span>
</template>

<script setup>
defineProps({
  size: {
    type: String,
    default: 'md', // sm | md | lg
  },
  decorative: {
    type: Boolean,
    default: true,
  },
})
</script>

<style scoped>
.brand-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.brand-mark__svg {
  display: block;
  width: 100%;
  height: 100%;
}

.brand-mark--sm {
  width: 2.25rem;
  height: 2.25rem;
}

.brand-mark--md {
  width: 2.75rem;
  height: 2.75rem;
}

.brand-mark--lg {
  width: 3.5rem;
  height: 3.5rem;
}

.brand-mark--xl {
  width: 4rem;
  height: 4rem;
}
</style>
