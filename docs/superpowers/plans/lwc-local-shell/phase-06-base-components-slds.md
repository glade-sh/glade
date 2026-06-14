# Phase 6: Base Components And SLDS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan with parallel subagent squads. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Future work: provide practical local implementations for common `lightning-*` base components and Salesforce Lightning Design System styling in both the Lightning shell and Visualforce Lightning Out host.

**Architecture:** Base component shims live in the LWC browser runtime and are mapped through import aliases. The first tier favors common dev loops: cards, buttons, inputs, layouts, forms, datatables, tabs, icons, spinners, toasts, modals, and record forms.

**Tech Stack:** LWC runtime modules, static CSS assets, Go import-map generation, browser tests, screenshot checks.

---

## Feature To Deliver

Most app-builder and Visualforce-hosted LWCs render with usable base components instead of broken custom elements or empty boxes. This branch documents the boundary; the base-component tier is not part of the current shipped LWC shell.

## Files

- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Create: `internal/lwcbrowser/base_components.go`
- Create: `internal/lwcbrowser/base_components_test.go`
- Create: `lwcruntime/src/lightning/button.mjs`
- Create: `lwcruntime/src/lightning/card.mjs`
- Create: `lwcruntime/src/lightning/input.mjs`
- Create: `lwcruntime/src/lightning/datatable.mjs`
- Create: `lwcruntime/src/lightning/layout.mjs`
- Create: `lwcruntime/src/lightning/tabset.mjs`
- Create: `lwcruntime/src/lightning/record-form.mjs`
- Create: `lwcruntime/src/lightning/toast.mjs`
- Create: `lwcruntime/src/slds/glade-slds.css`
- Add tests: `lwcruntime/test/base-components.test.mjs`
- Add tests: `lwcruntime/test/visualforce-base-components.test.mjs`

## Parallel Subagent Squads

Use parallel subagent squads where files do not overlap. The coordinator integrates one patch at a time.

- Component tier squad owns non-data base components.
- Data form squad owns `record-form`, `record-view-form`, and `record-edit-form`.
- Styling squad owns SLDS subset and asset serving.
- Test squad owns browser coverage and screenshot thresholds.
- Visualforce host squad owns base component coverage inside `/apex/<PageName>`.
- Review squad checks that unsupported base components have explicit diagnostics.

## Implementation Steps

- [ ] Add an import-map prefix for `lightning/` modules served from `/lightning/modules/lightning/<name>.js` or static runtime shims.
- [ ] Tier 1 components:
  - `lightning-card`
  - `lightning-button`
  - `lightning-button-icon`
  - `lightning-input`
  - `lightning-textarea`
  - `lightning-combobox`
  - `lightning-layout`
  - `lightning-layout-item`
  - `lightning-tabset`
  - `lightning-tab`
  - `lightning-spinner`
  - `lightning-icon`
- [ ] Tier 2 data components:
  - `lightning-datatable`
  - `lightning-record-form`
  - `lightning-record-view-form`
  - `lightning-record-edit-form`
  - `lightning-output-field`
  - `lightning-input-field`
  - `lightning-messages`
- [ ] Each component must dispatch the documented common DOM events used in local dev: `click`, `change`, `submit`, `success`, `error`, `cancel`, and row action where applicable.
- [ ] Serve `glade-slds.css` automatically in LWC shell pages. Keep it scoped to the shell page and base components.
- [ ] Use data from Phase 5 for record forms. If Phase 5 is not present in the branch, data forms must show `GLADELWC014 record form requires LDS support` with a test.
- [ ] Add `UnsupportedBaseComponent(name)` diagnostic for every unknown `lightning/*` import.
- [ ] Add screenshot tests for a card, form, datatable, and tabset in the shell.
- [ ] Add Visualforce Lightning Out screenshot or DOM tests for the same card, form, datatable, and tabset cases.

## Verification

```bash
go test ./internal/lwcbrowser ./internal/server -run 'Base|LWC|Lightning' -count=1
```

```bash
(cd lwcruntime && npm test -- --runInBand)
```

```bash
(cd lwcruntime && npm run build && node --test test/visualforce-base-components.test.mjs)
```

```bash
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --port 18085
# then open the base-component probe route when this future fixture exists.
```

## Done Gate

- Tier 1 base components render and dispatch common events.
- Tier 2 data components work when Phase 5 is present.
- Unknown base component imports report a diagnostic.
- Shell pages include SLDS subset without requiring network access.
- Visualforce Lightning Out pages include the same base component modules and styling behavior, unless a support row names a host-specific exception.
