# Compatibility

Glade tracks feature readiness in a machine-readable capability matrix. Each capability has a status:

| Status | Meaning |
| --- | --- |
| `supported` | Implemented and covered by compatibility tests. |
| `partial` | Common cases work, with documented gaps. |
| `stub` | Present for loading or controlled placeholders. |
| `unsupported` | Fails with a stable diagnostic before or during execution. |
| `unknown` | Not evaluated yet. |

The MVP gate is:

```bash
glade compat mvp
```

A release can only be promoted as MVP-ready when the required capabilities are supported:

```bash
glade compat mvp --require-ready
```

## First Layer

Use the [Support Map](/guide/support-map) when you want the broad answer:
runtime area, standard-library family, and whether the surface is supported,
partial, or unsupported.

Use this page and the generated reports when you need the lower layer:
capability IDs, release gates, and method-level standard-library rows.

## Reports

Generate the checked-in reports from the capability matrix:

```bash
glade compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
glade compat gaps --output docs/KNOWN_GAPS.md
glade compat stdlib --output docs/STDLIB_COVERAGE.md
```

The docs site carries a short dashboard copy and a support map for readers. The
repository remains the source of truth for generated markdown files.

## Fixtures

Compatibility fixtures cover parser, semantic, execution, test, storage, server, and stdlib behavior. Use them before moving a feature from `partial` to `supported`.

```bash
glade compat validate fixtures/example.json
glade compat run fixtures/example.json --json
```

## Local-test compatibility

Large local-test gates classify each test as pass, fail, unsupported, load error, compile error, or internal error. That makes progress measurable without hiding unsupported behavior behind generic failures.

The clean rule is simple. Add the fixture first. Then change the status.
