# Salesforce Local Apex Release Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce one release successor whose exact Glade binary is proven to compile and execute the declared local Apex profile, run composed Apex test projects, and pass the existing exact-SHA release authorities.

**Architecture:** Keep the active Salesforce campaign single-writer. Product changes live in isolated Glade worktrees, fixture orchestration stays in `glade-tools`, and release promotion reuses the existing candidate authority, local proof, Salesforce reconciliation, CI, release-check, and artifact-attestation rails. Do not add another completeness scanner, dashboard, evidence database, or hosted-service emulator.

**Tech Stack:** Go 1.26, Apex/SFDX fixtures, existing Glade CLI and VM, existing `glade-tools` compatibility runner, Bash, JSON/JUnit receipts, GitHub Actions release authorities.

---

## Current authority and scope

- Remote `main` was rebound on 2026-08-31 to `693d2e809732e27fac66d21d268e86b521af8cd8`.
- Work is isolated at `/Users/matt/Dev/glade-release-readiness-20260831` on `codex/apex-release-readiness`.
- The active Salesforce campaign remains separate. At the planning snapshot it had attempted every eligible row and was still resolving two rejected mismatches plus infrastructure outcomes.
- This branch receives no Salesforce credit. Any merge after the campaign freezes creates a fresh successor and requires fresh local and Salesforce proof.
- The release claim is **complete local Apex execution and test running for the declared local profile**. Hosted identity, licensing, external services, real network transport, and other named hosted boundaries remain explicit non-parity.

## Definition of done

- Every current-schema managed-package artifact has a supported, preserved `sourceApiVersion`; artifact runtime symbols carry it.
- Declared SFDX dependencies fail explicitly when missing or ambiguous; adjacent directories cannot silently change the selected package.
- A configured, hash-pinned describe snapshot participates in `check` and `test`, with project metadata taking precedence.
- Runner acceptance covers mock isolation, `SeeAllData`, and legacy `testMethod` through `apextest.Run`.
- A built release binary passes `check` and `test` across five composed fixtures with exactly `9/9` passing methods, plus one authorized real SFDX project.
- Public debug-test documentation matches observed behavior.
- The final successor passes `scripts/release-check.sh`, exact-SHA Required CI, exact-SHA Salesforce Correctness bound to one `glade-tools` SHA, artifact attestations, install, pinned-install, and upgrade checks.

## File ownership

### Glade product repository

- `internal/packageartifact/artifact.go`: artifact API-version validation and normalization.
- `internal/project/project.go`: artifact metadata validation and deterministic dependency resolution.
- `internal/typesys/symbols.go`: artifact symbol API-version propagation.
- `internal/config/config.go`: explicit hash-pinned schema snapshot configuration.
- `internal/schema/schema.go`: pinned snapshot loading and project-metadata precedence.
- `internal/gladecli/test_command_selectors_test.go`: imported-schema participation in CLI `check`/`test`.
- `internal/apextest/runner_test.go`: test-framework lifecycle acceptance.
- `site/docs-src/help/debug-apex-vscode.md`: truthful local-test debugging description.
- `docs/RELEASE_NOTES.md`: versioned notes, changed only in the final successor.

### Glade Tools maintenance repository

- `scripts/release-local-apex-check.sh`: release-binary composed-project gate.
- `scripts/release_local_apex_check_test.go`: fake-binary contract for exact fixtures and counts.
- `scripts/release-check.sh`: optional invocation with an exact candidate binary.
- `README.md`: operator command and receipt location.

## Task 1: Preserve managed-package artifact API versions

**Files:**
- Modify: `internal/packageartifact/artifact.go`
- Modify: `internal/packageartifact/artifact_test.go`
- Modify: `internal/project/project.go`
- Modify: `internal/project/project_test.go`
- Modify: `internal/typesys/symbols.go`
- Modify: `internal/typesys/symbols_test.go`

- [ ] **Step 1: Write the failing artifact construction test**

Add to `internal/packageartifact/artifact_test.go`:

```go
func TestBuildCapturedRequiresSupportedSourceAPIVersion(t *testing.T) {
	for _, sourceAPIVersion := range []string{"", "64.0", "67.1"} {
		t.Run(sourceAPIVersion, func(t *testing.T) {
			_, err := BuildCaptured(BuildCapturedOptions{
				Namespace:        "pkg",
				SourceAPIVersion: sourceAPIVersion,
			})
			if err == nil {
				t.Fatalf("BuildCaptured accepted sourceApiVersion %q", sourceAPIVersion)
			}
		})
	}
}

func TestBuildCapturedNormalizesSupportedSourceAPIVersion(t *testing.T) {
	artifact, err := BuildCaptured(BuildCapturedOptions{
		Namespace:        "pkg",
		SourceAPIVersion: "v67.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.SourceAPIVersion != "67.0" {
		t.Fatalf("sourceApiVersion = %q, want 67.0", artifact.SourceAPIVersion)
	}
}
```

- [ ] **Step 2: Run the artifact test and prove RED**

```bash
go test ./internal/packageartifact -run 'TestBuildCaptured(Requires|Normalizes)SupportedSourceAPIVersion' -count=1
```

Expected: FAIL because `BuildCaptured` trims but accepts missing and unsupported versions.

- [ ] **Step 3: Implement required version resolution**

Import `internal/apexversion` in `internal/packageartifact/artifact.go`, add:

```go
func requiredSourceAPIVersion(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("sourceApiVersion is required")
	}
	return apexversion.ResolveSource(raw)
}
```

Resolve it before constructing a captured artifact:

```go
sourceAPIVersion, err := requiredSourceAPIVersion(opts.SourceAPIVersion)
if err != nil {
	return Artifact{}, err
}
```

Set `Artifact.SourceAPIVersion` to `sourceAPIVersion`.

- [ ] **Step 4: Run the construction tests and prove GREEN**

```bash
go test ./internal/packageartifact -run 'TestBuildCaptured(Requires|Normalizes)SupportedSourceAPIVersion' -count=1
```

Expected: PASS.

- [ ] **Step 5: Write the failing current-schema validation test**

```go
func TestValidateRequiresSupportedSourceAPIVersionForCurrentSchema(t *testing.T) {
	for _, test := range []struct {
		name    string
		version string
		want    string
	}{
		{name: "missing", want: "sourceApiVersion is required"},
		{name: "unsupported", version: "64.0", want: "unsupported source API version"},
	} {
		t.Run(test.name, func(t *testing.T) {
			issues := Validate(Artifact{
				SchemaVersion:    CurrentSchemaVersion,
				Namespace:        "pkg",
				SourceHash:       "abc",
				SourceAPIVersion: test.version,
			})
			if len(issues) != 1 || !strings.Contains(issues[0], test.want) {
				t.Fatalf("issues = %#v, want %q", issues, test.want)
			}
		})
	}
}
```

- [ ] **Step 6: Run validation test and prove RED**

```bash
go test ./internal/packageartifact -run TestValidateRequiresSupportedSourceAPIVersionForCurrentSchema -count=1
```

Expected: FAIL with no version issue.

- [ ] **Step 7: Validate schema-2 provenance without inventing schema-1 history**

In `Validate`, after computing the effective schema version:

```go
if schemaVersion >= 2 {
	if _, err := requiredSourceAPIVersion(artifact.SourceAPIVersion); err != nil {
		issues = append(issues, err.Error())
	}
}
```

Keep `TestValidateTreatsOmittedArtifactSchemaVersionAsVersionOne` unchanged. Schema 1 remains readable but receives no invented API version.

- [ ] **Step 8: Add project-loader RED cases**

In `internal/project/project_test.go`, add `TestLoadReportsManagedPackageArtifactSourceAPIVersion` with these schema-2 artifacts:

```json
{"schemaVersion":2,"namespace":"pkg","version":"1.0","sourceHash":"abc"}
```

```json
{"schemaVersion":2,"namespace":"pkg","version":"1.0","sourceHash":"abc","sourceApiVersion":"64.0"}
```

Each load must return one dependency with `Status == "load_error"`, diagnostic code `dependency_load_error`, and a message naming the version problem.

- [ ] **Step 9: Prove project-loader RED**

```bash
go test ./internal/project -run TestLoadReportsManagedPackageArtifactSourceAPIVersion -count=1
```

Expected: FAIL because metadata does not read or validate the field.

- [ ] **Step 10: Validate metadata at the project boundary**

Add `SourceAPIVersion string \`json:"sourceApiVersion"\`` to `managedPackageArtifactMetadata`. Import `internal/apexversion`. In `validateManagedPackageArtifactMetadata`:

```go
if metadata.SchemaVersion >= 2 {
	if strings.TrimSpace(metadata.SourceAPIVersion) == "" {
		issues = append(issues, "sourceApiVersion is required")
	} else if _, err := apexversion.ResolveSource(metadata.SourceAPIVersion); err != nil {
		issues = append(issues, err.Error())
	}
}
```

- [ ] **Step 11: Prove project-loader GREEN**

```bash
go test ./internal/project -run 'TestLoadReportsManagedPackageArtifact(SourceAPIVersion|SchemaVersion|MissingVersion)' -count=1
```

Expected: PASS.

- [ ] **Step 12: Write the failing symbol-provenance assertion**

Update `TestBuildLoadsCapturedManagedPackageArtifactMetadata` to build with `SourceAPIVersion: "67.0"`, then assert:

```go
if got := idx.Types[0].EffectiveAPIVersion; got != "67.0" {
	t.Fatalf("effectiveApiVersion = %q, want 67.0", got)
}
```

- [ ] **Step 13: Prove symbol RED**

```bash
go test ./internal/typesys -run TestBuildLoadsCapturedManagedPackageArtifactMetadata -count=1
```

Expected: FAIL with an empty effective API version.

- [ ] **Step 14: Propagate the exact artifact version**

Change the conversion signature:

```go
func typeSymbolFromArtifact(namespace, version, sourceAPIVersion string, typ packageartifact.ApexType) TypeSymbol
```

Call it with `artifact.SourceAPIVersion` and set:

```go
EffectiveAPIVersion: sourceAPIVersion,
```

Before ingesting types in `appendArtifactDependency`, call `packageartifact.Validate`. On issues, append one `dependency_load_error` diagnostic and a `DependencyInfo` with `Status: "load_error"`, then return. Do not default an absent legacy version.

- [ ] **Step 15: Run focused GREEN verification**

```bash
go test ./internal/packageartifact ./internal/project ./internal/typesys -count=1
```

Expected: PASS.

- [ ] **Step 16: Commit the isolated product fix**

```bash
git add internal/packageartifact/artifact.go internal/packageartifact/artifact_test.go internal/project/project.go internal/project/project_test.go internal/typesys/symbols.go internal/typesys/symbols_test.go
git commit -m "validate package artifact API versions"
```

## Task 2: Prove runner lifecycle semantics end to end

**Files:**
- Modify: `internal/apextest/runner_test.go`

- [ ] **Step 1: Add a runner mock-isolation acceptance test**

Create two test methods in one temporary Apex project. The first calls `Test.setMock(HttpCalloutMock.class, new Mock())` and performs a callout. The second performs the same callout without installing a mock and must fail with the supported no-mock diagnostic. Assert one pass, one failure, and that reversing method order produces the same outcomes.

- [ ] **Step 2: Run the mock-isolation test**

```bash
go test ./internal/apextest -run TestRunnerDoesNotLeakHTTPMocksAcrossMethods -count=1
```

Expected: PASS if the existing full-clone journal is sufficient. If it fails, keep the test as RED and fix only the shared runner reset path.

- [ ] **Step 3: Add paired `SeeAllData` acceptance**

Use the supported gated ConnectApi path already covered at VM level. One class omits `SeeAllData` and must fail; one uses `@isTest(SeeAllData=true)` and must pass. Assert result statuses through `Run`, not discovery metadata.

- [ ] **Step 4: Run the `SeeAllData` acceptance**

```bash
go test ./internal/apextest -run TestRunnerAppliesSeeAllDataToExecution -count=1
```

Expected: PASS. Do not infer general access to arbitrary org data from this local gated contract.

- [ ] **Step 5: Add legacy `testMethod` execution acceptance**

Use this Apex source:

```apex
private class LegacyStyleTest {
    static testMethod void legacyRuns() {
        System.assertEquals(4, 2 + 2);
    }
}
```

Assert discovery and execution report one passed method named `legacyRuns`.

- [ ] **Step 6: Run runner acceptance**

```bash
go test ./internal/apextest -run 'TestRunner(DoesNotLeakHTTPMocksAcrossMethods|AppliesSeeAllDataToExecution|RunsLegacyTestMethod)' -count=1
go test ./internal/apextest -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit test-only evidence**

```bash
git add internal/apextest/runner_test.go
git commit -m "test Apex runner lifecycle contracts"
```

## Task 3: Make declared package dependency resolution deterministic

**Files:**
- Modify: `internal/project/project.go`
- Modify: `internal/project/project_test.go`

- [ ] **Step 1: Write ambiguous-sibling RED**

Create a consumer plus two sibling SFDX projects that both declare package `SharedPkg`. Load the consumer and assert no dependency is selected and one diagnostic has code `dependency_ambiguous` with both sorted candidate roots.

- [ ] **Step 2: Prove ambiguous-sibling RED**

```bash
go test ./internal/project -run TestLoadRejectsAmbiguousLocalSFDXPackageDependency -count=1
```

Expected: FAIL because the current resolver returns the first match.

- [ ] **Step 3: Collect matches before selecting**

Change `findLocalSFDXPackageDependencyRoot` to collect every unique declaring candidate. Return the only match, no match, or a sorted ambiguity error. Convert ambiguity into one `DependencyDiagnostic` with `Status: "ambiguous"` and `Code: "dependency_ambiguous"`.

- [ ] **Step 4: Write missing-declared-dependency RED**

Create an SFDX project whose package directory declares a dependency that has no explicit configured source/artifact and no matching local project. Assert one `dependency_missing` diagnostic instead of silent omission.

- [ ] **Step 5: Implement explicit missing diagnostics**

When a declared SFDX dependency has no configured or discovered source, append:

```go
DependencyDiagnostic{
	Namespace: packageName,
	Status:    "missing",
	Code:      "dependency_missing",
	Message:   "declared SFDX package dependency has no configured source or artifact",
}
```

- [ ] **Step 6: Run focused project verification**

```bash
go test ./internal/project -run 'TestLoad(RejectsAmbiguous|ReportsMissing)LocalSFDXPackageDependency' -count=1
go test ./internal/project -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit deterministic resolution**

```bash
git add internal/project/project.go internal/project/project_test.go
git commit -m "fail closed on ambiguous package dependencies"
```

## Task 4: Feed a hash-pinned describe snapshot into local execution

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/project/project.go`
- Modify: `internal/schema/schema.go`
- Modify: `internal/schema/schema_test.go`
- Modify: `internal/gladecli/test_command_selectors_test.go`
- Modify: `site/docs-src/reference/config.md`
- Modify: `site/docs-src/reference/cli.md`

- [ ] **Step 1: Write configuration RED**

Add a config fixture:

```yaml
project:
  schemaSnapshot: .glade/schema/org.json
  schemaSnapshotSHA256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

Assert `LoadFile` resolves `schemaSnapshot` relative to `glade.yml` and preserves the lowercase 64-hex digest.

- [ ] **Step 2: Prove config RED**

```bash
go test ./internal/config -run TestLoadSchemaSnapshotBinding -count=1
```

Expected: FAIL because the keys are ignored.

- [ ] **Step 3: Add only the two required config fields**

Add to `ProjectConfig` and `project.Project`:

```go
SchemaSnapshot       string `json:"schemaSnapshot,omitempty"`
SchemaSnapshotSHA256 string `json:"schemaSnapshotSha256,omitempty"`
```

Parse `project.schemaSnapshot` and `project.schemaSnapshotSHA256`. Resolve only the path relative to `glade.yml`; normalize the digest to lowercase and reject non-64-hex input.

- [ ] **Step 4: Write snapshot hash and precedence RED tests**

In `internal/schema/schema_test.go`, create a snapshot containing `Account.Org_Only__c` and local metadata containing `Account.Local_Only__c` plus a changed local label. Assert:

- wrong hash returns an error naming the configured and actual SHA-256;
- matching hash loads both fields;
- local metadata wins for overlapping object/field metadata;
- neither config field without the other is accepted.

- [ ] **Step 5: Prove schema RED**

```bash
go test ./internal/schema -run TestLoadProjectUsesPinnedSchemaSnapshot -count=1
```

Expected: FAIL because `LoadProject` ignores the snapshot.

- [ ] **Step 6: Implement snapshot loading at the shared schema boundary**

Add one private helper in `internal/schema/schema.go` that reads `p.SchemaSnapshot`, verifies SHA-256 with `crypto/sha256`, unmarshals `Schema`, and returns its objects. Seed `byName` from the snapshot before loading project metadata. Reuse `mergeObjectMetadata` so project metadata remains authoritative.

- [ ] **Step 7: Add CLI behavior acceptance**

In `internal/gladecli/test_command_selectors_test.go`, import a describe catalog to an output file, compute its SHA-256, write the two config keys, then prove both commands can resolve an org-only object and field referenced by Apex:

```bash
glade check --project <root> --json
glade test --project <root> --json --no-progress
```

- [ ] **Step 8: Run schema and CLI GREEN verification**

```bash
go test ./internal/config ./internal/schema -count=1
go test ./internal/gladecli -run TestSchemaImportDescribeFeedsPinnedCheckAndTest -count=1
```

Expected: PASS.

- [ ] **Step 9: Document the exact operator flow**

```bash
glade schema import describe --input reports/org-describe.json --output .glade/schema/org.json
shasum -a 256 .glade/schema/org.json
```

Show the two `glade.yml` keys. State that the code-intel cache remains advisory; only the pinned snapshot is a runtime input.

- [ ] **Step 10: Commit pinned schema execution**

```bash
git add internal/config/config.go internal/config/config_test.go internal/project/project.go internal/schema/schema.go internal/schema/schema_test.go internal/gladecli/test_command_selectors_test.go site/docs-src/reference/config.md site/docs-src/reference/cli.md
git commit -m "load pinned org schema for local Apex"
```

## Task 5: Add the composed release-binary gate in Glade Tools

**Files:**
- Create in a clean Glade Tools worktree: `scripts/release-local-apex-check.sh`
- Create: `scripts/release_local_apex_check_test.go`
- Modify: `scripts/release-check.sh`
- Modify: `README.md`

- [ ] **Step 1: Bind a clean Glade Tools base**

Do not use the conflicted `/Users/matt/Dev/glade-tools` checkout. Compare `git ls-remote origin refs/heads/main` with local `origin/main`, create an isolated worktree, and record its exact SHA in the release binder.

- [ ] **Step 2: Write the fake-binary RED test**

The test creates a fake `glade` executable that records arguments and emits JSON. Run the new script and assert it invokes `check` and `test --json --no-progress` exactly once for:

```text
enterprise-composed=2
org-like-runner=2
files-email=2
flow=1
resources-labels=2
```

Assert the script fails if any report count differs or any command exits nonzero.

- [ ] **Step 3: Prove script RED**

```bash
go test ./scripts -run TestReleaseLocalApexCheck -count=1
```

Expected: FAIL because the script does not exist.

- [ ] **Step 4: Implement the shell gate**

The script accepts exactly two arguments: the Glade executable and Glade source root. It uses `mktemp -d`, cleans it with a trap, executes the ten commands, parses JSON with the standard Python 3 library, and writes one `release-local-apex-summary.json` containing the exact binary SHA-256, Glade commit, Tools commit, five fixture counts, and aggregate `passed: 9`.

- [ ] **Step 5: Prove script GREEN**

```bash
go test ./scripts -run TestReleaseLocalApexCheck -count=1
```

Expected: PASS.

- [ ] **Step 6: Preserve normal Tools release-check behavior**

In `scripts/release-check.sh`, run the composed gate only when both `GLADE_RELEASE_BIN` and `GLADE_SOURCE_ROOT` are set; reject exactly one being set. This preserves the maintenance release check while allowing the final product candidate to produce a bound receipt.

- [ ] **Step 7: Run focused and full Tools verification**

```bash
go test ./scripts -count=1
GLADE_RELEASE_BIN=<candidate-bin> GLADE_SOURCE_ROOT=<candidate-source> scripts/release-check.sh
```

Expected: PASS and an exact `9/9` receipt.

- [ ] **Step 8: Commit the Tools gate**

```bash
git add scripts/release-local-apex-check.sh scripts/release_local_apex_check_test.go scripts/release-check.sh README.md
git commit -m "gate releases on composed local Apex projects"
```

## Task 6: Make local-test debugging documentation truthful

**Files:**
- Modify: `site/docs-src/help/debug-apex-vscode.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Write the docs contract RED**

Assert the page distinguishes post-run snapshot inspection from a live breakpoint session and does not promise that `glade test --debug` pauses inside the executing test.

- [ ] **Step 2: Prove docs RED**

```bash
npm test --prefix site -- --test-name-pattern='local test debug'
```

Expected: FAIL against the current live-breakpoint wording.

- [ ] **Step 3: Apply the smallest truthful correction**

Describe `Debug Local Test` as running the selected test and opening its result snapshot. Point users who need live statement breakpoints to the supported `glade debug`/DAP workflow. Do not build a second test runner without a separately approved live-debug acceptance requirement.

- [ ] **Step 4: Prove docs GREEN**

```bash
npm test --prefix site -- --test-name-pattern='local test debug'
```

Expected: PASS.

- [ ] **Step 5: Commit the docs correction**

```bash
git add site/docs-src/help/debug-apex-vscode.md site/tests/theme.test.mjs
git commit -m "clarify local Apex test debugging"
```

## Task 7: Assemble one successor and prove the release candidate

**Files:**
- Modify: `docs/RELEASE_NOTES.md`
- Do not modify campaign databases, ledgers, receipts, or evidence roots from this branch.

- [ ] **Step 1: Rebind the campaign outcome**

Record the final accepted campaign candidate SHA, exact Tools SHA, accepted/rejected/infrastructure counts, receipt hashes, and zero-open-cleanup result. Stop if any authority remains provisional.

- [ ] **Step 2: Review every isolated commit**

For each product commit, show the RED and GREEN command outputs. Drop any speculative change that lacks a failing behavior witness or release requirement.

- [ ] **Step 3: Create one release successor**

Merge only the approved product commits, the exact Tools binding, and a nonempty versioned `docs/RELEASE_NOTES.md` section. Build from a clean worktree. Record the new Glade SHA; do not inherit prior Salesforce credit.

- [ ] **Step 4: Run focused product verification**

```bash
go test ./internal/packageartifact ./internal/project ./internal/typesys ./internal/config ./internal/schema ./internal/apextest ./internal/gladecli -count=1
```

Expected: PASS.

- [ ] **Step 5: Run the full local release gate**

```bash
npm ci --prefix site
npm ci --prefix third_party/lwc
scripts/release-check.sh
```

Expected: PASS with validated site, Go-lane, distribution, doctor, and runtime-smoke artifacts.

- [ ] **Step 6: Run composed and real-project acceptance**

Run the Tools composed gate against the built candidate binary and require `9/9`. Run `check` and full `test --json --no-progress` on one explicitly authorized real SFDX project. Preserve commands, exit codes, report hashes, binary hash, and source SHAs outside the source tree.

- [ ] **Step 7: Obtain exact-SHA remote authorities**

Push the successor, wait for terminal-green `Required CI`, and run Salesforce Correctness for exactly the successor Glade SHA and reviewed Tools SHA. A green control plane or older receipt is not credit.

- [ ] **Step 8: Tag and publish**

```bash
git tag -a vX.Y.Z -m $'Release vX.Y.Z\n\nGlade-Tools-SHA: <lowercase-40-hex-tools-sha>'
git push <remote> vX.Y.Z
```

Verify four platform archives, checksums, provenance, and CycloneDX attestations through the existing Release workflow.

- [ ] **Step 9: Verify published artifacts**

Run fresh install, pinned install, `glade version`, `glade doctor`, the composed `9/9` matrix against a downloaded archive, and an update from the prior stable release. Compare downloaded bytes to published SHA-256 values before updating mutable channel pointers.

- [ ] **Step 10: Publish the bounded claim**

Release notes name the checked source API window, exact local test profile, composed-project receipt, real-project receipt, and hosted/non-parity boundaries. Do not state literal identity with every Salesforce-hosted behavior.

## Plan self-review

- Spec coverage: candidate isolation, local execution, test runner, schema, packages, composed fixtures, release binary, exact-SHA authorities, publishing, and bounded claims are each assigned to a task.
- Placeholder scan: the plan contains no deferred implementation placeholder. Candidate-specific values are resolved only at the exact-authority steps where they exist.
- Type consistency: `SourceAPIVersion`, `EffectiveAPIVersion`, `SchemaSnapshot`, and `SchemaSnapshotSHA256` use the same names across construction, validation, loading, and tests.
- YAGNI: no new scanner, dashboard, queue, database, hosted emulator, generalized Flow rewrite, or broad SOQL/regex expansion.
- Authority: no task mutates the active campaign. All merged product changes force one fresh successor and proof cycle.
- Testing: every behavior change starts with a focused failing test and records RED before GREEN.
- Release: the existing release system remains authoritative; the only new gate is the missing composed-project execution bridge.
