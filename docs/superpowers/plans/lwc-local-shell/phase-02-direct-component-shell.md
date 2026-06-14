# Phase 2: Direct Component Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan with parallel subagent squads. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `glade dev lwc` for isolated local LWC preview with hot reload, shell context, route-discovered previews, and browser-testable routes.

**Architecture:** Product server gets `/lwc/preview/component/<namespace>/<name>`. The route uses `lwcshell` target resolution, `lwcbrowser` compile manifest, and a small shell HTML template that boots the component inside Salesforce-like chrome.

**Tech Stack:** Go HTTP handlers, existing `internal/lwcbrowser.PreparePageConfig`, existing `lwcruntime`, project watch reload, Playwright tests where available.

---

## Feature Delivered

Developers can preview a single LWC without needing Visualforce and without deploying to Salesforce. The new direct shell must not break the existing Visualforce Lightning Out host.

## Files

- Create: `internal/server/lwc_shell.go`
- Create: `internal/server/lwc_shell_test.go`
- Create: `internal/gladecli/dev_lwc_command.go`
- Create: `internal/gladecli/dev_lwc_command_test.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/lwcbrowser/bootstrap.go`
- Modify: `internal/lwcbrowser/project.go`
- Modify: `lwcruntime/src/glade.out.mjs`
- Test data: `testdata/local-tests/lwc-shell`

## Parallel Subagent Squads

Use parallel subagent squads where files do not overlap. The coordinator integrates one patch at a time.

- CLI squad owns `dev_lwc_command.go` and help text.
- Server squad owns route handling and shell HTML.
- Runtime squad owns bootstrap config and component mount.
- Visualforce host squad owns regression tests that prove `$Lightning.use()` and `$Lightning.createComponent()` still mount LWCs under `/apex/<PageName>`.
- Watch squad owns reload behavior.
- Review squad runs server tests and browser smoke.

## Implementation Steps

- [ ] Add `glade dev lwc` under `runDev` with server flags:

```text
--project <root>
--port <port>
--addr <host:port>
--ready-file <path>
```

- [ ] Keep `glade dev vf` untouched except shared helpers that already exist.
- [ ] Add startup output:

```text
LWC dev shell: http://127.0.0.1:8080
Routes:
  /lwc/preview/component/c/contextProbe
Watching <root> for lwc, flexipage, tab, Visualforce, Apex, and static resource changes.
```

- [ ] Add `Server.HandleLWCShell` with route parsing and method checks. Unknown routes must return Glade JSON error, not a blank page.
- [ ] Add shell context sent to the browser. It must include namespace, component tag, route-derived public properties, `PageContext`, import map, manifest, and diagnostics.
- [ ] Update browser bootstrap so it can mount a component by tag without `$Lightning.createComponent`.
- [ ] Inject `recordId` and `objectApiName` as public properties when query parameters or page context are present.
- [ ] Add a right-side dev context panel only in local shell pages. It shows target, page type, record id, object, form factor, and diagnostics.
- [ ] Hot reload must rebuild source metadata, reset Lightning cache, and report the count of changed files. Do not restart the process.
- [ ] `--ready-file` writes the bound URL and discovered routes for scripts.

## Verification

```bash
go test ./internal/gladecli ./internal/server ./internal/lwcbrowser -run 'LWC|Dev' -count=1
```

```bash
go test ./internal/lwcruntime -count=1
```

```bash
(cd lwcruntime && npm run build && node --test test/visualforce-dev-server.test.mjs)
```

```bash
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --port 18080
```

Browser smoke path:

```text
http://127.0.0.1:18080/lwc/preview/component/c/contextProbe
```

## Done Gate

- Direct component route renders a custom LWC.
- Public properties and context are visible to the component.
- Navigation is still allowed to be unsupported here, but it must fail with `GLADELWC` diagnostics and a visible context panel entry.
- File changes under `lwc`, FlexiPages, tabs, Visualforce, Apex, and static resources reload the preview.
- Existing Visualforce Lightning Out fixture pages still render LWCs through `/apex/<PageName>`.
