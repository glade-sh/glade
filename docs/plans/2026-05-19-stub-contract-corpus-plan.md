# Stub Contract Corpus Plan

## Summary

Yes. Worthwhile. The repo already has stub shape coverage. The missing board is executable contract evidence.

Build a generated **Stub Contract Corpus** from `example-projects/stubs` and current `compat stub-behavior`. Every stub type and member gets one contract row. Each row tells oaer what to do: implement behavior, return passive DTO/defaults, capture side effects, or fail with a named unsupported diagnostic.

Current live counts:

- `stub-inventory`: `7,090` system stub classes, `1,373` SObject stubs.
- `stub-behavior`: `82,800` entries, `34,471` implemented, `47,656` passive default, `673` unsupported, `0` unknown.
- Shape is mostly cut square. Behavior evidence is the next notch.

## Key Changes

- Add `compat stub-contracts`.
  It emits and checks `docs/generated/STUB_CONTRACTS.json`.
  Each entry includes stub ID, type, member, status, contract mode, required evidence, implementation owner, and unsupported reason when applicable.

- Generate Apex probe classes under `probes/sfdx`.
  Do not hand-write thousands of probes. Generate partitioned classes by namespace or family, plus a generated router called from `ProbeRunner`.

- Use four contract modes.
  `org-diff`: run in scratch org and oaer, compare normalized result.
  `local-contract`: assert oaer behavior only, used for explicit unsupported and local-only side effects.
  `passive-dto`: assert constructor/property/getter/setter/default behavior.
  `compile-shape`: prove type/member availability when execution is unsafe or meaningless.

- Extend `internal/probe`.
  Load generated contract manifests beside the existing hand-written probe manifest. Keep existing probes as curated high-value tests. Add generated probes as the broad coverage net.

- Connect contracts to implementation work.
  When a contract fails, the report must name the needed implementation area: VM stdlib, SObject metadata, SOQL, DML, side-effect recorder, ApexPages/UI, async, or explicit unsupported.

## Implementation Steps

1. Add the contract schema and generator.
   Read `BuildStubBehaviorReport()` and `BuildStubInventoryReport()`.
   Emit stable IDs and one policy row per behavior entry.

2. Add `oaer compat stub-contracts --json|--output|--check`.
   Fail check mode when a stub member lacks a contract row, policy, or evidence mode.

3. Add generated Apex probe output.
   Generate small classes, not one big class. Keep class size under Apex limits.
   Add a generated router so `ProbeRunner` does not grow by hand.

4. Add probe manifest loading.
   Keep `internal/probe/manifest.go` for curated probes.
   Add generated contract manifest loading for stub contracts and tiers.

5. Add initial tiers.
   `stub-smoke`: core System/String/Date/JSON/Schema DTO probes.
   `stub-core`: high-use platform classes and SObjects.
   `stub-full`: all generated safe contracts.
   `stub-local`: explicit unsupported and local-only contracts.

6. Add gap reports.
   Report missing implementation as structured rows, not raw failures.
   Include `contractId`, `mode`, `status`, `owner`, `golden`, `local`, and next implementation hint.

7. Update docs.
   Replace the broad future language in `docs/BEHAVIORAL_STUB_SUPPORT_PLAN.md` with this contract workflow.
   Keep the rule: no behavior moves to implemented without contract evidence.

## Test Plan

- Unit test contract generation from a tiny fake stub set.
- Unit test stable contract IDs and deterministic JSON ordering.
- CLI tests for `compat stub-contracts --json`, `--output`, and `--check`.
- Probe tests for generated router dispatch.
- Local-only probe test for one passive DTO and one explicit unsupported member.
- Scratch-org validation gate for `stub-smoke` when `oaer-probe-lab` is available.
- Regression gate: `compat stub-inventory --json` and `compat stub-behavior --json` still run clean.

## Assumptions

- Primary harness is **Contract Corpus**.
- Generated coverage comes first. Hand-written probes stay only for tricky behavior.
- Cloud-only or mutating platform service calls do not get fake behavior. They get explicit unsupported contracts unless a deterministic local model exists.
- Scratch-org probes use public behavior only. No proprietary AER internals.
- Existing dirty worktree changes stay untouched during implementation.
