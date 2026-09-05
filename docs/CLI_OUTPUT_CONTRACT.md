# Glade CLI Output Contract

This document is the shared contract for Glade command output.

## Output Modes

- Default output is concise human text. It leads with the user outcome.
- `--verbose` and `--debug` may show internal detail and absolute paths.
- Completed result-producing `--json` modes write JSON to stdout; see the legacy and watch exceptions below.
- `--format` writes specialized artifacts such as SARIF, GitHub annotations, JUnit, Markdown, or HTML where a command supports them.
- `--quiet` suppresses nonessential human progress.
- `--plain` is reserved for future grep-friendly output.
- `--no-progress` suppresses progress output.
- `--color auto|always|never` is the target color contract. Current commands honor `NO_COLOR`, `TERM=dumb`, and non-TTY output. `--no-color` is reserved as an alias for `--color never`.

## Exit Codes

Current built-in commands use these results:

```text
0    success
1    command failure, including diagnostics, test failures, and most usage errors
2    unknown top-level command
3    config discovery failure where explicitly reported by config commands
```

The broader taxonomy in `glade help exit-codes` reserves `4` for
unsupported runtime boundaries, `5` for external dependencies, `70` for internal
errors, and `130` for interruption. These are not consistent current mappings.
Scripts should treat any nonzero exit as failure and inspect the command's
diagnostics. See the [exit-code reference](../site/docs-src/guide/exit-codes.md).

## Stdout And Stderr

Stdout carries primary command output, JSON, and machine-readable formats.

Stderr carries progress, warnings, non-primary status messages, and debug/internal messages.

Result-producing JSON modes keep progress off stdout.
An error before result construction may leave stdout empty and report the error
only on stderr. Check the process exit code before parsing output. Watch modes
emit NDJSON events rather than a single result envelope. Wizard output and
legacy paths such as `glade test failed --json` with no saved failures still
print human text.

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

Fields after `exitCode` are command-dependent and may be omitted. Existing
package-level JSON structs remain available inside the `data` field where needed
during migration; some commands still return their own JSON shape. Consumers
must use the schema for the specific command and mode.

## Diagnostics

Human diagnostics use:

```text
force-app/main/default/classes/RefinementService.cls:12:18
error GLADESEMA002 unknown type RefinementService
```

Default human output uses project-relative paths. Absolute paths are for verbose/debug output.

## Prompting

Interactive commands should make prompting explicit:

- `--wizard` prints a recommended command without making changes.
- `--yes` accepts inferred defaults.
- `--no-input` should fail instead of prompting where prompting is implemented.

Human output may evolve for clarity. JSON envelope fields are versioned and should remain stable.
