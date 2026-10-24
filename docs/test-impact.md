# Per-test impact traces

`glade test --impact-json <path>` writes a versioned JSON artifact alongside
normal test results. It runs the local one-shot test runner, honors class,
method and shard selection, and records passing and failing tests. It works
with `--read-only` when the artifact is outside the project. New artifact
directories use mode 0700 and new files use mode 0600.

Each `tests` entry identifies its class, method, source file and status. Its
sorted, deduplicated `sourceFiles`, `classes`, `methods` and `operations`
describe observed execution. Paths are relative to the project root, with
forward slashes; dependencies outside that root retain `../` components.
Operations include observed DML, SOQL and describe activity. Arguments, record
values, query text and source contents are omitted.

`sharedInputs` applies to every test. It conservatively includes indexed Apex
sources, their companion metadata, loaded project metadata, dependency
artifacts, schema snapshots and configuration paths. Org shape inputs include
`config/project-scratch-def.json`, `config/hc-project-scratch-def.json`,
`cumulusci.yml`, `cumulusci.template.yml` and the `config_file` JSON paths
discovered by the project loader in either CumulusCI file. These paths are
included for project, dependency and shim roots. Configuration and Apex
companion paths may be listed when absent, because adding them can change
behavior. Compilation and shared initialization consume inputs before a test
method runs; the artifact does not attribute those inputs to one method.

The top-level `inputInventoryComplete` field is explicitly `false`, even for
an empty report. This inventory is partial: inherited configuration, runner
data reads and negative dependencies are not exhaustively recorded. Every
reported test also carries `project-input-inventory-incomplete` in its
`fallbackReasons`. A loaded project or an untruncated trace does not establish
inventory completeness. Consumers **must run the full suite** when this field
is false or absent, including when consuming artifacts from older versions.
The current implementation never sets it to true; these artifacts cannot yet
authorize skipping tests.

Tests with other `fallbackReasons` also require conservative selection. For example,
`trace-truncated` means the runner reached its detailed-event bound, and
`test-not-completed` covers compile, cancellation and unsupported outcomes.
An empty artifact after a selector error establishes no execution coverage.

Consumers must run all tests when shared inputs change, the input inventory
changes, a trace is incomplete, or the runtime binary changes without an
independently validated mapping to execution keys. Observed method sets alone
do not prove that skipping a test is safe: static field access, setup, dynamic
lookup, negative dependencies and native dispatch are not exhaustively traced.
This flag records evidence; it does not change the test selector or provide
a mapping from Go changes to native/stdlib methods or semantic diagnostics.

Collection reuses the existing bounded VM trace and test isolation. It adds
trace/profile allocation and CPU cost during execution, then reduces events to
sets. Normal results omit these internal events unless `--trace`,
`--trace-blockers` or slow-test profiling independently requests them.
`BenchmarkImpactJSONTestRunner` measures an owned synthetic project with
collection disabled/enabled and reports output size and allocations. Results
depend on the project; tight loops can hit the trace bound. The runner retains
each test's bounded events until reduction, so a small final artifact does not
bound collection memory for a large suite.

`--impact-json` rejects connect, daemon, watch, debug, wizard, last-failed
and shard-plan-only modes. Those paths either execute elsewhere or can return
before running tests. Artifact destinations must be distinct from other
reports and duration history.
