# What Glade runs locally

<script setup>
import releaseManifest from '../../release-manifest.json'
</script>

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Capabilities</p>
  <p>Use this page to see what Glade runs locally, what runs with named limits, and what still requires Salesforce or a plugin.</p>
  <ul>
    <li>Start with the status legend.</li>
    <li>Check what requires Salesforce before an adoption review.</li>
    <li>Use the checked Apex/runtime reports for method-level detail.</li>
  </ul>
</div>

Start here when deciding whether Glade can run a project, a test class, or a
local Salesforce API flow. Checked Apex/runtime reports carry the method-level
detail. The LWC summary below links to the local shell guide and support table
when you need route and module detail.

**Verified with Glade {{ releaseManifest.version }}.** The generated
standard-library counts and searchable rows below come from the checked coverage
ledger in this repository state.

## Before you adopt Glade

- Your local loop uses supported Apex parse, check, test, SOQL, DML, trigger, and SObject paths.
- Your test suite can mock callouts and live side effects.
- Your project can tolerate explicit unsupported diagnostics for Salesforce-hosted services.
- You will keep a Salesforce validation gate for features Glade does not model.
- You will use first-party plugins for capability reports and advisory scans instead of expecting those scanners in base `glade --help`.

## Status key

<div class="docs-support-legend" aria-label="Capability status legend">
  <div class="docs-support-legend-card docs-support-legend-card-supported" aria-label="Runs locally">
    <span class="docs-status-chip docs-status-supported">Runs locally</span>
  </div>
  <div class="docs-support-legend-card docs-support-legend-card-partial" aria-label="Runs locally with limits">
    <span class="docs-status-chip docs-status-partial">Runs locally with limits</span>
  </div>
  <div class="docs-support-legend-card docs-support-legend-card-unsupported" aria-label="Requires Salesforce">
    <span class="docs-status-chip docs-status-unsupported">Requires Salesforce</span>
  </div>
  <div class="docs-support-legend-card docs-support-legend-card-unknown" aria-label="Not measured">
    <span class="docs-status-chip docs-status-unknown">Not measured</span>
  </div>
</div>

“Runs locally with limits” is the user-facing capability state. Generated
evidence catalogs may use narrower row-level classifications; their counts
apply only to the named catalog.

`GLADERUNTIME001` marks a body that the Apex parser accepts but the local VM
cannot lower. That body is compile-ready for `glade check`, but it receives no
runtime-support credit and local test execution reports `UnsupportedFeature`.

<GladeSupportExplorer />

## Published v0.2.11 surface snapshot

The following counts are the published v0.2.11 surface snapshot from a frozen
two-repository private corpus. Public artifacts identify the repositories only as
`private-corpus-001` and
`private-corpus-002`.

- 321 observed usage keys reconciled with zero unknown usage.
- 184 required surfaces were derived from sealed authoritative inputs.
- 178 compile-ready and test-ready surfaces have required local proof.
- 54 runtime-parity-ready surfaces have fresh Salesforce proof.
- 107 explicit zero-credit non-parity outcomes remain visible.
- Six hosted-deferred surfaces remain outside compile, test, and runtime-parity
  readiness in this snapshot.

These outcomes overlap and are not a claim of blanket Salesforce parity. Open
the [self-contained assurance explorer](/private-corpus-assurance.html) to
filter by namespace, repository, disposition, evidence, exclusion, or text.
The exact candidate, receipt hashes, replay result, and interpretation rules
are recorded in the
[release assurance note](https://github.com/glade-sh/glade/blob/v0.2.11/docs/PRIVATE_CORPUS_ASSURANCE.md).

## Next-release product-candidate validation

The named product candidate `68289fa1afe679b6593b5dfe8cba28bdf2f0ac10`
(binary SHA-256 `f54e1a5d39a34ef58e60946bfea4a1b3fed5e18cef92f4b066ccab81738e7f20`)
was checked separately from the published v0.2.11 surface snapshot.

- `private-corpus-001`: check exit 0/0 diagnostics; tests 12,315/12,315 with
  0 failed/compile/runtime/unsupported.
- `private-corpus-002`: check exit 0/0 diagnostics; tests 782/782 with
  0 failed/compile/runtime/unsupported.
- Public corpus: 86 projects, 40 expected/40 observed diagnostics, zero
  missing/unexpected/unclassified, and an exact identity multiset match. Public
  diagnostics are the known baseline, not passes.
- Salesforce current-release validation: 475/475 pass, zero fail/inconclusive, and
  cleanup PASS.

A later tracked release candidate still requires fresh exact-SHA validation.

## Runs locally

These areas are the main local development contract.

| Area | What to expect |
| --- | --- |
| Apex parsing and project indexing | Large Salesforce DX projects, nested types, namespace tokens, stable parse diagnostics, and case-insensitive rejection of all 121 Salesforce reserved words in non-method source identifier contexts. See [Apex language compatibility](/reference/apex-language-compatibility). |
| Project configuration and package contracts | `glade.yml` and Salesforce DX discovery cover package dirs, default namespace, namespace remaps, source-backed managed package dependencies, captured package artifacts, package shims, org features, storage, and local limits. |
| Semantic checks | Checked annotation, declaration, type, inheritance, trigger, statement, SOQL/SOSL, API-version, local, assignment, overload, and return-path rules, with token ranges for the supported VM subset. |
| Local Apex tests | `@isTest`, `@TestSetup`, isolated org state, static reset, governor windows, async drain, stack frames, JSON, and JUnit output. |
| SOQL, DML, triggers, and SObjects | Static and dynamic SOQL, DML statements, `Database.*` result objects, trigger context, schema-backed SObjects, and local SQLite-backed storage. |
| Local API server | Salesforce-style REST discovery, SObject CRUD, query/queryAll, limits and record counts, userinfo stubs, Tooling `executeAnonymous`, local Tooling source/schema metadata queries, Composite sObject insert, Composite Batch, Tree, and Graph local requests, Bulk API v2 simple query jobs, layout/default-value metadata, metadata job status, reset endpoints, and optional SQLite persistence. |
| Editor and debug tools | LSP diagnostics, symbols, hover, completion, rename, semantic tokens, DAP stepping, watch mode, and trace/profile reports. |

## Runs locally with limits

These areas cover useful local test paths. They are not exact Salesforce
service behavior.

| Area | Current limit |
| --- | --- |
| Core standard library | Common `System`, `String`, date/time, math, assertions, labels, URLs, user info, and collection paths are covered. Method-level details live in the standard-library report. |
| Schema and describe APIs | Local describe supports checked object, field, record type, child relationship, data category metadata, and generated standard-object metadata. Hosted full-org metadata services remain outside the local model. |
| JSON, regex, encoding, and crypto | Serialization, parsing, regex, base64, hex, URL encoding, and digest rows are supported for the checked local contract. |
| HTTP, SOAP, and callout mocks | Request and response mock paths work for tests. Glade does not perform live outbound service calls. |
| Messaging | Local message and result objects, template rendering, attachment retrieval, send options, and invocation counts are covered. Glade does not deliver email, push, or other live messages. |
| Visualforce controller and page rendering | Preview feature. PageReference, messages, current page, controller helpers, local `/apex/<PageName>` routes, common standard components, page lifecycle paths, signed view state with CSRF checks, transient field omission, static resources, uploads, remoting envelopes, Lightning Out/LWC dependency diagnostics, AJAX refresh paths, and local PDF fallback output are modeled for local development. Salesforce chrome, every component edge, exact lifecycle timing, Apex `PageReference.getContent*` output, and byte-for-byte PDF output remain outside the local contract. |
| Local LWC workbench and routes | Preview feature. `/` and `/lwc` open the Workbench Console with Component Lab and Page Workbench, `/lwc/builder` opens the page composer, and preview routes cover components, record/app/home pages, tabs, actions, utility bars, Flow screens/actions, configured community pages, and direct community components. |
| Local LWC data and services | Preview feature with local-data limits. Apex wire, selected LDS/UI API shims, local record mutations, schema/label/resource/user/community modules, bounded managed-content reads, navigation, LMS, toast/resource loading, `lightning/refresh`, Flow events, console and utility local models, practical base-component shims, and packaged SLDS 2 and classic SLDS assets run through local models. Experience Builder mutations and learning-platform APIs reject explicitly and remain hosted-only. See [Local LWC Shell](/guide/lwc-local-shell) and [`docs/LWC_SUPPORT.md`](https://github.com/glade-sh/glade/blob/main/docs/LWC_SUPPORT.md). |
| Visualforce Lightning Out for LWCs | Preview feature with limits. Visualforce-backed tab redirects and `/apex/<PageName>` Lightning Out hosts use the shared local LWC runtime. Hosted Lightning Out lifecycle timing and exact Salesforce chrome remain Salesforce checks. |
| Search and SOSL helpers | Local deterministic Search and SOSL rows are supported. Hosted ranking, analyzers, synonyms, and external indexes are not modeled. |
| Test helpers | Tracked `Test.*` local helper rows are supported. Hosted service accounting, packaged-resource expansion, and live External Service execution remain explicit gaps. |
| Local test harness and request context | Request/UIRequest context, install/uninstall hooks, sandbox post-copy helpers, scheduled Apex, QuickAction DTOs, BusinessHours calendars and holidays, seeded approval routing, and TrailblazerIdentity helper calls have deterministic local models. Live hosted engines are not contacted. |

## Requires Salesforce

Check this list before relying on Glade for a project.

| Area | Why it is outside the current local contract |
| --- | --- |
| Live Salesforce auth and sessions | The local server exposes local stubs. It does not implement real Salesforce OAuth, session validation, or org identity services. |
| Live service APIs | Answers zone search, password reset output, live identity/admin mutation, and hosted process/service engines require Salesforce-hosted data or execution. |
| Exact hosted Visualforce behavior | Glade serves local Visualforce pages for development. It does not promise Salesforce-hosted chrome, every component edge, exact lifecycle timing, Apex `PageReference.getContent*` output, every remoting/browser behavior, or byte-for-byte PDF output. |
| Exact hosted Lightning Experience behavior | The local LWC shell does not promise Salesforce-hosted app chrome, hosted permission assignment, full UI API and GraphQL semantics, every `lightning-*` base component edge, exact SLDS fidelity, Flow Builder behavior, live EMP streaming, hosted utility chrome, or every Lightning Out edge. Console workspace APIs and utility-bar APIs have local models for development; exact hosted console behavior remains a Salesforce check. The package-exposed base-component module surface resolves locally with practical shims, including common button/card/layout/formatted-number contracts, with a small source-backed allowlist for simple base components. |
| REST and Tooling APIs outside the local baseline | The local API server covers the checked local baseline, including Composite Batch, Tree, and Composite Graph local requests over supported subrequests, Bulk API v2 simple query jobs, layout/default-value metadata, metadata job status, and local Tooling responses. Bulk API locator paging, Streaming/PubSub, GraphQL, live metadata deploy/retrieve, live auth, and live org-only Tooling APIs remain outside the local contract. |
| Live outbound side effects | Real callouts, delivered email, push notifications, and external service mutations are not performed. Tests should use local mocks and result objects. |
| Exact Salesforce governor accounting | Glade tracks deterministic local limits. Salesforce's full production accounting and every platform-specific counter are not complete. |

Example diagnostic:

```text
UnsupportedFeature: unsupported call "Search.unavailable local search/SOSL surface"
```

## Area details

| Area | Status | Notes |
| --- | --- | --- |
| Apex front end | <span class="docs-status-chip docs-status-supported">Runs locally</span> | Parser, project loader, symbols, semantic checks, LSP, and diagnostics form the starting point. The checked compiler contract contains 400 language-rule rows. |
| Runtime and tests | <span class="docs-status-chip docs-status-supported">Runs locally</span> | VM execution, local tests, SObjects, SOQL, DML, triggers, async drain, and local storage are the core contract. |
| Local Salesforce API | <span class="docs-status-chip docs-status-supported">Runs locally</span> | Useful for local REST, SObject CRUD/query, record count, Tooling `executeAnonymous`, and local source/schema metadata flows. It is not a hosted-org replacement. |
| Standard library | <span class="docs-status-chip docs-status-supported">Runs locally</span> | The checked standard-library report has 268 supported rows, 18 unsupported hosted-boundary rows, and 0 partial rows. |
| Platform service APIs | <span class="docs-status-chip docs-status-supported">Runs locally</span> | Deterministic DTO and harness rows are modeled when the capability report says supported. Hosted service execution stays explicitly unsupported. |

## Standard library families

Counts come from the checked standard library capability report in this repository
state.

| Family | Status | Rows |
| --- | --- | ---: |
| `Database` | Runs locally | 37 supported / 37 tracked |
| Date, Datetime, Time, TimeZone | Runs locally | 26 supported / 26 tracked |
| String, Decimal, Boolean, Math | Runs locally | 32 supported / 32 tracked |
| System, Assert, Limits | Supported local rows, hosted gaps | 17 supported, 3 unsupported / 20 tracked |
| Schema and SObject | Supported local rows, hosted gaps | 7 supported, 1 unsupported / 8 tracked |
| Test helpers | Supported local rows, hosted gaps | 28 supported, 3 unsupported / 31 tracked |
| JSON, Pattern, EncodingUtil, Crypto | Runs locally | 17 supported / 17 tracked |
| ApexPages and PageReference | Supported controller rows, hosted rendering gaps | 15 supported, 2 unsupported / 17 tracked |
| HTTP and WebServiceCallout | Supported mock rows, live transport gaps | 6 supported, 2 unsupported / 8 tracked |
| Messaging | Supported local rows, hosted delivery gaps | 6 supported, 2 unsupported / 8 tracked |
| Search and SOSL helpers | Supported local rows, hosted ranking gap | 11 supported, 1 unsupported / 12 tracked |
| UserInfo, URL, Label, and TrailblazerIdentity | Broad local capability | 24 supported / 24 tracked |
| Type, FeatureManagement, and Exception | Supported local rows, hosted package gap | 8 supported, 1 unsupported / 9 tracked |
| Local test harness and request context | Supported local rows, hosted and malformed-input gaps | 32 supported, 2 unsupported / 34 tracked |
| Hosted-service and platform boundary rows | Requires Salesforce, plus stable diagnostics | 2 supported, 1 unsupported / 3 tracked |

The local test harness and request-context group includes Approval list
processing, BusinessHours, QuickAction, Request, UIRequest, Sandbox,
Schedulable, and AccessLevel rows. The hosted-service boundary group includes
the deterministic local Answers stub, the ResetPasswordResult row, and the
stable UnsupportedFeature diagnostic row.

## Capability claims

The checked capability status and standard-library report now show no partial rows.
Every remaining hosted-only behavior is split into an explicit unsupported row.

| Measure | Rows |
| --- | ---: |
| Capability features marked `supported` | 31 |
| Capability features marked `partial` | 0 |
| Capability features marked `unsupported` | 2 |
| Standard-library rows marked `supported` | 268 |
| Standard-library rows marked `partial` | 0 |
| Standard-library rows marked `unsupported` | 18 |

## Drill down

Use this page first, then open the method-level report when you need the checked row.

- Method-level standard-library rows: [`docs/STDLIB_COVERAGE.md`](https://github.com/glade-sh/glade/blob/main/docs/STDLIB_COVERAGE.md)
- Compatibility dashboard: [`docs/COMPATIBILITY_DASHBOARD.md`](https://github.com/glade-sh/glade/blob/main/docs/COMPATIBILITY_DASHBOARD.md)
- Known gaps: [`docs/KNOWN_GAPS.md`](https://github.com/glade-sh/glade/blob/main/docs/KNOWN_GAPS.md)
- Standard-object coverage: [`docs/STANDARD_OBJECT_COVERAGE.md`](https://github.com/glade-sh/glade/blob/main/docs/STANDARD_OBJECT_COVERAGE.md)
- Standard-object schema: [`docs/STANDARD_OBJECT_SCHEMA.md`](https://github.com/glade-sh/glade/blob/main/docs/STANDARD_OBJECT_SCHEMA.md)
- Local LWC shell guide: [Local LWC Shell](/guide/lwc-local-shell)

One rule keeps the marks honest. Do not call an API supported until the row has
implementation and compatibility evidence.
