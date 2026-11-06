<template>
  <RouterView v-if="isLogin" />
  <AppShell v-else />
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from '@/layouts/AppShell.vue'

const route = useRoute()
const isLogin = computed(() => route.path === '/login')
</script>
