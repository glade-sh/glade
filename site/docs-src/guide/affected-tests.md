# Run Only Affected Tests

Affected-test selection trims the test set by comparing local changes against a static Apex reference graph. It is designed for the inner loop: run the tests most likely to observe the files you changed.

```text
Changed file -> Apex reference graph -> selected tests
```

## Changed files from git

Use `glade test changed --since <base-ref>` to compare watchable project files
in the working tree against a git ref. This includes staged and unstaged
tracked changes and untracked, non-ignored watchable files. Replace `<base-ref>`
with the intended existing comparison ref and verify it resolves before running:

```bash
glade test changed --project . --since <base-ref> --json --no-progress
```

A changed production class selects tests that reach it through direct or
transitive references in the Apex graph. A changed test class selects itself.
Metadata and unknown file types fall back to conservative behavior.

## Watch and one-shot modes

Use watch mode during editing:

```bash
glade test --project . --watch
```

Use one-shot watch mode to run the initial suite, or the explicit class and
method selection, and exit:

```bash
glade test --project . --watch-once
```

`--watch-once` does not wait for an edit or compare against a git ref. Use
`glade test changed --since <base-ref>` for a single run selected by git changes.

Use the daemon when repeated parsing and graph rebuilds would cost too much
inside one watch process:

```bash
glade test --project . --daemon --watch
```

Use `glade test serve` when separate CLI invocations should stay warm across
terminals or editor tasks.

## Selection modes

The affected-test report distinguishes three useful outcomes:

- `direct` — a precise set of tests selected through direct and transitive
  references to changed Apex symbols.
- `all` — the safe fallback when Glade cannot narrow the impact.
- `none` — no tests selected for the observed change set.

`none` means Glade did not find tests affected by the current change set. It
can exit with code `0` while running zero tests. Read the test summary counts
and run an explicit relevant test or suite when you need execution evidence;
an empty selection does not validate the change.

```json
{
  "mode": "direct",
  "changed": ["force-app/main/default/classes/RefinementService.cls"],
  "tests": ["RefinementServiceTest"]
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
glade check --project . --json --no-progress
glade test changed --project . --since <base-ref> --json --no-progress
mkdir -p reports
glade test --project . --junit reports/glade-junit.xml
```

During a focused edit:

```bash
glade test --project . --class RefinementServiceTest --watch
```

When Glade cannot prove a smaller safe set, it runs more tests rather than risk
missing a failure.
