# Phase 0: LWC Baseline And Fixture Manifest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan with parallel subagent squads. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the local LWC shell and Visualforce Lightning Out baselines, fixture project, and fixture-manifest targets before product features move. The command may deploy the fixture project to `oaer-probe-max`; browser DOM and screenshot capture is a later oracle expansion.

**Architecture:** Product tests stay in `glade`; fixture manifests, scratch deploy setup, and generated evidence stay in sibling `glade-tools`. The current LWC capture command writes stable fixture-manifest targets for direct component, record page, app page, home page, tab, Visualforce Lightning Out, wire, Apex, and navigation cases. It does not yet open Salesforce pages or record browser output.

**Tech Stack:** Go, `glade-tools` compat plugin, Salesforce CLI, scratch org alias `oaer-probe-max`, JSON fixture-manifest reports, and later Playwright screenshots where the org page has browser-observable behavior.

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
- Modify in `glade`: `testdata/local-tests/lightning-out-vf/force-app/main/default/pages/MultiWidgetHost.page`
- Modify in `glade`: `testdata/local-tests/lightning-out-vf/force-app/main/default/lwc/*`
- Create in `glade-tools`: `internal/compat/lwc_capture.go`
- Create in `glade-tools`: `internal/compat/lwc_capture_test.go`
- Modify in `glade-tools`: `internal/toolcli/compat_command.go`
- Modify in `glade-tools`: `plugins/compat/plugin.json`

## Parallel Subagent Squads

Use parallel subagent squads where files do not overlap. The coordinator integrates one patch at a time.

- Fixture squad: sample project and seed data.
- Visualforce host squad: Lightning Out fixture page and Visualforce capture targets.
- Capture squad: `go run ./cmd/glade-plugin-compat lwc capture` fixture-manifest report from `../glade-tools`.
- Report squad: JSON schema, text summary, plugin manifest.
- Review squad: run fixture-manifest generation against `oaer-probe-max`, inspect artifacts, and check no product code depends on `glade-tools`.

## Implementation Steps

- [ ] Add the LWC shell fixture project. It must contain three LWCs:
  - `contextProbe`: prints `recordId`, `objectApiName`, `CurrentPageReference.type`, and `CurrentPageReference.attributes`.
  - `recordProbe`: uses `@wire(getRecord)` and `getFieldValue`.
  - `wireProbe`: calls `@salesforce/apex/LwcProbeController.loadItems` through wire and imperative call.
- [ ] Add FlexiPages for `RecordPage`, `AppPage`, and `HomePage`. Each page must put at least two custom LWCs in separate regions and give one component a configured property from `targetConfig`.
- [ ] Add `Lwc_Probe.tab-meta.xml` pointing at the tab-eligible component.
- [ ] Extend `testdata/local-tests/lightning-out-vf` so a Visualforce page mounts the same probe LWCs through `$Lightning.use()` and `$Lightning.createComponent()`. The Visualforce fixture must prove explicit component attrs, Apex wire, record wire, custom events, labels, static resources, and multiple LWC mounts.
- [ ] In `glade-tools`, add `LwcCaptureOptions` with fields:

```go
type LwcCaptureOptions struct {
	Project   string
	TargetOrg string
	Targets   []string
	Hosts     []string
	Out       string
	SkipDeploy bool
}
```

- [ ] Add `LwcCaptureReport` with command, target org, `mode:"fixture-manifest"`, deployed flag, cases, counts, and artifacts. Case names must be stable: `direct-component`, `record-page`, `app-page`, `home-page`, `custom-tab`, `visualforce-lightning-out`, `apex-wire`, `imperative-apex`, `navigation`.
- [ ] Add CLI form:

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --include-hosts lightning-shell,visualforce-lightning-out --out /tmp/glade-lwc-capture.json
```

- [ ] Add `--skip-deploy` for local command tests. It must emit a report with `deployed:false` and fixture target URLs.
- [ ] Use Salesforce CLI through `exec.CommandContext` only inside `glade-tools`. Product `glade` must not shell out to Salesforce CLI for local rendering.
- [ ] Add plugin manifest entries for `compat lwc`.
- [ ] Add text output with this shape:

```text
prepared 9 LWC fixture-manifest targets: prepared=9 pass=0 fail=0 artifacts=/tmp/glade-lwc-capture.json
```

## Verification

```bash
cd ../glade-tools
go test ./internal/compat ./internal/toolcli -run 'Lwc|Plugin' -count=1
```

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --skip-deploy --json
```

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --include-hosts lightning-shell,visualforce-lightning-out --out /tmp/glade-lwc-capture.json
```

This verifies scratch deploy and fixture-manifest output. It is not browser
capture proof until the later oracle step opens the deployed Salesforce pages
and records DOM, console, screenshot, and wire payload evidence.

## Done Gate

- Fixture deploys to `oaer-probe-max` when the scratch org is available.
- JSON fixture manifest contains `mode:"fixture-manifest"` and all nine stable case names, including `visualforce-lightning-out`.
- Fixture-manifest artifacts identify host coverage with `lwc.host.lightning-shell` and `lwc.host.visualforce-lightning-out`.
- Skip-deploy mode passes in tests.
- Product repo has no import or module dependency on `glade-tools`.
- Browser DOM, screenshot, and Salesforce runtime payload capture remain a separate oracle step unless implemented in `glade-tools`.
