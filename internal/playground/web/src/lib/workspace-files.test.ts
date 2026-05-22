import { describe, expect, it } from "vitest"

import {
  buildWorkspaceTree,
  defaultOpenFolderPaths,
  filterWorkspaceFiles,
  sourceFileIconName,
  workspaceSidebarGroups,
} from "@/lib/workspace-files"

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

describe("defaultOpenFolderPaths", () => {
  it("opens only the first root folder level by default", () => {
    const files = [
      { path: "force-app/main/default/classes/AccountService.cls", kind: "class" },
      { path: "packages/billing/force-app/main/default/classes/InvoiceService.cls", kind: "class" },
      { path: "seed.json", kind: "data" },
    ]

    expect([...defaultOpenFolderPaths(files)].sort()).toEqual(["force-app", "packages"])
  })
})

describe("workspaceSidebarGroups", () => {
  it("shows only the source tree and omits metadata/data clutter", () => {
    const files = [
      { path: "force-app/main/default/classes/AccountService.cls", kind: "class" },
      { path: "force-app/main/default/triggers/AccountTrigger.trigger", kind: "trigger" },
      { path: "sfdx-project.json", kind: "metadata" },
      { path: "config/settings.json", kind: "metadata" },
      { path: "seed.json", kind: "data" },
    ]

    const groups = workspaceSidebarGroups(files, "")

    expect(groups.map((group) => group.label)).toEqual(["Classes"])
    expect(groups[0].files.map((file) => file.path)).toEqual([
      "force-app/main/default/classes/AccountService.cls",
      "force-app/main/default/triggers/AccountTrigger.trigger",
    ])
  })

  it("keeps the source group visible when search has no matches", () => {
    const groups = workspaceSidebarGroups([{ path: "force-app/main/default/classes/AccountService.cls", kind: "class" }], "Nope")

    expect(groups).toHaveLength(1)
    expect(groups[0].forceVisible).toBe(true)
  })
})

describe("sourceFileIconName", () => {
  it("distinguishes classes and triggers", () => {
    expect(sourceFileIconName({ path: "force-app/main/default/classes/AccountService.cls", kind: "class" })).toBe("class")
    expect(sourceFileIconName({ path: "force-app/main/default/triggers/AccountTrigger.trigger", kind: "trigger" })).toBe("trigger")
  })
})
