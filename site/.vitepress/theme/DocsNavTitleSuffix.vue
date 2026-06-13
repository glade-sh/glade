<script setup lang="ts">
import { useData } from 'vitepress'
import { computed } from 'vue'

const { frontmatter } = useData()

const isDocs = computed(() => frontmatter.value.layout !== 'home')
</script>

<template>
  <span v-if="isDocs" class="docs-title-suffix">&nbsp;Docs</span>
</template>
