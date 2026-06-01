# Local Test Performance Regression Review

Date: 2026-06-01

Scope: staged changes in the worktree and commit `1b1893dfdedb75bacd82c669db8edf5e8612eef8`.

No code changes were made for this review.

## Salesforce Test Ground Rules

Salesforce Apex tests must keep test data isolated. For Apex saved on API version 24.0 or later, tests do not see pre-existing org records unless `@IsTest(SeeAllData=true)` grants that access. Unit test methods do not commit data to the org database. Test setup data is a starting point for each test method, not shared mutable state between methods.

That means a local runtime must keep these things isolated per test method:

- records created or changed by DML
- trigger side effects and rollback state
- static state and current request/page state
- governor limits and async test execution state
- any schema or metadata overlays that a test mutates

It does not mean every derived lookup table must be private. Immutable metadata-derived caches can be shared when the cache key proves the schema shape is the same and the cached values cannot be mutated by a test. Mutable data, mutable metadata overlays, and values returned by reference must stay private or be cloned on read.

Sources checked:

- Salesforce Apex Developer Guide, test data isolation: https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_testing_data_access.htm
- Salesforce Apex Developer Guide, running unit tests: https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_testing_unit_tests_running.htm
- Salesforce `@IsTest` annotation reference: https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_classes_annotation_isTest.htm

## Measured Regression

Focused timing was run with three temporary binaries:

- parent of `1b1893df`
- `1b1893df`
- current staged worktree

The useful probe was:

```bash
/tmp/glade-review-<bin> compat local-tests \
  --project example-projects/apex-recipes-main \
  --class AccountTriggerHandler_Tests \
  --json
```

Results:

| Build | Duration | Total alloc | Mallocs |
| --- | ---: | ---: | ---: |
| parent of `1b1893df` | 5,055 ms | 1.43 GB | 12,980,688 |
| `1b1893df` | 5,204 ms | 1.44 GB | 13,050,899 |
| staged worktree | 7,386 ms | 1.96 GB | 17,546,313 |

The named commit added only a small cost on this probe. The staged worktree added about 42 percent wall time over `1b1893df`, with about 523 MB more allocation.

The NPSP probe below confirmed the staged worktree is slower on a heavier corpus path, but it was already dominated by a runtime-gap loop and is not the cleanest gauge:

```bash
/tmp/glade-review-<bin> compat local-tests \
  --project example-projects/NPSP-rel-3.237 \
  --class ADDR_Addresses_TEST \
  --method updateAccAddrNew \
  --json
```

`1b1893df` ran in 27,101 ms. The staged worktree ran in 29,208 ms.

## Main Findings

### 1. Relationship describe caches stopped sharing across clones

File: `internal/vm/runtime_state.go`

`CloneRuntime` now creates private relationship describe caches:

```go
clone.jsonChildRelTypeCache = newJSONChildRelTypeLookupCache()
clone.childRelCache = newChildRelationshipCache()
```

That protects tests that mutate schema metadata after `SetOrg`. That part respects Salesforce isolation. A test method cannot poison another method with a stale schema shape.

The performance cost is that every test clone loses the shared metadata-derived relationship cache. The runner still primes a schema stamp before cloning:

```go
baseMachine.PrimeMetadataSchema(&org)
```

So the runner still expects shared schema cache behavior, but `CloneRuntime` no longer gives it for these caches. The old optimization was cut off at the stump.

The right recovery is not a blind revert. The right recovery is a split cache rule:

- share relationship describe caches for immutable schema stamps
- clone returned values or store immutable cache entries
- invalidate or fork the cache when a test mutates schema metadata
- keep per-test overlays private

That follows Salesforce behavior and keeps the old speed.

### 2. Child relationship cache values now deep-copy on load and store

File: `internal/vm/describe_runtime.go`

The staged worktree changed child relationship cache access from slice copy to value cloning:

```go
return cloneChildRelationshipValues(value), true
...
c.entries[key] = cloneChildRelationshipValues(value)
```

This fixes mutation bleed if callers alter returned describe values. It also adds allocation on a hot describe path. If the cache becomes shared again, this cloning may still be needed on read unless cache entries become immutable by construction.

Best direction:

- cache immutable child relationship descriptors
- return cheap value copies where fields are immutable
- deep-copy only fields that can be mutated by Apex-visible code

### 3. Trigger record preservation now clones broader record state

File: `internal/vm/trigger_runtime.go`

Before-trigger and store-trigger paths now preserve more fields and clone more values:

- `storeTriggerRecords` clones the stored record, then merges changed fields.
- `runTrigger` calls `preserveMissingRecordFields`.
- `preserveMissingRecordFields` clones missing field values from the original record.

This matches Salesforce behavior better. In a before trigger, `Trigger.new` contains records with fields that can persist through trigger execution. Missing fields should not vanish because a local conversion produced a narrower record.

The cost shows in the Recipes trigger class. On the staged worktree:

- `afterUpdateTestPositive`: 1,055 ms to 1,487 ms
- `beforeInsertTestPositive`: 54 ms to 121 ms
- total class: 5,204 ms to 7,386 ms

Best direction:

- preserve fields only when the trigger conversion path dropped fields
- avoid cloning fields that are known immutable scalar values
- track changed fields during VM-to-record conversion and merge only those
- keep full clone behavior for relationship/object values that can mutate

### 4. Alias propagation widened on method return and collection mutation

Files: `internal/vm/method_dispatch.go`, `internal/vm/value_aliasing.go`

The staged worktree added broader propagation from alias snapshots to scopes and statics. Collection mutations now call `propagateCollectionMutationFromSnapshot`. Method returns also propagate receiver aliases through scope and statics more often.

That is correctness work. Apex passes object references. Mutating a list, map, set, or object through one alias must be visible through another alias in the same transaction. Salesforce test isolation does not permit dropping those mutations inside a test method.

The cost is that these paths scan:

- live scopes
- static value references
- nested lists, maps, sets, and objects

Best direction:

- keep alias correctness inside one test method
- make propagation conditional on a known escaping ref
- maintain a ref-to-location index for scopes the same way statics use `staticValueRefs`
- skip nested scans when both old and new values have no nested refs
- keep static state clone-local between test methods

### 5. Field alias handling added linear scans

Files: `internal/vm/lookup_assign.go`, `internal/vm/dml_runtime.go`

The staged worktree added namespace-stripped and case-insensitive scans for field lookup and assignment. That helps Salesforce-shaped field behavior, especially namespace aliases and SObject field names.

The cost is repeated scans through field maps on common reads, writes, and DML conversion.

Best direction:

- cache per-object stripped field aliases
- cache per-value field alias lookups when the value shape changes
- keep SObject alias handling separate from ordinary Apex object fields
- preserve exact-name fast paths before namespace and case-fold fallback

### 6. `1b1893df` added source scanning in org setup

File: `internal/apextest/runner.go`

`1b1893df` added synthetic field set inference and product download URL formula inference. The source scanner reads Apex files and runs regex patterns to infer metadata.

That work belongs to org setup, not the per-method execution hot loop. On the Recipes trigger probe, `1b1893df` was close to its parent. It is not the main regression shown above.

Still, this path can matter on broad projects. It should stay cached by project/index key. It should not rerun per test method.

## Recommended Recovery Order

1. Restore shared immutable relationship describe caching with schema-stamp isolation.
2. Keep private caches only after schema mutation or metadata overlay mutation.
3. Add benchmarks around `AccountTriggerHandler_Tests` and one NPSP trigger-heavy method before changing hot paths.
4. Narrow trigger record preservation to changed or dropped fields.
5. Add alias-propagation fast guards and a scope ref index.
6. Cache stripped field aliases for SObject field reads, writes, and DML conversion.

## Guardrails

Do not trade correctness for speed:

- Do not share org records between test methods.
- Do not share static state between test methods.
- Do not share current page, request, test context, limits, async queue, or isolation journal.
- Do not share mutable schema overlays across test methods.
- Do not return mutable shared describe values unless they are cloned or immutable.

Share only what Salesforce would make stable across test methods:

- compiled Apex metadata
- immutable schema metadata
- immutable describe lookup tables keyed by a schema stamp
- parsed programs and method dispatch indexes
- standard platform metadata that tests cannot mutate

The fastest path is not a bare revert. It is putting the old caches back behind a sharper boundary.
