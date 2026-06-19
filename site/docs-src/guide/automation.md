# Automation and JSON

Use JSON modes in scripts. Human output is for people and may evolve.

```bash
glade check --project . --json --no-progress | jq .
glade test changed --project . --since origin/main --json --no-progress | jq .
```

## Envelope

Priority commands use a versioned envelope.

```json
{
  "schemaVersion": "1.0",
  "command": "check",
  "status": "failed",
  "exitCode": 1,
  "project": {},
  "summary": {},
  "diagnostics": [],
  "artifacts": [],
  "suggestions": []
}
```

During migration, command-specific legacy data may appear under `data`.

## Stdout and stderr

Stdout carries the primary result: JSON, SARIF, JUnit, Markdown, HTML, or terminal text.

Stderr carries progress, warnings, and internal status. `--json` keeps stdout parseable even when `--progress-json` writes NDJSON events to stderr.

## CI pattern

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
- run: glade doctor
- run: glade check --project . --format sarif --output glade-check.sarif
- run: glade test changed --project . --since origin/main --json --no-progress
- run: mkdir -p reports
- run: glade test --project . --junit reports/glade-junit.xml
```

Use `if: always()` only for artifact upload steps that must run after a failing check or test command.

## Specialized formats

| Command | Machine formats |
| --- | --- |
| `glade check` | JSON, SARIF, GitHub annotations |
| `glade test` | JSON, JUnit |
| `glade report` | JSON, HTML, Markdown, GitHub annotations, zip |
| `glade debug profile` | JSON, text, Markdown |
| `glade profile analyze` | JSON, text, Markdown, pprof |
