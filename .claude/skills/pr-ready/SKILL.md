---
name: pr-ready
description: Use before reporting a branch ready or opening a pull request in glade. Runs the checks reviewers bounce most often - fail-before proof, callers and route twins, loosened assertions, CI lane and race inventory, shared-state safety, public-safety scan, ABI bump class - and builds the PR body.
---

# Ready check

Do these in order on the finished branch. Fix each failure before the next
step. Do not ask for review with an open item.

## 1. Fail-before proof

    git log --oneline origin/main..HEAD        # test commit precedes fix commit
    git worktree add --detach <tmp> <test-commit>
    (cd <tmp> && CGO_ENABLED=1 go test ./internal/<pkg> -run '^TestName$')   # expect FAIL
    git worktree remove <tmp>

The failure must be the behavior under test, not a compile error from a
missing helper. If history cannot be ordered, apply only the test file to the
fix's parent in a detached worktree and record the output in the PR.

## 2. Callers and route twins

For each function whose behavior changed:

    git grep -n "<funcName>(" -- '*.go' ':!*_test.go'

List every caller and every overload. Each route that reaches the change has a
test: anonymous Apex, `@IsTest`, the runner, the server, Visualforce and LWC
hosts where they apply. Three overloads means three tests.

## 3. Existing tests and assertions

    git diff origin/main...HEAD -- '*_test.go' 'internal/*/testdata' | grep -n '^-'

Every removed or changed assertion, fixture row or budget needs a native row
or a stated local contract in the PR. Otherwise restore it.

If you regenerated data (schema, picklists, permissions, stubs), run the tests
that pin counts in that package before pushing: `go test ./internal/<pkg> -run
'Deterministic|Coverage|Policy|Inventory'`.

For `internal/sema` or `internal/typesys` changes:
`CGO_ENABLED=1 go test ./internal/apextest -run 'TestCorpusShape'`.

## 4. CI manifests

Needed when the diff adds a Go package, a goroutine, a `sync` primitive, a
worker pool or a `t.Parallel` test:

- `scripts/ci-package-lanes.json`: package lane and concurrent test count.
- `scripts/ci-race-push.json`: the package, when production code gains
  concurrency.
- Then: `go test -count=1 ./scripts -run 'Inventory|Concurrency|RacePush|Lane'`.

A changed path outside every Go package (for example `lwcruntime/`,
`third_party/lwc/`, root `testdata/`) selects the full suite in CI. Say so in
the PR.

## 5. Shared state

New cache, memo, pooled or shared value: state its key and its invalidation in
the PR. Add a test that mutates a returned value and reads the source again. A
copied struct still shares its maps and slices.

## 6. Mechanical gate

    gofmt -l $(git diff --name-only origin/main...HEAD -- '*.go')   # prints nothing
    CGO_ENABLED=1 go vet ./internal/<pkg>
    CGO_ENABLED=1 go test ./internal/repoguard
    bash scripts/pr-check.sh --repoguard --vet

Fix every FAIL. A WARN needs its reason in the PR. Exception: the `abi` check.
Do not change `SemanticABI`, `PlatformABI` or `testRuntimeCacheABI`; write
`Bump class: <names>` in the PR (rules: docs/WORKFLOW.md section 4).

## 7. Public safety and names

The diff, commit messages and PR body hold no third-party project names, org
ids, usernames, personal or temp paths. New files and tests are named for the
behavior, not for a ticket, case number or review round. No editor backups.

## 8. Clean-tree rerun

    mkdir <tmp> && git archive HEAD | tar -x -C <tmp>

Run the new tests there. Untracked files cannot hide a failure.

## 9. PR body

Fill `.github/PULL_REQUEST_TEMPLATE.md`. Every field has content or
`n/a: <reason>`. Include: change class; NATIVE lines or local contract;
fail-before output; callers and routes tested; commands and results; tests not
run and why; bump class; unproven cases left unchanged. A behavior change adds
its line under Unreleased in `docs/RELEASE_NOTES.md`.

Then stop. Do not polish after the PR is open.
