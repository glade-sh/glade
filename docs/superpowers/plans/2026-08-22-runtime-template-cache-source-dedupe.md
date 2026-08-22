# Runtime Template Cache and Source Deduplication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve immutable schema caches across runner-owned runtime-template clones and scan each semantic source view once, while keeping arbitrary org changes fail-closed.

**Architecture:** Add one narrow VM attach method for fresh clones whose non-empty runtime schema stamp matches the primed cache generation; all other calls continue through `SetOrg`. Deduplicate only source-wide type-contract regex checks using the existing file/namespace/remap cache key, retain per-type member checks, and bump the semantic cache ABI because diagnostic output changes.

**Tech Stack:** Go, existing `storage.RuntimeTemplate`, VM metadata caches, semantic analyzer, Go tests.

---

### Task 1: Deduplicate source-wide type-contract checks

**Files:**
- Modify: `internal/sema/type_contracts_test.go`
- Modify: `internal/sema/type_contracts.go`
- Modify: `internal/sema/sema.go`

- [ ] **Step 1: Write the failing nested-source test**

```go
func TestTypeContractScansNestedClassSourceOnce(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Outer.cls": `public class Outer {
  public class Inner {}
  public void run() { System.debug(new List()); }
}`,
	})
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "GLADESEMA019" && strings.Contains(diagnostic.Message, "raw collection construction") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("raw collection diagnostics = %d, want 1: %#v", count, result.Diagnostics)
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./internal/sema -run TestTypeContractScansNestedClassSourceOnce -count=1`

Expected: FAIL because the source-wide diagnostic is emitted once per nested type.

- [ ] **Step 3: Add the minimal source-view guard and bump the ABI**

```go
seenSources := make(map[string]bool)
// Keep member checks above this guard.
sourceKey := semaSourceCacheKey(typ.File, typ.Namespace, typ.SourceNamespaceRemaps)
if seenSources[sourceKey] {
	continue
}
seenSources[sourceKey] = true
```

Change `SemanticABI` from `sema-v1` to `sema-v2` in the same behavior change.

- [ ] **Step 4: Verify GREEN and the full semantic package**

Run: `go test ./internal/sema -run TestTypeContractScansNestedClassSourceOnce -count=1`

Expected: PASS.

Run: `go test ./internal/sema -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sema/type_contracts.go internal/sema/type_contracts_test.go internal/sema/sema.go
git commit -m "Speed up source type contract checks"
```

### Task 2: Preserve schema caches for matching runtime-template clones

**Files:**
- Modify: `internal/vm/data_test.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/apextest/runner.go`

- [ ] **Step 1: Write the failing VM behavior test**

```go
func TestSetRuntimeTemplateOrgRetainsMatchingSchemaCaches(t *testing.T) {
	template := storage.NewRuntimeTemplate(storage.NewOrgState())
	template.Org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
	}}
	PrimeRuntimeTemplateSchema(&template)

	base := New(nil)
	base.PrimeMetadataSchema(&template.Org)
	clone := base.CloneRuntime(nil)
	shared := clone.jsonChildRelTypeCache
	org := template.CloneRuntimeOrg()
	clone.SetRuntimeTemplateOrg(&org)
	if clone.jsonChildRelTypeCache != shared {
		t.Fatal("matching runtime template detached schema cache")
	}

	org.ClearRuntimeSchemaStamp()
	shared = clone.jsonChildRelTypeCache
	clone.SetRuntimeTemplateOrg(&org)
	if clone.jsonChildRelTypeCache == shared {
		t.Fatal("mutated runtime template retained schema cache")
	}
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./internal/vm -run TestSetRuntimeTemplateOrgRetainsMatchingSchemaCaches -count=1`

Expected: build failure because `SetRuntimeTemplateOrg` does not exist.

- [ ] **Step 3: Implement the narrow stamped attach**

```go
func (vm *VM) SetRuntimeTemplateOrg(org *storage.OrgState) {
	stamp := runtimeSchemaStampHintForOrg(org)
	if stamp == "" || stamp != vm.metadataCacheStamp {
		vm.SetOrg(org)
		return
	}
	vm.Org = org
	if vm.soqlExecutionCache == nil {
		vm.soqlExecutionCache = soql.NewExecutionCache()
	}
	if vm.Org != nil {
		vm.Org.Now = func() time.Time { return vm.fakeNow }
	}
	if vm.testContext != nil && isPlaceholderCurrentUser(vm.testContext.CurrentUser) {
		vm.testContext.CurrentUser = vm.defaultTestCurrentUser()
	}
}
```

Replace only the three setup/test clone attachment calls in `runner.go`. Leave `initializeTestOrg`, arbitrary callers, and public `SetOrg` unchanged.

- [ ] **Step 4: Verify GREEN, invalidation, and runner behavior**

Run: `go test ./internal/vm -run 'TestSetRuntimeTemplateOrgRetainsMatchingSchemaCaches|TestSetOrgSamePointerClearsFieldDescribeCacheAfterLabelMutation|TestSetOrgUsesRuntimeSchemaStampOnlyAfterClearingSharedCaches' -count=1`

Expected: PASS.

Run: `go test ./internal/apextest -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/vm/runtime_state.go internal/vm/data_test.go internal/apextest/runner.go
git commit -m "Reuse schema caches in test runtime clones"
```

### Task 3: Verify the integrated change

**Files:**
- No production changes expected.

- [ ] **Step 1: Run focused and race checks**

Run: `go test ./internal/sema ./internal/vm ./internal/apextest -count=1`

Expected: PASS.

Run: `go test -race ./internal/vm -run TestSetRuntimeTemplateOrgRetainsMatchingSchemaCaches -count=1`

Expected: PASS.

- [ ] **Step 2: Run the broad repository gate**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Build and verify the two full corpora**

Build the branch binary and run sf-cred and NU with `--parallelism 4 --no-cache --no-serve`. Require 4,565/4,565 sf-cred passes and the same NU projection: 11,518 passes plus the same eight assertion failures, with no compile, runtime, unsupported, timeout, or infrastructure errors.

- [ ] **Step 4: Review the final diff**

Run: `git diff origin/main...HEAD --check`

Expected: no whitespace errors.

Run: `git status --short --branch`

Expected: clean feature branch.
