<template>
  <NConfigProvider
    :theme="darkTheme"
    :theme-overrides="themeOverrides"
  >
    <NMessageProvider>
      <NuxtPage />
    </NMessageProvider>
  </NConfigProvider>
</template>

<script setup lang="ts">
import { NConfigProvider, NMessageProvider, darkTheme } from 'naive-ui'

const themeOverrides = {
  common: {
    primaryColor: '#8B5CF6',
    primaryColorHover: '#7C3AED',
    primaryColorPressed: '#6D28D9'
  }
}
</script>
