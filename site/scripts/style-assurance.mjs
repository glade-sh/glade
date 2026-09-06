import { readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const marker = '<!-- glade-shared-styles -->'

export function styleAssuranceHTML(html, index) {
  const links = [...index.matchAll(/<link\b[^>]*>/g)].map(match => match[0])
  const styles = links.filter(link => /\brel="[^"]*\bstylesheet\b[^"]*"/.test(link))
    .map(link => link.match(/\bhref="([^"]+)"/)?.[1])
    .filter(href => /^\/assets\/style\.[\w-]+\.css$/.test(href || ''))
  if (styles.length !== 1) throw new Error('Expected exactly one local main stylesheet in the built homepage')
  if (html.split(marker).length !== 2) throw new Error('Expected one assurance shared stylesheet marker')
  const cleaned = html.replace(/\s*<link\b[^>]*\bdata-glade-shared-style[^>]*>/g, '')
  return { html: cleaned.replace(marker, `${marker}\n  <link rel="stylesheet" href="${styles[0]}" data-glade-shared-style>`), href: styles[0] }
}

export async function styleAssuranceSite(dist) {
  const path = resolve(dist, 'private-corpus-assurance.html')
  const [html, index] = await Promise.all([readFile(path, 'utf8'), readFile(resolve(dist, 'index.html'), 'utf8')])
  const styled = styleAssuranceHTML(html, index)
  const css = await readFile(resolve(dist, styled.href.slice(1)), 'utf8')
  if (!css.includes('--glade-canvas') || !css.includes('Inter Variable')) throw new Error('Built stylesheet is missing shared Glade tokens or Inter font')
  await writeFile(path, styled.html)
  return styled.href
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const href = await styleAssuranceSite(fileURLToPath(new URL('../.vitepress/dist/', import.meta.url)))
  console.log(`Assurance shell uses shared site stylesheet: ${href}`)
}
