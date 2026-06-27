# Run Apex tests

Run Apex tests locally before the org round trip. Keep Salesforce as the
validation gate where hosted behavior matters.

## Before you start

Run from an SFDX project root. Make sure `glade doctor` can find the project and
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

- [Test runner](/guide/modules/test-runner)
- [Test startup cache](/guide/test-startup-cache)
- [Exit codes](/guide/exit-codes)
