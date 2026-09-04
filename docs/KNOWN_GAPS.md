# Known Gaps

Generated from the first-party compat plugin capability catalog.

## Known limits of the local runtime

This internal checklist does not establish complete Salesforce parity.

- Required complete: 22/22
- Required incomplete: 0

All required local capability rows are currently `supported`.
No required local support gaps are currently tracked.

## Limits

### `limits.exact-salesforce-accounting`: Exact hosted Salesforce governor accounting

- Status: `unsupported`
- Gap: Exact Salesforce runtime counter deltas require hosted execution accounting and remain outside deterministic local execution.

## Local API server

### `server.rest-breadth.hosted-auth-live-org-deploy`: Hosted auth, live-org Tooling, and deployment services

- Status: `unsupported`
- Gap: OAuth token issuance, live org-only Tooling objects, live metadata deploy/retrieve, and broader hosted REST namespaces require Salesforce services.
