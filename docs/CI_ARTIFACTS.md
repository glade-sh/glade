# CI And Artifacts

Glade can write machine-readable check results and saved test reports for CI.

Use the canonical [advisory pilot and enforcing gate](https://glade.sh/guide/ci-artifacts)
for complete pinned workflows. Start advisory: retain failed assessment/test
outcomes and artifacts without making local compatibility an unexpected merge
requirement. Install/doctor failures remain setup failures. Move to an enforcing
gate only after the team validates its chosen scope.

## Semantic Check Outputs

Use SARIF for code-scanning uploads:

```bash
glade check --project . --format sarif --output glade-check.sarif
```

In GitHub Actions, use `actions/checkout` with `fetch-depth: 0` before
`glade test changed --since origin/main`. The affected-test selector needs the
git base ref on disk.

Use GitHub annotations for inline workflow logs:

```bash
glade check --project . --format github
```

JSON remains available:

```bash
glade check --project . --format json --output glade-check.json --no-progress
```

`--json` is still accepted as the short form for `--format json`.

## Saved Test Runs

`glade dev test` writes a saved run directory under `.glade/runs`:

```bash
glade dev test --project . --out .glade/runs
```

Each run contains:

| File | Purpose |
| --- | --- |
| `run.json` | Project, run id, and creation time. |
| `results.json` | Full structured test result. |
| `summary.md` | Console-style human summary. |
| `junit.xml` | JUnit XML for CI test tabs. |
| `events.ndjson` | Watch events when the run came from watch mode. |

Read the latest run as JSON:

```bash
glade report show latest --runs-dir .glade/runs --json
```

Emit GitHub annotations for the latest failing tests:

```bash
glade report github latest --runs-dir .glade/runs
```

Export a browsable HTML artifact:

```bash
glade report export latest --runs-dir .glade/runs --format html --output glade-report.html
```

The default export remains a zip of the saved run directory:

```bash
glade report export latest --runs-dir .glade/runs --output glade-report.zip
```

## Refactor Proof Reports

Use the enterprise proof report when CI needs a branch-change artifact:

```bash
mkdir -p reports
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
glade report refactor-proof --project . --since origin/main --fail-on-api-break --format json
```

The report records the git diff, parse and semantic status, graph impact,
affected-test selection, optional trace summary, and public or global API
surface warnings. It does not execute the selected tests. Run the relevant tests
separately and inspect their executed counts; a successful report command is
not proof that tests ran.

## Repository release proof

Maintainers run `scripts/release-check.sh` before a tag. Its Go phase validates
the checked package inventory and writes local evidence beneath
`ci-artifacts/local-release/`. Each lane contains:

| File | Purpose |
| --- | --- |
| `events.json` | Raw `go test -json` events for the lane. |
| `package-summary.json` | Validated package ownership and result summary. |

The default lane execution is serial to bound memory. An explicit
`LOCAL_GO_TEST_JOBS` value greater than one may overlap only the final
independent lanes. The site phase runs its proofs exactly once and writes
`site/.vitepress/release-check.json`.

These are local maintainer artifacts, not product command output. Do not commit
them.
