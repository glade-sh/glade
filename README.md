# Glade

Glade is a clean-room Apex runtime for local development and testing. It reads
Salesforce projects from disk, checks Apex, runs supported tests without an org,
and exposes the same runtime through a CLI, editor tools, a playground, and a
Salesforce-shaped local API server.

Site: <https://glade.sh>

## Start Here

Install Glade:

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
glade doctor
```

`glade doctor` must report `parser: ok (tree-sitter)` before project parsing,
checking, or testing will work.

Run it in an SFDX project:

```bash
cd path/to/sfdx-project
glade check --project .
glade test --project . --json
```

Build from source when working on Glade itself:

```bash
git clone https://github.com/glade-sh/glade.git
cd glade
go build -o glade ./cmd/glade
./glade version
```

## What Glade Supports

The first layer is a support map. The second layer is the generated method
coverage and known-gap docs checked into this repository.

| Area | Current support |
| --- | --- |
| Apex parse, indexing, and semantic checks | Supported for the local development contract. |
| Local Apex tests | Supported for the VM subset, with isolated test data, statics, limits, async drain, and JSON/JUnit output. |
| SOQL, DML, triggers, SObjects, and storage | Supported for the checked local data runtime contract. |
| `Database` methods | Supported for the tracked local rows in the stdlib ledger. |
| `String`, dates, time, math, assertions, labels, URLs, and user info | Wide support, with exact rows in the stdlib ledger. |
| `Schema`, describe APIs, JSON, regex, HTTP mocks, email, Visualforce controller helpers, and many `Test.*` helpers | Partial. The local model covers common test paths and records gaps by method. |
| Platform services such as approval execution, quick actions, business-hours services, sandbox lifecycle, live request context, and identity services | Unsupported unless a row says otherwise. Glade should return a stable unsupported diagnostic, not silent wrong behavior. |
| Local API server, LSP, DAP, watch, profile, and performance scanning tools | Supported for local development. |

Drill down from there:

```bash
glade check --project .
glade test --project . --json
glade inspect performance --project . --json
```

- High-level readiness: [docs/COMPATIBILITY_DASHBOARD.md](docs/COMPATIBILITY_DASHBOARD.md)
- Method-level standard library coverage: [docs/STDLIB_COVERAGE.md](docs/STDLIB_COVERAGE.md)
- Known gaps: [docs/KNOWN_GAPS.md](docs/KNOWN_GAPS.md)
- Site support map: <https://glade.sh/docs/guide/support-map>

The rule is simple. A supported row has code and checked coverage.

## Common Workflows

Run one class, one method, or only tests affected by local changes:

```bash
glade test --project . --filter AccountServiceTest --json
glade test --project . --filter AccountServiceTest.testCreatesAccount --json
glade test --project . --changed-since origin/main --json
glade inspect performance --project . --json > reports/glade-performance.json
glade inspect performance --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

The source-only performance scan maps entry points and hard static patterns.
Trace input ranks measured spans and SOQL row counts.

For large project triage and maintenance scanners, use the sibling
`~/Dev/glade-tools` project. Those commands depend on this framework but are not
part of the published `glade` CLI:

```bash
glade-tools local-tests --project . --parallel auto --json
```

Run anonymous Apex:

```bash
glade exec "System.debug('hello from glade');"
```

Serve a Salesforce-shaped local API:

```bash
glade server --project . --addr 127.0.0.1:8080
```

## Docs

- [Install](docs/INSTALL.md)
- [Local Apex testing](docs/LOCAL_TESTING.md)
- [Dogfood checklist](docs/DOGFOOD_CHECKLIST.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Compatibility policy](docs/COMPATIBILITY.md)
- [Compatibility dashboard](docs/COMPATIBILITY_DASHBOARD.md)
- [Standard library coverage](docs/STDLIB_COVERAGE.md)
- [Known gaps](docs/KNOWN_GAPS.md)
- [Editor and debug setup](docs/EDITOR.md)
- [Release policy](docs/RELEASE_POLICY.md)

## Release and Distribution

Release notes, gates, archives, and Homebrew flow live in:

- [docs/RELEASE_NOTES.md](docs/RELEASE_NOTES.md)
- [docs/RELEASE_POLICY.md](docs/RELEASE_POLICY.md)
- [docs/DISTRIBUTION_WORKFLOW.md](docs/DISTRIBUTION_WORKFLOW.md)
