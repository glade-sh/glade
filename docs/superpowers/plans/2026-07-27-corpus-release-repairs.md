# Corpus Release Repairs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove confirmed corpus false positives and make the next-release verification gate deterministic under realistic test load.

**Architecture:** Keep Apex compatibility rules narrow and version-aware. Every semantic repair gets a focused red/green regression before the production change; corpus/configuration defects remain classified rather than silently suppressed. Readiness publication becomes atomic, and the local release gate uses the same bounded-resource model as CI.

**Tech Stack:** Go, Go tests, Apex parser/type index/sema, shell release checks.

---

### Task 1: Annotation contracts and generated-flow precedence

**Files:**
- Modify: `internal/apexlang/annotation_catalog.go`, `internal/apexlang/annotation_catalog_test.go`
- Modify: `internal/sema/annotation_contracts.go`, `internal/sema/annotation_contracts_test.go`, `internal/sema/scratch_rule_regressions_test.go`
- Modify: `internal/sema/type_members.go`, `internal/sema/sema_test.go`

- [ ] Add red tests for all valid string `JsonAccess` modes, method `IsTest(SeeAllData=true)`, a zero-parameter invocable method, API-49/API-50 NamespaceAccessible behavior, and a `Test.flow-meta.xml` project that must resolve both `System.Test` and `Flow.Interview.Test`.
- [ ] Run the focused tests and observe the current catalog/contract/flow failures.
- [ ] Implement string-mode JsonAccess validation, API-gated NamespaceAccessible interfaces, valid method SeeAllData, zero-or-one invocable parameters, de-duplicated nested declaration traversal, and no unqualified aliases for generated `Flow.Interview.*` types.
- [ ] Update the scratch oracle for a valid NamespaceAccessible interface and rerun focused annotation and flow tests.

### Task 2: Inheritance, enum-switch, lexical assignment, and static-final contracts

**Files:**
- Modify: `internal/sema/inheritance_contracts_test.go`, `internal/sema/declaration_contracts_test.go`, `internal/sema/statement_contracts_test.go`, `internal/sema/type_contracts_test.go`
- Modify: `internal/sema/sema_checks.go`, `internal/sema/statement_contracts.go`, `internal/sema/body_ir.go`

- [ ] Add red tests for a legacy abstract implementation without `override`, qualified `JSONToken` and `Schema.DisplayType` branches, case-insensitive local variables shadowing get-only properties, and static-final initialization in a static initializer.
- [ ] Add API-64/API-65 access-modifier controls without changing production behavior unless the existing version propagation fails.
- [ ] Change override enforcement to require `override` only for inherited concrete virtual methods; preserve required abstract implementation diagnostics.
- [ ] Accept only recognized platform enum values in switch cases, resolve lexical bindings before properties, and permit static-final initialization only from a static initializer.
- [ ] Rerun the focused semantic packages and retain negative controls for virtual overrides, arbitrary static fields, direct read-only property writes, instance initialization, and method reassignment.

### Task 3: Platform stubs, schema metadata, and classifier correctness

**Files:**
- Modify: platform symbol generation/input and its tests for `Messaging.InboundEmailHandler`, `Location`, and `Exception` signatures
- Modify: schema/project metadata loading and `internal/sema/dml_contracts_test.go`
- Modify: `/Users/matt/Dev/glade-tools` corpus classifier and classifier tests only if the fix belongs to the first-party maintenance tool

- [ ] Add red product tests for interface implementation, Location factory/distance calls, custom exception assignability, and namespaced/decomposed external-ID upsert metadata.
- [ ] Run each probe against current Glade and record the expected failure.
- [ ] Correct only the generated symbol/schema paths evidenced by those tests; do not add name-based exceptions in sema.
- [ ] Add a classifier test where a real `fflib_*.cls` source path is not labelled missing-package metadata.
- [ ] Rerun focused product and maintenance-tool tests.

### Task 4: Resolve the remaining oracle-dependent diagnostics

**Files:**
- Modify: `internal/sema/scratch_rule_regressions_test.go` and the corresponding sema implementation only after an oracle result

- [ ] Create minimal Salesforce compile probes for `Object.hashCode`, custom exception `getMessage`, Aura-enabled overload behavior, and any remaining exception/switch/type-inference families that are still present after Tasks 1-3.
- [ ] Record each result as an accept or reject scratch oracle.
- [ ] Implement only the behavior proven by the oracle, with a focused red/green test.
- [ ] Leave genuine partial-project dependency failures classified as corpus configuration work.

### Task 5: Readiness and release-harness reliability

**Files:**
- Create: `internal/gladecli/ready_file.go`, `internal/gladecli/ready_file_test.go`, `scripts/release_check_test.go`
- Modify: `internal/gladecli/dev_lwc_command.go`, `internal/gladecli/dev_vf_command.go`, `internal/gladecli/db_command.go`
- Modify: `scripts/ci_race_test.go`, `scripts/release-check.sh`

- [ ] Add a deterministic red test proving a published readiness file remains complete until atomic replacement.
- [ ] Implement same-directory temporary-file publication and rename; route all three ready-file producers through it.
- [ ] Replace the CI-race test's fixed startup deadline with an early-exit-aware condition wait bounded by the Go test deadline.
- [ ] Add a release-script contract test, then set `GOMAXPROCS=2` and use `go test -count=1 -p=2 ./...` in the local release gate.
- [ ] Run the ready-file, CI-race, release-script, and bounded-suite checks.

### Task 6: Rebuild and validate the release gate

**Files:**
- No source changes unless a validation failure identifies a new, reproducible defect.

- [ ] Run `go test -count=1 ./internal/sema`, focused changed-package tests, and `GOMAXPROCS=2 go test -count=1 -p=2 ./...`.
- [ ] Build a fresh Glade binary and rerun public success, public-fail classification, and anonymized private corpus commands from the audit evidence.
- [ ] Compare diagnostics against the pre-fix counts; every remaining nonzero diagnostic must be a documented oracle, corpus metadata defect, or explicit waiver.
- [ ] Run independent spec and code-quality reviews, then prepare the branch for user review.
