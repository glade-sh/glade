# Managed Package Dependency Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the next large parity slice by making source-backed and artifact-backed managed package dependencies resolve, enforce access rules, and run enough local contract behavior to move `pkgx` package blockers out of the example-project compile frontier.

**Architecture:** Treat managed packages as installed dependency contracts, not current-project source. Load dependency symbols and schema before the consuming project, preserve dependency provenance through sema and VM lookup, enforce cross-namespace `global` visibility, and classify unresolved package behavior as dependency-scoped diagnostics.

**Tech Stack:** Go 1.26, `internal/config`, `internal/project`, `internal/typesys`, `internal/sema`, `internal/vm`, `internal/apextest`, `internal/packageartifact`, compatibility fixtures.

---

## Why This Slice

The current large-project frontier points at installed package contracts before deeper runtime behavior:

- `src-nmb-nc-develop` is blocked first by unknown managed Apex type `pkgx.TrailAddress`.
- `nams-workspace` is blocked first by unknown managed Apex type `pkgx.TrailPlugin`.
- The checked managed-package plan says source-backed and artifact-backed dependency config, loading, symbol/schema loading, and first artifact support already exist; the remaining work is stronger cross-namespace access enforcement and applying artifact-backed contracts to `nams-workspace`.

This slice is the best next log to roll because it unlocks two measured example-project fronts and prevents broad compile-gap counts from hiding the next real runtime families.

## Alternatives Considered

1. **Managed package dependency closure.** Recommended. It attacks the current `pkgx` blockers and strengthens a general Salesforce package model.
2. **SOQL and dynamic relationship fidelity.** Valuable, but it comes after package contracts compile enough code to expose selector/runtime failures.
3. **Platform/UI controller APIs.** Valuable, but the unresolved projects still need dependency surfaces before controller tests can give clean signal.

## File Structure

- Modify: `internal/typesys/symbols.go` for dependency symbol/schema provenance and conflict behavior.
- Modify: `internal/sema/type_members.go` and `internal/sema/body_calls.go` for cross-namespace lookup and access diagnostics.
- Modify: `internal/vm/method.go`, `internal/vm/method_dispatch.go`, `internal/vm/construct_runtime.go`, `internal/vm/vm.go`, and `internal/vm/provenance.go` for dependency-aware runtime dispatch.
- Modify: `internal/apextest/runner.go` for dependency test exclusion, dependency class compilation, and result reporting.
- Modify: `internal/packageartifact/artifact.go` for any missing contract fields needed by sema/VM.
- Test: `internal/typesys/symbols_test.go`, `internal/sema/sema_test.go`, `internal/vm/provenance_test.go`, `internal/vm/method_test.go`, `internal/apextest/runner_test.go`, and `internal/packageartifact/artifact_test.go`.
- Create or extend: `testdata/local-tests/managed-package-access`.
- Create or extend: `testdata/local-tests/managed-package-runtime`.
- Update: `docs/MANAGED_PACKAGE_DEPENDENCY_PLAN.md`, `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`, and generated compatibility docs only when behavior changes.

### Task 1: Refresh The Dependency Frontier

**Files:**
- Read: `docs/fixtures/local-tests-example-projects.json`
- Read: `docs/MANAGED_PACKAGE_DEPENDENCY_PLAN.md`
- No code changes.

- [ ] **Step 1: Capture current managed-package blocker output**

Run:

```bash
go run ./cmd/glade compat local-tests --project example-projects/src-nmb-nc-develop --timeout 30000 --top-failures 8 --json
go run ./cmd/glade compat local-tests --project example-projects/nams-workspace --timeout 30000 --top-failures 8 --json
```

Expected before this slice: top failures still name missing or inaccessible `pkgx` Apex/schema surfaces, or dependency diagnostics show the next exact package setup issue.

- [ ] **Step 2: Build the local `pkgx` package artifact**

Run:

```bash
mkdir -p example-projects/.glade/packages/pkgx
go run ./cmd/glade package build \
  --project example-projects/src-nmb-nu-develop \
  --namespace pkgx \
  --version src-nmb-nu-develop@local \
  --output example-projects/.glade/packages/pkgx/package.glade.json
```

Expected: artifact contains global Apex types, global members, and installed namespace schema for `pkgx`.

### Task 2: Enforce Cross-Namespace Access In Sema

**Files:**
- Modify: `internal/sema/type_members.go`
- Modify: `internal/sema/body_calls.go`
- Test: `internal/sema/sema_test.go`

- [ ] **Step 1: Add failing sema coverage**

Add test cases proving:

```apex
// dependency package
global class TrailVisibleService {
    global static String ok() { return 'ok'; }
    public static String hidden() { return 'hidden'; }
}

// consuming package
public class Consumer {
    public static String allowed() { return pkgx.TrailVisibleService.ok(); }
    public static String denied() { return pkgx.TrailVisibleService.hidden(); }
}
```

Expected diagnostics: `allowed` passes; `denied` returns a dependency member access diagnostic rather than an unknown method.

- [ ] **Step 2: Implement dependency access checks**

Use existing dependency metadata on type/member symbols. Cross-namespace references must require `global` on dependency top-level types and called members. Same-dependency internal calls continue to see package `public` members.

- [ ] **Step 3: Verify sema**

Run:

```bash
go test ./internal/sema ./internal/typesys
```

Expected: dependency access tests pass and existing visibility tests remain green.

### Task 3: Preserve Dependency Provenance Through VM Dispatch

**Files:**
- Modify: `internal/vm/provenance.go`
- Modify: `internal/vm/method.go`
- Modify: `internal/vm/method_dispatch.go`
- Modify: `internal/vm/construct_runtime.go`
- Modify: `internal/vm/vm.go`
- Test: `internal/vm/provenance_test.go`
- Test: `internal/vm/method_test.go`

- [ ] **Step 1: Add failing VM dispatch coverage**

Add tests for duplicate project and dependency class names:

```apex
// dependency package pkgx
global class TrailCartService {
    global static String value() { return 'dependency'; }
}

// consuming project
public class TrailCartService {
    public static String value() { return 'project'; }
}

public class Consumer {
    public static String dependencyValue() {
        return pkgx.TrailCartService.value();
    }
}
```

Expected: `Consumer.dependencyValue()` returns `dependency`.

- [ ] **Step 2: Implement dispatch preference**

When a callee or receiver is namespace-qualified to a dependency, prefer dependency-origin fields, methods, constructors, static initializers, and class metadata. When unqualified inside the project, prefer project-origin symbols.

- [ ] **Step 3: Enforce runtime access parity**

Mirror sema denial for dependency `public`, `protected`, private, and `@TestVisible` members when calls cross from consuming package code.

- [ ] **Step 4: Verify VM**

Run:

```bash
go test ./internal/vm
```

Expected: dependency provenance tests pass, and existing VM dispatch tests remain green.

### Task 4: Compile And Run Source-Backed Dependency Contracts

**Files:**
- Modify: `internal/apextest/runner.go`
- Modify: `internal/vm/method.go`
- Test: `internal/apextest/runner_test.go`
- Fixture: `testdata/local-tests/managed-package-runtime`

- [ ] **Step 1: Add runtime fixture**

Create or extend `testdata/local-tests/managed-package-runtime` with:

```apex
// dependency package
global class TrailService {
    global static String externalValue() {
        return DependencyHelper.internalValue();
    }
}
public class DependencyHelper {
    public static String internalValue() {
        return 'from dependency';
    }
}

// consuming package test
@IsTest
private class ManagedRuntimeTest {
    @IsTest static void callsDependencyGlobalApi() {
        System.assertEquals('from dependency', pkgx.TrailService.externalValue());
    }
}
```

Expected: dependency internals are callable from dependency code, not from consuming code.

- [ ] **Step 2: Exclude dependency test classes by default**

Ensure dependency package tests do not inflate the consuming project test count unless a future explicit flag asks for them.

- [ ] **Step 3: Verify test runner**

Run:

```bash
go test ./internal/apextest ./internal/vm
go run ./cmd/glade test --project testdata/local-tests/managed-package-runtime --json
```

Expected: the consumer test passes and no dependency test class runs by default.

### Task 5: Apply Artifact Contracts To Example Projects

**Files:**
- Modify only local fixture/config files under `example-projects` if they are checked-in fixtures.
- Do not hard-code `pkgx` in production runtime code.

- [ ] **Step 1: Wire `pkgx` artifact config for focused runs**

Use:

```yaml
project:
  managedPackageDependencies: ["pkgx:artifact:../.glade/packages/pkgx/package.glade.json:src-nmb-nu-develop@local"]
```

Expected: local runs report the artifact as loaded.

- [ ] **Step 2: Run focused example-project probes**

Run:

```bash
go run ./cmd/glade compat local-tests --project example-projects/src-nmb-nc-develop --timeout 30000 --top-failures 8 --json
go run ./cmd/glade compat local-tests --project example-projects/nams-workspace --timeout 30000 --top-failures 8 --json
```

Expected: `pkgx.TrailAddress`, `pkgx.TrailPlugin`, and `pkgx__TrailLine__c` no longer appear as plain unknown type/SObject blockers. Remaining failures are compile gaps, runtime gaps, access diagnostics, or unsupported diagnostics with package context.

### Task 6: Update Gates And Docs

**Files:**
- Modify: `docs/MANAGED_PACKAGE_DEPENDENCY_PLAN.md`
- Modify: `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`
- Modify generated docs only through generator commands.

- [ ] **Step 1: Record the new frontier**

Update the managed-package plan with what now works and what the next measured blocker is.

- [ ] **Step 2: Refresh generated compatibility docs**

Run:

```bash
go run ./cmd/glade compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade compat gaps --output docs/KNOWN_GAPS.md
go run ./cmd/glade compat stdlib --output docs/STDLIB_COVERAGE.md
go run ./cmd/glade compat salesforce-coverage --output docs/generated/SALESFORCE_COVERAGE_MANIFEST.md
go run ./cmd/glade compat salesforce-coverage --output docs/generated/SALESFORCE_COVERAGE_MANIFEST.json
```

Expected: generated docs match the code after the slice.

- [ ] **Step 3: Final verification**

Run:

```bash
go test ./internal/config ./internal/project ./internal/packageartifact ./internal/typesys ./internal/sema ./internal/vm ./internal/apextest ./internal/compat
go run ./cmd/glade test --project testdata/local-tests/managed-package-consumer --json
go run ./cmd/glade test --project testdata/local-tests/managed-package-artifact-consumer --json
go run ./cmd/glade test --project testdata/local-tests/managed-package-access --json
go run ./cmd/glade test --project testdata/local-tests/managed-package-runtime --json
go run ./cmd/glade compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade compat gaps --check docs/KNOWN_GAPS.md
go run ./cmd/glade compat stdlib --check docs/STDLIB_COVERAGE.md
go run ./cmd/glade compat salesforce-coverage --check docs/generated/SALESFORCE_COVERAGE_MANIFEST.md
go run ./cmd/glade compat salesforce-coverage --check docs/generated/SALESFORCE_COVERAGE_MANIFEST.json
```

Expected: all commands exit 0. Example-project probes show a new top blocker beyond plain missing `pkgx` dependency contracts.

## Stop Conditions

- Do not add current-project stubs for `pkgx` classes or `pkgx__*` objects.
- Do not promote managed-package dependency support to `supported` until access enforcement, artifact loading, source-backed runtime execution, and fixture coverage all pass.
- If the local `pkgx` source/artifact is unavailable, stop at a clean `dependency_missing` or `dependency_load_error` diagnostic and keep production code general.
