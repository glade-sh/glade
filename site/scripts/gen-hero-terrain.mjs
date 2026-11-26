// Draws docs-src/public/home/hero-terrain.svg, the still first paint of the hero contour map.
// The height field here must stay identical to the one in .vitepress/theme/home/hero-terrain.ts
// at time zero, so the live canvas takes over from the image without the lines moving.
// Run: node scripts/gen-hero-terrain.mjs
import { writeFileSync } from 'node:fs'

const INTERVAL = 0.046, R = 0.8, STEP = 0.008, UNIT = 1000
const HILLS = [[0, 0, 0.30, 0.55], [0.35, -0.20, 0.22, 0.45], [-0.30, 0.25, 0.26, 0.40], [0.50, 0.30, 0.20, 0.50],
  [-0.55, -0.15, 0.24, 0.35], [0.15, 0.45, 0.18, 0.40], [-0.15, -0.40, 0.20, 0.30], [0.75, -0.05, 0.28, 0.35]]
function field(x, y) {
  let v = 0.10 * Math.sin(x * 2.1 + y * 1.3) + 0.18 * x
  HILLS.forEach(([hx, hy, r, a], i) => {
    const dx = x - hx - 0.05 * Math.sin(i * 1.7), dy = y - hy - 0.05 * Math.cos(i * 1.7)
    v += a * Math.exp(-(dx * dx + dy * dy) / (r * r))
  })
  return v / INTERVAL
}
const n = Math.round((2 * R) / STEP), at = (i) => -R + i * STEP
const grid = Array.from({ length: n + 1 }, (_, j) => Float64Array.from({ length: n + 1 }, (_, i) => field(at(i), at(j))))
let lo = Infinity, hi = -Infinity
for (const row of grid) for (const v of row) { lo = Math.min(lo, v); hi = Math.max(hi, v) }

function trace(level) {
  // Each crossing sits on one grid edge; segments that share an edge join into one line.
  const links = new Map(), point = new Map()
  const cross = (i, j, horizontal) => {
    const key = `${i},${j},${horizontal ? 'h' : 'v'}`
    if (!point.has(key)) {
      const a = grid[j][i], b = horizontal ? grid[j][i + 1] : grid[j + 1][i], t = (level - a) / (b - a)
      point.set(key, horizontal ? [at(i) + t * STEP, at(j)] : [at(i), at(j) + t * STEP])
    }
    return key
  }
  const join = (a, b) => { for (const [from, to] of [[a, b], [b, a]]) { if (!links.has(from)) links.set(from, []); links.get(from).push(to) } }
  for (let j = 0; j < n; j++) for (let i = 0; i < n; i++) {
    if (Math.hypot(at(i) + STEP / 2, at(j) + STEP / 2) > 0.77) continue
    const tl = grid[j][i] > level, tr = grid[j][i + 1] > level, br = grid[j + 1][i + 1] > level, bl = grid[j + 1][i] > level
    const edges = []
    if (tl !== tr) edges.push(cross(i, j, true))
    if (tr !== br) edges.push(cross(i + 1, j, false))
    if (bl !== br) edges.push(cross(i, j + 1, true))
    if (tl !== bl) edges.push(cross(i, j, false))
    if (edges.length === 2) join(edges[0], edges[1])
    else if (edges.length === 4) { if (tl) { join(edges[0], edges[1]); join(edges[2], edges[3]) } else { join(edges[0], edges[3]); join(edges[1], edges[2]) } }
  }
  const used = new Set(), paths = []
  const walk = (start) => {
    const line = [start]; used.add(start)
    for (let here = start; ;) {
      const next = (links.get(here) || []).find((k) => !used.has(k))
      if (!next) break
      used.add(next); line.push(next); here = next
    }
    return line
  }
  const starts = [...links.keys()].sort((a, b) => links.get(a).length - links.get(b).length)
  for (const start of starts) {
    if (used.has(start)) continue
    const line = walk(start), closed = links.get(start).length === 2 && links.get(line[line.length - 1]).includes(start) && line.length > 2
    // Thin the crossings to one every ~26 units, then round the corners: each kept point becomes a
    // quadratic control point and the curve passes through the midpoints between them.
    const pts = []
    line.forEach((key, index) => {
      const [x, y] = point.get(key).map((v) => v * UNIT), last = pts[pts.length - 1]
      if (!last || index === line.length - 1 || Math.hypot(x - last[0], y - last[1]) >= 26) pts.push([x, y])
    })
    if (pts.length < 3) continue
    const r = (v) => String(Math.round(v)), mid = (a, b) => [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2]
    let d
    if (closed) {
      const start = mid(pts[pts.length - 1], pts[0])
      d = `M${r(start[0])} ${r(start[1])}`
      pts.forEach((pt, i) => { const m = mid(pt, pts[(i + 1) % pts.length]); d += `Q${r(pt[0])} ${r(pt[1])} ${r(m[0])} ${r(m[1])}` })
      d += 'z'
    } else {
      d = `M${r(pts[0][0])} ${r(pts[0][1])}`
      for (let i = 1; i < pts.length - 1; i++) { const m = i === pts.length - 2 ? pts[i + 1] : mid(pts[i], pts[i + 1]); d += `Q${r(pts[i][0])} ${r(pts[i][1])} ${r(m[0])} ${r(m[1])}` }
    }
    paths.push(d.replace(/ -/g, '-'))
  }
  return paths.join('')
}
let thin = '', index = ''
for (let level = Math.ceil(lo); level <= Math.floor(hi); level++) { if (level % 4 === 0) index += trace(level); else thin += trace(level) }
const size = 2 * R * UNIT
const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${-size / 2} ${-size / 2} ${size} ${size}" fill="none" stroke="url(#g)" stroke-linejoin="round" stroke-linecap="round"><defs><radialGradient id="g" gradientUnits="userSpaceOnUse" cx="0" cy="0" r="${size / 2}"><stop offset="0" stop-color="#8feaf2"/><stop offset=".3" stop-color="#86c8fa"/><stop offset=".46" stop-color="#80a9ff" stop-opacity=".84"/><stop offset=".62" stop-color="#8b98ff" stop-opacity=".5"/><stop offset=".78" stop-color="#958bff" stop-opacity=".16"/><stop offset=".94" stop-color="#9985ff" stop-opacity="0"/></radialGradient></defs><path stroke-width="1.4" stroke-opacity=".3" d="${thin}"/><path stroke-width="2.1" stroke-opacity=".43" d="${index}"/></svg>\n`
writeFileSync(new URL('../docs-src/public/home/hero-terrain.svg', import.meta.url), svg)
console.log(svg.length, 'bytes')
