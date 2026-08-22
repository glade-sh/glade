# CACHE_API67_RESULT

Reconciles the `Cache.Org`, `Cache.Session`, `Cache.Partition`, `Cache.OrgPartition`,
and `Cache.SessionPartition` Apex surface with Salesforce API 67, from the
`core-runtime-cache-partition-evidence` dry-run observations.

## Current contract correction

This is a historical dry-run record, not the current authority for
`validateKeys`. A later verified Salesforce receipt establishes that all three
partition receivers accept `validateKeys(Boolean, Set<String>)` at API 54, 55,
and 67. `validateKeys(Boolean, List<String>)` is accepted only at API 54 and
rejected at API 55 and 67. The collection element type is exactly `String`.

## Salesforce facts (raw dry-run observations)

- `getAvgValueSize` was removed after version 49.0.
- `getMaxValueSize` was removed after version 49.0.
- `Cache.Org.isAvailable()` does not exist.
- `Partition.createFullyQualifiedKey(String,String,String)`,
  `createFullyQualifiedPartition(String,String)`, `validatePartitionName(String)`,
  `validateKey(Boolean,String)`, and `validateKeyValue(Boolean,String,Object)`
  are static methods and must not be callable through an instance.
- The historical dry-run reported `Cache.Partition.validateKeys`,
  `Cache.OrgPartition.validateKeys`, and `Cache.SessionPartition.validateKeys`
  as removed after version 54.0. That observation is superseded by the current
  contract correction above.
- Fixture casts show builder-scoped `remove` returns `Boolean`, not `String`.
- `Cache.Session.isAvailable()` and partition `isAvailable()` are not in the
  rejection list and stay in the surface.

## Historical change snapshot

The following describes the historical packet that produced these dry-run
observations. It is superseded wherever it conflicts with the current contract
correction above.

- `internal/typesys/product_namespace_symbols_generated.go` — dropped
  `Cache.Org.isAvailable`.
- `internal/typesys/system_stub_symbols_generated.go` — dropped
  `getAvgValueSize`/`getMaxValueSize` from the five cache classes.
- `internal/sema/api67_surface.go` — `semaAPI67RejectedPlatformCall` rejects
  `cache.org.isavailable`, the value-size stat methods on all five classes,
  `validateKeys` on all three partition classes, and the five remaining static
  partition helpers when called through an instance. Generated legacy shapes
  stay available for evidence/versioned catalogs.
- `internal/vm/dispatch.go` — `Cache.Org.isAvailable` case removed; value-size
  cases removed; default case routes partition static calls through
  `callCachePartitionStaticDefault`.
- `internal/vm/dispatch_static.go` — canonical static callee list dropped
  `Cache.Org.isAvailable` and the value-size methods.
- `internal/vm/method_dispatch.go` — generated-family static fallback now
  routes partition statics through `callCachePartitionStaticDefault`
  (replaces the `validateCacheBuilder`-only carve-out).
- `internal/vm/platform_http_cache_resources.go` — instance partition member
  handler no longer admits the six static helpers or the value-size methods;
  builder-scoped `remove` returns `Boolean` on both static and instance paths;
  `callCachePartitionStaticDefault` implements the remaining static helpers and
  rejects removed `validateKeys` calls.
- `internal/vm/platform_test.go` — accepted shapes (static helpers,
  `Cache.Session.isAvailable`, `(Boolean)` builder remove) and a new
  `TestExecPlatformCacheAPI67RejectedShapes` proving runtime rejection.
- `internal/sema/cache_api67_surface_test.go` — accepted/rejected compile
  shapes via `AnalyzeAnonymous`.
- `internal/apextest/runner_test.go` — new end-to-end fixture
  `TestRunCasesContextCacheAPI67StaticPartitionHelpers` (remaining static
  helpers + Boolean builder remove through the full runner).

## Commands run

- `go build ./...` — ok
- `go test ./internal/vm` — ok (full package)
- `go test ./internal/sema -run 'Cache|API67'` — ok
- `go test ./internal/typesys` — ok
- `go test ./internal/apextest -run 'Cache'` and full `go test ./internal/apextest` — ok
- `go test ./internal/sema -run Cache -count=1` — ok
- `go test ./internal/vm -run Cache -count=1` — ok
- `go test ./internal/apextest -run Cache -count=1` — ok
- `go test ./internal/codeintel ./internal/startupcache ./internal/dap ./internal/repoguard ./internal/gladecli` — ok
- `go test ./...` — only two pre-existing failures, both present at HEAD
  before this change (see blockers)
- `scripts/smoke.sh` — smoke: ok

## Remaining blockers (pre-existing, outside this packet)

- `internal/sema` `TestPrepareAnalysisIndexKeepsPlatformSymbolsOutOfWorkspaceIndex`:
  platform type `Schema.FieldSetMap` is not known without index hydration.
  Verified failing at HEAD `e1a5bc18` with a clean tree.
- `internal/server` `TestToolingExecuteAnonymousLocalEventBusAndConnectApiStubs`:
  local `EventBus.publish(new Account(...))` now rejects non-platform-event
  records. Verified failing at HEAD `e1a5bc18` with a clean tree.
