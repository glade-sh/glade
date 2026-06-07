import { renderToString } from "react-dom/server"
import { expect, test, vi } from "vitest"

import App from "./App"

test("hides Database as a center workspace tab until Advanced is on", () => {
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => (key === "glade-playground-theme" ? "dark" : "false"),
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

  expect(html).toContain("Apex Source")
  expect(html).toContain("Execute Anonymous")
  expect(html).not.toContain('data-testid="workspace-database-tab"')
  expect(html).not.toContain("database-work-surface")
})

test("renders Database as a center workspace tab when Advanced is on", () => {
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => (key === "glade-playground-advanced" ? "true" : "dark"),
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

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

test("links the playground docs action to the docs reference area", () => {
  vi.stubGlobal("localStorage", {
    getItem: () => "dark",
    setItem: () => undefined,
  })

  const html = renderToString(<App />)

  expect(html).toContain('href="https://glade.sh/docs/"')
})
