type ApplySavedContentInput = {
  path: string
  savedContent: string
  currentContent: string
  dirtyPaths: ReadonlySet<string>
}

type ApplySavedContentResult = {
  dirtyPaths: Set<string>
  shouldReplaceEditorContent: boolean
}

export function applySavedContent({
  path,
  savedContent,
  currentContent,
  dirtyPaths,
}: ApplySavedContentInput): ApplySavedContentResult {
  const nextDirtyPaths = new Set(dirtyPaths)
  const shouldReplaceEditorContent = currentContent === savedContent
  if (shouldReplaceEditorContent) {
    nextDirtyPaths.delete(path)
  } else {
    nextDirtyPaths.add(path)
  }
  return { dirtyPaths: nextDirtyPaths, shouldReplaceEditorContent }
}

type ShouldApplyRunResultInput = {
  startedAtEditSeq: number
  currentEditSeq: number
}

export function shouldApplyRunResult({ startedAtEditSeq, currentEditSeq }: ShouldApplyRunResultInput) {
  return startedAtEditSeq === currentEditSeq
}

type PathsToSaveBeforeRunInput = {
  dirtyPaths: ReadonlySet<string>
  sourcePath: string
}

export function pathsToSaveBeforeRun({ dirtyPaths, sourcePath }: PathsToSaveBeforeRunInput) {
  if (!sourcePath || !dirtyPaths.has(sourcePath)) return []
  return [sourcePath]
}
