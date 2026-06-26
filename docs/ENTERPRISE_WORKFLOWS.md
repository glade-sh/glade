# Enterprise Workflows

Glade can inspect a large Apex project, report architecture risk, find
conservative cruft candidates, and collect proof for branch changes without
waiting on an org.

The enterprise commands use local evidence. They do not claim full Salesforce
parity.

## Assessment

Generate a codebase assessment:

```bash
mkdir -p reports
glade report assess --project . --format html --out reports/glade-assessment.html --include-metadata --include-tests
```

The report includes inventory, top risk signals, trigger map, SOQL/DML counts,
async and callout indicators, fflib-style inventory, test health, findings, and
known limitations.

## Cruft And Dead Code

Scan for conservative delete, deprecate, review, and do-not-delete candidates:

```bash
mkdir -p reports
glade report cruft --project . --format html --out reports/glade-cruft.html
```

Global and public symbols are protected by default. A global package surface is
not a safe-delete candidate. Dynamic Apex, string dispatch, custom metadata
routing, Aura, LWC, and invocable exposure lower confidence.

## Refactor Proof

Collect proof for a branch change:

```bash
mkdir -p reports
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
```

The proof report records git diff, parse/index status, semantic status, graph
impact, affected-test selection, optional trace summary, and public/global API
surface warnings.

Use `--fail-on-api-break` when CI should fail on public or global API surface
changes:

```bash
glade report refactor-proof --project . --since origin/main --fail-on-api-break --format json
```

## Graph And References

Build the project graph directly, inspect definitions and references, or plan a
safe rename:

```bash
glade inspect graph --project . --json
glade inspect definition --project . --symbol Account.Name
glade inspect references --project . --symbol RefinementService.total --json
glade refactor rename --project . --symbol RefinementService --to FileRefinementService --dry-run --json
```

Definition, reference, and rename commands use the same code-intelligence graph
as LSP definition, references, rename, hover, and completion.

## Runtime Traces

Write local test traces:

```bash
mkdir -p reports
glade test --project . --class MyPassingTest --trace reports/glade-trace.json --json
```

Trace output uses Glade's Chrome trace-event document. The enterprise proof
report can summarize an existing trace with `--trace`.

## Service Config Validation

Validate service configuration while keeping runtime behavior honest:

```bash
glade test --project . --services .glade/services.yml --json
```

Supported first-pass service config keys:

```yaml
version: 0
mode: strict
calloutFixtures: [fixtures/callouts/pricing-success.json]
asyncDrain: true
asyncMaxDepth: 5
platformEventsOut: reports/platform-events.jsonl
```

This validates the file and fixture paths. Runtime fixture injection is not
enabled by this first packet. Create `reports/` before using
`platformEventsOut: reports/platform-events.jsonl`.

## Known Limitations

- Static graph references are conservative and may over-select.
- Dynamic Apex and metadata-driven dispatch reduce confidence.
- Global and public managed-package APIs are never safe-delete candidates.
- Service config validation is available before full runtime fixture injection.
- Compatibility and support-map generation remain plugin-owned.
