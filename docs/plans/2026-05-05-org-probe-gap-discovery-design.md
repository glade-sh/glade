# Org Probe Gap Discovery — Design Document

**Date**: 2026-05-05  
**Scope**: Language, platform, and metadata gap detection using a live Salesforce scratch org  
**Owner**: TBD (implementation phase)  
**Status**: Design approved

---

## 1. Overview

This plan defines a repeatable harness that treats a real Salesforce scratch org as the "golden" runtime and compares its behavior against the local `oaer` runtime. The goal is to turn guesswork into empirical data: instead of reading Salesforce docs and hoping we interpreted them correctly, we execute the same code on both sides and diff the outcomes.

The harness produces two artifacts:
1. A structured **gap report** for immediate review.
2. Reusable **compatibility fixtures** for long-term CI regression coverage.

---

## 2. Architecture

```
Probe Sources
    |
    +---> Real Scratch Org (sfdx deploy + Tooling API)
    |         |
    |         v
    |   Golden Responses  ----+
    |                          |
    +---> Local oaer Runtime   |     Diff Engine
              |                |          |
              v                |          v
        Local Responses  ------+->  gap-report.json
                                         |
                                         v
                              docs/fixtures/*.json
                              + capability updates
```

### 2.1 Real Org Track

- Deploy probe Apex classes and metadata to the scratch org via `sfdx force source push`.
- Execute probes through:
  - `@RestResource` endpoints (HTTP)
  - `@AuraEnabled` methods (HTTP)
  - Anonymous Apex via Tooling API `executeAnonymous`
- Capture response bodies, exception types, governor limit headers, and side effects (record IDs, field values, DML counts).
- Normalize output by stripping org-specific noise (session tokens, org IDs, timestamps within tolerance).

### 2.2 Local Track

- Load the **exact same metadata schema** into `oaer` (via `sfdx-project.json` or explicit object/field definitions).
- Seed the **exact same test data** into the local SQLite store.
- Run probes through:
  - `oaer server` for REST probes (identical HTTP requests)
  - `oaer exec` for anonymous probes
  - `oaer test` for test-class probes
- Capture local responses in the same normalized schema.

### 2.3 Diff Engine

The diff engine walks both output trees and categorizes every delta:

| Gap Type | Definition | Severity |
|---|---|---|
| `panic_gap` | Works in org; crashes `oaer` | Critical |
| `unsupported_gap` | Works in org; fails locally with unsupported error | High |
| `behavioral_gap` | Same API; different result | Medium |
| `limit_gap` | Governor limit behavior differs | Medium |
| `metadata_gap` | Metadata feature exists in org but not in `oaer` | Low-Medium |

---

## 3. Probe Taxonomy

Probes are organized into six categories. Each category is a directory of deployable Apex classes plus assertion scripts.

### 3.1 Language & Types (~40 probes)

Edge cases that are easy to get subtly wrong:
- Null coercion in arithmetic and string concatenation
- `==` vs `===` behavior for SObjects and collections
- Date/DateTime math (leap years, time zones, DST boundaries)
- Exception type hierarchy (`DmlException` vs `QueryException` vs `NullPointerException`)
- Collection generics (`Map<Id, SObject>` key semantics, set ordering)
- SOQL for-loop binding behavior (`for (Account a : [SELECT ...])`)

### 3.2 Stdlib & System (~80 probes)

Every standard library method that `oaer` claims to support:
- `String`: `format()`, `escapeSingleQuotes()`, `join()`, `split()`, locale-specific methods
- `DateTime` / `Date`: `valueOf()`, `format()`, `addDays()`, `newInstance()` edge cases
- `Decimal` / `Math`: rounding modes, `pow()`, division scale
- `JSON`: `serialize()` on nested proxies, `deserializeUntyped()` type fidelity
- `Limits`: accuracy of `getLimitQueries()`, `getLimitDmlRows()`, etc.
- `Crypto` / `EncodingUtil`: base64, URL encoding, hash algorithms

### 3.3 Data Runtime (~50 probes)

SOQL behavior is the top source of local-only blind spots:
- Relationship queries (`Parent__r.Name`, `Child__r` subqueries)
- Polymorphic ID references (`What.Name`, `Who.Email`)
- Aggregate functions (`COUNT`, `COUNT_DISTINCT`, `AVG`, `MIN`, `MAX` null handling)
- Dynamic SOQL composition and execution
- SOSL stubs (expected to be unsupported or stubbed)
- `Schema.describeSObjects()` accuracy for field types, picklists, record types

### 3.4 DML & Triggers (~35 probes)

- Trigger context variables (`Trigger.isBefore`, `Trigger.newMap` key stability)
- DML statement side effects and return values
- `upsert` external-ID semantics
- `merge` record reparenting behavior
- Validation rule firing order and error message format
- `undelete` behavior and recovery scope

### 3.5 Limits & Async (~25 probes)

- Governor limit accuracy under load (SOQL rows, DML rows, heap, CPU)
- `@future` method stubs: compile, enqueue, and controlled execution
- `Queueable` stubs: chaining, job ID returned
- `Batchable` stubs: scope, stateful, finish behavior

### 3.6 Metadata & Platform (~30 probes)

- Custom settings hierarchy queries (`List` vs `Hierarchy`)
- Platform event publish behavior stubs
- Custom label resolution (`System.Label.MyLabel`)
- Custom metadata type SOQL (`__mdt`)
- Flow save-order interactions (before/after triggers vs flow)
- Approval process trigger hooks

---

## 4. Execution Phases

The probe run is a five-phase workflow designed to be repeatable in ~25 minutes.

### Phase 1 — Bootstrap & Deploy (~5 min)

1. Validate scratch org connectivity (`sfdx force org display`).
2. Push probe SFDX source to the org.
3. Seed the org with deterministic test data:
   - 10 Accounts, 20 Contacts, 5 custom objects with known field values
   - Custom settings records, custom metadata records
   - Platform event definitions (no instances)
4. Export the org schema and seed data to a local fixture.
5. Start `oaer server` locally with the same schema and seed data loaded.

### Phase 2 — Golden Capture (~10 min)

1. Iterate through every probe class.
2. For REST probes: send HTTP requests to org endpoints; record status, headers, body, stack traces.
3. For anonymous probes: use Tooling API `executeAnonymous`; record `result` and `debugLog`.
4. For test-class probes: run via Apex Test Execution API; record pass/fail and coverage.
5. Capture side effects by querying the org afterward (e.g., record counts, field values).
6. Write all golden responses to `golden/<probe-id>.json` in a normalized schema.

### Phase 3 — Local Replay (~5 min)

1. Replay the exact same HTTP requests against `localhost` (oaer server).
2. Run `oaer exec` for anonymous probes.
3. Run `oaer test` for test-class probes.
4. Execute the same post-probe queries against the local SQLite database.
5. Write local responses to `local/<probe-id>.json`.

### Phase 4 — Diff & Classify (~2 min)

1. Walk both output trees.
2. Strip org-specific noise:
   - Record IDs (replace with deterministic patterns)
   - Session tokens and org IDs
   - Timestamps (compare within ±2 second tolerance)
3. Categorize every delta using the five gap types.
4. Sort by severity: `panic_gap` → `unsupported_gap` → `behavioral_gap` → `limit_gap` → `metadata_gap`.
5. Write `gap-report.json`.

### Phase 5 — Fixture Extraction (~3 min)

1. For every `panic_gap` and `unsupported_gap`, auto-generate a compatibility fixture.
2. Include: inline Apex source, expected output, schema metadata, seed data.
3. Write fixtures to a staging directory for human review.
4. Generate a Markdown summary of new findings for `docs/KNOWN_GAPS.md`.
5. Suggest capability matrix updates in `internal/capability`.

---

## 5. Deliverables

### 5.1 Gap Report (`gap-report.json`)

Machine-readable diff with one entry per probe:

```json
{
  "probeId": "stdlib.datetime.valueof-null",
  "category": "Stdlib & System",
  "gapType": "behavioral_gap",
  "severity": "medium",
  "capabilityId": "stdlib.datetime",
  "golden": { "result": null, "exception": "System.NullPointerException" },
  "local": { "result": "1970-01-01T00:00:00Z", "exception": null },
  "diff": "Org throws NPE on null input; oaer returns epoch"
}
```

### 5.2 Compatibility Fixtures

New `docs/fixtures/*.json` entries following the existing fixture schema. Each fixture includes:
- `command`: `exec`, `test`, or `server`
- `source`: inline Apex
- `schema`: required objects and fields
- `seed`: required records
- `expect`: expected output, errors, or side effects
- `evidence`: symbol link for `oaer compat evidence`

### 5.3 Capability Matrix Updates

Suggested status changes for `internal/capability`:
- Promotion: `partial` → `supported` when all probes pass
- Demotion: `supported` → `partial` when probes fail
- Evidence linking: fixture path attached to catalog entry

---

## 6. Integration with Existing Tooling

| Existing Command | How the Harness Feeds In |
|---|---|
| `oaer compat mvp` | Uses updated capability statuses to compute readiness |
| `oaer compat evidence` | New fixtures provide evidence; prevents ungated promoted entries |
| `oaer compat dashboard` | Regenerated from updated capability matrix |
| `oaer compat gaps` | Incorporates org-verified gap findings |
| `go test ./internal/compat` | New fixtures run automatically as regression tests |

---

## 7. Prerequisites & Inputs

From the user:
1. A scratch org with API access (username + instance URL + access token, or authorized via `sfdx`).
2. `sfdx` CLI installed and authenticated locally.
3. `oaer` built from the target commit (to ensure local runtime matches code under test).

The harness will provide:
1. A complete SFDX project containing all probe classes and metadata.
2. A CLI wrapper (`scripts/run-org-probes.sh` or `oaer probe org`) that orchestrates the five phases.

---

## 8. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Scratch org expires mid-run | High | Use a persistent dev org or refresh token; probe runtime is <30 min |
| Org API rate limits | Medium | Batch probes; add 200ms between requests; use bulk API for seed data |
| Side effects pollute org | Low | All probes use namespaced test objects; cleanup script runs post-probe |
| Local/server drift from org | Medium | Schema is exported from org and imported locally; same seed data |
| Sensitive data in fixtures | Medium | Strip all real IDs, names, and emails before fixture commit |

---

## 9. Future Work (Post-MVP)

- **Nightly CI hook**: Run a fast subset of probes (~50 high-value probes) against a persistent scratch org.
- **Golden response replay mode**: Skip the real org and diff against cached golden responses for local-only PR validation.
- **Auto-fix suggestions**: For `behavioral_gap` entries with clear patterns, generate a Go patch suggestion.

---

## 10. Success Criteria

The harness is considered successful when:
1. It can be run end-to-end in under 30 minutes.
2. It discovers at least 10 new gaps not currently tracked in `docs/KNOWN_GAPS.md`.
3. It generates at least 5 new fixtures that pass `go test ./internal/compat`.
4. It produces zero false-positive `panic_gap` reports (every reported panic is reproducible).
