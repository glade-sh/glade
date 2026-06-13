<script setup lang="ts">
import { onContentUpdated, useRoute } from 'vitepress'
import { nextTick, onMounted, watch } from 'vue'

const route = useRoute()

function updateSidebarCurrent() {
  document
    .querySelectorAll('.VPSidebar a[aria-current="page"]')
    .forEach((link) => link.removeAttribute('aria-current'))

  document
    .querySelectorAll('.VPSidebarItem.is-active[data-glade-active="true"]')
    .forEach((item) => {
      item.classList.remove('is-active')
      item.removeAttribute('data-glade-active')
    })

  const currentPath = window.location.pathname.replace(/\/$/, '')

  document.querySelectorAll('.VPSidebarItem.is-link > .item > a.link').forEach((link) => {
    const href = link.getAttribute('href')?.replace(/\/$/, '')
    if (href && href === currentPath) {
      const item = link.closest('.VPSidebarItem')
      item?.classList.add('is-active')
      item?.setAttribute('data-glade-active', 'true')
    }
  })

  document
    .querySelectorAll('.VPSidebarItem.is-active > .item > a.link')
    .forEach((link) => link.setAttribute('aria-current', 'page'))
}

function setupCommandFilter() {
  document.querySelectorAll<HTMLInputElement>('[data-command-filter]').forEach((input) => {
    if (input.dataset.enhanced === 'true') return

    const targetSelector = input.dataset.commandFilter || '.docs-command-card'
    const cards = Array.from(document.querySelectorAll<HTMLElement>(targetSelector))

    const update = () => {
      const query = input.value.trim().toLowerCase()

      cards.forEach((card) => {
        const haystack = card.textContent?.toLowerCase() || ''
        card.hidden = query.length > 0 && !haystack.includes(query)
      })
    }

    input.dataset.enhanced = 'true'
    input.addEventListener('input', update)
    update()
  })
}

function enhanceDocs() {
  if (typeof document === 'undefined') return

  nextTick(() => {
    updateSidebarCurrent()
    setupCommandFilter()
  })
}

onMounted(enhanceDocs)
onContentUpdated(enhanceDocs)
watch(() => route.path, enhanceDocs)
</script>

<template></template>
