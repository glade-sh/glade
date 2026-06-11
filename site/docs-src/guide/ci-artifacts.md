# CI And Artifacts

Glade writes machine-readable check results and saved test reports for CI.

## Semantic checks

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
glade check --project . --format json --output glade-check.json
```

`--json` is still accepted as the short form for `--format json`.

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
