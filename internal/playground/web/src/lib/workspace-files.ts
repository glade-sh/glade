export type WorkspaceFileLike = {
  path: string
  kind: string
}

export function filterWorkspaceFiles<T extends WorkspaceFileLike>(files: T[], search: string): T[] {
  const query = search.trim().toLowerCase()
  const sourceFiles = files.filter((file) => file.kind === "class" || file.kind === "trigger")
  if (!query) return sourceFiles
  return sourceFiles.filter((file) => file.path.toLowerCase().includes(query))
}
