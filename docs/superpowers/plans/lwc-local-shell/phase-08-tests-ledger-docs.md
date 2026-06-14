# Phase 8: Browser Tests, Support Ledger, And Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Certify chosen LWC shell features with browser tests, scratch-org captures, generated support ledgers, and user docs.

**Architecture:** Product tests prove local behavior. `glade-tools` compares local and scratch evidence. Public docs list supported, partial, unsupported, and Salesforce-only behavior without exposing maintenance internals in base `glade --help`.

**Tech Stack:** Go tests, Node runtime tests, Playwright browser tests, `glade-tools` compat reports, checked markdown support docs.

---

## Feature Delivered

Users get a trustworthy support map and command docs. Agents get a phase-by-phase gate before claiming parity.

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

## Parallel Squads

- Browser test squad owns end-to-end local rendering cases.
- Ledger squad owns support status and capture comparison.
- Docs squad owns product docs.
- CLI docs squad owns help text and examples.
- Review squad runs all phase gates selected for certification.

## Implementation Steps

- [ ] Add a support model with statuses:
  - `supported`
  - `partial`
  - `unsupported`
  - `salesforce-only`
- [ ] Add feature IDs:
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
- [ ] Add product command:

```bash
glade report lwc-shell --project . --json
```

It reads local support state and project usage. It does not call scratch orgs.

- [ ] Add `glade-tools` command:

```bash
glade compat lwc ledger --captures /tmp/glade-lwc-capture.json --output docs/generated/LWC_SHELL_SUPPORT.md
```

- [ ] Add checked-mode command:

```bash
glade compat lwc ledger --captures /tmp/glade-lwc-capture.json --check docs/generated/LWC_SHELL_SUPPORT.md
```

- [ ] Add Playwright browser cases for every feature marked `supported`.
- [ ] Add console-error gate: supported routes must produce zero unexpected browser console errors.
- [ ] Add docs with examples for direct component, record page, app page, home page, tab, record data, Apex controllers, hot reload, and scratch comparison.
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
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc ledger --captures /tmp/glade-lwc-capture.json --output ../glade/docs/generated/LWC_SHELL_SUPPORT.md)
```

## Done Gate

- Every `supported` feature has a local browser or Go test and, where needed, scratch capture evidence.
- Docs name the commands and known boundaries.
- Checked generated support output passes.
- Base `glade --help` remains product-focused and does not expose maintenance-only compat commands.
