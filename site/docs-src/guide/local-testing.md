---
pageType: reference
canonicalTask: /guide/workflows/apex-tests
---

# Run Apex tests locally

`glade test` discovers Apex test classes from a Salesforce DX project, compiles supported code, runs tests in the local VM, and reports stable outcomes. It uses the same project loader, parser, semantic analyzer, storage, DML, SOQL, trigger, and limit stack as the rest of the CLI.

The `RefinementServiceTest.opensFile` selectors below match the maintained
editor walkthrough. Your project must contain that class and method, or you
must substitute its actual test names. For a self-contained first run, use
[`SampleTest.adds` in the quickstart](/guide/quickstart#sample-project).
The terminal output below is illustrative; timings are not a benchmark.

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

  ✓  RefinementServiceTest.opensFile  42ms

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
combined with daemon, watch, watch-once, connect, wizard, last-failed, or
shard-plan-only modes.

Check performance JSON reports the project and semantic cache provenance.
`semanticCache.source` records whether the exact result came from memory, disk,
or build; related fields cover waits, safe-miss reasons, retained bytes, and
evictions. Check counters also cover source reads, reused logical views,
allocations, and garbage collection.

Test performance JSON adds an `execution` object with invocation arguments,
requested and effective parallelism, method-parallel policy, `GOMAXPROCS`, disk
cache policy, and execution mode. Its remaining counters cover cache phases
and slow classes or methods. These versioned local artifacts contain project
paths and, for tests, command arguments, so review them before sharing.

`glade check` and the test semantic gate keep exact results under
`.glade/semantic/`. Use `glade check --no-cache` or test `--no-cache` to bypass
semantic cache reads, writes, and memory reuse. Immutable source-generation
validation still runs before a cached result is published or returned.

## Filter tests

Run a test class:

```bash
glade test --project . --class RefinementServiceTest
```

Run a single method (replace the example names with a test in your project):

```bash
glade test --project . --class RefinementServiceTest --method opensFile
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

## Split and balance suites

Use an exact class file, one class per line, when a wrapper already owns
selection. Blank and `#` comment lines are ignored:

```bash
glade test --project . --class-file test-classes.txt
glade test --project . --shard-count 4 --shard-index 0
glade test --project . \
  --shard-count 4 \
  --duration-history .glade/test-durations.json \
  --write-class-shards reports/test-shards
glade test --project . --test-timeout 2m
```

Unfiltered, unsharded runs maintain `.glade/test-durations.json`. Duration
history balances later shards; the assignment is deterministic for the same
selected classes, shard count, and history. `--write-class-shards` writes
`shard-NNN.txt` class lists and exits without running tests. The default
per-test timeout is five minutes. On memory-constrained hosts,
`--gc-aggressive` trades more frequent garbage collection for lower heap
growth.

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

Run the initial suite, or the explicit class and method selection, and exit:

```bash
glade test --project . --watch-once
```

`--watch-once` does not wait for a change or select tests from a git diff. Use
`glade test changed --since <base-ref>` for a single run selected by git changes.

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
- [LWC preview](/guide/modules#lwc-preview)
- [Preview Visualforce locally](/guide/workflows/visualforce-preview)
- [Visualforce preview](/guide/modules#visualforce-preview)

## Warm startup across CLI runs

Large projects rebuild local org state and helper compilation on cold start.
Eligible `glade test` modes write that harness to
`.glade/test/startup.meta.json` plus a hashed payload and reload it when
fingerprint checks pass. One-shot parallel-method runs with more than one
effective worker deliberately bypass restored-runtime disk caching to protect
test isolation; `glade test --wizard` prints the effective policy.

Semantic analysis is independent. `glade check` and `glade test` can reuse an
exact result under `.glade/semantic/` from memory, disk, or build even when the
restored-runtime payload is bypassed. Exact source, companion metadata, schema,
dependency, analyzer and platform ABI, and option identity must match.

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

`glade test clear-cache` removes the startup and semantic caches for the
project. Test `--no-cache` bypasses the startup and semantic caches, including
semantic memory reuse, when debugging harness or analysis issues. A separately
running test server keeps its own memory until stopped or restarted.

## CI pattern

A small CI gate can check the project, run affected tests, then write JUnit
output for test reporting. Replace `<base-ref>` with the intended existing
comparison ref and ensure it is available in the checkout:

```bash
glade check --project . --json
glade test changed --project . --since <base-ref> --json --no-progress
mkdir -p reports
glade test --project . --junit reports/glade-junit.xml
```

Saved run artifacts and CI annotations are covered in [Add Glade to CI](/guide/ci-artifacts).

## Outcomes

Local test runs separate assertion failures from load errors, compile errors,
unsupported features, and internal errors. That split matters. A failing
assertion means the test ran and failed. An unsupported feature means
the runtime stopped at a known unsupported Salesforce API. Unsupported test
outcomes count as errors and exit with code `1`.

Read the JSON `summary` counts: `total`, `passed`, `failed`, `errors`, `skipped`,
and `unsupported`. A run with zero selected tests can exit with code `0`; it
does not provide test execution evidence. Run an explicit relevant test or
suite if an affected selection is empty.

```text
  ✓  RefinementServiceTest.opensFile  42ms
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
