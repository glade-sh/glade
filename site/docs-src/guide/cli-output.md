# CLI output modes

Glade writes human output first. It names the result, the file or artifact that changed, and the next command to run.

```text
Glade check

✓ No diagnostics found

Checked:
  Apex types   19
  Triggers     3
  Objects      8
```

## Human output

Default command output is for a terminal. It uses short sections, project-relative paths, and `Next:` or `Try:` commands after non-trivial results.

Human output may change when it makes the command clearer. Do not parse it in CI.

## Machine output

Use `--json` when a script needs stable fields.

```bash
glade doctor --project . --json
glade check --project . --json
glade test --project . --json --no-progress
glade exec --json "System.debug('local');"
glade debug profile --log apex.log --json
```

JSON goes to stdout. Progress and warnings go to stderr. `--json` never mixes human text into stdout.

## Progress

Use `--no-progress` for logs, CI, and copied examples.

```bash
glade check --project . --no-progress
glade test --project . --json --no-progress
```

`--progress-json` writes NDJSON progress events to stderr for tools that want a live feed.

## Color

Glade disables color for non-TTY output, `NO_COLOR`, and `TERM=dumb`.

```bash
NO_COLOR=1 glade check --project .
TERM=dumb glade test --project .
```

The target contract reserves `--color auto|always|never` and `--no-color`. Until those flags are wired through every command, use the environment variables above.

## Paths

Human output uses project-relative paths by default:

```text
force-app/main/default/classes/RefinementService.cls:12:18
```

Verbose and debug output may show absolute paths.
