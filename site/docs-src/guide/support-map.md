# Support Map

Start here when deciding whether Glade can run a project or a test path. This
page stays high level. The generated ledgers carry the method rows.

## Status Key

| Status | Meaning |
| --- | --- |
| Supported | Implemented and covered by compatibility fixtures. |
| Partial | Common local paths work, and gaps are named. |
| Unsupported | Glade should fail with a stable unsupported diagnostic. |
| Unknown | A row exists, but the team has not measured it yet. |

## Runtime Areas

| Area | Status | What to expect |
| --- | --- | --- |
| Apex parsing and project indexing | Supported | Large SFDX projects, nested types, namespace tokens, and stable parse diagnostics are covered by the local MVP gate. |
| Semantic analysis | Supported | Method bodies, constructors, inheritance, interfaces, locals, assignments, return paths, overloads, and token ranges are checked for the supported VM subset. |
| Local Apex tests | Supported | Test discovery, `@TestSetup`, isolated org state, static reset, limits windows, async drain, stack frames, JSON, and JUnit reports are covered for the supported runtime. |
| SOQL, DML, triggers, and SObjects | Supported | Static and dynamic SOQL, DML statements, `Database.*` result shapes, trigger context, schema-backed SObjects, and local storage are covered for the checked local data model. |
| Local API server | Supported | Salesforce-shaped discovery, CRUD, query, queryAll, limits, userinfo, Tooling `executeAnonymous`, Composite sObject insert, reset endpoints, and SQLite persistence are covered. |
| Editor and tooling | Supported | LSP diagnostics, symbols, hover, completion, rename, semantic tokens, DAP stepping, watch mode, and trace/profile reports are covered for the local MVP contract. |
| Core standard library | Wide support | The common local test surface is covered. Method-level detail lives in the standard library ledger. |
| Platform service APIs | Unsupported by default | Services that need live Salesforce process engines, identity services, request context, or sandbox lifecycle fail with explicit unsupported diagnostics unless the ledger says otherwise. |

## Standard Library Families

Counts come from `glade compat stdlib --json` in this repository state.

| Family | First-layer status | Method rows |
| --- | --- | ---: |
| `Database` | Supported | 37 supported / 37 tracked |
| Date, Datetime, Time, TimeZone | Supported | 26 supported / 26 tracked |
| String and primitives | Wide support | 21 supported, 3 partial / 24 tracked |
| System, Assert, Limits | Mixed | 12 supported, 1 partial, 4 unsupported / 17 tracked |
| Schema and SObject | Partial | 1 supported, 6 partial / 7 tracked |
| Test helpers | Mixed | 8 supported, 7 partial, 13 unsupported / 28 tracked |
| JSON, Pattern, EncodingUtil, Crypto | Partial | 4 supported, 13 partial / 17 tracked |
| ApexPages and PageReference | Wide support | 12 supported, 2 partial, 1 unknown / 15 tracked |
| HTTP and WebServiceCallout | Partial | 1 supported, 4 partial / 5 tracked |
| Messaging | Partial | 1 supported, 4 partial / 5 tracked |
| Search and SOSL helpers | Partial | 7 partial, 4 unsupported / 11 tracked |
| Service-only platform APIs | Unsupported service surface | 35 unsupported / 35 tracked |

The service-only group includes Approval, BusinessHours, QuickAction, Request,
UIRequest, Sandbox, TrailblazerIdentity, Answers, ResetPasswordResult,
Schedulable, and AccessLevel edge rows.

## Drill Down

Use the map first, then cut down to the exact row:

```bash
glade compat mvp
glade compat matrix --json
glade compat stdlib --json
```

- Generated capability dashboard: [Compatibility Dashboard](/guide/compatibility-dashboard)
- Method-level standard library rows: [`docs/STDLIB_COVERAGE.md`](https://github.com/glade-sh/glade/blob/main/docs/STDLIB_COVERAGE.md)
- Current known gaps: [`docs/KNOWN_GAPS.md`](https://github.com/glade-sh/glade/blob/main/docs/KNOWN_GAPS.md)
- Project triage: `glade compat local-tests --project . --parallel auto --json`

One rule keeps the marks honest. Do not call a surface supported until the row
has implementation and compatibility evidence.
