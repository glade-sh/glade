<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { editorSupportCatalog } from './generated/editorSupport'

const query = ref('')
const status = ref('all')
const page = ref(1)
const resultHeading = ref<HTMLElement | null>(null)
const pageSize = 25
const statuses = ['all', 'supported', 'partial', 'stub', 'unsupported', 'unknown']
const entries = editorSupportCatalog.rows
const hasFilters = computed(() => query.value.trim() !== '' || status.value !== 'all')
const filtered = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return entries.filter((entry) =>
    (status.value === 'all' || entry.status === status.value) &&
    (!needle || `${entry.area}.${entry.api} ${entry.notes}`.toLowerCase().includes(needle))
  )
})
const pageCount = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const shown = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))
let restoring = false
let mounted = false
function readState() {
  restoring = true
  const params = new URLSearchParams(window.location.search)
  query.value = params.get('q') || ''
  const savedStatus = params.get('status') || 'all'
  status.value = statuses.includes(savedStatus) ? savedStatus : 'all'
  const savedPage = Number(params.get('page'))
  page.value = Math.min(pageCount.value, Math.max(1, Number.isSafeInteger(savedPage) ? savedPage : 1))
  restoring = false
}
function writeState(push = false) {
  if (!mounted || restoring) return
  const url = new URL(window.location.href)
  for (const [key, value] of [['q', query.value.trim()], ['status', status.value === 'all' ? '' : status.value], ['page', page.value === 1 ? '' : String(page.value)]]) {
    if (value) url.searchParams.set(key, value)
    else url.searchParams.delete(key)
  }
  if (url.href !== window.location.href) window.history[push ? 'pushState' : 'replaceState'](window.history.state, '', url)
}
watch([query, status], () => {
  if (restoring) return
  page.value = 1
  writeState()
}, { flush: 'sync' })
async function changePage(value: number) {
  page.value = Math.max(1, Math.min(pageCount.value, value))
  writeState(true)
  await nextTick()
  resultHeading.value?.focus({ preventScroll: true })
  resultHeading.value?.scrollIntoView({ block: 'nearest' })
}
function clearFilters() {
  query.value = ''
  status.value = 'all'
}
onMounted(() => {
  readState()
  mounted = true
  writeState()
  window.addEventListener('popstate', readState)
})
onBeforeUnmount(() => window.removeEventListener('popstate', readState))
function statusClass(value: string) {
  if (value === 'supported') return 'docs-status-supported'
  if (value === 'partial' || value === 'stub') return 'docs-status-partial'
  if (value === 'unsupported') return 'docs-status-unsupported'
  return 'docs-status-unknown'
}
</script>

<template>
  <section class="support-explorer" aria-labelledby="support-explorer-heading">
    <div class="support-explorer-heading">
      <div>
        <p class="docs-card-kicker">Selected APIs</p>
        <h2 id="support-explorer-heading">Explore capability notes</h2>
      </div>

    </div>

    <div class="support-explorer-controls">
      <label>Search capability notes<input v-model="query" type="search" placeholder="Try Database.insert or Answers" autocomplete="off"></label>
      <label>Status<select v-model="status">
        <option value="all">All raw statuses</option>
        <option value="supported">supported — Runs locally</option>
        <option value="partial">partial — Runs locally with limits</option>
        <option value="stub">stub — Local stand-in</option>
        <option value="unsupported">unsupported — Requires Salesforce</option>
        <option value="unknown">unknown — Not measured</option>
      </select></label>
      <button class="support-explorer-button" type="button" @click="clearFilters">Clear filters</button>
    </div>
    <p class="support-explorer-scope">Behavior and limits for selected Apex APIs. Use the <a href="#drill-down">coverage guides</a> for the broader runtime, standard library and object model.</p>
    <p>Runs locally describes a local implementation, including deterministic models. Runs locally with limits covers partial or stub rows. Requires Salesforce identifies unsupported rows. None of these labels alone establishes Salesforce parity.</p>
    <p ref="resultHeading" class="support-explorer-result" role="status" aria-live="polite" tabindex="-1">
      <template v-if="filtered.length">
        <template v-if="hasFilters">{{ filtered.length }} matching note{{ filtered.length === 1 ? '' : 's' }}.</template>
        Page {{ page }} of {{ pageCount }}.
      </template>
      <template v-else>No matching notes. An API can be implemented without a note here. <a href="#drill-down">Browse the coverage guides</a> or clear the filters.</template>
    </p>
    <ul class="support-explorer-list" tabindex="0" aria-label="Capability notes">
      <li v-for="entry in shown" :key="entry.id">
        <div><code>{{ entry.api }}</code><span :class="['docs-status-chip', statusClass(entry.status)]">{{ editorSupportCatalog.statusLabels[entry.status] }}</span></div>
        <dl class="support-row-detail">
          <div><dt>Execution classification</dt><dd><code>{{ entry.status }}</code></dd></div>
          <div><dt>Modeled behavior and limits</dt><dd>{{ entry.notes || 'No behavior detail recorded.' }}</dd></div>
          <div><dt>Evidence</dt><dd>Checked source: <a :href="`https://github.com/glade-sh/glade/blob/main/${editorSupportCatalog.generatedFrom}`">{{ editorSupportCatalog.generatedFrom }}</a>. This row does not attach a live Salesforce parity receipt.</dd></div>
        </dl>
      </li>
    </ul>
    <nav class="support-explorer-pagination" aria-label="Capability result pages">
      <button class="support-explorer-button" type="button" :disabled="page === 1" @click="changePage(1)">First</button>
      <button class="support-explorer-button" type="button" aria-label="Previous" :disabled="page === 1" @click="changePage(page - 1)">Prev</button>
      <span>Page {{ page }} of {{ pageCount }}</span>
      <button class="support-explorer-button" type="button" :disabled="page >= pageCount" @click="changePage(page + 1)">Next</button>
      <button class="support-explorer-button" type="button" :disabled="page >= pageCount" @click="changePage(pageCount)">Last</button>
    </nav>
    <p class="support-explorer-foot">Notes come from <code>{{ editorSupportCatalog.generatedFrom }}</code>. Follow the source reports below for regression-test links and evidence scope.</p>
    <noscript>Search and pagination need JavaScript. The source reports linked below remain readable without it.</noscript>
  </section>
</template>

<style scoped>
.support-explorer-heading h2 { margin: 0; padding: 0; border: 0; }
.support-explorer-heading .docs-card-kicker { margin: 0 0 12px; }
.support-explorer-heading, .support-explorer-controls { flex-wrap: wrap; }
.support-explorer-controls label { min-width: 0; }
.support-explorer-controls select, .support-explorer-controls input { width: 100%; min-width: 0; }
.support-explorer-list li > div { flex-wrap: wrap; }
.support-explorer-button { align-self: end; border: 1px solid var(--glade-control); border-radius: 8px; padding: 8px 12px; min-height: 44px; color: var(--glade-text); background: var(--glade-surface); font: inherit; font-size: 14px; font-weight: 550; cursor: pointer; }
.support-explorer-button:hover:not(:disabled) { border-color: var(--glade-link); background: var(--glade-active); }
.support-explorer-button:disabled { opacity: .5; cursor: default; }
.support-explorer-button:focus-visible, .support-explorer-result:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 3px; }
.support-explorer-pagination { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin: 20px 0; }
.support-explorer-list { padding: 0; list-style: none; }
.support-explorer-list code { overflow-wrap: anywhere; }
.support-row-detail { margin: 12px 0 0; font-size: 13px; }
.support-row-detail > div { display: block; margin-top: 8px; }
.support-row-detail dt { font-weight: 600; color: var(--vp-c-text-2); }
.support-row-detail dd { margin: 2px 0 0; overflow-wrap: anywhere; }
@media (max-width: 640px) {
  .support-explorer-controls > .support-explorer-button { align-self: stretch; }
  .support-explorer-pagination { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
  .support-explorer-pagination > span { grid-column: 1 / -1; grid-row: 1; text-align: center; color: var(--glade-muted); font-size: 13px; }
  .support-explorer-pagination > button { padding-inline: 4px; font-size: 13px; }
}
</style>
