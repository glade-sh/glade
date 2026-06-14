# Support map

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Support</p>
  <p>Use this page to decide what Glade can run locally, what works with named limits, and what still needs Salesforce or a plugin.</p>
  <ul>
    <li>Start with the status legend.</li>
    <li>Check the unsupported list before a pilot.</li>
    <li>Use the generated ledgers for exact method rows.</li>
  </ul>
</div>

Start here when deciding whether Glade can run a project, a test class, or a
local Salesforce-shaped API flow. This page is the public map. The generated
ledgers carry the exact ledger rows.

## Before you adopt Glade

- Your local loop uses supported Apex parse, check, test, SOQL, DML, trigger, and SObject paths.
- Your test suite can mock callouts and live side effects.
- Your project can tolerate explicit unsupported diagnostics for Salesforce-hosted services.
- You will keep a Salesforce org gate for features Glade does not model.
- You will use first-party plugins for support ledgers and advisory scans instead of expecting those scanners in base `glade --help`.

## Status key

<div class="docs-support-legend" aria-label="Support status legend">
  <span class="docs-status-chip docs-status-supported">Works well</span>
  <span class="docs-status-chip docs-status-partial">Works with limits</span>
  <span class="docs-status-chip docs-status-unsupported">Not supported</span>
  <span class="docs-status-chip docs-status-unknown">Not measured</span>
</div>

| Status | Meaning |
| --- | --- |
| <span class="docs-status-chip docs-status-supported">Works well</span> | Implemented, fixture-backed, and fit for normal local use. |
| <span class="docs-status-chip docs-status-partial">Works with limits</span> | Common local paths work. The limits are named. |
| <span class="docs-status-chip docs-status-unsupported">Not supported</span> | Glade should stop with a stable unsupported diagnostic. |
| <span class="docs-status-chip docs-status-unknown">Not measured</span> | A row exists, but the team has not measured it yet. |

## Works Well

These areas are the main local development contract.

| Area | What to expect |
| --- | --- |
| Apex parsing and project indexing | Large SFDX projects, nested types, namespace tokens, and stable parse diagnostics. |
| Semantic checks | Type references, inheritance, interfaces, overloads, locals, assignments, return paths, and token ranges for the supported VM subset. |
| Local Apex tests | `@isTest`, `@TestSetup`, isolated org state, static reset, governor windows, async drain, stack frames, JSON, and JUnit output. |
| SOQL, DML, triggers, and SObjects | Static and dynamic SOQL, DML statements, `Database.*` result shapes, trigger context, schema-backed SObjects, and local SQLite-backed storage. |
| Local API server | Salesforce-shaped REST discovery, SObject CRUD, query/queryAll, limits and record counts, userinfo stubs, Tooling `executeAnonymous`, local Tooling source/schema metadata queries, Composite sObject insert, reset endpoints, and optional SQLite persistence. |
| Editor and debug tools | LSP diagnostics, symbols, hover, completion, rename, semantic tokens, DAP stepping, watch mode, and trace/profile reports. |

## Works with limits

These areas cover useful local test paths. They are not full Salesforce
service parity.

| Area | Current limit |
| --- | --- |
| Core standard library | Common `System`, `String`, date/time, math, assertions, labels, URLs, user info, and collection paths are covered. Exact method rows live in the standard-library ledger. |
| Schema and describe APIs | Local describe supports checked object, field, record type, child relationship, data category metadata, and generated standard-object shape. Hosted full-org metadata services remain outside the local model. |
| JSON, regex, encoding, and crypto | Serialization, parsing, regex, base64, hex, URL encoding, and digest rows are supported for the checked local contract. |
| HTTP, SOAP, and callout mocks | Request/response-shaped mock paths work for tests. Glade does not perform live outbound service calls. |
| Messaging | Local message/result shapes, template rendering, attachment retrieval, send options, and invocation counts are covered. Glade does not deliver email, push, or other live messages. |
| Visualforce controller and page rendering | PageReference, messages, current page, controller helpers, local `/apex/<PageName>` routes, common standard components, page lifecycle paths, static resources, uploads, remoting envelopes, and local PDF fallback output are modeled for local development. Full Salesforce chrome, every component edge, exact lifecycle timing, and byte-for-byte PDF parity remain outside the local contract. |
| Search and SOSL helpers | Local deterministic Search and SOSL rows are supported. Hosted ranking, analyzers, synonyms, and external indexes are not modeled. |
| Test helpers | Tracked `Test.*` local helper rows are supported. Hosted service accounting, packaged-resource expansion, and live External Service execution remain explicit gaps. |
| Local test harness and request context | Request/UIRequest context, install/uninstall hooks, sandbox post-copy helpers, scheduled Apex, QuickAction DTOs, BusinessHours calendars and holidays, seeded approval routing, and TrailblazerIdentity helper calls have deterministic local models. Live hosted engines are not contacted. |

## Not supported today

This is the smaller list a first user should check before betting on Glade.

| Area | Why it is outside the current local contract |
| --- | --- |
| Live Salesforce auth and sessions | The local server exposes local stubs. It does not implement real Salesforce OAuth, session validation, or org identity services. |
| Fenced live service APIs | Answers zone search, password reset output, live identity/admin mutation, and hosted process/service engines require Salesforce-hosted data or execution. |
| Exact hosted Visualforce parity | Glade serves local Visualforce pages for development. It does not promise Salesforce-hosted chrome, every component edge, exact lifecycle timing, every remoting/browser behavior, or byte-for-byte PDF output. |
| Broad REST and Tooling API parity | The local API server covers the checked local baseline, including Composite Batch and Tree, Bulk API v2 simple query jobs, layout/default-value metadata, metadata job status, and local Tooling shapes. Broader Bulk API locator paging, Composite Graph execution, Streaming/PubSub, GraphQL, live metadata deploy/retrieve, live auth, and live org-only Tooling surfaces remain outside the local contract. |
| Live outbound side effects | Real callouts, delivered email, push notifications, and external service mutations are not performed. Tests should use local mocks and result objects. |
| Exact Salesforce governor accounting | Glade tracks deterministic local limits. Salesforce's full production accounting and every platform-specific counter are not complete. |

Example diagnostic:

```text
UnsupportedFeature: unsupported call "Answers.findSimilar local Answers zone search surface"
```

## Area Detail

| Area | First-layer status | Notes |
| --- | --- | --- |
| Apex front end | <span class="docs-status-chip docs-status-supported">Works well</span> | Parser, project loader, symbols, semantic checks, LSP, and diagnostics form the front door. |
| Runtime and tests | <span class="docs-status-chip docs-status-supported">Works well</span> | VM execution, local tests, SObjects, SOQL, DML, triggers, async drain, and local storage are the core contract. |
| Local Salesforce API | <span class="docs-status-chip docs-status-supported">Works well</span> | Useful for local REST, SObject CRUD/query, record count, Tooling `executeAnonymous`, and local source/schema metadata flows. It is not a hosted-org replacement. |
| Standard library | <span class="docs-status-chip docs-status-supported">Works well</span> | The checked ledger has 265 supported rows, 19 unsupported hosted-boundary rows, and 0 partial rows. |
| Platform service APIs | <span class="docs-status-chip docs-status-supported">Works well</span> | Deterministic DTO and harness rows are modeled when the ledger says supported. Hosted service execution stays explicit unsupported. |

## Standard Library Families

Counts come from the checked standard library coverage report in this repository
state.

| Family | First-layer status | Ledger rows |
| --- | --- | ---: |
| `Database` | Works well | 37 supported / 37 tracked |
| Date, Datetime, Time, TimeZone | Works well | 26 supported / 26 tracked |
| String, Decimal, Boolean, Math | Works well | 32 supported / 32 tracked |
| System, Assert, Limits | Supported local rows, hosted fences | 17 supported, 3 unsupported / 20 tracked |
| Schema and SObject | Supported local rows, hosted fences | 7 supported, 1 unsupported / 8 tracked |
| Test helpers | Supported local rows, hosted fences | 28 supported, 3 unsupported / 31 tracked |
| JSON, Pattern, EncodingUtil, Crypto | Works well | 17 supported / 17 tracked |
| ApexPages and PageReference | Supported controller rows, hosted rendering fences | 15 supported, 2 unsupported / 17 tracked |
| HTTP and WebServiceCallout | Supported mock rows, live transport fences | 6 supported, 2 unsupported / 8 tracked |
| Messaging | Supported local rows, hosted delivery fences | 6 supported, 2 unsupported / 8 tracked |
| Search and SOSL helpers | Supported local rows, hosted ranking fence | 11 supported, 1 unsupported / 12 tracked |
| UserInfo, URL, Label, and TrailblazerIdentity | Wide local support | 24 supported / 24 tracked |
| Type, FeatureManagement, and Exception | Supported local rows, hosted package fence | 8 supported, 1 unsupported / 9 tracked |
| Local test harness and request context | Supported local rows, hosted/malformed fences | 30 supported, 2 unsupported / 32 tracked |
| Fenced hosted-service and platform boundary rows | Not supported, plus stable diagnostics | 1 supported diagnostic row, 2 unsupported / 3 tracked |

The local test harness and request-context group includes Approval,
BusinessHours, QuickAction, Request, UIRequest, Sandbox, Schedulable, and
AccessLevel rows. The fenced hosted-service group includes Answers and
ResetPasswordResult rows plus the stable UnsupportedFeature diagnostic row.

## Current Surface Landscape

The checked capability status and standard-library ledger now report no partial rows.
Every remaining hosted-only behavior is split into an exact unsupported row.

| Measure | Rows |
| --- | ---: |
| Capability features marked `supported` | 30 |
| Capability features marked `partial` | 0 |
| Capability features marked `unsupported` | 2 |
| Standard-library rows marked `supported` | 265 |
| Standard-library rows marked `partial` | 0 |
| Standard-library rows marked `unsupported` | 19 |

## Drill Down

Use this map first, then cut down to the exact checked row.

- Method-level standard-library rows: [`docs/STDLIB_COVERAGE.md`](https://github.com/glade-sh/glade/blob/main/docs/STDLIB_COVERAGE.md)

One rule keeps the marks honest. Do not call a surface supported until the row
has implementation and compatibility evidence.
