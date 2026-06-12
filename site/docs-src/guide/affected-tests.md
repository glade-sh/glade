# Run Only Affected Tests

Affected-test selection trims the test set by comparing local changes against a static Apex reference graph. It is designed for the inner loop: run the tests most likely to observe the files you changed.

```text
Changed file -> Apex reference graph -> selected tests
```

## Changed files from git

Use `glade test changed --since <ref>` to compare tracked project files against
a git ref:

```bash
glade test changed --project . --since origin/main --json
```

A changed production class selects tests that reference it. A changed test class selects itself. Metadata and unknown file types fall back to conservative behavior.

## Watch and one-shot modes

Use watch mode during editing:

```bash
glade test --project . --watch
```

Use one-shot watch mode for tools that want a single affected run:

```bash
glade test --project . --watch-once
```

Use the daemon when repeated parsing and graph rebuilds would cost too much
inside one watch process:

```bash
glade test --project . --daemon --watch
```

Use `glade test serve` when separate CLI invocations should stay warm across
terminals or editor tasks.

## Selection modes

The affected-test report distinguishes three useful outcomes:

- `direct` — tests selected from direct references to changed Apex symbols.
- `all` — the safe fallback when Glade cannot narrow the impact.
- `none` — no tests selected for the observed change set.

`none` means Glade did not find tests affected by the current change set.

```json
{
  "mode": "direct",
  "changed": ["force-app/main/default/classes/AccountService.cls"],
  "tests": ["AccountServiceTest"]
}
```

## NDJSON watch events

Daemon and watch flows can emit newline-delimited JSON events for editors and automation. Consumers should treat each line as one event and avoid depending on human console text.

A typical event stream contains file-change notices, selection summaries, run starts, per-test outcomes, diagnostics, and run completion records.

```bash
glade test --project . --watch --json
```

## Practical recipes

Before opening a pull request:

```bash
glade check --project . --json
glade test changed --project . --since origin/main --json
glade test --project . --junit reports/glade-junit.xml
```

During a focused edit:

```bash
glade test --project . --filter AccountServiceTest --watch
```

When Glade cannot prove a smaller safe set, it runs more tests rather than risk
missing a failure.
