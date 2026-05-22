import { describe, expect, it } from "vitest"

import { applySavedContent, shouldApplyRunResult } from "@/lib/save-state"

describe("applySavedContent", () => {
  it("keeps a file dirty when a newer edit exists after a save started", () => {
    const next = applySavedContent({
      path: "force-app/main/default/classes/AccountPlayground.cls",
      savedContent: "public class AccountPlayground { String oldValue; }",
      currentContent: "public class AccountPlayground { String newValue; }",
      dirtyPaths: new Set(["force-app/main/default/classes/AccountPlayground.cls"]),
    })

    expect(next.dirtyPaths.has("force-app/main/default/classes/AccountPlayground.cls")).toBe(true)
    expect(next.shouldReplaceEditorContent).toBe(false)
  })

  it("clears a file dirty mark when the saved content is still current", () => {
    const next = applySavedContent({
      path: "anonymous.apex",
      savedContent: "System.debug('current');",
      currentContent: "System.debug('current');",
      dirtyPaths: new Set(["anonymous.apex"]),
    })

    expect(next.dirtyPaths.has("anonymous.apex")).toBe(false)
    expect(next.shouldReplaceEditorContent).toBe(true)
  })
})

describe("shouldApplyRunResult", () => {
  it("rejects a run result when edits happened after the run started", () => {
    expect(shouldApplyRunResult({ startedAtEditSeq: 4, currentEditSeq: 5 })).toBe(false)
  })

  it("accepts a run result when the editor has not changed since the run started", () => {
    expect(shouldApplyRunResult({ startedAtEditSeq: 4, currentEditSeq: 4 })).toBe(true)
  })
})
