# Salesforce Surface Proof Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reach a visible and auditable 100% Salesforce surface-proof result by closing the remaining local contracts, proving every runtime-required surface against Salesforce or an explicit non-parity authority, and keeping operator action, machine health, delivery, cleanup, and exact-candidate state visible throughout.

**Architecture:** Reuse Glade Tools' existing campaign, local-proof, candidate-authority, Dev Hub authority, oracle bundle, scratch-org lifecycle, shard, reconciliation, failure-preservation, report, and cleanup machinery. Add only three missing pieces: encrypted multi-machine Dev Hub login, a live static dashboard derived from existing receipts, and an exact full-runtime-surface scope/index so the oracle flow can cover all runtime obligations rather than only private-corpus-used rows.

**Tech Stack:** Go 1.26, Glade, Glade Tools, Salesforce CLI, existing JSON receipts, `age`, POSIX shell, Python 3 standard library, SSH, Git/GitHub, and static Markdown/JSON/HTML.

---

## Authority and starting state

This plan starts from the merged repositories and must be rebased onto current `origin/main` before each task:

- Glade: `028f1d4b540c71aa0dcac369fa1b83a3591dc770`
- Glade Tools: `a1c60225889aa79cc11dcc02aeae784e194019b9`
- Inventory: 197,730 / 197,730 accounted, zero unassigned
- Required proof checkpoints: 18,427 / 26,651 complete, 69.1%
- Compile shape: 1,994 / 1,994
- Runtime shape: 8,217 / 8,219
- Local behavior: 8,216 / 8,219
- Current-candidate Salesforce adjudication: 0 / 8,219
- Controlled private corpus: 8 / 8 Salesforce-valid projects execute
- Eligible private tests: 22,067 / 22,068; the sole failure is a reviewed private-source test defect
- Local fixture evidence: 10,213 / 10,213 required rows

The current status snapshot is `/Users/matt/Dev/glade-evidence/salesforce-adoption/STATUS.md`. It is not a live authority. Its claim that all three machines are connected is stale: the Mac's `glade-dev-hub` is connected, while Casper and Razor currently return `NamedOrgNotFoundError`.

### Exact five local residuals

1. `apex:System.String.join(List<Object>,String)` — missing shape
2. `apex:System.String.join(Set<Object>,String)` — missing shape
3. `apex:applauncher.AccountSettingsController.getExtraFields(String)` — missing behavior
4. `apex:applauncher.AppMenu.setAppVisibility(Id,Boolean)` — missing behavior
5. `apex:applauncher.EmployeeLoginLinkController.getEmployeeLoginUrl(String)` — missing behavior

The three AppLauncher rows already have exact local UnsupportedFeature fixtures and explicit API-67 compile-oracle exclusion text. They are Salesforce-internal hosted surfaces, not honest deterministic local mocks. Do not fabricate successful behavior to increase the percentage. Add exact hosted-deferred policy exceptions, retain the negative fixtures, and regenerate the denominator. The expected post-closeout runtime set is 8,216 rows; regenerated artifacts are authoritative.

## What 100% means

The dashboard displays two separate Salesforce numbers:

- **Proof completion:** every required checkpoint has a terminal, current-candidate disposition.
- **Salesforce match rate:** runtime rows whose exact contract matched Salesforce.

A terminal Salesforce disposition is exactly one of:

- `matched`: exact current-candidate local and Salesforce contract passed;
- `explicit-nonparity`: an independently authorized exclusion, current-API rejection, or hosted-only boundary;
- `open`: not yet run;
- `product-mismatch`: Salesforce result is valid and Glade differs;
- `inconclusive`: infrastructure, authentication, quota, or cleanup prevented a trustworthy result.

Only `matched` contributes to the match rate. `matched` and authorized `explicit-nonparity` contribute to proof completion. The other three do not.

The program is done only when:

- every inventory row is accounted, unique, and assigned;
- every required shape has positive proof or an exact negative contract;
- every runtime-required row has local shape and behavior proof;
- every runtime-required row is `matched` or authorized `explicit-nonparity` on one frozen candidate;
- every current private project is classified and every Salesforce-valid project runs on that candidate;
- release validation and CI pass;
- no scratch org, temporary branch, or campaign worktree remains;
- final status, receipts, and encrypted backup are sealed.

Reclassification changes the denominator only through a reviewed policy/source PR followed by full profile regeneration. Never change the denominator inside the status renderer.

## Deliberate non-goals

- No Dev Hub signup or creation automation. Matt provides Dev Hubs manually.
- No Vault, SOPS, Temporal, Airflow, Grafana, database, queue service, or custom web server.
- No plaintext credential file, shared private key, or credential in logs/chat.
- No surface-by-surface branch or PR.
- No source checkout inside an evidence root.
- No historical Salesforce credit across a changed Glade binary.
- No fake status PR when a wave produces only sealed proof receipts.

## File and PR map

| PR | Repository | Purpose |
| --- | --- | --- |
| 1 | Glade Tools | Operator identity and encrypted Dev Hub login |
| 2 | Glade Tools | Worker health and live static dashboard |
| 3 | Glade Tools | Exact all-runtime scope and full local-proof planning |
| 4 | Glade Tools | Generic oracle scope binding and cumulative wave index |
| 5 | Glade | Close two `String.join` shapes |
| 6 | Glade Tools | Reclassify three exact AppLauncher hosted boundaries |
| 7+ | Glade or Glade Tools | Root-cause corrections found by discovery and counted waves |

PRs 1 and 5 may run in parallel because they touch different repositories. PR 2 follows PR 1. PR 3 may run while the Glade product PR is in CI. PR 4 follows PR 3. PR 6 must merge before generating the final all-runtime scope.

## Operating rules

Use a clean worktree from current `origin/main`. Do not reuse `/Users/matt/Dev/glade` or `/Users/matt/Dev/glade-tools` when either is dirty or behind.

Run immediately after creating every worktree:

```bash
git config user.name mattsimonis
git config user.email 720686+mattsimonis@users.noreply.github.com
test "$(git config user.name)" = mattsimonis
test "$(git config user.email)" = 720686+mattsimonis@users.noreply.github.com
```

Before every push:

```bash
git log --format='%an%x09%ae' origin/main..HEAD | sort -u
```

Expected output:

```text
mattsimonis	720686+mattsimonis@users.noreply.github.com
```

Delivery rules:

- TDD RED before product or trust-boundary implementation.
- One repository and one root cause per PR.
- Do not stack product code on an unmerged product base.
- Merge only after focused review and required CI pass.
- Delete the merged worktree and branch immediately.
- A four-hour block ends with a reviewable PR, merged PR, or sealed oracle milestone with a dashboard delta.
- A pure Salesforce execution wave does not require a no-op PR.

---

### Task 1: Add the author guard and encrypted Dev Hub login

**Repository:** Glade Tools

**Files:**

- Create: `scripts/assert-branch-author.sh`
- Create: `scripts/assert-branch-author.test.sh`
- Create: `scripts/corpus-assurance/dev-hub-auth.sh`
- Create: `scripts/corpus-assurance/dev-hub-auth.test.sh`
- Modify: `docs/SALESFORCE_ADOPTION_WORKFLOW.md`

- [ ] **Step 1: Write the author-guard RED test**

Create a temporary repository. A `mattsimonis <720686+mattsimonis@users.noreply.github.com>` commit must pass. A second commit by `Other User <other@example.invalid>` must fail with:

```text
unexpected branch author: Other User <other@example.invalid>
```

- [ ] **Step 2: Verify RED**

```bash
bash scripts/assert-branch-author.test.sh
```

Expected: nonzero because the guard does not exist.

- [ ] **Step 3: Implement the minimal guard**

Accept only `--base`, `--name`, and `--email`. Reject an empty branch range. Check every `git log --format='%an%x09%ae' "$base"..HEAD` row. Print no commit message or diff content.

- [ ] **Step 4: Write the Dev Hub auth RED test**

Use fake `age`, `sf`, and `git` executables. Cover these exact contracts:

- `init-host` creates a mode-0600 identity only when absent and prints only its public recipient;
- `put` encrypts the URL from `sf org auth show-sfdx-auth-url --json` to every recipient;
- `login` pipes decryption into `sf org login sfdx-url --sfdx-url-stdin --alias glade-dev-hub --set-default-dev-hub --json`;
- stdout, stderr, shell traces, and files outside encrypted output never contain `force://`;
- missing recipients, duplicate encrypted aliases, decryption failure, and Salesforce login failure return nonzero.

- [ ] **Step 5: Verify RED**

```bash
bash scripts/corpus-assurance/dev-hub-auth.test.sh
```

Expected: nonzero because the helper does not exist.

- [ ] **Step 6: Implement the auth helper**

Public interface:

```text
dev-hub-auth.sh init-host --identity ABSOLUTE_PATH
dev-hub-auth.sh put --store ABSOLUTE_DIR --alias NAME --source-alias NAME --sf-bin ABSOLUTE_SF
dev-hub-auth.sh login --store ABSOLUTE_DIR --alias NAME --identity ABSOLUTE_PATH --sf-bin ABSOLUTE_SF
dev-hub-auth.sh verify --store ABSOLUTE_DIR --alias NAME --identity ABSOLUTE_PATH --sf-bin ABSOLUTE_SF
```

Use `set -euo pipefail`, `set +x`, `umask 077`, `age`, `git pull --ff-only`, and stdin. The store contains only public recipients and ciphertext:

```text
recipients.txt
devhubs/glade-dev-hub.sfdx-auth-url.age
```

Each machine's private identity stays at `~/.config/glade-proof-auth/identity.txt` and is never synchronized.

- [ ] **Step 7: Document the one operator action**

Use `/Users/matt/Dev/glade-proof-auth`, backed by a private bare Git remote on Casper. The SMB backup contains only an encrypted Git bundle at `/Volumes/Photos/glade-bak/glade-proof-auth.bundle`.

Matt's one credential command is:

```bash
scripts/corpus-assurance/dev-hub-auth.sh put \
  --store /Users/matt/Dev/glade-proof-auth \
  --alias glade-dev-hub \
  --source-alias glade-dev-hub \
  --sf-bin /usr/local/bin/sf
```

- [ ] **Step 8: Verify and commit**

```bash
bash scripts/assert-branch-author.test.sh
bash scripts/corpus-assurance/dev-hub-auth.test.sh
git diff --check
git add scripts/assert-branch-author.sh scripts/assert-branch-author.test.sh \
  scripts/corpus-assurance/dev-hub-auth.sh \
  scripts/corpus-assurance/dev-hub-auth.test.sh \
  docs/SALESFORCE_ADOPTION_WORKFLOW.md
git commit -m "Add durable Dev Hub operator login"
```

**Acceptance:** Matt adds a connected Dev Hub once; Mac, Casper, and Razor authenticate noninteractively without plaintext secrets or shared private keys.

---

### Task 2: Add worker health and a live static dashboard

**Repository:** Glade Tools

**Files:**

- Create: `scripts/corpus-assurance/worker-health.py`
- Create: `scripts/corpus-assurance/worker-health_test.py`
- Create: `scripts/render-salesforce-dashboard.py`
- Create: `scripts/render_salesforce_dashboard_test.py`
- Create: `scripts/corpus-assurance/watch-salesforce-status.sh`
- Create: `scripts/corpus-assurance/watch-salesforce-status.test.sh`
- Modify: `scripts/render-salesforce-completeness.sh`
- Modify: `scripts/render-salesforce-completeness.test.sh`
- Modify: `docs/SALESFORCE_ADOPTION_WORKFLOW.md`

- [ ] **Step 1: Write worker-health RED tests**

Use Python `unittest` and fake SSH. Require one normalized row per machine:

```json
{
  "name": "casper",
  "host": "matt@casper.local",
  "reachable": true,
  "devHub": {
    "connected": true,
    "alias": "glade-dev-hub",
    "orgId": "00D000000000001",
    "username": "worker@example.invalid",
    "activeScratchOrgsRemaining": 3,
    "dailyScratchOrgsRemaining": 6
  },
  "diskFreeBytes": 107374182400,
  "run": {
    "id": "surface-wave-01-shard-0",
    "phase": "salesforce-run",
    "heartbeatAt": "2026-08-20T20:00:00Z"
  }
}
```

Cover unreachable SSH, missing alias, malformed Salesforce JSON, mismatched Org ID, low disk, stale heartbeat, and missing run marker. Output must never include tokens, auth URLs, cookies, or environment variables.

- [ ] **Step 2: Verify RED**

```bash
python3 -m unittest scripts/corpus-assurance/worker-health_test.py
```

- [ ] **Step 3: Implement worker health with the standard library**

Public CLI:

```text
worker-health.py --host NAME=SSH_TARGET --alias glade-dev-hub --expected-org-id ORG_ID --output FILE
```

Run one bounded SSH command per host. Probe `sf org display --json`, `sf limits api display --json`, `df -k`, and an optional mode-0600 run marker. Use a 30-second timeout. Write a complete unhealthy row instead of dropping a machine.

- [ ] **Step 4: Extend completeness renderer tests**

Add optional inputs:

```text
--salesforce-index SURFACE_ORACLE_INDEX.json
--campaign-state CAMPAIGN_STATE.json
--worker-health WORKER_HEALTH.json
--delivery DELIVERY.json
--json-output STATUS.json
```

Require separate adjudicated, matched, explicit non-parity, mismatch, inconclusive, and open counts. Require exact candidate/Tools commits, active phase/time, machine health/quota/disk, PR/CI state, cleanup state, and one action with `summary`, `reason`, `action`, and `clearsWhen`.

- [ ] **Step 5: Implement JSON output without changing the denominator**

Keep `render-salesforce-completeness.sh` as the metric calculator. Use `jq -n`; do not add a Go command. Missing Salesforce index means `not-started`, not current proof. Fail closed on candidate, scope, profile, ledger, or count mismatch.

Action priority is credential/connectivity, quota/cleanup, product mismatch, inconclusive retry, PR review/CI, then next wave.

- [ ] **Step 6: Implement the static HTML renderer**

`render-salesforce-dashboard.py --status STATUS.json --output STATUS.html` uses only `json`, `html`, `argparse`, and `pathlib`. Render completion, tiers, candidate, pipeline, machines, outcomes, PR/CI, action, and cleanup. Use `<meta http-equiv="refresh" content="30">`. Add no JavaScript framework or server.

- [ ] **Step 7: Add the bounded watch loop**

Refresh health, Markdown, JSON, and HTML atomically every 30 seconds while a campaign is active. Accept `--once` for tests. Exit when the campaign is `closed` or `blocked`.

- [ ] **Step 8: Verify and commit**

```bash
python3 -m unittest scripts/corpus-assurance/worker-health_test.py
bash scripts/render-salesforce-completeness.test.sh
python3 -m unittest scripts/render_salesforce_dashboard_test.py
bash scripts/corpus-assurance/watch-salesforce-status.test.sh
git diff --check
git add scripts docs/SALESFORCE_ADOPTION_WORKFLOW.md
git commit -m "Show live Salesforce campaign status"
```

**Acceptance:** one page answers what is complete, what is running, which machine owns it, what failed, what PR is active, and whether Matt must act.

---

### Task 3: Derive an exact all-runtime scope and reuse local proof

**Repository:** Glade Tools

**Files:**

- Create: `internal/corpusassurance/surface_scope.go`
- Create: `internal/corpusassurance/surface_scope_test.go`
- Modify: `internal/corpusassurance/localproof_plan.go`
- Create: `internal/corpusassurance/localproof_plan_test.go`
- Modify: `internal/toolcli/corpus_assurance_command.go`
- Modify: `internal/toolcli/corpus_assurance_command_test.go`
- Modify: `docs/SALESFORCE_ADOPTION_WORKFLOW.md`

- [ ] **Step 1: Write the all-runtime scope RED test**

Build a small source profile containing compile, deterministic, local-runtime, and hosted rows. Require a create-only `SURFACE_ORACLE_SCOPE.json` containing exactly the deterministic and local-runtime rows, sorted by exact `surfaceId`, with these bindings:

```json
{
  "schemaVersion": 1,
  "kind": "all-runtime",
  "sourceProfileSha256": "...",
  "ledgerSha256": "...",
  "policySha256": "...",
  "total": 2,
  "byDisposition": {
    "deterministic-mock-required": 1,
    "local-runtime-required": 1
  },
  "rows": []
}
```

Reject duplicate/blank IDs, unknown dispositions, caller-supplied row lists, hash drift, unsorted rows, and output overwrite.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/corpusassurance -run '^TestBuildSurfaceOracleScope' -count=1
```

- [ ] **Step 3: Implement the exact derived scope**

Add `BuildSurfaceOracleScope(sourceProfilePath, ledgerPath, policyPath, outputPath)`. Read each input once, strict-decode JSON, derive rows only from the source profile, and bind the exact bytes. Do not accept a SurfaceID option or selection file.

Add the CLI:

```text
glade-tools corpus assurance surface-scope \
  --source-profile SOURCE_PROFILE.json \
  --ledger LEDGER.json \
  --policy SUPPORT_POLICY.json \
  --output SURFACE_ORACLE_SCOPE.json
```

- [ ] **Step 4: Refactor local-proof selection behind one exact-set helper**

Extract the existing fixture discovery and selection body behind an internal helper that accepts an already-derived `map[surfaceId]disposition`. Keep `BuildLocalProofPlan` behavior byte-compatible for private corpus inputs.

Add `BuildSurfaceLocalProofPlan`, which obtains its exact set from `SURFACE_ORACLE_SCOPE.json`, then writes the existing `LocalProofProfile`, `LocalProofUsage`, `LocalProofDecision`, and `LocalProofFixtureManifest` formats. Do not create a second fixture runner.

- [ ] **Step 5: Add adversarial local-proof tests**

Prove:

- private-corpus planning remains byte-identical;
- all-runtime planning contains every and only scope row requiring a fixture;
- a caller-narrowed profile, usage, decision, or fixture manifest is rejected by `RunLocalProof`;
- a scope/profile/ledger/policy hash mismatch is rejected;
- hosted and compile-only rows cannot enter the all-runtime local-proof plan;
- one fixture may own several rows, but every required row has exactly one selected owner.

- [ ] **Step 6: Add the CLI mode**

Extend `local-proof-plan` with mutually exclusive `--sealed-usage` and `--surface-scope` modes. The all-runtime invocation is:

```text
glade-tools corpus assurance local-proof-plan \
  --surface-scope SURFACE_ORACLE_SCOPE.json \
  --source-profile SOURCE_PROFILE.json \
  --ledger LEDGER.json \
  --policy SUPPORT_POLICY.json \
  --fixture-root docs/fixtures \
  --profile-output LOCAL_PROFILE.json \
  --usage-output LOCAL_USAGE.json \
  --decision-output LOCAL_DECISION.json \
  --manifest-output LOCAL_FIXTURE_MANIFEST.json
```

- [ ] **Step 7: Verify and commit**

```bash
go test ./internal/corpusassurance -run '^(TestBuildSurfaceOracleScope|TestBuild.*LocalProofPlan|TestRunLocalProof)' -count=1
go test ./internal/toolcli -run '^TestCorpusAssurance' -count=1
git diff --check
git add internal/corpusassurance internal/toolcli docs/SALESFORCE_ADOPTION_WORKFLOW.md
git commit -m "Plan proof for every runtime surface"
```

**Acceptance:** one derived file names the complete runtime denominator. The existing local-proof runner proves that entire set without private-corpus usage acting as a filter.

---

### Task 4: Bind oracle artifacts to a generic scope and accumulate waves

**Repository:** Glade Tools

**Files:**

- Create: `internal/corpusassurance/surface_oracle.go`
- Create: `internal/corpusassurance/surface_oracle_test.go`
- Modify: `internal/corpusassurance/oracle.go`
- Modify: `internal/corpusassurance/oracle_test.go`
- Modify: `internal/corpusassurance/bundle.go`
- Modify: `internal/corpusassurance/bundle_test.go`
- Modify: `internal/corpusassurance/reconciliation.go`
- Modify: `internal/corpusassurance/salesforce_test.go`
- Modify: `internal/toolcli/corpus_assurance_command.go`
- Modify: `internal/toolcli/corpus_assurance_command_test.go`
- Modify: `docs/SALESFORCE_ADOPTION_WORKFLOW.md`

- [ ] **Step 1: Write generic-scope binding RED tests**

Introduce the value:

```go
type OracleScopeBinding struct {
    Kind   string `json:"kind"`
    SHA256 string `json:"sha256"`
}
```

Allow exactly `private-corpus` and `all-runtime`. Require the exact binding in new assurance profile, directive, plan, exclusion, bundle, reconciliation, and index artifacts. Existing private-corpus artifacts remain readable; newly written artifacts use the generic binding.

Test that swapping a valid private scope for a valid full scope fails even when candidate and row counts match.

- [ ] **Step 2: Write wave-plan RED tests**

`BuildSurfaceWavePlan` must derive the next work from the full scope, the selected fixture manifest, exclusions, and optional predecessor index. It accepts `--max-fixtures`, not SurfaceIDs. Require:

- exact candidate, Tools, scope, profile, local-proof, and fixture-manifest hashes;
- whole-fixture ownership only;
- no row already terminal in the predecessor index;
- two deterministic shards, capped at 16 fixtures each by default;
- stable ordering by namespace, fixture name, and SurfaceID;
- create-only output;
- zero overlap, missing, or duplicate rows.

- [ ] **Step 3: Write cumulative-index RED tests**

Define `SURFACE_ORACLE_INDEX.json` with one row per full-scope SurfaceID and exactly these states: `matched`, `explicit-nonparity`, `open`, `product-mismatch`, `inconclusive`.

Each successor index binds:

- candidate and Tools artifacts;
- `OracleScopeBinding`;
- source profile, ledger, policy, local proof, and fixture manifest;
- optional predecessor path and SHA;
- one exact exclusion authority or Salesforce reconciliation receipt;
- outcome counts and sorted rows.

Reject changed candidate binaries, changed scope, altered predecessor bytes, duplicate outcomes, missing scope IDs, passed rows not present in a retained packet, and failed infrastructure promoted to product mismatch.

- [ ] **Step 4: Implement the minimum shared path**

Factor the current oracle planner around an exact, already-authorized set. Keep fixture selection, Dev Hub authority, org lifecycle, dispatch, run, cleanup, packet retention, and reconciliation unchanged. Add only:

```text
corpus assurance surface-oracle-plan
corpus assurance surface-wave-plan
corpus assurance surface-oracle-index
```

`surface-oracle-index` has two create-only modes:

```text
--initialize --scope ... --exclusion-authority ...
--predecessor ... --reconciliation ... --packet ...
```

A trustworthy failed Salesforce assertion becomes `product-mismatch`. Authentication, quota, dispatch, timeout, cleanup, or packet-integrity failure becomes `inconclusive` and is not counted complete.

One SurfaceID may require more than one oracle action. Mark it `matched` only when every action in its bound plan passed; one trustworthy failed action makes that SurfaceID `product-mismatch`.

- [ ] **Step 5: Prove backwards compatibility and adversarial boundaries**

Run existing private-corpus oracle tests unchanged. Add focused tests for:

- candidate commit same but binary hash changed;
- profile bytes changed with identical rows;
- a forged passed reconciliation without its packet;
- a wave attempting to split a fixture;
- a second wave repeating a terminal row;
- an explicit non-parity row without exclusion authority;
- final scope/index equality with zero open, mismatch, or inconclusive rows.

- [ ] **Step 6: Verify and commit**

```bash
go test ./internal/corpusassurance -run '^(Test.*Oracle|Test.*SurfaceWave|Test.*SurfaceScope|Test.*Reconciliation)' -count=1
go test ./internal/toolcli -run '^TestCorpusAssurance' -count=1
git diff --check
git add internal/corpusassurance internal/toolcli docs/SALESFORCE_ADOPTION_WORKFLOW.md
git commit -m "Accumulate full-surface Salesforce proof"
```

**Acceptance:** the existing Salesforce executor can process the whole runtime set in bounded waves, and one current-candidate index shows exact cumulative outcomes without replaying or trusting historical credit.

---

### Task 5: Close the two exact `String.join` shape gaps

**Repository:** Glade

**Files:**

- Modify: `internal/typesys/standard_symbols.go`
- Modify: `internal/typesys/standard_symbols_test.go`
- Modify: `internal/sema/cb104_stdlib_parity_test.go`
- Verify: `internal/vm/platform_test.go`

- [ ] **Step 1: Write RED shape tests**

Require these exact static methods on `System.String`:

```text
String join(List<Object>, String)
String join(Set<Object>, String)
```

Retain the existing `String join(Object, String)` compatibility entry. Add semantic compilation cases assigning each result to `String`.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/typesys ./internal/sema \
  -run '^(Test.*Standard.*String|TestCB104.*String)' -count=1
```

Expected: the exact overload assertions fail before the symbol change.

- [ ] **Step 3: Add only the two standard-symbol overloads**

Do not change `VM.stringJoin`; it already accepts List, Set, and Iterable. Do not delete the broad Object entry until a separate usage audit proves it unnecessary.

- [ ] **Step 4: Verify shape and runtime behavior**

```bash
go test ./internal/typesys ./internal/sema -count=1
go test ./internal/vm -run '^TestExecPlatformAPIs$' -count=1
go test ./internal/vm -count=1
git diff --check
```

- [ ] **Step 5: Commit and open the product PR**

```bash
git add internal/typesys/standard_symbols.go \
  internal/typesys/standard_symbols_test.go \
  internal/sema/cb104_stdlib_parity_test.go
git commit -m "Declare exact String join overloads"
test "$(git log --format='%an%x09%ae' origin/main..HEAD | sort -u)" = \
  $'mattsimonis\t720686+mattsimonis@users.noreply.github.com'
```

**Acceptance:** a regenerated profile has zero missing-shape rows, and both overloads retain the already-tested local runtime behavior.

---

### Task 6: Close the three AppLauncher rows without inventing behavior

**Repository:** Glade Tools

**Files:**

- Modify: `docs/fixtures/apex-local-support-policy.json`
- Modify: `internal/surfaceledger/applauncher_closeout_evidence_test.go`
- Verify:
  - `docs/fixtures/core-runtime-applauncher-account-settings-extra-fields-unsupported.json`
  - `docs/fixtures/core-runtime-applauncher-app-menu-visibility-unsupported.json`
  - `docs/fixtures/core-runtime-applauncher-employee-login-url-unsupported.json`

- [ ] **Step 1: Write RED exact-policy assertions**

Require the three exact type/member pairs to resolve `hosted-deferred` while an adjacent AppLauncher method remains `deterministic-mock-required`. Require the existing negative fixtures to remain sole owners with `BehaviorUnsupported`, `salesforceEligible:false`, `policy-local-only`, and explicit zero-parity text.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/surfaceledger \
  -run '^TestAppLauncherCloseoutHasExactExecutableOwnership$' -count=1
```

- [ ] **Step 3: Add three member exceptions**

Add only:

```text
AccountSettingsController.getExtraFields
AppMenu.setAppVisibility
EmployeeLoginLinkController.getEmployeeLoginUrl
```

Each reason must say the API-67 clean-org compile oracle rejects the Salesforce-internal surface and the local fixture proves only Glade's explicit rejection. Do not broaden the AppLauncher namespace rule.

- [ ] **Step 4: Verify policy and negative contracts**

```bash
go test ./internal/surfaceledger \
  -run '^(TestAppLauncherCloseoutHasExactExecutableOwnership|TestSupportProfile.*|TestOverlap.*)$' \
  -count=1
for fixture in \
  docs/fixtures/core-runtime-applauncher-account-settings-extra-fields-unsupported.json \
  docs/fixtures/core-runtime-applauncher-app-menu-visibility-unsupported.json \
  docs/fixtures/core-runtime-applauncher-employee-login-url-unsupported.json; do
  go run ./cmd/glade-tools compat validate "$fixture"
  go run ./cmd/glade-tools compat run "$fixture"
done
git diff --check
```

- [ ] **Step 5: Regenerate and inspect the exact delta**

Use the existing surface refresh command with the same docs, org, Glade snapshot, fixture manifest, and policy inputs. The full changed-ID set must be exactly these three rows plus any mechanically inseparable identity companion explicitly reviewed in the delta. Expected profile effect: three runtime obligations become hosted-deferred and the remaining local gap count becomes zero.

- [ ] **Step 6: Commit and open the Tools PR**

```bash
git add docs/fixtures/apex-local-support-policy.json \
  internal/surfaceledger/applauncher_closeout_evidence_test.go
git commit -m "Classify exact hosted AppLauncher methods"
```

**Acceptance:** local shape and behavior completion are 100% on the reviewed post-policy denominator, with all three negative contracts retained and zero Salesforce match credit.

---

### Task 7: Provision one encrypted Dev Hub identity to all workers

**Repository:** Operator state; no product PR

**Files created outside source repositories:**

- `/Users/matt/Dev/glade-proof-auth/recipients.txt`
- `/Users/matt/Dev/glade-proof-auth/devhubs/glade-dev-hub.sfdx-auth-url.age`
- `~/.config/glade-proof-auth/identity.txt` on Mac, Casper, and Razor
- `/Volumes/Photos/glade-bak/glade-proof-auth.bundle`

- [ ] **Step 1: Install the only new operator dependency**

Install `age` on all three hosts. Do not add it to either Go module.

- [ ] **Step 2: Create independent host identities**

Run `dev-hub-auth.sh init-host` locally on each machine. Add only the three public recipients to `recipients.txt`; commit and push the private auth-store repository.

- [ ] **Step 3: Perform Matt's single credential action**

On the Mac, while `glade-dev-hub` is connected, run the `put` command from Task 1. Verify the resulting Git diff contains only ciphertext and recipient changes. Push it.

- [ ] **Step 4: Authenticate workers noninteractively**

On Casper and Razor:

```bash
git -C ~/Dev/glade-proof-auth pull --ff-only
~/Dev/glade-tools/scripts/corpus-assurance/dev-hub-auth.sh login \
  --store ~/Dev/glade-proof-auth \
  --alias glade-dev-hub \
  --identity ~/.config/glade-proof-auth/identity.txt \
  --sf-bin /usr/local/bin/sf
```

- [ ] **Step 5: Verify one common Dev Hub and capacity**

Generate `WORKER_HEALTH.json`. Require all three connected rows to report the same Dev Hub Org ID. Record active and daily scratch-org remaining counts. Never print the auth URL.

- [ ] **Step 6: Write the encrypted backup**

Create a Git bundle of the encrypted auth store at `/Volumes/Photos/glade-bak/glade-proof-auth.bundle`. Verify it with `git bundle verify`. The bundle contains no private age identity.

**Acceptance:** adding or rotating a manually supplied Dev Hub is one Mac command; both workers self-login from the encrypted store; the dashboard reports one shared Org ID and current quota.

---

### Task 8: Freeze one candidate and run the high-risk discovery wave

**Repositories:** Glade and Glade Tools, both clean and detached for proof

**Attempt root:** `/Users/matt/Dev/glade-evidence/salesforce-adoption/<run-id>-surface-proof-discovery01`

- [ ] **Step 1: Wait for Tasks 1–7 to be merged**

Do not build the counted candidate from a branch. Require both repositories at current `origin/main`, clean, with exact sibling module resolution. Run the branch author guard on every merged feature before deleting its worktree.

- [ ] **Step 2: Regenerate the authoritative local state once**

Using exact current docs, org, Glade, fixtures, policy, and corpus usage inputs, regenerate:

```text
GLADE_SNAPSHOT.json
EVIDENCE_SNAPSHOT.json
LEDGER.json
SOURCE_PROFILE.json
SURFACE_ORACLE_SCOPE.json
LOCAL_PROFILE.json
LOCAL_USAGE.json
LOCAL_DECISION.json
LOCAL_FIXTURE_MANIFEST.json
LOCAL_PROOF.json
```

Require:

- 197,730 inventory rows or the newly reviewed exact inventory denominator;
- zero duplicates and zero unassigned;
- zero compile, shape, behavior, or local-fixture gaps;
- exact scope equality to deterministic-mock plus local-runtime profile rows;
- every scope row has exactly one selected local fixture owner;
- all local fixture commands pass on the frozen candidate.

- [ ] **Step 3: Build and bind the candidate once**

Run the existing create-only sequence:

```text
candidate-build -> independent REVIEW PENDING-to-PASS
candidate-authority -> attempt-init -> prepare
release-validate -> dev-hub-authority
```

Use fixed Go 1.26/CGO/`-trimpath` settings. Independently byte-rebuild both binaries. Require clean detached source roots, exact sibling resolution, and an artifact-only attempt root.

- [ ] **Step 4: Generate the first wave without a hand-selected ID list**

Run `surface-wave-plan` with no predecessor index and the default 32-fixture cap. Its deterministic order puts executable local-runtime fixtures before deterministic mocks, then sorts by namespace and fixture name. Produce two shards of at most 16 fixtures.

- [ ] **Step 5: Execute on Casper and Razor in parallel**

Casper owns shard 0. Razor owns shard 1. For each worker:

```text
org-create -> org-preflight -> salesforce-dispatch
-> salesforce-run -> org-cleanup
```

The Mac coordinates only. It does not run a third shard. Health and heartbeat appear on the dashboard throughout.

- [ ] **Step 6: Reconcile before interpreting results**

Copy both lifecycle receipts and executor outputs back. Run `salesforce-reconcile`, verify the retained packet, then render the dashboard. Never inspect an unbound remote log and call it proof.

- [ ] **Step 7: Decide whether the discovery wave can count**

If the wave has zero `product-mismatch`, zero `inconclusive`, and requires no fixture, policy, candidate, or Tools change, initialize the cumulative index and ingest it as wave 1.

If any source or binary must change, preserve the wave as diagnostic evidence, fix the discovered root causes through Task 10, and build a successor candidate. Do not carry its Salesforce credit forward.

**Acceptance:** the full path runs end to end on both workers, cleanup is proven, dashboard status is live, and either wave 1 is counted or every blocker has an exact owner and next action.

---

### Task 9: Execute the remaining fixture-family waves

**Repository changes:** none for a clean wave

**Default capacity:** two workers, 16 fixtures each, two waves per day

- [ ] **Step 1: Preflight every wave**

Require before scratch-org creation:

- candidate, Tools, scope, profile, proof, fixture manifest, exclusion authority, and predecessor index hashes unchanged;
- both workers reachable and bound to the expected Dev Hub Org ID;
- at least two active scratch-org slots and two daily creates available;
- at least 50 GiB free on each worker and 100 GiB on the Mac evidence volume;
- prior remote roots cleaned or preserved with an exact failure receipt;
- no active product or policy PR that could change the candidate.

- [ ] **Step 2: Derive the next wave**

Run `surface-wave-plan --predecessor SURFACE_ORACLE_INDEX.json`. The planner selects the next whole fixtures from `open` rows only. Do not edit its SurfaceID set. Independently assert:

```text
selected rows ∩ terminal rows = empty
shard 0 ∩ shard 1 = empty
selected rows ∪ remaining open rows = prior open rows
```

- [ ] **Step 3: Run both shards concurrently**

Use the existing oracle bundle and remote lifecycle. Start no more than two scratch orgs concurrently. Reserve two daily creates for one retry wave. The dashboard must show each machine's run ID, phase, heartbeat, quota, and last error.

- [ ] **Step 4: Always close the org lifecycle**

On success, require both cleanup receipts before reconciliation. On failure, run `remote-failure-preserve`, mark the wave `inconclusive`, and stop. Never retry into the same create-only root.

- [ ] **Step 5: Append one immutable index successor**

After verified reconciliation, write a successor `SURFACE_ORACLE_INDEX.json`. Verify predecessor bytes, packet bytes, counts, exact candidate, and scope equality. Refresh Markdown, JSON, and HTML status atomically.

- [ ] **Step 6: Publish a daily human checkpoint**

The dashboard and daily summary state only:

```text
Candidate: <Glade SHA> / <Tools SHA>
Proof complete: <terminal>/<runtime total> (<percent>)
Salesforce matched: <matched>/<runtime total> (<percent>)
Explicit non-parity: <count>
Open / mismatch / inconclusive: <counts>
Today: <waves>, <fixtures>, <rows>, <org creates>
Machines: Mac/Casper/Razor health
PR/CI: <active item or none>
User action: <none or exact command>
Next: <wave or root-cause PR>
```

- [ ] **Step 7: Continue until no open row remains**

The present estimate is about 488 selected fixture owners. At 32 fixtures per wave, plan for roughly 16 waves. With one Dev Hub and two two-worker waves per day, expect 6–8 execution days including retry reserve. Recalculate after the discovery wave; the dashboard uses actual counts, never this estimate.

**Acceptance:** every clean wave produces a verified index delta and visible percentage movement in hours, not days, without a no-op PR.

---

### Task 10: Resolve product mismatches in small root-cause PRs

**Repositories:** Glade for product behavior; Glade Tools for fixture, policy, or proof truth

- [ ] **Step 1: Stop the counted campaign on a trustworthy mismatch**

Set the dashboard action to the exact SurfaceID, local result, Salesforce result, owning fixture, and responsible repository. Do not continue spending scratch-org quota on a candidate known to be wrong.

- [ ] **Step 2: Classify the mismatch before editing**

Choose exactly one:

1. **Product defect:** Salesforce contract is valid and Glade differs. Fix Glade at the shared runtime/type-system root cause.
2. **Fixture or oracle defect:** the local or Salesforce witness is wrong or too weak. Fix Glade Tools without changing product behavior.
3. **Hosted or absent Salesforce surface:** the exact API/version rejects it or behavior requires unavailable hosted state. Add an exact reviewed non-parity authority; do not fabricate a pass.
4. **Infrastructure:** authentication, quota, timeout, transport, or cleanup. Preserve as inconclusive and repair operations only.

- [ ] **Step 3: Use one focused RED and one focused GREEN**

For product work, reproduce the Salesforce-observed boundary in the smallest Glade test. Trace all callers and fix the shared path once. Run the focused package and `go test ./internal/vm -count=1` when VM behavior changes.

For evidence work, bind exact SurfaceIDs, direct source witnesses, sole fixture ownership, local-only metadata, and zero-parity wording as applicable. Regenerate the full raw changed-ID set and reject unrelated rows.

- [ ] **Step 4: Review, merge, and clean promptly**

Open one PR per root cause. Request focused review. Merge when CI passes. Immediately remove the merged worktree and local branch, then prune worktree metadata. Do not stack the next product correction on an unmerged product base.

- [ ] **Step 5: Restart current-candidate proof honestly**

Any Glade binary change invalidates every prior current-candidate Salesforce match. Freeze a successor candidate and start a new index from zero. Diagnostic packets remain retained but grant no current credit. A Tools-only dashboard or auth change that does not alter bound proof inputs does not force a product restart; the authority validator decides this from hashes.

**Acceptance:** each mismatch becomes a small merged correction or an exact authorized non-parity row; no failure is hidden in a broad batch or silently converted to completion.

---

### Task 11: Enforce delivery, cleanup, and disk cadence

**Repositories:** both

- [ ] **Step 1: Keep one active development PR per repository**

Parallelize independent Glade and Tools work, but keep dependency order explicit. Every four hours, require one of: open reviewable PR, merged PR, verified Salesforce wave, or an exact user-action blocker on the dashboard.

- [ ] **Step 2: Gate every push on author identity**

From a Glade Tools worktree, run `scripts/assert-branch-author.sh`. From a Glade worktree, run the equivalent `git log` assertion in Task 5. Every feature commit must be authored by `mattsimonis <720686+mattsimonis@users.noreply.github.com>`. GitHub-generated merge commits may display Matt Simonis; source commits must not.

- [ ] **Step 3: Remove merged worktrees immediately**

For each candidate worktree:

```bash
git status --short
git merge-base --is-ancestor <branch-tip> origin/main
git worktree remove <exact-worktree-path>
git branch -d <exact-merged-branch>
git worktree prune
```

Stop if the worktree is dirty, the tip is unmerged, or the target path is ambiguous. A worktree is temporary execution state, not long-term storage.

- [ ] **Step 4: Keep only useful evidence locally**

Retain the active attempt, the last accepted attempt, and unresolved failure packets locally. After a successor is accepted and its review index verifies, archive older roots as a compressed tar with a SHA-256 manifest to `smb://casper.local/Photos/glade-bak`, verify the archive on the share, then move the local source root to Trash. Never archive source checkouts or caches as proof.

- [ ] **Step 5: Bound caches and free-space risk**

Reuse commit-scoped Go caches during an active candidate. Delete caches only after no worktree or retained rebuild references that candidate. Dashboard warnings:

- Mac evidence volume below 100 GiB free;
- worker below 50 GiB free;
- hard block below 25 GiB anywhere.

Do not retain candidate binaries outside sealed attempts. Do not keep copied evidence bundles per failed retry when the verified review index already references the immutable originals.

- [ ] **Step 6: Keep PR and CI state visible**

The watch loop records open PR number, head SHA, draft/readiness, required checks, failures, and merge state for both repositories. A red check is the dashboard action until assigned or proven unrelated. Merge passing PRs, then refresh the candidate-drift warning.

**Acceptance:** there is no multi-day invisible work, no merged branch left as a worktree, no unexplained author identity, and disk pressure cannot silently halt proof execution.

---

### Task 12: Promote the exact 100% result

**Repositories:** Glade, Glade Tools, and the external evidence root

- [ ] **Step 1: Require exact final arithmetic**

On one frozen candidate:

```text
inventory accounted = inventory total
compile proof = compile total
runtime shape = reviewed runtime total
local behavior = reviewed runtime total
local fixture evidence = all locally required rows
Salesforce matched + explicit non-parity = reviewed runtime total
open = product-mismatch = inconclusive = unassigned = duplicate = 0
```

The scope row set and index row set must be exactly equal. Separately report the match rate; do not call explicit non-parity a Salesforce match.

- [ ] **Step 2: Rerun the private corpus on the final candidate**

Refresh the controlled mirror manifests. Run all Salesforce-valid private projects and eligible tests with `--no-cache`. Preserve source-invalid projects as source defects, not Glade failures. Require zero Glade compile/runtime/infrastructure failures on the valid set.

- [ ] **Step 3: Run final product and Tools validation**

```bash
# Glade
go test ./...
scripts/smoke.sh

# Glade Tools
go test ./internal/corpusassurance ./internal/surfaceledger ./internal/toolcli -count=1
scripts/release-check.sh
```

Use the existing release-validation producer for the authoritative receipt. Do not substitute terminal output for a receipt.

- [ ] **Step 4: Independently review the sealed root**

Verify file-set seal, hashes, candidate byte rebuilds, clean detached roots, exact sibling resolution, local proof, exclusion authority, every Salesforce reconciliation packet, cumulative index lineage, private corpus receipts, cleanup receipts, and final status arithmetic. The reviewer changes only `PENDING` to `PASS` when every gate is exact.

- [ ] **Step 5: Publish human-readable completion**

Update the checked support report and status page with:

- proof completion 100%;
- exact Salesforce match rate;
- exact explicit non-parity count and downloadable reasons;
- final candidate and Tools commits;
- private corpus result;
- completion date and API version;
- zero open user action.

Open one final status PR. Merge after CI and independent review. Do not imply that an explicit hosted/absent boundary behaves like Salesforce locally.

- [ ] **Step 6: Seal, back up, and clean**

Write the final checksum manifest, verify exact file-set equality, copy the sealed evidence archive and encrypted auth-store bundle to `smb://casper.local/Photos/glade-bak`, verify both on the share, remove all campaign scratch orgs, then clean merged branches/worktrees and superseded local attempt roots under Task 11 rules.

**Acceptance:** the public status says exactly how 100% was reached, the proof is reproducible from one candidate and retained packets, Salesforce match and explicit non-parity remain distinct, and no active campaign resources remain.

---

## Milestone gates and visible percentages

| Milestone | Required visible signal | Exit gate |
| --- | --- | --- |
| M0 — Plan | Current baseline shown | 69.1%, 18,427 / 26,651; five exact local residuals; Salesforce current-candidate 0 / 8,219 |
| M1 — Operations | Dashboard live | Mac, Casper, Razor health visible; one exact operator action; PR/CI and disk visible |
| M2 — Local 100% | Compile, runtime shape, local behavior, and local evidence each show 100% | Tasks 5 and 6 merged; regenerated profile has zero local gap |
| M3 — Full scope | Salesforce denominator frozen | Scope equals every deterministic-mock plus local-runtime row; full local proof passes |
| M4 — Discovery | First 32-fixture wave reconciled | Either counted wave 1 or exact mismatch/inconclusive owners shown |
| M5 — Counted campaign | Percentage changes after every wave | Index lineage valid; terminal count rises; no invisible carryover |
| M6 — Final | Proof completion 100% | `matched + explicit-nonparity = runtime total`; open/mismatch/inconclusive all zero |

The renderer computes all percentages from bound artifacts. Milestone prose never overrides arithmetic.

## Immediate execution order

1. Open Tools PR 1 for author guard and encrypted Dev Hub login.
2. In parallel, open Glade PR 5 for the two exact `String.join` shapes.
3. Merge green PRs and clean both worktrees.
4. Open Tools PR 2 for worker health and the static dashboard.
5. Open Tools PR 3 for all-runtime scope and local-proof reuse.
6. Open Tools PR 6 for the three exact AppLauncher hosted exceptions.
7. Open Tools PR 4 for generic scope binding, wave planning, and the cumulative index.
8. Matt runs the single encrypted Dev Hub `put` command; automation logs in Casper and Razor.
9. Freeze one merged candidate pair and run discovery wave 1.
10. Continue counted waves, stopping only for a dashboard-owned mismatch, inconclusive receipt, quota limit, or exact user action.

Do not start another local-evidence fixture wave before M2. The current local fixture denominator is already complete; the remaining local work is two product shape declarations and three honest policy reclassifications.

## User-action contract

The dashboard may ask Matt for action only when automation cannot safely proceed. Every action card contains one copy-pasteable step and its success condition. Expected actions are limited to:

- connect or replace a manually supplied Dev Hub, then run the encrypted `put` command;
- approve or reject an explicit non-parity policy PR;
- increase scratch-org capacity or wait for the daily quota reset;
- resolve an external GitHub or network outage.

It must never say only “authentication failed,” “pipeline blocked,” or “needs review.” It names the machine, alias, failing command, retained receipt, exact action, and the condition that clears the card.

## Plan completion check

Before starting implementation, verify:

```bash
git -C /Users/matt/Dev/glade status --short
git -C /Users/matt/Dev/glade-tools status --short
git -C /Users/matt/Dev/glade rev-parse origin/main
git -C /Users/matt/Dev/glade-tools rev-parse origin/main
```

Create fresh task worktrees from those live `origin/main` commits even if the shared checkouts are dirty. The SHAs in this document record the planning baseline, not permission to build from stale commits.
