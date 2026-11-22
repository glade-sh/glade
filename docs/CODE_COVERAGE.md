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
glade test --class RefinementServiceTest --code-coverage --junit reports/junit.xml
glade test --coverage-gate RunLocalTests
glade test --coverage-gate RunSpecifiedTests --class-file tests.txt
```

- `--code-coverage` records coverage. Without it the run records nothing and
  its output has no coverage block, as with `sf apex run test`.
- `--coverage-gate <RunLocalTests|RunSpecifiedTests>` implies
  `--code-coverage` and applies that test level's deployment rule. A failed
  gate fails the run (exit code `1`, JSON status `failed`), like a failed test.
- `--synchronous` reports coverage the way `sf apex run test --synchronous`
  does: every trigger in the project is listed in the run and in every test's
  rows, empty when no test fired it. Without it the report follows the default
  asynchronous `sf apex run test`, which lists only what the tests reached.
  `testRunCoverage` differs between the two by the uncalled triggers' lines.
  The flag does not change how tests run, and it needs `--code-coverage` or
  `--coverage-gate`.

Both flags work on every `glade test` route: a local run, `--daemon`, and a
running test server (`--connect` or auto-connect). The routes return the same
coverage.

- `--code-coverage` also works with selectors, class shards and `--watch`. The
  org-wide figure then counts only that run's coverage: a shard's org-wide
  percent is shard-local, and shards are not merged.
- `--coverage-gate RunLocalTests` needs every local test. It refuses
  `--class`, `--method`, `--class-file`, `--filter`, `--shard-count`,
  `--changed-since`, `--last-failed`, `--watch` and `--write-class-shards`.
- `--coverage-gate RunSpecifiedTests` applies to a selection; that is its
  meaning.

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

- `@TestSetup` coverage goes to the first test of the class in method name
  order that ran. A helper called only from `@TestSetup` is covered, and a
  trigger fired by a setup insert counts. Later tests of the class see only the
  setup's carried declaration lines.
- A failed or throwing test keeps the lines it ran. A method that never ran
  (a compile error, a skipped case, or a method a failed `@TestSetup` kept
  from running) has no per-test rows and adds no lines.
- Class and run coverage is the union of distinct lines. A line two tests run
  is one covered line.
- With `--synchronous`, every trigger in the project is listed in the run and
  in every test's rows, with zero lines covered when no test fired it. In the
  default (asynchronous) report a trigger is listed only once a test reached it.
- Async work a `@TestSetup` method enqueues (Queueable, `@future`, batch,
  scheduled Apex) runs at the end of setup, the way `Test.stopTest` runs it. Its data is in
  every test method's org, its lines count, and the test method's
  `Limits.getQueueableJobs()`, `getFutureCalls()` and `getAsyncCalls()` start
  from the setup's counts. A job that throws, or a Queueable that chains
  another (`System.AsyncException: Maximum stack depth has been reached.`),
  fails setup: every test method fails with that exception and the class has
  no per-test rows. A setup's `@future` call leaves no `AsyncApexJob` row for
  the tests; Queueable and batch rows stay. A setup that calls
  `Test.startTest`/`stopTest` itself runs its work there and passes zero
  counts to the tests. Work those jobs queue runs too: a Queueable that a
  batch `finish()` enqueues runs before the tests. A scheduled job runs once;
  its `CronTrigger` stays and its `AsyncApexJob` row reads `Queued`. A
  platform event published in setup is not delivered before the tests (call
  `Test.getEventBus().deliver()` in setup to deliver it), and change data
  capture events from setup DML are not delivered.

Per-test rows carry declaration lines, as Salesforce stores them. Within a
test class, in method name order by code point (`Alpha`, `Gamma`, `Zeta`,
`apple`, `beta`, `delta`):

- a test has a row for every class an earlier test reached, even when it did
  not reach that class itself;
- a row holds the test's own lines plus the method, constructor and property
  declaration lines that earlier tests covered. Statement lines are never
  carried.

Carry stays inside one test class, also when one asynchronous run holds
several test classes, with parallel testing on or off. A rerun of one method
starts a new carry. So a test's row can show a method name line covered that
the test never called. The run totals are unaffected: they are the union of
lines, and every carried line was covered by an earlier test.

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
test's `coveredLines`. Per-test rows omit them to keep large runs small. Each
component also lists `declarationLines`, the lines that carry between rows.

The test daemon and server send compact rows (each test's own lines for what
it reached); the JSON writer adds the carried lines and trigger rows, so every
route prints the same rows.

The envelope `summary` gains `coverageGateFailed: true` when a gate fails.

Mapping to `sf apex run test --code-coverage --json`:

| sf field | glade field |
| --- | --- |
| `coverage.coverage[].name`, `totalLines`, `totalCovered`, `coveredPercent` | `coverage.components[]` same names |
| `coverage.coverage[].lines` (line to 1 or 0) | `coveredLines`, `uncoveredLines` |
| `coverage.records[]` (per test method) | `suites[].cases[].coverage[]` |
| `coverage.summary.testRunCoverage`, `orgWideCoverage` (`"45%"`) | `coverage.summary.testRunCoverage`, `orgWideCoverage` (`45`) |

### JUnit

As with sf, JUnit carries the two percentages: each `<testsuite>` gets
`<properties>` with `testRunCoverage` and `orgWideCoverage` (for example
`45%`). With `--coverage-gate` it also gets `coverageGate`
(`RunLocalTests:failed`) and one `coverageGateWarning` per warning, which sf
does not have, so a CI tool that reads only JUnit sees a failed gate. There is
no per-line data in JUnit.

## One run against a fresh org

Salesforce stores coverage between runs: aggregates only grow, a rerun of a
test class replaces that class's per-test rows, and the sf summary reads the
stored aggregate. `glade test` reports one run, as against a freshly deployed
org: the components' lines are the union over this run's tests, and
org-wide counts every project class and trigger with this run's coverage
alone. `internal/apexcov` models the stored-row rules (`apexcov.Store`), and
the conformance test replays the captured run sequence through it.

## Known gaps

The following differences remain outside the current coverage model. Native
observations and uncaptured behavior are distinguished below; an uncaptured
shape is not a claim about Salesforce behavior.

- **CV5 edit invalidation.** Out of scope because `glade test` reports one run
  and does not persist coverage to invalidate after an edit. A native reset on
  redeploy has not been captured.
- **CV6 deployment defaults and test identifiers.** Out of scope because the
  coverage gate models a project test run, not non-Apex deployments or
  Salesforce deployment test identifiers. These defaults are not captured.
- **CV7 package selection and denominators.** Out of scope because the gate
  treats the project as the deployment and excludes managed package dependencies;
  package-specific selection and denominators are not captured.
- **CV9 unit and integration coverage.** Out of scope because Glade does not
  maintain separate unit and integration coverage totals; this distinction is
  not captured.
- **Synchronous and asynchronous APIs.** Differences beyond `--synchronous`,
  such as setup-row `testsRan` counts, are out of scope because the flag selects
  a coverage reporting shape rather than a Salesforce test API. Those differences
  are not modeled.
- **Empty classes.** Listing untouched 0/0 classes is out of scope for the
  run report, which lists reached classes rather than reproducing all Tooling
  aggregate rows. A reached empty class is listed at 0/0 in the captures at
  both APIs; omitting an untouched empty class does not change percentages.
- **Partly executed statements.** Operand-by-operand tracking before a throw
  is out of scope for the current statement-level instrumentation. Glade counts
  all non-lazy lines when the statement starts; this shape has no native row.
- **Field initializer short-circuit operands.** Separate lazy-operand tracking
  is out of scope for the current initializer instrumentation. Glade counts all
  initializer lines when the class initializes; this shape has no native row.
- **RunSpecifiedTests and triggers.** Trigger gating is out of scope for the
  per-class gate because there is no captured row establishing its rule. Glade
  applies that gate to classes only and does not gate an empty trigger.
- **Store Only Aggregate Code Coverage.** Out of scope because Glade reports
  per-test coverage and has no equivalent org setting; the setting's behavior
  has not been captured.

The conformance tests are `internal/gladecli/test_coverage_native_conformance_test.go`
(the step-by-step replay, the 14-method carry-over run in
`test_runner_coverage_suite`, the carry, setup and trigger-listing rows in
`test_runner_coverage_carry`, the setup async rows in
`test_runner_coverage_setup_async`, fresh-org run summaries and daemon
equivalence) and
`internal/apexcov/native_conformance_test.go` (line maps, unions, percentages
and the nine deployment gates).
