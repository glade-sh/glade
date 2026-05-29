# Local Apex Testing

Glade runs Apex tests from local Salesforce source without a Salesforce org.
There is no org login, scratch org, source push, or metadata deploy in the
normal loop. Glade reads the SFDX project on disk, loads supported metadata, and
runs tests in the local VM.

## Project Setup

Start from a Salesforce-shaped project with `sfdx-project.json` at the root:

```text
my-project/
  sfdx-project.json
  force-app/main/default/classes/
  force-app/main/default/triggers/
  force-app/main/default/objects/
```

Then run from the project root:

```bash
glade doctor
glade check --project .
glade test --project .
```

`glade check` parses and type-checks the project. `glade test` discovers
`@IsTest` classes and runs them locally. Add `--json` for machine-readable
output and `--junit reports/glade-junit.xml` for CI reports.

## Run Tests Without An Org

Run every local Apex test:

```bash
glade test --project . --json
```

Run one test class or one method:

```bash
glade test --project . --filter AccountServiceTest --json
glade test --project . --filter AccountServiceTest.testCreatesAccount --json
```

Use `--parallelism` to choose the worker count. Method-level parallel execution
is on by default; use `--no-parallel-methods` only when a project has tests that
share mutable local state.

```bash
glade test --project . --parallelism 8 --json
```

## Run Tests Affected By Local Changes

Use `--changed-since <git-ref>` to run tests selected from the dependency graph
of changed files. The ref is passed to git, so common choices are `origin/main`,
`main`, or a merge-base branch used by CI.

```bash
git fetch origin main
glade test --project . --changed-since origin/main --json
```

For repeated local loops, keep the project warm with the test daemon:

```bash
glade test --project . --daemon --changed-since origin/main --json
```

For editor-style feedback, use watch mode. The first run covers the full test
set; later runs select affected tests from file changes:

```bash
glade test --project . --daemon --watch
```

## Compatibility Local-Test Runs

`glade compat local-tests` uses the same local runtime but reports readiness
outcomes for large-project parity work: `pass`, `fail`, `unsupported`,
`loadError`, `compileError`, and `internalError`.

Use it when you want triage output, blocker grouping, sharding, or the
compatibility JSON shape:

```bash
glade compat local-tests --project . --parallel auto --json
```

Focused runs use explicit class and method flags:

```bash
glade compat local-tests --project . --class AccountServiceTest --json
glade compat local-tests --project . --class AccountServiceTest --method testCreatesAccount --json
```

Affected-test selection is available here too:

```bash
glade compat local-tests --project . --changed-since origin/main --parallel auto --json
```

For this command, `--parallel <n|auto>` controls class workers. The
day-to-day `glade test` command uses `--parallelism <n>` and has
`--parallel-methods` enabled by default.

## Anonymous Apex

Run a short execute-anonymous snippet against local source:

```bash
glade exec --project . "System.debug('hello from glade');"
```

Capture a trace when you need to inspect runtime behavior:

```bash
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade profile analyze reports/trace.json
```

## Playground

Use the browser playground for quick experiments with local files, anonymous
Apex, logs, limits, traces, and local org diffs:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

The playground runs on localhost and uses the same compiler, VM, and storage
layers as `glade test` and `glade exec`.
