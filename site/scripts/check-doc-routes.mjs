import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const siteRoot = path.resolve(scriptDir, '..')
const docsRoot = path.join(siteRoot, 'docs-src')
const configPath = path.join(siteRoot, '.vitepress', 'config.ts')
const routeManifestPath = path.join(siteRoot, 'routes.json')
const classifications = new Set(['nav', 'deep-link', 'redirect', 'noindex'])

async function markdownSources(dir = docsRoot, prefix = '') {
  const entries = await readdir(dir, { withFileTypes: true })
  const nested = await Promise.all(entries.map(async (entry) => {
    const relative = path.posix.join(prefix, entry.name)
    if (entry.isDirectory()) return markdownSources(path.join(dir, entry.name), relative)
    return entry.isFile() && entry.name.endsWith('.md') ? [relative] : []
  }))
  return nested.flat().sort()
}

function configuredRoutes(config) {
  const routes = new Set()
  const pattern = /\blink:\s*['"]([^'"]+)['"]/g
  for (const match of config.matchAll(pattern)) {
    const route = match[1]
    if (route.startsWith('/') && !route.startsWith('//')) routes.add(route.replace(/\/$/, '') || '/')
  }
  return routes
}

const [{ routes }, sources, config] = await Promise.all([
  readFile(routeManifestPath, 'utf8').then(JSON.parse),
  markdownSources(),
  readFile(configPath, 'utf8')
])

const problems = []
const bySource = new Map()
const byRoute = new Map()
for (const entry of routes) {
  if (!entry || typeof entry.route !== 'string') {
    problems.push(`invalid route entry: ${JSON.stringify(entry)}`)
    continue
  }
  const label = entry.source || entry.route
  if (!classifications.has(entry.classification)) problems.push(`${label}: invalid classification ${entry.classification}`)
  if (entry.classification === 'redirect') {
    if ('source' in entry) problems.push(`${entry.route}: redirect entries must not retain a Markdown source`)
    if (typeof entry.destination !== 'string' || !entry.destination.startsWith('/')) problems.push(`${entry.route}: redirect destination is invalid`)
  } else if (typeof entry.source !== 'string') {
    problems.push(`${entry.route}: published route is missing a Markdown source`)
  }
  if (entry.source && bySource.has(entry.source)) problems.push(`${entry.source}: duplicate source route entry`)
  if (byRoute.has(entry.route)) problems.push(`${entry.route}: duplicate public route entry`)
  if (entry.source) bySource.set(entry.source, entry)
  byRoute.set(entry.route.replace(/\/$/, '') || '/', entry)
}

for (const source of sources) {
  if (!bySource.has(source)) problems.push(`${source}: Markdown source has no public-route classification`)
}
for (const source of bySource.keys()) {
  if (!sources.includes(source)) problems.push(`${source}: route manifest source does not exist`)
}
for (const route of configuredRoutes(config)) {
  if (!byRoute.has(route)) problems.push(`${route}: configured internal link is not classified in routes.json`)
}

if (problems.length > 0) {
  console.error(`Route contract failed:\n${problems.map((problem) => `- ${problem}`).join('\n')}`)
  process.exit(1)
}

console.log(`Route contract passed for ${sources.length} Markdown sources and ${routes.length} classified routes.`)
