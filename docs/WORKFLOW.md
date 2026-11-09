# Development Workflow

How a change moves from a ticket to a release. This is the process document.
[AGENTS.md](../AGENTS.md) holds the rules and the validation ladder,
[AI_SETUP.md](AI_SETUP.md) the per-surface commands, and
[RELEASE_POLICY.md](RELEASE_POLICY.md) with
[DISTRIBUTION_WORKFLOW.md](DISTRIBUTION_WORKFLOW.md) the release mechanics.

Three facts shape everything below:

- Every change reaches `main` as a pull request. The hosted `Required CI`
  check is the merge gate, and the merge queue re-runs it on the merged result.
- Captured Salesforce behavior is the oracle. A behavior change is backed by a
  native row before the code changes. Nobody fixes a case from inference.
- Each stage is one command with one printed verdict. Where a step needs
  judgment, this document names the rule.

## 1. The flow

| # | Stage | Who | Output | Done when |
| --- | --- | --- | --- | --- |
| 1 | Ticket | coordinator | scope, acceptance criteria, owned files, change class | a coder can start without asking a question |
| 2 | Branch | coder | a branch from `main` in its own worktree | `git status` is clean and the base is current `main` |
| 3 | Evidence | coder | case table and native rows (`org.tsv`, `ORG-RUN.txt`) | every changed behavior has a row at 67.0 and at the floor |
| 4 | Test first | coder | a failing test committed before the fix | the test fails at the test commit and passes at HEAD |
| 5 | Local checks | coder | focused tests, one package run, repoguard | green, or each failure explained in the PR |
| 6 | Pull request | coder | the PR with the template filled in | every field has content or `n/a: <reason>` |
| 7 | CI | hosted | `Required CI` green | the check passes on the PR head |
| 8 | Review | reviewer | one PR review | no open blocking finding, threads resolved |
| 9 | Merge | merge queue or coordinator | a squash commit on `main` | the queue run is green |
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

Fixtures live at `internal/<pkg>/testdata/<family>/`, where `<pkg>` is the
package whose test reads them: `apextest` (under `testdata/conformance/`) for
runtime behavior, `sema` for compile-time diagnostics, `visualforce`, `lwc`
and `dml` for their surfaces. A family directory holds `cases.tsv` (or
`cases.json`), the synthetic Apex sources, `org.tsv` and `ORG-RUN.txt`.

1. Write `cases.tsv`: one row per case with `id`, `api`, `route`, the source,
   and the expected result once captured. Routes are anonymous Apex, an
   `@IsTest` class, the project test runner, the server, and the hosts
   (Visualforce, LWC) that reach the code. APIs are 67.0 and the floor (62.0
   Apex, 59.0 LWC and Visualforce). Give each "accepts" case a "rejects" twin
   and a boundary row. Example for a rule that rejects a negative scale: one
   row that accepts scale 2, one that rejects scale -1 with the exception type
   and message, and one boundary row at scale 0.
2. Capture against a fresh scratch org. The ticket names the org
   authorization; the private operations guide carries the setup for internal
   agents. With no org access, go to section 5 item 1 before writing any
   code. Write `org.tsv` and `ORG-RUN.txt` into the family directory. Scrub
   org ids, aliases and personal paths. Delete the org afterwards.
3. Encode the rows in one data-driven Go test in that package, copying the
   pattern of an existing `*_conformance_test.go` there. The fixture,
   `org.tsv`, `ORG-RUN.txt` and the test go into the test commit of section
   3.4. The test is the oracle from then on, and CI runs it without
   credentials.
4. Fix only rows the test proves wrong. Cite every row you rely on, one line
   per row, in the commit message and in the PR:
   `NATIVE <behavior> <path>/org.tsv:<line> <route> <api>`. A behavior
   captured at two APIs cites both rows.

A row already in the repository backs a fix without a new capture when its
route and API match the test. With no row at all, stop and ask (section 5).

### 3.4 Test first, then fix

```bash
# 1. Commit the evidence and the test, without the fix.
git add <fixture dir> <test files>
git commit -m "test: <behavior> (NATIVE ...)"
# 2. Show the test fails at that commit (the fix is absent, so this is the
#    "fails on its parent" proof AGENTS.md asks for).
CGO_ENABLED=1 go test ./internal/<pkg> -run '^TestName$'   # expect FAIL
# 3. Commit the fix and show the test passes.
git add <files>
git commit -m "fix: <behavior> (NATIVE ...)"
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
CI runs the 12 shards. `-timeout=0` matters for `internal/vm`,
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

### 3.7 CI

`Required CI` runs on every PR and again in the merge queue; a run takes
about 40 minutes. When it is red, read the failing job's log, fix on the
branch, and push. Do not re-run a red job hoping for green. When the failing
test is unrelated to your change and passes on `main`, say so in the PR and
open a ticket for it; the coordinator decides on a rerun. The runners carry
Node, Playwright and a browser, so an "environmental" result does not exist
in CI. Locally such a test is "not run", never "passed". `scripts/ci-local-all`
mirrors `Required CI` when hosted CI is unavailable.

### 3.8 Review

One reviewer per PR, assigned by the coordinator, following appendix B.
Blocking findings: a correctness defect; a regression (an existing test or
behavior changes without a cited row); a behavior change without a `NATIVE`
line; a missing route twin; a deleted or loosened assertion without a row;
third-party names or content; credentials, org ids or personal paths.
Everything else is a comment, not a block. The reviewer may fix a small
finding with a commit on the branch and approve, saying so in the review.
After "changes requested", the coder fixes on the branch without rebasing, and
the reviewer re-reviews only the new commits.

Docs-only PRs, and test or fixture PRs that only add, need no review. CI still
gates them.

### 3.9 Merge

Squash merge through the merge queue, or by the coordinator when the queue is
off. The squash message is the PR title plus the `NATIVE` lines. The branch is
deleted on merge. The coordinator moves the ticket to Done on the project
board with the PR link. When the queue removes a PR for a conflict (no check
is running on it then), rebase onto `main`, rerun the focused tests, push, and
re-queue.

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

1. The nightly is green on the candidate commit.
2. The full real-project tier is green on the exact candidate.
3. The performance aggregate is measured once on the candidate with identical
   outcomes; the notes carry only measured numbers.
4. The release notes section is written and reviewed.
5. `scripts/release-check.sh` is green on the exact tree, and so are
   `gofmt -l .`, `go vet -trimpath ./...`, `go test -count=1 ./scripts` (from a
   real git checkout with `jq` and Node) and `go test ./internal/repoguard`.
6. The diff since the last tag passes the leak scan: no third-party names,
   personal paths, org ids or credentials.
7. Tag and publish per [DISTRIBUTION_WORKFLOW.md](DISTRIBUTION_WORKFLOW.md).
8. Install and run `glade doctor` on one machine from the published archive.

No step repeats on an unchanged tree. A gap found while qualifying a release
is fixed before the tag unless the maintainer decides otherwise.

## 4. Change classes

Every ticket and PR names one class: `docs`, `tests`, `tooling`, `behavior`,
`performance` or `generated`.

| Class | Covers | Needs | Does not need |
| --- | --- | --- | --- |
| `docs` | documentation only | PR, CI | review, rows |
| `tests` | tests or fixtures only | PR, CI; when an existing assertion is deleted or changed: a cited row per change and a review | review for pure additions |
| `tooling` | CI, scripts, build, release tooling | PR, CI; the lane and inventory tests when lanes change | rows |
| `behavior` | parser, sema, vm, soql, dml, storage, Visualforce, LWC, server behavior | rows, test first, route twins, review, the ABI bump below | |
| `performance` | speed or memory with identical outcomes | identical outcomes (CI proves them) and the targeted benchmark in the PR; the release measures the aggregate | per-PR project measurement |
| `generated` | generated files, including the standard-object schema ([STANDARD_OBJECT_SCHEMA.md](STANDARD_OBJECT_SCHEMA.md)) | change the input or the generator, regenerate, commit both | hand edits |

ABI bumps, in the same PR and once:

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

Raise the number by one. When `main` moved and bumped the same constant,
rebase and take the higher value plus one. Tests that pin a version or a
per-version counter change in the same PR, with the reason in the message.

## 5. Stop and ask

Stop and ask the coordinator, in the PR or the ticket, when:

1. a behavior change has no native row and you cannot capture one;
2. the fix needs files outside your owned set, a generated output, or an ABI
   constant that another open PR also changes (look at the open PRs);
3. a test that passes on `main` fails on your branch and an hour of work has
   not explained it;
4. the change would alter a public support claim, a hosted boundary, or an
   excluded area (ConnectApi, Slack, DataWeave).

Everything else: decide from this document and state the decision in the PR.

## 6. For agents

Each stage is one command and one verdict. Report in the PR, not in chat. Do
not polish after the PR is open; add commits only for review findings. Never
rewrite history under a running check. When unsure whether a rule applies,
apply it and say so.

## Appendix A: coder brief

```text
Ticket: <id> <title>
Base: origin/main at <sha>. Branch: <prefix>/<ticket>-<slug>. Worktree: <path>.
Owned files or packages: <list>. Do not touch: <list>.
Change class: <docs | tests | tooling | behavior | performance | generated>.
Acceptance: <observable outcomes, one per line>.
Evidence: <path of existing rows, or "capture needed: <cases>">.
Done: PR open with the template complete, Required CI green, review requested.
Rules: docs/WORKFLOW.md sections 3 to 5, and AGENTS.md.
```

## Appendix B: review instructions

```text
Review pull request <url> for glade (an offline Salesforce Apex runtime in Go).
Read-only, except that you may push one small fix commit to the branch and say so.
Check in this order:
1. Every behavior change has a NATIVE line citing org.tsv:<line> at a matching route and API.
2. The test commit precedes the fix and fails on its own: check out the test commit and run the test.
3. Route twins: a test for each route that reaches the changed code (anonymous, @IsTest, runner,
   server, hosts; 62.0 and 67.0 where applicable).
4. Existing tests: `go test ./internal/<touched pkg>` at HEAD and at the merge base; a pass that
   becomes a failure is blocking.
5. No deleted or loosened assertion without a cited row. No third-party project names or content.
   No org ids, aliases, personal paths or credentials.
6. For sema or typesys changes: `go test ./internal/apextest -run 'TestCorpusShape'` at HEAD.
Verdict: APPROVE, or REQUEST CHANGES listing only blocking findings as file:line with the rule
broken. Everything else is a comment.
```

## Appendix C: capture request

```text
Behavior: <one sentence>
Apex and metadata: <complete synthetic source>
APIs: 67.0 and <floor>
Routes: <anonymous | @IsTest | runner | server | Visualforce | LWC>
Org shape: <features, settings, permission sets>
Output: org.tsv with id, api, route, compiled, success, outcome; ORG-RUN.txt with API versions
and timestamps and no org ids
```

## Appendix D: glossary

- Native row: one observed Salesforce result (a line of `org.tsv`) with its
  provenance in `ORG-RUN.txt`.
- Route: the path by which code reaches the runtime: anonymous Apex, an
  `@IsTest` class, the project test runner, the server API, a Visualforce or
  LWC host.
- Route twin: the same case on another route.
- Floor: the lowest supported API version, 62.0 for Apex and 59.0 for LWC and
  Visualforce.
- Golden: the committed expected result of a real-project run.
- Change class: section 4.
