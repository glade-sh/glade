import { renderToString } from "react-dom/server"
import { expect, test, vi } from "vitest"

import App from "./App"

test("renders Apex and Database as primary workspace tabs", () => {
  vi.stubGlobal("localStorage", {
    getItem: () => "dark",
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

  expect(html).toContain("Apex")
  expect(html).toContain("Database")
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
