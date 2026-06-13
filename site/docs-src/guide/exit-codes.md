# Exit codes

Glade uses exit status for automation. Scripts should trust the process status first, then read JSON fields when they need detail.

| Code | Meaning |
| ---: | --- |
| 0 | Success |
| 1 | Diagnostics, test failures, or expected validation failure |
| 2 | Usage error or invalid flags |
| 3 | Project or config discovery failure |
| 4 | Unsupported local runtime boundary |
| 5 | External dependency or toolchain failure |
| 70 | Internal Glade error |
| 130 | Interrupted by Ctrl-C |

Some legacy command paths still return `1` for usage or flag errors while the full exit-code migration lands. Scripts should treat both `1` and `2` as command failure unless a command page documents a narrower contract.

Examples:

```bash
glade check --project .
echo $?

glade test --project . --json --no-progress > glade-test.json
echo $?
```

JSON envelopes repeat the exit code:

```json
{
  "schemaVersion": "1.0",
  "command": "test",
  "status": "failed",
  "exitCode": 1
}
```
