---
pageType: guide
canonicalTask: /guide/workflows/ci
---

# Glade CI quickstart

Use Glade in CI when pull requests need local checks, affected tests, saved
reports, and stable process exits.

The git examples assume `origin/main` is your intended base ref and is available
locally. Substitute the correct existing ref for your repository before running
changed-test or refactor commands.

## Before you start

Choose the [pinned advisory pilot](/guide/ci-artifacts#advisory-pilot) while
evaluating Glade. Use the [enforcing gate](/guide/ci-artifacts#enforcing-gate)
only for paths your team has validated. Neither replaces Salesforce validation.

Fetch enough git history for `origin/main`. Create `reports` before writing
artifacts. Use JSON, SARIF, and JUnit for machines; keep human output for local
runs.

## Steps

Write SARIF for code scanning:

```bash
mkdir -p reports
glade check --project . --format sarif --output reports/glade-check.sarif
```

Run changed tests as JSON:

```bash
glade test changed --project . --since origin/main --json --no-progress
```

Write JUnit:

```bash
glade test --project . --junit reports/glade-junit.xml --no-progress
```

## Expected output

`glade check` writes SARIF to the report path. Changed tests write JSON to
stdout. JUnit output lands in `reports/glade-junit.xml`. Non-zero exit codes
mark failed gates.

## Common wrong turn

Do not hide failed checks with broad `|| true` shell glue. Let Glade fail the
gate, then upload artifacts with the CI system's always-run upload step.

## Deeper reference

- [Add Glade to CI](/guide/ci-artifacts)
- [Automation and JSON](/guide/automation)
- [Exit codes](/guide/exit-codes)
- [CI setup](/help/ci-setup)
