# Maintainer Proof Reports

Most users should start with the [Support map](/guide/support-map). This
page summarizes generated reports used by maintainers and release gates.

The checked source report lives at `docs/COMPATIBILITY_DASHBOARD.md` in the
repository. The first-party `compat` plugin regenerates it.

## Current Gate

The checked dashboard in this repository state reports:

| Measure | Value |
| --- | ---: |
| Readiness | ready |
| Required complete | 21/21 |
| Required incomplete | 0 |
| Required supported capabilities | 21 |
| Tracked post-MVP partial capabilities | 9 |

## Current Surface Landscape

The current maintainer surface refresh reports zero gaps and zero failure rows.

| Measure | Value |
| --- | ---: |
| Implemented rows | 130268 |
| Partial rows | 1 |
| Passive shape rows | 47493 |
| Stub/no-op rows | 262 |
| Explicit unsupported rows | 6337 |
| Missing shape gaps | 0 |
| Missing behavior gaps | 0 |
| Missing evidence gaps | 0 |
| Failure rows | 0 |

## Report Set

| Report | Use |
| --- | --- |
| [`docs/COMPATIBILITY_DASHBOARD.md`](https://github.com/glade-sh/glade/blob/main/docs/COMPATIBILITY_DASHBOARD.md) | Capability gate for parser, semantic analysis, VM, tests, storage, server, stdlib, tooling, and release rows. |
| [`docs/STDLIB_COVERAGE.md`](https://github.com/glade-sh/glade/blob/main/docs/STDLIB_COVERAGE.md) | Method-level standard-library support rows. |
| [`docs/KNOWN_GAPS.md`](https://github.com/glade-sh/glade/blob/main/docs/KNOWN_GAPS.md) | Current unsupported and post-MVP gaps. |

## Status Values

- `supported` means covered behavior.
- `partial` means usable behavior with explicit gaps.
- `stub` means a controlled placeholder exists.
- `unsupported` means Glade should report a stable diagnostic.
- `unknown` means the team has not measured it yet.

The dashboard is a measuring stick, not the public support map.
