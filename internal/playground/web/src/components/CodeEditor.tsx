import { useMemo, useRef } from "react"
import Prism from "prismjs"
import "prismjs/components/prism-clike"
import "prismjs/components/prism-sql"
import "prismjs/components/prism-apex"
import { Play } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

type CodeEditorProps = {
  title: string
  subtitle?: string
  contextLabel?: string
  contextTitle?: string
  value: string
  onChange: (value: string) => void
  className?: string
  runLabel?: string
  running?: boolean
  readOnly?: boolean
  onRun?: () => void
}

export function CodeEditor({
  title,
  subtitle,
  contextLabel,
  contextTitle,
  value,
  onChange,
  className,
  runLabel,
  running = false,
  readOnly = false,
  onRun,
}: CodeEditorProps) {
  const highlightRef = useRef<HTMLPreElement>(null)
  const gutterRef = useRef<HTMLPreElement>(null)
  const lineCount = Math.max(1, value.split("\n").length)
  const highlighted = useMemo(() => {
    const grammar = Prism.languages.apex ?? Prism.languages.clike
    return Prism.highlight(value, grammar, "apex")
  }, [value])
  const gutter = useMemo(
    () =>
      Array.from({ length: lineCount }, (_, index) => String(index + 1).padStart(2, " ")).join("\n"),
    [lineCount],
  )

  return (
    <section className={cn("pane flex min-h-0 flex-col", className)}>
      <header className="pane-header">
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-sm font-semibold">{title}</h2>
          {subtitle ? <p className="truncate font-mono text-[11px] text-muted-foreground">{subtitle}</p> : null}
        </div>
        <div className="flex min-w-0 shrink items-center gap-2">
          {contextLabel ? (
            <span
              className="max-w-[220px] truncate font-mono text-[11px] text-muted-foreground"
              title={contextTitle ?? contextLabel}
            >
              {contextLabel}
            </span>
          ) : null}
          <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
            {lineCount} ln
          </span>
        </div>
        {onRun ? (
          <Button size="sm" onClick={onRun} disabled={running} title="Run">
            <Play />
            {runLabel ?? "Run"}
          </Button>
        ) : null}
      </header>
      <div className="editor-shell">
        <pre ref={gutterRef} className="editor-gutter" aria-hidden="true">
          {gutter}
        </pre>
        <div className="editor-main">
          <pre
            ref={highlightRef}
            className="editor-highlight language-apex"
            aria-hidden="true"
            dangerouslySetInnerHTML={{ __html: highlighted + "\n" }}
          />
          <textarea
            className="editor-input"
            value={value}
            readOnly={readOnly}
            spellCheck={false}
            onChange={(event) => onChange(event.target.value)}
            onScroll={(event) => {
              if (highlightRef.current) {
                highlightRef.current.scrollTop = event.currentTarget.scrollTop
                highlightRef.current.scrollLeft = event.currentTarget.scrollLeft
              }
              if (gutterRef.current) {
                gutterRef.current.scrollTop = event.currentTarget.scrollTop
              }
            }}
          />
        </div>
      </div>
    </section>
  )
}
