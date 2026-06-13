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
- You will use first-party plugins for maintainer ledgers and advisory scans instead of expecting those scanners in base `glade --help`.

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
| Schema and describe APIs | Local describe supports checked object, field, record type, child relationship, and generated standard-object shape. Full org metadata parity remains outside the local model. |
| JSON, regex, encoding, and crypto | Common serialization, parsing, regex, base64, hex, URL encoding, and digest paths are covered. Edge semantics stay method-level. |
| HTTP, SOAP, and callout mocks | Request/response-shaped mock paths work for tests. Glade does not perform live outbound service calls. |
| Messaging | Local message/result shapes and invocation counts are covered. Glade does not deliver email, push, or other live messages. |
| Visualforce controller helpers | PageReference, messages, current page, and controller test helpers are modeled for controller tests. Glade does not render full Visualforce pages. |
| Search and SOSL helpers | Local deterministic test paths exist. Full Salesforce search ranking and index behavior are not modeled. |
| Test helpers | Many common `Test.*` paths work. Service-dependent helpers and org-global behavior remain explicit gaps. |
| Local test harness and request context | Request/UIRequest context, install/uninstall hooks, sandbox post-copy helpers, scheduled Apex, QuickAction DTOs, BusinessHours week schedules, approval result shapes, and TrailblazerIdentity helper calls have deterministic local models. Live hosted engines are not contacted. |

## Not supported today

This is the smaller list a first user should check before betting on Glade.

| Area | Why it is outside the current local contract |
| --- | --- |
| Live Salesforce auth and sessions | The local server exposes local stubs. It does not implement real Salesforce OAuth, session validation, or org identity services. |
| Fenced live service APIs | Answers zone search, password reset output, live identity/admin mutation, and hosted process/service engines require Salesforce-hosted data or execution. |
| Full Visualforce rendering | Controller logic is the supported path. Component rendering, page lifecycle, `getContent`, and PDF generation remain outside the current runtime. |
| Broad REST and Tooling API parity | The local API server covers the checked local baseline. Bulk API, Composite Batch/Graph, Streaming/PubSub, GraphQL, layout/default-value metadata, metadata deploy/retrieve jobs, and live org-only Tooling surfaces remain future work. |
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
| Standard library | <span class="docs-status-chip docs-status-partial">Works with limits</span> | Broad local support, with exact method status in the checked ledger. |
| Platform service APIs | <span class="docs-status-chip docs-status-partial">Works with limits</span> | Deterministic DTO and harness rows are modeled when the ledger says so. Hosted service execution stays explicit unsupported. |

## Standard Library Families

Counts come from the checked standard library coverage report in this repository
state.

| Family | First-layer status | Ledger rows |
| --- | --- | ---: |
| `Database` | Works well | 37 supported / 37 tracked |
| Date, Datetime, Time, TimeZone | Works well | 26 supported / 26 tracked |
| String, Decimal, Boolean, Math | Wide local support | 29 supported, 3 partial / 32 tracked |
| System, Assert, Limits | Mixed | 13 supported, 4 partial / 17 tracked |
| Schema and SObject | Works with limits | 1 supported, 6 partial / 7 tracked |
| Test helpers | Works with limits | 18 supported, 10 partial / 28 tracked |
| JSON, Pattern, EncodingUtil, Crypto | Works with limits | 4 supported, 13 partial / 17 tracked |
| ApexPages and PageReference | Wide controller support | 13 supported, 2 partial / 15 tracked |
| HTTP and WebServiceCallout | Works with limits | 1 supported, 4 partial / 5 tracked |
| Messaging | Works with limits | 1 supported, 4 partial / 5 tracked |
| Search and SOSL helpers | Works with limits | 11 partial / 11 tracked |
| UserInfo, URL, Label, and TrailblazerIdentity | Wide local support | 24 supported / 24 tracked |
| Type, FeatureManagement, and Exception | Works with limits | 6 supported, 2 partial / 8 tracked |
| Local test harness and request context | Works with limits | 13 supported, 17 partial / 30 tracked |
| Fenced live service APIs | Not supported | 2 unsupported / 2 tracked |

The local test harness and request-context group includes Approval,
BusinessHours, QuickAction, Request, UIRequest, Sandbox, Schedulable, and
AccessLevel edge rows. The fenced live-service group includes Answers and
ResetPasswordResult rows.

## Current Surface Landscape

The maintainer surface refresh separates product support from generated shape,
passive DTOs, stub/no-op rows, and explicit unsupported fences. In the current
checked landscape it reports:

| Bucket | Rows |
| --- | ---: |
| Implemented | 130266 |
| Partial | 1 |
| Passive shape | 47494 |
| Stub/no-op | 262 |
| Explicit unsupported | 6338 |
| Missing shape gaps | 0 |
| Missing behavior gaps | 0 |
| Missing evidence gaps | 0 |
| Failure rows | 0 |

## Drill Down

Use this map first, then cut down to the exact checked row.

- Ledger standard-library rows: [`docs/STDLIB_COVERAGE.md`](https://github.com/glade-sh/glade/blob/main/docs/STDLIB_COVERAGE.md)
- Current known gaps: [`docs/KNOWN_GAPS.md`](https://github.com/glade-sh/glade/blob/main/docs/KNOWN_GAPS.md)
- Maintainer proof reports: [Maintainer Proof Reports](/guide/compatibility-dashboard)

One rule keeps the marks honest. Do not call a surface supported until the row
has implementation and compatibility evidence.
