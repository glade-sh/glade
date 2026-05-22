import { describe, expect, it } from "vitest"

import { filterWorkspaceFiles } from "@/lib/workspace-files"

describe("filterWorkspaceFiles", () => {
  it("filters class and trigger files by full path", () => {
    const files = [
      { path: "force-app/main/default/classes/AccountService.cls", kind: "class" },
      { path: "packages/billing/force-app/main/default/classes/InvoiceService.cls", kind: "class" },
      { path: "force-app/main/default/triggers/AccountTrigger.trigger", kind: "trigger" },
      { path: "seed.json", kind: "data" },
    ]

    const filtered = filterWorkspaceFiles(files, "packages/billing")

    expect(filtered.map((file) => file.path)).toEqual([
      "packages/billing/force-app/main/default/classes/InvoiceService.cls",
    ])
  })

  it("keeps all class files when the search text is blank", () => {
    const files = [
      { path: "force-app/main/default/classes/AccountService.cls", kind: "class" },
      { path: "force-app/main/default/triggers/AccountTrigger.trigger", kind: "trigger" },
    ]

    expect(filterWorkspaceFiles(files, "")).toHaveLength(2)
  })
})
