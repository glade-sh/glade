---
name: release-cut
description: Use when cutting a glade release candidate - ABI bumps, release notes, release PR checks, exact-SHA Required CI, tag and publish. Not for ordinary change PRs, which never bump ABI constants.
---

# Release cut

Only the release owner bumps version constants. A change PR names its bump
class and leaves the constants alone.

## ABI constants

Raise each by one, once, in the release PR. Update the tests that pin a version
or a per-version counter in the same PR, with the reason in the message.

- `SemanticABI` and `PlatformABI`: `internal/sema/sema.go`.
- `testRuntimeCacheABI`: `internal/apextest/startup_cache.go`. Bump it with
  every `SemanticABI` bump.
- `dapCacheVersion`: `internal/gladecli/dap_cache.go`, only when the debug
  adapter cache layout changes.

Path rules for each class are in `docs/WORKFLOW.md` section 4. `scripts/pr-check.sh`
reports which bumps the touched paths require.

## Steps

1. Run the nightly and real-project tier on the exact candidate. Notes list the
   checks passed and any known issues (`docs/RELEASE_POLICY.md`).
2. Measure the performance aggregate with matched outcomes. Notes carry only
   measured numbers.
3. Write the release notes. Run the first project check from `docs/INSTALL.md`
   on a real Salesforce DX project.
4. On the clean candidate tree: `scripts/ci-local-all`, `scripts/release-check.sh`,
   `gofmt -l .` (prints nothing), `go vet -trimpath ./...`,
   `go test -count=1 ./scripts` (real checkout, `jq` and Node available) and
   `go test ./internal/repoguard`.
5. Scan the diff since the last tag for third-party names, personal paths, org
   ids and credentials.
6. After merge and before tagging, run `scripts/verify-required-ci.sh` for the
   exact candidate and check the separate Browser, Race and Security workflows.
7. Tag and publish per `docs/DISTRIBUTION_WORKFLOW.md`.
8. Install from the published archive on one machine and run `glade doctor`.

No step repeats on an unchanged tree. Fix a gap found while qualifying before
the tag unless the maintainer decides otherwise.
