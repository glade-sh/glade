# Glade Public Launch Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `executing-plans` for authorized inline execution, or `subagent-driven-development` if the user chooses delegated execution. Steps use checkbox (`- [ ]`) syntax for tracking. This document is a plan, not authorization to implement, publish, change repository settings, or contact anyone.

**Goal:** Make Glade ready for an honest, low-friction early-adopter announcement: a newcomer understands the value, runs a real test, understands the limits, and knows how to give useful feedback.

**Architecture:** Repair the existing product and adoption path. Keep runtime behavior in Glade, maintenance catalogs in Glade Tools, and one canonical quickstart connected from the site, README, and organization profile. Use existing tests, release checks, Cloudflare Pages deployment, and GitHub feedback channels; do not introduce another runtime, documentation system, community platform, or telemetry service.

**Tech Stack:** Go/Apex runtime and CLI; React/TypeScript playground; Vue/VitePress website; Node test runner, Vitest, and Playwright; VS Code extension; GitHub Actions and Cloudflare Pages.

---

## Implementation status — September 4, 2026

Parallel implementation was authorized after this plan was written. Changes are
in isolated local branches; they are not deployed or released. The original
step checkboxes below retain the review checklist, including external gates.
This table distinguishes implemented work from the remaining acceptance work.

| Tasks | Local implementation | Remaining gate |
| --- | --- | --- |
| 0 | Exact product/Tools public-main baselines and isolated adjacent worktrees recorded; original checkout preserved | Select the approved immutable release candidate |
| 1–2 | Source-error/cache guard, meaningful built-in test, and shared packaged runtime smoke implemented with red/green regressions | Final published-archive validation |
| 3 | Confirmed records and bounded history reviewed; exact neutral replacements retained privately | Owner approval, targeted edits, and public read-back; disclosures remain unresolved |
| 4 | Private reporting enabled and public entry routes checked; policies corrected | Monitored fallback contact, product/Tools licensing decisions, authenticated reporter acceptance |
| 5–6 | Catalog descriptions reconciled; complete 288-row explorer and independent contract tests | Regenerate only if the chosen candidate changes |
| 7–11 | Canonical first-test guide, API/setup corrections, replay labels, responsive/accessibility fixes, editor/CI guidance, feedback forms, README/profile drafts implemented | Real editor breakpoint acceptance, hosted advisory/enforcing CI trials, approved profile/settings publication |
| 12 | Dependency updates, complete scanner triage, bounded ZIP/server hardening, packaged JS inventory and VSIX notices implemented | Owner disposition of residual findings/ruleset changes; exact release attestations |
| 13 | Current/historical evidence separation and unreleased notes implemented; local candidate verification underway | Hosted/cross-platform/Salesforce gates, release approval/publication, exact Cloudflare deployment and live smoke |
| 14–15 | Walkthrough shot list, manual pilot sheet, and announcement draft prepared privately | Shipped-version recording, approved invitations, actual pilot results, approved outreach; none sent |

G1–G3 remain open. Local green checks do not establish Salesforce equivalence,
close privacy/licensing decisions, or authorize publication. No release assets,
tags, public issue text, or original checkout history have been replaced.

## 1. Outcome, scope, and baseline

This implements the September 4 public-readiness review. It targets **early-adopter readiness**, not general Salesforce equivalence, GA, a complete security certification, or closure of the private compatibility program.

Reviewed baseline:

| Surface | Reviewed identity |
| --- | --- |
| Product public main / deployed site | `6607048dd07addd4ab4f33a4bdac3c5ae452ac9e` |
| Product v0.2.13 tag | `04c55539045d782ce56e2e5d92d4fb637ba03741` |
| Tools public main | `50e6a7ee983f73c088d4180c84295459087bdd3d` |
| Published first-party plugins | 0.2.12; a different number from the product is not itself a defect |

These are audit inputs, not the future release candidate. Rebind all evidence when execution begins. The existing local product checkout was clean but ahead of and behind public main. Preserve it; do implementation in an isolated checkout based on the exact selected public-main commit.

### What visitors should learn

Use this positioning consistently, subject to the licensing decision in Task 4:

> Glade is an open-source local Apex runtime and developer toolkit. Run supported tests, inspect and debug code, and exercise local SOQL, DML, and triggers without deploying to an org. Keep Salesforce for final validation and hosted platform behavior.

Explain the implementation model immediately after the first example:

> Glade reads your source and metadata from disk and checks and executes supported behavior in its own local runtime. The CLI, editor tools, and local browser interfaces provide different ways to use that runtime. It does not connect to a hidden Salesforce org. It is more than a static analyzer, but it does not reproduce every hosted Salesforce service.

The three primary outcomes are: develop/debug one test; work with local data; automate feedback in an editor, CI, or an AI-assisted workflow. Introduce previews, local APIs, package contracts, and maintenance plugins after the first win.

### Release versus announcement gates

| Gate | Required result | What it permits |
| --- | --- | --- |
| G0 — Safe starting point | Exact inputs, owned work area, decisions assigned | Authorized implementation |
| G1 — Trustworthy candidate | No false Pass; tested sample; privacy remediation; reporting and licensing resolved; corrected support/onboarding claims | Release-candidate review |
| G2 — Verified distribution | Approved immutable release, packaged workflow checks, current evidence, exact live deployment | Outside-developer pilot |
| G3 — Self-service pilot | First-run results recorded; repeat blockers fixed or excluded clearly; maintainers ready for reports | Approved early-adopter announcement |
| G4 — Follow-through | Feedback triaged and repeated blockers addressed | Deliberate expansion of adoption |

A calendar deadline does not override a failed gate. If G2/G3 cannot be met before Dreamforce, share a clearly bounded preview with individually supported evaluators only after approval; do not present it as self-service-ready.

## 2. Decisions and authority

Implementation can proceed on independent tasks once authorized. Do not hold runtime fixes for a licensing discussion, but do not declare the complete ecosystem ready while the decision is unresolved.

| Decision | Owner | Needed before | Recommended direction |
| --- | --- | --- | --- |
| Exact public records to sanitize and private evidence retention | Matt / repository owner | Privacy mutations | Target confirmed disclosures and reviewed related records; preserve useful private evidence; no default deletion/history rewrite |
| Private vulnerability reporting and real fallback contact | Matt / security contact | G1 | Enable GitHub private reporting; name a monitored private fallback |
| Product license normalization; Tools source/binary rights and notices | Copyright owner, with legal advice if needed | G1 / plugin promotion | Make the intended rights explicit; do not infer Tools licensing from Glade |
| Ruleset changes | Repository owner | Applying settings changes | Strengthen the existing rule after checking release/bot workflows |
| Extension version policy | Maintainer | New packaged VSIX | Use meaningful monotonic versions; record the tested product pairing |
| Release version, candidate, merge and publication | Release owner | G2 | New immutable patch; never replace v0.2.13 assets or silently rewrite old evidence |
| Pilot participants, invitation text, channels, and public announcement | Matt | Outreach | Small developer pilot, then one focused early-adopter invitation |
| Community conduct/contact commitments | Maintainer | Publishing those commitments | Short, maintainable guidance with an actual escalation route |

No task authorizes Salesforce org creation, new private-corpus campaigns, credential changes, repository transfers, mass history edits, or public messages. If existing release policy requires fresh external evidence, request the necessary authority and retain the gate until it is satisfied or the claim is explicitly narrowed.

## 3. Work packages and ownership

The work is split into independently reviewable packages below. Each can become its own PR or execution batch; a second set of overlapping design documents is unnecessary. Keep one writer for each repository/artifact. Parallel execution is optional and requires the user's choice; do not run overlapping broad Go suites under memory pressure.

| Package | Tasks | Main responsibility |
| --- | --- | --- |
| Runtime and sample | 1–2 | Product maintainer |
| Public trust | 3–4, 12 | Repository/security/release owners |
| Support truth | 5–6 | Tools catalog owner, then product documentation owner |
| Adoption experience | 7–11 | Product/docs/editor maintainers |
| Distribution and launch | 13–15 | Release owner, then Matt / pilot coordinator |

### Task 0 — Establish the execution baseline

**Files:** Read `AGENTS.md`, `docs/RELEASE_POLICY.md`, `.github/release-authorities.json`; no product edits.

- [ ] Record `git status --short --branch`, `git rev-parse HEAD`, and `git ls-remote origin refs/heads/main` in product and Tools. Record current live `/site-build.json`, release manifest, release tags, and plugin registry separately.
- [ ] Use `using-git-worktrees` to create owned, isolated work areas from the selected public-main SHAs. Keep the product/Tools adjacency required by the Tools build. Do not reset, rebase, or clean the existing checkout.
- [ ] Assign one candidate/evidence writer. Keep private review material outside public repositories; use neutral labels in public artifacts.
- [ ] Run the existing focused baseline checks for the packages being changed. Classify existing failures separately from regressions.
- [ ] Record the decisions above as approved, pending, or declined. A declined change requires a concrete alternative that satisfies the same user-facing promise, not an unchecked box marked complete.

**Exit:** Exact starting SHAs and ownership recorded; existing work preserved; no accidental release/settings/outreach side effects.

### Task 1 — Stop the playground from reporting success on invalid source

**Modify:** `internal/playground/runner.go`.
**Test:** `internal/playground/runner_test.go`.

The loader can return error diagnostics with a nil Go error. `Runner.Run` currently appends those diagnostics but executes anyway. Check their severity before the result-cache return and before anonymous compilation/execution. A guard after the cache lookup is insufficient.

- [ ] Add a regression that injects an error diagnostic through the existing `loadWorkspaceIndex` seam while retaining a valid index. Start with the following warm-cache/persistent-state case; import `reflect` for the state comparison:

```go
func TestRunnerSourceErrorCannotReusePassingCache(t *testing.T) {
    ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
    if err != nil { t.Fatal(err) }
    runner := NewRunner(ws, RunnerOptions{Version: "test"})
    req := RunRequest{
        AnonymousBody: "insert new Account(Name = 'Source error guard');",
        Mode: RunModePersist,
        LimitMode: "permissive",
        UseCache: true,
    }
    first, err := runner.Run(t.Context(), req)
    if err != nil || first.Status != RunStatusPass {
        t.Fatalf("warm run: result=%#v err=%v", first, err)
    }
    before := runner.store.org.Clone()
    originalLoader := runner.loadWorkspaceIndex
    runner.loadWorkspaceIndex = func(root string) (typesys.Index, []diagnostic.Diagnostic, error) {
        index, diagnostics, err := originalLoader(root)
        diagnostics = append(diagnostics, diagnostic.Diagnostic{
            Severity: diagnostic.Error,
            Message: "source error guard probe",
        })
        return index, diagnostics, err
    }
    // Reload diagnostics without erasing the passing result-cache entry.
    runner.runtimeTemplate = nil
    result, err := runner.Run(t.Context(), req)
    if err != nil { t.Fatal(err) }
    if result.Status != RunStatusCompileError || result.CacheHit {
        t.Fatalf("invalid source reused success: %#v", result)
    }
    if len(result.Logs) != 0 || len(result.OrgDiff) != 0 {
        t.Fatalf("invalid source executed: %#v", result)
    }
    if !reflect.DeepEqual(before, runner.store.org) {
        t.Fatal("invalid source changed persistent state")
    }
}
```

- [ ] Run `go test ./internal/playground -run '^TestRunnerSourceErrorCannotReusePassingCache$' -count=1`. Confirm it fails because a passing cached result is returned, not because the test fixture cannot load.
- [ ] Move `RunResult` initialization and structured index-diagnostic attachment before the cache branch. Preserve the existing returned-loader-error handling, then insert this guard before cache lookup:

```go
if (diagnostic.Report{Diagnostics: indexDiagnostics}).HasErrors() {
    result.Status = RunStatusCompileError
    result.CompileMS = millisSince(compileStart)
    result.CompletedAt = time.Now().UTC()
    return result, nil
}
```

Reuse the existing `diagnostic.Report.HasErrors` severity check. Do not attach diagnostics twice. Preserve source locations and messages. Do not change CLI reserved-word rules, downgrade errors, suppress warnings, or cache this failure as a successful run.

- [ ] Extend the existing runner tests with this explicit matrix: real parse error and loader-supplied schema error; scratch and persist modes; cold and warm cache; warning-only source still executes; source correction permits a later successful run. For errors assert compile-error status, retained diagnostic, no logs/DML diff, unchanged persisted state and unchanged last visible org. Reopen the persisted workspace to verify disk state as well as memory.
- [ ] Run `go test ./internal/playground -count=1` and `go test ./internal/gladecli -count=1`. Expected: all relevant tests pass, including existing caching and isolation behavior.
- [ ] Review the focused diff and commit only these owned files as `fix(playground): reject source errors before execution and cache reuse`.

**Exit:** Source errors cannot coexist with Pass or mutate data, including the cached path. This is not a claim that every CLI semantic check is now implemented in the playground.

### Task 2 — Make the recommended sample a real testable first win

**Modify:** `internal/playground/examples.go`, `internal/playground/runner_test.go`, `site/docs-src/guide/examples.md`.
**Extend packaged checks:** `scripts/smoke-runtime.sh`, `scripts/smoke_runtime_test.go`; retain the invocation and integrity guarantees in `scripts/smoke-distribution.sh`, `scripts/smoke_distribution_test.go`. Distribution smoke already delegates runtime checks to the shared runtime script; add behavior checks there rather than duplicate them.

- [ ] Add a failing assertion to the existing example execution tests: a passing result must contain no error-severity diagnostic. Add a check/test regression for the `refinement-service` workspace that requires a named test and a nonzero executed-test count.
- [ ] Run the focused example tests before fixing the sample. Confirm the original reserved parameter and zero-test contract are exposed.
- [ ] Rename `number` to `accountNumber` in the service signature and assignment. Add `force-app/main/default/classes/RefinementServiceTest.cls` to the same built-in template with this meaningful test:

```apex
@IsTest
private class RefinementServiceTest {
  @IsTest
  static void createsAndLabelsFileRow() {
    Account row = RefinementService.createFileRow('Refine 01', 'F-100');
    System.assertNotEquals(null, row.Id);
    System.assertEquals('F-100', row.AccountNumber);
    System.assertEquals('Refine 01 #F-100', FileRow.label(row));
    List<Account> saved = [SELECT Id FROM Account WHERE Id = :row.Id];
    System.assertEquals(1, saved.size());
  }
}
```

- [ ] Use the existing workspace/example-loading path to materialize the sample for tests. Do not add a new example export/scaffolding subsystem. `glade examples run` currently prints an opening command; it is not a project export or test command. Likewise, `playground --once` constructs the handler but does not perform the browser's example-load request. A packaged regression must actually load the example before checking its files and tests.
- [ ] Exercise `glade check`, the named `RefinementServiceTest`, and the supplied anonymous script against the materialized sample. Require clean diagnostics, at least one named passing test, expected label output, and the expected inserted row in the selected local run mode.
- [ ] Run `go test ./internal/playground -run 'TestExample|TestRunner' -count=1` and `go test ./scripts -run 'Test.*Distribution' -count=1`. Extend the packaged smoke invocation to reject total-zero test success rather than only checking exit 0.
- [ ] Recheck every built-in example in the existing grouped execution suite after Task 1. Correct newly exposed sample defects individually; do not weaken the error guard to recover old green tests.
- [ ] Commit the sample and its checks as `fix(examples): ship a checked refinement service test loop`.

**Exit:** The same built-in sample works in the CLI and playground from a packaged candidate, not just from a development binary. Its docs make no unsupported claim about test execution or export.

### Task 3 — Remediate the identified public disclosures

**Targets:** Public product issues #8 and #9, plus the bounded set of related history identified during review. **Private artifact:** a disclosure-review ledger outside public repositories. Never copy the sensitive identifiers into this plan, tests, screenshots, issue titles, or public receipts.

- [ ] Read the identified titles/bodies/comments and enumerate related attachments, PRs, older docs, and release references. Record URLs/IDs and exposure categories privately; review public repository history for the known private identifiers without printing them into public logs.
- [ ] Present the exact affected records, proposed neutral replacements, and private-retention plan to the owner for approval. Identify any credentials separately; identifier cleanup is not credential rotation.
- [ ] After approval, sanitize only the approved targets. Preserve technical meaning using labels such as “private managed-package corpus.” Do not delete records or rewrite Git history by default.
- [ ] Reopen each affected public URL without maintainer-only privileges. Verify titles, bodies, comments, and attachments within the approved scope. Have a second human or authorized reviewer inspect the sanitized public views.
- [ ] Record precisely what was reviewed and what was not. Escalate copies/caches outside project control; do not promise that earlier public exposure has been erased.

**Exit:** Confirmed exposures addressed and the reviewed scope documented. Broad publicity stays blocked on unresolved confirmed private disclosures.

### Task 4 — Make reporting and licensing promises true

**Modify:** Product `SECURITY.md`, `LICENSE`, `docs/SECURITY_TRUST.md`, `site/docs-src/guide/security-trust.md`; Tools `LICENSE` and distribution notice handling after owner decision.
**Inspect/extend:** Product and Tools release-bundle scripts and their existing distribution tests.

- [ ] Confirm the private-reporting setting with `gh api repos/glade-sh/glade/private-vulnerability-reporting`. The reviewed value was false; do not assume it remains unchanged.
- [ ] Obtain approval to enable private vulnerability reporting and name a real monitored fallback. Update the policy with the direct reporting route and actual fallback. Never invent an address or ask security reporters to use public issues.
- [ ] Verify the reporter experience from a non-maintainer account, without submitting a fake vulnerability report. Check reporting arrangements for the advertised Tools plugins too.
- [ ] Ask the copyright owner to confirm the product's intended license text and Tools source/binary licensing. Normalize product licensing to the approved standard text and appropriate notices without changing rights by implication.
- [ ] Add the approved Tools license and include required license/notice material in plugin archives. Audit bundled third-party notices separately from the project's own license. If Tools rights cannot be resolved, stop promoting plugin installation as adoption-ready; record the limitation explicitly.
- [ ] Extend existing archive tests to assert the approved license/notice files are actually packaged. Run `go test ./scripts -count=1` in each affected repo and inspect the extracted candidate archive.
- [ ] After approved publication, recheck GitHub license classification and the downloaded archive contents. A GitHub badge alone is not proof that all distribution obligations are satisfied.
- [ ] Commit repository-file changes separately from settings changes; retain a settings read-back receipt.

**Exit:** Private reporting works, a usable fallback exists, and advertised source/binary rights and notices are explicit. No legal conclusion is inferred from a scanner label.

### Task 5 — Reconcile capability data at its owner

**Tools modify/test:** `internal/capability/stdlib.go`, `internal/capability/capability.go`, `internal/capability/capability_test.go` and the existing stdlib tests.
**Product generated outputs:** `docs/STDLIB_COVERAGE.md`, `docs/KNOWN_GAPS.md`, `docs/COMPATIBILITY_DASHBOARD.md`.

- [ ] Bind the product and Tools candidate SHAs before regeneration. Reconcile `Decimal.divide`: reviewed product output calls it deferred/unsupported, while the reviewed Tools source marks it supported. Inspect the candidate implementation and existing tests/receipts; do not choose the most flattering row or regenerate blindly.
- [ ] Correct String indexing descriptions against the candidate implementation and regression tests. Preserve distinctions among UTF-8 bytes, Unicode code points, and UTF-16 code units; the reviewed substring implementation/test uses UTF-16.
- [ ] Add assertions in the existing capability tests for the corrected descriptions and for gaps output that does not imply broad parity from an internal required-row checklist. Retain stable machine identifiers unless a real consumer requires a versioned change.
- [ ] Replace public-facing completion shorthand with bounded language: “Tracked local capability checks” and “Known limits of the local runtime.” State explicitly when a deterministic harness result does not implement the hosted service.
- [ ] Generate through the existing Tools entry point, from the owned sibling checkout:

```bash
go test ./internal/capability -count=1
go run ./cmd/glade-tools stdlib --output ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools gaps --output ../glade/docs/KNOWN_GAPS.md
go run ./cmd/glade-tools dashboard --output ../glade/docs/COMPATIBILITY_DASHBOARD.md
```

Expected: successful tests and reviewable, evidence-supported output changes. If the execution checkout uses a different sibling layout, bind that actual product path first; never write into the user's unrelated checkout.

- [ ] Diff every status change, not just the summary. Reject new “supported” or parity credit without the corresponding evidence. Keep unsupported/deferred behaviors discoverable even if no required-MVP rows remain open.
- [ ] Commit Tools source/tests and product generated output as coordinated, separately identified changes. Base Glade must not acquire a dependency on Tools internals.

**Exit:** Gaps, ledger, dashboard, and candidate behavior agree; compile/local-runtime/hosted-parity evidence remains distinct.

### Task 6 — Give the support explorer one complete population

**Modify:** `site/scripts/build-editor-support.mjs`, `site/.vitepress/theme/editor/editorSupportTypes.ts`, `site/.vitepress/theme/GladeSupportExplorer.vue`.
**Regenerate:** `site/.vitepress/theme/generated/editorSupport.ts`, `site/docs-src/public/data/editor-support.json`.
**Test:** Create `site/tests/support-catalog.test.mjs`; extend `site/tests/browser/site.spec.ts`.

- [ ] Add a Node contract test that parses every ledger table row using the generator's existing Markdown row pattern and compares it with a new `rows` collection in the generated JSON. Match area, full API label, status, and notes, preserving non-completion-shaped labels. The initial test must fail against the current catalog.
- [ ] Add the following data contract to the existing catalog types; keep autocomplete receivers separate:

```ts
export type SupportLedgerRow = {
  readonly id: string
  readonly area: string
  readonly api: string
  readonly status: EditorSupportStatus
  readonly notes: string
}
// Add to EditorSupportCatalog:
// readonly rows: readonly SupportLedgerRow[]
```

- [ ] Parse the ledger once into these rows. Use `JSON.stringify([area, api])` as the stable row identity and fail generation on a duplicate identity rather than silently discarding conflicting statuses. Preserve all full labels; completion parsing must not decide whether a support row exists. If the ledger itself has a duplicate, resolve that source entry with the catalog owner.
- [ ] Derive `summary` by counting those rows. Keep the existing curated receivers/root completions for the editor demo only. Render explorer entries and text/status filtering from `rows`; retain the existing first-50 display limit but distinguish matched count from displayed count.
- [ ] In `support-catalog.test.mjs`, assert exact row correspondence, unique IDs, and these invariants using Node's built-in assertions:

```js
assert.equal(new Set(catalog.rows.map(row => row.id)).size, catalog.rows.length)
for (const status of Object.keys(catalog.summary)) {
  assert.equal(catalog.summary[status], catalog.rows.filter(row => row.status === status).length)
}
assert.equal(Object.values(catalog.summary).reduce((sum, count) => sum + count, 0), catalog.rows.length)
```

Also test a full label that the old completion parser dropped and any true duplicate fixture. The UI's “with limits” filter must count `partial + stub`; unknown rows must not disappear silently.

- [ ] Add a browser assertion for each status filter, a search for an unsupported full ledger label, and the total-versus-first-50 wording. Verify autocomplete still works without adding curated completions to adoption counts.
- [ ] Run `npm --prefix site run generate:editor-support`, `npm --prefix site test`, and `npm --prefix site run test:browser -- --project=desktop-1440`. Expected: freshness, contract, and rendered filter assertions pass. Repeat all responsive projects at the integration gate.
- [ ] Commit as `fix(site): reconcile support filters with complete ledger rows`.

**Exit:** Every headline/filter reconciles with the same complete collection. No percentage implies coverage of all Salesforce APIs.

### Task 7 — Establish a canonical sample-first quickstart

**Modify:** `site/docs-src/guide/quickstart.md`, `site/docs-src/guide/installation.md`, `site/docs-src/guide/examples.md`, `site/docs-src/guide/modules.md`, `site/docs-src/help/index.md`, `README.md`.
**Reuse:** Existing sample/workspace paths, screenshot tooling, and distribution smoke. `site/scripts/help-project/setup.mjs` remains capture tooling, not an installer prerequisite: it removes its output root and currently creates an API64 fixture.

- [ ] Start the quickstart with two routes: “Try the sample” and “Use my Salesforce DX project.” Use Task 2's corrected bundled sample; no source-repository checkout or screenshot setup script should be required.
- [ ] Verify this existing sample-acquisition route against the fixed packaged candidate. From a new evaluation directory, launch:

```bash
glade playground --data-root .glade/playground --db .glade/playground/org.sqlite --example refinement-service --open
```

Wait for the browser to load Refinement Service and confirm the test class is present. Stop that owned server with Ctrl-C, then run from the same evaluation directory:

```bash
glade check --project .glade/playground/workspaces/default --json
glade test --project .glade/playground/workspaces/default --class RefinementServiceTest --json
```

Expected after Task 2: clean check, named test `createsAndLabelsFileRow`, executed total at least one, and no failed tests. The source-backed workspace path is explicit so the plan does not invent export behavior. Do not publish this as working for unchanged v0.2.13. If the actual packaged browser-load path fails, fix that demonstrated path or use an existing checked public fixture; do not add a second sample-distribution system by default.
- [ ] Structure each route as install → open/create project context → initialize → doctor → focused check/test → interpret the result → feedback/next step. Use the named `RefinementServiceTest` for the sample. For BYO, explain how to choose an existing test class before a full suite.
- [ ] Show the actual passing test name and nonzero total from a packaged candidate. Distinguish successful execution, a supported diagnostic, missing project dependencies, and a setup failure. A zero-test run is not the promised first win.
- [ ] Include a copyable PATH repair for the documented install destination, supported OS/architectures, exact tested version, files/directories created, and scoped cleanup. Derive paths from the installer actually used; do not replace the user's home or existing Glade configuration for a demo.
- [ ] Add this compact API box to quickstart, support-map prerequisites, and LWC setup, reconfirmed against the candidate policy:

| Axis | Reviewed contract to verify before publication |
| --- | --- |
| Checked Apex source | 65.0, 66.0, 67.0 |
| Historical source | Preserve well-formed versions; no implied checked correctness/parity |
| Execute Anonymous | Checked source-version window |
| LWC bundle | Exact supported bundle version required |
| Local HTTP endpoints | Checked 60.0, 65.0, 66.0, 67.0; default 65.0 |

Explain that a clean historical-project check/test does not make all commands eligible. Do not advise upgrading project metadata merely to turn a result green. State native Windows distribution accurately; do not claim WSL support without a recorded test.

- [ ] Walk both routes from clean temporary directories with the release candidate and no Glade source checkout on discovery paths. Record commands, exit codes, parser availability, selected tests, counts, and time to first useful result. Repeat on the supported platform matrix during Task 13.
- [ ] Update existing route/content tests to cover the two routes, named test, API box, expected result, and feedback link. Run `npm --prefix site test` and the packaged sample smoke.
- [ ] Commit as `docs: connect sample and project quickstarts to a verified first test`.

**Exit:** A visitor without an existing project can get a real first result; a developer with a large project is not forced into an unexplained whole-suite run.

### Task 8 — Repair setup, local API examples, and documentation links

**Modify:** `site/docs-src/guide/workflows/lwc-preview.md`, `site/docs-src/guide/lwc-local-shell.md`, `site/docs-src/guide/modules.md`, `site/docs-src/guide/workflows/visualforce-preview.md`, `docs/LWC_LOCAL_SHELL.md`, `docs/LOCAL_TESTING.md`; `internal/playground/web/src/App.tsx`, `internal/playground/web/src/App.test.tsx`; `site/docs-src/public/_redirects`, `site/routes.json`.
**Test:** Existing playground web tests, route tests, and distribution/server smoke tests.

- [ ] Replace release-user prerequisites that require `glade toolchain install` with bundled-toolchain verification and the documented launch command. Keep source-copy installation in source-development instructions only. Run the release layout with `GLADE_ROOT` unset and isolated Glade/data homes; status and launch must work without a source checkout.
- [ ] Change the known Visualforce `v61.0` support-route snippets to the candidate-verified `v65.0` route. Verify the exact documented request and expected payload, not just a server 200. Do not globally reject or replace v60, which is a separately supported endpoint version.
- [ ] Change the playground Docs URL and test expectation to `https://glade.sh/guide/`. Add these legacy aliases and corresponding redirect-only route entries:

```text
/docs /guide/ 301
/docs/ /guide/ 301
```

Route entries must have classification `redirect`, destination `/guide/`, and no Markdown source, following existing `site/routes.json` entries.

- [ ] Run `npm --prefix internal/playground/web test`, `npm --prefix internal/playground/web run build`, and `npm --prefix site run check:routes`. Run affected Go server/distribution tests from the existing harness.
- [ ] Smoke the exact HTTP snippet and Docs button using the packaged candidate. After deployment, verify both old `/docs` URLs redirect and the final guide returns 200.
- [ ] Commit as `fix(docs): align setup and links with packaged runtime behavior`.

**Exit:** Documented local requests work, release users are not sent through source-install prerequisites, and installed older playgrounds have a working documentation destination.

### Task 9 — Clarify the website demo and fix rendered defects

**Modify:** `site/docs-src/guide/workbench.md`, `site/docs-src/guide/workflows.md`, `site/docs-src/guide/quickstart.md`, `site/.vitepress/config.ts`, `site/docs-src/public/js/home.js`, `site/docs-src/public/css/home.css`, `site/docs-src/index.md`.
**Test:** `site/tests/site-ux-contract.test.mjs`, `site/tests/accessibility-contract.test.mjs`, `site/tests/browser/site.spec.ts`.

- [ ] Relabel the website replay with this visible boundary: “Illustrative replay — this page does not execute edited Apex.” Change its action to “Replay example” and route “Execute Apex and SOQL” task links to the real anonymous-Apex/local-playground guide. Preserve the demonstration; do not add a website VM.
- [ ] Name the surfaces consistently: website capability explorer; local Playground; LWC Workbench Console; VS Code extension. Explain them at their entry points without a product-wide command rename.
- [ ] Add a browser regression for the light-theme terminal card's computed foreground/background contrast and a screenshot at 320px/390px. Confirm the present heading/command fail the intended normal-text 4.5:1 contrast target.
- [ ] Give the dark terminal surfaces explicit light foreground colors with selectors that survive VitePress code styling. Start with the existing command foreground `#d5eadb` for the heading and command, scoped under `.home-loop-visual`, and verify the actual composited background before accepting it. Keep both light/dark themes; do not disable light mode or change global code colors.
- [ ] Reproduce the hydration mismatch and unknown highlighting-tag warning with console/page-error capture on a clean direct load and SPA navigation to the explorer. Determine their actual source before modifying code. Do not suppress console output, weaken the error check, or assume the warning is harmless because controls respond.
- [ ] After a minimal cause-specific fix, add the reproducing navigation to the existing browser error gate. Verify direct load, back/forward, and navigation back to the homepage. An unresolved warning needs an explicit disposition; an unresolved hydration error blocks G2.
- [ ] Use a descriptive homepage document/OG title: “Glade — Local Apex Runtime for Salesforce Developers.” Retain the working social card. Verify title, description, canonical, OG metadata, keyboard operation, search, anchors, and responsive layout in the rendered build.
- [ ] Run `npm --prefix site test`, `npm --prefix site run build`, `npm --prefix site run check:built`, and `npm --prefix site run test:browser`. Expected: tests pass across configured projects; no horizontal overflow or unexplained browser errors. Verify the repaired surfaces live in Task 13.
- [ ] Commit the demo wording, contrast fix, and any diagnosed hydration/highlighting fix as separate focused changes.

**Exit:** Visitors cannot confuse replayed output with execution; light-theme content is readable; clean browser navigation meets the existing error gate.

### Task 10 — Make editor and CI instructions truthful and repeatable

**Modify:** `site/docs-src/guide/editor.md`; `site/docs-src/help/run-one-apex-test.md`, `site/docs-src/help/anonymous-apex-scratch.md`, `site/docs-src/help/local-data-environments.md`; `site/docs-src/guide/tester-field-guide.md`, `site/docs-src/guide/ci-artifacts.md`, `site/docs-src/guide/workflows/ci.md`, `site/docs-src/help/ci-setup.md`, `site/scripts/help-project/setup.mjs`; `contrib/vscode-glade/package.json`, `contrib/vscode-glade/test/package.test.js`.
**Read:** `internal/gladecli/editor_command.go`, installer version handling, existing CI examples.

- [ ] Describe `glade editor doctor` accurately: editor/binary and bundled VSIX availability, not proof that the extension is installed. Provide an explicit installed-extension check and a real VS Code smoke path. Do not invent an exit-code guarantee for JSON `ok: false`.
- [ ] Remove the clean-profile/theme requirements from normal user prerequisites. Keep Catppuccin and screenshot-only setup in capture tooling; users can keep their own editor theme/profile.
- [ ] Add source/support metadata to the extension package:

```json
{
  "homepage": "https://glade.sh/guide/editor",
  "repository": { "type": "git", "url": "https://github.com/glade-sh/glade.git", "directory": "contrib/vscode-glade" },
  "bugs": { "url": "https://github.com/glade-sh/glade/issues" }
}
```

Adopt the approved monotonic extension version policy and verify it in the packaged VSIX. State bundled installation explicitly; do not link to nonexistent Marketplace/Open VSX listings or publish there without authorization.

- [ ] Consolidate CI around one canonical advisory example and one enforcing example. Pin the tested product version and plugin versions. The reviewed stable installation pin is `curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=v0.2.13 sh`; update to the approved new tag only after that distribution is verified. The installer environment must be on the shell side of the pipe.
- [ ] Correct the CI template emitted by `site/scripts/help-project/setup.mjs` as well as rendered prose so regeneration does not restore the unpinned example. Run that generator only against a newly created, explicitly owned temporary output directory using `HELP_PROJECT_ROOT`; it deletes its output root. Keep its historical source-version behavior distinct from a checked anonymous-execution demo, and do not present canned capture output as a newly executed receipt.
- [ ] For the advisory example, set `continue-on-error: true` on the Glade assessment/test step and retain artifacts with `if: always()`. Preserve readable exit status/outcome in the report. For the enforcing example, omit `continue-on-error` so failure blocks the job. Do not use `|| true` to erase evidence. Keep checkout/installation failures distinguishable from a Glade compatibility finding.
- [ ] Run both workflow variants against a passing public fixture and an intentionally failing test. Confirm advisory failure preserves artifacts without becoming a required merge gate; enforcing failure fails the job. Confirm required project history/dependencies are present for any affected-test example.
- [ ] Run `npm --prefix contrib/vscode-glade test`, `npm --prefix contrib/vscode-glade run package`, and `npm --prefix site test`. Expected: extension compilation, its existing tests, VSIX packaging, and site contracts pass. Test real VS Code installation, test discovery, one passing/failing test, and one breakpoint with the packaged extension in an isolated profile.
- [ ] Commit editor and CI corrections separately.

**Exit:** Installation guidance matches what doctor checks; documented CI behavior matches its label; bundled editor identity/version and debugging are verified.

### Task 11 — Align public front doors and feedback

**Product modify:** `README.md`, `site/docs-src/index.md`, `site/docs-src/help/index.md`, `site/docs-src/guide/quickstart.md`, `site/docs-src/guide/ai-assisted-apex.md`.
**Product create:** `CONTRIBUTING.md`, `.github/ISSUE_TEMPLATE/bug_report.yml`, `.github/ISSUE_TEMPLATE/workflow_feedback.yml`, `.github/ISSUE_TEMPLATE/config.yml`.
**Tools modify:** `README.md`, `docs/plugin-registry.md`; add concise `CONTRIBUTING.md` linking product rules rather than duplicating a handbook.
**Organization:** `glade-sh/.github` → `profile/README.md`; approved About/topics/pinned-repository settings.

- [ ] Reorder the product README: definition/value; verified first test; three workflows; limitations; feedback; deeper references. Move reserved-word accounting, catalog taxonomy, and exhaustive command lists behind existing references. Do not remove useful detail from its canonical reference.
- [ ] Rewrite Tools' opening for users: compat/performance/orgpackage purposes, verified installation commands, supported product pairing, docs. Remove stale “private repository” claims. Retain internal-import/build adjacency details below the user path.
- [ ] Reuse the canonical quickstart in the organization profile; establish project context before doctor. After approval, fill Tools About/website/topics and pin product and Tools. Do not edit unrelated organization settings.
- [ ] Promote the existing GitHub feedback route on the homepage, Help landing, and quickstart completion. Keep security reports on the separate private route. Validate navigation as a logged-out visitor.
- [ ] Create a bug-report form requesting version, OS/architecture, command, source API version, expected/actual behavior, selected test names/count, and a minimal public reproduction. Include: “Do not paste proprietary source, credentials, private package names, customer records, or unredacted support bundles.” Link the private security route. The workflow-feedback form asks what the user wanted to do, where they got stuck, and what would make it useful.
- [ ] Write a short contributor guide: product/Tools boundary, focused test commands, clean-room/public evidence requirements, private-source handling, how to ask for help, and how to propose a change. Publish community conduct/escalation commitments only after the maintainer approves a real contact and process. A PR template or separate code of conduct is optional, not a launch prerequisite by itself.
- [ ] Add the AI workflow guard: “Do not rewrite valid Salesforce behavior to satisfy a Glade limitation. Report the mismatch and use authorized Salesforce validation for that path.” For refactors preserve a passing baseline; require a new red test when fixing a newly demonstrated bug, not as a ritual for every change.
- [ ] Keep privacy/runtime claims distinct: local test isolation is not an OS sandbox; core local commands not uploading source is not a promise that custom code, imports, plugins, or external AI providers never use the network.
- [ ] Check issue-form YAML and rendered previews, all public links, README install commands, and organization quickstart. Run `npm --prefix site test` in product and `go test ./scripts -count=1` in Tools; expected: public-content and distribution contracts pass. Commit each repository's owned changes separately.

**Exit:** Every public entry point tells the same story and offers the same useful next step. There is one clear feedback destination, not a collection of unstaffed channels.

### Task 12 — Triage security findings and inventory the shipped archive

**Modify as justified:** `.github/workflows/security.yml`, `.github/workflows/release.yml`, `scripts/release-bundle.py`, existing release/SBOM tests, `docs/SECURITY_TRUST.md`, `site/docs-src/guide/security-trust.md`; affected dependency manifests/locks only after triage.
**Settings target:** Existing product ruleset `20896189`, subject to fresh identification and owner approval.

- [ ] Fetch the exact candidate's scanner results. Separate dependency presence, runtime reachability, production versus development use, and source findings. Do not restate the reviewed 22 scanner entries as 22 exploitable runtime vulnerabilities.
- [ ] Rerun the existing govulncheck, production JavaScript audit, extension audit, and gosec lanes using their checked workflow commands. Record tool versions and report scope. Explain that `-no-fail` uploads findings but does not prove none exist.
- [ ] Resolve actionable findings with bounded dependency/source changes and regression checks. Record accepted residuals with finding ID, affected component, reason, owner, and review trigger. Do not force a major upgrade or hide a badge to improve appearance.
- [ ] Label current published SBOM scope as Go-only. Extend the existing release inventory to cover the actual bundled LWC/Babel JavaScript and VSIX dependencies, using resolved packaged dependency data and an existing compatible inventory tool if available. Do not create a generic inventory service or claim lockfile-only development dependencies are all shipped.
- [ ] Extend archive tests to reconcile packaged component families against the SBOM and include required notices. Verify both provenance and SBOM attestations against the downloaded candidate/release asset. Authenticity and completeness are separate checks.
- [ ] If full archive inventory cannot be completed before the pilot, retain explicit Go-only scope and a tracked follow-up; do not make complete-supply-chain-inventory claims. Any severe unresolved security issue requires owner disposition before G2, not automatic acceptance because CI is green.
- [ ] Inspect the current ruleset and release/bot permissions. Propose requiring PRs and preventing force-push/deletion within the existing rule, preserving the approved release path. Apply only after owner approval; read back the setting and verify a normal authorized PR/release workflow remains viable.
- [ ] Run focused dependency/package tests plus `go test ./scripts -count=1` and affected release checks. Commit technical changes separately from settings receipts.

**Exit:** Security evidence is interpretable, residuals are owned, reporting works, and inventory claims match actual inventory scope. No security certification is implied.

### Task 13 — Publish current evidence and verify the exact release/deployment

**Modify:** `site/docs-src/guide/support-map.md`, `docs/RELEASE_NOTES.md`, existing candidate-validation/evidence records and release manifest through their existing generators. Reuse `docs/RELEASE_POLICY.md`, `scripts/release-check.sh`, `scripts/release-distribution-check.sh`, Tools `scripts/release-local-apex-check.sh`, and site postdeploy smoke.

- [ ] Put current capability guidance ahead of historical validation detail. Add a compact evidence table with release, product/Tools SHA, check type, denominator, outcome, and receipt link. Derive “verified with” from actual evidence metadata, not the latest-release manifest alone.
- [ ] Preserve explicitly labeled v0.2.11/v0.2.12 receipts as historical evidence. If current evidence is missing, say so. Do not turn Gate 0, projected/version-upgraded runs, compile readiness, local test success, or deterministic harnesses into Salesforce runtime/parity credit. Explain overlapping assurance sets without adding them together.
- [ ] Add current navigation/errata for the old v0.2.12 validation link missing `docs/`. Do not silently edit an immutable historical receipt or replace a published asset.
- [ ] Write user-first release notes: useful changes; upgrade instructions; fixed first-run defects; known limits; supported OS/API contracts; tested product/plugin/extension versions; exact evidence links. Select the new version with the release owner rather than assuming the next tag is available.
- [ ] Build the authorized exact candidate with the existing parser-enabled path. Record binary SHA, source SHAs, build configuration, and `doctor --json` with `parserOK: true`. Run focused tests, then `scripts/release-check.sh`, affected Tools release checks, and the required hosted CI/Race/Security/Browser lanes. Run broad Go checks serially rather than duplicating them concurrently.
- [ ] Run the existing Tools `scripts/release-local-apex-check.sh` with the absolute candidate binary and product source root. Record its actual denominator. Passing the small local gate does not close public/private corpus or Salesforce evidence requirements; satisfy existing release policy separately with authorized exact-candidate receipts.
- [ ] Test the packaged candidate on all claimed Darwin/Linux architecture targets through actual runners or clearly labeled available coverage. Verify install, PATH, doctor/parser, corrected sample nonzero tests, invalid-source rejection, local data persistence boundary, bundled toolchain launch, editor package, and advertised plugin pairing. Missing runtime coverage must be disclosed and release-policy requirements must still pass; a cross-compiled archive is not a runtime test.
- [ ] Obtain release-owner approval, then publish a new immutable release through the existing release path. Verify downloads, checksums, provenance, inventory attestation/scope, manifest, GitHub latest, plugin registry, and exact assets. If site-only changes move the site SHA beyond the product tag, record both identities explicitly.
- [ ] Deploy through the existing Cloudflare Pages path. Do not replace the Pages project or create a dummy commit to repair an old repository label. Wait for the intended commit in live `/site-build.json`, then run:

```bash
npm --prefix site run smoke:postdeploy -- --base-url https://glade.sh --expected-commit "$GLADE_SITE_CANDIDATE_SHA"
```

`GLADE_SITE_CANDIDATE_SHA` must be bound to the approved full site SHA before invocation. Expected: the smoke reports that exact SHA, with release/manifest alignment, routes/redirects, browser and security-header checks passing.

- [ ] Repeat the user journey from the public download, outside the source checkout. Inspect the actual live sample instructions, support filters, demo labels, light-theme terminal, Docs redirects, search, social metadata, and private reporting route. Record screenshots/receipts without private paths or identifiers.
- [ ] If a release defect remains, stop the announcement, document the known issue, and prepare a new immutable fix release or an approved rollback using the existing policy. Do not overwrite artifacts or borrow a green result from another SHA.

**Exit:** G2 is supported by exact release and live-site evidence. Local green tests or a successful deployment alone do not satisfy it.

### Task 14 — Produce one real walkthrough and run a small pilot

**Modify/reuse:** `site/docs-src/guide/tester-field-guide.md`, `site/docs-src/guide/ai-assisted-apex.md`, existing public sample and demo destination.
**Private artifact:** a small manual pilot-results table outside public repositories. **Media:** publish only approved, reviewed output.

- [ ] Record a roughly 90-second walkthrough using the shipped version and synthetic/public source: show the source, a meaningful failing assertion, the fix, a named nonzero passing test, one breakpoint or local-data result, and the final Salesforce validation boundary. Display the version and test count.
- [ ] Use measured timing only if a performance claim is needed. Record hardware, OS, version, test count, cold/warm conditions, repeated observations, and failures. Do not recycle the homepage's illustrative 412ms as a benchmark. A useful workflow video does not require a comparative speed claim.
- [ ] Review all visible paths, terminal titles, browser tabs, data, logs, and audio for private identifiers before publication. Check captions/readability and provide a text walkthrough so video is not required to start.
- [ ] Obtain approval for 5–10 invitations spanning relevant OS/architectures, source API versions, and project shapes. Start with the corrected sample, then a focused test in the tester's own project if they are authorized to use it. Do not collect proprietary source by default.
- [ ] Use the existing pilot guide and this manual table: neutral tester/project label; OS/architecture; Glade/plugin versions; source API version; install result; doctor result; named tests selected/executed/passed; time to first useful result; first mismatch; whether the next step was clear; issue link/owner.
- [ ] Ask three questions: “What did you think Glade would do?”, “Where did you first get stuck?”, and “Would you use this loop again, and for what?” Record observation separately from interpretation.
- [ ] Triage each failure as setup/docs, unsupported behavior, product regression, performance, or expectation mismatch. Fix repeat first-run blockers before broad promotion; rerun the original tester's reproduction after the fix.
- [ ] Make G3 explicit: the sample journey is independently completed on every platform being promoted; the sample always has nonzero tests; no unresolved false-success/privacy/reporting blocker; no repeated unexplained setup failure; remaining limitations and response owners are visible. Report the actual n/N pilot outcomes rather than inventing a target conversion rate or declaring success from downloads.

**Exit:** Outside-developer evidence confirms that the corrected onboarding and feedback loop work. Any unsupported platform or workflow is excluded clearly from announcement promises.

### Task 15 — Announce deliberately and close the feedback loop

**Artifacts:** One approved announcement draft linking the canonical quickstart, demo, limitations/support map, release, and GitHub feedback destination. No new community service.

- [ ] Draft an early-adopter invitation around local Apex development/debugging, not exhaustive API counts. Include who it is for, a concrete first result, what requires Salesforce, the exact release, and the kind of feedback wanted.
- [ ] Review wording against the release evidence table and pilot results. Do not claim full Salesforce parity, arbitrary historical-version support, an OS sandbox, zero network use, complete inventory before it exists, or performance numbers not measured.
- [ ] Get approval for final copy, channels, recipients, and publication time. Publish/send only after that approval. Dreamforce relevance must not imply Salesforce endorsement or an official event affiliation.
- [ ] Reserve maintainer response capacity for the announcement window. Use the existing issues and labels; agree on a realistic response expectation before stating one publicly.
- [ ] Review incoming reports at an agreed cadence. Use a short manual list of repeated blockers, owners, reproduction status, and next release. Scheduling a recurring review or notifications requires a separate user request.
- [ ] In the first post-pilot/post-announcement review, prioritize repeated setup and correctness problems. Consider Marketplace/Open VSX publication, additional channels, more demo workflows, or broader inventory automation only when actual demand/maintenance needs justify them.

**Exit:** The announcement matches verified capabilities, feedback has an owner, and expansion is driven by observed needs rather than a larger launch checklist.

## 4. Suggested preparation sequence

This is a sequencing target for the September 15–17 Dreamforce window, not a staffing estimate or promise that all work fits eleven days. Confirm capacity at execution kickoff.

| Window | Focus | Stop condition |
| --- | --- | --- |
| September 4–6 | Baseline; Tasks 1–4; begin catalog reconciliation | Do not promote a candidate with false Pass, confirmed unresolved private disclosures, broken private reporting, or unresolved advertised license rights |
| September 6–9 | Tasks 5–12; connect the sample and correct public surfaces | Do not freeze a release while support counts, copied commands, or core demo promises contradict behavior |
| September 9–11 | Task 13 release and live verification; prepare walkthrough | Do not invite a broad self-service pilot on an unverified distribution |
| September 11–14 | Task 14 small pilot and reproduced-blocker fixes | Do not announce as self-service-ready if the pilot still needs unexplained maintainer intervention |
| September 15 onward | Task 15 only if G3 and outreach approval are satisfied | Otherwise continue a bounded, clearly labeled evaluation with approved participants |

If capacity is constrained, defer Marketplace publication, extra community channels, comparative benchmarking, and broad design work first. Complete-archive inventory can have a disclosed follow-up if policy permits; false-success behavior, privacy/reporting, licensing promises, support-count truth, and the first-test path cannot be papered over.

## 5. Audit-to-plan coverage

| Review item | Addressed by | Closure evidence |
| --- | --- | --- |
| 1. Invalid example / false Pass / zero tests | Tasks 1–2, 13 | Cold/warm-cache error tests; no persisted DML; packaged named nonzero test |
| 2. Public private identifiers | Task 3 | Approved target list and public read-back; bounded scope statement |
| 3. Disabled reporting / unnamed fallback | Task 4 | Enabled intended route and verified non-maintainer experience |
| 4. Support counts, duplicates, gaps shorthand | Tasks 5–6 | Complete row-set correspondence and every filter count; corrected generated gaps |
| 5. Product/Tools licensing and archive notices | Task 4 | Owner decision, normalized files, inspected packaged notices |
| 6. Sample-first onboarding, PATH, setup, CI/editor prerequisites | Tasks 7–8, 10 | Clean-install walkthrough; real toolchain/editor checks; passing/failing CI examples |
| 7. API eligibility, v61 snippets, dead Docs link | Tasks 7–8 | Version matrix; exact local requests; shipped UI and legacy redirects |
| 8. Replay mistaken for execution / surface naming | Task 9 | Visible replay label and correct real-workflow links |
| 9. Light-theme contrast / browser warnings | Task 9 | Computed contrast and responsive screenshots; cause-specific console regression |
| 10. Current evidence and String catalog drift | Tasks 5, 13 | Candidate-bound evidence table; source-owned descriptions; historical boundaries |
| 11. Security findings, rules, provenance, inventory scope | Tasks 4, 12–13 | Scoped triage, approved setting read-back, archive/attestation checks |
| 12. Tools/org/README/feedback/contributing/editor/release/search | Tasks 10–13 | Consistent public front doors, forms, metadata, verified notes/links |
| Explain value and how it works | Tasks 7, 11, 14–15 | Shared concise positioning; actual walkthrough; AI/trust limits |
| Pilot and sustained feedback | Tasks 14–15 | Actual first-run outcomes and owned reproduction/response queue |
| Exact inputs and preserved local work | Tasks 0, 13 | Clean owned work area; exact SHA/asset/deployment receipts |

## 6. Completion and handoff

At execution time, review and commit each bounded change after its focused checks; stage exact owned paths rather than `git add .`. Broad tests belong at the integrated candidate gate. Code examples above describe intended changes and have **not** been implemented or executed by writing this plan.

The release closeout must list: changed PRs/SHAs; product/Tools/extension versions; required test and corpus receipts; tested platform matrix; packaged first-test result; security/license/privacy dispositions; deployed site SHA; pilot outcomes; remaining limitations with owners; and the exact approved announcement scope.

**Plan-writing status:** Planning only. No runtime, site, issue, setting, license, release, or outreach changes are completed by this document. All execution checkboxes intentionally remain unchecked.
