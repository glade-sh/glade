# Add Glade to CI

Glade writes machine-readable check results and saved test reports for CI.

```yaml
name: glade
on: [pull_request]
jobs:
  glade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: curl -fsSL https://glade.sh/install.sh | sh
      - run: echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: glade doctor
      - run: glade check --project . --format sarif --output glade-check.sarif
      - run: glade test changed --project . --since origin/main --json --no-progress
      - run: mkdir -p reports
      - run: glade test --project . --junit reports/glade-junit.xml
```

## Semantic checks

Use `fetch-depth: 0` when a workflow runs
`glade test changed --project . --since origin/main`. The affected-test selector
needs the git base ref on disk.

Use SARIF for code-scanning uploads:

```bash
glade check --project . --format sarif --output glade-check.sarif
```

Use GitHub annotations for inline workflow logs:

```bash
glade check --project . --format github
```

JSON remains available:

```bash
glade check --project . --format json --output glade-check.json --no-progress
```

`--json` is still accepted as the short form for `--format json`.
JSON output uses the versioned envelope in [Automation and JSON](/guide/automation).

## Saved test runs

`glade dev test` writes a saved run directory under `.glade/runs`:

```bash
glade dev test --project . --out .glade/runs
```

Each run contains `run.json`, `results.json`, `summary.md`, `junit.xml`, and
`events.ndjson`.

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

The default export remains a zip:

```bash
glade report export latest --runs-dir .glade/runs --output glade-report.zip
```

## Refactor proof reports

Use a proof report when CI needs a branch-change artifact:

```bash
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
glade report refactor-proof --project . --since origin/main --fail-on-api-break --format json
```

The report records the git diff, parse and semantic status, graph impact,
affected-test selection, optional trace summary, and public or global API
surface warnings.

Upload the files after a gate, even when the test or check step fails:

```yaml
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: glade-results
          path: |
            glade-check.sarif
            reports/glade-junit.xml
            .glade/runs
```

`glade check` and `glade test` return non-zero for diagnostics or failed test
outcomes that should block a gate. Use `if: always()` only for upload steps that
must run after a failure. Exit codes are listed in [Exit codes](/guide/exit-codes).
