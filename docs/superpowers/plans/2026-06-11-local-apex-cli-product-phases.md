# Local Apex CLI Product Phases Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the local Apex CLI product plan into three ordered implementation phases, with parallel work lanes inside each phase.

**Architecture:** Keep product commands in `glade`. Keep maintenance scanners, ledgers, and compatibility fixture workflows out of this repository's public CLI. Phase 1 builds the handles and first proof surfaces. Phase 2 adds measured evidence and agent policy. Phase 3 adds deeper enterprise/runtime behavior.

**Tech Stack:** Go, existing `internal/gladecli`, `internal/cliui`, `internal/config`, `internal/testdaemon`, `internal/testreport`, `internal/profile`, `internal/debuglog`, `internal/storage`, the first-party performance plugin, and the existing CLI test harness.

---

## Current State

The checkout has an untracked `docs/DX_IMPROVEMENTS.md`. Treat it as user work. Do not edit, delete, stage, or reformat it unless the user says so.

The useful existing pieces are already here:

- `glade test` has `--filter`, `--changed-since`, `--watch`, `--daemon`, `--json`, `--junit`, progress, warm server, startup cache, and timeout flags.
- `glade exec` runs anonymous Apex and can write traces and debug logs.
- `glade debug` parses, profiles, explains, and synthesizes from Salesforce debug logs.
- `glade dev` writes run artifacts under `.glade/runs`.
- `glade report` lists and exports saved run reports.
- `glade db` seeds, resets, exports, and inspects local org storage.
- The first-party performance plugin owns performance scan style findings.

Do not add an `apexlocal` binary. The product surface is `glade`.

Do not add `compat` or surface-ledger commands back to core Glade. They ship through the first-party compat plugin.

## Phase Order

Work Phase 1 to completion before Phase 2. Work Phase 2 to completion before Phase 3.

Phase 1 is the cabin floor: command handles, onboarding, daemon control, report packets, and local data inspection. It should make the tool easier to call by humans and agents without changing deep runtime semantics.

Phase 2 is the bench and shelves: coverage, SARIF, PR reports, AI policy, trace diffing, changed security scans, randomized order, and flaky detection. It turns existing evidence into structured proof.

Phase 3 is the tight joinery: fixture validation, bulk projection, shape diffing, HTML reports, editor-facing views, confidence scoring, and command polish. It should only start after the first two phases prove stable.

## Parallel Work Model

Use one subagent per area inside a phase.

Each subagent starts with:

```bash
git status --short
go test ./internal/gladecli ./internal/cliui ./internal/config ./internal/testdaemon ./internal/testreport ./internal/storage ./internal/profile ./internal/debuglog -count=1
```

If the baseline is red, record the failing package and error. Do not fix unrelated failures.

For parallel execution, use separate worktrees or isolated `codex/` branches:

```bash
git worktree add /tmp/glade-phase1-cli -b codex/phase1-cli .
git worktree add /tmp/glade-phase1-config -b codex/phase1-config .
git worktree add /tmp/glade-phase1-daemon -b codex/phase1-daemon .
git worktree add /tmp/glade-phase1-report -b codex/phase1-report .
git worktree add /tmp/glade-phase1-runtime -b codex/phase1-runtime .
```

One integration captain owns final merges for each phase. That worker resolves shared edits in `internal/gladecli/cli.go`, runs the phase gate, and writes the phase closeout note.

## Phase 1 Exit Gate

Phase 1 is complete when all of these pass:

```bash
go test ./internal/gladecli ./internal/cliui ./internal/config ./internal/testdaemon ./internal/testreport ./internal/storage ./internal/profile -count=1
go run ./cmd/glade --help
go run ./cmd/glade exec --help
go run ./cmd/glade completion zsh >/tmp/glade-completion.zsh
go run ./cmd/glade version --json
go run ./cmd/glade doctor --json
go run ./cmd/glade config validate --project .
go run ./cmd/glade config show --project . --json
go run ./cmd/glade test daemon status --project . || true
git diff --check
```

`glade test daemon status --project .` may exit nonzero when no daemon is running. It must print a clear status, not a stack trace.

## Phase 1 Areas

### Area A: CLI Discoverability and Machine Output

**Owner:** Phase 1 CLI subagent.

**Purpose:** Make every command self-describing and add small JSON surfaces that agents can trust.

**Files:**

- Create: `internal/gladecli/completion_command.go`
- Create: `internal/gladecli/completion_command_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/cliui/help.go`
- Modify: `internal/cliui/cliui_test.go`

**Commands added or changed:**

- `glade completion bash|zsh|fish`
- `glade version --json`
- `glade doctor --json`
- `glade help <command>`
- `glade <command> --help` for every top-level command

**Steps:**

- [ ] Add failing CLI tests:
  - `TestRunExecHelpPrintsUsage`
  - `TestRunDebugHelpPrintsUsage`
  - `TestRunDBHelpPrintsUsage`
  - `TestRunCompletionZsh`
  - `TestRunCompletionRejectsUnknownShell`
  - `TestRunVersionJSON`
  - `TestRunDoctorJSON`

- [ ] Verify the first test cut fails:

```bash
go test ./internal/gladecli -run 'TestRun(ExecHelp|DebugHelp|DBHelp|CompletionZsh|VersionJSON|DoctorJSON)' -count=1
```

Expected before implementation: failures for missing help or completion behavior.

- [ ] Add `runCompletion(args []string, w io.Writer) error` in `internal/gladecli/completion_command.go`.

The first implementation may use a static command and flag registry. Do not build a full flag parser in Phase 1.

- [ ] Split version handling out of the `Run` switch into `runVersion(args []string, w io.Writer) error`.

JSON shape:

```json
{
  "version": "0.0.0-dev",
  "go": "go1.x",
  "os": "darwin",
  "arch": "arm64"
}
```

- [ ] Add `--json` support to `runDoctor`.

Use the existing doctor data already gathered for text output. The JSON must include parser status and project root when available.

- [ ] Extend `printHelpTopic` so every top-level command has a useful help writer.

Keep `WriteTestHelp` as-is. Add command-specific help text for `exec`, `debug`, `db`, `dev`, `report`, `editor`, `server`, `playground`, `package`, `profile`, `inspect`, `schema`, `check`, `parse`, `doctor`, `version`, and `completion`.

- [ ] Update command handlers so `glade <command> --help` exits zero and does not execute the command body.

- [ ] Run:

```bash
go test ./internal/gladecli ./internal/cliui -count=1
go run ./cmd/glade exec --help
go run ./cmd/glade completion zsh >/tmp/glade-completion.zsh
```

Expected: tests pass; `exec --help` prints usage; completion output is nonempty.

**Acceptance Notes:**

- No command should treat `--help` as source Apex, a fixture path, a log path, or a project path.
- Completion output can be basic. It must include top-level commands and common flags.
- Do not rewrite all flag parsing in Phase 1.

### Area B: Init and Config

**Owner:** Phase 1 config subagent.

**Purpose:** Give users and agents a clean way to create and inspect project configuration.

**Files:**

- Create: `internal/gladecli/init_command.go`
- Create: `internal/gladecli/config_command.go`
- Create: `internal/gladecli/init_config_command_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `docs/INSTALL.md`
- Modify: `site/docs-src/guide/installation.md`
- Modify: `site/docs-src/guide/cli-reference.md`

**Commands added:**

- `glade init --project <root> --output <path>`
- `glade config validate --project <root>`
- `glade config show --project <root> --json`

**Steps:**

- [ ] Add failing tests:
  - `TestRunInitWritesGladeYML`
  - `TestRunInitRefusesOverwriteWithoutForce`
  - `TestRunConfigValidateAcceptsGeneratedFile`
  - `TestRunConfigShowJSONIncludesPackageDirs`

- [ ] Verify failing tests:

```bash
go test ./internal/gladecli ./internal/config -run 'TestRun(Init|Config)|TestConfig' -count=1
```

Expected before implementation: missing command failures.

- [ ] Implement `glade init`.

Behavior:

- Load the SFDX project if present.
- Write a valid `glade.yml`.
- Include package directories discovered from `sfdx-project.json`.
- Include `project.defaultNamespace` only when known.
- Refuse to overwrite an existing output file unless `--force` is present.
- Print the written path in text mode.
- Print the resolved config in JSON mode when `--json` is present.

- [ ] Implement `glade config validate`.

Behavior:

- Load `glade.yml` and project defaults.
- Return exit code 0 for valid config.
- Return exit code 1 with a file and line-oriented message for invalid YAML subset.

- [ ] Implement `glade config show --json`.

Behavior:

- Print the resolved project root, package directories, default namespace, managed package dependencies, and org features.
- Use stable key order through Go struct fields.

- [ ] Update docs with the new commands.

- [ ] Run:

```bash
go test ./internal/gladecli ./internal/config -count=1
tmp="$(mktemp -d)"
go run ./cmd/glade init --project . --output "$tmp/glade.yml"
go run ./cmd/glade config validate --project .
go run ./cmd/glade config show --project . --json
```

Expected: tests pass; init writes one file; config commands print clear output.

**Acceptance Notes:**

- Do not add interactive prompts in Phase 1. Noninteractive output is easier to test and safer for agents.
- Do not invent config keys that are not read by product code.

### Area C: Test Daemon Lifecycle

**Owner:** Phase 1 daemon subagent.

**Purpose:** Make the warm test server observable and controllable.

**Files:**

- Modify: `internal/gladecli/test_command.go`
- Modify: `internal/gladecli/test_serve_command.go`
- Modify: `internal/gladecli/test_serve_command_test.go`
- Modify: `internal/testdaemon/client.go`
- Modify: `internal/testdaemon/server.go`
- Modify: `internal/testdaemon/server_test.go`
- Modify: `internal/testdaemon/paths.go`
- Modify: `site/docs-src/guide/affected-tests.md`

**Commands added:**

- `glade test daemon status --project <root>`
- `glade test daemon stop --project <root>`
- `glade test daemon restart --project <root>`

**Steps:**

- [ ] Add failing tests:
  - `TestRunTestDaemonStatusNoServer`
  - `TestRunTestDaemonStatusRunningServer`
  - `TestRunTestDaemonStopRemovesSocketAndPID`
  - `TestRunTestDaemonRestartStopsThenStarts`

- [ ] Verify failing tests:

```bash
go test ./internal/gladecli ./internal/testdaemon -run 'TestRunTestDaemon|Test.*Shutdown' -count=1
```

Expected before implementation: missing command or usage failures.

- [ ] Reuse the existing `OpPing` and `OpShutdown` protocol.

Do not add a second daemon protocol. `client.go` already has `Ping` and `Shutdown`.

- [ ] Add status output.

Text shape:

```text
test daemon: stopped
project: /abs/path
socket: /abs/path/.glade/test/serve.sock
pid: none
```

Running shape:

```text
test daemon: running
project: /abs/path
ready: true
warming: false
socket: /abs/path/.glade/test/serve.sock
pid: 12345
```

- [ ] Add stale socket cleanup.

If the socket exists but `Ping` fails, status prints `stale`. Stop removes the stale socket and pid file.

- [ ] Add restart behavior.

Restart calls stop first. Then it starts the same server path used by `glade test serve`. In tests, use a context timeout and a temp project root.

- [ ] Run:

```bash
go test ./internal/gladecli ./internal/testdaemon -count=1
go run ./cmd/glade test daemon status --project . || true
```

Expected: tests pass; status is readable without a live daemon.

**Acceptance Notes:**

- Do not leave a daemon process running at the end of tests.
- Do not claim the runtime is warm unless the ping response says `ready: true`.

### Area D: Failure Packets, Fidelity Labels, and Report JSON

**Owner:** Phase 1 report subagent.

**Purpose:** Turn test failures into compact, stable packets and report the local-fidelity level honestly.

**Files:**

- Create: `internal/testreport/explain.go`
- Create: `internal/testreport/fidelity.go`
- Modify: `internal/testreport/model.go`
- Modify: `internal/testreport/reporters.go`
- Modify: `internal/testreport/reporters_test.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/gladecli/cli_test.go`

**Commands changed:**

- `glade test --json`
- `glade dev test`
- `glade report show latest --json`

**Steps:**

- [ ] Add failing tests:
  - `TestFailurePacketIncludesPrimaryFrame`
  - `TestFailurePacketDetectsMissingQueryRows`
  - `TestFidelityHighForModeledRun`
  - `TestFidelityMediumForUnsupportedCase`
  - `TestReportShowLatestJSON`

- [ ] Verify failing tests:

```bash
go test ./internal/testreport ./internal/gladecli -run 'Test(FailurePacket|Fidelity|ReportShowLatestJSON)' -count=1
```

Expected before implementation: missing fields or missing JSON option.

- [ ] Add `FailurePacket` to `testreport.Case`.

JSON shape:

```json
{
  "summary": "System.AssertException in AccountServiceTest.testCreates",
  "location": {"file": "force-app/main/default/classes/AccountServiceTest.cls", "line": 42},
  "likelyCause": "assertion failed",
  "nextCommand": "glade test --project . --filter AccountServiceTest.testCreates --json"
}
```

- [ ] Add `Fidelity` to `testreport.Run`.

Values:

- `high`: no unsupported cases and no approximated categories recorded.
- `medium`: unsupported or approximated behavior appeared, but tests still produced structured outcomes.
- `low`: command hit a broad runtime error before local behavior could be trusted.

- [ ] Derive Phase 1 fidelity from available result data.

Use unsupported count, compile errors, runtime errors, and `Problem.Type`. Do not add a new runtime fidelity engine yet.

- [ ] Add JSON support to `glade report show latest --json`.

Read the saved `results.json` and include run metadata from `run.json`.

- [ ] Render one compact failure packet in console output after the first problem.

Keep stack traces available. Do not replace raw data in JSON.

- [ ] Run:

```bash
go test ./internal/testreport ./internal/gladecli -count=1
```

Expected: tests pass; JSON output includes `fidelity` and `failurePacket` when relevant.

**Acceptance Notes:**

- Packets are explanations, not repairs. Avoid suggesting a code change unless the evidence names it.
- Keep JSON stable. Agents will read it.

### Area E: Local Data Inspection and Dry-Run Diff

**Owner:** Phase 1 runtime subagent.

**Purpose:** Let users inspect local data with SOQL and preview anonymous Apex mutations.

**Files:**

- Create: `internal/storage/diff.go`
- Create: `internal/storage/diff_test.go`
- Modify: `internal/gladecli/db_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `docs/INSTALL.md`
- Modify: `site/docs-src/guide/cli-reference.md`

**Commands added or changed:**

- `glade db query --db <path> [--project <root>] [--json] "<SOQL>"`
- `glade exec --project <root> --db <path> --dry-run --data-diff "<anonymous apex>"`

**Steps:**

- [ ] Add failing tests:
  - `TestRunDBQueryText`
  - `TestRunDBQueryJSON`
  - `TestRunExecDryRunDoesNotPersistDBChanges`
  - `TestRunExecDryRunPrintsDataDiff`
  - `TestStorageDiffSummarizesInsertedUpdatedDeleted`

- [ ] Verify failing tests:

```bash
go test ./internal/gladecli ./internal/storage -run 'TestRun(DBQuery|ExecDryRun)|TestStorageDiff' -count=1
```

Expected before implementation: missing command and missing diff helpers.

- [ ] Add `storage.Diff(before, after OrgState) DiffSummary`.

Minimum JSON shape:

```json
{
  "inserted": {"Account": 1},
  "updated": {"Contact": 2},
  "deleted": {"Invoice__c": 1}
}
```

- [ ] Implement `glade db query`.

Use the existing local SOQL/runtime path. Return records as JSON when `--json` is present. Text mode prints object name, row count, and selected fields.

- [ ] Extend `glade exec` with `--project`, `--db`, `--dry-run`, and `--data-diff`.

When `--db` is present, load org state from SQLite and run against it. When `--dry-run` is present, do not save the mutated org state. When `--data-diff` is present, print `storage.Diff` after execution.

- [ ] Preserve existing `glade exec "System.debug(...)"` behavior.

The default no-project, no-db anonymous path must still work.

- [ ] Run:

```bash
go test ./internal/gladecli ./internal/storage ./internal/vm -count=1
tmp="$(mktemp -d)"
go run ./cmd/glade db reset --db "$tmp/org.sqlite" --project . --json
go run ./cmd/glade db query --db "$tmp/org.sqlite" --project . --json "SELECT Id FROM Account LIMIT 1"
```

Expected: tests pass; query returns stable JSON.

**Acceptance Notes:**

- Do not persist dry-run changes.
- Do not broaden SOQL support in this area. Use what the runtime already supports.

### Phase 1 Integration Captain

**Owner:** One worker after Area A-E branches are ready.

**Steps:**

- [ ] Merge Area A first. It owns top-level command registration and help.
- [ ] Merge Area B. Resolve only command switch conflicts in `internal/gladecli/cli.go`.
- [ ] Merge Area C. Keep `test` subcommand parsing local to `test_command.go`.
- [ ] Merge Area D. Ensure `testreport.Run` JSON remains backward readable.
- [ ] Merge Area E. Recheck `glade exec` help because Area A and E both touch it.
- [ ] Run the full Phase 1 exit gate.
- [ ] Write `docs/superpowers/plans/2026-06-11-local-apex-cli-product-phases-phase1-closeout.md` with exact commands and results.

## Phase 2 Exit Gate

Phase 2 is complete when all of these pass:

```bash
go test ./internal/gladecli ./internal/testreport ./internal/profile ./internal/watch ./internal/apextest -count=1
go run ./cmd/glade check --project . --format sarif > /tmp/glade-check.sarif
go run ./cmd/glade coverage --project . --json > /tmp/glade-coverage.json
go run ./cmd/glade trace diff testdata/traces/before.json testdata/traces/after.json --json || true
go run ./cmd/glade policy check --project . --changed-since HEAD --json
git diff --check
```

The trace diff command may use fixture paths added in Phase 2 tests. Do not depend on external org access.

## Phase 2 Areas

### Area A: SARIF, CI Formats, and Diagnostic Grouping

**Purpose:** Make `glade check` findings usable in GitHub, editors, and CI logs.

**Files:**

- Create: `internal/diagnostic/sarif.go`
- Create: `internal/diagnostic/sarif_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/diagnostic.go`
- Modify: `site/docs-src/guide/cli-reference.md`

**Work:**

- Add `glade check --format text|json|sarif`.
- Add `--output <path>` for SARIF and JSON.
- Add grouped text output by file.
- Add `--max-diagnostics <n>`.

**Tests:**

```bash
go test ./internal/diagnostic ./internal/gladecli ./internal/cliui -count=1
```

### Area B: Coverage

**Purpose:** Report line coverage, changed-line coverage, and coverage by test from trace/source evidence.

**Files:**

- Create: `internal/coverage/model.go`
- Create: `internal/coverage/analyze.go`
- Create: `internal/coverage/report.go`
- Create: `internal/coverage/coverage_test.go`
- Create: `internal/gladecli/coverage_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/apextest/runner.go`
- Modify: `internal/testreport/model.go`

**Work:**

- Add `glade coverage --project <root> [--json] [--changed-since <ref>]`.
- Store enough per-test trace/source line data to answer "which test covered this line."
- Emit changed-line coverage by intersecting git changes with covered Apex lines.

**Tests:**

```bash
go test ./internal/coverage ./internal/apextest ./internal/gladecli -count=1
```

### Area C: AI Policy and Repair Packets

**Purpose:** Give agents a repo-local contract for what to run and a compact packet for failed work.

**Files:**

- Create: `internal/policy/model.go`
- Create: `internal/policy/load.go`
- Create: `internal/policy/check.go`
- Create: `internal/policy/policy_test.go`
- Create: `internal/gladecli/policy_command.go`
- Modify: `internal/testreport/explain.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `site/docs-src/guide/affected-tests.md`

**Work:**

- Add `.glade/policy.yml` support.
- Add `glade policy check --project <root> --changed-since <ref> --json`.
- Add `glade report repair-packet latest --json`.
- Include required commands, failed evidence, relevant files, and fidelity warnings.

**Tests:**

```bash
go test ./internal/policy ./internal/gladecli ./internal/testreport -count=1
```

### Area D: Trace Diff, PR Report, and Report Export

**Purpose:** Compare behavior across runs and produce a reviewer-ready summary.

**Files:**

- Create: `internal/profile/diff.go`
- Create: `internal/profile/diff_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/testreport/reporters.go`
- Modify: `internal/testreport/reporters_test.go`

**Work:**

- Add `glade trace diff <before> <after> [--json]`.
- Add `glade report export latest --format markdown|json`.
- Add a PR summary with tests, changed coverage when present, fidelity, and top findings.

**Tests:**

```bash
go test ./internal/profile ./internal/testreport ./internal/gladecli -count=1
```

### Area E: Runtime-Aware Security Scan

**Purpose:** Add changed-code security findings without creating a maintenance scanner.

**Files:**

- Create: `internal/securityscan/model.go`
- Create: `internal/securityscan/analyze.go`
- Create: `internal/securityscan/report.go`
- Create: `internal/securityscan/securityscan_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/model.go` only if shared finding types are reused by the performance plugin.

**Work:**

- Add `glade inspect security --project <root> [--changed-since <ref>] [--json]`.
- Detect unsafe dynamic SOQL/SOSL, missing CRUD/FLS hints, sensitive `System.debug`, hardcoded secret strings, and unsafe callout construction.
- Mark findings as `static`, `measured`, or `combined` when trace input is supplied.

**Tests:**

```bash
go test ./internal/securityscan ./internal/gladecli -count=1
```

### Area F: Random Order and Flaky Detection

**Purpose:** Expose order-dependent tests and remember repeated local failures.

**Files:**

- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/runner_test.go`
- Modify: `internal/gladecli/test_command.go`
- Modify: `internal/gladecli/dev_command.go`
- Create: `internal/runhistory/history.go`
- Create: `internal/runhistory/history_test.go`

**Work:**

- Add `glade test --order stable|random --seed <n>`.
- Include the seed in JSON and console output.
- Track recent outcomes under `.glade/runs/history.json`.
- Add `glade report flaky --project <root> --json`.

**Tests:**

```bash
go test ./internal/apextest ./internal/gladecli ./internal/runhistory -count=1
```

## Phase 3 Exit Gate

Phase 3 is complete when all of these pass:

```bash
go test ./internal/gladecli ./internal/storage ./internal/profile ./internal/testreport ./internal/watch ./internal/config -count=1
go run ./cmd/glade fixtures validate --project . --fixture testdata/fixtures/minimal.json --json || true
go run ./cmd/glade shape diff --project . --left testdata/shapes/minimal.json --right testdata/shapes/enterprise.json --json || true
go run ./cmd/glade report export latest --format html --output /tmp/glade-report.html || true
git diff --check
```

Fixture and shape fixture paths should be committed by Phase 3 tests.

## Phase 3 Areas

### Area A: Fixture Validation and Scenario Library

**Purpose:** Validate local fixture data against schema, record types, picklists, and relationships.

**Files:**

- Create: `internal/fixturecheck/check.go`
- Create: `internal/fixturecheck/check_test.go`
- Create: `internal/gladecli/fixtures_command.go`
- Modify: `internal/storage/fixture.go`
- Modify: `internal/gladecli/cli.go`

**Work:**

- Add `glade fixtures validate --project <root> --fixture <path> --json`.
- Add named scenarios under `.glade/scenarios/*.json`.
- Add `glade fixtures list --project <root>`.
- Keep generated synthetic data out of Phase 3 unless fixture validation is already green.

**Tests:**

```bash
go test ./internal/fixturecheck ./internal/storage ./internal/gladecli -count=1
```

### Area B: Bulk Projection and Edge-Case Runs

**Purpose:** Estimate governor risk at larger record counts using real trace evidence.

**Files:**

- Create: `internal/bulkrisk/model.go`
- Create: `internal/bulkrisk/project.go`
- Create: `internal/bulkrisk/bulkrisk_test.go`
- Modify: `internal/profile/profile.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/report.go`
- Modify: `internal/gladecli/cli.go`

**Work:**

- Add `glade inspect bulk --project <root> --trace <path> --records 1,10,50,200 --json`.
- Use observed SOQL/DML counts and loop source locations.
- Report projection confidence as `low`, `medium`, or `high`.

**Tests:**

```bash
go test ./internal/bulkrisk ./internal/profile ./internal/gladecli -count=1
```

### Area C: Shape Profiles and Shape Diff

**Purpose:** Show modeled, approximated, stubbed, and unsupported org-shape differences.

**Files:**

- Create: `internal/shape/model.go`
- Create: `internal/shape/diff.go`
- Create: `internal/shape/fidelity.go`
- Create: `internal/shape/shape_test.go`
- Create: `internal/gladecli/shape_command.go`
- Modify: `internal/config/config.go`
- Modify: `internal/storage/model.go`
- Modify: `internal/gladecli/cli.go`

**Work:**

- Add named shape profiles in `glade.yml`.
- Add `glade shape show --project <root> --json`.
- Add `glade shape diff --left <path> --right <path> --json`.
- Feed shape fidelity into `testreport.Fidelity`.

**Tests:**

```bash
go test ./internal/shape ./internal/config ./internal/storage ./internal/gladecli -count=1
```

### Area D: HTML Reports and Confidence Score

**Purpose:** Create shareable local run artifacts without changing the core JSON contract.

**Files:**

- Create: `internal/testreport/html.go`
- Create: `internal/testreport/html_test.go`
- Create: `internal/confidence/score.go`
- Create: `internal/confidence/score_test.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/gladecli/cli.go`

**Work:**

- Add `glade report export latest --format html --output <path>`.
- Add confidence scoring from changed tests, coverage, scan findings, and fidelity.
- Include confidence in Markdown, HTML, and JSON report exports.

**Tests:**

```bash
go test ./internal/testreport ./internal/confidence ./internal/gladecli -count=1
```

### Area E: Editor-Facing Views and Command Palette Hooks

**Purpose:** Feed editors without building a large extension inside this repo.

**Files:**

- Modify: `internal/gladecli/editor_command.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `site/docs-src/guide/editor.md`
- Create: `docs/EDITOR_COMMANDS.md`

**Work:**

- Add `glade editor doctor vscode --json`.
- Add `glade editor commands vscode --json`.
- Include commands for run changed tests, run failed tests, show latest report, show trace profile, inspect db, and run anonymous Apex.
- Keep VS Code extension packaging out of this phase unless the existing editor command already owns it.

**Tests:**

```bash
go test ./internal/gladecli -run 'TestRunEditor' -count=1
```

### Area F: Shared Flag Parser and Color Control

**Purpose:** Pay down command parsing duplication after the behavior is known.

**Files:**

- Create: `internal/flagparse/parser.go`
- Create: `internal/flagparse/parser_test.go`
- Modify: `internal/gladecli/*.go` in small batches.
- Modify: `internal/cliui/theme.go`
- Modify: `internal/cliui/theme_test.go`

**Work:**

- Add a small parser for `--flag value`, `--flag=value`, booleans, short aliases, and `--`.
- Add typo suggestions for unknown flags.
- Add `--color auto|always|never` support where console output is produced.
- Migrate commands in this order: `version`, `doctor`, `parse`, `check`, `exec`, `db`, `test`, `dev`, `report`.

**Tests:**

```bash
go test ./internal/flagparse ./internal/gladecli ./internal/cliui -count=1
```

## Phase Closeout Template

At the end of each phase, write a closeout file with the exact phase number in
the filename and title.

Include these sections:

- Branches Merged
- Commands Run
- Result
- Notes for Next Phase

The Commands Run section must list the full commands from the phase gate and
any extra focused checks. Do not use ellipses. Record pass or fail beside each
command.

The Notes for Next Phase section must name exact behavior to preserve, risky
files, and suggested first subagents.

## Final Guardrails

- Keep `glade` product-facing.
- Keep maintenance scanners and compatibility ledgers in first-party plugins.
- Keep JSON contracts stable once introduced.
- Add tests before implementation in each area.
- Prefer small command files over growing `internal/gladecli/cli.go`.
- Do not claim a phase complete until the phase gate passes.
