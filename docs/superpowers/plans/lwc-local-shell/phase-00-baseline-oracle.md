# Phase 0: LWC Baseline And Scratch Oracle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the local LWC shell baseline and scratch-org oracle captures before product features move.

**Architecture:** Product tests stay in `glade`; scratch capture and generated evidence stay in sibling `glade-tools`. The oracle uses `oaer-probe-max` to capture real Salesforce behavior for direct component, record page, app page, home page, tab, wire, Apex, and navigation cases.

**Tech Stack:** Go, `glade-tools` compat plugin, Salesforce CLI, scratch org alias `oaer-probe-max`, JSON capture reports, Playwright screenshots where the org page has browser-observable behavior.

---

## Feature Delivered

This phase delivers no user-facing rendering feature. It delivers the scoreboard and sample project that every rendering phase uses.

## Files

- Create in `glade`: `testdata/local-tests/lwc-shell/sfdx-project.json`
- Create in `glade`: `testdata/local-tests/lwc-shell/force-app/main/default/lwc/contextProbe/*`
- Create in `glade`: `testdata/local-tests/lwc-shell/force-app/main/default/lwc/recordProbe/*`
- Create in `glade`: `testdata/local-tests/lwc-shell/force-app/main/default/lwc/wireProbe/*`
- Create in `glade`: `testdata/local-tests/lwc-shell/force-app/main/default/classes/LwcProbeController.cls`
- Create in `glade`: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml`
- Create in `glade`: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Sales_Dashboard.flexipage-meta.xml`
- Create in `glade`: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Custom_Home.flexipage-meta.xml`
- Create in `glade`: `testdata/local-tests/lwc-shell/force-app/main/default/tabs/Lwc_Probe.tab-meta.xml`
- Create in `glade-tools`: `internal/compat/lwc_capture.go`
- Create in `glade-tools`: `internal/compat/lwc_capture_test.go`
- Modify in `glade-tools`: `internal/toolcli/compat_command.go`
- Modify in `glade-tools`: `plugins/compat/plugin.json`

## Parallel Squads

- Fixture squad: sample project and seed data.
- Capture squad: `glade compat lwc capture`.
- Report squad: JSON schema, text summary, plugin manifest.
- Review squad: run capture against `oaer-probe-max`, inspect artifacts, and check no product code depends on `glade-tools`.

## Implementation Steps

- [ ] Add the LWC shell fixture project. It must contain three LWCs:
  - `contextProbe`: prints `recordId`, `objectApiName`, `CurrentPageReference.type`, and `CurrentPageReference.attributes`.
  - `recordProbe`: uses `@wire(getRecord)` and `getFieldValue`.
  - `wireProbe`: calls `@salesforce/apex/LwcProbeController.loadItems` through wire and imperative call.
- [ ] Add FlexiPages for `RecordPage`, `AppPage`, and `HomePage`. Each page must put at least two custom LWCs in separate regions and give one component a configured property from `targetConfig`.
- [ ] Add `Lwc_Probe.tab-meta.xml` pointing at the tab-eligible component.
- [ ] In `glade-tools`, add `LwcCaptureOptions` with fields:

```go
type LwcCaptureOptions struct {
	Project   string
	TargetOrg string
	Targets   []string
	Out       string
	SkipDeploy bool
}
```

- [ ] Add `LwcCaptureReport` with command, target org, deployed flag, cases, counts, and artifacts. Case names must be stable: `direct-component`, `record-page`, `app-page`, `home-page`, `custom-tab`, `apex-wire`, `imperative-apex`, `navigation`.
- [ ] Add CLI form:

```bash
glade compat lwc capture --target-org oaer-probe-max --project testdata/local-tests/lwc-shell --out /tmp/glade-lwc-capture.json
```

- [ ] Add `--skip-deploy` for local command tests. It must emit a report with `deployed:false` and fixture target URLs.
- [ ] Use Salesforce CLI through `exec.CommandContext` only inside `glade-tools`. Product `glade` must not shell out to Salesforce CLI for local rendering.
- [ ] Add plugin manifest entries for `compat lwc`.
- [ ] Add text output with this shape:

```text
captured 8 LWC targets: pass=8 fail=0 artifacts=/tmp/glade-lwc-capture.json
```

## Verification

```bash
go test ./internal/compat ./internal/toolcli -run 'Lwc|Plugin' -count=1
```

```bash
go run ./cmd/glade-plugin-compat lwc capture --project ../glade/testdata/local-tests/lwc-shell --skip-deploy --json
```

```bash
go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --out /tmp/glade-lwc-capture.json
```

## Done Gate

- Fixture deploys to `oaer-probe-max`.
- JSON capture contains all eight stable case names.
- Skip-deploy mode passes in tests.
- Product repo has no import or module dependency on `glade-tools`.
