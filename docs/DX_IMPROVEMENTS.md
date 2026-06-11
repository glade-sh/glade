# Glade CLI DX Improvements — Deep Dive

Analysis date: 2026-06-10. Based on full source review of `cmd/`, `internal/gladecli/`,
`internal/cliui/`, `internal/config/`, `internal/project/`, and related packages.

## Current CLI Surface (23 top-level commands)

```
version  doctor  completion config init    parse   inspect schema check
exec     debug   editor     dap    test    dev     report  lsp    profile
package  server  playground db     help
```

---

## 1. Shell Completion (P0 — High Impact)

For a CLI with 23 top-level commands and dozens of flags, shell completion was the
single highest-ROI DX gap. The first cut is now in place.

**Current state:** Phase 1 implemented: `glade completion bash|zsh|fish`
generates scripts from a shared command catalog. Terminal users can now complete
top-level commands, subcommands, and long flags. `docs/INSTALL.md` and
`glade help completion` include install snippets.

**Remaining polish:** add smarter value completion for project paths and enum-like
values, and keep command metadata in sync as more command internals move to the
shared flag parser.

**Files touched:** `internal/gladecli/completion_command.go` (new), `internal/gladecli/cli.go`.

---

## 2. Help System (P0 — High Impact)

The help system had been largely absent. Phase 1 implemented: `glade <command>
--help`, `glade <subcommand> --help`, and `glade help <command>` now use the
shared command catalog. `glade help test` keeps its deeper hand-written reference.
`glade help exit-codes` now documents the process exit code contract. Before
that, every other command that received unknown flags or no args printed a bare
one-line usage error with no flag descriptions:

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
- Generate help directly from flag definitions instead of keeping metadata in a
  separate command catalog.

**Files touched:** `internal/cliui/help.go`, `internal/gladecli/cli.go`.

---

## 3. Flag Parsing Boilerplate (P1 — Medium Impact, Internal Quality)

Before Phase 1, every command manually parsed flags with index-based loops. This
pattern was repeated verbatim across 9 files covering roughly 500 lines of
near-identical code:

```go
// Older manual pattern, repeated across command files
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

The `parseProjectFlags` and `parseProjectProgressFlags` helpers are partial
solutions that only cover `--project` and `--json`/`--progress`.
They don't address per-command flags like `--filter`, `--changed-since`, `--addr`,
`--limit-mode`, `--watch`, etc.

**Current state:** Phase 1 added `internal/flagparse`, with bool/string flags,
short aliases, `--` end-of-options handling, and Levenshtein suggestions. It now
backs the common project flag helpers plus high-traffic commands such as
`version`, `doctor`, `parse`, `inspect performance`, `schema load`, `check`,
`exec`, `test`, `lsp`, `profile analyze`, and `package build`. Leaf commands
that still have hand loops get typo suggestions at the `Run` error formatter.

**Remaining polish:** migrate the remaining leaf command loops (`dev`, `debug`,
`editor`, `db`, `server`, and `playground`) when those files next move, then
derive command help from parser registrations instead of duplicate metadata.

**Files touched:** `internal/flagparse/`, `internal/gladecli/cli.go`,
`internal/gladecli/test_command.go`.

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
| `report` | Yes | `show latest --json`; GitHub and HTML outputs also landed |
| `lsp` | No | JSON protocol already |
| `profile analyze` | Yes | |
| `package build/info/validate/diff` | Yes | Build writes artifact JSON; info, validate, and diff also support `--json` |
| `server` | No | Not applicable |
| `playground` | No | Not applicable |
| `db` | Yes | inspect/export |
| `dev` | No | Human-oriented by design |

Remaining key gap:
- Add structured output to `editor doctor` if editor integrations need agent-readable status.

**Files touched:** `internal/gladecli/cli.go` (version, doctor), `internal/gladecli/dev_command.go` (report).

---

## 5. Progress Indicators (P2 — Medium Impact)

Phase 5 wired the existing progress renderer into the slow local workflow commands
that have bounded work:
- `check` — 4-phase progress (load project, load metadata, index, semantic checks)
- `schema load` — 2-phase progress
- `test` — rich progress with running count, per-test status, slow-test warnings
- `parse` — Apex file discovery and per-file parse ticks
- `package build` — project load, metadata load, symbol index, artifact write
- `db seed` — fixture read, apply, save

The progress infrastructure in `cliui/` already supports modes: auto (TTY), line,
JSON-NDJSON, and off. Keep new slow commands on that renderer instead of
inventing one-off progress text.

**Files touched:** `internal/gladecli/cli.go` (parse, package), `internal/gladecli/db_command.go`,
`docs/RICH_LOCAL_WORKFLOWS.md`.

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

**Current state:** Phase 2 implemented:
- `glade config validate` validates `glade.yml` syntax with proper error reporting.
- `glade config show` prints resolved config layered with `sfdx-project.json`.
- `glade config init` and `glade init` scaffold `glade.yml` from inferred SFDX
  package directories and namespace, with a terminal prompt path plus repeatable
  flags for package dirs and org features.

**Remaining polish:** run `doctor` after init, offer next-command probes, and add
config-backed defaults for common `test` and `check` flags.

**Missing config keys:**
- `test.timeout` — default per-test timeout
- `test.parallelism` — default worker count
- `test.watchBackend` — native/poll/auto
- `test.debounce` — watch debounce interval
- `test.limitMode` — strict/permissive
- `check.ignore` — patterns or rule codes to suppress in `check`

Currently, these are all CLI-only flags. A project can't express "this test suite needs
`--no-parallel-methods`" except through shell wrappers or Makefiles.

The inline list format `[a, b, c]` is now documented in `docs/CONFIG.md`.

**Files touched:** `internal/gladecli/config_command.go`, `internal/gladecli/cli.go`,
`internal/cliui/help.go`, `docs/CONFIG.md`, `docs/INSTALL.md`.

---

## 7. Watch/Test Daemon UX (P2 — Medium Impact)

Two overlapping watch paths exist:
1. `glade test --watch` / `glade test --daemon` — emits NDJSON events to stdout
2. `glade dev watch` — human-friendly output, writes run artifacts, supports
   `--failed` and `--changed`

The daemon socket (`glade test serve`) auto-connects from subsequent `glade test`
runs through `tryTestServerRun()` in `test_serve_command.go`.

**Current state:** Phase 3 implemented the daily loop surface:
- `glade test daemon status` shows stopped, stale, warming, or ready state.
- `glade test daemon stop` shuts down a reachable server and removes stale socket
  or pid files.
- `glade test changed --since HEAD` wraps the existing affected-test path.
- `glade test failed` and `glade test --last-failed` rerun failed tests recorded
  by the last completed run.
- `glade test --wizard` prints cache, daemon, and next-command suggestions without
  hiding the exact command.
- Progress output now includes a startup cache hint.

**Remaining polish:** `glade test daemon restart` should wait for a real process
supervisor or background-start story. `glade test serve` remains the explicit
start command.

**Unify dev test and test watch:**
`dev test --changed` and `test --changed-since HEAD` are semantically identical
but implemented as separate code paths. The `changedSinceSelection` function
lives in `dev_command.go` but is called from `test_command.go`. Consolidate into
`internal/watch/`.

**Files touched:** `internal/gladecli/test_command.go`, `internal/gladecli/dev_command.go`,
`internal/testdaemon/`.

---

## 8. New Command Opportunities

### 8a. `glade init` (Done — Medium Effort)

Bootstraps `glade.yml` from an existing project. The command infers package
directories and namespace from `sfdx-project.json`, prompts at a terminal,
accepts repeatable `--package-dir` and `--feature` flags, and refuses to
overwrite unless `--force` is set.

```
glade init --project . --yes
glade config init --project . --yes --package-dir force-app --feature PersonAccounts
glade config validate --project .
glade config show --project . --json
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

## 9. First Cut Completed

The first DX cut was a shared command catalog that feeds:

- `glade <command> --help`
- `glade help <command>`
- `glade completion bash|zsh|fish`

That move made the CLI easier to discover without changing runtime behavior.
It gives wizard work a single place to read command names, flags, subcommands,
and examples.

This cut should stay product-only. Do not add completions or help topics for
maintenance surfaces that moved to `glade-tools`.

## 10. Wizard Ideas That Fit Glade

### 10a. `glade init` wizard (Done)

`glade init` now infers package dirs and namespace from `sfdx-project.json`,
prompts in a terminal, and accepts CI-safe flags:

```bash
glade init --project . --yes
glade init --project . --namespace pkg --package-dir force-app --force
```

### 10b. `glade test --wizard` (Done)

Help developers pick a fast local test path.

The wizard inspects existing `.glade/test/startup.gob` and a reachable test
server socket. Then it prints command choices:

- changed files: `glade test changed --since HEAD`
- last failure: `glade test --last-failed`
- warm loop: `glade test serve --project .`
- stubborn cache: `glade test clear-cache --project .`

It does not run a command. The user sees the exact command first.

### 10c. `glade db seed --wizard` (Done)

Good for local app developers who want a working org state.

Phase 5 added a command printer for seed runs:

```bash
glade db seed --wizard --db .glade/org.sqlite --project . fixtures/dev.json
```

It prints the seed command with progress enabled and the follow-up inspect
command for the same database. It does not mutate the database.

### 10d. `glade playground --wizard` (Done)

Good for demos and support.

Phase 5 added a command printer that preserves the selected project, workspace,
data root, project refs, examples, public mode, limits, and browser choice:

```bash
glade playground --wizard --project . --examples
```

Keep this behind the existing `playground` command. A separate top-level TUI
would add weight before the normal CLI surface is finished.

## 11. More DX Ideas

- `glade doctor --fix-suggestions`: print concrete next commands for missing
  config, parser-disabled builds, and stale test cache.
- `glade explain <error-code>`: describe `GLADE*` diagnostics with examples and
  likely fixes.
- `glade test daemon restart`: add a true background restart once daemon start is
  process-managed instead of foreground `serve`.
- `glade report open latest`: open the latest HTML/Markdown report when one
  exists.

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

**Current state:** Phase 4 implemented:
- **SARIF** — Static Analysis Results Interchange Format. Standard format supported
  by GitHub, GitLab, VS Code. `glade check --format sarif --output results.sarif`
  writes code-scanning output.
- **GitHub Actions annotations** — `glade check --format github` emits semantic
  diagnostics, and `glade report github latest` emits latest saved-run failures.
- **Report JSON** — `glade report show latest --json` combines `latest.json`,
  `run.json`, and `results.json`.
- **HTML report** — `glade report export latest --format html --output report.html`
  writes a single-file saved-run report for CI artifacts.

**Remaining polish:**
- **TAP** — Test Anything Protocol. Common in CI/CD pipelines.
  `glade test --format tap`

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

**Current state:** unknown command and flag suggestions are in place for the
shared command catalog and high-traffic parser-backed commands.

**Remaining polish:**
- Derive all help and suggestions from parser registrations so leaf command loops
  cannot drift from the command catalog.
- Group diagnostics by file and severity in human output.

Historical problem:
```
$ glade test --filteer AccountServiceTest
glade: unknown flag "--filteer"
```

**Usage messages are too terse:**
```
usage: glade parse <paths...> [--json]
```
Doesn't explain that `<paths...>` accepts files or directories, or that directories
are walked recursively for `.cls` and `.trigger` files.

Exit codes are now documented under `glade help exit-codes`.

---

## 15. CI/CD Integration (P2 — Medium Impact)

Phase 4 added first-class CI outputs: JSON check output, SARIF, GitHub
annotations, saved test runs, and HTML report export. Remaining polish is
workflow scaffolding and shorter summary modes:

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

Phase 5 filled out the artifact loop:
- `glade package info <artifact.json>` prints namespace, version, source hash, and counts.
- `glade package validate <artifact.json>` checks artifact shape before publishing or installing.
- `glade package diff <from.json> <to.json>` compares two artifact versions.

All three support `--json`.

---

## 17. DB Command Depth (P2-P3 — Medium Impact)

`glade db` covers seed, reset, export, and inspect. Missing:
- `glade db query "SELECT Id, Name FROM Account WHERE Name LIKE 'A%'"` — raw SOQL against the local database
- `glade db dump` — full table dump for debugging
- `glade db migrate` — schema version migration support (schema version already tracked)

---

## 18. Version Command (P1 — Small Impact)

`glade version` now supports `--json`. The in-code default remains
`"0.0.0-dev"`, and `scripts/build-local.sh` plus `scripts/release-build.sh`
stamp real versions with Go `-ldflags`.

**Current:**
```
$ glade version --json
{"version":"v0.7.2","go":"go1.23.4","os":"darwin","arch":"arm64"}
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
| **Done** | Phase 1 discoverability: completion install docs, deeper help, shared flag parser, typo suggestions | Medium | CLI/help/docs |
| **Done** | `--json` on `report`, `version`, and `doctor` | Small | CLI/report |
| **Done** | Phase 2 project setup: `config show`, `config validate`, `config init`, `glade init` | Medium | CLI/help/docs |
| **Done** | Phase 3 daily test loop: daemon status/stop, changed alias, last failed, wizard, cache hints | Medium | CLI/help/docs |
| **Done** | Phase 4 CI and artifacts: report JSON, GitHub annotations, SARIF, HTML report export | Medium | CLI/help/docs |
| **Done** | Phase 5 rich local workflows: progress on parse/package/db seed, DB seed wizard, playground wizard, package info/validate/diff | Medium | CLI/help/docs |
| **P2** | `glade coverage` | Medium | 1 new file |
| **P2** | `glade db query` | Medium | 1 file |
| **P2** | CI integration (`hook`, `ci` formats) | Small-Medium | 2 files |
| **P3** | `--color` / `NO_COLOR` explicit | Small | 1 file |
| **P3** | Diagnostic grouping in `check` output | Medium | 2 files |
| **P3** | `glade lint` / `glade explain` | Medium | 1-2 new files |
| **P4** | `glade bench` | Small-Medium | 1 new file |
| **P4** | `glade fmt` / `glade docs` / `glade graph` | Large | 3+ new files |
| **P5** | `glade hook install` | Small | 1 new file |
| **P5** | `glade db dump/migrate` | Medium | 1 file |
| **P5** | `glade explain <code>` | Small | 1 new file |

Assessed by reading every command file, the config/project/cliui/watch/testdaemon
packages, and testing the CLI surface manually.
