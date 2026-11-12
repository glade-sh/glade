# Development Workflow

How a change moves from a ticket to a release. This is the process document.
[AGENTS.md](../AGENTS.md) holds the rules and the validation ladder,
[AI_SETUP.md](AI_SETUP.md) the per-surface commands, and
[RELEASE_POLICY.md](RELEASE_POLICY.md) with
[DISTRIBUTION_WORKFLOW.md](DISTRIBUTION_WORKFLOW.md) the release mechanics.

Three facts shape everything below:

- Every change reaches `main` as a pull request. The hosted `Required CI`
  check on the pull request is the merge gate.
- For Salesforce-observable compatibility changes, captured Salesforce
  behavior is the oracle; native rows come before the code changes. For
  Glade-specific behavior without a Salesforce counterpart, cite the local
  contract and add a regression test. Do not infer Salesforce behavior.
- Each stage is one command with one printed verdict. Where a step needs
  judgment, this document names the rule.

## 1. The flow

| # | Stage | Who | Output | Done when |
| --- | --- | --- | --- | --- |
| 1 | Ticket | coordinator | scope, acceptance criteria, owned files, change class | a coder can start without asking a question |
| 2 | Branch | coder | a branch from `main` in its own worktree | `git status` is clean and the base is current `main` |
| 3 | Evidence | coder | native rows or local contract, as applicable | compatibility changes have rows at the applicable current supported version and floor; Glade-specific changes cite their local contract |
| 4 | Test first | coder | a failing test committed before the fix | the test fails at the test commit and passes at HEAD |
| 5 | Local checks | coder | focused tests, one package run, repoguard | green, or each failure explained in the PR |
| 6 | Pull request | coder | the PR with the template filled in | every field has content or `n/a: <reason>` |
| 7 | CI | hosted | `Required CI` green | the check passes on the PR merge result for the current head and base |
| 8 | Review | reviewer | one PR review, after CI is green | no open blocking finding, threads resolved |
| 9 | Merge | coordinator | a squash commit on `main` | the PR check is green and the branch is up to date with `main` |
| 10 | Nightly | host cron | real-project suite diff against goldens | `NEW = 0`, or one ticket per NEW |
| 11 | Release | maintainer | tag, archives, notes | the release checklist is green |

Cadence: merges are continuous. The real-project suite runs nightly on `main`
and once on each release candidate. A release is cut when the Unreleased notes
justify it, normally every two to three weeks. The maintainer decides.

## 2. Roles

- Maintainer: scope, release word, and authorizations (orgs, accounts, hosts).
- Coordinator (one session at a time): writes tickets, starts coders, assigns
  reviews, reads the nightly result, cuts releases. Does not code on coder
  branches.
- Coder (one ticket, one branch, one PR): does stages 2 to 6, answers review
  findings, never merges.
- Reviewer (one per PR): reads the diff and the evidence, runs focused tests,
  posts one review.

## 3. Stages in detail

### 3.1 Ticket

A ticket states the user-visible problem, the acceptance criteria as
observable outcomes, the files or packages the coder owns, what is out of
scope, and the change class (section 4). A ticket that needs native evidence
says so and names the cases to capture. The PR links the ticket id.

### 3.2 Branch and worktree

```bash
git fetch origin
git worktree add <path> -b <prefix>/<ticket>-<slug> origin/main
```

`<prefix>` is one of `fix`, `feat`, `test`, `docs`, `perf`, `ci`. `<path>` is a
directory outside the checkout; a sibling directory works. One worktree per
coder. Never two live coders on one file. Do not rebase a branch while a
review or a check is running on it; add commits instead.

### 3.3 Evidence (behavior changes only)

Identify each changed behavior as Salesforce-observable compatibility behavior
or Glade-specific behavior without a Salesforce counterpart. A change spanning
both needs both forms of evidence. For Glade-specific behavior, cite the
applicable documented or intended local contract in the PR and commit, and add
a regression test that exercises it, including relevant acceptance, rejection
and boundary cases. Follow the test-first process in section 3.4. Do not use a
local contract to assert Salesforce compatibility.

For Salesforce-observable compatibility changes, fixtures live at
`internal/<pkg>/testdata/<family>/`, where `<pkg>` is the package whose test
reads them: `apextest` (under `testdata/conformance/`) for
runtime behavior, `sema` for compile-time diagnostics, `visualforce`, `lwc`
and `dml` for their surfaces. A family directory holds `cases.tsv` (or
`cases.json`), the synthetic Apex sources, `org.tsv` and `native-run.md`.

1. Write `cases.tsv`: one row per case with `id`, `api`, `route`, the source,
   and the expected result once captured. Routes are anonymous Apex, an
   `@IsTest` class, the project test runner, the server, and the hosts
   (Visualforce, LWC) that reach the code. Capture at the applicable current
   supported version (68.0 for Apex, 67.0 for LWC and Visualforce) and the
   floor (62.0 Apex, 59.0 LWC and Visualforce). Include 67.0 for Apex when
   relevant to the change. Give each "accepts" case a "rejects" twin
   and a boundary row. Example for a rule that rejects a negative scale: one
   row that accepts scale 2, one that rejects scale -1 with the exception type
   and message, and one boundary row at scale 0.
2. Capture against a fresh scratch org. The ticket names the org
   authorization; the private operations guide carries the setup for internal
   agents. With no org access, go to section 5 item 1 before writing any
   code. Write `org.tsv` and `native-run.md` into the family directory. Scrub
   org ids, aliases and personal paths. Delete the org afterwards.
3. Encode the rows in one data-driven Go test in that package, copying the
   pattern of an existing `*_conformance_test.go` there. The fixture,
   `org.tsv`, `native-run.md` and the test go into the test commit of section
   3.4. The test is the oracle from then on, and CI runs it without
   credentials.
4. Fix only rows the test proves wrong. Cite every row you rely on, one line
   per row, in the commit message and in the PR:
   `NATIVE <behavior> <path>/org.tsv:<line> <route> <api>`. A behavior
   captured at two APIs cites both rows.

A row already in the repository backs a fix without a new capture when its
route and API match the test. If the required native row is missing and cannot
be captured, stop and ask (section 5).

### 3.4 Test first, then fix

```bash
# 1. Commit the applicable evidence and the test, without the fix.
git add <fixture dir> <test files>
git commit -m "test: <behavior> (<native row or local contract>)"
# 2. Show the test fails at that commit (the fix is absent, so this is the
#    "fails on its parent" proof AGENTS.md asks for).
CGO_ENABLED=1 go test ./internal/<pkg> -run '^TestName$'   # expect FAIL
# 3. Commit the fix and show the test passes.
git add <files>
git commit -m "fix: <behavior> (<native row or local contract>)"
CGO_ENABLED=1 go test ./internal/<pkg> -run '^TestName$'   # expect ok
```

For new tooling or a new feature, a test that fails because the code does not
exist yet is valid fail-before evidence. When history cannot be reordered,
prove fail-before in a detached worktree at the fix's parent with only the
test file applied, and record the output in the PR.

### 3.5 Local checks

```bash
CGO_ENABLED=1 go test ./internal/<pkg> -run '^(TestA|TestB)$'   # while iterating: unlimited
CGO_ENABLED=1 go test -timeout=0 ./internal/<pkg>                # once after the last code change in <pkg>
CGO_ENABLED=1 go test ./internal/repoguard                       # once
gofmt -l .                                                        # prints nothing
go vet ./internal/<pkg>
```

A full package run is spent once per package per code change; a new change
in that package allows one more. Exception: never run the whole
`internal/apextest` package locally. Run your own apextest tests with `-run`;
CI runs the 14 shards. `-timeout=0` matters for `internal/vm`,
`internal/gladecli` and `internal/visualforce`, which can exceed Go's default
ten minutes.

When a test fails at HEAD, run the same test on `origin/main`: the same
failure there is pre-existing (say so in the PR and link or open its ticket);
a pass there means the failure is yours.

### 3.6 Pull request

Open the PR against `main` with the template filled in. The PR body is the
report. Title: `<area>: <what changes>`, for example `vm: reject a negative
Decimal scale`. A reviewer must be able to judge the change from the body and
the diff alone. A behavior change is always user-visible: add its line to
`docs/RELEASE_NOTES.md` under Unreleased in the same PR.

Before asking for review, run `bash scripts/pr-check.sh` (add `--repoguard
--vet` for the full local gate) on the finished branch. It applies the
machine-checkable rules from sections 3.4 to 3.6 and 4, including gofmt, ABI
bumps, test-first order, leak, org-id and personal-path scans, the lane
manifest, and an optional `--body FILE` check. Fix every FAIL before asking for
review; a WARN needs its reason in the PR.

### 3.7 CI

`Required CI` runs a fast lane on every PR, including drafts. It builds with CGO,
checks formatting, vet, repository guard, workflow lint, leak scan and scripts
tests, and tests changed Go packages and their direct importers (the fast PR lane, below). The Apex, LWC compile and Visualforce shards of the fast lane plan from the same shared duration-history artifact the full suite uses. A changed path outside
every Go package (for example `lwcruntime/**`, `third_party/lwc/**`, root
`testdata/**`, workflows or root files) selects the full package set; only
`docs/**` and Markdown files outside `testdata` are docs-only and select
nothing. Pushes to `main`, scheduled and manually dispatched runs use the
full suite. A pull request, including an integration train (section 3.12),
runs only the fast lane over the union of its changed packages, so a train PR
also needs a dispatched full run before it merges (section 3.12). Superseded PR runs
are cancelled, so push once and let the run finish; do not close and reopen a
PR to retrigger CI, which cancels the run in flight. For a
`pull_request` run, checkout and tests use GitHub's merge result for the PR
head and base, not the head tree alone. The merge commit has the PR head as its
second parent; identify both the head and tested merge tree when reporting it.
This PR check is distinct from the release authority in section 3.11. When it
is red, read the failing job's log, fix on the branch, and push. Do not re-run a red job hoping for green. When the failing
test is unrelated to your change and passes on `main`, say so in the PR and
open a ticket for it; the coordinator decides on a rerun. The runners carry
Node, Playwright and a browser, so an "environmental" result does not exist
in CI. Locally such a test is "not run", never "passed". `scripts/ci-local-all`
mirrors the full suite when hosted CI is unavailable.

### 3.8 Review

One reviewer per PR, assigned by the coordinator, following appendix B.
Blocking findings: a correctness defect; a regression; a Salesforce-compatible
behavior change without a matching `NATIVE` line; a Glade-specific behavior
change without a cited local contract and regression test; a missing applicable
route twin; a deleted or loosened assertion without appropriate evidence;
third-party names or content; credentials, org ids or personal paths.
Everything else is a comment, not a block. The reviewer may fix a small
finding with a commit on the branch and approve, saying so in the review.
After "changes requested", the coder fixes on the branch without rebasing, and
the reviewer re-reviews only the new commits.

Docs-only PRs, and test or fixture PRs that only add, need no review. CI still
gates them.

### 3.9 Merge

The coordinator squash merges after `Required CI` passes on the PR and review
is done. `main` requires branches to be up to date and there is no merge queue,
so a PR that fell behind `main` is updated and must pass `Required CI` again
before it merges. A train PR also merges only after the full CI run dispatched
on the train branch is green on the exact head (section 3.12). Prefer an integration train (section 3.12) when several PRs
are ready together. The squash
message is the PR title plus applicable `NATIVE` lines or local contract
citations. The branch is deleted on merge.
The coordinator moves the ticket to Done on the project board with the PR link.
If the branch conflicts with `main`, resolve the conflict, rerun the focused
tests and push; do not rewrite history under a running check. Do not merge a
train or `main` into a component PR branch to refresh it; that discards its
green CI result.

### 3.10 Nightly real-project suite

Each night `main` is built and run against pinned real projects: the
synthetic shape projects in this tree and external projects configured
outside this repository. Each project has a golden result file, and the run
prints the diff. A NEW failing test or diagnostic is red: the coordinator
bisects the day's merges along the first-parent chain, reverts or fixes
forward, and opens a ticket with a synthetic repro either way. A golden
changes only in a PR that cites the native row or states the intended
change.

### 3.11 Release

1. Run the nightly and full real-project tier on the exact candidate. The
   release notes record the checks passed; failures are limited to known issues
   listed there, per [RELEASE_POLICY.md](RELEASE_POLICY.md#release-gate).
2. Measure the performance aggregate with matched outcomes and report the scope;
   the notes carry only measured numbers.
3. Write and review the release notes. Run the first project check from
   [INSTALL.md](INSTALL.md) on a real Salesforce DX project before tagging.
4. Before opening or updating the release PR, run `scripts/ci-local-all` on
   the clean candidate tree. On the exact release tree, pass
   `scripts/release-check.sh`, `gofmt -l .`, `go vet -trimpath ./...`,
   `go test -count=1 ./scripts` (in a real checkout with `jq` and Node), and
   `go test ./internal/repoguard`.
5. Check the diff since the last tag for third-party names, personal paths,
   org ids and credentials.
6. After merge and before tagging, verify the exact candidate's main-push
   `Required CI` and trusted `Salesforce Correctness` authorities as specified
   in the release policy, including its limited tree-attestation alternative
   and the bound tools commit. Check the separate
   `Browser`, `Race` and `Security` workflows as required operator checks.
7. Tag and publish per [DISTRIBUTION_WORKFLOW.md](DISTRIBUTION_WORKFLOW.md).
8. Install and run `glade doctor` on one machine from the published archive.

No step repeats on an unchanged tree. A gap found while qualifying a release
is fixed before the tag unless the maintainer decides otherwise.

### 3.12 Integration trains and component branches

Component branches are small, one ticket each. The coder validates a component
branch with build, vet and the focused tests for the change, then pushes it.
The full suite is not run per component. A train's pull request runs the fast
lane over the union of its components. Every train PR also gets a full CI run
before it merges: dispatch the CI workflow on the train branch with
`gh workflow run ci.yml --ref <train branch>`. The train merges only when that
dispatched run is green on the exact head and the train review approves. The
full run on the push to `main` is then confirmation, not the first full test.
New commits on the train restart the dispatched run.

- Train: the coordinator combines several approved PRs into one branch built
  from their heads and opens one PR. It gets one `Required CI` run and one
  review of the merge resolutions only; the component diffs were already
  reviewed.
- CI before review: a reviewer starts on a head only after `Required CI` is
  green on that exact head. A PR enters a train on its own green head. New
  commits restart both.
- Version constants: `SemanticABI`, `PlatformABI` and `testRuntimeCacheABI` are
  not bumped on component branches or in trains. One bump per release is made
  at the cut, in the release PR. Component PRs state which bump class they need.
- Corpus and real-project runs from section 3.10 and 3.11 are reused when the
  product tree is identical to the one already measured.

The fast PR lane (section 3.7) is the only CI a component PR needs before
review.

## 4. Change classes

Every ticket and PR names one class: `docs`, `tests`, `tooling`, `behavior`,
`performance` or `generated`.

| Class | Covers | Needs | Does not need |
| --- | --- | --- | --- |
| `docs` | documentation only | PR, CI | review, rows |
| `tests` | tests or fixtures only | PR, CI; when an existing assertion is deleted or changed: applicable native row or local contract per change and a review | review for pure additions |
| `tooling` | CI, scripts, build, release tooling | PR, CI; the lane and inventory tests when lanes change | rows |
| `behavior` | parser, sema, vm, soql, dml, storage, Visualforce, LWC, server behavior | native rows for compatibility or a local contract for Glade-specific behavior, test first, applicable route twins, review, the ABI bump below | |
| `performance` | speed or memory with identical outcomes | a matched before/after outcome comparison with its scope and the targeted benchmark in the PR; the release measures the aggregate | per-PR project measurement |
| `generated` | generated files, including the standard-object schema ([STANDARD_OBJECT_SCHEMA.md](STANDARD_OBJECT_SCHEMA.md)) | change the input or the generator, regenerate, commit both | hand edits |

ABI bumps, once per release at the cut (section 3.12); a component PR names
the bump class it needs instead of changing the constant:

- `SemanticABI` in `internal/sema/sema.go`: any non-test change under
  `internal/sema`, `typesys`, `apexast`, `apexlang`, `ir`, `project`, `schema`,
  `sobject`, `semanticcache`, `frontendcache` or `codeintel`.
- `PlatformABI` in the same file: any change to a file under `internal/vm` or
  `internal/sema` whose name starts with `platform_`, `builtin_` or
  `generated_platform_`, or ends in `_overlay.go`.
- `testRuntimeCacheABI` in `internal/apextest/startup_cache.go`: any non-test
  change under `internal/vm`, `apextest`, `dml`, `soql`, `sosl`, `storage`,
  `automation`, `startupcache`, `trace` or `apexlog`, and every `SemanticABI`
  bump.
- `dapCacheVersion` in `internal/gladecli/dap_cache.go`: only when the debug
  adapter cache layout changes.

Raise the number by one at the cut. Tests that pin a version or a
per-version counter change in the same release PR, with the reason in the
message.

## 5. Stop and ask

Stop and ask the coordinator, in the PR or the ticket, when:

1. a Salesforce-observable compatibility change has no native row and you
   cannot capture one;
2. the fix needs files outside your owned set, a generated output, or an ABI
   constant that another open PR also changes (look at the open PRs);
3. a test that passes on `main` fails on your branch and an hour of work has
   not explained it;
4. the change would alter a public support claim, a hosted boundary, or an
   excluded area (Slack, DataWeave, or hosted ConnectApi services). ConnectApi
   passive shapes and documented test behavior are supported; see
   [RELEASE_POLICY.md](RELEASE_POLICY.md#salesforce-api-versions).

Everything else: decide from this document and state the decision in the PR.

## 6. For agents

Each stage is one command and one verdict. Report in the PR, not in chat. Do
not polish after the PR is open; add commits only to fix review findings or red
CI. Never rewrite history under a running check. When unsure whether a rule applies,
apply it and say so.

## Appendix A: coder brief

```text
Ticket: <id> <title>
Base: origin/main at <sha>. Branch: <prefix>/<ticket>-<slug>. Worktree: <path>.
Owned files or packages: <list>. Do not touch: <list>.
Change class: <docs | tests | tooling | behavior | performance | generated>.
Acceptance: <observable outcomes, one per line>.
Evidence: <native rows or "capture needed: <cases>" for compatibility; local contract for Glade-specific behavior>.
Done: PR open with the template complete, Required CI green, review requested.
Rules: docs/WORKFLOW.md sections 3 to 5, and AGENTS.md.
```

## Appendix B: review instructions

```text
Review pull request <url> for glade (an offline Salesforce Apex runtime in Go).
Read-only, except that you may push one small fix commit to the branch and say so.
Check in this order:
1. Each Salesforce-observable compatibility change has a NATIVE line citing org.tsv:<line> at a
   matching route and API. Each Glade-specific change cites its local contract and regression test.
2. The test commit precedes the fix and fails on its own: check out the test commit and run the test.
3. Route twins: a test for each route that reaches the changed code (anonymous, @IsTest, runner,
   server, hosts; use the applicable current supported version, floor, and 67.0 for Apex where
   relevant to Salesforce compatibility).
4. Existing tests: `go test ./internal/<touched pkg>` at HEAD and at the merge base; a pass that
   becomes a failure is blocking. For apextest, run focused tests with `-run`; CI runs its 14 shards.
5. No deleted or loosened assertion without a matching native row or local contract.
   No third-party project names or content.
   No org ids, aliases, personal paths or credentials.
6. For sema or typesys changes: `go test ./internal/apextest -run 'TestCorpusShape'` at HEAD.
Verdict: APPROVE, or REQUEST CHANGES listing only blocking findings as file:line with the rule
broken. Everything else is a comment.
```

## Appendix C: capture request

```text
Behavior: <one sentence>
Apex and metadata: <complete synthetic source>
APIs: <applicable current supported version> and <floor>; 67.0 for Apex where relevant
Routes: <anonymous | @IsTest | runner | server | Visualforce | LWC>
Org shape: <features, settings, permission sets>
Output: org.tsv with id, api, route, compiled, success, outcome; native-run.md with API versions
and timestamps and no org ids
```

## Appendix D: glossary

- Native row: one observed Salesforce result (a line of `org.tsv`) with its
  provenance in `native-run.md`.
- Route: the path by which code reaches the runtime: anonymous Apex, an
  `@IsTest` class, the project test runner, the server API, a Visualforce or
  LWC host.
- Route twin: the same case on another route.
- Floor: the lowest supported API version, 62.0 for Apex and 59.0 for LWC and
  Visualforce.
- Golden: the committed expected result of a real-project run.
- Change class: section 4.
