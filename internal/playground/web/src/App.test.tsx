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
