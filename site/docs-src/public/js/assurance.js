const data = JSON.parse(document.getElementById('assurance-data').textContent)
const rows = data.rows || []
const repositoryRows = data.repositorySurfaceRows || []
const summaries = data.repositorySummaries || []
const ids = ['namespace', 'disposition', 'repository', 'evidence', 'exclusion']
const controls = Object.fromEntries(ids.map(id => [id, document.getElementById(id)]))
const search = document.getElementById('text')
const clear = document.getElementById('clear-filters')
const theme = document.getElementById('theme-toggle')
const systemTheme = matchMedia('(prefers-color-scheme: dark)')

function syncThemeButton() {
  const dark = document.documentElement.classList.contains('dark')
  theme.textContent = dark ? 'Light mode' : 'Dark mode'
  theme.setAttribute('aria-label', `Switch to ${dark ? 'light' : 'dark'} appearance`)
}
theme.addEventListener('click', () => {
  const dark = document.documentElement.classList.toggle('dark')
  try { localStorage.setItem('vitepress-theme-appearance', dark ? 'dark' : 'light') } catch {}
  syncThemeButton()
})
systemTheme.addEventListener('change', () => {
  let preference
  try { preference = localStorage.getItem('vitepress-theme-appearance') } catch {}
  if (!preference || preference === 'auto') document.documentElement.classList.toggle('dark', systemTheme.matches)
  syncThemeButton()
})
syncThemeButton()

function values(select) { return [...new Set(rows.flatMap(select).filter(Boolean))].sort() }
function options(id, values) {
  for (const value of values) controls[id].add(new Option(value, value))
}
options('namespace', values(row => [row.namespace]))
options('disposition', values(row => [row.disposition]))
options('repository', values(row => row.repositoryIds || []))
options('evidence', values(row => [row.localEvidence, row.salesforceEvidence]))
options('exclusion', values(row => [row.exclusionClass]))

function readiness(row) { return row.runtimeParityReady ? 'runtime-parity-ready' : row.testReady ? 'test-ready' : row.compileReady ? 'compile-ready' : 'not-ready' }
function nonparity(row) { return row.nonParity ? (row.exclusionReason || row.nonParityReason || 'non-parity') : '' }
function render(id, entries, cells, columns) {
  const body = document.getElementById(id)
  const fragment = document.createDocumentFragment()
  for (const entry of entries) {
    const row = document.createElement('tr')
    for (const value of cells(entry)) {
      const cell = document.createElement('td')
      cell.textContent = String(value ?? '')
      row.append(cell)
    }
    fragment.append(row)
  }
  if (!entries.length) {
    const row = document.createElement('tr')
    const cell = document.createElement('td')
    cell.colSpan = columns
    cell.className = 'assurance-empty'
    cell.textContent = 'No sealed rows match these filters.'
    row.append(cell)
    fragment.append(row)
  }
  body.replaceChildren(fragment)
}
function draw() {
  const query = search.value.toLowerCase()
  const matches = row => (!controls.namespace.value || row.namespace === controls.namespace.value)
    && (!controls.disposition.value || row.disposition === controls.disposition.value)
    && (!controls.repository.value || (row.repositoryIds || [row.repositoryId]).includes(controls.repository.value))
    && (!controls.evidence.value || row.localEvidence === controls.evidence.value || row.salesforceEvidence === controls.evidence.value)
    && (!controls.exclusion.value || row.exclusionClass === controls.exclusion.value)
    && JSON.stringify(row).toLowerCase().includes(query)
  const selectedRows = rows.filter(matches)
  const selectedRepositoryRows = repositoryRows.filter(matches)
  const selectedSummaries = summaries.filter(row => !controls.repository.value || row.repositoryId === controls.repository.value)
  render('rows', selectedRows, row => [row.surfaceId, (row.repositoryIds || []).join(', '), row.disposition, row.localEvidence, row.salesforceEvidence, readiness(row), nonparity(row)], 7)
  render('repository-rows', selectedRepositoryRows, row => [row.repositoryId, row.surfaceId, row.compileReady, row.testReady, row.runtimeParityReady, readiness(row), nonparity(row)], 7)
  render('repository-summaries', selectedSummaries, row => [row.repositoryId, row.surfaceCount, row.compileReady, row.testReady, row.runtimeParityReady, row.nonParity ? row.nonParityReason : ''], 6)
  const filtered = Boolean(query || Object.values(controls).some(control => control.value))
  clear.disabled = !filtered
  document.getElementById('result-summary').textContent = !filtered
    ? 'All sealed surface rows are shown.'
    : selectedRows.length
      ? `${selectedRows.length} matching sealed surface rows · ${selectedRepositoryRows.length} repository × surface rows.`
      : 'No sealed rows match. Change or clear the filters to explore the snapshot.'
}
Object.values(controls).forEach(control => control.addEventListener('input', draw))
search.addEventListener('input', draw)
document.querySelector('.assurance-filters').addEventListener('submit', event => event.preventDefault())
document.querySelector('.assurance-filters').addEventListener('reset', event => {
  event.preventDefault()
  Object.values(controls).forEach(control => { control.value = '' })
  search.value = ''
  draw()
  search.focus()
})
draw()
