import { renderToString } from "react-dom/server"
import { expect, test, vi } from "vitest"

import App from "./App"

test("renders Database as a center workspace tab", () => {
  vi.stubGlobal("localStorage", {
    getItem: () => "dark",
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

  expect(html).toContain("Apex Source")
  expect(html).toContain("Execute Anonymous")
  expect(html).toContain("Database")
  expect(html).toContain('data-testid="workspace-database-tab"')
  expect(html).not.toContain("database-work-surface")
})

test("does not render database filters before the Database tab opens", () => {
  vi.stubGlobal("localStorage", {
    getItem: () => "dark",
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

  expect(html).not.toContain("System records")
  expect(html).not.toContain("Custom SObjects")
  expect(html).not.toContain("Custom metadata")
  expect(html).not.toContain("Custom settings")
})
