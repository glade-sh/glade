# Phase 4: App, Home, And Tab Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan with parallel subagent squads. Steps use checkbox (`- [x]`) syntax for tracking.

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

## Parallel Subagent Squads

Use parallel subagent squads where files do not overlap. The coordinator integrates one patch at a time.

- App page squad owns `/lwc/preview/app/<Page>`.
- Home page squad owns `/lwc/preview/home/<Page>`.
- Tab squad owns `/lwc/preview/tab/<Tab>`.
- Route/context squad owns PageReference construction.
- Review squad runs all three shell routes and compares fixture-manifest output.

## Implementation Steps

- [x] Add startup route discovery for:

```text
/lwc/preview/app/<flexipageName>
/lwc/preview/home/<flexipageName>
/lwc/preview/tab/<tabApiName>
```

- [x] Keep `glade dev lwc` as a server command. Render target selection happens by URL route.
- [x] Add routes:

```text
/lwc/preview/app/<pageName>
/lwc/preview/home/<pageName>
/lwc/preview/tab/<tabApiName>
```

- [x] Resolve `AppPage` and `HomePage` FlexiPages. They must not require `recordId`.
- [x] Construct `CurrentPageReference`:
  - App page: `standard__app` when `--app` exists, or `standard__component` for URL-addressable component preview.
  - Home page: `standard__namedPage` with `pageName=home`.
  - Tab: `standard__navItemPage` with `apiName=<tabApiName>`.
- [x] For custom tab metadata, support Lightning Component tabs that point at an LWC and tabs that point at a FlexiPage. Visualforce tabs must redirect to `/apex/<Page>` so Lightning Out stays in the Visualforce host. Web and object tabs must return `GLADELWC007 tab target unsupported`.
- [x] Render app/home page regions with the same region engine from Phase 3.
- [x] Render tab shell with app nav bar, selected tab marker, and component content.
- [x] Add tests for tab API name normalization: label `My Custom Tab` maps to `My_Custom_Tab`.

## Verification

```bash
go test ./internal/lwcshell ./internal/server ./internal/gladecli -run 'AppPage|HomePage|Tab|LWC' -count=1
```

```bash
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --port 18082
# then open:
# http://127.0.0.1:18082/lwc/preview/app/Sales_Dashboard
# http://127.0.0.1:18082/lwc/preview/home/Custom_Home
# http://127.0.0.1:18082/lwc/preview/tab/Lwc_Probe
```

Fixture-manifest comparison:

```bash
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --targets app-page,home-page,custom-tab --out /tmp/glade-lwc-app-tab-capture.json)
```

## Done Gate

- App page, home page, and tab routes render from metadata.
- `CurrentPageReference` values match public Salesforce page-reference shapes.
- Unsupported tab types fail with a named diagnostic and a test.
- Visualforce Lightning Out remains a separate host. A custom tab that points to Visualforce redirects to `/apex/<Page>` and uses the Visualforce Lightning Out host.
