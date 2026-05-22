import { describe, expect, it } from "vitest"

import { buildWorkspaceTree, filterWorkspaceFiles } from "@/lib/workspace-files"

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

describe("buildWorkspaceTree", () => {
  it("groups class files into nested folders by project-relative path", () => {
    const files = [
      { path: "force-app/main/default/classes/AccountService.cls", kind: "class" },
      { path: "packages/billing/force-app/main/default/classes/InvoiceService.cls", kind: "class" },
      { path: "packages/billing/force-app/main/default/classes/InvoiceServiceTest.cls", kind: "class" },
    ]

    const tree = buildWorkspaceTree(files)

    expect(tree.map((node) => node.name)).toEqual(["force-app", "packages"])
    const packages = tree.find((node) => node.name === "packages")
    const billing = packages?.children.find((node) => node.name === "billing")
    const classes = billing?.children
      .find((node) => node.name === "force-app")
      ?.children.find((node) => node.name === "main")
      ?.children.find((node) => node.name === "default")
      ?.children.find((node) => node.name === "classes")
    expect(classes?.children.map((node) => node.name)).toEqual(["InvoiceService.cls", "InvoiceServiceTest.cls"])
    expect(classes?.children[0].file?.path).toBe("packages/billing/force-app/main/default/classes/InvoiceService.cls")
  })

  it("keeps matching files visible inside their folder path", () => {
    const files = [
      { path: "force-app/main/default/classes/AccountService.cls", kind: "class" },
      { path: "packages/billing/force-app/main/default/classes/InvoiceService.cls", kind: "class" },
    ]

    const tree = buildWorkspaceTree(files, "Invoice")

    expect(tree.map((node) => node.name)).toEqual(["packages"])
  })
})
