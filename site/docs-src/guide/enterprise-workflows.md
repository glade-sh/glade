# Enterprise Workflows

Use these commands when a large Apex project needs a map before it needs edits.
The reports use local evidence. They do not claim exact Salesforce behavior.

## Assessment

Generate a codebase assessment:

```bash
mkdir -p reports
glade report assess --project . --format html --out reports/glade-assessment.html --include-metadata --include-tests
```

The report includes inventory, top risk signals, trigger map, SOQL and DML
counts, async and callout indicators, fflib-style inventory, test health,
findings, and known limitations.

## Cruft and dead code

Scan for conservative delete, deprecate, review, and do-not-delete candidates:

```bash
mkdir -p reports
glade report cruft --project . --format html --out reports/glade-cruft.html
```

Global and public symbols are protected by default. A global package API is
not a safe-delete candidate. Dynamic Apex, string dispatch, custom metadata
routing, Aura, LWC, and invocable exposure lower confidence.

## Refactor Proof

Collect proof for a branch change:

```bash
mkdir -p reports
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
```

The proof report records git diff, parse and semantic status, graph impact,
affected-test selection, optional trace summary, and public or global API
warnings.

Use `--fail-on-api-break` when CI should fail on public or global API
changes:

```bash
glade report refactor-proof --project . --since origin/main --fail-on-api-break --format json
```

## Graph, references, and traces

Build the project graph directly:

```bash
glade inspect graph --project . --json
glade inspect definition --project . --symbol Account.Name
glade inspect references --project . --symbol InvoiceService.total --json
glade refactor rename --project . --symbol InvoiceService --to BillingService --dry-run --json
```

Definition, reference, and rename commands use the same code-intelligence graph
as LSP definition, references, rename, hover, and completion.

Write a local test trace, then summarize it in a proof report:

```bash
mkdir -p reports
glade test --project . --class MyPassingTest --trace reports/glade-trace.json --json
glade report refactor-proof --project . --since origin/main --trace reports/glade-trace.json --format html --out reports/glade-refactor-proof.html
```

## Known Limits

- Static graph references are conservative and may over-select.
- Dynamic Apex and metadata-driven dispatch reduce confidence.
- Global and public managed-package APIs are never safe-delete candidates.
- Service config validation exists before full runtime fixture injection.
- Compatibility and support-map generation remain plugin-owned.
