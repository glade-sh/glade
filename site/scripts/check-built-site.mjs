import { readdir, readFile, stat } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const siteRoot = path.resolve(scriptDir, '..')
const distRoot = path.join(siteRoot, '.vitepress', 'dist')
const { routes } = JSON.parse(await readFile(path.join(siteRoot, 'routes.json'), 'utf8'))
const failures = []
const indexableTitles = new Map()
const indexableDescriptions = new Map()
const fragmentIDs = new Map()
const siteBuildPath = path.join(distRoot, 'site-build.json')

async function exists(file) {
  try { return (await stat(file)).isFile() } catch { return false }
}

function outputPath(route) {
  if (route === '/') return path.join(distRoot, 'index.html')
  const relative = route.replace(/^\//, '')
  return route.endsWith('/')
    ? path.join(distRoot, relative, 'index.html')
    : path.join(distRoot, `${relative}.html`)
}

async function htmlFiles(dir = distRoot) {
  const entries = await readdir(dir, { withFileTypes: true })
  const all = await Promise.all(entries.map(async (entry) => {
    const file = path.join(dir, entry.name)
    if (entry.isDirectory()) return htmlFiles(file)
    return entry.isFile() && entry.name.endsWith('.html') ? [file] : []
  }))
  return all.flat()
}

function localReference(value) {
  if (!value || /^(?:https?:|mailto:|tel:|data:|javascript:)/.test(value)) return null
  const hashIndex = value.indexOf('#')
  const target = (hashIndex === -1 ? value : value.slice(0, hashIndex)).split('?', 1)[0]
  const encodedFragment = hashIndex === -1 ? '' : value.slice(hashIndex + 1)
  let fragment = ''
  try {
    fragment = decodeURIComponent(encodedFragment)
  } catch {
    fragment = encodedFragment
  }
  return { target, fragment }
}

async function firstExistingFile(candidates) {
  for (const candidate of candidates) {
    if (await exists(candidate)) return candidate
  }
  return null
}

async function idsFor(file) {
  if (fragmentIDs.has(file)) return fragmentIDs.get(file)
  const html = await readFile(file, 'utf8')
  const ids = new Set([...html.matchAll(/\sid=["']([^"']+)["']/g)].map((match) => match[1]))
  fragmentIDs.set(file, ids)
  return ids
}

function titleText(html) {
  return html.match(/<title>([^<]*)<\/title>/)?.[1] || ''
}

function metaContent(html, attribute, value) {
  const escaped = value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return html.match(new RegExp(`<meta\\b[^>]*\\b${attribute}="${escaped}"[^>]*\\bcontent="([^"]*)"[^>]*>`))?.[1] || ''
}

for (const entry of routes) {
  const file = outputPath(entry.route)
  if (entry.classification === 'redirect') {
    if (await exists(file)) failures.push(`${entry.route}: redirect source was emitted as a searchable page`)
    continue
  }
  if (!(await exists(file))) {
    failures.push(`missing built route ${entry.route}`)
    continue
  }
  const html = await readFile(file, 'utf8')
  const canonical = `https://glade.sh${entry.route}`
  const canonicalMatches = [...html.matchAll(/<link\b[^>]*\brel="canonical"[^>]*\bhref="([^"]+)"[^>]*>/g)]
  if (canonicalMatches.length !== 1 || canonicalMatches[0][1] !== canonical) {
    failures.push(`${entry.route}: expected one self-referencing canonical (${canonical})`)
  }
  for (const pattern of [
    /<meta\b[^>]*\bname="description"[^>]*\bcontent="[^"]{24,}"[^>]*>/,
    /<meta\b[^>]*\bproperty="og:title"[^>]*\bcontent="[^"]+"[^>]*>/,
    /<meta\b[^>]*\bproperty="og:description"[^>]*\bcontent="[^"]{24,}"[^>]*>/,
    /<meta\b[^>]*\bproperty="og:url"[^>]*\bcontent="[^"]+"[^>]*>/,
    /<meta\b[^>]*\bname="twitter:title"[^>]*\bcontent="[^"]+"[^>]*>/,
    /<meta\b[^>]*\bname="twitter:description"[^>]*\bcontent="[^"]{24,}"[^>]*>/
  ]) {
    if (!pattern.test(html)) failures.push(`${entry.route}: missing route-specific metadata`)
  }
  const shouldNoindex = entry.classification === 'noindex'
  const hasNoindex = /<meta\b[^>]*\bname="robots"[^>]*\bcontent="noindex"[^>]*>/.test(html)
  if (shouldNoindex !== hasNoindex) failures.push(`${entry.route}: incorrect robots indexing state`)
  const pageTitle = titleText(html)
  const description = metaContent(html, 'name', 'description')
  const ogTitle = metaContent(html, 'property', 'og:title')
  const ogDescription = metaContent(html, 'property', 'og:description')
  const ogURL = metaContent(html, 'property', 'og:url')
  const twitterTitle = metaContent(html, 'name', 'twitter:title')
  const twitterDescription = metaContent(html, 'name', 'twitter:description')
  if (entry.route === '/' && pageTitle !== 'Glade') failures.push('/: homepage title must be exactly Glade')
  if (ogTitle !== pageTitle || twitterTitle !== pageTitle) failures.push(`${entry.route}: social titles must equal the page title`)
  if (ogDescription !== description || twitterDescription !== description) failures.push(`${entry.route}: social descriptions must equal the page description`)
  if (ogURL !== canonical) failures.push(`${entry.route}: social URL must equal the canonical`)
  if (/^Learn how .* works in Glade,/.test(description)) failures.push(`${entry.route}: description uses the generic fallback`)
  if (!shouldNoindex) {
    if (indexableTitles.has(pageTitle)) failures.push(`${entry.route}: duplicate indexable title shared with ${indexableTitles.get(pageTitle)}`)
    if (indexableDescriptions.has(description)) failures.push(`${entry.route}: duplicate indexable description shared with ${indexableDescriptions.get(description)}`)
    indexableTitles.set(pageTitle, entry.route)
    indexableDescriptions.set(description, entry.route)
  }
}
if (await exists(path.join(distRoot, 'public', 'help', 'screenshots', 'README', 'index.html'))) {
  failures.push('internal screenshot instructions were emitted as a public page')
}

const pages = await htmlFiles()
for (const file of pages) {
  const html = await readFile(file, 'utf8')
  const relative = path.relative(distRoot, file)
  const titleCount = (html.match(/<title>/g) || []).length
  const h1Count = (html.match(/<h1(?:\s|>)/g) || []).length
  if (titleCount !== 1) failures.push(`${relative}: expected one title, found ${titleCount}`)
  if (h1Count !== 1 && !relative.endsWith('404.html')) failures.push(`${relative}: expected one H1, found ${h1Count}`)

  const ids = [...html.matchAll(/\sid=["']([^"']+)["']/g)].map((match) => match[1])
  if (new Set(ids).size !== ids.length) failures.push(`${relative}: duplicate id`)
  fragmentIDs.set(file, new Set(ids))

  if (/&lt;[a-z0-9-]+ [a-z0-9-]+&gt;/i.test(html)) {
    failures.push(`${relative}: malformed angle-bracket placeholder`)
  }

  const attributes = [...html.matchAll(/(?:href|src)=["']([^"']+)["']/g)].map((match) => match[1])
  for (const value of attributes) {
    const reference = localReference(value)
    if (!reference) continue
    const resolved = !reference.target
      ? file
      : reference.target === '/'
        ? path.join(distRoot, 'index.html')
        : reference.target.startsWith('/')
          ? path.join(distRoot, reference.target)
          : path.resolve(path.dirname(file), reference.target)
    const candidates = reference.target
      ? [resolved, `${resolved}.html`, path.join(resolved, 'index.html')]
      : [file]
    const targetFile = await firstExistingFile(candidates)
    if (!targetFile) {
      failures.push(`${relative}: missing local target ${value}`)
      continue
    }
    if (reference.fragment && !(await idsFor(targetFile)).has(reference.fragment)) {
      failures.push(`${relative}: missing local fragment ${value}`)
    }
  }
}

for (const required of ['_headers', '_redirects', 'robots.txt', 'sitemap.xml', 'install.sh', 'site-build.json']) {
  if (!(await exists(path.join(distRoot, required)))) failures.push(`missing built public artifact /${required}`)
}

if (await exists(siteBuildPath)) {
  const siteBuild = JSON.parse(await readFile(siteBuildPath, 'utf8'))
  const releaseManifest = JSON.parse(await readFile(path.join(siteRoot, 'release-manifest.json'), 'utf8'))
  if (siteBuild.schemaVersion !== 1) failures.push('site-build.json: unsupported schema')
  if (siteBuild.releaseVersion !== releaseManifest.version) failures.push('site-build.json: release version differs from checked manifest')
  if (!siteBuild.siteCommit || !siteBuild.builtAt) failures.push('site-build.json: missing build identity')
}

const sitemap = await readFile(path.join(distRoot, 'sitemap.xml'), 'utf8').catch(() => '')
for (const entry of routes) {
  const listed = sitemap.includes(`<loc>https://glade.sh${entry.route}</loc>`)
  const shouldList = entry.classification === 'nav' || entry.classification === 'deep-link'
  if (listed !== shouldList) failures.push(`${entry.route}: incorrect sitemap inclusion`)
}

const headers = await readFile(path.join(distRoot, '_headers'), 'utf8').catch(() => '')
const headerRules = []
let currentRule
for (const line of headers.split('\n')) {
  if (line.startsWith('/')) {
    currentRule = { path: line.trim(), headers: new Map() }
    headerRules.push(currentRule)
    continue
  }
  const match = line.match(/^\s+([^:]+):\s*(.+)$/)
  if (match && currentRule) currentRule.headers.set(match[1].toLowerCase(), match[2])
}
const assetRule = headerRules.find((rule) => rule.path === '/assets/*')
const globalRule = headerRules.find((rule) => rule.path === '/*')
if (!assetRule || !/max-age=31536000, immutable/.test(assetRule.headers.get('cache-control') || '')) {
  failures.push('_headers: assets are not immutable')
}
if (/must-revalidate/.test(assetRule?.headers.get('cache-control') || '')) failures.push('_headers: assets also revalidate')
if (!globalRule || globalRule.headers.has('cache-control')) failures.push('_headers: global rule must not set a cache policy')
if (globalRule?.headers.get('x-frame-options') !== 'DENY' || !globalRule?.headers.has('permissions-policy')) {
  failures.push('_headers: global security headers are missing')
}
for (const routePrefix of ['/', '/guide/*', '/reference/*', '/help/*', '/maintainer/*', '/private-corpus-assurance.html', '/install.sh']) {
  const rule = headerRules.find((candidate) => candidate.path === routePrefix)
  if (!/max-age=0, must-revalidate/.test(rule?.headers.get('cache-control') || '')) {
    failures.push(`_headers: ${routePrefix} does not revalidate`)
  }
}

if (failures.length > 0) {
  console.error(`Built-site check failed:\n${[...new Set(failures)].map((failure) => `- ${failure}`).join('\n')}`)
  process.exit(1)
}

console.log(`Built-site check passed for ${pages.length} HTML pages and ${routes.length} classified routes.`)
