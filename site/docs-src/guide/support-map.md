# Apex and Salesforce Support

Start here when deciding whether Glade can run a project, a test class, or a
local Salesforce-shaped API flow. This page is the public map. The generated
ledgers carry the exact ledger rows.

## Status Key

| Status | Meaning |
| --- | --- |
| Works well | Implemented, fixture-backed, and fit for normal local use. |
| Works with limits | Common local paths work. The limits are named. |
| Not supported | Glade should stop with a stable unsupported diagnostic. |
| Not measured | A row exists, but the team has not measured it yet. |

## Works Well

These areas are the main local development contract.

| Area | What to expect |
| --- | --- |
| Apex parsing and project indexing | Large SFDX projects, nested types, namespace tokens, and stable parse diagnostics. |
| Semantic checks | Type references, inheritance, interfaces, overloads, locals, assignments, return paths, and token ranges for the supported VM subset. |
| Local Apex tests | `@isTest`, `@TestSetup`, isolated org state, static reset, governor windows, async drain, stack frames, JSON, and JUnit output. |
| SOQL, DML, triggers, and SObjects | Static and dynamic SOQL, DML statements, `Database.*` result shapes, trigger context, schema-backed SObjects, and local SQLite-backed storage. |
| Local API server | Salesforce-shaped REST discovery, SObject CRUD, query/queryAll, limits, userinfo stubs, Tooling `executeAnonymous`, Composite sObject insert, reset endpoints, and optional SQLite persistence. |
| Editor and debug tools | LSP diagnostics, symbols, hover, completion, rename, semantic tokens, DAP stepping, watch mode, and trace/profile reports. |

## Works With Limits

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
| Local test harness and request context | Request/UIRequest context, install/uninstall hooks, sandbox post-copy helpers, scheduled Apex, QuickAction DTOs, BusinessHours week schedules, and approval result shapes have deterministic local models. Live hosted engines are not contacted. |

## Not Supported Today

This is the smaller list a first user should check before betting on Glade.

| Area | Why it is outside the current local contract |
| --- | --- |
| Live Salesforce auth and sessions | The local server exposes local stubs. It does not implement real Salesforce OAuth, session validation, or org identity services. |
| Fenced live service APIs | Trailblazer identity, Answers, and password reset services require Salesforce-hosted engines. |
| Full Visualforce rendering | Controller logic is the supported path. Component rendering, page lifecycle, `getContent`, and PDF generation remain outside the current runtime. |
| Broad REST and Tooling API parity | The local API server covers the checked local baseline. Bulk API, Composite Graph, Streaming/PubSub, GraphQL, layout metadata, and broad Tooling object coverage remain future work. |
| Live outbound side effects | Real callouts, delivered email, push notifications, and external service mutations are not performed. Tests should use local mocks and result objects. |
| Exact Salesforce governor accounting | Glade tracks deterministic local limits. Salesforce's full production accounting and every platform-specific counter are not complete. |

## Area Detail

| Area | First-layer status | Notes |
| --- | --- | --- |
| Apex front end | Works well | Parser, project loader, symbols, semantic checks, LSP, and diagnostics form the front door. |
| Runtime and tests | Works well | VM execution, local tests, SObjects, SOQL, DML, triggers, async drain, and local storage are the core contract. |
| Local Salesforce API | Works well | Useful for local REST and Tooling `executeAnonymous` flows. It is not a hosted-org replacement. |
| Standard library | Works with limits | Broad local support, with exact method status in the checked ledger. |
| Platform service APIs | Not supported by default | Service-backed rows should fail with explicit unsupported diagnostics unless the ledger says otherwise. |

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
| ApexPages and PageReference | Wide controller support | 12 supported, 2 partial, 1 unknown / 15 tracked |
| HTTP and WebServiceCallout | Works with limits | 1 supported, 4 partial / 5 tracked |
| Messaging | Works with limits | 1 supported, 4 partial / 5 tracked |
| Search and SOSL helpers | Works with limits | 11 partial / 11 tracked |
| UserInfo, URL, and Label | Wide local support | 21 supported / 21 tracked |
| Type, FeatureManagement, Exception, and diagnostics | Works with limits | 6 supported, 3 partial / 9 tracked |
| Local test harness and request context | Works with limits | 12 supported, 18 partial / 30 tracked |
| Fenced live service APIs | Not supported | 5 unsupported / 5 tracked |

The local test harness and request-context group includes Approval,
BusinessHours, QuickAction, Request, UIRequest, Sandbox, Schedulable, and
AccessLevel edge rows. The fenced live-service group includes Answers,
ResetPasswordResult, and TrailblazerIdentity rows.

## Drill Down

Use this map first, then cut down to the exact checked row.

- Ledger standard-library rows: [`docs/STDLIB_COVERAGE.md`](https://github.com/glade-sh/glade/blob/main/docs/STDLIB_COVERAGE.md)
- Current known gaps: [`docs/KNOWN_GAPS.md`](https://github.com/glade-sh/glade/blob/main/docs/KNOWN_GAPS.md)
- Developer compatibility reports: [Developer Reports](/guide/compatibility-dashboard)

One rule keeps the marks honest. Do not call a surface supported until the row
has implementation and compatibility evidence.
