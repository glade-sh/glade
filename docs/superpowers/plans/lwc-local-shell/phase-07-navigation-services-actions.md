# Phase 7: Navigation, Services, And Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan with parallel subagent squads. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement host-aware local services for current page reference, basic navigation, toast events, Lightning Message Service, and resource loading. Keep modals and quick actions as future shell-service work unless implemented with tests.

**Architecture:** Browser modules communicate with a shell service object installed by the page bootstrap. The current service owns PageReference values, URL generation, basic navigation, toast events, message channels, and resource loading. Later phases can add modal lifecycle and action context.

**Tech Stack:** `lwcruntime` shell service, Go PageReference construction, `@salesforce/messageChannel` shims, server routes, browser tests.

---

## Feature Delivered

Components that use supported shell services can be tested locally in record, app, home, tab, and Visualforce Lightning Out contexts.

## Files

- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `internal/lwcbrowser/salesforce_modules_test.go`
- Modify: `internal/server/lwc_shell.go`
- Create: `lwcruntime/src/shims/navigation.mjs`
- Create: `lwcruntime/src/shims/toast.mjs`
- Create: `lwcruntime/src/shims/message-service.mjs`
- Create: `lwcruntime/src/shims/modal.mjs`
- Create: `lwcruntime/test/navigation.test.mjs`
- Create: `lwcruntime/test/services.test.mjs`
- Create: `lwcruntime/test/visualforce-services.test.mjs`
- Test data: `testdata/local-tests/lwc-shell/force-app/main/default/messageChannels/LwcProbe.messageChannel-meta.xml`

## Parallel Subagent Squads

Use parallel subagent squads where files do not overlap. The coordinator integrates one patch at a time.

- Navigation squad owns `NavigationMixin`, `CurrentPageReference`, and URL generation.
- Toast/resource squad owns toast events and resource loading.
- LMS squad owns message channel metadata and publish/subscribe.
- Future action squad owns record and global action contexts.
- Visualforce host squad owns CurrentPageReference, toast event, LMS, and resource-loading behavior inside `/apex/<PageName>`.
- Review squad runs browser service tests across direct, record, app, and tab pages.

## Implementation Steps

- [x] Replace throwing `lightning/navigation` shim with working exports:
  - `CurrentPageReference`
  - `NavigationMixin.Navigate`
  - `NavigationMixin.GenerateUrl`
- [x] Support PageReference types from the local shell boundary:
  - `standard__app`
  - `standard__navItemPage`
  - `standard__objectPage`
  - `standard__recordPage`
  - `standard__recordRelationshipPage`
  - `standard__component`
  - `standard__quickAction`
  - `standard__webPage`
  - `standard__namedPage`
- [x] Generate local URLs that stay under `/lwc/preview/*` when the destination is local. External web pages return their URL.
- [x] Dispatch route changes through local URL navigation where supported.
- [x] Implement `lightning/platformShowToastEvent` as a browser event shim exposing variant, title, message, mode, and links.
- [x] Implement `lightning/messageService` with `publish`, `subscribe`, `unsubscribe`, `MessageContext`, `APPLICATION_SCOPE`, and local message channel imports.
- [x] Implement `lightning/platformResourceLoader` for local scripts and styles.
- [x] Keep `LightningModal` as a practical local approximation with `open()` and close-value tests; full hosted backdrop and focus-return behavior remain outside this phase.
- [x] Leave quick action context as future work unless the branch adds:
  - record action receives `recordId`.
  - global action receives no record unless state supplies one.
  - unsupported action types return `GLADELWC015 action type unsupported`.
- [x] Add tests for navigation from a record page to supported local routes.
- [x] Add Visualforce Lightning Out tests for `CurrentPageReference`, toast event, LMS publish/subscribe, resource loading, and navigation diagnostics inside `/apex/<PageName>`.

## Verification

```bash
go test ./internal/lwcbrowser ./internal/server ./internal/lwcshell -run 'Navigation|Toast|Message|Action|LWC' -count=1
```

```bash
(cd lwcruntime && npm test -- --runInBand)
```

```bash
(cd lwcruntime && npm run build && node --test test/visualforce-services.test.mjs)
```

Fixture-manifest comparison:

```bash
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --include-hosts lightning-shell,visualforce-lightning-out --targets navigation,quick-action --out /tmp/glade-lwc-navigation-capture.json)
```

## Done Gate

- `CurrentPageReference` emits correct context for direct, record, app, home, and tab pages.
- Navigation mixin generates stable URLs and navigates supported local targets.
- Toast events, LMS, and resource loading pass browser tests.
- Modal and quick-action context stay future work unless this phase adds tests and support rows.
- Unsupported page-reference types and action types have named diagnostics.
- Visualforce Lightning Out host exposes host-appropriate CurrentPageReference and service behavior, with diagnostics where Salesforce behavior is shell-only.
