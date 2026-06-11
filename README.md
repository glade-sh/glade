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
glade init --project . --yes
glade config validate --project .
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
| Local API server, LSP, DAP, watch, profile, and plugin-managed scanners | Supported for local development. |

Drill down from there:

```bash
glade check --project .
glade test --project . --json
glade plugins install @glade/performance
glade performance scan --project . --json
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
glade test changed --project . --since origin/main --json
glade test failed --project .
glade plugins install @glade/performance
glade performance scan --project . --json > reports/glade-performance.json
glade performance scan --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

The source-only performance scan maps entry points and hard static patterns.
Trace input ranks measured spans and SOQL row counts.

Maintenance scanners ship as plugins. The product repo contains the plugin
manager, command router, runtime, local schema import, and product test runner.
Maintenance scanners, advisory performance scans, docs inventory, fixtures, and
generated ledgers ship as plugins and do not live in the product runtime
packages.

First-party plugins are documented in [docs/PLUGINS.md](docs/PLUGINS.md).
Install them as `@glade/compat` and `@glade/performance`. The short aliases
`compat` and `performance` resolve to those canonical names. Third-party
plugins use the same executable manifest contract.

List installable marketplace plugins:

```bash
glade plugins available
```

Run anonymous Apex:

```bash
glade exec "System.debug('hello from glade');"
```

Serve a Salesforce-shaped local API:

```bash
glade server --project . --addr 127.0.0.1:8080
```

Write CI artifacts from the same local run loop:

```bash
glade check --project . --format sarif --output reports/glade-check.sarif
glade dev test --project . --out .glade/runs
glade report github latest --runs-dir .glade/runs
glade report export latest --runs-dir .glade/runs --format html --output reports/glade-report.html
```

## Docs

- [Install](docs/INSTALL.md)
- [Project configuration](docs/CONFIG.md)
- [Local Apex testing](docs/LOCAL_TESTING.md)
- [CI artifacts](docs/CI_ARTIFACTS.md)
- [Rich local workflows](docs/RICH_LOCAL_WORKFLOWS.md)
- [Test startup cache](docs/TEST_STARTUP_CACHE.md)
- [Plugins](docs/PLUGINS.md)
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
