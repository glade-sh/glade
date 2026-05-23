# Oaer CLI DX Output Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a precise, clean, focused CLI experience for local Apex development while keeping existing scriptable `oaer` commands stable.

**Architecture:** Keep the existing command surfaces as the machine-friendly guts. Add a human-first `oaer dev` cockpit, richer help, run artifacts under `.oaer/runs`, and a focused watch renderer over the existing `internal/watch` NDJSON events.

**Tech Stack:** Go 1.26, `internal/oaercli`, `internal/watch`, `internal/testreport`, `internal/config`, and small new packages only where they remove real duplication.

---

## Design Rules

Human output must be precise, sparse, scannable, and action-oriented.

Every human command prints in this order:

1. Scope: what project, files, tests, or checks the command touched.
2. Selection: why these tests or checks ran.
3. Result: pass, fail, unsupported, compile error, or internal error.
4. Evidence: first useful failure detail, not the whole haystack.
5. Artifact: exact path to the report, when a report exists.
6. Next step: only when there is a clear one.

Machine output stays plain JSON, NDJSON, or JUnit. No color. No decorative text. No human-only labels in machine formats.

TTY output may use color only for status and emphasis. `NO_COLOR`, non-TTY output, JSON, NDJSON, JUnit, and CI mode must stay unstyled.

## Command Shape

Add the friendly front door:

```bash
oaer dev
oaer dev watch
oaer dev test
oaer dev test --all
oaer dev test --class InvoiceServiceTest
oaer dev test --test InvoiceServiceTest.rejectsBadEmail
oaer dev test --changed
oaer dev test --failed
oaer report list
oaer report show latest
oaer report export latest
```

Keep existing commands stable:

```bash
oaer test --json
oaer test --watch
oaer compat local-tests --json
oaer package build
```

Fix help behavior early:

```bash
oaer test --help
oaer help test
oaer compat local-tests --help
```

## Example Output

### `oaer dev`

```text
Project: /Users/matt/Dev/acme
Package dirs: 2
Apex classes: 418
Apex tests: 96
Metadata: loaded
Last run: failed, 4m ago

Next:
  oaer dev test --failed
  oaer dev watch
```

### Passing Test Run

```text
Selected 12 tests from 3 changed files.

PASS AccountServiceTest.createsAccount       42ms
PASS AccountServiceTest.updatesAccount       38ms
PASS InvoiceServiceTest.appliesDiscount      71ms

Result: 12 passed, 0 failed, 12 total, 812ms
Report: .oaer/runs/20260523-143210/summary.md
```

### Failing Test Run

```text
Selected 3 tests from 1 changed file.

PASS AccountServiceTest.createsAccount        42ms
PASS AccountServiceTest.updatesAccount        38ms
FAIL AccountServiceTest.rejectsBadEmail       51ms

Expected ValidationException.
Got no exception.

at AccountServiceTest.rejectsBadEmail:34

Result: 2 passed, 1 failed, 3 total, 181ms
Report: .oaer/runs/20260523-143210/summary.md

Next:
  oaer dev test --test AccountServiceTest.rejectsBadEmail
```

### Watch

```text
Watching force-app.
Strategy: affected tests

Changed:
  force-app/main/default/classes/InvoiceService.cls

Selected:
  InvoiceServiceTest
  InvoiceRollupTest

Result: 28 passed, 0 failed, 28 total, 1.9s
Report: .oaer/runs/20260523-143519/summary.md
```

### Watch Fallback

```text
Changed:
  force-app/main/default/objects/Invoice__c/fields/Status__c.field-meta.xml

Selection: all tests
Reason: schema change can affect any test.

Result: 761 passed, 0 failed, 761 total, 18.4s
Report: .oaer/runs/20260523-144011/summary.md
```

### Unsupported Surface

```text
Selected 1 test from 1 changed file.

UNSUPPORTED PdfControllerTest.rendersInvoicePdf  14ms

PageReference.getContentAsPDF is not supported.

Result: 0 passed, 0 failed, 1 unsupported, 1 total, 44ms
Report: .oaer/runs/20260523-144322/summary.md
```

### JSON

```json
{
  "summary": {
    "total": 3,
    "passed": 2,
    "failed": 1,
    "unsupported": 0,
    "durationMs": 181
  },
  "reportPath": ".oaer/runs/20260523-143210/results.json"
}
```

## Files

- Modify: `internal/oaercli/cli.go`
  - Route `oaer dev`, `oaer report`, command-specific help, and selector flags.
- Modify: `internal/oaercli/cli_test.go`
  - Add command help, selector, report, artifact, and output-shape tests.
- Modify: `internal/testreport/reporters.go`
  - Add focused human output and Markdown summary rendering.
- Modify: `internal/testreport/model.go`
  - Add fields only if needed for unsupported counts, report paths, or stable display names.
- Modify: `internal/watch/events.go`
  - Add selection reason fields only if the existing `TestSelection` model cannot already carry them.
- Modify: `internal/watch/affected.go`
  - Expose affected-test reason and fallback confidence for watch output.
- Create: `internal/runartifact/runartifact.go`
  - Own run IDs, run directory creation, manifests, `latest.json`, cleanup, and export metadata.
- Create: `internal/runartifact/runartifact_test.go`
  - Cover deterministic run IDs, latest pointer writes, manifest writes, cleanup, and export manifests.
- Create: `internal/cliui/cliui.go`
  - Own TTY-aware status rendering, width-safe rows, color decisions, and `NO_COLOR` handling.
- Create: `internal/cliui/cliui_test.go`
  - Cover no-color mode, non-TTY mode, row rendering, and focused failure output.

## Tasks

### Task 1: Help That Works Everywhere

**Files:**
- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/cli_test.go`

- [ ] Add tests for `oaer test --help`, `oaer help test`, and `oaer compat local-tests --help`.
- [ ] Implement command-specific help routing before flag parsing.
- [ ] Add examples to help text for `test`, `compat local-tests`, and top-level `help`.
- [ ] Run: `go test ./internal/oaercli`

Expected result: help commands exit 0 and print scoped help instead of `unknown flag "--help"`.

### Task 2: Focused Console Test Output

**Files:**
- Modify: `internal/testreport/reporters.go`
- Modify: `internal/testreport/model.go`
- Modify: `internal/oaercli/cli_test.go`

- [ ] Add tests for pass, fail, unsupported, compile-error, and mixed-result console output.
- [ ] Change human console output to print selection summary, case rows, first failure evidence, result line, and report path when provided.
- [ ] Keep JSON and JUnit byte shapes stable except for intentional additive fields.
- [ ] Run: `go test ./internal/testreport ./internal/oaercli`

Expected result: human test output matches the examples in this plan.

### Task 3: Run Artifacts

**Files:**
- Create: `internal/runartifact/runartifact.go`
- Create: `internal/runartifact/runartifact_test.go`
- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/cli_test.go`

- [ ] Add `RunID`, `RunDir`, and manifest helpers.
- [ ] Write `.oaer/runs/<run-id>/run.json`, `summary.md`, `results.json`, `junit.xml`, `selection.json`, and `events.ndjson` when a DX command runs.
- [ ] Write `.oaer/runs/latest.json` as a JSON pointer file.
- [ ] Add `--out` to override the run root for DX commands.
- [ ] Run: `go test ./internal/runartifact ./internal/oaercli`

Expected result: DX commands leave a small, predictable artifact trail and raw commands do not write artifacts unless asked.

### Task 4: `oaer dev` Cockpit

**Files:**
- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/cli_test.go`

- [ ] Add `oaer dev` routing.
- [ ] Print project root, package dirs, Apex class count, Apex test count, metadata status, last run, and next commands.
- [ ] Add `oaer dev test` as a wrapper over existing test execution.
- [ ] Add selectors: `--all`, `--class`, `--test`, `--changed`, and `--failed`.
- [ ] Run: `go test ./internal/oaercli`

Expected result: common local-test workflows have a short human command while existing `oaer test` remains stable.

### Task 5: Watch Renderer

**Files:**
- Modify: `internal/watch/events.go`
- Modify: `internal/watch/affected.go`
- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/cli_test.go`

- [ ] Preserve `oaer test --watch` as NDJSON.
- [ ] Add `oaer dev watch` as a human renderer over the same event stream.
- [ ] Print changed files, selected tests, fallback reason, run duration, result, and report path.
- [ ] Cancel stale runs and suppress stale output using existing watch behavior.
- [ ] Run: `go test ./internal/watch ./internal/oaercli`

Expected result: watch mode gives a clean loop for humans and stable events for editors.

### Task 6: Report Commands

**Files:**
- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/cli_test.go`
- Modify: `internal/runartifact/runartifact.go`
- Modify: `internal/runartifact/runartifact_test.go`

- [ ] Add `oaer report list`.
- [ ] Add `oaer report show latest`.
- [ ] Add `oaer report export latest`.
- [ ] Add `oaer report clean --keep N`.
- [ ] Run: `go test ./internal/runartifact ./internal/oaercli`

Expected result: developers can inspect, share, and prune run evidence without hunting through files.

### Task 7: Final Validation

**Files:**
- Modify as needed from prior tasks only.

- [ ] Run: `gofmt` on changed Go files.
- [ ] Run: `go test ./internal/oaercli ./internal/watch ./internal/testreport ./internal/runartifact ./internal/cliui`.
- [ ] Run: `go run ./cmd/oaer test --help`.
- [ ] Run: `go run ./cmd/oaer dev --project testdata/local-tests/basic`.
- [ ] Run: `go run ./cmd/oaer dev test --project testdata/local-tests/basic --out .oaer/runs`.
- [ ] Run: `go run ./cmd/oaer test --project testdata/local-tests/basic --watch-once`.

Expected result: all targeted checks pass and output stays focused under both human and machine modes.

## Non-Goals

- Do not rewrite the CLI framework.
- Do not change Apex runtime behavior.
- Do not add project-specific routes or stubs.
- Do not add decorative terminal UI.
- Do not put human text into JSON, NDJSON, or JUnit output.

## Inspiration

- GitHub CLI: scriptable JSON, aliases, and strong command verbs.
- Vercel CLI: whole workflow from local setup through deploy.
- Fly CLI: status-first deploy output and monitor links.
- Stripe CLI: local event loops with `listen`, `trigger`, and `logs tail`.
- Bun, Deno, and uv: fast defaults and direct project-aware commands.
- Charm Gum: restraint around prompts, spinners, tables, and filters.
