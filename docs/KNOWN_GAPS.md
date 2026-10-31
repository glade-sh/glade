# Known Gaps

Generated from the first-party compat plugin capability catalog.

## Known limits of the local runtime

This checklist does not establish complete Salesforce parity.

- Required complete: 22/22
- Required incomplete: 0

All required local capability rows are currently `supported`.
No required local support gaps are currently tracked.

## Automation

### `automation.assignment.scope`: Assignment rule scope

- Status: `unsupported`
- Gap: The local assignment rule implementation is bounded to the supported Lead and Case metadata and checked selection paths; it does not establish full hosted assignment behavior.

## Automation

### `automation.duplicate.matching`: Duplicate matching scope

- Status: `unsupported`
- Gap: Local duplicate matching does not enforce fuzzy, cross-object or Person Account matching.

## Automation

### `automation.flow.hosted`: Hosted Flow and unsupported nodes

- Status: `unsupported`
- Gap: Screen and wait nodes, after-delete Flow and hosted scheduling are outside the local Flow contract.

## Events

### `events.change.delivery`: Change-event hosted delivery

- Status: `unsupported`
- Gap: Local change-event journal and subscriber execution do not establish hosted event streaming or delivery timing.

## Limits

### `limits.exact-salesforce-accounting`: Exact hosted Salesforce governor accounting

- Status: `unsupported`
- Gap: Exact Salesforce runtime counter deltas require hosted execution accounting and remain outside deterministic local execution.

## Local API server

### `server.rest-breadth.hosted-auth-live-org-deploy`: Hosted auth, live-org Tooling, and deployment services

- Status: `unsupported`
- Gap: OAuth token issuance, live org-only Tooling objects, live metadata deploy/retrieve, and broader hosted REST namespaces require Salesforce services.

## Platform

### `platform.exclusions`: Excluded platforms

- Status: `unsupported`
- Gap: DataWeave, ConnectApi and Slack are excluded and unsupported.

## Security

### `security.default-permissions.scope`: Default permission scope

- Status: `unsupported`
- Gap: Local default permission behavior covers the checked configurations and does not establish full hosted permission parity.
