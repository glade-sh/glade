# Oaer CLI DX Plan

## Summary
The CLI has good machinery now. The handles are rough. Keep `oaer test`, `check`, `package`, `compat`, and friends as stable scriptable tools. Add a friendly `oaer dev` cockpit over them.

Approved defaults:
- Surface: `oaer dev` cockpit plus existing command guts.
- Watch: conservative affected-test runs.
- Voice: quiet playful in TTY output, plain in JSON and CI.
- Artifacts: `.oaer/runs` by default for DX commands.

## Key Changes
- Add `oaer dev`:
  - `oaer dev` prints project status, discovered tests, last run, config, and next useful commands.
  - `oaer dev watch` renders human watch output while reusing existing NDJSON watch events underneath.
  - `oaer dev test` wraps common manual runs: all tests, class, method, changed files, and failed-from-last.
  - `oaer dev export package` wraps `oaer package build` with safer defaults.
- Keep machine surfaces stable:
  - `oaer test --watch` keeps NDJSON behavior for editors and automation.
  - Add `--format pretty|console|json|ndjson` where useful.
  - Add per-command help: `oaer test --help`, `oaer dev --help`, `oaer report --help`.
- Add run artifacts:
  - Default DX run root: `.oaer/runs/<run-id>/`.
  - Write `run.json`, `summary.md`, `results.json`, `junit.xml`, `selection.json`, `events.ndjson`, and optional trace/profile files.
  - Use `.oaer/runs/latest.json` as a portable pointer, not a symlink.
  - Add `oaer report list|show|export|clean`.
- Improve selectors:
  - Add `--class`, `--method`, `--test Class.method`, `--class-list`, `--class-file`, `--changed-since`, and `--failed-from latest`.
  - Keep current `--filter` as a compatibility alias.
  - Print selection reasons in human output and `selection.json`.
- Make watch useful:
  - Default `oaer dev watch` to affected tests.
  - Run all tests for schema changes, triggers, unknown classes, weak dependency edges, and deleted files.
  - Show changed files, selected tests, fallback reason, run duration, and next command.
  - Cancel stale runs and suppress stale results, matching current watch behavior.

## Interfaces And Types
- Add `internal/runartifact` for run IDs, artifact manifests, latest pointer, cleanup, and export.
- Add `internal/cliui` for TTY detection, color, width-aware tables, progress lines, and quiet playful text.
- Extend `internal/testreport` with Markdown summaries and failure-focused console rendering.
- Extend `internal/watch` selection output with `confidence`, `reason`, `changedFiles`, and `fallback`.
- Extend `oaer.yml` with scalar keys only:
  - `dx.runsDir`
  - `dx.color`
  - `dx.whimsy`
  - `watch.debounce`
  - `watch.strategy`
  - `reports.keep`

## Test Plan
- CLI tests for `--help`, unknown flags, selector aliases, and stable exit codes.
- Artifact tests for run directory creation, `latest.json`, cleanup, export, and no writes in raw commands unless `--out` is passed.
- Watch tests for direct dependency selection, trigger/schema fallback to all, unknown fallback to all, stale run suppression, and pretty renderer output.
- Report tests for JSON, Markdown, JUnit, failures-only, and failed-from-last reruns.
- Smoke checks:
  - `go test ./internal/oaercli ./internal/watch ./internal/testreport ./internal/config`
  - `go run ./cmd/oaer dev --project testdata/local-tests/basic`
  - `go run ./cmd/oaer dev test --project testdata/local-tests/basic --out .oaer/runs`
  - `go run ./cmd/oaer test --project testdata/local-tests/basic --watch-once`

## Assumptions
- This is DX work, not runtime parity work.
- Existing dirty WIP in `internal/oaercli`, `internal/packageartifact`, and related files must be preserved or folded in, not overwritten.
- Implementation should happen in a sibling worktree because the checkout is dirty.
- No whimsy appears in JSON, NDJSON, JUnit, CI mode, or `NO_COLOR` output.
- Existing commands remain backward compatible while `oaer dev` becomes the fun front door.
