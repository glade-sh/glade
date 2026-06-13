# Glade CLI Polish Handoff — Parallel Squad Agent Plan

**Purpose:** turn Glade’s CLI output from “technically useful” into **polished, easy, clean, and self-discoverable**.

**Core product standard:** Glade should always tell the user:

1. **What happened**
2. **Why it matters**
3. **What changed**
4. **What to run next**
5. **How to get stable machine output**

The current CLI has strong capability, but the output often exposes engine internals before user outcomes. This plan splits the work into parallel squads so multiple local agents can make progress without stepping on each other.

---

## Source context

This plan is based on:

- The current `cli-reference.md` command surface.
- The uploaded CLI contact sheet showing `glade --help`, `doctor`, `check`, `test`, `exec --debug-log`, `debug profile`, and `inspect symbols` output.
- CLI UX research patterns from:
  - CLI Guidelines: <https://clig.dev/>
  - Heroku CLI Style Guide: <https://devcenter.heroku.com/articles/cli-style-guide>
  - Google Cloud SDK scripting guidance: <https://docs.cloud.google.com/sdk/docs/scripting-gcloud>

Known Glade command areas include `doctor`, `config`, `parse`, `inspect`, `schema`, `check`, `report`, `exec`, `test`, `dev`, `profile`, `plugins`, `debug`, `editor`, `dap`, `package`, `server`, `db`, and `playground`. The reference already includes machine-friendly modes such as `--json`, SARIF, GitHub output, JUnit, HTML reports, and playground examples. Treat that breadth as an advantage, but make the default output more human-first.

---

# Executive summary

## What is good already

- Glade has a credible, local-first command surface.
- `glade test` has useful developer-loop features: changed tests, watch, last-failed, daemon, connect, JSON, JUnit, and limit modes.
- `glade check` already supports JSON, SARIF, and GitHub output.
- `glade playground` has built-in examples that could become a great onboarding path.
- The CLI already has the right product themes: local checks, local storage, debug-log profiling, local REST API, and explicit runtime boundaries.

## What needs polish

- Top-level help currently feels like a command inventory, not a guided product doorway.
- Default outputs show engine phases / raw JSON / full paths too early.
- `doctor` should feel like a first-run success path, not just a validation dump.
- `check` should lead with diagnostics and fixes, not parser progress.
- `test` should feel like a clean test runner.
- `exec --debug-log` should summarize by default and save raw logs to files.
- `debug profile` should not look like Markdown in TTY mode unless Markdown is requested.
- `inspect symbols` should default to summaries, relative paths, and scannable tables.
- JSON output needs a stable versioned envelope across commands.
- Exit codes, color, progress, stdout/stderr, and prompting behavior should be formalized.

---

# Product output philosophy

## Human output default

Default output should be concise, readable, and actionable.

Rules:

- Show the user outcome first.
- Prefer project-relative paths.
- Avoid full absolute paths unless `--verbose` or `--debug` is passed.
- Avoid raw JSON in normal output.
- Avoid heavy ASCII borders in normal output.
- Use section spacing and aligned key/value lines.
- Include next-step commands after non-trivial outputs.
- Include stable diagnostic codes when failures occur.

Example pattern:

```text
Glade check

✕ 1 diagnostic found

force-app/main/default/classes/AccountService.cls:12:18
error GLADESEM002 metadata reference not found

Why:
  Account.LatestInvoice__c is referenced by Apex, but Glade could not
  find that field in the local schema.

Try:
  glade schema load --project .
  glade check --project .

Summary:
  files checked  42
  diagnostics    1
  runtime        1.0s
  exit code      1
```

## Machine output explicit

Machine output should be stable and boring.

Rules:

- `--json` produces JSON only.
- No ANSI color in JSON.
- No progress in JSON.
- No human logs mixed into JSON.
- JSON keys are stable and versioned.
- Human output may change for clarity; JSON output should be treated as a contract.

## Output ladder

Implement or document this ladder consistently:

```text
default       concise human output
--verbose     more detail for normal debugging
--debug       internal details for Glade maintainers
--json        stable machine-readable schema
--format      specialized artifacts: json, sarif, github, junit, markdown, html
--quiet       only essential output
--plain       grep-friendly, no borders, no color, one record per line
--no-progress suppress progress/spinners
--color       auto | always | never
--no-color    alias for --color never
```

Do not invent every flag everywhere immediately. The goal is consistency where supported and a clear migration path where not supported yet.

---

# Parallel squad structure

## Squad map

| Squad | Focus | Can run in parallel? | Depends on |
|---|---|---:|---|
| 0. Orchestrator | Shared contracts, coordination, branch plan | Yes | None |
| 1. Output Core | Renderer, colors, progress, paths, stdout/stderr | Yes | Squad 0 contracts |
| 2. Help & Discovery | Top-level help, examples, workflows, explain, support | Yes | Squad 0 naming decisions |
| 3. JSON & Automation | JSON envelope, exit codes, CI contracts | Yes | Squad 0 contracts |
| 4. Command UX A | `doctor`, `check`, `parse`, `inspect`, `schema` | Yes | Squad 1 renderer |
| 5. Command UX B | `test`, `dev`, `report` | Yes | Squad 1 renderer, Squad 3 exit codes |
| 6. Command UX C | `exec`, `debug`, `profile`, `server`, `db`, `playground` | Yes | Squad 1 renderer, Squad 3 JSON rules |
| 7. Docs & Website Sync | CLI docs, examples, screenshots, help pages | Yes | Squads 2–6 specs |
| 8. QA / Golden Tests | Snapshot tests, no-color, no-TTY, JSON validity | Yes | Squads 1–6 outputs |

## Recommended branch names

```text
cli-polish-orchestrator
cli-output-core
cli-help-discovery
cli-json-automation-contracts
cli-ux-doctor-check-inspect
cli-ux-test-dev-report
cli-ux-exec-debug-server-db-playground
cli-docs-sync
cli-output-golden-tests
```

## Merge order

1. Orchestrator contracts
2. Output Core renderer
3. JSON & Automation contracts
4. Help & Discovery
5. Command UX A/B/C
6. QA / Golden Tests
7. Docs & Website Sync

If the codebase does not support parallel changes cleanly, use a feature flag while developing:

```bash
GLADE_OUTPUT_V2=1 glade check --project .
```

Then flip the default after snapshot tests pass.

---

# Squad 0 — Orchestrator contracts

## Mission

Create the shared decisions that all squads must follow.

## Tasks

- [ ] Create `docs/internal/cli-output-contract.md` or equivalent.
- [ ] Define the output ladder: default, verbose, debug, json, quiet, plain, color, no-progress.
- [ ] Define the exit-code contract.
- [ ] Define the JSON envelope contract.
- [ ] Define the diagnostic object contract.
- [ ] Define stdout/stderr rules.
- [ ] Define path display rules: project-relative by default, absolute only with `--verbose` / `--debug`.
- [ ] Define color and TTY behavior.
- [ ] Define prompting contract: `--wizard`, `--yes`, `--no-input`.
- [ ] Create a short migration note: “human output may change; JSON is stable.”

## Exit-code contract

Adopt this unless the project already has a better one:

```text
0    success
1    diagnostics, test failures, or expected validation failure
2    usage error or invalid flags
3    project/config discovery failure
4    unsupported local runtime boundary
5    external dependency/toolchain failure
70   internal Glade error
130  interrupted by Ctrl-C
```

## Stdout/stderr contract

```text
stdout:
  Primary command output.
  JSON output.
  Machine-readable output.

stderr:
  Progress.
  Warnings.
  Non-primary status messages.
  Debug/internal messages.
```

For `--json`, stdout must contain only JSON and stderr must not contain progress unless an explicit debug mode is requested.

## Color contract

```text
--color auto    default; color only when TTY supports it
--color always  force color
--color never   no color
--no-color      alias for --color never
```

Respect:

```text
NO_COLOR
TERM=dumb
non-TTY output
```

## Acceptance criteria

- [ ] One contract document exists and is referenced by all squad PRs.
- [ ] All squads agree on exit codes, JSON envelope, color, path, and progress rules.
- [ ] Any intentional deviations are listed explicitly.

---

# Squad 1 — Output Core renderer

## Mission

Build a shared output layer so command teams do not each invent their own formatting.

## Likely files

Adjust paths to actual repo structure:

```text
internal/cli/output/
internal/cli/output/renderer.go
internal/cli/output/color.go
internal/cli/output/progress.go
internal/cli/output/table.go
internal/cli/output/paths.go
internal/cli/output/json.go
```

## Tasks

- [ ] Add a `Renderer` interface for human, JSON, quiet, plain, verbose, and debug modes.
- [ ] Add helpers for:
  - [ ] section headings
  - [ ] key/value summaries
  - [ ] diagnostics
  - [ ] next steps
  - [ ] artifacts
  - [ ] status labels: passed, failed, warning, partial, unsupported
  - [ ] relative path formatting
  - [ ] tables with narrow-terminal fallback
- [ ] Add terminal width detection.
- [ ] Add TTY detection.
- [ ] Add global progress behavior:
  - [ ] TTY default: progress allowed
  - [ ] non-TTY: no spinner/animation
  - [ ] `--no-progress`: no progress
  - [ ] `--json`: no progress
  - [ ] `--quiet`: no progress
- [ ] Replace heavy ASCII box drawing in normal output with cleaner sections.
- [ ] Preserve current decorative output only if explicitly requested through `--format pretty` or equivalent. Do not make decorative boxes the default.

## Renderer examples

### Summary helper

```text
Summary:
  files checked  42
  diagnostics    1
  runtime        1.0s
  exit code      1
```

### Next-step helper

```text
Next:
  glade schema load --project .
  glade check --project .
```

### Artifact helper

```text
Artifacts:
  SARIF   reports/glade-check.sarif
  JUnit   reports/glade-junit.xml
  Trace   reports/trace.json
```

## Acceptance criteria

- [ ] Commands can use shared rendering primitives instead of ad-hoc formatting.
- [ ] Human output is readable at 80 columns.
- [ ] Color can be disabled reliably.
- [ ] Progress does not pollute redirected output or JSON output.
- [ ] Absolute paths are hidden unless verbose/debug mode is active.

---

# Squad 2 — Help & discovery

## Mission

Make Glade self-discoverable from the terminal.

## Current issue

Top-level help currently reads like a long command inventory. New users should see workflows and starting points before they see every command.

## Tasks

- [ ] Redesign top-level `glade` / `glade --help` output around workflows.
- [ ] Keep full command inventory available through `glade help commands` or existing help system.
- [ ] Add or expose:
  - [ ] `glade examples`
  - [ ] `glade help workflows`
  - [ ] `glade explain <error-code>`
  - [ ] `glade support`
  - [ ] `glade help exit-codes`
- [ ] Add examples near the top of command-specific help.
- [ ] Add “Start here” commands in top-level help.
- [ ] Add shell completion descriptions for subcommands and common flag values.
- [ ] Add one-line docs links where appropriate.

## Proposed top-level help

```text
Glade — local Apex runtime

Usage:
  glade <command> [flags]

Start here:
  glade doctor
  glade check
  glade test changed --since origin/main
  glade playground --examples --open

Workflows:
  check        catch Apex source and metadata issues
  test         run local Apex tests
  exec         run anonymous Apex against local state
  debug        parse, profile, and explain Salesforce debug logs
  server       serve a local Salesforce-shaped REST API
  playground  open the browser workbench

Setup:
  init         create glade.yml
  doctor       check project and toolchain
  completion   install shell completions

Inspect:
  inspect      explore symbols and dependency graph
  schema       load or import local Salesforce metadata
  db           seed, reset, inspect, and export local state

Reports:
  report       generate assessment, cruft, refactor-proof, and CI reports

More:
  glade help workflows
  glade examples
  glade support
  glade help exit-codes
```

## `glade examples` proposal

```text
$ glade examples

Built-in examples

ID                           Name                         Tags
account-service              Account Factory + Selector   services,tests
deal-desk-discount-guard     Deal Desk Discount Guard     dml,limits
limit-counter-drill          Governor Counter Drill       limits

Try:
  glade playground --example account-service --open
  glade examples show account-service
```

Commands to implement:

```bash
glade examples
glade examples --tag limits
glade examples show account-service
glade examples run account-service
glade playground --example account-service --open
```

## `glade explain <error-code>` proposal

```text
$ glade explain GLADESEM002

GLADESEM002 — metadata reference not found

Glade found an Apex reference to metadata that is not present in the local schema.

Common causes:
  - custom field metadata has not been loaded
  - project path is wrong
  - schema file is stale

Try:
  glade schema load --project .
  glade check --project .

Docs:
  https://glade.sh/errors/GLADESEM002
```

## Acceptance criteria

- [ ] A new user can run `glade` and know the next three useful commands.
- [ ] Top-level help fits in a normal terminal without feeling like a wall of text.
- [ ] Examples are discoverable without opening the website.
- [ ] Error codes can be explained from the CLI.

---

# Squad 3 — JSON & automation contracts

## Mission

Make Glade dependable in CI and scripts.

## Tasks

- [ ] Define a shared JSON envelope.
- [ ] Apply it first to `doctor`, `check`, `test`, `exec`, `debug profile`, and `inspect symbols`.
- [ ] Add schema versioning.
- [ ] Ensure `--json` is colorless and progress-free.
- [ ] Ensure JSON is valid even on failure.
- [ ] Ensure exit codes match the contract.
- [ ] Add tests for JSON validity and key presence.
- [ ] Add docs section: “Automation contract.”
- [ ] Add `--format` behavior table for commands supporting SARIF, GitHub, JUnit, Markdown, HTML.

## Shared JSON envelope

```json
{
  "schemaVersion": "1.0",
  "command": "check",
  "status": "failed",
  "exitCode": 1,
  "project": {
    "root": "/path/to/project",
    "packageDirs": ["force-app"]
  },
  "summary": {},
  "diagnostics": [],
  "artifacts": [],
  "timings": {},
  "suggestions": []
}
```

## Diagnostic object

```json
{
  "code": "GLADESEM002",
  "severity": "error",
  "message": "metadata reference not found",
  "file": "force-app/main/default/classes/AccountService.cls",
  "line": 12,
  "column": 18,
  "symbol": "Account.LatestInvoice__c",
  "why": "The field is referenced by Apex but is not present in the local schema.",
  "try": [
    "glade schema load --project .",
    "glade check --project ."
  ],
  "docs": "https://glade.sh/errors/GLADESEM002"
}
```

## Test JSON example

```json
{
  "schemaVersion": "1.0",
  "command": "test",
  "status": "passed",
  "exitCode": 0,
  "summary": {
    "selected": 2,
    "passed": 2,
    "failed": 0,
    "skipped": 18,
    "runtimeMs": 1200
  },
  "tests": [
    {
      "name": "AccountServiceTest.passes",
      "status": "passed",
      "durationMs": 8
    }
  ],
  "artifacts": [],
  "suggestions": [
    "glade test --watch",
    "glade test failed"
  ]
}
```

## Acceptance criteria

- [ ] `glade check --json | jq .` always succeeds.
- [ ] `glade test --json | jq .` always succeeds.
- [ ] Failing commands still emit valid JSON when `--json` is requested.
- [ ] Progress never appears in JSON output.
- [ ] Scripts can depend on exit status and structured fields.

---

# Squad 4 — Command UX A: `doctor`, `check`, `parse`, `inspect`, `schema`

## Mission

Polish the core “can I work locally?” and “what is wrong?” commands.

---

## `glade doctor`

### Current issue

`doctor` should be the first-run success path. It should not feel like an internal configuration dump.

### Desired success output

```text
$ glade doctor

Glade doctor

Project       ✓ SFDX project found
Config        ✓ glade.yml
Schema        ✓ 12 objects loaded
Toolchain     ✓ local runtime ready
Local state   ✓ .glade/local-org.sqlite

Ready.

Next:
  glade check
  glade test changed --since origin/main
  glade playground --examples --open
```

### Desired fix-needed output

```text
$ glade doctor

Glade doctor

Project       ✓ SFDX project found
Config        ! no glade.yml found
Schema        ! no local schema loaded
Toolchain     ✓ local runtime ready

2 setup steps needed.

Fix:
  glade init --package-dir force-app
  glade schema load --project .

Then run:
  glade check
```

### Tasks

- [ ] Rewrite human output around readiness and next steps.
- [ ] Keep raw config/toolchain detail behind `--verbose` or `--json`.
- [ ] Add failure modes with exact fix commands.
- [ ] Add golden tests for success, missing config, missing schema, missing toolchain.

---

## `glade check`

### Current issue

Default output should lead with diagnostics and fixes, not parser phases or raw JSON.

### Desired success output

```text
$ glade check

Glade check

✓ No diagnostics found

Checked:
  Apex types   19
  Triggers     3
  Objects      8

Runtime: 1.0s
```

### Desired failure output

```text
$ glade check

Glade check

✕ 1 diagnostic found

force-app/main/default/classes/AccountService.cls:12:18
error GLADESEM002 metadata reference not found

Account.LatestInvoice__c is referenced by Apex but is not present
in the local schema.

Try:
  glade schema load --project .
  glade check --project .

Summary:
  files checked  42
  diagnostics    1
  runtime        1.0s
  exit code      1
```

### Tasks

- [ ] Default output: human summary + diagnostics + try steps.
- [ ] `--json`: structured envelope only.
- [ ] `--format sarif`: SARIF artifact only.
- [ ] `--format github`: GitHub annotation output.
- [ ] Use stable diagnostic codes.
- [ ] Hide engine phases unless `--verbose`.
- [ ] Hide full paths unless `--verbose`.
- [ ] Add golden tests for pass, fail, missing schema, unsupported boundary.

---

## `glade parse`

### Desired output

```text
$ glade parse force-app/main/default/classes

Glade parse

Parsed:
  files       42
  diagnostics 0
  runtime     420ms

✓ No parser diagnostics found
```

For parser errors:

```text
✕ 2 parser diagnostics

force-app/main/default/classes/Broken.cls:7:5
error GLADEPARSE001 expected ';' after statement

Try:
  Fix the syntax error and run:
  glade parse force-app/main/default/classes/Broken.cls
```

### Tasks

- [ ] Use shared diagnostic renderer.
- [ ] Progress only when TTY and not JSON.
- [ ] Add concise success summary.

---

## `glade inspect symbols`

### Current issue

Current output is raw and shows too much path noise.

### Desired output

```text
$ glade inspect symbols

Project symbols

Summary:
  Apex types   2
  Triggers     0
  Objects      1
  Fields       0

Symbols:
  Kind    Name             File
  class   AccountSelector  force-app/.../AccountSelector.cls
  class   AccountService   force-app/.../AccountService.cls
  object  Account          schema/Account.object-meta.xml
```

### Tasks

- [ ] Show summary first.
- [ ] Use relative paths by default.
- [ ] Add `--full-paths` if not already available.
- [ ] Add filters: `--kind class`, `--kind trigger`, `--kind object` if feasible.
- [ ] Use JSON envelope for `--json`.

---

## `glade schema load`

### Desired output

```text
$ glade schema load

Glade schema load

Loaded local metadata

Objects   12
Fields    148
Sources   force-app/main/default/objects
Runtime   320ms

Next:
  glade check
```

### Tasks

- [ ] Summarize loaded metadata.
- [ ] Show skipped/unsupported metadata clearly.
- [ ] Add next command.
- [ ] Use JSON envelope for `--json`.

## Acceptance criteria for Squad 4

- [ ] `doctor` feels like onboarding.
- [ ] `check` failures feel like mini docs.
- [ ] `parse` and `inspect` are scannable at 80 columns.
- [ ] All commands use relative paths by default.
- [ ] All commands have valid JSON mode where supported.

---

# Squad 5 — Command UX B: `test`, `dev`, `report`

## Mission

Make Glade’s local test loop feel excellent.

---

## `glade test`

### Current issue

The command is powerful, but the output should feel like a polished test runner: one-line passes, expanded failure detail only for failures, summary up front.

### Desired passing output

```text
$ glade test changed --since origin/main

Glade test

Selected: 2 tests affected by 3 changed files
Passed:   2
Failed:   0
Skipped:  18
Runtime:  1.2s

✓ AccountServiceTest.passes                 8ms
✓ InvoiceRollupTest.recalculatesTotals     14ms

Next:
  glade test --watch
  glade test failed
```

### Desired failing output

```text
$ glade test --filter AccountServiceTest

Glade test

Selected: 3 tests
Passed:   2
Failed:   1
Runtime:  1.4s

✕ AccountServiceTest.createsInvoice  12ms

  System.AssertException: expected 1, got 0
  force-app/main/default/classes/AccountServiceTest.cls:42

Try:
  glade test failed
  glade test --filter AccountServiceTest.createsInvoice --debug
```

### Tasks

- [ ] Rework default output to test-runner style.
- [ ] One-line pass entries.
- [ ] Expanded details only for failures.
- [ ] Clear selected/passed/failed/skipped/runtime summary.
- [ ] Add affected-test explanation for `changed --since`.
- [ ] Add next commands.
- [ ] Support `--json` envelope.
- [ ] Support JUnit artifact path summary when `--junit` is used.
- [ ] Ensure daemon/connect messaging is quiet unless relevant.

---

## `glade dev`

### Current issue

Potential confusion with `glade test --watch`. Clarify `dev` as the opinionated local feedback loop.

### Desired help copy

```text
glade dev
  Starts the local feedback loop: changed tests, last failures, reports,
  and artifacts under .glade/runs.

glade test
  Runs exact test selections.
```

### Desired output

```text
$ glade dev

Glade dev

Watching project for Apex changes.

On change:
  run changed tests
  rerun last failures
  write artifacts to .glade/runs

Press Ctrl-C to stop.
```

### Tasks

- [ ] Clarify `dev` vs `test` in help.
- [ ] Show what the loop will do before it starts.
- [ ] Use clean run summaries after each cycle.
- [ ] Write artifact paths clearly.

---

## `glade report`

### Desired output

```text
$ glade report show latest

Glade report

Run:     .glade/runs/2026-06-13T03-48-00
Status:  failed
Tests:   2 passed, 1 failed
Check:   1 diagnostic

Artifacts:
  HTML    .glade/runs/latest/report.html
  JSON    .glade/runs/latest/report.json

Open:
  glade report export latest --format html --output glade-report.html
```

### Tasks

- [ ] Make artifacts first-class.
- [ ] Use summary + artifacts + next steps.
- [ ] Keep JSON stable for `--json`.

## Acceptance criteria for Squad 5

- [ ] `glade test` feels as clean as a modern test runner.
- [ ] Failure output points to exact test, exact file/line, and next action.
- [ ] `dev` has a clearly differentiated purpose.
- [ ] `report` makes generated artifacts obvious.

---

# Squad 6 — Command UX C: `exec`, `debug`, `profile`, `server`, `db`, `playground`

## Mission

Polish local runtime, debug-log, server, storage, and playground flows.

---

## `glade exec`

### Current issue

`exec --debug-log` appears to dump full Salesforce-style logs directly into the default output. That is useful but too much for the main path.

### Desired default output

```text
$ glade exec "System.debug('local');"

Glade exec

✓ Anonymous Apex executed

Debug:
  USER_DEBUG local

Limits:
  SOQL queries    0 / 100
  DML statements  0 / 150
  CPU time        1ms / 10000ms

Log:
  .glade/logs/exec-2026-06-13T03-48-00.log

Next:
  glade debug profile --log .glade/logs/exec-2026-06-13T03-48-00.log
  glade db inspect
```

### Add explicit log modes

If feasible:

```bash
glade exec --debug-log summary "System.debug('local');"
glade exec --debug-log raw "System.debug('local');"
glade exec --log-out reports/exec.log "System.debug('local');"
```

### Tasks

- [ ] Summarize debug log by default.
- [ ] Save raw log to file and print path.
- [ ] Add raw log mode explicitly.
- [ ] Show limits summary.
- [ ] Add next commands.
- [ ] Use JSON envelope for `--json`, if supported.

---

## `glade debug profile`

### Current issue

Output currently looks Markdown-ish with `##` headings. In TTY mode, it should feel like terminal output. Markdown should be explicit via `--format markdown`.

### Desired output

```text
$ glade debug profile --log subscriber.log

Glade debug profile

Events: 4

Runtime:
  SOQL queries     1 query / 1 row
  DML statements   1 statement / 1 row
  Callouts         0
  CPU              0ms
  Heap             0 bytes

Hot events:
  Rank  Event                  Count  Rows  Duration
  1     apex.debug             2      0     0ms
  2     SELECT Account         1      1     0ms
  3     apex.dml.insert        1      1     0ms

Next:
  glade debug explain --log subscriber.log --project .
```

### Tasks

- [ ] Remove Markdown headings from default TTY output.
- [ ] Keep Markdown behind `--format markdown`.
- [ ] Add short hot-events table.
- [ ] Add next command.

---

## `glade profile analyze`

### Potential confusion

`glade profile analyze` and `glade debug profile` may overlap in user perception.

### Help clarification

```text
glade debug profile --log apex.log
  Profile a Salesforce debug log.

glade profile analyze reports/trace.json
  Profile a Glade native trace.
```

### Tasks

- [ ] Add help copy clarifying the distinction.
- [ ] Align output structure with `debug profile`.
- [ ] Support `--json` envelope.

---

## `glade server`

### Desired output

```text
$ glade server --project . --db .glade/local-org.sqlite --addr 127.0.0.1:8080

Glade server

Local Salesforce-shaped REST API started

Address   http://127.0.0.1:8080
Database  .glade/local-org.sqlite
Project   .
Mode      local

Try:
  curl "http://127.0.0.1:8080/services/data/v60.0/query?q=SELECT+Name+FROM+Account"

Stop:
  Ctrl-C
```

### Tasks

- [ ] Show address, DB, project, mode.
- [ ] Add curl example.
- [ ] Clarify when persistence is enabled or disabled.
- [ ] Handle port-in-use errors with fix steps.

---

## `glade db`

### Desired output

```text
$ glade db inspect --db .glade/local-org.sqlite

Glade db inspect

Database: .glade/local-org.sqlite

Objects:
  Account      12 records
  Contact      8 records
  Opportunity  3 records

Next:
  glade db export --db .glade/local-org.sqlite > exported-fixture.json
```

### Tasks

- [ ] Make DB operations summarize changed records.
- [ ] Add safety confirmations for destructive resets unless `--yes`.
- [ ] Make `--no-input` fail instead of prompting.
- [ ] Use JSON envelope for `--json`.

---

## `glade playground`

### Desired output

```text
$ glade playground --examples --addr 127.0.0.1:1789 --open

Glade playground

Started local browser workbench

URL       http://127.0.0.1:1789
Examples  enabled
Database  .glade/playground/org.sqlite

Opened in browser.

Stop:
  Ctrl-C
```

For `--list-examples`, use the same table as `glade examples`.

### Tasks

- [ ] Align playground output with server output.
- [ ] Add URL, DB mode, examples mode, reset mode.
- [ ] Route examples listing through shared examples renderer.

## Acceptance criteria for Squad 6

- [ ] `exec` default is concise and does not dump raw logs unless requested.
- [ ] `debug profile` is readable in a terminal and Markdown only when requested.
- [ ] Server/playground outputs include URL, persistence mode, and stop instructions.
- [ ] DB output summarizes state changes.

---

# Squad 7 — Docs & website sync

## Mission

Keep docs, CLI examples, homepage demos, and screenshots aligned with the new CLI output contract.

## Tasks

- [ ] Update CLI reference after output changes.
- [ ] Add docs page: “CLI output modes.”
- [ ] Add docs page: “Exit codes.”
- [ ] Add docs page: “Automation and JSON schema.”
- [ ] Add docs page: “Error codes and `glade explain`.”
- [ ] Update quickstart to start with:
  - [ ] `glade doctor`
  - [ ] `glade check`
  - [ ] `glade test changed --since origin/main`
  - [ ] `glade playground --examples --open`
- [ ] Regenerate all marketing screenshots with new human output.
- [ ] Ensure homepage workbench command output matches real CLI output.
- [ ] Document `NO_COLOR`, `--no-color`, `--no-progress`, and JSON behavior.
- [ ] Document that human output may evolve, while JSON schema is versioned.

## Docs structure proposal

```text
/guide/overview
/guide/quickstart
/guide/cli-output
/guide/automation
/guide/exit-codes
/guide/errors
/guide/examples
/guide/support-map
/reference/cli
/reference/json-schema
```

## Acceptance criteria

- [ ] Website examples use real CLI output, not stale mock output.
- [ ] CLI docs explain human vs automation output.
- [ ] New users get a clean path from install → doctor → check → test → playground.
- [ ] Error codes and next steps are documented.

---

# Squad 8 — QA / golden tests

## Mission

Prevent output polish from regressing.

## Tasks

- [ ] Add golden snapshot tests for human output.
- [ ] Add JSON validity tests for every `--json` command.
- [ ] Add no-color tests.
- [ ] Add non-TTY tests.
- [ ] Add narrow terminal width tests, e.g. 80 columns and 100 columns.
- [ ] Add path redaction tests to ensure full paths do not appear in default output.
- [ ] Add progress suppression tests for `--json`, `--quiet`, `--no-progress`, and non-TTY.
- [ ] Add exit-code tests.
- [ ] Add smoke tests for top-level help and command-specific help.
- [ ] Add test fixture projects for pass/fail/missing schema/unsupported behavior.

## Test matrix

| Scenario | Commands |
|---|---|
| first-run ready | `glade doctor` |
| missing config | `glade doctor` |
| passing check | `glade check` |
| failing check | `glade check` |
| JSON check | `glade check --json` |
| passing tests | `glade test --filter PassingTest` |
| failing tests | `glade test --filter FailingTest` |
| changed tests | `glade test changed --since origin/main` |
| exec summary | `glade exec "System.debug('local');"` |
| raw debug log | `glade exec --debug-log raw ...` |
| debug profile | `glade debug profile --log fixture.log` |
| symbols | `glade inspect symbols` |
| server startup | `glade server --addr 127.0.0.1:0` |
| playground examples | `glade examples` / `glade playground --list-examples` |

## Environment tests

```bash
NO_COLOR=1 glade check
TERM=dumb glade check
glade check --no-color
glade check --no-progress
glade check --json | jq .
glade check > out.txt
```

Assertions:

- [ ] No ANSI color when disabled.
- [ ] No progress/spinner text in redirected output.
- [ ] JSON is parseable.
- [ ] Full local absolute paths are absent by default.
- [ ] Exit codes match contract.

## Acceptance criteria

- [ ] Snapshot tests are easy to update intentionally.
- [ ] CI fails on accidental output regressions.
- [ ] Every command touched by this refactor has at least one human-output test and one JSON/automation test where relevant.

---

# Cross-squad product decisions

## Do not rename commands in P0

Keep command names stable while improving output. Avoid big command renames during the polish sprint.

Exceptions to consider later:

- Clarify `glade dev` vs `glade test --watch` in help first; only rename if confusion remains.
- Clarify `glade debug profile` vs `glade profile analyze` in help first.

## Use stable diagnostic codes

All actionable diagnostics should include codes:

```text
GLADEPARSE001
GLADESEM002
GLADECFG001
GLADEORG004
GLADETEST001
```

Codes should support:

```bash
glade explain GLADESEM002
```

## Every non-trivial command should include `Next:` or `Try:`

Good:

```text
Next:
  glade test --watch
  glade test failed
```

For failures:

```text
Try:
  glade schema load --project .
  glade check --project .
```

Do not overdo it. One to three next commands is enough.

## Full paths policy

Default:

```text
force-app/main/default/classes/AccountService.cls:12:18
```

Verbose/debug:

```text
/Users/matt/Dev/glade/worktrees/.../force-app/main/default/classes/AccountService.cls:12:18
```

## Box drawing policy

Use fewer ASCII boxes in default output. They can be nice in screenshots, but they wrap poorly and make copying harder. Prefer simple sections and aligned summaries.

Allowed:

- Tables for list commands.
- Simple separators where useful.
- No heavy decorative borders in default command output.

---

# Implementation phases

## Phase 0 — Contracts and test fixtures

Owner: Squads 0 and 8

- [ ] Define output contract.
- [ ] Define JSON envelope.
- [ ] Define exit codes.
- [ ] Create fixtures for pass/fail/missing schema/unsupported behavior.
- [ ] Create snapshot test harness.

## Phase 1 — Renderer foundation

Owner: Squad 1

- [ ] Build output renderer.
- [ ] Add color/progress/path helpers.
- [ ] Add TTY and width handling.
- [ ] Add helper functions for summaries, diagnostics, artifacts, and next steps.

## Phase 2 — Command adoption

Owners: Squads 4, 5, 6

- [ ] Rewrite human output for priority commands.
- [ ] Keep `--json` stable and parseable.
- [ ] Add next-step suggestions.
- [ ] Add golden tests.

## Phase 3 — Discovery layer

Owner: Squad 2

- [ ] Improve top-level help.
- [ ] Add `examples`, `explain`, `support`, `help workflows`, `help exit-codes`.
- [ ] Update completions.

## Phase 4 — Docs and website sync

Owner: Squad 7

- [ ] Update docs.
- [ ] Regenerate screenshots.
- [ ] Align homepage workbench demo output with actual CLI output.

## Phase 5 — QA hardening

Owner: Squad 8

- [ ] Run terminal matrix.
- [ ] Run JSON matrix.
- [ ] Run no-color/non-TTY/narrow-width checks.
- [ ] Confirm exit codes.

---

# Priority order

## P0 — must do first

- [ ] Shared output renderer.
- [ ] Human vs JSON output contract.
- [ ] Top-level help reorganization.
- [ ] Exit-code contract.
- [ ] TTY/progress/color behavior.
- [ ] `doctor`, `check`, and `test` output polish.
- [ ] Golden tests for the above.

## P1 — high impact

- [ ] `exec` summary output and raw-log handling.
- [ ] `debug profile` terminal output cleanup.
- [ ] `inspect symbols` scannability.
- [ ] `glade examples`.
- [ ] `glade explain <error-code>`.
- [ ] Docs pages for automation, output modes, and exit codes.

## P2 — polish and breadth

- [ ] `server`, `db`, and `playground` output polish.
- [ ] `dev` vs `test --watch` help clarification.
- [ ] `profile analyze` vs `debug profile` help clarification.
- [ ] Shell completion descriptions.
- [ ] Full docs and website screenshot refresh.

---

# Local parallel-agent prompt

Use this prompt to launch each squad agent. Replace `{SQUAD_NAME}` and `{SQUAD_SCOPE}`.

```text
You are working on the Glade CLI polish sprint as {SQUAD_NAME}.

Read:
- glade_cli_polish_parallel_squads_handoff.md
- cli-reference.md
- the relevant command implementation files
- existing tests and golden outputs

Your scope:
{SQUAD_SCOPE}

Product goal:
Glade should feel polished, easy, clean, and self-discoverable. Human output should lead with user outcomes. JSON output should be stable, colorless, and progress-free. Commands should tell users what happened, why it matters, what changed, and what to run next.

Constraints:
- Do not rename commands unless the handoff explicitly says to.
- Do not break existing JSON consumers. Add versioned schema fields carefully.
- Hide full absolute paths in default human output.
- Respect --json, --no-progress, --no-color, NO_COLOR, TERM=dumb, and non-TTY behavior.
- Add or update tests for every output behavior you change.
- Keep command examples aligned with docs and homepage demos.

Deliverables:
1. Code changes for your squad scope.
2. Golden output tests or JSON contract tests.
3. Updated docs snippets where needed.
4. A short implementation note listing commands changed, flags affected, and migration concerns.

Before finishing:
- Run relevant unit tests.
- Run command snapshots at 80 columns.
- Run JSON outputs through jq.
- Verify no progress text appears in JSON or redirected output.
- Verify --no-color and NO_COLOR remove ANSI codes.
```

---

# Squad-specific prompt blocks

## Squad 1 prompt scope

```text
Build the shared CLI output renderer: summaries, diagnostics, next steps, artifacts, color, progress, path shortening, TTY/non-TTY handling, and terminal width fallback. Do not rewrite every command yet; provide primitives and migrate at least one small command as proof.
```

## Squad 2 prompt scope

```text
Redesign top-level help and discovery commands. Add or wire up glade examples, glade help workflows, glade explain <error-code>, glade support, and glade help exit-codes. Prioritize self-discovery and concise help.
```

## Squad 3 prompt scope

```text
Define and implement the shared JSON envelope and automation contract for priority commands. Make --json outputs valid, colorless, progress-free, and stable. Add exit-code tests and JSON schema docs.
```

## Squad 4 prompt scope

```text
Polish doctor, check, parse, inspect symbols, inspect graph, and schema load/import outputs. Lead with user outcomes, diagnostics, fix commands, and relative paths. Use shared renderer and add golden tests.
```

## Squad 5 prompt scope

```text
Polish test, dev, and report outputs. Make test output feel like a clean modern test runner. Clarify dev as the opinionated local feedback loop. Make report artifacts first-class. Add snapshot and JSON/JUnit-related tests.
```

## Squad 6 prompt scope

```text
Polish exec, debug, profile, server, db, and playground outputs. Summarize debug logs by default, save raw logs to files, clean up debug profile TTY output, and make server/playground/db commands stateful and actionable.
```

## Squad 7 prompt scope

```text
Update docs and website snippets to match the new CLI output contract. Add docs pages for output modes, automation/JSON schema, exit codes, errors/explain, and examples. Regenerate screenshots after commands are updated.
```

## Squad 8 prompt scope

```text
Build the golden output QA harness. Add tests for human output snapshots, JSON validity, exit codes, no-color, NO_COLOR, TERM=dumb, non-TTY output, --no-progress, narrow terminal widths, and default path redaction.
```

---

# Definition of done

This sprint is done when:

- [ ] `glade` top-level help tells a new user where to start.
- [ ] `glade doctor` provides readiness, fixes, and next steps.
- [ ] `glade check` surfaces diagnostics and fix commands before engine details.
- [ ] `glade test` reads like a clean modern test runner.
- [ ] `glade exec` summarizes logs and writes raw logs to files unless raw output is requested.
- [ ] `glade debug profile` is terminal-native by default and Markdown only when requested.
- [ ] `glade inspect symbols` is scannable and uses relative paths.
- [ ] `--json` outputs are valid, stable, colorless, and progress-free.
- [ ] Exit codes are documented and tested.
- [ ] Color/progress behavior respects TTY, `NO_COLOR`, `TERM=dumb`, `--no-color`, and `--no-progress`.
- [ ] Full absolute paths do not appear in default output.
- [ ] Docs and website demos match actual CLI output.
- [ ] Golden tests protect the new output style.

---

# Final product standard

The polished Glade CLI should feel like this:

```text
Caught 1 issue.
Ran 2 tests.
Inserted 1 record.
Profiled 4 log events.
Started a local Salesforce-shaped API.
```

Then it should show:

```text
where it happened
what changed
what to run next
how to get machine output
```

That is the difference between an impressive engine and a product-grade CLI.
