import { renderToString } from "react-dom/server"
import { expect, test, vi } from "vitest"

import App from "./App"

test("renders Apex editors and Database as separate work surfaces", () => {
  vi.stubGlobal("localStorage", {
    getItem: () => "dark",
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

  expect(html).toContain("Apex Source")
  expect(html).toContain("Execute Anonymous")
  expect(html).toContain("Database")
  expect(html).toContain("database-work-surface")
})

test("renders database object filter controls", () => {
  vi.stubGlobal("localStorage", {
    getItem: () => "dark",
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

  expect(html).toContain("System records")
  expect(html).toContain("Custom SObjects")
  expect(html).toContain("Custom metadata")
  expect(html).toContain("Custom settings")
})
