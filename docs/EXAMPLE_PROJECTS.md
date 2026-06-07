# Example Project Compatibility Harness

The `glade compat examples` command inventories local Salesforce-shaped projects and reports
what Glade supports, what is unsupported, and what blocks progress.

## Running the Harness

Scan a single project:

```bash
glade compat examples --project path/to/project
```

Scan multiple projects:

```bash
glade compat examples --project path/to/project-a --project path/to/project-b
```

Output as JSON:

```bash
glade compat examples --project path/to/project --json
```

Write to a file:

```bash
glade compat examples --project path/to/project --output example-report.json
```

Check an existing report for drift:

```bash
glade compat examples --project path/to/project --check example-report.json
```

## Report Format

The machine-readable report contains, per project:

- **Project metadata**: name, root path, source layout type (sfdx, legacy, mixed).
- **Asset counts**: Apex classes, triggers, test classes, objects, fields, Visualforce
  pages/components, Aura/LWC components, workflows, flows, static resources, etc.
- **Apex constructs**: classes, interfaces, enums, annotations, sharing modes, async
  interfaces used.
- **Runtime usage**: SOQL features, DML features, trigger operations, and stdlib
  namespace references observed in source.
- **Diagnostics**: grouped by category:
  - `observed-blocker` — prevents parse/check/test progress.
  - `observed-runtime-gap` — code reaches unsupported runtime behavior.
  - `observed-parity-gap` — controlled result differs from Salesforce.
  - `unobserved-parity-followup` — not needed for examples yet.
- **Top blockers**: capabilities with the most occurrences and affected files.
- **Surfaces**: unsupported platform surfaces found by the gap scanner.

## Example Projects

The `example-projects/` directory contains local compatibility projects used to
derive the support plan. Run the harness against them:

```bash
for dir in example-projects/*; do
  echo "--- $(basename $dir) ---"
  go run ./cmd/glade compat examples --project "$dir" --json | jq '.projects[0].counts'
done
```

## Running Apex Tests Locally

For day-to-day use in your own project, start with
[LOCAL_TESTING.md](LOCAL_TESTING.md). This page keeps the example-project
corpus and compatibility harness details close at hand.

Use `glade test` when you want the local developer test runner shape for a
single Salesforce project. It runs from source on disk and does not require a
Salesforce org:

```bash
go run ./cmd/glade test --project example-projects/src-nmb-nutpl-develop --json
go run ./cmd/glade test --project example-projects/src-nmb-nutpl-develop --filter MyTestClass --json
go run ./cmd/glade test --project example-projects/src-nmb-nutpl-develop --filter MyTestClass.testMethod --json
```

To run tests selected from changed-file dependencies:

```bash
go run ./cmd/glade test \
  --project example-projects/src-nmb-nutpl-develop \
  --changed-since origin/main \
  --json
```

For compatibility triage, prefer `compat local-tests`. It reports outcomes as
`pass`, `fail`, `unsupported`, `loadError`, `compileError`, or
`internalError`, and can cap large-project runs by distinct blocker groups:

```bash
go run ./cmd/glade compat local-tests \
  --project example-projects/src-nmb-nutpl-develop \
  --timeout 30000 \
  --parallel auto \
  --top-failures 8 \
  --json
```

Default full-project runs now auto-tune class parallelism. For most local runs,
start with:

```bash
go run ./cmd/glade compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --parallel auto \
  --json
```

When running in sharded CI, use auto shard selectors with env wiring:

```bash
GLADE_SHARD_COUNT=6 GLADE_SHARD_INDEX=2 \
go run ./cmd/glade compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --parallel auto \
  --shard-count auto \
  --shard-index auto \
  --duration-history /path/to/perf.json \
  --json
```

Focused class or method run:

```bash
go run ./cmd/glade compat local-tests \
  --project example-projects/sf-cred-pkg-develop \
  --class AccountsTriggerHandlerTest \
  --method testSomeBehavior \
  --json
```

Affected compatibility run:

```bash
go run ./cmd/glade compat local-tests \
  --project example-projects/sf-cred-pkg-develop \
  --changed-since origin/main \
  --parallel auto \
  --json
```

For `compat local-tests`, use `--parallel <n|auto>` for class workers. For
day-to-day `glade test`, use `--parallelism <n>`; method-level parallelism is on
by default there.

Large-project blocker triage:

```bash
go build -o /tmp/glade ./cmd/glade
/tmp/glade compat local-tests \
  --project example-projects/sf-cred-pkg-develop \
  --blockers-only \
  --top-failures 20 \
  --max-failure-groups 20 \
  --timeout 20000 \
  --parallel 4 \
  --json
```

Use the checked owned-corpus baseline as a fast confidence gate:

```bash
go run ./cmd/glade compat local-tests --check docs/fixtures/local-tests-corpus.json --json
```

## Phase Gate

Current status as of 2026-06-07:

- The server-example execution harness is green across the checked
  `example-projects` corpus: `pass=101 fail=0 unsupported=0 missing=0`.
- The owned local-test corpus baseline is green via
  `go run ./cmd/glade compat local-tests --check
  docs/fixtures/local-tests-corpus.json --json`.
- A full `example-projects` post-parity inventory is green as a scanner and
  readiness gate. It is not the same as proving every example-project Apex test
  runs end to end.
- Current release-hardening dogfood runtime gates are green for
  `sf-cred-pkg-develop` (`total=4565 pass=4565`), `src-nmb-nu-develop`
  (`total=11526 pass=11526`), and `nams-workspace`
  (`total=5723 pass=5723`). `src-nmb-nutpl-develop` remains the fast runtime
  sentinel at `total=761 pass=761`.
- Full runtime support for all six example projects is not complete yet. NPSP
  and `src-nmb-nc-develop` remain separate frontier gates unless freshly rerun.
  Historical six-project baseline detail is tracked in
  `docs/fixtures/local-tests-example-projects.json`.
- `glade compat examples`, `glade compat server-examples`, and
  `glade compat post-parity` are separate gates. The zero post-parity inventory
  means no current scanner/test-readiness blockers are known for the checked
  example projects; it is not the same as proving every example-project Apex
  test runs end to end.

Phase 0 is complete when:

1. `glade compat examples` produces a stable report for each example project.
2. The report counts match manual inspection.
3. No panic occurs during scan or check.
4. Reduced compatibility fixtures cover observed selector, trigger, controller,
   HTTP mock, and REST patterns.
