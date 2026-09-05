# Exit codes

Glade uses exit status for automation. Scripts should trust the process status first, then read JSON fields when they need detail.

Current built-in command results:

| Code | Meaning |
| ---: | --- |
| 0 | Success |
| 1 | Command failure, including diagnostics, test failures, and most usage errors |
| 2 | Unknown top-level command |
| 3 | Config discovery failure where explicitly reported by config commands |

`glade test` returns `1` for unsupported test outcomes: those outcomes count
as test errors. Inspect the JSON summary and per-test status. A zero-test run
can return `0`, so success does not by itself establish test coverage.

The broader exit-code taxonomy reserves `4` for unsupported runtime boundaries,
`5` for external dependencies, `70` for internal errors, and `130` for
interruption. These are not consistent current mappings. Do not rely on those
specific codes to classify a failure; use the command diagnostics. Plugins may
return their own process status.

Examples:

```bash
glade check --project .
echo $?

glade test --project . --json --no-progress > glade-test.json
echo $?
```

Commands using a result envelope repeat the exit code. Early failures may
produce no JSON, so check the process status before parsing stdout:

```json
{
  "schemaVersion": "1.0",
  "command": "test",
  "status": "failed",
  "exitCode": 1
}
```
