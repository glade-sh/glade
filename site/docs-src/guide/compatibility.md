# Compatibility

Glade tracks feature readiness in a machine-readable capability matrix. Each capability has a status:

| Status | Meaning |
| --- | --- |
| `supported` | Implemented and covered by compatibility tests. |
| `partial` | Common cases work, with documented gaps. |
| `stub` | Present for loading or controlled placeholders. |
| `unsupported` | Fails with a stable diagnostic before or during execution. |
| `unknown` | Not evaluated yet. |

## First Layer

Use the [Support Map](/guide/support-map) when you want the broad answer:
runtime area, standard-library family, and whether the surface is supported,
partial, or unsupported.

Use this page and the checked reports when you need the lower layer:
support status and method-level standard-library rows.

## Reports

The docs site carries a short dashboard copy and a support map for readers.
The detailed checked reports live in the repository.

## Fixtures

Compatibility fixtures cover parser, semantic, execution, test, storage,
server, and stdlib behavior. They are maintained in `glade-tools`, not in the
published `glade` CLI.

## Local-test compatibility

Large local-test gates classify each test as pass, fail, unsupported, load
error, compile error, or internal error. Those gates live in `glade-tools`.

The clean rule is simple. Add the fixture first. Then change the status.
