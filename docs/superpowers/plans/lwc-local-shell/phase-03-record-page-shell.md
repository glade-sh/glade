# Phase 3: Record Page Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render local Lightning record pages from FlexiPage metadata with record context, record header, regions, component properties, and object validation.

**Architecture:** The server resolves a `RecordPage` FlexiPage through `lwcshell`, reads record data from local `storage.OrgState`, builds a Salesforce-like record shell, and mounts each component instance into its region.

**Tech Stack:** Go XML model from Phase 1, local org storage, server HTML templates, LWC runtime, UI API wire route integration.

---

## Feature Delivered

Developers can open a local record page and test LWCs with the same `recordId` and object context they get in Lightning App Builder.

## Files

- Modify: `internal/server/lwc_shell.go`
- Modify: `internal/server/lwc_shell_test.go`
- Modify: `internal/gladecli/dev_lwc_command.go`
- Create: `internal/lwcshell/record_page.go`
- Create: `internal/lwcshell/record_page_test.go`
- Modify: `internal/server/lightning_wire.go`
- Test data: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml`

## Parallel Squads

- Page resolver squad owns record page lookup and object matching.
- Shell UI squad owns record header, region grid, and context injection.
- Data squad owns record lookup and field display payloads.
- CLI squad owns `--record-page`.
- Review squad compares local shell to `oaer-probe-max` capture for the fixture page.

## Implementation Steps

- [ ] Add `--record-page <name>` to `glade dev lwc`.
- [ ] Require `--record-id` for record page preview unless the fixture seed has exactly one record for the page object. If missing and ambiguous, emit `GLADELWC008 record id required`.
- [ ] Resolve FlexiPage `type=RecordPage`; reject other types for this command with `GLADELWC009 page type mismatch`.
- [ ] Derive `objectApiName` from FlexiPage `sobjectType`; if a flag also supplies object, require an exact case-insensitive match.
- [ ] Build local route:

```text
/lwc/preview/record/<pageName>/<recordId>
```

- [ ] Read the record by ID from `storage.OrgState`. The record header must show object label, record name, record ID, and page label.
- [ ] Render FlexiPage regions in template order. If a template name is unknown, use a stable fallback grid and report `GLADELWC010 template approximated`.
- [ ] For each component instance, pass configured properties plus `recordId` and `objectApiName`.
- [ ] Support FlexiPage component visibility rules only when they are simple field equality checks. For other visibility rules, render the component and emit `GLADELWC011 visibility rule not evaluated`.
- [ ] Ensure `getRecord` and `getObjectInfo` receive the same record and object context when the component wires data.
- [ ] Add tests that a record-page component sees `recordId`, sees XML property values, and disappears for a simple false visibility rule.

## Verification

```bash
go test ./internal/lwcshell ./internal/server ./internal/gladecli -run 'RecordPage|LWC' -count=1
```

```bash
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --record-page Account_Record_Page --record-id 001000000000001AAA --port 18081
```

Scratch comparison:

```bash
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --targets record-page --out /tmp/glade-lwc-record-capture.json)
```

## Done Gate

- Record page route renders at least two LWC instances from FlexiPage regions.
- Components receive `recordId`, `objectApiName`, and configured properties.
- `getRecord` returns local record data on the page.
- Unsupported template or visibility behavior reports diagnostics on page and in JSON.
