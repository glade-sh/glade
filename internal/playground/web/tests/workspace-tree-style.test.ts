import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { expect, test } from "vitest"

const cssPath = fileURLToPath(new URL("../src/index.css", import.meta.url))

test("workspace tree owns two-axis scrolling without widening search", () => {
  const css = readFileSync(cssPath, "utf8")

  expect(css).toMatch(/\.workspace-tree-scroll\s*{[^}]*overflow:\s*auto/s)
  expect(css).toMatch(/\.workspace-tree-content\s*{[^}]*width:\s*max-content/s)
  expect(css).toMatch(/\.workspace-tree-search\s*{[^}]*width:\s*100%/s)
})

test("themeable shell surfaces use CSS variables instead of fixed dark fills", () => {
  const css = readFileSync(cssPath, "utf8")

  expect(css).toMatch(/\.topbar\s*{[^}]*background:\s*var\(--pane\)/s)
  expect(css).toMatch(/:root\s*{[^}]*--scanline:\s*rgba\(7,\s*16,\s*21,\s*0\.045\)/s)
  expect(css).toMatch(/\.dark\s*{[^}]*--scanline:\s*rgba\(255,\s*255,\s*255,\s*0\.016\)/s)
  expect(css).toMatch(/\.playground-shell::before\s*{[^}]*var\(--scanline\)/s)
  expect(css).toMatch(/\.command-backdrop\s*{[^}]*background:\s*rgba\(0,\s*0,\s*0,\s*0\.58\)/s)
  expect(css).toMatch(/:root\s*{[^}]*--selection-background:\s*rgba\(7,\s*136,\s*165,\s*0\.16\)/s)
  expect(css).toMatch(/\.dark\s*{[^}]*--selection-background:\s*rgba\(55,\s*217,\s*255,\s*0\.1\)/s)
})
