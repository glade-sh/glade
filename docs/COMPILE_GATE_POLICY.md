# Compile gate scope: analysis and policy proposal

Status: proposal for a product decision. This document changes no runtime
behavior. Measurements describe the current implementation; projected counts
below are estimates for the proposed policies, not executed improvements.

The recommended policy is conservative dependency scoping: run tests only when
errors can be assigned to known compile units or structured metadata dependencies.
Keep unresolved project errors visible and the command unsuccessful. Preserve
whole-project fallback when attribution is unavailable. Do not equate a partial
local run with a successful Salesforce deployment.

## Existing contract and remaining fallbacks

`glade test` excludes invalid source classes and their transitive dependents,
reports compile errors for their selected tests, and runs the other selected
tests. Exclusions without selected tests still make the command unsuccessful.
The model represents a local project with invalid units left out, rather than an
atomic deployment of every file. String literals, `Type.forName` strings, and
shadowing variables are boundaries, not static class dependencies.

Trigger units and references to a configured unavailable namespace already use
this scoping. `compile_gate_missing_dependency.go` matches resolver diagnostics
to structured dependency records and then binds namespace references. It does
not map arbitrary file-less errors by parsing their message text.

Whole-project fallback still occurs for:

- `GLADESEMA003`: an unknown relationship target produced by
  `sema.checkSchemaReferences`, with no source file attached.
- Class metadata errors, including unsupported `apiVersion`.
- Source that cannot be indexed as an attributable compile unit.
- Other unattributed diagnostics, unmatched dependency records, and source reads
  that fail during dependency analysis. These last paths are established by code
  inspection and existing unit boundaries, not all by complete CLI projects.
- Schema load failures, which become file-less `GLADESCHEMA001` errors before
  the gate can attribute source units.

Attribution failure at any subset rerun restores the original selected cases to
compile-error results. Earlier successful attribution does not protect the rest
of the project from a later unassignable diagnostic.

## Synthetic measurements

Each project selects five one-method test classes: `DependentTest` calls
`Broken.value()`, `IndependentTest` asserts a constant, `DynamicTest` uses
`Type.forName('Broken')`, `ShadowTest` uses a local variable named `Broken`, and
`DmlTest` inserts an Account and checks its count. A valid `Broken.value()`
returns 7. The invalid-class variant adds `Integer n = 'bad';` to that method.
The trigger variants attach a before-insert Account trigger calling either
`Broken.value()` or an absent helper. Package variants declare an unavailable
package in `sfdx-project.json`; the static-reference variant calls
`AbsentPkg.Tool.value()` from `Broken.value()`.

The lookup variant declares `Envelope__c.Target__c` as a lookup to the unknown
noncustom name `AbsentTarget`. None of the five test classes uses this object.
A custom target ending in `__c` is a separate boundary: `schema.LoadProject`
automatically adds a partial custom-object definition, so it does not produce
the same missing-reference diagnostic. Directly injecting a schema without that
placeholder does produce the diagnostic; that is a distinct index-entry route.

The metadata variants set only `Broken` to API 999.0 or use truncated object XML.
The unindexable variant replaces `Broken` with
`public class Broken { public static Integer value({`.

The same case shapes are exercised at source APIs 62.0, 67.0, and 68.0. Counts
are selected test methods, not excluded units or diagnostic counts.

| Project shape | Tests run / 5 | Compile-error tests | Excluded units | CLI exit |
| --- | ---: | ---: | ---: | ---: |
| Valid control | 5 | 0 | 0 | 0 |
| Invalid class | 4 | 1 | 2 | 1 |
| Invalid class referenced by trigger | 4 | 1 | 3 | 1 |
| Invalid trigger, valid helper | 5 | 0 | 1 | 1 |
| Missing package with static namespace reference | 4 | 1 | 2 | 1 |
| Missing package without a bound reference | 5 | 0 | 0 | 0 |
| Raw package version ID, no matching source reference | 5 | 0 | 0 | 0 |
| Missing lookup target, unrelated tests | 0 | 5 | 0 | 1 |
| Custom lookup placeholder boundary | 5 | 0 | 0 | 0 |
| Unsupported class API metadata | 0 | 5 | 0 | 1 |
| Malformed object XML | 0 | 5 | 0 | 1 |
| Unindexable source | 0 | 5 | 0 | 1 |

An empty configured dependency namespace is rejected during project loading,
before any test result is produced. That boundary is not a successful example
of scoping an unattributed dependency.

An excluded trigger does not prevent `DmlTest` from inserting its target object:
the test runs without that trigger and passes its count assertion. This proves
execution with reduced project contents; it does not prove native trigger
behavior or preservation of the original project's assertions.

An unavailable declared package with no bound references can disappear from the
test outcome entirely. The resolver still produced an error, but attribution
records it as handled even if it finds no source units. A package version ID or
package label is not evidence of the Apex namespace or provided schema. Treating
these strings as namespace identities cannot establish package independence.

## Cardinality check

A separate synthetic scale project contains 115 generated classes and 371
methods: 26 classes have four methods and 89 have three. Only the first class's
four methods call `Broken.value()`; all other methods assert a constant. These
sources and dependencies are invented for this measurement. The project does
not reproduce any external repository's dependency graph or expected outcomes.

| Scale shape | Passed / 371 | Compile-error methods | CLI exit |
| --- | ---: | ---: | ---: |
| Invalid `Broken` class | 367 | 4 | 1 |
| Unknown lookup target, all selected methods unrelated | 0 | 371 | 1 |
| Unsupported `Broken` API metadata | 0 | 371 | 1 |
| Missing raw package ID, no bound references | 371 | 0 | 0 |

All four outcomes match across source APIs 62.0, 67.0, and 68.0. The unknown
lookup example is intentionally a whole-project gate despite zero test
references to its object. The raw-ID example demonstrates loss of a project
error, not package availability or a native-equivalent passing project.

## Routes and native evidence limits

The runner experiment uses `RunCasesContext` with disk caching disabled. CLI
measurements use `glade test --json --no-cache --no-serve --read-only` and
`glade exec --json`, with separate independent and helper-calling anonymous
bodies. Named `@IsTest` methods run through the project test runner.

Anonymous execution analyzes the supplied body and registers the project runtime;
it does not invoke `runWithCompileGate`. An independent body can therefore run
where all named tests receive a project compile error. A helper-call failure may
appear at execution rather than as the named test runner's dependent compile
error. Extending test scoping alone would not align these routes.

| Shape | Independent anonymous body | Anonymous `Broken.value()` call |
| --- | --- | --- |
| Invalid class or invalid helper behind a trigger | Runs | Runtime assignment error |
| Missing static package member | Runs | Unsupported-call error |
| Unknown lookup target | Runs | Runs |
| Unsupported class API metadata | Runs | Runs |
| Unindexable source | Runs | Unknown-method compile error |
| Malformed object XML | Load error | Load error |

These outcomes are identical across the three measured source APIs. The matrix
contains 117 CLI invocations: 39 project selections and 78 anonymous bodies.
Three project selections reject invalid configuration before producing test
results. The rows are observations of Glade, not assertions of native fidelity.

LSP semantic analysis and diagnostic publication also do not execute the compile
gate. `Handler.PublishDiagnostics` drops diagnostics with no file. A project may
therefore block every selected test while publishing no diagnostic for its
file-less project error. This is a presentation gap, not a separate count of
recovered tests. Any future attribution change must preserve source identity for
LSP and the CLI, rather than inventing a location from diagnostic prose.

Existing native exports in
`internal/apextest/testdata/conformance/compile_gate.json` include:

- `GATE2-07T`: check-only deployment rejects an invalid class and does not run a
  requested independent test at APIs 62.0 and 67.0.
- `GATE2-14F`: invalid class API metadata rejects the deployment before tests.
- `G08` and `G12`: string/dynamic references do not create the static dependency.

The `dependency_gate_{baseline,signature,packages,transition}.json` exports cover
204 historical native observations across APIs 62.0, 67.0, and 68.0. Their
conformance adapter asserts 156 observations and explicitly carries 48; passing
that adapter cannot be called 204 matched cells. Namespace resolver attribution
is local policy, not a Salesforce diagnostic contract. Saved-state invalidation,
lazy recompilation, coverage-only observations, and metadata transitions remain
separate from a fresh deployment. The 48 carried observations comprise 30 named
validation, 12 lazy-recompilation, three coverage-only, and three metadata
transition observations; their local lifecycle equivalents are not established.

No new Salesforce captures were made for this analysis. Existing exports are
historical observations, not a same-release recapture. New schema-relationship
scoping and trigger-dependent DML behavior still require native captures before
any semantic fix, with `org.tsv` and `ORG-RUN.txt`, acceptance/rejection twins,
unrelated-object boundaries, anonymous and named routes, and APIs 62/67/68.

## Policy options and projected counts

Let N be selected methods, C the methods in the transitive invalid class closure,
and M the methods depending on invalid metadata. Counts describe eligibility to
run; passing still depends on each test's runtime behavior. Overlapping closures
must be unioned before counting.

| Option | Scope | Five-test class/trigger example | Five-test unused invalid lookup | Fidelity and tradeoff |
| --- | --- | --- | --- | --- |
| Atomic project compilation | Any error blocks everything | 0 run | 0 run | Matches a failed all-source native deployment; discards the existing partial-run contract. |
| Existing dependency policy | Source/namespace binding, remaining errors global | 4 run (5 for trigger-only error) | 0 run | Useful local projection; triggers may be removed and unreferenced resolver errors may vanish. |
| Conservative structured scoping (recommended) | Preserve existing class closure; scope only proven metadata dependencies; unknown ownership remains global | 4 eligible; trigger DML needs a separate captured rule | Up to 5 eligible, with project failure retained | Gains N minus union(C, M) only when attribution is trustworthy; preserves explicit incompleteness and existing metadata rejection boundaries. |
| Best-effort file scoping or ignore file-less errors | Drop anything that cannot be bound | Up to 4 or 5 run | 5 run | Maximum apparent throughput; risks executing invalid routes, dropping package requirements, and creating a false successful result. |

A structured lookup example with one dependent test and four unrelated tests
would permit 4, rather than 0, methods under the recommendation. The measured
unused-lookup example could permit 5. Both are projections awaiting native facts
and a policy decision. Unsupported API metadata and unindexable source stay at
0 under the conservative option. In the 371-method synthetic project, proven
unused lookup ownership would make up to 371 methods eligible instead of zero,
while retaining a failing project result. The invalid-class variant remains at
367 eligible methods. Unidentified package contracts instead retain a project
failure and whole-project fallback: that raw-ID variant would have zero eligible
methods, compared with the current 371 apparent passes. Declaring zero matched
references must not certify that the dependency is unused.

For trigger exclusions, the existing local projection may be retained explicitly,
or target-object/event DML could be treated as an additional dependency after
capture. In the measured invalid-helper project, blocking `DmlTest` too would
reduce eligible methods from 4 to 3. In the trigger-only project it would reduce
5 to 4. Dynamic DML, flows, and cascading triggers make an incomplete static DML
scan unsafe. The recommendation does not claim that such a scan is sufficient.

## Recommended decision

Keep dependency-scoped partial execution as the default local test policy.
Amend it only for diagnostics with structured, testable ownership. Retain every
unresolved project error and a nonzero result even when some tests execute.
Preserve existing whole-project rejection for metadata-version errors and cases
without trustworthy ownership. Do not silently bypass missing contracts, infer
package namespaces from version IDs, or remove triggers and describe the result
as native-equivalent execution.

Approve a capture-first investigation of lookup ownership and trigger/DML
boundaries separately from implementing either rule. A follow-up regression
must compile and fail in an earlier test-only commit, then cover the unit path
and a complete synthetic project. Cover the CLI anonymous, named `@IsTest`,
selected test runner, and LSP publication routes at APIs 62/67/68, including a
rejection twin and a must-not-apply boundary. Keep existing deployment behavior
and native row assertions intact unless a new native row justifies changing them.

An aggregate blocked-test count is only an upper bound on candidates. Removing a
first missing-dependency diagnostic does not clear simultaneous semantic errors,
prove package availability, or establish that any method passes. Count recovery
only from an exact-candidate advisor rerun after legitimate prerequisites and the
remaining diagnostics have been resolved. This proposal makes no recovery claim.

## Reproducing the relevant checks

Use Go 1.26.10 with CGO enabled. Focused Go commands use `GOMAXPROCS=2`,
`GOMEMLIMIT=2GiB`, `GOTOOLCHAIN=local`, and `nice -n 10`. The local CGO binary
build uses `GOMAXPROCS=1`. No whole-package or repository test suite is needed
for this document.

```sh
go test ./internal/apextest -run '^(TestDependencyGateMissingDependency.*|TestCompileGateExcludesTriggerReferencingExcludedClass|TestCompileGateRerunScopesTriggerDependency)$' -count=1 -timeout=170s
# Repeat this conformance selection with 67[.]0 and 68[.]0.
go test ./internal/apextest -run '^TestDependencyGate(Baseline|Signature|Packages|Transition)Conformance$/^62[.]0$' -count=1 -timeout=170s
go test ./internal/gladecli -run '^TestRunTestDependencyScopedCompileGate$' -count=1 -timeout=170s
go test ./internal/lsp -run '^(TestBuildPublishDiagnosticsConvertsRanges|TestPublishTestDiagnosticsUsesProblemStackFrames)$' -count=1 -timeout=170s
glade test --project <synthetic-project> --json --no-cache --no-serve --read-only
glade exec --project <synthetic-project> --json 'System.assertEquals(7,7);'
glade exec --project <synthetic-project> --json 'System.assertEquals(7,Broken.value());'
```

The exploratory Go overlay harnesses log observed counts; their successful Go
test exits mean that measurement completed, not that the blocked cases meet a
new policy. There is no fail-before/pass-after implementation pair: no behavior
was changed and no regression assertion was added or weakened.
