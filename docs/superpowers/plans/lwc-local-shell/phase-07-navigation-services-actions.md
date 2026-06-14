# Phase 7: Navigation, Services, And Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement local shell services for navigation, current page reference, toasts, Lightning Message Service, modals, and quick actions.

**Architecture:** Browser modules communicate with a shell service object installed by the page bootstrap. The service owns route changes, URL generation, toast rendering, message channels, modal lifecycle, and action context.

**Tech Stack:** `lwcruntime` shell service, Go PageReference construction, `@salesforce/messageChannel` shims, server routes, browser tests.

---

## Feature Delivered

Components that use standard shell services can be tested locally in record, app, home, and tab contexts.

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
- Test data: `testdata/local-tests/lwc-shell/force-app/main/default/messageChannels/LwcProbe.messageChannel-meta.xml`

## Parallel Squads

- Navigation squad owns `NavigationMixin`, `CurrentPageReference`, and URL generation.
- Toast/modal squad owns visible shell overlays.
- LMS squad owns message channel metadata and publish/subscribe.
- Quick action squad owns record and global action contexts.
- Review squad runs browser service tests across direct, record, app, and tab pages.

## Implementation Steps

- [ ] Replace throwing `lightning/navigation` shim with working exports:
  - `CurrentPageReference`
  - `NavigationMixin.Navigate`
  - `NavigationMixin.GenerateUrl`
- [ ] Support PageReference types from the local shell boundary:
  - `standard__app`
  - `standard__navItemPage`
  - `standard__objectPage`
  - `standard__recordPage`
  - `standard__recordRelationshipPage`
  - `standard__component`
  - `standard__quickAction`
  - `standard__webPage`
  - `standard__namedPage`
- [ ] Generate local URLs that stay under `/lwc/preview/*` when the destination is local. External web pages return their URL.
- [ ] Dispatch route changes inside the same shell without a full server restart.
- [ ] Implement `lightning/platformShowToastEvent`. Toasts must render in the shell and expose variant, title, message, mode, and links.
- [ ] Implement `lightning/messageService` with `publish`, `subscribe`, `unsubscribe`, `MessageContext`, `APPLICATION_SCOPE`, and local message channel imports.
- [ ] Implement a practical `LightningModal` shim with `open()`, close value, backdrop, and focus return.
- [ ] Implement quick action context:
  - record action receives `recordId`.
  - global action receives no record unless state supplies one.
  - unsupported action types return `GLADELWC015 action type unsupported`.
- [ ] Add tests for navigation from a record page to object home, record view, app page, and tab page.

## Verification

```bash
go test ./internal/lwcbrowser ./internal/server ./internal/lwcshell -run 'Navigation|Toast|Message|Action|LWC' -count=1
```

```bash
(cd lwcruntime && npm test -- --runInBand)
```

Scratch comparison:

```bash
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --targets navigation,quick-action --out /tmp/glade-lwc-navigation-capture.json)
```

## Done Gate

- `CurrentPageReference` emits correct context for direct, record, app, home, and tab pages.
- Navigation mixin changes local shell routes and generates stable URLs.
- Toasts, LMS, modals, and quick-action context pass browser tests.
- Unsupported page-reference types and action types have named diagnostics.
