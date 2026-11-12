# AI Contributor Guide

Shared rules for coding agents and human contributors, independent of client or
model. This file states rules. Procedures live in task skills under
`.claude/skills/<name>/SKILL.md` and in the linked docs. Read the skill whose
description matches the task. A client that does not load skills automatically
should open that file directly.

- Process, ticket to release: [docs/WORKFLOW.md](docs/WORKFLOW.md)
- Packages and runtime path: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Checks per changed surface: [docs/AI_SETUP.md](docs/AI_SETUP.md#validation-by-surface)
- Allowed inputs: [docs/CLEAN_ROOM.md](docs/CLEAN_ROOM.md)

## Voice

Write plainly. Keep observations concrete. Name the file, command, and result.
Use short sentences.

## Working agreement

- Inspect the branch, commit and `git status --short` before editing. Preserve
  unrelated changes. Stage only files this task owns.
- One ticket, one branch from `origin/main`, one worktree, one pull request.
  Never two writers in one worktree.
- Every change reaches `main` as a pull request. The hosted `Required CI`
  check is the merge gate. The pull request body is the report.
- Carry the work through implementation, checks and the self-check. Decide
  routine choices from existing patterns. Stop only for the conditions in
  WORKFLOW.md section 5.
- Once a commit is under review, add commits only for review findings or red
  CI. Never rewrite history under a running check.

## Project boundary

This repository is the `glade` product: Apex parsing and semantic checks; the
VM, test runner, SOQL, DML, storage, schema and server runtime; the product
CLI; product docs and release scripts.

Maintenance tooling lives in the sibling `glade-tools` repository: compatibility
fixtures and runners, capability catalogs, generated ledgers and reports,
project-readiness scans. Tools may depend on this repository. This repository
must not depend on `glade-tools`. Do not add a maintenance command to base
`glade`.

## Rules for behavior changes

- Salesforce-observable behavior comes only from captured native rows
  (`org.tsv` plus `native-run.md`) or a written public specification. Never
  infer it. Never infer field behavior from field names.
- Implement the narrowest rule the rows prove. Key it on captured facts, not on
  identifiers or literal tokens. Leave unproven cases at current behavior and
  list them in the pull request.
- Test first: commit a test that fails on its parent, then the fix.
- Never loosen, delete or skip an existing assertion, fixture or budget without
  a native row or a stated local contract.
- Trace every caller of a function you change. Test each route that reaches it:
  anonymous Apex, `@IsTest`, the test runner, the server, and the Visualforce
  and LWC hosts where they apply.
- Reuse `storage`, `dml`, `soql`, `vm`, `apextest`, `server` and `testreport`.
  Do not add a parallel implementation for one CLI or server path.
- Never add project-specific exceptions to product code.
- Do not change `SemanticABI`, `PlatformABI` or `testRuntimeCacheABI`. Name the
  bump class your change needs in the pull request; the release cut bumps once.
- Read a generated file's header before editing. Change the input or the
  generator and regenerate.
- Use the `capture-conformance` skill for the capture and fixture procedure.

## Validation

Use the Go version in `go.mod`, a C compiler and `CGO_ENABLED=1`. A no-CGO
build is not a working parser.

```bash
CGO_ENABLED=1 go test ./internal/<package> -run '^(TestName|TestOther)$'
CGO_ENABLED=1 go test ./internal/repoguard
bash scripts/pr-check.sh
```

- Run focused tests while iterating. Hosted CI runs every package.
- Never run the whole `internal/apextest` package locally; CI runs it in 14
  shards (`GLADE_APEXTEST_SHARD=i/n` reproduces one).
- Use `-timeout=0` for `internal/vm`, `internal/gladecli` and
  `internal/visualforce`.
- A failing test: rerun it alone, then on `origin/main`. The same failure there
  is pre-existing; say so in the pull request.
- A test that needs Node, Playwright or Chromium and did not run is "not run",
  never "passed".
- A new Go package, goroutine, `sync` primitive or `t.Parallel` test needs its
  entries in `scripts/ci-package-lanes.json` and `scripts/ci-race-push.json`.
  `go test ./scripts` checks them.
- Before asking for review, run the `pr-ready` skill. For other surfaces (site,
  playground, LWC, VS Code extension, release), use the `validate` skill.

## Public repository hygiene

- No third-party project names, paths or source. No org ids, usernames or
  aliases. No personal absolute paths. No credentials.
- Name files and tests for the behavior, not for a ticket, review round or case
  number.
- Do not commit built binaries, `/bin/`, `/dist/`, `coverage.out`, compiled
  test binaries, editor backups (`*.bak`, `*.orig`, `*-E`, `*~`) or run dumps.
- A user-visible change adds its line under Unreleased in
  `docs/RELEASE_NOTES.md`.

## Reporting

Report commands and outcomes. Name skipped tests, empty selections,
environmental limits and remaining uncertainty. A local green gate does not
establish live parity or release readiness.
