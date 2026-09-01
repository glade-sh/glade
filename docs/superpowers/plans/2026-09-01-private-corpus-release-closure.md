# Private Corpus Release Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the prior zero-failure private-corpus acceptance result on a clean candidate from `main`, remove the two confirmed semantic false positives, and produce exact-candidate public, private, local, CI, Race, and Salesforce release evidence.

**Architecture:** Fix both false positives at their shared semantic-analysis decision points in `glade`. Reuse the existing `glade-tools` sealed inventory and replay workflow for private-corpus acceptance; do not add another manifest or treat the broad 19-root discovery scan as release authority. Keep historical declared-version measurements, supported-version simulation, local replay, and Salesforce proof as separate receipts.

**Tech Stack:** Go, `go test`, Glade CLI, Glade Tools corpus assurance, Git/GitHub Actions, JSON receipts.

---

## Execution worktrees

Use these isolated paths. Do not modify the user's dirty checkouts at `/Users/matt/Dev/glade` or `/Users/matt/Dev/glade-tools`.

```bash
glade_root=/Users/matt/.config/superpowers/worktrees/glade/codex-private-corpus-release-plan
tools_root=/Users/matt/.config/superpowers/worktrees/glade-tools/codex-private-corpus-release-closure

git -C "$glade_root" merge-base --is-ancestor origin/main HEAD
git -C /Users/matt/Dev/glade-tools worktree add \
  -b codex/private-corpus-release-closure \
  "$tools_root" origin/main
```

Expected: the Glade branch is based on current `origin/main`; the Glade Tools worktree is clean at current `origin/main`.

Before implementation, commit this plan so both candidate roots can satisfy the clean-worktree authority:

```bash
git -C "$glade_root" add docs/superpowers/plans/2026-09-01-private-corpus-release-closure.md
git -C "$glade_root" commit -m "docs: plan private corpus release closure"
```

## Recovered baseline and acceptance contract

The prior published assurance snapshot is decisive: both sealed private repositories passed every required `glade check` and real `glade test` shard. There is no accepted historical private failure to grandfather into this release.

The remembered "few failures" does match the older broad tag scans, but not the release authority: v0.2.8 reported 18/19 roots exiting 0, v0.2.9 reported 17/19, v0.2.10 reported 8/19, and v0.2.11 reported 14/19 at each project's declared version. Those runs used discovered nested roots and changing historical check coverage, so they explain history but do not define today's accepted denominator.

The newer broad scan is not comparable to that authority:

- It discovered 19 nested SFDX roots instead of replaying the two sealed repository roots.
- It rewrote one project at a time for source-version simulation, leaving sibling dependencies on their declared historical versions.
- Current Glade intentionally supports source API versions 65.0, 66.0, and 67.0 only. A normalized run is not proof of original-version correctness.
- A bare parallel `go test ./...` is not the release command. On clean `main` it can hit Go's default 10-minute package timeout and miss `GLADE_ROOT` in a nonstandard worktree. The fixed release validator uses a 45-minute timeout, bounded `GOMAXPROCS`, and the bound source root.

Release acceptance is therefore:

1. The exact two-repository `IN_SCOPE.json` denominator is preserved.
2. Every sealed repository-level `glade check` exits 0.
3. Every required four-way `glade test` shard exits 0 and reuses the bound semantic cache.
4. The merge replay accepts every expected repository and shard with no waiver or expected-failure list.
5. The public corpus has no project regression from the prior accepted release and no unexplained new error diagnostic.
6. The exact final Glade SHA receives fresh `Required CI` and `Salesforce Correctness` authorities.

## Task 1: Recover and seal the historical comparison

**Evidence only; do not change product code.**

- [ ] Create a fresh private run directory and record the current clean references:

```bash
run_root="$(mktemp -d /tmp/glade-private-release-closure.XXXXXX)"
git -C "$glade_root" rev-parse HEAD
git -C "$tools_root" rev-parse HEAD
```

Expected: Glade starts from the current `origin/main` lineage and Glade Tools starts from its current `origin/main` lineage. Neither worktree has tracked or untracked changes.

- [ ] Recover the prior public statement from commit `89fdc70dfe5e77ec05e909b9de45b417c444da1b`:

```bash
git -C "$glade_root" show \
  89fdc70dfe5e77ec05e909b9de45b417c444da1b:docs/PRIVATE_CORPUS_ASSURANCE.md \
  >"$run_root/prior-private-corpus-assurance.md"
rg -n "passed every required|eight passing records|Frozen scope SHA-256|Receipt SHA-256" \
  "$run_root/prior-private-corpus-assurance.md"
```

Expected: the snapshot states that both repositories passed every required check and real test shard, with eight passing replay records.

- [ ] Locate the immutable prior private receipt by the published receipt SHA-256. Validate the bytes before using it:

```bash
test -f "$prior_receipt"
shasum -a 256 "$prior_receipt"
```

Expected: `bf0a7b7a9fc2b0a7e505677b37c4891acea4f9b1cad8edb0c6ba714e3709517c`.

- [ ] If the raw receipt is unavailable, record that limitation. Use the checked public snapshot only for the zero-failure historical result; do not invent per-project or per-shard details.

- [ ] Write a private comparison ledger under `$run_root`, not in either public repository. It must contain: prior candidate SHA, prior tools SHA, prior inventory hash, prior receipt hash, two repositories, eight passing records, and zero accepted failures.

- [ ] Attach the existing v0.2.8-v0.2.11 broad-scan summaries to the ledger as `historicalDiagnosticOnly`. Preserve each project's declared source version. Keep the API65-normalized comparisons in a separate `upgradeSimulation` section and set original-version correctness credit to false.

No commit.

## Task 2: Fix standard SObject instance fields being classified as static

**Files:**

- Modify: `internal/sema/public_corpus_static_access_test.go`
- Modify: `internal/sema/sema_checks.go`

- [ ] Add a failing regression test to `internal/sema/public_corpus_static_access_test.go` that assigns standard `Document` instance fields:

```go
func TestPublicCorpusAllowsDocumentInstanceFieldAssignmentsRegardlessOfWarmup(t *testing.T) {
	for _, files := range [][]string{
		{"DocumentWriter.cls"},
		{"DocumentWarmup.cls", "DocumentWriter.cls"},
		{"DocumentWriter.cls", "DocumentWarmup.cls"},
	} {
		root := t.TempDir()
		writeSemaFile(t, filepath.Join(root, "DocumentWriter.cls"), `
public class DocumentWriter {
  public static void write(Id folderId) {
    Document value = new Document();
    value.FolderId = folderId;
    value.Body = Blob.valueOf('body');
  }
}
`)
		writeSemaFile(t, filepath.Join(root, "DocumentWarmup.cls"), `
public class DocumentWarmup {
  public static Document read(Document value) { return value; }
}
`)
		result := analyzePublicCorpusFiles(t, root, files...)
		assertNoDiagnosticContaining(t, result, "GLADESEMA027", "static fields cannot be accessed through an instance")
	}
}
```

- [ ] Run the focused RED test:

```bash
go test ./internal/sema -run '^TestPublicCorpusAllowsDocumentInstanceFieldAssignmentsRegardlessOfWarmup$' -count=1
```

Expected before the fix: at least the cold `DocumentWriter.cls` case fails with `GLADESEMA027` for `FolderId` or `Body`.

- [ ] Make the smallest shared fix in the dotted-assignment check in `internal/sema/sema_checks.go`:

```go
if found && hasModifier(field.member.Modifiers, "static") &&
	!hasModifier(field.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
	diagnostics = append(diagnostics, semaFieldAccessDiagnostic(
		typ, member, receiver+"."+fieldName,
		"static fields cannot be accessed through an instance",
		bodyOffset+match[2], bodyOffset+match[5], source,
	))
}
```

This follows the marker exclusion already used by `body_ir.go` and `body_calls.go`. Do not alter the stored standard-object field model and do not special-case `Document`, `FolderId`, or `Body`.

- [ ] Run the focused test and the surrounding static-access suite:

```bash
go test ./internal/sema \
  -run '^(TestPublicCorpusAllowsDocumentInstanceFieldAssignmentsRegardlessOfWarmup|TestPublicCorpusAllowsStaticMethodCallThroughInstance)$' \
  -count=1
```

Expected: PASS.

- [ ] Commit:

```bash
git add internal/sema/public_corpus_static_access_test.go internal/sema/sema_checks.go
git commit -m "fix sema standard object instance assignments"
```

## Task 3: Stop inferring incompatible numeric field types from literal order

**Files:**

- Modify: `internal/sema/sema_test.go`
- Modify: `internal/sema/schema_inference.go`

An exact NPSP repro showed 42 of the 43 new public `GLADESEMA018` diagnostics came from this order-sensitive inference. The one remaining new relationship assignment stays in Task 4 for composed-metadata adjudication.

- [ ] Add a failing regression test near the existing project-referenced schema tests in `internal/sema/sema_test.go`:

```go
func TestAnalyzeProjectReferencedNumericFieldInferenceAcceptsIntegerAndDecimalObservations(t *testing.T) {
	t.Run("inferred field widens numeric literals", func(t *testing.T) {
		root := t.TempDir()
		writeSemaFile(t, filepath.Join(root, "UsesInferredAmount.cls"), `
public class UsesInferredAmount {
  public static void seed() {
    Partial__c first = new Partial__c(Amount__c = 1);
    Partial__c second = new Partial__c(Amount__c = 1.5);
  }
  public static void run(Partial__c value) { value.Amount__c = 2.5; }
}
`)
		index := typesys.Build(project.Project{
			Root: root,
			ApexFiles: []string{filepath.Join(root, "UsesInferredAmount.cls")},
		}, schema.Schema{Objects: []schema.Object{{Name: "Partial__c"}}})
		result := Analyze(index)
		assertNoDiagnosticContaining(t, result, "GLADESEMA018", "Amount__c")
	})

	t.Run("authoritative integer field still rejects decimal", func(t *testing.T) {
		root := t.TempDir()
		writeSemaFile(t, filepath.Join(root, "UsesDeclaredCount.cls"), `
public class UsesDeclaredCount {
  public static void run(Declared__c value) { value.Count__c = 2.5; }
}
`)
		index := typesys.Build(project.Project{
			Root: root,
			ApexFiles: []string{filepath.Join(root, "UsesDeclaredCount.cls")},
		}, schema.Schema{Objects: []schema.Object{{Name: "Declared__c", Fields: []schema.Field{{Name: "Count__c", Type: "Integer"}}}}})
		result := Analyze(index)
		found := false
		for _, diag := range result.Diagnostics {
			found = found || diag.Code == "GLADESEMA018" && containsString(diag.Message, "Count__c")
		}
		if !found {
			t.Fatalf("known invalid assignment was not rejected: %#v", result.Diagnostics)
		}
	})
}
```

- [ ] Run the RED test:

```bash
go test ./internal/sema \
  -run '^TestAnalyzeProjectReferencedNumericFieldInferenceAcceptsIntegerAndDecimalObservations$' \
  -count=1
```

Expected before the fix: the inferred-field subtest reports `GLADESEMA018` because the first integer literal fixes `Amount__c` as `Integer`; the authoritative control also reports `GLADESEMA018` and passes.

- [ ] Make numeric literal inference use the common Salesforce field type in `semaProjectReferencedSchemaFieldTypeFromValue` in `internal/sema/schema_inference.go`:

```go
case decimalLiteralPattern.MatchString(value), intLiteralPattern.MatchString(value):
	return "Number"
```

This keeps useful source-backed inference but maps both integer and decimal observations to Apex `Decimal`, which accepts integer values. Do not change authoritative metadata and do not suppress `GLADESEMA018` in the assignment checker.

- [ ] Run the focused inference tests:

```bash
go test ./internal/sema \
  -run '^(TestAnalyzeProjectReferencedNumericFieldInferenceAcceptsIntegerAndDecimalObservations|TestAnalyzeProjectReferencedLiteralFieldTypesFlowToAssignmentsAndConstructors|TestAnalyzeProjectReferencedDateFactoryFieldTypeFlowsToCalls|TestAnalyzeProjectReferencedFieldsRefineExistingAnyAndObjectFields)$' \
  -count=1
```

Expected: PASS, including the authoritative known-invalid control.

- [ ] Commit:

```bash
git add internal/sema/sema_test.go internal/sema/schema_inference.go
git commit -m "fix sema numeric field inference"
```

## Task 4: Prove the semantic fixes beyond the two reproductions

**Files:**

- Modify: `docs/RELEASE_NOTES.md`

- [ ] Run the complete semantic package without cache:

```bash
go test ./internal/sema -count=1
```

Expected: PASS.

- [ ] Add two narrow release-note bullets: standard SObject instance assignments no longer produce a static-field diagnostic, and project-referenced numeric fields no longer depend on whether an integer or decimal literal was observed first. Do not claim blanket Salesforce parity.

- [ ] Run documentation and repository guards, then commit the notes before producing any candidate evidence:

```bash
go test ./internal/repoguard -count=1
git diff --check
git add docs/RELEASE_NOTES.md
git commit -m "docs: note corpus semantic fixes"
```

Expected: PASS and no private names or paths in the public tree.

- [ ] Run the broad Go suite with the release validator's bounded environment, then smoke checks:

```bash
GLADE_ROOT="$glade_root" GOMAXPROCS=2 \
  go test -timeout 45m -count=1 ./...
scripts/smoke.sh
```

Expected: PASS.

- [ ] Build a parser-capable candidate outside the repository:

```bash
candidate_dist="$(mktemp -d /tmp/glade-candidate.XXXXXX)"
DIST_DIR="$candidate_dist" CGO_ENABLED=1 scripts/build-local.sh
"$candidate_dist/glade" doctor --json
```

Expected: `parserOK` is `true` and the embedded version identifies the exact candidate commit.

- [ ] Re-run the same public and private broad diagnostic scans used during diagnosis. Keep the exact roots, Glade Tools SHA, target source API, and output paths in `$run_root`.

Expected public delta from the prior accepted binary:

- The `Document.FolderId` and `Document.Body` `GLADESEMA027` errors are gone.
- Numeric fields observed first with an integer literal and later with a decimal no longer produce false `GLADESEMA018` errors.
- No previously passing public project becomes failing.
- Every remaining new error is individually tied to Salesforce-invalid source or a documented unsupported boundary. `unclassified` must be zero.

Expected private diagnostic interpretation:

- The 19-root normalized scan remains diagnostic only.
- `dependency_missing` and `dependency_load_error` caused by one-project-at-a-time API simulation do not receive product-failure credit.
- The scan is not allowed to waive a failure in the sealed replay.

- [ ] Write a concise root-cause ledger under `$run_root` with one row per remaining new diagnostic fingerprint: project neutral ID, code, file hash, prior result, current result, classification, evidence, and owner (`product`, `corpus source`, `unsupported version`, or `simulation topology`).

No commit unless this step exposes another product regression. For each additional product regression, repeat Tasks 2-3's pattern: minimal fixture, focused RED test, shared root-cause fix, focused GREEN test, separate commit.

## Task 5: Run the authoritative private-corpus replay

**Reuse unchanged:** `glade-tools/internal/corpusassurance/inventory.go`, `replay.go`, and `release.go`.

Do not add a corpus manifest, a failure allowlist, or a second replay command.

- [ ] Use clean worktrees at the exact proposed Glade and Glade Tools commits. Create a new private `IN_SCOPE.json` containing the same two neutral repository IDs and the current clean private repository commits.

- [ ] Build and seal both binaries with the existing candidate builder:

```bash
candidate_sha="$(git -C "$glade_root" rev-parse HEAD)"
tools_sha="$(git -C "$tools_root" rev-parse HEAD)"
(cd "$tools_root" && go run ./cmd/glade-tools corpus assurance candidate-build \
  --candidate-root "$glade_root" \
  --tools-root "$tools_root" \
  --candidate-ref "$candidate_sha" \
  --tools-ref "$tools_sha" \
  --candidate-output "$run_root/bin/glade" \
  --tools-output "$run_root/bin/glade-tools" \
  --receipt-output "$run_root/candidate-build.json" \
  --review-output "$run_root/candidate-review.txt" \
  --tools-freeze-output "$run_root/tools.freeze")
```

Expected: a create-only schema-v2 receipt, parser-capable candidate, clean source roots, and exact binary hashes.

- [ ] Complete the independent candidate review, then create the candidate authority with `corpus assurance candidate-authority`.

- [ ] Create a fresh assurance attempt and immutable snapshots with the existing `attempt-init`/`attempt` and `prepare` commands. Do not reuse a prior candidate's attempt, snapshots, cleanup authority, or replay receipt.

- [ ] Run `corpus assurance replay` for the `local` and `replay-worker` host manifests. Preserve all raw shard receipts and cleanup evidence.

- [ ] Merge the shards with the existing validator:

```bash
"$run_root/bin/glade-tools" corpus assurance merge-replay \
  --inventory-spec "$run_root/IN_SCOPE.json" \
  --root-manifest "$run_root/inventory/MANIFEST.json" \
  --host-manifest "$run_root/inventory/hosts/local/manifest.json" \
  --host-manifest "$run_root/inventory/hosts/replay-worker/manifest.json" \
  --shard "$run_root/replay-local.json" \
  --shard "$run_root/replay-worker.json" \
  --output "$run_root/replay-merge.json"
```

Expected acceptance:

- two exact repository IDs present once each;
- eight complete repository/shard records when both repositories require the four test shards;
- every check and test receipt has `passed: true`, `exitCode: 0`, and `timedOut: false`;
- every required test has valid semantic-cache evidence and positive disk hits;
- candidate, tools, inventory, manifest, source, execution-tree, and command-spec hashes all bind;
- merge exits 0 with no waiver.

- [ ] If any replay command fails, stop promotion. Reproduce that exact retained command against its sealed snapshot, use `git bisect run` between the last passing assurance candidate and current `main` to find the first bad product commit, and add a focused Glade regression test before fixing it. Infrastructure interruption may be rerun only after its receipt proves no product command completed with a semantic/runtime failure.

No public-repository commit. The run artifacts remain private evidence.

## Task 6: Run exact-candidate release gates

**Files:** no changes. If any release input changes, discard the receipts and restart Task 5.

- [ ] Run fixed release validation from Glade Tools:

```bash
"$run_root/bin/glade-tools" corpus assurance release-validate \
  --attempt "$run_root/attempt.json" \
  --glade-root "$glade_root" \
  --candidate "$run_root/bin/glade" \
  --tools-root "$tools_root" \
  --tools "$run_root/bin/glade-tools" \
  --tools-freeze "$run_root/tools.freeze" \
  --output "$run_root/release-validation.json"
```

Expected: product `go test ./...`, smoke, and Glade Tools release checks all pass from clean exact roots.

- [ ] Run the fresh Salesforce correctness campaign for the same exact Glade and Glade Tools commits. Publish `Salesforce Correctness` only after the private replay, local proof, Salesforce obligations, cleanup, and final receipt all validate.

- [ ] Push the candidate branch and wait for exact-SHA `Required CI`. Rerun the Race workflow on a healthy runner. A runner shutdown or exit 143 is infrastructure evidence, not a product failure, but the release still waits for a completed passing Race run.

- [ ] Verify final authorities:

```bash
candidate_sha="$(git -C "$glade_root" rev-parse HEAD)"
tools_sha="$(git -C "$tools_root" rev-parse HEAD)"
gh api "repos/glade-sh/glade/commits/$candidate_sha/check-runs" \
  --jq '.check_runs[] | [.name,.status,.conclusion,.head_sha,.external_id] | @tsv'
GITHUB_REPOSITORY=glade-sh/glade \
  scripts/verify-salesforce-check.sh "$candidate_sha" "$tools_sha"
```

Expected:

- `Required CI`: completed/success on the exact candidate SHA;
- exactly one trusted `Salesforce Correctness`: completed/success, exact candidate SHA, exact tools SHA, bound receipt digest;
- Race: completed/success on the exact candidate SHA;
- no current-candidate release input changed after these receipts were produced.

## Final verification checklist

- [ ] `go test ./internal/sema -count=1`
- [ ] `GLADE_ROOT="$glade_root" GOMAXPROCS=2 go test -timeout 45m -count=1 ./...`
- [ ] `scripts/smoke.sh`
- [ ] `scripts/release-check.sh`
- [ ] Glade Tools `scripts/release-check.sh` against the exact candidate
- [ ] Public corpus: no project regression; zero unexplained new errors; zero unclassified
- [ ] Private sealed replay: both repositories check-clean; all required test shards pass; merge accepted
- [ ] Fresh exact-SHA `Required CI`
- [ ] Fresh exact-SHA passing Race run
- [ ] Fresh exact-SHA `Salesforce Correctness` bound to the exact tools SHA
- [ ] Release notes state only the behavior proven by these receipts

## Scope deliberately skipped

- No change to `glade-tools corpus check` discovery or one-project upgrade simulation. That command remains a diagnostic scanner; the existing sealed replay already solves release acceptance.
- No restoration of historical source API versions below 65.0. Original-version behavior stays a separate historical measurement.
- No expected-failure list for the private corpus. The prior accepted baseline was zero failures, so the new candidate must return to zero.
