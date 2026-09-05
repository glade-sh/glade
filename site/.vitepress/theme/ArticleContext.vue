<script setup lang="ts">
import { computed } from 'vue'
import { useData, withBase } from 'vitepress'
const { page, frontmatter } = useData()
const section = computed(() => {
  const path = page.value.relativePath
  if (path.startsWith('maintainer/')) return { label: 'Contributors', href: '/maintainer/' }
  if (path.startsWith('help/')) return { label: 'Help', href: '/help/' }
  if (path.startsWith('reference/')) return { label: 'Reference', href: '/reference/cli' }
  if (path === 'guide/workflows.md' || path.startsWith('guide/workflows/')) return { label: 'Guides', href: '/guide/workflows' }
  if (path.includes('support-map') || path === 'guide/workbench.md') return { label: 'Compatibility', href: '/guide/support-map' }
  return { label: 'Docs', href: '/guide/' }
})
const pageType = computed(() => frontmatter.value.pageType || (section.value.label === 'Reference' ? 'Reference' : section.value.label === 'Contributors' ? 'Contributor guide' : 'Guide'))
</script>

<template>
  <div class="glade-article-context" v-if="!page.isNotFound">
    <nav aria-label="Breadcrumb"><a :href="withBase(section.href)">{{ section.label }}</a><template v-if="page.title !== section.label"><span aria-hidden="true"> / </span><span>{{ page.title }}</span></template></nav>
    <p class="glade-page-type">{{ pageType }}</p>
  </div>
</template>
