---
pageType: guide
canonicalTask: /guide/workflows/apex-tests
---

# Run Apex tests

Run Apex tests locally before the org round trip. Keep Salesforce as the
validation gate where hosted behavior matters.

The git examples assume `origin/main` is your intended base ref and is available
locally. Substitute the correct existing ref for your repository before running
changed-test or refactor commands.

## Before you start

Run from a Salesforce DX project root. Make sure `glade doctor --project .` can find the project and
toolchain. Use committed fixtures for records, metadata, and mocks that the test
expects.

## Compile errors and test context

A compile error excludes the invalid class, its dependents, and triggers that
reference it. Unrelated tests still run. Excluded tests report `compile_error`;
`excludedClasses` also includes invalid non-test classes and triggers. The
command exits `1` while any exclusion remains, even if all executed tests pass.
Unattributed errors and invalid class metadata can still block the project.

The default running user uses `America/Los_Angeles`. Set `TimeZoneSidKey`
explicitly for inserted `System.runAs` users. See [local test outcomes and
limits](/guide/local-testing#running-user-and-limits).

## Steps

Run the full local suite:

```bash
glade test --project .
```

Run one class:

```bash
glade test --project . --class RefinementServiceTest
```

Run one method (replace the example names with a test in your project):

```bash
glade test --project . --class RefinementServiceTest --method opensFile
```

Split a larger suite with exact class lists or deterministic shards:

```bash
glade test --project . --class-file test-classes.txt
glade test --project . --shard-count 4 --shard-index 0
glade test --project . \
  --shard-count 4 \
  --duration-history .glade/test-durations.json \
  --write-class-shards reports/test-shards
glade test --project . --test-timeout 2m
```

Class files contain one exact class per line. Duration history balances later
shards; the assignment is deterministic for the same selected classes, shard
count, and history. `--write-class-shards` writes `shard-NNN.txt` class lists
and exits.

The per-test wall-clock cap defaults to **10 minutes** and also applies to
`@TestSetup`. Choose a Go duration such as `90s` or `15m` with
`GLADE_TEST_TIMEOUT`; `--test-timeout` takes precedence over the environment,
then the default. Invalid or negative settings are errors. `0` disables the
per-test cap. This policy also applies to daemon, watch, and `glade dev`
test/watch runs. CPU and governor limits are independent and unchanged.
Timeout failures name the applied setting and explain how to raise or disable it.
The VM checks cancellation every 64 Apex statements. A long native operation
(such as a SOQL/DML scan, JSON conversion, regex or sort) can run beyond the
cap before reaching the next cancellation check.

Keep an explicit cap in CI:

```bash
glade test --test-timeout 2m
```

For a longer local run or to disable the cap:

```bash
GLADE_TEST_TIMEOUT=15m glade test
GLADE_TEST_TIMEOUT=0 glade test
```

Run tests affected by a git ref:

```bash
glade test changed --project . --since origin/main
```

Rerun the last failures:

```bash
glade test failed --project .
```

Write JUnit for CI:

```bash
mkdir -p reports
glade test --project . --junit reports/glade-junit.xml
```

## When no tests run

Check `total`, `passed`, `failed`, `errors`, `skipped`, and `unsupported` in
`--json` output. An affected selection can exit `0` with zero tests; run an
explicit relevant class or suite before claiming execution evidence. The
`RefinementServiceTest.opensFile` example is from the editor walkthrough.
[Quickstart](/guide/quickstart#sample-project) owns the standalone sample.

## Expected output

The terminal prints selected, passed, and failed test counts. A failed assertion
or unsupported feature returns a non-zero exit code. JUnit output lands at the
path you pass with `--junit`.

## Common wrong turn

Do not treat a local pass as proof of every hosted side effect. Live callouts,
delivered email, exact governor accounting, and hosted service engines still
need Salesforce validation.

## Deeper reference

- [Detailed test runner guide](/guide/local-testing)
- [Test runner architecture](/guide/modules#test-runner)
- [Test startup cache](/guide/test-startup-cache)
- [Exit codes](/guide/exit-codes)
