# Automation and JSON

Use JSON modes in scripts. Human output is for people and may evolve. In Bash
or Zsh, enable `pipefail` so a formatter such as `jq` does not hide a failed
Glade command. Replace `<base-ref>` with the intended existing comparison ref
and verify it resolves in the checkout.

```bash
set -o pipefail
glade check --project . --json --no-progress | jq .
glade test changed --project . --since <base-ref> --json --no-progress | jq .
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

Fields after `exitCode` are command-dependent and may be omitted. During
migration, command-specific legacy data may appear under `data`; some commands
still use their own JSON shape.

Inspect saved test JSON as well as the process status. The `summary` contains
`total`, `passed`, `failed`, `errors`, `skipped`, and `unsupported` counts.
Unsupported test outcomes count as errors and exit with code `1`. An empty
affected selection can exit with code `0`; run an explicit relevant test or
suite when execution evidence is required.

## Stdout and stderr

Stdout carries the primary result: JSON, SARIF, JUnit, Markdown, HTML, or terminal text.

Completed result-producing JSON modes keep progress on stderr, including
`--progress-json` NDJSON events. Early setup failures may leave stdout empty;
check process status before parsing. Watch modes emit NDJSON on stdout, while
wizard output and legacy paths such as `glade test failed --json` with no saved
failures still print human text. See [CLI output modes](/guide/cli-output).

## CI pattern

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
- run: test -f glade.yml || glade init --project . --yes
- run: glade doctor --project .
- run: glade check --project . --format sarif --output glade-check.sarif
- run: glade test changed --project . --since <base-ref> --json --no-progress
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
