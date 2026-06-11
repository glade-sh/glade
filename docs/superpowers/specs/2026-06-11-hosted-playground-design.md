# Hosted Playground Design

**Date:** 2026-06-11
**Status:** Approved for implementation planning
**Goal:** Make the hosted Glade playground a polished public demo for Salesforce developers while keeping local playground mode more permissive.

---

## Context

The current playground already has a React UI, built-in examples, a public mode, embedded static assets, and a Go server under `internal/playground`. Public mode enforces several important guardrails:

- strict limit mode
- short run timeout
- forced scratch runs
- disabled seed endpoint
- no public run-result cache writes
- per-IP fixed-window rate limiting
- workspace file and byte caps

That is a good shell. The public experience still needs three changes before it can be the front door:

1. The hosted server must isolate users from each other.
2. The example gallery must show richer Salesforce-shaped projects.
3. The UI must lead with readable code, visible run script, and proof of runtime behavior.

One live issue surfaced during exploration: saved source files can pass `glade check`, but the playground runner can keep using a stale compiled runtime until the process restarts. Hosted multi-file editing depends on fixing runtime invalidation on save, example load, and reset.

## Product Boundary

This remains product work in the `glade` repository:

- playground command behavior
- playground server/session model
- playground UI
- checked-in built-in examples
- product docs and deploy docs

This design does not add compat scanners, maintenance ledgers, or generated support dashboards to base `glade`.

## Hosted Mode

Add a clear hosted mode for production deployment:

```bash
glade playground --hosted --examples --addr 0.0.0.0:${PORT:-8080}
```

`--hosted` is the recommended public deployment flag. Existing `--public` can remain as an alias for one release, but docs and Docker should move to `--hosted`.

Hosted mode defaults:

- bind to `0.0.0.0:${PORT:-8080}` when `--addr` is not set
- force scratch run mode on the server
- force strict limit mode with public caps
- reject persist requests server-side
- disable seed endpoint
- disable local project refs
- disable arbitrary local project loading
- cap request body size
- cap session workspace file count and bytes
- use per-IP and per-session rate limits
- trust `X-Forwarded-For` only when `--trust-proxy` is set
- set cache headers for static assets
- clean expired session workspaces and caches

Local mode stays permissive:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

Local mode keeps project refs, seed, persist mode, database-backed org state, class create/delete, and permissive limit mode unless the user asks otherwise.

## Session Isolation

Hosted mode must not share a single workspace or runner across visitors.

Use a session manager in `internal/playground`:

- create a signed opaque session id for each visitor
- store session id in an HTTP-only, same-site cookie
- do not support URL session tokens in v1
- map each session to a workspace root under a hosted data root
- map each session to its own runner and last scratch org snapshot
- load examples by copying immutable template files into that session workspace
- save files only inside that session workspace
- run anonymous Apex only against that session's files and scratch org
- return `/playground/api/database` from that session's latest scratch org

Session cleanup:

- default TTL: 2 hours after last access
- remove expired workspace directories
- remove expired run caches
- cap total active sessions
- return a clear 429 or 503 error when the instance is full

Recommended flags:

```bash
--session-ttl 2h
--max-sessions 1000
--max-session-bytes 2MB
--trust-proxy
--examples-profile showcase
```

## Showcase Examples

The hosted gallery should contain several complex but synchronous examples. No batch, queueable, future, or schedulable examples in hosted v1. Those surfaces need test context or async draining and make the public story less clear.

Each example should have:

- 4 to 7 source files
- ordered source tabs
- one always-visible `anonymous.apex` run script
- tags for touched Salesforce surfaces
- expected proof summary before first run
- stable logs and org diff after run

### Deal Desk Workflow

Default hosted example.

Files:

- `OpportunityService.cls`
- `OpportunitySelector.cls`
- `DiscountPolicy.cls`
- `DealDeskReport.cls`
- `OpportunityTrigger.trigger`
- `anonymous.apex`

Surfaces:

- classes
- trigger
- DML
- SOQL
- limits
- org diff
- standard objects: Account, Opportunity

Flow:

1. Anonymous Apex creates an Account and two Opportunities.
2. Trigger normalizes fields on insert.
3. Service applies discount policy.
4. Selector reads back Opportunities.
5. Report prints totals and policy labels.
6. Output shows logs, DML rows, SOQL rows, and org diff.

### Support Case Triage

Files:

- `CaseIntakeService.cls`
- `ContactSelector.cls`
- `SlaPolicy.cls`
- `CaseSummary.cls`
- `CaseTrigger.trigger`
- `anonymous.apex`

Surfaces:

- Account, Contact, Case
- trigger
- relationship fields
- SOQL filters
- DML
- limits
- org diff

Flow:

1. Anonymous Apex creates an Account and Contact.
2. Service opens several Cases.
3. Trigger assigns priority and status.
4. SLA policy class computes tier.
5. Summary class queries counts by priority.

### Transaction Rollback Drill

Files:

- `InvoiceService.cls`
- `InvoiceLineService.cls`
- `CreditPolicy.cls`
- `InvoiceReport.cls`
- `anonymous.apex`

Surfaces:

- savepoint
- rollback
- DML
- SOQL
- runtime error path or handled failure path
- org diff

Flow:

1. Service creates parent and line records.
2. Credit policy fails one branch.
3. Service rolls back the failed branch.
4. Report proves only the valid rows remain.

### Custom Object Fulfillment

Files:

- `objects/Shipment__c/Shipment__c.object-meta.xml`
- `objects/Shipment__c/fields/Status__c.field-meta.xml`
- `objects/Shipment__c/fields/TrackingNumber__c.field-meta.xml`
- `ShipmentService.cls`
- `ShipmentSelector.cls`
- `ShipmentReport.cls`
- `anonymous.apex`

Surfaces:

- project metadata load
- custom object
- custom fields
- DML
- SOQL
- org diff

Flow:

1. Glade loads custom object metadata from the workspace.
2. Anonymous Apex creates shipments.
3. Service updates status.
4. Selector and report read rows by status.

### Permission-Aware Service

Files:

- `UserContext.cls`
- `PermissionPolicy.cls`
- `AccountVisibilityService.cls`
- `VisibilityReport.cls`
- `anonymous.apex`

Surfaces:

- UserInfo
- deterministic platform user/profile data
- service branch behavior
- SOQL
- logs

Flow:

1. Anonymous Apex asks UserInfo for current user/profile facts.
2. Policy class chooses an access branch.
3. Service reads Accounts through that branch.
4. Report prints the selected branch and queried records.

### Validation Failure Path

Files:

- `OrderService.cls`
- `OrderPolicy.cls`
- `OrderReport.cls`
- `anonymous.apex`

Surfaces:

- clean diagnostics
- handled exception
- DML before failure
- final org diff

Flow:

1. Anonymous Apex creates a valid record.
2. Service attempts an invalid operation.
3. Policy returns a concrete error.
4. Problems panel shows a clear message and location when available.

## UI Design

Default hosted screen is a demo workbench, not a file manager.

### Left Pane

Showcase gallery. Each card includes:

- example name
- one-sentence purpose
- tags for Salesforce surfaces
- loaded state

The file tree moves behind Advanced.

### Center Pane

Project source tabs sit above the source editor. Tabs include project files only:

```text
OpportunityService.cls | OpportunitySelector.cls | DiscountPolicy.cls | OpportunityTrigger.trigger | DealDeskReport.cls
```

The selected source editor sits below the tabs.

`Execute Anonymous` is a fixed lower pane. It is always visible because it is the script that ties the example together. It does not live in the source tab row.

Desktop layout:

```text
[source tabs]
[selected project source editor]
[execute anonymous editor]
```

Mobile layout:

```text
[source tabs]
[selected source]
[execute anonymous]
[output]
```

On mobile, the gallery collapses before the source or output panes.

### Right Pane

The output pane starts with a proof strip:

- status
- total runtime
- DML statements and rows
- SOQL queries and rows
- org diff count
- touched surfaces

Before first run, the output pane should not be empty. It should show the expected proof for the selected example. After run, it shows measured results.

Tabs:

- Logs
- Org diff
- Limits
- Problems

Advanced adds:

- file tree
- database browser
- trace JSON
- command palette
- class create/delete in local mode
- seed controls in local mode
- persist toggle in local mode

Hosted mode hides local-only controls even if Advanced is enabled.

## Security And Abuse Controls

Hosted mode accepts arbitrary Apex from the public internet. It is not a Go process sandbox. Deploy it as an unprivileged container or systemd service with OS-level limits.

Controls:

- run as non-root
- no write access outside hosted data root
- strict request body limit
- strict workspace byte limit
- strict file count limit
- short run timeout
- strict VM governor caps
- async caps remain zero in hosted v1
- per-session mutation rate limit
- per-IP mutation rate limit
- session TTL cleanup
- no persisted org state
- no seed endpoint
- no local project references
- no project path supplied from requests
- `X-Forwarded-For` honored only behind trusted proxy mode

Recommended reverse proxy:

- terminate TLS at Caddy, nginx, or DigitalOcean Load Balancer
- set security headers
- gzip or brotli static assets
- cache immutable assets by hashed name when build output supports it
- route health check to `/playground/`

## DigitalOcean Deployment Recommendation

Current DigitalOcean pricing checked on 2026-06-11:

- Basic 4 GiB / 2 vCPU / 80 GiB SSD / 4,000 GiB transfer: $24/month.
- Basic 8 GiB / 4 vCPU / 160 GiB SSD / 5,000 GiB transfer: $48/month.
- CPU-Optimized 4 GiB / 2 dedicated vCPU / 25 GiB SSD / 4,000 GiB transfer: $42/month.
- General Purpose 8 GiB / 2 dedicated vCPU / 25 GiB SSD / 4,000 GiB transfer: $63/month.

Sources:

- https://www.digitalocean.com/pricing/droplets
- https://docs.digitalocean.com/products/droplets/concepts/choosing-a-plan/
- https://docs.digitalocean.com/products/droplets/details/pricing/

Recommended starting Droplet:

```text
Basic Premium AMD or Intel
4 GiB RAM
2 vCPU
80 GiB SSD
4,000 GiB transfer
Ubuntu LTS
```

Reason:

- The playground is bursty.
- Hosted caps keep individual runs short.
- Session data is small and temporary.
- The static bundle is modest.
- CPU contention is acceptable at first if rate limits are set.

Start with one $24/month Droplet and a Docker or systemd deployment. Put Caddy or nginx in front of it. Measure for one week.

Scale-up line:

- If CPU is pegged during ordinary traffic, move to CPU-Optimized 2 vCPU at $42/month.
- If memory pressure or session count is the limit, move to Basic 8 GiB / 4 vCPU at $48/month.
- If public launch traffic is expected, use two 4 GiB Basic Droplets behind a DigitalOcean Load Balancer and keep sessions sticky by cookie.

Initial process limits:

```text
GOMAXPROCS=2
--session-ttl 2h
--max-sessions 500
--max-session-bytes 2MB
--rate-per-minute 30
--run-timeout 5s
```

For a single 4 GiB Droplet, set a conservative hosted session target:

- 500 active sessions on disk
- 20 to 40 concurrent runs
- 30 mutating requests per minute per IP
- 2 MiB workspace cap per session

These are starting numbers, not promises. Load test before sending launch traffic.

## Bandwidth Plan

The current embedded playground static files are small enough for one Droplet, but asset caching should still be improved.

Required changes:

- fingerprint built assets where possible
- serve JS/CSS with long-lived cache headers when fingerprinted
- keep `index.html` short-cache or no-cache
- gzip static assets at reverse proxy or Go server
- avoid sending full file contents in workspace metadata
- read file contents only when a tab opens
- keep database browser row cap

DigitalOcean inbound bandwidth is free. Outbound transfer is included by plan and billed after the pool is exceeded. With a 4 GiB Basic Droplet, 4,000 GiB transfer is enough for a public playground unless a crawler or abusive client pounds full reloads. Edge caching and rate limits are still worth the trouble.

## Command And Docs Changes

CLI help should distinguish local and hosted use:

```bash
glade playground --open
glade playground --project . --db .glade/playground/org.sqlite --open
glade playground --hosted --examples --addr 0.0.0.0:${PORT:-8080}
```

Docs to update:

- `docs/PLAYGROUND_HOSTING.md`
- `site/docs-src/guide/playground.md`
- `site/docs-src/guide/cli-reference.md`
- `docs/INSTALL.md`
- `docs/EDITOR.md`
- `Dockerfile`
- `.do/app.yaml`

The public docs must say:

- hosted playground does not persist org state
- examples are copied into an isolated session
- local playground is the right place for persistent experiments
- public examples avoid async/batch by design

## Testing

Focused backend tests:

```bash
go test ./internal/playground
go test ./internal/gladecli
```

Web tests:

```bash
cd internal/playground/web
npm test
npm run build
```

Broader gates:

```bash
go test ./...
scripts/smoke.sh
```

Browser checks:

- hosted default example loads
- source tabs switch without losing dirty content
- anonymous pane remains visible
- run result updates proof strip
- hosted mode hides persist/seed/local controls
- two browser sessions cannot see each other's files or rows
- mobile layout keeps source, anonymous, and output readable

Security checks:

- public persist request is rejected server-side
- seed endpoint returns forbidden in hosted mode
- local project reference cannot be loaded in hosted mode
- path traversal fails
- oversized file save fails
- session workspace cap fails cleanly
- rate limit returns 429
- stale runtime is invalidated after source save

## Non-Goals

- No batch, future, queueable, or scheduled Apex examples in hosted v1.
- No user accounts.
- No long-term hosted persistence.
- No collaborative editing.
- No compatibility scanner or maintenance dashboard in base `glade`.
- No dependence on `glade-tools`.
