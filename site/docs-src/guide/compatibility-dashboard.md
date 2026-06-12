# Developer Reports

Start with [Apex and Salesforce Support](/guide/support-map) when you want to
know whether Glade can run a project or test path. This page is for maintainers
who need the generated proof reports.

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
