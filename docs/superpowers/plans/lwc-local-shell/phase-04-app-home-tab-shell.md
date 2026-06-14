# Phase 4: App, Home, And Tab Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render app pages, home pages, and custom tabs from Salesforce metadata.

**Architecture:** Reuse the direct component shell and FlexiPage renderer, but switch context contracts to app, home, and nav-item page references. Custom tabs resolve either to a tab-eligible LWC or to a FlexiPage target.

**Tech Stack:** Go server routes, `lwcshell` metadata resolver, custom tab metadata parser, LWC runtime, local navigation context.

---

## Feature Delivered

Developers can run dashboard LWCs, home-page LWCs, and tab LWCs in a local Salesforce-like shell.

## Files

- Modify: `internal/gladecli/dev_lwc_command.go`
- Modify: `internal/server/lwc_shell.go`
- Create: `internal/lwcshell/app_page.go`
- Create: `internal/lwcshell/app_page_test.go`
- Modify: `internal/lwcshell/tab.go`
- Modify: `internal/lwcshell/resolve.go`
- Test data: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Sales_Dashboard.flexipage-meta.xml`
- Test data: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Custom_Home.flexipage-meta.xml`
- Test data: `testdata/local-tests/lwc-shell/force-app/main/default/tabs/Lwc_Probe.tab-meta.xml`

## Parallel Squads

- App page squad owns `--app-page`.
- Home page squad owns `--home-page`.
- Tab squad owns `--tab`.
- Route/context squad owns PageReference construction.
- Review squad runs all three shell routes and compares scratch capture.

## Implementation Steps

- [ ] Add CLI flags:

```text
--app-page <flexipageName>
--home-page <flexipageName>
--tab <tabApiName>
--app <appDeveloperName>
```

- [ ] Enforce one render target per command. If more than one target flag is present, return `GLADELWC012 choose one render target`.
- [ ] Add routes:

```text
/lwc/preview/app/<pageName>
/lwc/preview/home/<pageName>
/lwc/preview/tab/<tabApiName>
```

- [ ] Resolve `AppPage` and `HomePage` FlexiPages. They must not require `recordId`.
- [ ] Construct `CurrentPageReference`:
  - App page: `standard__app` when `--app` exists, or `standard__component` for URL-addressable component preview.
  - Home page: `standard__namedPage` with `pageName=home`.
  - Tab: `standard__navItemPage` with `apiName=<tabApiName>`.
- [ ] For custom tab metadata, support Lightning Component tabs that point at an LWC and tabs that point at a FlexiPage. Visualforce and web tabs must return `GLADELWC007 tab target unsupported`.
- [ ] Render app/home page regions with the same region engine from Phase 3.
- [ ] Render tab shell with app nav bar, selected tab marker, and component content.
- [ ] Add tests for tab API name normalization: label `My Custom Tab` maps to `My_Custom_Tab`.

## Verification

```bash
go test ./internal/lwcshell ./internal/server ./internal/gladecli -run 'AppPage|HomePage|Tab|LWC' -count=1
```

```bash
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --app-page Sales_Dashboard --port 18082
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --home-page Custom_Home --port 18083
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --tab Lwc_Probe --port 18084
```

Scratch comparison:

```bash
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --targets app-page,home-page,custom-tab --out /tmp/glade-lwc-app-tab-capture.json)
```

## Done Gate

- App page, home page, and tab routes render from metadata.
- `CurrentPageReference` values match public Salesforce page-reference shapes.
- Unsupported tab types fail with a named diagnostic and a test.
