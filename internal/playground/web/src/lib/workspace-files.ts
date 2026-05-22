export type WorkspaceFileLike = {
  path: string
  kind: string
}

export type WorkspaceTreeNode<T extends WorkspaceFileLike = WorkspaceFileLike> = {
  id: string
  name: string
  path: string
  kind: "folder" | "file"
  file?: T
  children: WorkspaceTreeNode<T>[]
}

export type WorkspaceSidebarGroup<T extends WorkspaceFileLike = WorkspaceFileLike> = {
  label: "Classes"
  files: T[]
  tree: WorkspaceTreeNode<T>[]
  forceVisible: boolean
}

export function filterWorkspaceFiles<T extends WorkspaceFileLike>(files: T[], search: string): T[] {
  const query = search.trim().toLowerCase()
  const sourceFiles = files.filter((file) => file.kind === "class" || file.kind === "trigger")
  if (!query) return sourceFiles
  return sourceFiles.filter((file) => file.path.toLowerCase().includes(query))
}

export function buildWorkspaceTree<T extends WorkspaceFileLike>(files: T[], search = ""): WorkspaceTreeNode<T>[] {
  const root: WorkspaceTreeNode<T>[] = []
  for (const file of filterWorkspaceFiles(files, search)) {
    const parts = file.path.split("/").filter(Boolean)
    let children = root
    let path = ""
    parts.forEach((part, index) => {
      path = path ? `${path}/${part}` : part
      const isFile = index === parts.length - 1
      let node = children.find((item) => item.name === part && item.kind === (isFile ? "file" : "folder"))
      if (!node) {
        node = {
          id: path,
          name: part,
          path,
          kind: isFile ? "file" : "folder",
          file: isFile ? file : undefined,
          children: [],
        }
        children.push(node)
        children.sort(compareTreeNodes)
      }
      children = node.children
    })
  }
  return root
}

export function defaultOpenFolderPaths<T extends WorkspaceFileLike>(files: T[]) {
  return new Set(buildWorkspaceTree(files).map((node) => node.path))
}

export function workspaceSidebarGroups<T extends WorkspaceFileLike>(files: T[], search: string): WorkspaceSidebarGroup<T>[] {
  const sourceCount = files.filter((file) => file.kind === "class" || file.kind === "trigger").length
  const sourceFiles = filterWorkspaceFiles(files, search)
  return [
    {
      label: "Classes",
      files: sourceFiles,
      tree: buildWorkspaceTree(files, search),
      forceVisible: sourceCount > 0 || search.trim() !== "",
    },
  ]
}

export function sourceFileIconName(file: WorkspaceFileLike) {
  return file.kind === "trigger" ? "trigger" : "class"
}

function compareTreeNodes<T extends WorkspaceFileLike>(left: WorkspaceTreeNode<T>, right: WorkspaceTreeNode<T>) {
  if (left.kind !== right.kind) return left.kind === "folder" ? -1 : 1
  return left.name.localeCompare(right.name)
}
