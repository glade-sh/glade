# Probe Gap Handoff - 2026-05-06

## Current State

The full probe suite is green against `oaer-probe-lab`.

- Probes run: 85
- Gaps: 0
- Unsupported: 0
- Behavioral: 0
- Panics: 0

Validation commands run:

```bash
go test ./...
go run ./cmd/oaer probe deploy --target-org oaer-probe-lab
go run ./cmd/oaer probe org --target-org oaer-probe-lab --output probes/output
```

`probes/output/gap-report.json` is now generated evidence and is ignored rather than committed.

## What Changed

- Golden org capture now resets probe data at the start of the run and before stateful probes, which removes the order-dependent DML/SOQL pollution that caused gap counts to swing between runs.
- `probe deploy` now deploys the probe SFDX project, assigns the probe permission set, fails on cleanup/seed errors, and uses `--ignore-conflicts` for repeat scratch-org lab updates.
- `probe local` writes `probes/output/local-results.json` instead of overwriting the shared org gap report.
- Salesforce Apex log-header daily-limit failures now fall back to an assertion-message capture path, so org capture can continue when debug logs are exhausted.
- Probe output and SFDX local tracking directories are ignored and removed from version control.

## Gap Fixes Landed

- `Decimal.divide(3, 2, RoundingMode.HALF_UP)` accepts an integer divisor/scale shape.
- Apex array syntax such as `String[]` and `Database.SaveResult[]` is treated as list-compatible at runtime.
- `AggregateResult.get(String)` is supported for aggregate aliases such as `COUNT_DISTINCT(... ) dist`.
- `Limits.getAsyncCalls()` and `Limits.getLimitAsyncCalls()` aliases are modeled.
- `Limits.getHeapSize()` recalculates current frame heap before returning.
- String-to-`Id` assignment validates Id shape and throws a catchable `StringException` for invalid Id text.
- Custom metadata describes now report `isCustom()` for `__mdt` objects.
- Probe-local schema includes Account-to-Contacts child relationship metadata.
- Unstable probes were tightened:
  - Dynamic SOQL uses a deterministic numeric predicate.
  - JSON map serialization checks semantic round-trip instead of key order.
  - `Datetime.valueOfGmt` formats with `formatGmt`.
  - SOQL subquery checks relationship support without depending on org Account row count.

## Notes For Next Work

The probe framework is now a usable parity loop: deploy/reset the scratch org, run `probe org`, and treat any new nonzero gap report as a regression or a newly exposed parity target. Keep `probes/output` local-only unless a future task explicitly asks for checked-in golden artifacts.
