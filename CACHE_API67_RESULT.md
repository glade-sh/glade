# CACHE_API67_RESULT

Reconciles the `Cache.Org`, `Cache.Session`, `Cache.Partition`, `Cache.OrgPartition`,
and `Cache.SessionPartition` Apex surface with Salesforce API 67, from the
`core-runtime-cache-partition-evidence` dry-run observations.

## Salesforce facts (raw dry-run observations)

- `getAvgValueSize` was removed after version 49.0.
- `getMaxValueSize` was removed after version 49.0.
- `Cache.Org.isAvailable()` does not exist.
- `Partition.createFullyQualifiedKey(String,String,String)`,
  `createFullyQualifiedPartition(String,String)`, `validatePartitionName(String)`,
  `validateKey(Boolean,String)`, `validateKeyValue(Boolean,String,Object)`, and
  `validateKeys(Boolean,Set<String>)` are static methods and must not be
  callable through an instance.
- Fixture casts show builder-scoped `remove` returns `Boolean`, not `String`.
- `Cache.Session.isAvailable()` and partition `isAvailable()` are not in the
  rejection list and stay in the surface.

## Files changed

- `internal/typesys/product_namespace_symbols_generated.go` — dropped
  `Cache.Org.isAvailable`.
- `internal/typesys/system_stub_symbols_generated.go` — dropped
  `getAvgValueSize`/`getMaxValueSize` from the five cache classes.
- `internal/sema/api67_surface.go` — `semaAPI67RejectedPlatformCall` rejects
  `cache.org.isavailable`, the value-size stat methods on all five classes, and
  the six static partition helpers when called through an instance. Generated
  legacy shapes stay available for evidence/versioned catalogs.
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
  new `callCachePartitionStaticDefault` implements the static helpers.
- `internal/vm/platform_test.go` — accepted shapes (static helpers,
  `Cache.Session.isAvailable`, `(Boolean)` builder remove) and a new
  `TestExecPlatformCacheAPI67RejectedShapes` proving runtime rejection.
- `internal/sema/cache_api67_surface_test.go` — accepted/rejected compile
  shapes via `AnalyzeAnonymous`.
- `internal/apextest/runner_test.go` — new end-to-end fixture
  `TestRunCasesContextCacheAPI67StaticPartitionHelpers` (static helpers +
  Boolean builder remove through the full runner).

## Commands run

- `go build ./...` — ok
- `go test ./internal/vm` — ok (full package)
- `go test ./internal/sema -run 'Cache|API67'` — ok
- `go test ./internal/typesys` — ok
- `go test ./internal/apextest -run 'Cache'` and full `go test ./internal/apextest` — ok
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
