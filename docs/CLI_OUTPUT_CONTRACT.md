# Glade CLI Output Contract

This document is the shared contract for Glade command output.

## Output Modes

- Default output is concise human text. It leads with the user outcome.
- `--verbose` and `--debug` may show internal detail and absolute paths.
- `--json` writes stable JSON to stdout only.
- `--format` writes specialized artifacts such as SARIF, GitHub annotations, JUnit, Markdown, or HTML where a command supports them.
- `--quiet` suppresses nonessential human progress.
- `--plain` is reserved for future grep-friendly output.
- `--no-progress` suppresses progress output.
- `--color auto|always|never` is the target color contract. Current commands honor `NO_COLOR`, `TERM=dumb`, and non-TTY output. `--no-color` is reserved as an alias for `--color never`.

## Exit Codes

```text
0    success
1    diagnostics, test failures, or expected validation failure
2    usage error or invalid flags
3    project/config discovery failure
4    unsupported local runtime boundary
5    external dependency/toolchain failure
70   internal Glade error
130  interrupted by Ctrl-C
```

Commands that still return a smaller legacy set must document that behavior in help until migrated. During this migration, some usage and flag errors still return `1`; scripts should treat both `1` and `2` as command failure unless a command documents a narrower contract.

## Stdout And Stderr

Stdout carries primary command output, JSON, and machine-readable formats.

Stderr carries progress, warnings, non-primary status messages, and debug/internal messages.

For `--json`, stdout must contain JSON only. Progress must not mix into stdout.

## JSON Envelope

Priority command JSON uses this envelope at the CLI boundary:

```json
{
  "schemaVersion": "1.0",
  "command": "check",
  "status": "passed",
  "exitCode": 0,
  "project": {},
  "summary": {},
  "diagnostics": [],
  "artifacts": [],
  "timings": {},
  "suggestions": []
}
```

Existing package-level JSON structs remain available inside the `data` field where needed during migration.

## Diagnostics

Human diagnostics use:

```text
force-app/main/default/classes/AccountService.cls:12:18
error GLADESEMA002 unknown type AccountService
```

Default human output uses project-relative paths. Absolute paths are for verbose/debug output.

## Prompting

Interactive commands should make prompting explicit:

- `--wizard` prints a recommended command without making changes.
- `--yes` accepts inferred defaults.
- `--no-input` should fail instead of prompting where prompting is implemented.

Human output may evolve for clarity. JSON envelope fields are versioned and should remain stable.
