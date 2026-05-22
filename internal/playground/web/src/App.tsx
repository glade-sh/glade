import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
  BookOpen,
  Braces,
  Boxes,
  CheckCircle2,
  CircleAlert,
  CircleDashed,
  Command,
  Database,
  FileCode2,
  FolderTree,
  Moon,
  Play,
  Plus,
  RefreshCcw,
  RotateCcw,
  Save,
  Search,
  Sun,
  Trash2,
  Zap,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { CodeEditor } from "@/components/CodeEditor"
import { cn } from "@/lib/utils"
import { applySavedContent, shouldApplyRunResult } from "@/lib/save-state"
import { filterWorkspaceFiles } from "@/lib/workspace-files"

const API_BASE = "/playground/api/"

type WorkspaceFile = {
  path: string
  kind: "class" | "trigger" | "anonymous" | "data" | "metadata" | "other"
  version: number
  size: number
}

type WorkspaceMetadata = {
  id: string
  root: string
  projectRoot: string
  exampleId?: string
  files: WorkspaceFile[]
  anonymousBody?: string
  workspaceHash?: string
  limitMode?: string
}

type ExampleProject = {
  id: string
  name: string
  description: string
  tags?: string[]
  fileCount: number
  source?: "builtin" | "local"
  path?: string
}

type FileSaveResponse = {
  file: WorkspaceFile
  workspaceHash: string
}

type Diagnostic = {
  severity: string
  message: string
  line?: number
  column?: number
}

type VarResult = {
  name: string
  type?: string
  value: unknown
}

type OrgDiff = {
  object: string
  inserted: number
  updated: number
  deleted: number
  insertedIds?: string[]
}

type RunResult = {
  runId: string
  cacheHit: boolean
  status: string
  compileMs: number
  executeMs: number
  diagnostics?: Diagnostic[]
  logs?: string[]
  vars?: VarResult[]
  limits?: Record<string, number>
  trace?: unknown[]
  orgDiff?: OrgDiff[]
  errorMessage?: string
  startedAt?: string
}

async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(API_BASE + path, {
    ...init,
    headers: {
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...init.headers,
    },
  })
  const text = await response.text()
  let body: unknown = {}
  try {
    body = text ? JSON.parse(text) : {}
  } catch {
    body = { error: text }
  }
  if (!response.ok) {
    const message = typeof body === "object" && body && "error" in body ? String(body.error) : response.statusText
    throw new Error(message)
  }
  return body as T
}

async function readWorkspaceFile(path: string) {
  const response = await fetch("/" + path)
  if (!response.ok) {
    throw new Error(`could not read ${path}`)
  }
  return response.text()
}

function fileName(path: string) {
  return path.split("/").pop() ?? path
}

function shortHash(hash?: string) {
  if (!hash) return "-"
  return hash.length > 18 ? `${hash.slice(0, 16)}...` : hash
}

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`
  return `${(size / 1024).toFixed(1)} KB`
}

function valuePreview(value: unknown) {
  if (value == null) return ""
  if (typeof value === "string") return value
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function statusVariant(status: string): "success" | "warning" | "danger" | "outline" {
  if (status === "Pass" || status === "pass") return "success"
  if (status === "Running" || status === "Loading" || status === "Saving" || status === "Deleting") return "warning"
  if (status === "Error" || status.includes("error")) return "danger"
  return "outline"
}

export default function App() {
  const [theme, setTheme] = useState<"dark" | "light">(() => {
    const saved = localStorage.getItem("oaer-playground-theme")
    return saved === "light" ? "light" : "dark"
  })
  const [meta, setMeta] = useState<WorkspaceMetadata | null>(null)
  const [versions, setVersions] = useState<Record<string, number>>({})
  const [contentByPath, setContentByPath] = useState<Record<string, string>>({})
  const [sourcePath, setSourcePath] = useState<string>("")
  const [activePath, setActivePath] = useState<string>("")
  const [anonymousPath, setAnonymousPath] = useState<string>("anonymous.apex")
  const [anonymous, setAnonymous] = useState("")
  const [mode, setMode] = useState<"scratch" | "persist">("scratch")
  const [limitMode, setLimitMode] = useState("permissive")
  const [status, setStatus] = useState("Loading")
  const [cacheState, setCacheState] = useState<"stale" | "fresh" | "hit">("stale")
  const [result, setResult] = useState<RunResult | null>(null)
  const [problemMessage, setProblemMessage] = useState("")
  const [running, setRunning] = useState(false)
  const [dirtyPaths, setDirtyPaths] = useState<Set<string>>(new Set())
  const [commandOpen, setCommandOpen] = useState(false)
  const [examples, setExamples] = useState<ExampleProject[]>([])
  const [selectedExample, setSelectedExample] = useState("")
  const [canLoadExamples, setCanLoadExamples] = useState(true)
  const [classSearch, setClassSearch] = useState("")

  const metaRef = useRef<WorkspaceMetadata | null>(null)
  const versionsRef = useRef<Record<string, number>>({})
  const contentRef = useRef<Record<string, string>>({})
  const sourcePathRef = useRef("")
  const anonymousPathRef = useRef("anonymous.apex")
  const anonymousRef = useRef("")
  const modeRef = useRef<"scratch" | "persist">("scratch")
  const limitModeRef = useRef("permissive")
  const dirtyRef = useRef<Set<string>>(new Set())
  const editSeqRef = useRef(0)
  const runSeqRef = useRef(0)

  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark")
    localStorage.setItem("oaer-playground-theme", theme)
  }, [theme])

  useEffect(() => {
    metaRef.current = meta
  }, [meta])

  useEffect(() => {
    versionsRef.current = versions
  }, [versions])

  useEffect(() => {
    contentRef.current = contentByPath
  }, [contentByPath])

  useEffect(() => {
    sourcePathRef.current = sourcePath
  }, [sourcePath])

  useEffect(() => {
    anonymousPathRef.current = anonymousPath
  }, [anonymousPath])

  useEffect(() => {
    anonymousRef.current = anonymous
  }, [anonymous])

  useEffect(() => {
    modeRef.current = mode
  }, [mode])

  useEffect(() => {
    limitModeRef.current = limitMode
  }, [limitMode])

  const replaceDirty = useCallback((next: Set<string>) => {
    dirtyRef.current = next
    setDirtyPaths(new Set(next))
  }, [])

  const markDirty = useCallback(
    (path: string) => {
      editSeqRef.current += 1
      const next = new Set(dirtyRef.current)
      next.add(path)
      replaceDirty(next)
      setCacheState("stale")
    },
    [replaceDirty],
  )

  const saveFile = useCallback(
    async (path: string, content: string) => {
      const response = await api<FileSaveResponse>("files", {
        method: "PUT",
        body: JSON.stringify({
          path,
          content,
          version: versionsRef.current[path] ?? 0,
        }),
      })
      const currentContent =
        path === anonymousPathRef.current ? anonymousRef.current : contentRef.current[path] ?? content
      const saved = applySavedContent({
        path,
        savedContent: content,
        currentContent,
        dirtyPaths: dirtyRef.current,
      })
      const nextVersions = { ...versionsRef.current, [response.file.path]: response.file.version }
      versionsRef.current = nextVersions
      setVersions(nextVersions)
      if (saved.shouldReplaceEditorContent) {
        contentRef.current = { ...contentRef.current, [path]: content }
        setContentByPath((current) => ({ ...current, [path]: content }))
      }
      setMeta((current) => {
        if (!current) return current
        const index = current.files.findIndex((file) => file.path === response.file.path)
        const files = [...current.files]
        if (index >= 0) files[index] = response.file
        else files.push(response.file)
        files.sort((a, b) => a.path.localeCompare(b.path))
        return { ...current, files, workspaceHash: response.workspaceHash }
      })
      replaceDirty(saved.dirtyPaths)
      return response
    },
    [replaceDirty],
  )

  const saveDirty = useCallback(async () => {
    setStatus("Saving")
    const dirty = new Set(dirtyRef.current)
    const source = sourcePathRef.current
    const anon = anonymousPathRef.current
    if (source && dirty.has(source)) {
      await saveFile(source, contentRef.current[source] ?? "")
    }
    if (anon && dirty.has(anon)) {
      await saveFile(anon, anonymousRef.current)
    }
    setStatus("Saved")
  }, [saveFile])

  const run = useCallback(
    async () => {
      const runSeq = runSeqRef.current + 1
      runSeqRef.current = runSeq
      const startedAtEditSeq = editSeqRef.current
      setStatus("Running")
      setProblemMessage("")
      setResult(null)
      setRunning(true)
      try {
        const dirty = new Set(dirtyRef.current)
        const source = sourcePathRef.current
        const anon = anonymousPathRef.current
        if (source && dirty.has(source)) {
          await saveFile(source, contentRef.current[source] ?? "")
        }
        if (anon && dirty.has(anon)) {
          await saveFile(anon, anonymousRef.current)
        }
        if (!shouldApplyRunResult({ startedAtEditSeq, currentEditSeq: editSeqRef.current })) {
          setStatus("Ready")
          return
        }
        const next = await api<RunResult>("run", {
          method: "POST",
          body: JSON.stringify({
            anonymousBody: anonymousRef.current,
            mode: modeRef.current,
            limitMode: limitModeRef.current,
            useCache: true,
          }),
        })
        if (
          runSeq !== runSeqRef.current ||
          !shouldApplyRunResult({ startedAtEditSeq, currentEditSeq: editSeqRef.current })
        ) {
          return
        }
        setResult(next)
        setStatus(next.status === "pass" ? "Pass" : "Error")
        setCacheState(next.cacheHit ? "hit" : "fresh")
      } catch (error) {
        if (runSeq === runSeqRef.current) {
          setProblemMessage(error instanceof Error ? error.message : String(error))
          setStatus("Error")
        }
      } finally {
        if (runSeq === runSeqRef.current) {
          setRunning(false)
        }
      }
    },
    [saveFile],
  )

  const openFile = useCallback(async (path: string) => {
    const metadata = metaRef.current
    const file = metadata?.files.find((item) => item.path === path)
    if (!file) return
    setActivePath(path)
    if (file.kind === "anonymous") {
      setAnonymousPath(path)
      anonymousPathRef.current = path
      const cached = contentRef.current[path]
      const content = cached ?? (await readWorkspaceFile(path))
      anonymousRef.current = content
      setAnonymous(content)
      setContentByPath((current) => ({ ...current, [path]: content }))
      return
    }
    const cached = contentRef.current[path]
    const content = cached ?? (await readWorkspaceFile(path))
    sourcePathRef.current = path
    setSourcePath(path)
    setContentByPath((current) => ({ ...current, [path]: content }))
  }, [])

  const applyWorkspace = useCallback(
    async (workspace: WorkspaceMetadata, options: { preferred?: string; loadLatest?: boolean } = {}) => {
      const nextVersions = Object.fromEntries(workspace.files.map((file) => [file.path, file.version]))
      const anonymousFile = workspace.files.find((file) => file.kind === "anonymous")
      const firstSource =
        workspace.files.find((file) => file.path === options.preferred) ??
        workspace.files.find((file) => file.kind === "class") ??
        workspace.files.find((file) => file.kind !== "anonymous")
      const nextContent: Record<string, string> = {}
      if (anonymousFile) nextContent[anonymousFile.path] = workspace.anonymousBody ?? ""
      setMeta(workspace)
      setVersions(nextVersions)
      setLimitMode(workspace.limitMode || "permissive")
      setAnonymousPath(anonymousFile?.path ?? "anonymous.apex")
      setAnonymous(workspace.anonymousBody ?? "")
      setContentByPath(nextContent)
      if (workspace.exampleId) setSelectedExample(workspace.exampleId)
      replaceDirty(new Set())
      metaRef.current = workspace
      contentRef.current = nextContent
      versionsRef.current = nextVersions
      limitModeRef.current = workspace.limitMode || "permissive"
      anonymousPathRef.current = anonymousFile?.path ?? "anonymous.apex"
      anonymousRef.current = workspace.anonymousBody ?? ""
      sourcePathRef.current = ""
      setSourcePath("")
      setActivePath("")
      if (firstSource) {
        await openFile(firstSource.path)
      }
      if (options.loadLatest ?? true) {
        const latest = await api<{ found: boolean; result?: RunResult }>("runs/latest").catch(() => null)
        if (latest?.found && latest.result) {
          setResult(latest.result)
          setCacheState(latest.result.cacheHit ? "hit" : "fresh")
        }
      }
      setStatus("Ready")
    },
    [openFile, replaceDirty],
  )

  const loadWorkspace = useCallback(
    async (preferred?: string) => {
      setStatus("Loading")
      const workspace = await api<WorkspaceMetadata>("workspace")
      await applyWorkspace(workspace, { preferred, loadLatest: true })
    },
    [applyWorkspace],
  )

  useEffect(() => {
    void loadWorkspace().catch((error) => {
      setProblemMessage(error instanceof Error ? error.message : String(error))
      setStatus("Error")
    })
  }, [loadWorkspace])

  useEffect(() => {
    let cancelled = false
    void api<{ examples: ExampleProject[]; canLoad: boolean }>("examples")
      .then((body) => {
        if (cancelled) return
        setExamples(body.examples)
        setCanLoadExamples(body.canLoad)
        setSelectedExample((current) => current || body.examples[0]?.id || "")
      })
      .catch(() => {
        if (!cancelled) setCanLoadExamples(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const key = event.key.toLowerCase()
      if ((event.metaKey || event.ctrlKey) && key === "enter") {
        event.preventDefault()
        void run()
      }
      if ((event.metaKey || event.ctrlKey) && key === "s") {
        event.preventDefault()
        void saveDirty().catch((error) => {
          setProblemMessage(error instanceof Error ? error.message : String(error))
          setStatus("Error")
        })
      }
      if ((event.metaKey || event.ctrlKey) && event.shiftKey && key === "r") {
        event.preventDefault()
        setStatus("Resetting")
        void api("reset", { method: "POST" })
          .then(() => {
            setCacheState("stale")
            setStatus("Ready")
          })
          .catch((error) => {
            setProblemMessage(error instanceof Error ? error.message : String(error))
            setStatus("Error")
          })
      }
      if ((event.metaKey || event.ctrlKey) && key === "k") {
        event.preventDefault()
        setCommandOpen(true)
      }
      if (event.key === "Escape") setCommandOpen(false)
    }
    document.addEventListener("keydown", onKeyDown)
    return () => document.removeEventListener("keydown", onKeyDown)
  }, [run, saveDirty])

  const groups = useMemo(() => {
    const files = meta?.files ?? []
    const sourceCount = files.filter((file) => file.kind === "class" || file.kind === "trigger").length
    const groups = [
      {
        label: "Classes",
        icon: FileCode2,
        files: filterWorkspaceFiles(files, classSearch),
        forceVisible: sourceCount > 0 || classSearch.trim() !== "",
      },
      { label: "Data", icon: Database, files: files.filter((file) => file.kind === "data") },
      { label: "Metadata", icon: Braces, files: files.filter((file) => file.kind === "metadata" || file.kind === "other") },
    ]
    return groups.filter((group) => group.files.length > 0 || group.forceVisible)
  }, [classSearch, meta?.files])

  const sourceContent = sourcePath ? contentByPath[sourcePath] ?? "" : ""
  const runTime = result ? `${(result.compileMs ?? 0) + (result.executeMs ?? 0)} ms` : "-"
  const logs = result?.logs ?? []
  const diagnostics = result?.diagnostics ?? []
  const vars = result?.vars ?? []
  const limits = result?.limits ?? {}
  const orgDiff = result?.orgDiff ?? []
  const cacheLabel = cacheState === "hit" ? "cache hit" : cacheState === "fresh" ? "cache fresh" : "cache stale"
  const selectedExampleDetails = useMemo(
    () => examples.find((example) => example.id === selectedExample),
    [examples, selectedExample],
  )

  const onSourceChange = (value: string) => {
    if (!sourcePath) return
    contentRef.current = { ...contentRef.current, [sourcePath]: value }
    setContentByPath((current) => ({ ...current, [sourcePath]: value }))
    markDirty(sourcePath)
  }

  const onAnonymousChange = (value: string) => {
    anonymousRef.current = value
    setAnonymous(value)
    markDirty(anonymousPath)
  }

  const createClass = async () => {
    const suffix = Date.now().toString().slice(-4)
    const name = `Scratch${suffix}`
    const path = `force-app/main/default/classes/${name}.cls`
    const content = `public class ${name} {\n  public static String label() {\n    return '${name}';\n  }\n}\n`
    setStatus("Saving")
    await saveFile(path, content)
    sourcePathRef.current = path
    setSourcePath(path)
    setActivePath(path)
    setStatus("Saved")
  }

  const loadExample = useCallback(async () => {
    if (!selectedExample || !canLoadExamples) return
    if (dirtyRef.current.size > 0 && !window.confirm("Load example and replace this scratch workspace?")) return
    runSeqRef.current += 1
    setRunning(false)
    setStatus("Loading")
    setProblemMessage("")
    setResult(null)
    setCacheState("stale")
    const workspace = await api<WorkspaceMetadata>("examples/load", {
      method: "POST",
      body: JSON.stringify({ id: selectedExample }),
    })
    await applyWorkspace(workspace, { loadLatest: false })
  }, [applyWorkspace, canLoadExamples, selectedExample])

  const deleteWorkspaceFile = async (path: string) => {
    if (!window.confirm(`Delete ${fileName(path)} from this workspace?`)) return
    runSeqRef.current += 1
    setRunning(false)
    setStatus("Deleting")
    setProblemMessage("")
    setResult(null)
    await api(`files?path=${encodeURIComponent(path)}`, { method: "DELETE" })
    const workspace = await api<WorkspaceMetadata>("workspace")
    const preferred = sourcePathRef.current === path ? undefined : sourcePathRef.current
    setCacheState("stale")
    await applyWorkspace(workspace, { preferred, loadLatest: false })
  }

  const resetOrg = async () => {
    setStatus("Resetting")
    await api("reset", { method: "POST" })
    setCacheState("stale")
    setStatus("Ready")
  }

  const seedOrg = async () => {
    setStatus("Seeding")
    await api("seed", { method: "POST" })
    setCacheState("stale")
    setStatus("Seeded")
  }

  const saveAndHandle = () => {
    void saveDirty().catch((error) => {
      setProblemMessage(error instanceof Error ? error.message : String(error))
      setStatus("Error")
    })
  }

  const runAndHandle = () => {
    void run().catch((error) => {
      setProblemMessage(error instanceof Error ? error.message : String(error))
      setStatus("Error")
    })
  }

  const loadExampleAndHandle = () => {
    void loadExample().catch((error) => {
      setProblemMessage(error instanceof Error ? error.message : String(error))
      setStatus("Error")
    })
  }

  return (
    <div className="flex h-screen min-h-[720px] flex-col bg-background text-foreground">
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-border bg-background/95 px-4">
        <div className="flex min-w-0 items-center gap-3">
          <div className="grid size-9 place-items-center rounded-md bg-primary text-sm font-bold text-primary-foreground">
            OA
          </div>
          <div className="min-w-0">
            <h1 className="truncate text-sm font-semibold">OAER Apex Playground</h1>
            <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
              <span>{meta?.id ?? "default"}</span>
              <span className="font-mono">{shortHash(meta?.workspaceHash)}</span>
            </div>
          </div>
          <Badge variant={statusVariant(status)}>{status}</Badge>
          <Badge variant={cacheState === "stale" ? "warning" : "success"}>{cacheLabel}</Badge>
        </div>
        <div className="flex items-center gap-2">
          <Select
            value={mode}
            onValueChange={(value) => {
              const next = value as "scratch" | "persist"
              modeRef.current = next
              setMode(next)
              setCacheState("stale")
            }}
          >
            <SelectTrigger className="w-[118px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="scratch">scratch</SelectItem>
              <SelectItem value="persist">persist</SelectItem>
            </SelectContent>
          </Select>
          <Select
            value={limitMode}
            onValueChange={(value) => {
              limitModeRef.current = value
              setLimitMode(value)
              setCacheState("stale")
            }}
          >
            <SelectTrigger className="w-[132px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="permissive">permissive</SelectItem>
              <SelectItem value="strict">strict</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="outline" size="icon" onClick={saveAndHandle} title="Save">
            <Save />
          </Button>
          <Button onClick={runAndHandle} disabled={running} title="Run">
            <Play />
            Run
          </Button>
          <Button variant="ghost" size="icon" onClick={() => setCommandOpen(true)} title="Command">
            <Command />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            title="Theme"
          >
            {theme === "dark" ? <Sun /> : <Moon />}
          </Button>
        </div>
      </header>

      <main className="grid min-h-0 flex-1 grid-cols-[280px_minmax(420px,1fr)_minmax(360px,430px)] gap-3 p-3 max-xl:grid-cols-[250px_minmax(420px,1fr)] max-xl:[&_.output-pane]:col-span-2 max-lg:grid-cols-1 max-lg:overflow-auto">
        <aside className="pane flex min-h-0 flex-col">
          <header className="pane-header">
            <div className="flex items-center gap-2">
              <FolderTree className="size-4 text-primary" />
              <h2 className="text-sm font-semibold">Workspace</h2>
            </div>
            <Button size="sm" variant="outline" onClick={() => void createClass()} title="New class">
              <Plus />
              Class
            </Button>
          </header>
          {examples.length > 0 ? (
            <div className="space-y-2 border-b border-border p-2">
              <div className="flex gap-2">
                <Select value={selectedExample} onValueChange={setSelectedExample}>
                  <SelectTrigger className="h-8 min-w-0 flex-1">
                    <SelectValue placeholder="Projects" />
                  </SelectTrigger>
                  <SelectContent>
                    {examples.map((example) => (
                      <SelectItem key={example.id} value={example.id}>
                        {example.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button
                  className="h-8 shrink-0"
                  variant="outline"
                  onClick={loadExampleAndHandle}
                  disabled={!canLoadExamples || !selectedExample}
                  title={canLoadExamples ? "Load project" : "Project loading only works in scratch workspaces"}
                >
                  <BookOpen />
                  Load
                </Button>
              </div>
              {selectedExampleDetails ? (
                <div className="space-y-1 px-1 pb-1">
                  <p className="line-clamp-2 text-[11px] leading-4 text-muted-foreground">
                    {selectedExampleDetails.description}
                  </p>
                  <div className="flex flex-wrap gap-1">
                    {selectedExampleDetails.fileCount > 0 ? (
                      <Badge variant="outline">{selectedExampleDetails.fileCount} files</Badge>
                    ) : null}
                    {(selectedExampleDetails.tags ?? []).map((tag) => (
                      <Badge key={tag} variant="outline">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </div>
              ) : null}
            </div>
          ) : null}
          <ScrollArea className="min-h-0 flex-1">
            <div className="space-y-4 p-3">
              {groups.map((group) => {
                const Icon = group.icon
                return (
                  <div key={group.label}>
                    <div className="mb-2 flex items-center gap-2 text-[11px] font-semibold uppercase text-muted-foreground">
                      <Icon className="size-3.5" />
                      {group.label}
                    </div>
                    {group.label === "Classes" ? (
                      <label className="mb-2 flex h-8 items-center gap-2 rounded-md border border-border bg-background/70 px-2 text-muted-foreground">
                        <Search className="size-3.5" />
                        <input
                          className="min-w-0 flex-1 border-0 bg-transparent text-xs text-foreground outline-none placeholder:text-muted-foreground"
                          value={classSearch}
                          onChange={(event) => setClassSearch(event.target.value)}
                          placeholder="Search classes"
                        />
                      </label>
                    ) : null}
                    <div className="space-y-1">
                      {group.files.length === 0 && group.label === "Classes" ? (
                        <div className="rounded-md border border-dashed border-border px-3 py-2 text-xs text-muted-foreground">
                          No matching classes
                        </div>
                      ) : null}
                      {group.files.map((file) => {
                        const selected = file.path === activePath || file.path === sourcePath
                        const dirty = dirtyPaths.has(file.path)
                        return (
                          <div
                            key={file.path}
                            className={cn("file-row", selected && "selected")}
                          >
                            <button
                              className="min-w-0 flex-1 border-0 bg-transparent p-0 text-left text-inherit"
                              onClick={() => {
                                void openFile(file.path).catch((error) => {
                                  setProblemMessage(error instanceof Error ? error.message : String(error))
                                  setStatus("Error")
                                })
                              }}
                            >
                              <span className="block truncate text-xs font-medium">{fileName(file.path)}</span>
                              <span className="block truncate font-mono text-[10px] text-muted-foreground">
                                {file.path}
                              </span>
                            </button>
                            <span className="flex shrink-0 items-center gap-1 font-mono text-[10px] text-muted-foreground">
                              {dirty ? <CircleDashed className="size-3 text-amber-500" /> : null}
                              v{file.version}
                              <span>{formatBytes(file.size)}</span>
                            </span>
                            {file.kind === "class" || file.kind === "trigger" ? (
                              <Button
                                variant="ghost"
                                size="icon"
                                className="size-7 shrink-0 text-muted-foreground hover:text-destructive"
                                title={`Delete ${fileName(file.path)}`}
                                onClick={() => {
                                  void deleteWorkspaceFile(file.path).catch((error) => {
                                    setProblemMessage(error instanceof Error ? error.message : String(error))
                                    setStatus("Error")
                                  })
                                }}
                              >
                                <Trash2 />
                              </Button>
                            ) : null}
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )
              })}
            </div>
          </ScrollArea>
        </aside>

        <section className="grid min-h-0 grid-rows-[minmax(0,1.25fr)_minmax(230px,0.75fr)] gap-3">
          <CodeEditor
            title="Apex Source"
            subtitle={sourcePath || "No source file selected"}
            value={sourceContent}
            onChange={onSourceChange}
          />
          <CodeEditor
            title="Execute Anonymous"
            subtitle={anonymousPath}
            value={anonymous}
            onChange={onAnonymousChange}
            runLabel={running ? "Running" : "Run"}
            running={running}
            onRun={runAndHandle}
          />
        </section>

        <aside className="pane output-pane min-h-0">
          <header className="pane-header">
            <div className="flex min-w-0 items-center gap-2">
              {status === "Pass" ? (
                <CheckCircle2 className="size-4 text-violet-400" />
              ) : status === "Error" ? (
                <CircleAlert className="size-4 text-red-500" />
              ) : (
                <Boxes className="size-4 text-primary" />
              )}
              <h2 className="truncate text-sm font-semibold">Output</h2>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="icon" onClick={() => void seedOrg()} title="Seed data">
                <Database />
              </Button>
              <Button variant="ghost" size="icon" onClick={() => void resetOrg()} title="Reset org">
                <RotateCcw />
              </Button>
            </div>
          </header>

          <div className="grid grid-cols-4 gap-2 p-3">
            <div className="metric">
              <span>Status</span>
              <strong>{result?.status ?? "-"}</strong>
            </div>
            <div className="metric">
              <span>Time</span>
              <strong>{runTime}</strong>
            </div>
            <div className="metric">
              <span>DML</span>
              <strong>{limits.dmlStatements ?? 0}</strong>
            </div>
            <div className="metric">
              <span>Rows</span>
              <strong>{limits.dmlRows ?? 0}</strong>
            </div>
          </div>

          <Tabs defaultValue="logs" className="flex min-h-0 flex-1 flex-col px-3 pb-3">
            <TabsList className="grid w-full grid-cols-5">
              <TabsTrigger value="logs">Logs</TabsTrigger>
              <TabsTrigger value="vars">Vars</TabsTrigger>
              <TabsTrigger value="problems">Problems</TabsTrigger>
              <TabsTrigger value="limits">Limits</TabsTrigger>
              <TabsTrigger value="trace">Trace</TabsTrigger>
            </TabsList>
            <TabsContent value="logs" className="min-h-0 flex-1">
              <ScrollArea className="result-box">
                <pre>{logs.length ? logs.join("\n") : result?.errorMessage || "No output"}</pre>
              </ScrollArea>
            </TabsContent>
            <TabsContent value="vars" className="min-h-0 flex-1">
              <ScrollArea className="result-box">
                <table className="result-table">
                  <tbody>
                    {vars.length ? (
                      vars.map((item) => (
                        <tr key={item.name}>
                          <th>{item.name}</th>
                          <td>{item.type || "-"}</td>
                          <td>{valuePreview(item.value)}</td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td>No variables</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </ScrollArea>
            </TabsContent>
            <TabsContent value="problems" className="min-h-0 flex-1">
              <ScrollArea className="result-box">
                <div className="space-y-2 p-3">
                  {problemMessage ? <div className="problem danger">{problemMessage}</div> : null}
                  {diagnostics.length ? (
                    diagnostics.map((item, index) => (
                      <div className="problem" key={`${item.message}-${index}`}>
                        <Badge variant={item.severity === "error" ? "danger" : "warning"}>{item.severity}</Badge>
                        <span>{item.message}</span>
                        {item.line ? <code>{item.line}:{item.column ?? 0}</code> : null}
                      </div>
                    ))
                  ) : !problemMessage ? (
                    <div className="text-sm text-muted-foreground">No problems</div>
                  ) : null}
                </div>
              </ScrollArea>
            </TabsContent>
            <TabsContent value="limits" className="min-h-0 flex-1">
              <ScrollArea className="result-box">
                <table className="result-table">
                  <tbody>
                    {Object.entries(limits).map(([key, value]) => (
                      <tr key={key}>
                        <th>{key}</th>
                        <td>{value}</td>
                      </tr>
                    ))}
                    {orgDiff.map((item) => (
                      <tr key={item.object}>
                        <th>{item.object}</th>
                        <td>{`+${item.inserted} ~${item.updated} -${item.deleted}`}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </ScrollArea>
            </TabsContent>
            <TabsContent value="trace" className="min-h-0 flex-1">
              <ScrollArea className="result-box">
                <pre>{JSON.stringify(result?.trace ?? [], null, 2)}</pre>
              </ScrollArea>
            </TabsContent>
          </Tabs>
        </aside>
      </main>

      {commandOpen ? (
        <div className="command-backdrop" onMouseDown={() => setCommandOpen(false)}>
          <div className="command-panel" onMouseDown={(event) => event.stopPropagation()}>
            <div className="flex items-center gap-2 border-b border-border px-3 py-2 text-sm text-muted-foreground">
              <Search className="size-4" />
              <span>Command</span>
            </div>
            <div className="p-2">
              {[
                { label: "Run", icon: Play, action: runAndHandle },
                { label: "Save", icon: Save, action: saveAndHandle },
                { label: "Load example", icon: BookOpen, action: loadExampleAndHandle },
                { label: "New class", icon: Plus, action: () => void createClass() },
                { label: "Seed data", icon: Database, action: () => void seedOrg() },
                { label: "Reset org", icon: RefreshCcw, action: () => void resetOrg() },
                {
                  label: theme === "dark" ? "Light mode" : "Dark mode",
                  icon: Zap,
                  action: () => setTheme(theme === "dark" ? "light" : "dark"),
                },
              ].map((item) => {
                const Icon = item.icon
                return (
                  <button
                    key={item.label}
                    className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-muted"
                    onClick={() => {
                      item.action()
                      setCommandOpen(false)
                    }}
                  >
                    <Icon className="size-4 text-primary" />
                    {item.label}
                  </button>
                )
              })}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
