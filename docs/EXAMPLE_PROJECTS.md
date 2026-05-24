# Example Project Compatibility Harness

The `oaer compat examples` command inventories local Salesforce-shaped projects and reports
what OAER supports, what is unsupported, and what blocks progress.

## Running the Harness

Scan a single project:

```bash
oaer compat examples --project path/to/project
```

Scan multiple projects:

```bash
oaer compat examples --project path/to/project-a --project path/to/project-b
```

Output as JSON:

```bash
oaer compat examples --project path/to/project --json
```

Write to a file:

```bash
oaer compat examples --project path/to/project --output example-report.json
```

Check an existing report for drift:

```bash
oaer compat examples --project path/to/project --check example-report.json
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
  go run ./cmd/oaer compat examples --project "$dir" --json | jq '.projects[0].counts'
done
```

## Running Apex Tests Locally

Use `oaer test` when you want the local developer test runner shape for a
single Salesforce project:

```bash
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --json
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --filter MyTestClass --json
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --filter MyTestClass.testMethod --json
```

For compatibility triage, prefer `compat local-tests`. It reports outcomes as
`pass`, `fail`, `unsupported`, `loadError`, `compileError`, or
`internalError`, and can cap large-project runs by distinct blocker groups:

```bash
go run ./cmd/oaer compat local-tests \
  --project example-projects/src-nmb-nutpl-develop \
  --timeout 30000 \
  --top-failures 8 \
  --json
```

Default full-project runs now auto-tune class parallelism. For most local runs,
start with:

```bash
go run ./cmd/oaer compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --json
```

When running in sharded CI, use auto shard selectors with env wiring:

```bash
OAER_SHARD_COUNT=6 OAER_SHARD_INDEX=2 \
go run ./cmd/oaer compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --parallel auto \
  --shard-count auto \
  --shard-index auto \
  --duration-history /path/to/perf.json \
  --json
```

Focused class or method run:

```bash
go run ./cmd/oaer compat local-tests \
  --project example-projects/sf-cred-pkg-develop \
  --class AccountsTriggerHandlerTest \
  --method testSomeBehavior \
  --json
```

Large-project blocker triage:

```bash
go build -o /tmp/oaer ./cmd/oaer
/tmp/oaer compat local-tests \
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
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-corpus.json --json
```

## Phase Gate

Current status as of 2026-05-15:

- The server-example execution harness is green across the checked
  `example-projects` corpus: `pass=101 fail=0 unsupported=0 missing=0`.
- The owned local-test corpus baseline is green via
  `go run ./cmd/oaer compat local-tests --check
  docs/fixtures/local-tests-corpus.json --json`.
- A full `example-projects` post-parity inventory currently includes the public
  stubs and is not green: `filesScanned=59479 findings=4076
  testBlockingFindings=4076 surfaces=2`. The remaining scanner buckets are
  `platform.cache-connectapi` and `metadata.apex-deploy`.
- `src-nmb-nutpl-develop` is the current green runtime sentinel:
  `go run ./cmd/oaer compat local-tests --project
  example-projects/src-nmb-nutpl-develop --timeout 30000 --top-failures 8
  --json` reports `total=761 pass=761` with no failures, unsupported outcomes,
  load errors, compile errors, or internal errors.
- Full runtime support for all six example projects is not complete yet. The
  current six-project runtime baseline is
  `docs/fixtures/local-tests-example-projects.json`: one project is green and
  the other five stop at compile-gap frontiers such as missing `znu` managed
  package types, missing standard object/type coverage, and package/source
  duplicate-symbol issues.
- `oaer compat examples`, `oaer compat server-examples`, and
  `oaer compat post-parity` are separate gates. The zero post-parity inventory
  means no current scanner/test-readiness blockers are known for the checked
  example projects; it is not the same as proving every example-project Apex
  test runs end to end.

Phase 0 is complete when:

1. `oaer compat examples` produces a stable report for each example project.
2. The report counts match manual inspection.
3. No panic occurs during scan or check.
4. Reduced compatibility fixtures cover observed selector, trigger, controller,
   HTTP mock, and REST patterns.
