# Glade CLI DX Improvements — Deep Dive

Analysis date: 2026-06-10. Based on full source review of `cmd/`, `internal/gladecli/`,
`internal/cliui/`, `internal/config/`, `internal/project/`, and related packages.

## Current CLI Surface (19 top-level commands)

```
version  doctor  parse  inspect  schema  check  exec  debug  editor
dap      test    dev    report   lsp      profile package server playground  db  help
```

---

## 1. Shell Completion (P0 — High Impact)

For a CLI with 19+ commands and dozens of flags, shell completion was the
single highest-ROI DX gap. The first cut is now in place.

**Current state:** First cut implemented: `glade completion bash|zsh|fish`
generates scripts from a shared command catalog. Terminal users can now complete
top-level commands, subcommands, and long flags.

**Remaining polish:** add install snippets to docs, add smarter value completion
for project paths and enum-like values, and keep command metadata in sync as the
flag parser improves.

**Files touched:** `internal/gladecli/completion_command.go` (new), `internal/gladecli/cli.go`.

---

## 2. Help System (P0 — High Impact)

The help system had been largely absent. First cut implemented: `glade <command>
--help`, `glade <subcommand> --help`, and `glade help <command>` now use the
shared command catalog. `glade help test` keeps its deeper hand-written reference.
Before that, every other command that received unknown flags or no args printed a
bare one-line usage error with no flag descriptions:

```
$ glade parse --help
glade: usage: glade parse <paths...> [--json]

$ glade check --help
glade: usage: glade check [--project <root>] [--json] [--progress|...]
```

Compare to `glade test --help` which produces a deeper reference with sections for
"Persistent test server", "Serve flags", "Common flags", "Startup cache", and
"Examples". That richer hand-written pattern still exists only for `test`.

**Remaining polish:**
- Add deeper per-subcommand topics where a command has many modes.
- Generate help directly from the future shared flag parser.
- Add typo suggestions for unknown commands and flags.

**Files touched:** `internal/cliui/help.go`, `internal/gladecli/cli.go`.

---

## 3. Flag Parsing Boilerplate (P1 — Medium Impact, Internal Quality)

Every command manually parses flags with index-based loops. This pattern is repeated
verbatim across 9 files covering roughly 500 lines of near-identical code:

```go
// From cli.go:770 (pattern repeated in every command file)
for i := 0; i < len(args); i++ {
    switch args[i] {
    case "--project":
        if i+1 >= len(args) { return errors.New("--project requires a value") }
        root = args[i+1]
        i++
    case "--json":
        jsonOut = true
    default:
        return fmt.Errorf("unknown flag %q", args[i])
    }
}
```

The `parseProjectFlags` helper at `cli.go:959` and `parseProjectProgressFlags` at
`cli.go:978` are partial solutions that only cover `--project` and `--json`/`--progress`.
They don't address per-command flags like `--filter`, `--changed-since`, `--addr`,
`--limit-mode`, `--watch`, etc.

**Problems:**
- Every new flag needs boilerplate in multiple places
- Subtle bugs from inconsistent handling (e.g., unknown flags silently consumed as
  positionals in `parse`)
- No `--` end-of-options support anywhere
- No short flags (`-p` for `--project`, `-j` for `--json`)
- Flag value validation is scattered

**Suggested:** A lightweight internal flag parser (conceptually similar to `flag.FlagSet`
but without `flag`'s global state and `os.Args` assumptions, since `Run()` takes `args []string`).
This would:
- Eliminate ~500 lines of boilerplate
- Make `--help` generation automatic (flags are registered with descriptions)
- Enable short flags
- Provide consistent error messages including Levenshtein suggestions for typos

**Files touched:** New `internal/flagparse/` or embedding in `internal/cliui/`.
All command files rewritten to use it.

---

## 4. `--json` Consistency (P1 — Medium Impact)

Current `--json` support by command:

| Command | `--json` | Notes |
|---------|----------|-------|
| `version` | Yes | First cut emits version, Go runtime, OS, and architecture |
| `doctor` | Yes | First cut emits `DoctorInfo` as JSON |
| `parse` | Yes | |
| `inspect` | Yes | Both symbols and performance |
| `schema load` | Yes | |
| `check` | Yes | |
| `exec` | Yes | |
| `test` | Yes | |
| `debug` | Yes | Partial (some subcommands) |
| `editor` | No | Not applicable |
| `dap` | No | JSON protocol already |
| `report` | No | No structured output |
| `lsp` | No | JSON protocol already |
| `profile analyze` | Yes | |
| `package build` | No | `--json` flag exists but only controls stdout — the artifact is always JSON |
| `server` | No | Not applicable |
| `playground` | No | Not applicable |
| `db` | Yes | inspect/export |
| `dev` | No | Human-oriented by design |

Remaining key gap:
- `glade report show latest --json` should emit the run summary

**Files touched:** `internal/gladecli/cli.go` (version, doctor), `internal/gladecli/dev_command.go` (report).

---

## 5. Progress Indicators (P2 — Medium Impact)

Only three commands use the progress renderer system:
- `check` — 4-phase progress (load project, load metadata, index, semantic checks)
- `schema load` — 2-phase progress
- `test` — rich progress with running count, per-test status, slow-test warnings

Commands that _could_ benefit:
- `parse` — scanning hundreds of files with no feedback; a simple file count ticker
- `package build` — indexing and artifact generation can be slow on large projects
- `db seed` — large fixture files take time
- `exec` with large computations

The progress infrastructure in `cliui/` already supports modes: auto (TTY), line,
JSON-NDJSON, and off. It's well-designed and needs only to be wired into more commands.

**Files touched:** `internal/gladecli/cli.go` (parse, package), `internal/gladecli/db_command.go`.

---

## 6. Config and Dotfile Management (P1-P2 — Medium Impact)

`glade.yml` is the sole config file, parsed by `config/config.go` with a custom YAML
subset parser. It supports only 5 keys:

```yaml
project:
  root: ""
  packageDirs: []
  defaultNamespace: ""
  managedPackageDependencies: []
org:
  features: []
```

**Missing CLI commands for config:**
- `glade config validate` — validates `glade.yml` syntax with proper error reporting
- `glade config show` — prints resolved config (layered with `sfdx-project.json`)
- `glade config init` — interactive scaffolding that asks project root, namespace,
  package dirs, and writes a valid `glade.yml`

**Missing config keys:**
- `test.timeout` — default per-test timeout
- `test.parallelism` — default worker count
- `test.watchBackend` — native/poll/auto
- `test.debounce` — watch debounce interval
- `test.limitMode` — strict/permissive
- `check.ignore` — patterns or rule codes to suppress in `check`

Currently, these are all CLI-only flags. A project can't express "this test suite needs
`--no-parallel-methods`" except through shell wrappers or Makefiles.

**The inline list format** `[a, b, c]` in the YAML subset parser is undocumented. Users
must infer it from source.

**Files touched:** `internal/config/config.go`, new `internal/gladecli/config_command.go`,
`docs/` (config reference).

---

## 7. Watch/Test Daemon UX (P2 — Medium Impact)

Two overlapping watch paths exist:
1. `glade test --watch` / `glade test --daemon` — emits NDJSON events to stdout
2. `glade dev watch` — human-friendly output, writes run artifacts, supports
   `--failed` and `--changed`

The daemon socket (`glade test serve`) auto-connects from subsequent `glade test` runs
via `tryTestServerRun()` in `test_command.go:274`. This is elegant. But there are gaps:

**No lifecycle management:**
- `glade test serve` starts a daemon. There's no `stop`, `status`, or `list` command.
- If the terminal dies, the daemon socket remains. The daemon process is orphaned.
- `testdaemon.ServerReachable()` checks the socket but can give a false positive on
  a stale socket.

**Suggested:**
- `glade test daemon list` — show running daemons (project root, PID, uptime)
- `glade test daemon stop` — graceful shutdown via socket or signal
- `glade test daemon status` — health check (is index warm? last run?)
- `glade test daemon restart` — stop + start

**Unify dev test and test watch:**
`dev test --changed` (dev_command.go:186) and `test --changed-since HEAD`
(test_command.go:346) are semantically identical but implemented as separate code
paths. The `changedSinceSelection` function lives in `dev_command.go` but is called
from `test_command.go`. Consolidate into `internal/watch/`.

**Files touched:** `internal/gladecli/test_command.go`, `internal/gladecli/dev_command.go`,
`internal/testdaemon/`.

---

## 8. New Command Opportunities

### 8a. `glade init` (P1 — Medium Effort)

Scaffolds a new SFDX project or bootstraps `glade.yml` from an existing one.
The `doctor` command already walks up the directory tree looking for config.
`init` is the natural pairing.

```
glade init                # interactive wizard
glade init --project .    # scaffold glade.yml from current directory
glade init --output .     # create full project skeleton
```

### 8b. `glade coverage` (P2 — Medium Effort)

The VM executes tests and `vm.Trace` records line-level execution via `trace.Event`.
There is no code coverage report. A `coverage` command that intersects trace data
with the AST would give developer-visible coverage — percentage, per-file breakdown,
uncovered lines.

### 8c. `glade lint` (P3 — Medium Effort)

`check` is the full semantic analyzer. A lightweight `lint` that runs only fast,
style-oriented checks (separate from full semantic analysis) would be useful for
editor integration and pre-commit hooks. Could also be a mode on `check`:
`glade check --lint-only`.

---

## 9. Best First Cut for Glade

The best first DX cut is a shared command catalog that feeds:

- `glade <command> --help`
- `glade help <command>`
- `glade completion bash|zsh|fish`

That one move makes the CLI easier to discover without changing runtime behavior.
It also gives future wizard work a single place to read command names, flags,
subcommands, and examples.

This cut should stay product-only. Do not add completions or help topics for
maintenance surfaces that moved to `glade-tools`.

## 10. Wizard Ideas That Fit Glade

### 10a. `glade init` wizard (P1)

This is the most useful wizard.

Flow:

1. Detect `sfdx-project.json`, `glade.yml`, package directories, namespace, and
   scratch org features.
2. Ask only for missing values.
3. Write a small `glade.yml`.
4. Run `glade doctor`.
5. Print the next two commands:

```bash
glade check --project .
glade test --project .
```

Flags should make it CI-safe:

```bash
glade init --project . --yes
glade init --project . --namespace pkg --package-dir force-app --force
```

### 10b. `glade test --wizard` (P2)

Help developers pick a fast local test path.

The wizard can inspect project size, existing `.glade/test/startup.gob`, and a
reachable test server socket. Then it recommends one command:

- single class: `glade test --filter AccountServiceTest`
- changed files: `glade test --changed-since HEAD`
- warm loop: `glade test serve --project .`
- stubborn cache: `glade test clear-cache --project .`

It should not hide the command it runs. Show the exact command, then ask before
running it.

### 10c. `glade db seed --wizard` (P2)

Good for local app developers who want a working org state.

Flow:

1. Pick or create `.glade/org.sqlite`.
2. Offer fixture files found under `testdata/`, `fixtures/`, and `data/`.
3. Show object and record counts before writing.
4. Run `glade db inspect --db <path>` after seeding.

### 10d. `glade playground --wizard` (P3)

Good for demos and support.

Flow:

1. Pick workspace.
2. Pick project reference.
3. Pick database.
4. Offer examples.
5. Start playground with `--open`.

Keep this behind the existing `playground` command. A separate top-level TUI
would add weight before the normal CLI surface is finished.

## 11. More DX Ideas

- `glade doctor --fix-suggestions`: print concrete next commands for missing
  config, parser-disabled builds, and stale test cache.
- `glade explain <error-code>`: describe `GLADE*` diagnostics with examples and
  likely fixes.
- `glade test --last-failed`: promote the existing `dev test --failed` flow into
  the main test command.
- `glade test daemon status|stop|restart`: make the warm server lifecycle visible.
- `glade check --format github`: emit annotations for CI without a wrapper script.
- `glade report open latest`: open the latest HTML/Markdown report when one
  exists.
- `glade config show|validate`: inspect resolved `glade.yml` plus inferred SFDX
  settings before running heavier commands.

### 8d. `glade explain <code>` (P5 — Small Effort)

Given an Apex error code like `GLADESCHEMA001`, print what it means and common fixes.
The diagnostic catalog exists already in the codebase — just needs CLI surface.

### 8e. `glade bench` (P4 — Small-Medium Effort)

The VM already has benchmarking infrastructure (`vm_benchmark_test.go`). A top-level
command that runs standard benchmarks and prints throughput (compiles/sec, tests/sec,
SOQL/sec) would help developers tune their setup and compare Glade builds.

### 8f. `glade fmt` (P4 — Large Effort)

Apex formatter in the Go toolchain style. The parse tree is available through
`apexast`. Implementation requires a pretty-printer over the AST.

### 8g. `glade docs` (P4 — Medium Effort)

Generate API docs or class references from the index. `typesys.Index` contains every
type and member signature. Rendering is a templating problem. Could output Markdown
or HTML.

### 8h. `glade graph` (P4 — Medium Effort)

Dependency graph visualization. The `typesys.Index` tracks super classes, interfaces,
and member relationships. Emitting DOT or Mermaid syntax would help understand
large projects.

---

## 12. Output Formats (P2-P3 — Medium Impact)

Current test output formats: console (plain text), JSON, JUnit XML, NDJSON (watch).

**Missing:**
- **SARIF** — Static Analysis Results Interchange Format. Standard format supported
  by GitHub, GitLab, VS Code. Emitting `check` results as SARIF would enable native
  CI annotations: `glade check --format sarif --output results.sarif`
- **TAP** — Test Anything Protocol. Common in CI/CD pipelines.
  `glade test --format tap`
- **HTML report** — The `dev test` command already writes `summary.md` to
  `.glade/runs`. A simple HTML template would make reports browsable in CI artifacts.
- **GitHub Actions annotations** — `::error file=AccountService.cls,line=42::message`

---

## 13. Color/TTY Control (P3 — Low Impact)

The `Theme` struct in `cliui/theme.go` detects TTY via `ColorEnabled(isTTY, noColor)`.
The `noColor` parameter comes from an environment variable check. There is no CLI
override.

**Suggested:**
```
--color=auto|always|never
NO_COLOR=1 glade test ...   # already partially supported
FORCE_COLOR=1 glade test ...  # force color in pipes
```

---

## 14. Error Message Quality (P3 — Low Impact)

**Unknown flag suggestions:**
```
$ glade test --filteer AccountServiceTest
glade: unknown flag "--filteer"
```
No "did you mean `--filter`?" suggestion. Given the flag list is known for each
command, Levenshtein distance suggestions are straightforward and high-signal.

**Usage messages are too terse:**
```
usage: glade parse <paths...> [--json]
```
Doesn't explain that `<paths...>` accepts files or directories, or that directories
are walked recursively for `.cls` and `.trigger` files.

**Exit code documentation:**
The exit code contract is undocumented:
- `parse`, `inspect`, `check` return 1 on errors
- `test` returns 1 on test failures or errors
- `dev` conditionally returns 1 based on whether tests ran
- Unknown command returns 2

This should be documented in `--help` output and in a consistent help topic.

---

## 15. CI/CD Integration (P2 — Medium Impact)

No built-in CI utilities:

**Pre-commit hooks:**
```
glade hook install pre-commit   # writes .git/hooks/pre-commit
glade hook install pre-push     # runs changed-since origin/main
```

**CI summary mode:**
```
glade check --format ci    # one-line summary
glade test --format ci     # "PASS: 42 tests in 1.2s" or "FAIL: 3/42 tests"
```

**GitHub Actions setup:**
A `glade ci init` that writes a `.github/workflows/glade.yml` with check + test steps.

---

## 16. Package Command Depth (P3 — Low-Medium Impact)

`glade package build` produces a JSON artifact. Missing subcommands:
- `glade package validate` — validate an existing artifact against a project
- `glade package info` — print artifact metadata (namespace, version, type count) without full file load
- `glade package diff artifact1.json artifact2.json` — compare two artifact versions

---

## 17. DB Command Depth (P2-P3 — Medium Impact)

`glade db` covers seed, reset, export, and inspect. Missing:
- `glade db query "SELECT Id, Name FROM Account WHERE Name LIKE 'A%'"` — raw SOQL against the local database
- `glade db dump` — full table dump for debugging
- `glade db migrate` — schema version migration support (schema version already tracked)

---

## 18. Version Command (P1 — Small Impact)

`glade version` now supports `--json`. The `Version` variable is still hardcoded
to `"0.0.0-dev"` at `cli.go:32` — there's no ldflags-based injection visible in
the command-driver code (though it likely exists in the build scripts).

**Current:**
```
$ glade version --json
{"version":"0.7.2","go":"go1.23.4","os":"darwin","arch":"arm64"}
```

---

## 19. Diagnostic Display Quality (P3 — Low-Medium Impact)

`glade check` output uses `diagnostic.Report.WriteText` which prints one diagnostic
per line. For large projects, this produces an ungrouped wall of text.

**Suggested improvements:**
- Group by file: show file header, then diagnostics underneath
- Group by severity: errors first, then warnings
- Optional `--by-file` and `--by-rule` flags
- Summary line: "42 errors in 12 files"
- `--max-diagnostics N` to cap output

---

## 20. `--no-` / `--` Negation Convention (P4 — Low Impact)

Current negation flags:
- `--no-progress`, `--quiet` (aliases)
- `--no-parallel-methods`
- `--no-serve`
- `--no-cache`
- `--no-watch`

Inconsistent: some have `--no-` prefix, `--quiet` is an alias. The `--progress` flag
has both `--progress` (on), `--no-progress` (off), and `--progress-json` (alternate
mode). This is a lot of flags for one concept.

A more conventional approach: `--progress=(auto|line|json|off)` with `--no-progress`
as a deprecated alias.

---

## Priority Matrix Summary

| Priority | Area | Effort | File Impact |
|----------|------|--------|-------------|
| **P0** | Shell completion first cut | Small | 1 new file |
| **P0** | `--help` for every command first cut | Medium | 2 files |
| **P1** | Shared flag parser | Medium | 1 new package, all command files |
| **P1** | `--json` on `report` (`version`/`doctor` landed) | Small | 1 file |
| **P1** | `glade init` scaffolding | Medium | 1 new file |
| **P1** | Config validation and show commands | Small-Medium | 1 new file |
| **P2** | Watch/daemon management commands | Small-Medium | 2 files |
| **P2** | SARIF output for `check` | Small | 1 file |
| **P2** | `glade coverage` | Medium | 1 new file |
| **P2** | `glade db query` | Medium | 1 file |
| **P2** | CI integration (`hook`, `ci` formats) | Small-Medium | 2 files |
| **P2** | Progress on `parse` and `package` | Small | 2 files |
| **P3** | `--color` / `NO_COLOR` explicit | Small | 1 file |
| **P3** | Levenshtein suggestions for unknown flags | Small | Shared flag parser |
| **P3** | Diagnostic grouping in `check` output | Medium | 2 files |
| **P3** | `glade lint` / `glade explain` | Medium | 1-2 new files |
| **P4** | `glade bench` | Small-Medium | 1 new file |
| **P4** | `glade fmt` / `glade docs` / `glade graph` | Large | 3+ new files |
| **P4** | HTML test reports | Medium | 1 file |
| **P5** | `glade hook install` | Small | 1 new file |
| **P5** | `glade package validate/info/diff` | Medium | 1 file |
| **P5** | `glade db dump/migrate` | Medium | 1 file |
| **P5** | `glade explain <code>` | Small | 1 new file |

Assessed by reading every command file, the config/project/cliui/watch/testdaemon
packages, and testing the CLI surface manually.
