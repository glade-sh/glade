#!/usr/bin/env node
import { mkdir, readFile, rename, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const siteRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const manifest = JSON.parse(await readFile(path.join(siteRoot, 'release-manifest.json'), 'utf8'))
const output = path.join(siteRoot, '.vitepress', 'dist', 'site-build.json')
const sourceDate = process.env.SOURCE_DATE_EPOCH
const builtAt = sourceDate ? new Date(Number(sourceDate) * 1000).toISOString() : new Date().toISOString()
const siteBuild = {
  schemaVersion: 1,
  siteCommit: process.env.CF_PAGES_COMMIT_SHA || process.env.GITHUB_SHA || 'local-preview',
  releaseVersion: manifest.version,
  builtAt
}

await mkdir(path.dirname(output), { recursive: true })
const temporary = `${output}.tmp-${process.pid}`
await writeFile(temporary, `${JSON.stringify(siteBuild, null, 2)}\n`)
await rename(temporary, output)
