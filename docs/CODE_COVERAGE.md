# Code Coverage

`glade test --code-coverage` records Apex code coverage the way Salesforce
counts it: which lines of each class and trigger the selected tests ran. The
report follows `sf apex run test --code-coverage`. Each test gets its own
per-class rows, and the run gets per-component line arrays and two weighted
percentages. `--coverage-gate` applies the 75% deployment rule to the result.

Every rule on this page comes from native rows captured at API 62.0 and 67.0
(`internal/apextest/testdata/native_conformance/test_runner_coverage`). The
two versions agree on every row.

## Commands

```bash
glade test --code-coverage
glade test --code-coverage --json
glade test --class InvoiceServiceTest --code-coverage --junit reports/junit.xml
glade test --coverage-gate RunLocalTests
glade test --coverage-gate RunSpecifiedTests --class-file tests.txt
```

- `--code-coverage` records coverage. Without it the run records nothing and
  its output has no coverage block, as with `sf apex run test`.
- `--coverage-gate <RunLocalTests|RunSpecifiedTests>` implies
  `--code-coverage` and applies that test level's deployment rule. A failed
  gate fails the run (exit code `1`, JSON status `failed`), like a failed test.

Both flags work with every `glade test` route: a local run, `--daemon`, a
running test server (`--connect` or auto-connect), `--watch`, class shards and
selectors. The routes return the same coverage.

## What counts

A class or trigger is measured when it is project source, not a test class and
not from a managed package dependency. An `@IsTest` class has no coverage, and
neither does a public helper declared inside one. Interfaces and enums have no
executable lines.

Executable lines:

- Every statement's first line. A statement over several lines also counts
  each line that holds a non-constant operand. A constant string
  concatenation counts only its first line.
- Method and constructor name lines, covered when the method is entered.
- Field lines with an initializer, covered when the class initializes: static
  fields when the class is first used in a test, instance fields when an
  instance is constructed.
- Property declaration lines, with or without accessor bodies, covered when
  the property is read or written.
- `try {`, `catch (...)` and `} while (...)` lines, and each `when` label line
  except `when else`. A `when` line is covered when the switch tests it.
- The line that closes the query of a SOQL `for` loop, covered on each
  iteration.

Not executable: braces, class headers, `static {`, `do {`, `} else {`,
`finally`, `when else`, blank lines, comments, field declarations without an
initializer, and a statement that is only a `System.debug` call, including its
argument lines.

Short-circuit operands and ternary branches have their own lines: the line of
`&& b` stays uncovered when `a` is false, and the line of the branch a
ternary does not take stays uncovered.

## How tests add up

- A test's coverage includes its `@TestSetup` method's. A helper called only
  from `@TestSetup` is covered, and a trigger fired by a setup insert counts.
- A failed or throwing test keeps the lines it ran.
- Class and run coverage is the union of distinct lines. A line two tests run
  is one covered line.
- Every trigger in the project is listed for every test and in the run, with
  zero lines covered when no test fired it.

## Percentages

| Figure | Scope | Rounding |
| --- | --- | --- |
| `coveredPercent` | one class or trigger | nearest percent, halves up |
| `testRunCoverage` | the components the run lists | nearest percent, halves up, weighted by lines |
| `orgWideCoverage` | every class and trigger in the project | floored, weighted by lines |
| gate `percent` | every class and trigger in the project | floored, weighted by lines |

Weighted means total covered lines over total executable lines. The mean of
class percentages is a different number and is never used. A component with
no executable lines reports 0%.

## Deployment gates

`RunLocalTests` (and `RunAllTestsInOrg`) requires:

- floor(org-wide percent) of at least 75. 74.5% reports 74 and fails with
  `Average test coverage across all Apex Classes and Triggers is 74%, at least 75% test coverage is required.`
- one covered line in every trigger. A trigger at 0.33% passes. A trigger
  with no covered line fails even when the org-wide figure passes:
  `Test coverage of selected Apex Trigger is 0%, at least 1% test coverage is required`.

`RunSpecifiedTests` requires each class to reach 75% on its own, whatever the
org total: `Test coverage of selected Apex Class is 15%, at least 75% test coverage is required`.
Glade treats the project as the deployment, so every project class with
executable lines is checked. Gate warnings name the class or trigger first.

## Output

### Console

The summary follows the test results:

```text
Code coverage: 45% of 38 lines in this run (17 covered), 11% org-wide
  CvShapeA   50%  17/34  uncovered 6,7,8,10,14,15,35,39,40,41,43,44,+5
  CvTrig      0%  0/4  uncovered 2,3,4,6
Coverage gate RunLocalTests: failed (11%)
  Average test coverage across all Apex Classes and Triggers is 11%, at least 75% test coverage is required.
```

### JSON

`--json` adds `data.coverage` to the run and `coverage` to each test case.
Line arrays list line numbers, not hit counts.

```json
{
  "coverage": {
    "summary": {
      "totalLines": 38, "coveredLines": 17, "testRunCoverage": 45,
      "orgWideTotalLines": 146, "orgWideCoveredLines": 17, "orgWideCoverage": 11
    },
    "components": [
      {
        "name": "CvShapeA", "type": "class", "file": "force-app/main/default/classes/CvShapeA.cls",
        "totalLines": 34, "totalCovered": 17, "coveredPercent": 50,
        "coveredLines": [2, 4, 18], "uncoveredLines": [6, 7, 8]
      }
    ],
    "gate": {
      "testLevel": "RunLocalTests", "percent": 11, "passed": false,
      "warnings": ["Average test coverage across all Apex Classes and Triggers is 11%, at least 75% test coverage is required."]
    }
  }
}
```

Each case carries its own rows:

```json
"coverage": [
  {"name": "CvShapeA", "type": "class", "numLinesCovered": 17, "numLinesUncovered": 17, "coveredLines": [2, 4, 18]}
]
```

A test's uncovered lines are the component's executable lines (its
`coveredLines` and `uncoveredLines` in `data.coverage.components`) less the
test's `coveredLines`. Per-test rows omit them to keep large runs small.

The envelope `summary` gains `coverageGateFailed: true` when a gate fails.

Mapping to `sf apex run test --code-coverage --json`:

| sf field | glade field |
| --- | --- |
| `coverage.coverage[].name`, `totalLines`, `totalCovered`, `coveredPercent` | `coverage.components[]` same names |
| `coverage.coverage[].lines` (line to 1 or 0) | `coveredLines`, `uncoveredLines` |
| `coverage.records[]` (per test method) | `suites[].cases[].coverage[]` |
| `coverage.summary.testRunCoverage`, `orgWideCoverage` (`"45%"`) | `coverage.summary.testRunCoverage`, `orgWideCoverage` (`45`) |

### JUnit

As with sf, JUnit carries only the two percentages: each `<testsuite>` gets
`<properties>` with `testRunCoverage` and `orgWideCoverage` (for example
`45%`). There is no per-line data in JUnit.

## One run against a fresh org

Salesforce stores coverage between runs: aggregates only grow, a rerun of a
test class replaces that class's per-test rows, and the sf summary reads the
stored aggregate. `glade test` reports one run, as against a freshly deployed
org: the components' lines are the union over this run's tests, and
org-wide counts every project class and trigger with this run's coverage
alone. `internal/apexcov` models the stored-row rules (`apexcov.Store`), and
the conformance test replays the captured run sequence through it.

## Known gaps

These shapes are not captured yet, or Glade does not match them:

- **CV5 edit invalidation.** A redeploy resets a changed class's stored
  coverage natively. Glade keeps no coverage between runs, so there is
  nothing to invalidate; the native reset shape is not captured.
- **CV6** non-Apex deployment defaults and deployment test identifiers,
  **CV7** package selection and package denominators, and **CV9** unit versus
  integration-test coverage are not captured. Managed package classes are
  excluded from every figure.
- **Async work enqueued in `@TestSetup`.** Natively a Queueable enqueued in
  setup runs at the end of setup and its lines count. `glade test` does not
  run async work enqueued in `@TestSetup`, so those lines stay uncovered.
- **Per-test rows of a multi-method run.** Natively, per-test rows in a run
  of several methods carry method-signature lines over from earlier methods.
  Glade's per-test rows match the native rows of solo runs.
- **One run listing.** One native two-method run listed no uncalled trigger;
  every other captured run lists them. Glade always lists every trigger.
- **Partly executed statements.** A statement covers all of its non-lazy
  lines when it starts. A multi-line statement that throws before reaching a
  later operand line still counts that line. No row covers this shape.
- **Field initializers** count all their lines when the class initializes;
  a short-circuit operand inside an initializer is not tracked separately.
- **RunSpecifiedTests and triggers.** The per-class rule is applied to
  classes only; no row shows how Salesforce gates a trigger under
  RunSpecifiedTests. A trigger with no executable lines is not gated.
- **Store Only Aggregate Code Coverage** (an org setting) has no local
  equivalent.

The conformance tests are `internal/gladecli/test_coverage_native_conformance_test.go`
(the replay, fresh-org run summaries and daemon equivalence) and
`internal/apexcov/native_conformance_test.go` (line maps, unions, percentages
and the nine deployment gates).
