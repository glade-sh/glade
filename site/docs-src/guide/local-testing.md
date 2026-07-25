# Run Apex Tests Locally

`glade test` discovers Apex test classes from an SFDX project, compiles supported code, runs tests in the local VM, and reports stable outcomes. It uses the same project loader, parser, semantic analyzer, storage, DML, SOQL, trigger, and limit stack as the rest of the CLI.

## Run all tests

```bash
glade test --project .
```

```text
Glade test

1 selected, 1 passed, 0 failed

Selected: 1
Passed:   1
Failed:   0
Runtime:  420ms

  ✓  RefinementServiceTest.testRefinesFileRow  42ms

Next:
  glade test --watch
  glade test failed
```

Machine-readable output:

```bash
glade test --project . --json
```

`--json` writes the versioned envelope described in [Automation and JSON](/guide/automation).

JUnit output for CI:

```bash
mkdir -p reports
glade test --project . --junit reports/glade-junit.xml
```

## Capture local telemetry

Write CPU and heap profiles for `check` or a local test run. A local one-shot
test run can also write opt-in performance counters:

```bash
mkdir -p reports
glade check --project . --cpu-profile reports/check.cpu.pprof --mem-profile reports/check.mem.pprof --perf-json reports/check.perf.json
glade test --project . --no-serve --no-cache --cpu-profile reports/test.cpu.pprof --mem-profile reports/test.mem.pprof --perf-json reports/test.perf.json
```

These local telemetry artifacts do not replace Salesforce validation. CPU and
heap profiles capture the lifetime of the local command and may also profile
daemon or watch mode; they are written when that command exits and cannot be
used with `--connect`. `--perf-json` is for a local one-shot run and cannot be
combined with daemon, watch, or connect modes.

## Filter tests

Run a test class:

```bash
glade test --project . --class RefinementServiceTest
```

Run a single method:

```bash
glade test --project . --class RefinementServiceTest --method testRefinesFileRow
```

Use exact class and method selectors for the short inner loop. Then run the wider suite before shipping.

Use `--parallelism` to set the total worker budget. Method-level parallelism is
enabled by default; within that fixed budget, the scheduler uses recorded class
durations when available and shifts freed capacity to methods in the remaining
classes. Use `--no-parallel-methods` only for tests that share mutable local
state.

```bash
glade test --project . --parallelism 8 --json
```

## Limit modes

Glade supports limit modes for local execution. `strict` enforces supported governor limits closer to Salesforce behavior. `permissive` keeps the local loop moving when a project depends on unfinished areas.

```bash
glade test --project . --limit-mode strict
glade test --project . --limit-mode permissive --json
```

Use `strict` for gates. Use `permissive` when you are carving into unsupported terrain and want the next useful diagnostic.

## Watch mode

Run tests on file changes:

```bash
glade test --project . --watch
```

Run one affected-test pass and exit:

```bash
glade test --project . --watch-once
```

Run the daemon-backed watch loop:

```bash
glade test --project . --daemon --watch
```

Eligible Apex-only edits update incrementally. Metadata, configuration,
project-topology, uncertain, and recovery changes use an authoritative rebuild.
Selection remains conservative and runs the full suite when Glade cannot prove
a narrower set is safe.

Run tests affected by a git ref without remembering the lower-level flag:

```bash
glade test changed --project . --since HEAD
```

Rerun failures from the last completed run:

```bash
glade test failed --project .
glade test --project . --last-failed
```

Print the next likely loop commands without running tests:

```bash
glade test --project . --wizard
```

## Local preview surfaces

LWC and Visualforce preview have their own workflow and product pages.

- [Preview LWC locally](/guide/workflows/lwc-preview)
- [LWC preview](/guide/modules/lwc-preview)
- [Preview Visualforce locally](/guide/workflows/visualforce-preview)
- [Visualforce preview](/guide/modules/visualforce-preview)

## Warm startup across CLI runs

Large projects rebuild local org state and helper compilation on cold start.
`glade test` writes that harness to `.glade/test/startup.meta.json` plus a
hashed payload after the first cold build and reloads it when fingerprint checks
pass.

**[Test Startup Cache](/guide/test-startup-cache)** explains when the cache is
created, how it stays up to date, when it can be wrong, and how to recover.

```bash
glade test serve --project .
glade test daemon status --project .
glade test --project . --class RefinementServiceTest
glade test daemon stop --project .
glade test clear-cache --project .
glade test --project . --no-cache --class RefinementServiceTest
```

Clear the cache after `git pull` or Glade upgrades. Use `--no-cache` when
debugging harness issues.

## CI pattern

A small CI gate can check the project, run affected tests, then write JUnit output for test reporting:

```bash
glade check --project . --json
glade test changed --project . --since origin/main --json --no-progress
mkdir -p reports
glade test --project . --junit reports/glade-junit.xml
```

Saved run artifacts and CI annotations are covered in [Add Glade to CI](/guide/ci-artifacts).

## Outcomes

Local test runs separate assertion failures from load errors, compile errors,
unsupported features, and internal errors. That split matters. A failing
assertion means the test ran and failed. An unsupported feature means
the runtime stopped at a known unsupported Salesforce API.

```text
  ✓  RefinementServiceTest.testRefinesFileRow  42ms
  ✗  RefinementServiceTest.testRejectsBlankFileRow  12ms

  RefinementServiceTest.testRejectsBlankFileRow
  System.AssertException: expected 1, got 0

  force-app/main/default/classes/RefinementServiceTest.cls:42
```

Check [what Glade runs locally](/guide/support-map) before relying on platform
service APIs, exact hosted Visualforce behavior, live side effects, or REST
behavior outside the checked local baseline.

::: tip Try it
Exercise the runtime your tests rely on - DML, triggers, and governor limits - in the local playground:

```bash
glade playground --examples --addr 127.0.0.1:1789 --open
```
:::
