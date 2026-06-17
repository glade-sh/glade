# Generated LWC Shell Support

Prepared support rows come from `glade-plugin-compat lwc capture --target-org
oaer-probe-max --include-hosts lightning-shell,visualforce-lightning-out`,
which writes a local JSON report such as
`/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json`.

This artifact records LWC shell support rows from the latest strict local
browser oracle. Rows marked `supported-local` have passing local browser
evidence. Rows marked `supported` require passing local and Salesforce browser
comparison evidence. A strict local browser oracle captures local Glade shell
DOM for every checked target. A separate two-sided browser oracle captures
local Glade shell DOM and live Salesforce DOM for deployed Lightning paths:

```bash
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org oaer-probe-max \
  --project /Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell \
  --include-hosts lightning-shell,visualforce-lightning-out \
  --skip-deploy \
  --local-browser-capture \
  --glade-bin /tmp/glade-lwc-shell-bin \
  --out /tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json
```

Two-sided browser proof still uses the smaller deployed Lightning path set:

```bash
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org oaer-probe-max \
  --project /Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell \
  --targets app-page,custom-tab,url-addressable-component \
  --local-browser-capture \
  --glade-bin /tmp/glade-lwc-shell-bin \
  --browser-capture \
  --out /tmp/glade-lwc-two-sided-browser-check.json
```

Latest local browser proof: all 21 LWC shell targets passed against the local
shell with no browser console errors or page errors. The report stores stable
paths only and does not write transient server URLs. Latest two-sided browser
proof remains `app-page`, `custom-tab`, and `url-addressable-component`
against both the local shell and `oaer-probe-max`, with no browser console
errors or page errors. Each two-sided case includes a selector-scoped
`comparison` block for normalized visible text and project LWC component counts.
The app-page target passes inside `c-wire-probe`, the custom-tab target passes
inside `c-context-probe`, and the URL-addressable target passes inside
`c-action-probe`. The app-page and custom-tab checks use
`/lightning/app/c__Lwc_Shell/n/Lwc_Probe` after `Lwc_Shell_Access` is assigned.
Record pages, quick actions, and Visualforce Lightning Out remain org-setup
dependent browser-oracle targets.

The evidence column names the report path produced by the capture command. The
report itself is an external run artifact, not a checked-in JSON file.

| Feature | Host | Status | Evidence | Notes |
| --- | --- | --- | --- | --- |
| `lwc.host.lightning-shell` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.host.lightning-shell/lightning-shell` | Host lane has passing local browser evidence; live Salesforce parity remains target-specific. |
| `lwc.host.visualforce-lightning-out` | `visualforce-lightning-out` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.host.visualforce-lightning-out/visualforce-lightning-out` | Host lane has passing local browser evidence; live Salesforce parity remains target-specific. |
| `lwc.target.direct-component` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.target.direct-component/lightning-shell` | Direct component route mounts the context probe bundle. |
| `lwc.target.record-page` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.target.record-page/lightning-shell` | Record page route resolves Account_Record_Page and passes record context. |
| `lwc.target.app-page` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.target.app-page/lightning-shell` | App page route resolves Sales_Dashboard and dashboard region components. |
| `lwc.target.home-page` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.target.home-page/lightning-shell` | Home route resolves Custom_Home with the local home template. |
| `lwc.target.custom-tab` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.target.custom-tab/lightning-shell` | Custom tab route resolves Lwc_Probe to its flexipage. |
| `lwc.target.url-addressable-component` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.target.url-addressable-component/lightning-shell` | URL state is preserved in the target metadata for later PageReference comparison. |
| `lwc.target.record-quick-action` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.target.record-quick-action/lightning-shell` | Quick action metadata points at c:actionProbe. |
| `lwc.target.visualforce-lightning-out` | `visualforce-lightning-out` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.target.visualforce-lightning-out/visualforce-lightning-out` | Visualforce host target records the shared Lightning Out runtime lane. |
| `lwc.service.apex-wire` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.apex-wire/lightning-shell` | wireProbe imports @salesforce/apex/LwcProbeController.wireAccounts. |
| `lwc.service.imperative-apex` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.imperative-apex/lightning-shell` | wireProbe imports @salesforce/apex/LwcProbeController.imperativeAccount. |
| `lwc.service.lds-read` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.lds-read/lightning-shell` | Local UI API covers getRecord and getRecords with local data limits. Object-info compatibility exports remain available from lightning/uiRecordApi. |
| `lwc.service.ui-object-info` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.ui-object-info/lightning-shell` | Local `lightning/uiObjectInfoApi` covers getObjectInfo, getObjectInfos, getPicklistValues, and getPicklistValuesByRecordType with local schema limits. objectInfoProbe verifies the browser wire path. |
| `lwc.service.ui-related-list` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.ui-related-list/lightning-shell` | Local `lightning/uiRelatedListApi` getRelatedListRecords returns deterministic child rows from local relationships. relatedListProbe verifies the browser wire path; related-list metadata adapters remain a Salesforce check. |
| `lwc.service.lds-create-defaults` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.lds-create-defaults/lightning-shell` | Local UI API covers getRecordCreateDefaults plus create and update record-input helpers for common local form flows. Create defaults include project `.layout-meta.xml` field sections when present, with a generated full layout from createable fields as the local fallback. |
| `lwc.service.ui-layout` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.ui-layout/lightning-shell` | Local `lightning/uiLayoutApi` getLayout returns the same Record Layout shape as create defaults. layoutProbe verifies the browser wire path. |
| `lwc.service.lds-mutation` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.lds-mutation/lightning-shell` | Local server and runtime tests cover createRecord, updateRecord, deleteRecord, optional fields, soft-delete read misses, refreshApex, and notifyRecordUpdateAvailable. Salesforce browser DOM diff remains pending. |
| `lwc.service.navigation` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.navigation/lightning-shell` | Runtime browser tests cover GenerateUrl, Navigate, CurrentPageReference, and local route diagnostics; strict capture proves the shell route loads without browser errors. |
| `lwc.service.toast` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.toast/lightning-shell` | Runtime browser tests cover ShowToastEvent capture and rendered shell toast text; strict capture proves the shell route loads without browser errors. |
| `lwc.service.lms` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.lms/lightning-shell` | Runtime browser tests cover local Lightning Message Service publish, subscribe, and unsubscribe; strict capture proves the shell route loads without browser errors. |
| `lwc.service.base-components` | `lightning-shell` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.base-components/lightning-shell` | Practical base shims render common lightning/* modules and cover click, change, submit, LDS-backed record form read and success/error submit events, datatable rowaction, tab active, and unsupported-attribute diagnostics. |
| `lwc.service.base-components` | `visualforce-lightning-out` | supported-local | `/tmp/glade-lwc-full-shell-local-browser-strict-record-forms.json#/support/lwc.service.base-components/visualforce-lightning-out` | Visualforce Lightning Out mounts the same practical base component shims through c:baseComponentHost. |
