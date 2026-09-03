<script setup lang="ts">
import { onContentUpdated, useRoute } from 'vitepress'
import { nextTick, onMounted, onUnmounted, watch } from 'vue'

const route = useRoute()
const loadedScripts = new Map<string, Promise<void>>()

declare global {
  interface Window {
    gladeHighlightAllCodeBlocks?: () => void
    gladeInitHomeDemos?: () => void
  }
}

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
    const href = link.getAttribute('href')?.split('#', 1)[0].replace(/\/$/, '')
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

function repairSidebarDisclosureControls() {
  document.querySelectorAll<HTMLElement>('.VPSidebar .item[role="button"] .caret[role="button"]').forEach((caret) => {
    caret.removeAttribute('role')
    caret.removeAttribute('tabindex')
    caret.removeAttribute('aria-label')
  })
}

function closeMobileNavigationOnEscape(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  const button = document.querySelector<HTMLButtonElement>('.VPNavBarHamburger[aria-expanded="true"]')
  if (!button) return
  button.click()
  button.focus({ preventScroll: true })
}

function setupCommandFilter() {
  document.querySelectorAll<HTMLInputElement>('[data-command-filter]').forEach((input) => {
    if (input.dataset.enhanced === 'true') return

    const targetSelector = input.dataset.commandFilter || '.docs-command-card'
    const cards = Array.from(document.querySelectorAll<HTMLElement>(targetSelector))
    let result = input.parentElement?.querySelector<HTMLElement>('[data-command-filter-status]')
    if (!result) {
      result = document.createElement('p')
      result.dataset.commandFilterStatus = ''
      result.className = 'docs-command-filter-status'
      result.setAttribute('role', 'status')
      result.setAttribute('aria-live', 'polite')
      input.insertAdjacentElement('afterend', result)
    }

    const update = () => {
      const query = input.value.trim().toLowerCase()
      let visible = 0

      cards.forEach((card) => {
        const haystack = card.textContent?.toLowerCase() || ''
        card.hidden = query.length > 0 && !haystack.includes(query)
        if (!card.hidden) visible += 1
      })
      result!.textContent = query.length > 0
        ? `${visible} command group${visible === 1 ? '' : 's'} match “${input.value.trim()}”.${visible === 0 ? ' Try a different command or browse the full list.' : ''}`
        : `${visible} command groups available.`
    }

    input.dataset.enhanced = 'true'
    input.addEventListener('input', update)
    update()
  })
}

async function loadRouteAssets() {
  await loadScript('/js/highlight.js')
  if (route.path === '/' || route.path === '/guide/workbench') {
    await loadScript('/js/home.js')
  }
}

function loadScript(src: string) {
  const existing = loadedScripts.get(src)
  if (existing) return existing
  const loaded = new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => reject(new Error(`could not load ${src}`))
    document.head.appendChild(script)
  })
  loadedScripts.set(src, loaded)
  return loaded
}

async function enhanceDocs() {
  if (typeof document === 'undefined') return

  await loadRouteAssets()

  nextTick(() => {
    repairSidebarDisclosureControls()
    updateSidebarCurrent()
    setupCommandFilter()
    window.gladeHighlightAllCodeBlocks?.()
    window.gladeInitHomeDemos?.()
    window.dispatchEvent(new CustomEvent('glade:content-updated'))
  })
}

onMounted(() => {
  document.addEventListener('keydown', closeMobileNavigationOnEscape)
  enhanceDocs()
})
onUnmounted(() => document.removeEventListener('keydown', closeMobileNavigationOnEscape))
onContentUpdated(enhanceDocs)
watch(() => route.path, enhanceDocs)
</script>

<template>
  <span class="docs-enhancer-root" hidden aria-hidden="true"></span>
</template>
