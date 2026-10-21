---
pageType: reference
canonicalTask: /guide/workflows/ci
---

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

`excludedClasses` names invalid classes, their dependents, and excluded
triggers; `summary.excludedClasses` counts them. Any exclusion exits `1`, even
when all executed tests passed.

## Local Flow execution

Local DML loads active record-triggered Flows from project metadata. The
`recordTriggerType` selects create, update, create-and-update, or delete;
local execution handles before-save and after-save Flows, plus explicit `Delete`
Flows in the before-delete phase. After-delete Flows are not executed locally.
The checked paths cover entry filters, decisions, assignments, supported record
lookup/create/update/delete, loops, subflows, and Apex actions. For supported
after-save entry criteria, `doesRequireRecordChangedToMeetCriteria` compares the
current record with its prior state. `$Record` and `$Record__Prior` support the
captured current/prior record behavior. `Invocable.Action` can call supported
local Flow actions and returns local result/error objects.

Screen and wait nodes remain unsupported. Hosted scheduling, interactive Flow
screens, and the full hosted Flow engine remain outside the local contract.
An unmodeled node is not proof that the whole Flow runs correctly.

`glade check` reports `GLADEAUTO001` workflow warnings, `GLADEAUTO002` Flow
warnings, and `GLADEAUTO003` for an automation file skipped because it could
not be read or parsed. Other files continue to load. `GLADEAUTO004` is an error
for invalid supported Flow definitions, such as a bad triggering-record field
reference. Inspect diagnostics before relying on an automation result.

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
