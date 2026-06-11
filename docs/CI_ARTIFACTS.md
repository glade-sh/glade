# CI And Artifacts

Glade can write machine-readable check results and saved test reports for CI.

## Semantic Check Outputs

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
