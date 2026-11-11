---
name: generated-files
description: Use before editing any glade file that has a generated header, the standard-object schema, stub overlays, or a docs file listed as a checked support artifact.
---

# Generated files

Never edit generated output by hand. Change the input or the generator,
regenerate, and commit input and output together. Read the header first: it
names the generator.

## Map

| Output | Owner | How to change it |
| --- | --- | --- |
| `internal/storage/standard_schema_generated.go` | generator in the sibling tools repository | `docs/STANDARD_OBJECT_SCHEMA.md` lists the input and the command. |
| `docs/STANDARD_OBJECT_COVERAGE.md` | generated from `internal/storage` metadata | Regenerate. About 360 KB: search it, do not read it. |
| `docs/COMPATIBILITY_DASHBOARD.md`, `docs/KNOWN_GAPS.md`, `docs/STDLIB_COVERAGE.md` | first-party compat plugin | Run the plugin. Base `glade` consumes them and does not regenerate them. |
| `internal/playground/static` | `internal/playground/web` build | `npm run build --prefix internal/playground/web`; review the diff. |
| `internal/lwcruntime/embed/glade.out.js` | `lwcruntime` build | `npm test --prefix lwcruntime` rebuilds it; review the diff. |
| Any file with a "generated" or "do not edit" header | the generator the header names | Follow the header. |

Maintenance tools live in the sibling `glade-tools` repository. This repository
does not depend on them, and a maintenance command does not belong in base
`glade`.

## After regenerating

Run the tests that pin counts in the package, for example
`go test ./internal/<pkg> -run 'Deterministic|Coverage|Policy|Inventory'`, then
`go test ./internal/repoguard`. Generated snapshots carry a date and a scope;
their counts do not establish fresh release validation.
