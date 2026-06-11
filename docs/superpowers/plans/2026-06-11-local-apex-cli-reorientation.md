# Local Apex CLI Reorientation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reset the local Apex CLI product roadmap after the recent DX, plugin, CI, report, package, and daemon work.

**Architecture:** Keep base `glade` focused on local runtime, test execution, project setup, report artifacts, trace/profile analysis, storage, editor/server/playground surfaces, and plugin hosting. Move maintenance scanners, support ledgers, advisory performance scans, broad security scans, docs inventory, corpora readiness, and compatibility fixture runners into first-party plugins. Build the next core phase around evidence that comes from local execution.

**Tech Stack:** Go 1.26, existing `internal/gladecli`, `internal/cliui`, `internal/flagparse`, `internal/testreport`, `internal/profile`, `internal/storage`, `internal/apextest`, `internal/testdaemon`, `internal/pluginhost`, `internal/diagnostic`, and existing docs under `docs/` and `site/docs-src/`.

---

## Current Landscape

Use this file instead of the original broad phase plan when choosing the next work packet. The old file remains useful history:

- `docs/superpowers/plans/2026-06-11-local-apex-cli-product-phases.md`

The current binary shows these product command roots:

```text
version, doctor, config, init, parse, inspect, schema, check, exec, debug,
editor, dap, test, dev, report, lsp, profile, plugins, package, server,
playground, db, completion, help
```

Recently completed or moved:

- Shell completion exists: `glade completion bash|zsh|fish`.
- Command help exists for the broad command surface.
- `version --json` and `doctor --json` exist.
- `config show`, `config validate`, `config init`, and top-level `init` exist.
- `check --format sarif|github` and `--output` exist.
- `report show latest --json`, `report github latest`, and HTML report export exist.
- `test daemon status|stop`, `test changed`, `test failed`, `--last-failed`, `--wizard`, exact class/method flags, class-file input, sharding, and duration history exist.
- `package info`, `package validate`, and `package diff` exist.
- `parse`, `package build`, and `db seed` use the shared progress renderer.
- Plugin host and marketplace commands exist under `glade plugins`.
- `@glade/compat` and `@glade/performance` are now the first-party plugin shape for maintenance and advisory scanning.
- `internal/perfscan` is no longer a core package. Do not rebuild it in base Glade.

Focused checks run during this reorientation:

```bash
go run ./cmd/glade --help
go run ./cmd/glade exec --help
go run ./cmd/glade config --help
go run ./cmd/glade init --help
go run ./cmd/glade completion zsh
go run ./cmd/glade version --json
go run ./cmd/glade doctor --json
go run ./cmd/glade test daemon status --project .
go run ./cmd/glade config show --project . --json
go test ./internal/gladecli -run 'TestPlugins(Available|SearchWithout|Help)' -count=1
go test ./internal/diagnostic -run 'TestWrite(SARIF|GitHub)' -count=1
go test ./internal/pluginhost -count=1
go test ./internal/testdaemon -count=1
```

The broad `glade check --project . --format sarif` smoke was stopped after it ran too long for orientation. Do not treat that command as a current pass/fail signal.

Current dirty checkout note:

- `internal/gladecli/cli_test.go` has user changes for plugin marketplace availability/search/help tests. Do not overwrite them.

## Product Boundary

Keep in base Glade:

- Local Apex parse, semantic checks, runtime, tests, SOQL, DML, triggers, limits, storage, schema, local server, playground, LSP/DAP, debug-log parsing, trace/profile analysis, saved reports, plugin host, and core project setup.

Keep in first-party plugins:

- Compatibility fixtures, surface ledger, corpus readiness scans, generated support reports, docs inventory, advisory performance scans, advisory security scans, broad AI-code-review scans, and parity dashboards.

This changes the old roadmap:

- Do not add `glade inspect security` to base Glade as a broad static scanner.
- Do not add a new core performance scanner.
- Do not add `compat` or `surface` code to base Glade. Those roots are plugin-owned when installed.
- Do not add a new `apexlocal` binary.

## New Phase Order

Work Phase 1 to completion before starting Phase 2. Work Phase 2 to completion before starting Phase 3.

Phase 1: core execution evidence.

Phase 2: policy, CI wiring, and team workflows.

Phase 3: shape, fixture, bulk, and polish.

Each phase can use parallel subagents by area. One integration captain merges branches and runs the phase gate.

## Phase 1: Core Execution Evidence

Phase 1 replaces the old Phase 1. The old discoverability and setup pieces are already done. The next product value is making local execution explain itself.

### Phase 1 Gate

```bash
go test ./internal/gladecli ./internal/testreport ./internal/profile ./internal/storage ./internal/apextest -count=1
go run ./cmd/glade test --help
go run ./cmd/glade report --help
go run ./cmd/glade profile --help
go run ./cmd/glade db --help
git diff --check
```

Add new command smoke checks as each area lands.

### Area 1A: Failure Packets And Fidelity

**Owner:** report/test subagent.

**Purpose:** Make failed local runs explain the cause, location, next command, and local-fidelity level.

**Files:**

- Create: `internal/testreport/explain.go`
- Create: `internal/testreport/fidelity.go`
- Modify: `internal/testreport/model.go`
- Modify: `internal/testreport/reporters.go`
- Modify: `internal/testreport/reporters_test.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/gladecli/cli_test.go`

**Work:**

- Add `FailurePacket` to `testreport.Case`.
- Add `Fidelity` to `testreport.Run`.
- Derive the first fidelity level from existing result data: unsupported count, compile errors, runtime errors, and problem type.
- Print one failure packet in console output after the first failing case.
- Preserve raw `Problem`, `Trace`, and `Profile` JSON.

**Tests to add:**

- `TestFailurePacketIncludesPrimaryFrame`
- `TestFailurePacketBuildsNextCommandForMethod`
- `TestFidelityHighForCleanRun`
- `TestFidelityMediumForUnsupportedRun`
- `TestFidelityLowForRuntimeSetupFailure`

**Focused command:**

```bash
go test ./internal/testreport ./internal/gladecli -run 'Test(FailurePacket|Fidelity)' -count=1
```

### Area 1B: Coverage From Local Trace Evidence

**Owner:** coverage subagent.

**Purpose:** Report line coverage and changed-line coverage from local test execution.

**Files:**

- Create: `internal/coverage/model.go`
- Create: `internal/coverage/analyze.go`
- Create: `internal/coverage/report.go`
- Create: `internal/coverage/coverage_test.go`
- Create: `internal/gladecli/coverage_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`
- Modify: `internal/apextest/runner.go`
- Modify: `internal/testreport/model.go`

**Command:**

```bash
glade coverage --project <root> [--json] [--changed-since <ref>]
```

**Work:**

- Reuse trace events that already carry source line and file data.
- Report per-file executable lines, covered lines, and uncovered lines.
- Add changed-line coverage by intersecting `watch.GitChangesSince` with coverage data.
- Add coverage-by-test only after line and changed-line reports pass.

**Tests to add:**

- `TestCoverageCountsExecutedLines`
- `TestCoverageChangedLinesUsesGitDiff`
- `TestRunCoverageJSON`
- `TestRunCoverageText`

**Focused command:**

```bash
go test ./internal/coverage ./internal/apextest ./internal/gladecli -count=1
```

### Area 1C: DB Query And Exec Dry-Run Diff

**Owner:** storage/runtime subagent.

**Purpose:** Let users inspect local org data and preview anonymous Apex mutations without saving them.

**Files:**

- Create: `internal/storage/diff.go`
- Create: `internal/storage/diff_test.go`
- Modify: `internal/gladecli/db_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/vm/runtime_state.go` only if a public setter/getter is needed for existing org state.

**Commands:**

```bash
glade db query --db <path> [--project <root>] [--json] "<SOQL>"
glade exec --project <root> --db <path> --dry-run --data-diff "<anonymous apex>"
```

**Work:**

- Add `storage.Diff(before, after OrgState) DiffSummary`.
- Implement `db query` through the existing local SOQL/runtime path.
- Add `exec --db` to run anonymous Apex against a persistent local org state.
- Add `exec --dry-run` so mutations are not saved.
- Add `exec --data-diff` so inserted, updated, and deleted record counts print in text and JSON.

**Tests to add:**

- `TestStorageDiffSummarizesInsertedUpdatedDeleted`
- `TestRunDBQueryJSON`
- `TestRunDBQueryText`
- `TestRunExecDryRunDoesNotPersistDBChanges`
- `TestRunExecDryRunPrintsDataDiff`

**Focused command:**

```bash
go test ./internal/storage ./internal/gladecli ./internal/vm -run 'Test(StorageDiff|RunDBQuery|RunExecDryRun)' -count=1
```

### Area 1D: Trace/Profile Diff

**Owner:** profile subagent.

**Purpose:** Compare two local traces without adding a new broad top-level command.

**Files:**

- Create: `internal/profile/diff.go`
- Create: `internal/profile/diff_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`

**Command:**

```bash
glade profile diff <before-trace.json> <after-trace.json> [--json]
```

**Work:**

- Compare `profile.Report` output, not raw trace events.
- Report changed SOQL, DML, async, callout, email, CPU, heap, and hot span counts.
- Text output should show only changed rows by default.
- JSON output should include before, after, and delta fields.

**Tests to add:**

- `TestProfileDiffShowsLimitDeltas`
- `TestProfileDiffShowsNewHotSpan`
- `TestRunProfileDiffJSON`
- `TestRunProfileDiffText`

**Focused command:**

```bash
go test ./internal/profile ./internal/gladecli -run 'Test(ProfileDiff|RunProfileDiff)' -count=1
```

## Phase 2: Policy, CI Wiring, And Team Workflows

Phase 2 turns Phase 1 evidence into repo and CI decisions. Do not start broad scanners here. Use local run data first.

### Phase 2 Gate

```bash
go test ./internal/gladecli ./internal/testreport ./internal/diagnostic ./internal/watch ./internal/config -count=1
go run ./cmd/glade report --help
go run ./cmd/glade check --help
go run ./cmd/glade test --help
git diff --check
```

### Area 2A: Repo Policy

**Files:**

- Create: `internal/policy/model.go`
- Create: `internal/policy/load.go`
- Create: `internal/policy/check.go`
- Create: `internal/policy/policy_test.go`
- Create: `internal/gladecli/policy_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`

**Command:**

```bash
glade policy check --project <root> --changed-since <ref> --json
```

**Work:**

- Read `.glade/policy.yml`.
- Map changed file families to required commands.
- Include fidelity warnings from latest reports when present.
- Print exact commands an agent or developer must run.

### Area 2B: Repair Packet Export

**Files:**

- Modify: `internal/testreport/explain.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/gladecli/cli_test.go`

**Command:**

```bash
glade report repair-packet latest --json
```

**Work:**

- Use Phase 1 failure packets.
- Include failing test, primary location, relevant files, next command, fidelity, and raw report path.
- Do not invent code fixes.

### Area 2C: CI Scaffolding

**Files:**

- Create: `internal/gladecli/ci_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`
- Modify: `site/docs-src/guide/ci-artifacts.md`

**Commands:**

```bash
glade ci init --project <root> --github-actions --output <path>
glade check --format ci
glade test --format ci
```

**Work:**

- Generate a small GitHub Actions workflow with `glade check`, `glade test --json`, and SARIF upload instructions.
- Add one-line CI formats after JSON/JUnit/SARIF remain unchanged.

### Area 2D: Diagnostic Display And Color Control

**Files:**

- Modify: `internal/diagnostic/report.go`
- Modify: `internal/cliui/diagnostic.go`
- Modify: `internal/cliui/theme.go`
- Modify: `internal/gladecli/cli.go`

**Work:**

- Add grouped diagnostic text by file and severity.
- Add `--max-diagnostics <n>` to `check`.
- Add `--color auto|always|never` for console commands with styled output.

## Phase 3: Shape, Fixture, Bulk, And Polish

Phase 3 is for deeper runtime confidence. It should not compete with plugin maintenance scanners.

### Area 3A: Fixture Validation

**Command:**

```bash
glade fixtures validate --project <root> --fixture <path> --json
```

**Work:**

- Validate fixture object names, field names, field reference targets, required relationship aliases, record type IDs, and picklist values already known to local schema.
- Reuse `storage.ReadFixture` and `storage.ApplyFixture` semantics.

### Area 3B: Shape Show And Diff

**Commands:**

```bash
glade shape show --project <root> --json
glade shape diff --left <shape.json> --right <shape.json> --json
```

**Work:**

- Represent modeled, approximated, stubbed, and unsupported shape items.
- Feed shape fidelity into `testreport.Fidelity` only after Phase 1 fidelity lands.

### Area 3C: Bulk Projection

**Command:**

```bash
glade profile bulk --trace <trace.json> --records 1,10,50,200 --json
```

**Work:**

- Use measured SOQL/DML counts and source loop evidence.
- Report projection confidence. Do not fake CPU precision.

### Area 3D: Flaky And Random Order

**Commands:**

```bash
glade test --order random --seed <n>
glade report flaky --project <root> --json
```

**Work:**

- Keep default order stable.
- Store seed and recent outcomes in `.glade/runs`.
- Report tests with mixed outcomes across recent runs.

## Deferred Or Dropped From Core

Keep these out of the next core phase:

- Broad security scan in base Glade. Put advisory security scanning in a plugin.
- Broad performance scanner in base Glade. Use `@glade/performance`.
- Reintroducing `compat`, `surface`, or ledger code into base Glade.
- `glade test daemon restart` until there is a real background start model.
- `glade fmt`, `glade docs`, `glade graph`, and `glade bench` until the evidence loop is better.
- A new TUI or heavy editor UI before JSON/report contracts settle.

## Suggested Next Parallel Run

Start Phase 1 with four subagents:

- Report/test worker: Area 1A.
- Coverage worker: Area 1B.
- Storage/runtime worker: Area 1C.
- Profile worker: Area 1D.

Use separate worktrees. Merge in this order:

1. Area 1D profile diff.
2. Area 1A failure/fidelity.
3. Area 1B coverage.
4. Area 1C DB query and dry-run diff.

Area 1C touches the runtime and should merge last.

## Closeout Rule

At the end of Phase 1, write:

```text
docs/superpowers/plans/2026-06-11-local-apex-cli-reorientation-phase1-closeout.md
```

Include:

- branches merged
- commands run
- exact pass/fail output summary
- known risks
- recommended Phase 2 first packet
