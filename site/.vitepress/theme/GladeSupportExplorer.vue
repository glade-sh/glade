<script setup lang="ts">
import { computed, ref } from 'vue'
import { editorSupportCatalog } from './generated/editorSupport'

const query = ref('')
const status = ref('all')

const entries = Object.entries(editorSupportCatalog.receivers).flatMap(([receiver, value]) =>
  value.items.map((item) => ({ ...item, receiver, id: `${receiver}.${item.signature || item.label}` }))
)

const filtered = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return entries.filter((entry) => {
    const statusMatches = status.value === 'all' || entry.status === status.value || (status.value === 'partial' && entry.status === 'stub')
    const textMatches = !needle || `${entry.receiver}.${entry.label} ${entry.signature || ''} ${entry.info || ''}`.toLowerCase().includes(needle)
    return statusMatches && textMatches
  })
})

const shown = computed(() => filtered.value.slice(0, 50))
const limitedCount = editorSupportCatalog.summary.partial + editorSupportCatalog.summary.stub

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
        <p class="docs-card-kicker">Generated standard-library catalog</p>
        <h2 id="support-explorer-heading">Search checked API rows</h2>
      </div>
      <dl class="support-explorer-summary">
        <div><dt>Runs locally</dt><dd>{{ editorSupportCatalog.summary.supported }}</dd></div>
        <div><dt>Runs locally with limits</dt><dd>{{ limitedCount }}</dd></div>
        <div><dt>Requires Salesforce</dt><dd>{{ editorSupportCatalog.summary.unsupported }}</dd></div>
      </dl>
    </div>
    <div class="support-explorer-controls">
      <label>
        Search APIs
        <input v-model="query" type="search" placeholder="Try Database.insert or Answers" autocomplete="off">
      </label>
      <label>
        Status
        <select v-model="status">
          <option value="all">All statuses</option>
          <option value="supported">Runs locally</option>
          <option value="partial">Runs locally with limits</option>
          <option value="unsupported">Requires Salesforce</option>
        </select>
      </label>
    </div>
    <p class="support-explorer-result" role="status" aria-live="polite">
      {{ filtered.length }} checked row{{ filtered.length === 1 ? '' : 's' }} match.{{ filtered.length > shown.length ? ` Showing the first ${shown.length}.` : '' }}
    </p>
    <ul class="support-explorer-list" tabindex="0" aria-label="Matching checked API rows">
      <li v-for="entry in shown" :key="entry.id">
        <div>
          <code>{{ entry.receiver }}.{{ entry.signature || entry.label }}</code>
          <span :class="['docs-status-chip', statusClass(entry.status)]">{{ entry.statusLabel }}</span>
        </div>
        <p>{{ entry.info }}</p>
      </li>
    </ul>
    <p class="support-explorer-foot">Counts and rows come from <code>{{ editorSupportCatalog.generatedFrom }}</code>. Use the complete checked ledgers below for evidence and regression-test links.</p>
  </section>
</template>
