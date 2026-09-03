# Run Apex tests

Run Apex tests locally before the org round trip. Keep Salesforce as the
validation gate where hosted behavior matters.

## Before you start

Run from a Salesforce DX project root. Make sure `glade doctor --project .` can find the project and
toolchain. Use committed fixtures for records, metadata, and mocks that the test
expects.

## Steps

Run the full local suite:

```bash
glade test --project .
```

Run one class:

```bash
glade test --project . --class RefinementServiceTest
```

Run one method:

```bash
glade test --project . --class RefinementServiceTest --method testRefinesFileRow
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
and exits. The default per-test timeout is five minutes.

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
