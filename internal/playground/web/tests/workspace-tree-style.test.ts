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
