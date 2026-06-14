# Phase 8: Browser Tests, Support Ledger, And Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan with parallel subagent squads. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Certify chosen LWC features in both Lightning shell and Visualforce Lightning Out hosts with browser tests, the current fixture-manifest oracle input, future scratch-org browser captures, generated support ledgers, and user docs.

**Architecture:** Product tests prove local behavior. `glade-tools` compares local and scratch evidence. Public docs list supported, partial, unsupported, and Salesforce-only behavior without exposing maintenance internals in base `glade --help`.

**Tech Stack:** Go tests, Node runtime tests, Playwright browser tests, `glade-tools` compat reports, checked markdown support docs.

---

## Feature Delivered

Users get a trustworthy support map and command docs. Agents get a phase-by-phase gate before claiming parity, split by host. Today the LWC compat command prepares fixture-manifest targets; browser/org capture and ledger generation are later work in this phase.

## Files

- Create: `internal/lwcshell/support.go`
- Create: `internal/lwcshell/support_test.go`
- Create: `docs/LWC_LOCAL_SHELL.md`
- Modify: `docs/LOCAL_TESTING.md`
- Modify: `site/docs-src/guide/local-testing.md`
- Create in `glade-tools`: `internal/compat/lwc_ledger.go`
- Create in `glade-tools`: `internal/compat/lwc_ledger_test.go`
- Modify in `glade-tools`: `internal/toolcli/compat_command.go`
- Generated output target: `docs/generated/LWC_SHELL_SUPPORT.md`

## Parallel Subagent Squads

Use parallel subagent squads where files do not overlap. The coordinator integrates one patch at a time.

- Browser test squad owns end-to-end local rendering cases.
- Ledger squad owns support status and capture comparison.
- Docs squad owns product docs.
- CLI docs squad owns help text and examples.
- Visualforce host squad owns `/apex/<PageName>` Lightning Out documentation, tests, and support rows.
- Review squad runs all phase gates selected for certification.

## Implementation Steps

- [ ] Add a support model with statuses:
  - `supported`
  - `partial`
  - `unsupported`
  - `salesforce-only`
- [ ] Add feature IDs:
  - `lwc.host.lightning-shell`
  - `lwc.host.visualforce-lightning-out`
  - `lwc.shell.direct-component`
  - `lwc.shell.record-page`
  - `lwc.shell.app-page`
  - `lwc.shell.home-page`
  - `lwc.shell.custom-tab`
  - `lwc.wire.apex`
  - `lwc.wire.ui-record-api`
  - `lwc.lds.cache`
  - `lwc.base-components.tier1`
  - `lwc.base-components.data-forms`
  - `lwc.navigation.page-reference`
  - `lwc.services.toast`
  - `lwc.services.message-service`
  - `lwc.services.modal`
  - `lwc.actions.quick-action`
- [ ] Every support row must include a host coverage field with one of:
  - `lightning-shell`
  - `visualforce-lightning-out`
  - `both`
  - `salesforce-only`
- [ ] Add future product command:

```bash
# future:
glade report lwc-shell --project . --json
```

It reads local support state and project usage. It does not call scratch orgs.

- [ ] Add future `glade-tools` ledger command:

```bash
# future:
glade compat lwc ledger --captures /tmp/glade-lwc-capture.json --output docs/generated/LWC_SHELL_SUPPORT.md
```

- [ ] Add future checked-mode command:

```bash
# future:
glade compat lwc ledger --captures /tmp/glade-lwc-capture.json --check docs/generated/LWC_SHELL_SUPPORT.md
```

- [ ] Add Playwright browser cases for every feature marked `supported` in each host.
- [ ] Add console-error gate: supported routes must produce zero unexpected browser console errors.
- [ ] Add docs with examples for direct component, record page, app page, home page, tab, Visualforce Lightning Out page, record data, Apex controllers, hot reload, and scratch comparison.
- [ ] Add public docs note that exact Lightning Experience private internals are not the goal; public contracts and local dev behavior are the goal.

## Verification

```bash
go test ./internal/lwcshell ./internal/gladecli ./internal/server ./internal/lwcbrowser -run 'LWC|Support|Report' -count=1
```

```bash
(cd lwcruntime && npm test -- --runInBand)
```

```bash
(cd ../glade-tools && go test ./internal/compat ./internal/toolcli -run 'Lwc|Ledger' -count=1)
```

```bash
# future:
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc ledger --captures /tmp/glade-lwc-capture.json --output ../glade/docs/generated/LWC_SHELL_SUPPORT.md)
```

## Done Gate

- Every `supported` feature has a local browser or Go test and, where needed, fixture-manifest or later scratch browser-capture evidence.
- Every `supported` feature names host coverage. Shared runtime features must have evidence for both `lwc.host.lightning-shell` and `lwc.host.visualforce-lightning-out`.
- Docs name the commands and known boundaries.
- Checked generated support output passes.
- Base `glade --help` remains product-focused and does not expose maintenance-only compat commands.
