# Example Project Compatibility Harness

The `oaer compat examples` command inventories real Salesforce projects and reports
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

## Phase Gate

Phase 0 is complete when:

1. `oaer compat examples` produces a stable report for each example project.
2. The report counts match manual inspection.
3. No panic occurs during scan or check.
4. Reduced compatibility fixtures cover observed selector, trigger, controller,
   HTTP mock, and REST patterns.
