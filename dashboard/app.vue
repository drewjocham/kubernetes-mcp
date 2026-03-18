<template>
  <NConfigProvider>
    <NMessageProvider>
      <NuxtPage />
    </NMessageProvider>
  </NConfigProvider>
</template>

<script setup lang="ts">
import { NConfigProvider, NMessageProvider } from 'naive-ui'
</script>
