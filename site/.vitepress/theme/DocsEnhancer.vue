<script setup lang="ts">
import { onContentUpdated, useRoute, withBase } from 'vitepress'
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

function setupCommandLookup() {
  document.querySelectorAll<HTMLInputElement>('[data-command-lookup]').forEach((input) => {
    if (input.dataset.enhanced === 'true') return
    const article = input.closest('.vp-doc')
    if (!article) return
    // Index the rendered reference itself so results cannot drift from its anchors.
    const sections = Array.from(article.querySelectorAll<HTMLElement>('h2[id], h3[id]')).map((heading) => {
      const fragments: string[] = []
      let sibling = heading.nextElementSibling
      while (sibling && !/^H[23]$/.test(sibling.tagName)) {
        fragments.push(sibling.textContent || '')
        sibling = sibling.nextElementSibling
      }
      return { id: heading.id, title: heading.textContent?.replace(/[\u200B-\u200D\uFEFF]/g, '').replace(/\s*#\s*$/, '').trim() || '', text: fragments.join(' ').replace(/\s+/g, ' ') }
    })
    const status = document.createElement('p')
    status.dataset.commandLookupStatus = ''
    status.setAttribute('role', 'status')
    status.setAttribute('aria-live', 'polite')
    const results = document.createElement('ul')
    results.className = 'docs-command-lookup-results'
    results.setAttribute('aria-label', 'Command and flag matches')
    results.tabIndex = 0
    results.hidden = true
    input.insertAdjacentElement('afterend', status)
    status.insertAdjacentElement('afterend', results)
    const update = () => {
      const needle = input.value.trim().toLowerCase()
      results.replaceChildren()
      results.hidden = !needle
      if (!needle) {
        status.textContent = 'Search command names, flags, and reference details. The complete reference stays below.'
        return
      }
      const matches = sections.filter((section) => `${section.title} ${section.text}`.toLowerCase().includes(needle))
      status.textContent = `${matches.length} reference section${matches.length === 1 ? '' : 's'} match “${input.value.trim()}”.${matches.length ? '' : ' Try a command name or a shorter flag.'}`
      results.hidden = matches.length === 0
      for (const match of matches) {
        const item = document.createElement('li')
        const link = document.createElement('a')
        link.href = `#${match.id}`
        link.textContent = match.title
        const detail = document.createElement('p')
        const offset = Math.max(0, match.text.toLowerCase().indexOf(needle) - 60)
        detail.textContent = `${offset ? '…' : ''}${match.text.slice(offset, offset + 140)}${match.text.length > offset + 140 ? '…' : ''}`
        item.append(link, detail)
        results.append(item)
      }
    }
    input.dataset.enhanced = 'true'
    input.addEventListener('input', update)
    update()
  })
}

async function loadRouteAssets() {
  await loadScript(withBase('/js/highlight.js'))
  if (route.path === withBase('/guide/workbench') || route.path === withBase('/guide/workbench.html')) {
    await loadScript(withBase('/js/home.js'))
  }
}

function loadScript(src: string) {
  const existing = loadedScripts.get(src)
  if (existing) return existing

  // This component is recreated when the custom home swaps with VitePress's
  // default layout. Keep route assets process-wide so revisiting Workbench
  // cannot evaluate their global event handlers again.
  const assetURL = new URL(src, document.baseURI).href
  const prior = Array.from(document.querySelectorAll<HTMLScriptElement>('script[data-glade-route-asset]'))
    .find((script) => script.src === assetURL)
  if (prior) {
    const loaded = prior.dataset.gladeAssetState === 'loaded'
      ? Promise.resolve()
      : new Promise<void>((resolve, reject) => {
          prior.addEventListener('load', () => resolve(), { once: true })
          prior.addEventListener('error', () => reject(new Error(`could not load ${src}`)), { once: true })
        })
    loadedScripts.set(src, loaded)
    return loaded
  }

  const loaded = new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src
    script.async = true
    script.dataset.gladeRouteAsset = src
    script.dataset.gladeAssetState = 'loading'
    script.onload = () => { script.dataset.gladeAssetState = 'loaded'; resolve() }
    script.onerror = () => { loadedScripts.delete(src); script.remove(); reject(new Error(`could not load ${src}`)) }
    document.head.appendChild(script)
  })
  loadedScripts.set(src, loaded)
  return loaded
}

// VitePress's local-search dialog is teleported to body and does not restore
// its opener on dismissal. Observe only body children, not the article subtree.
let searchObserver: MutationObserver | undefined
let searchOpener: HTMLElement | undefined
let searchLocation = ''
function rememberSearchOpener(event: MouseEvent | KeyboardEvent) {
  if (!(event.target instanceof Element) || event.target.closest('.VPLocalSearchBox')) return
  const trigger = event.target.closest<HTMLElement>('.VPNavBarSearchButton')
  const shortcut = event instanceof KeyboardEvent && ((event.key === 'k' && (event.metaKey || event.ctrlKey)) || event.key === '/')
  if (!trigger && !shortcut) return
  searchOpener = trigger || document.activeElement as HTMLElement
  searchLocation = window.location.href
}
function observeSearchDismissal() {
  searchObserver = new MutationObserver((records) => {
    for (const record of records) for (const node of record.removedNodes) {
      if (!(node instanceof HTMLElement) || !node.classList.contains('VPLocalSearchBox')) continue
      const opener = searchOpener
      if (opener?.isConnected && searchLocation === window.location.href) opener.focus()
      searchOpener = undefined
    }
  })
  searchObserver.observe(document.body, { childList: true })
}

let enhancementRevision = 0
async function enhanceDocs() {
  if (typeof document === 'undefined') return

  const revision = ++enhancementRevision
  try { await loadRouteAssets() } catch (error) { console.error(error) }

  nextTick(() => {
    if (revision !== enhancementRevision) return
    repairSidebarDisclosureControls()
    updateSidebarCurrent()
    setupCommandFilter()
    setupCommandLookup()
    window.gladeHighlightAllCodeBlocks?.()
    window.gladeInitHomeDemos?.()
    window.dispatchEvent(new CustomEvent('glade:content-updated'))
  })
}

onMounted(() => {
  document.addEventListener('keydown', closeMobileNavigationOnEscape)
  document.addEventListener('click', rememberSearchOpener, true)
  document.addEventListener('keydown', rememberSearchOpener, true)
  observeSearchDismissal()
  enhanceDocs()
})
onUnmounted(() => {
  enhancementRevision++
  document.removeEventListener('keydown', closeMobileNavigationOnEscape)
  document.removeEventListener('click', rememberSearchOpener, true)
  document.removeEventListener('keydown', rememberSearchOpener, true)
  searchObserver?.disconnect()
})
onContentUpdated(enhanceDocs)
watch(() => route.path, enhanceDocs)
</script>

<template>
  <span class="docs-enhancer-root" hidden aria-hidden="true"></span>
</template>
