import { renderToString } from "react-dom/server"
import { expect, test, vi } from "vitest"

import App, {
  closeSourceTab,
  formatTimeout,
  limitUsagePreview,
  resultDefaultTab,
  selectSourceTab,
  sourceTabItems,
  type SourceTabFile,
} from "./App"

const files: SourceTabFile[] = [
  { path: "force-app/main/default/classes/AccountService.cls", kind: "class", readOnly: false },
  { path: "force-app/main/default/classes/InvoiceService.cls", kind: "class", readOnly: false },
  { path: "force-app/main/default/triggers/AccountTrigger.trigger", kind: "trigger", readOnly: true },
]

function stubLocalStorage(advanced = false) {
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => {
      if (key === "glade-playground-advanced") return advanced ? "true" : "false"
      if (key === "glade-playground-theme") return "dark"
      return null
    },
    setItem: () => undefined,
  })
}

test("renders source tabs above the source editor", () => {
  stubLocalStorage()

  const html = renderToString(<App />)

  expect(html).toContain('data-testid="source-tab-strip"')
  expect(html.indexOf('data-testid="source-tab-strip"')).toBeLessThan(html.indexOf("Apex Source"))
})

test("selecting a source tab changes the active source file", () => {
  const state = selectSourceTab(
    {
      sourcePath: "force-app/main/default/classes/AccountService.cls",
      activePath: "force-app/main/default/classes/AccountService.cls",
      sourceTabs: files.map((file) => file.path),
    },
    "force-app/main/default/classes/InvoiceService.cls",
  )

  expect(state.sourcePath).toBe("force-app/main/default/classes/InvoiceService.cls")
  expect(state.activePath).toBe("force-app/main/default/classes/InvoiceService.cls")
})

test("execute anonymous stays visible in advanced mode", () => {
  stubLocalStorage(true)

  const html = renderToString(<App />)

  expect(html).toContain("Apex Source")
  expect(html).toContain("Execute Anonymous")
  expect(html).not.toContain('data-testid="workspace-database-tab"')
})

test("dirty source tab shows a marker after edit", () => {
  const tabs = sourceTabItems({
    files,
    sourceTabs: files.map((file) => file.path),
    activePath: "force-app/main/default/classes/AccountService.cls",
    dirtyPaths: new Set(["force-app/main/default/classes/InvoiceService.cls"]),
  })

  expect(tabs.find((tab) => tab.path === "force-app/main/default/classes/InvoiceService.cls")?.dirty).toBe(true)
})

test("database browser does not replace anonymous editor", () => {
  stubLocalStorage(true)

  const html = renderToString(<App />)

  expect(html).toContain("Execute Anonymous")
  expect(html).toContain('data-testid="output-database-tab"')
  expect(html).not.toContain('data-testid="workspace-database-tab"')
})

test("persist mode selector is visible in local mode", () => {
  stubLocalStorage()

  const html = renderToString(<App />)

  expect(html).toContain('data-testid="run-mode-selector"')
  expect(html).toContain("scratch")
  expect(html).toContain("persist")
})

test("memory-only state appears when db path is empty", () => {
  stubLocalStorage()

  const html = renderToString(<App />)

  expect(html).toContain("memory-only")
})

test("does not close the last source tab", () => {
  const state = closeSourceTab(
    {
      sourcePath: "force-app/main/default/classes/AccountService.cls",
      activePath: "force-app/main/default/classes/AccountService.cls",
      sourceTabs: ["force-app/main/default/classes/AccountService.cls"],
    },
    "force-app/main/default/classes/AccountService.cls",
  )

  expect(state.sourceTabs).toEqual(["force-app/main/default/classes/AccountService.cls"])
  expect(state.sourcePath).toBe("force-app/main/default/classes/AccountService.cls")
})

test("hides Database as a center workspace tab until Advanced is on", () => {
  stubLocalStorage()

  const html = renderToString(<App />)

  expect(html).toContain("Apex Source")
  expect(html).toContain("Execute Anonymous")
  expect(html).not.toContain('data-testid="workspace-database-tab"')
  expect(html).not.toContain("database-work-surface")
})

test("renders Database as an output tab when Advanced is on", () => {
  stubLocalStorage(true)

  const html = renderToString(<App />)

  expect(html).toContain("Database")
  expect(html).toContain('data-testid="output-database-tab"')
  expect(html).not.toContain('data-testid="workspace-database-tab"')
  expect(html).not.toContain("database-work-surface")
})

test("does not render database filters before the Database output tab opens", () => {
  stubLocalStorage()

  const html = renderToString(<App />)

  expect(html).not.toContain("System records")
  expect(html).not.toContain("Custom SObjects")
  expect(html).not.toContain("Custom metadata")
  expect(html).not.toContain("Custom settings")
})

test("links the playground docs action to the guide", () => {
  stubLocalStorage()

  const html = renderToString(<App />)

	expect(html).toContain('href="https://glade.sh/guide/"')
})

test("renders launch surfaces and state controls", () => {
  stubLocalStorage()

  const html = renderToString(<App />)

  expect(html).toContain("Apex")
  expect(html).toContain("Visualforce")
  expect(html).toContain("LWC")
  expect(html).toContain("Share")
  expect(html).toContain("Reset data")
  expect(html).toContain('data-testid="playground-policy"')
})

test("shows an enforced zero governor cap", () => {
  expect(limitUsagePreview("callouts", 0, { callouts: 0 })).toBe("0 / 0")
})

test("formats configured timeouts without overstating them", () => {
  expect(formatTimeout(500)).toBe("500ms")
  expect(formatTimeout(1500)).toBe("1.5s")
  expect(formatTimeout(5000)).toBe("5s")
})

test("opens failed runs on the actionable problems tab", () => {
  expect(
    resultDefaultTab({
      runId: "run-1",
      cacheHit: false,
      status: "runtime_error",
      compileMs: 0,
      executeMs: 1,
      logs: ["before failure"],
    }),
  ).toBe("problems")
})
