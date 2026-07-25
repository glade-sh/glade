<p align="center">
  <img src="site/docs-src/public/logo-mark.svg" alt="Glade boxed contour mark" width="96" height="96">
</p>

<h1 align="center">Glade</h1>

<p align="center">
  Local Apex runtime and tooling for Salesforce teams.
</p>

<p align="center">
  <a href="https://github.com/glade-sh/glade/actions/workflows/security.yml"><img alt="Security workflow" src="https://github.com/glade-sh/glade/actions/workflows/security.yml/badge.svg?branch=main"></a>
</p>

<p align="center">
  <a href="https://glade.sh">Site</a>
  ·
  <a href="https://glade.sh/guide/quickstart">Quickstart</a>
  ·
  <a href="https://glade.sh/guide/support-map">Capability map</a>
  ·
  <a href="https://glade.sh/guide/security-trust">Security</a>
  ·
  <a href="docs/TESTER_FIELD_GUIDE.md">Tester field guide</a>
</p>

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

`glade doctor` must report `Ready.` before project parsing, checking, or
testing will work.

For a small tester pilot, start with
[docs/TESTER_FIELD_GUIDE.md](docs/TESTER_FIELD_GUIDE.md). It shows the install,
first project run, VS Code setup, AI handoff prompt, CI gate, and useful
follow-on workflows in one place.

Run it in an SFDX project:

```bash
cd path/to/sfdx-project
glade init --project . --yes
glade config validate --project .
glade check --project .
glade test --project . --json --no-progress
```

Build from source when working on Glade itself:

```bash
git clone https://github.com/glade-sh/glade.git
cd glade
go build -o glade ./cmd/glade
./glade version
```

## What Glade Supports

The first layer is the public Apex and Salesforce support map. The second layer
is the generated method coverage and known-gap docs checked into this
repository.

| Area | Current support |
| --- | --- |
| Apex parse, indexing, and semantic checks | Works well for the local development contract. |
| Project configuration | `glade.yml` and `sfdx-project.json` discovery cover package directories, default namespace, namespace remaps, source-backed managed package dependencies, captured package artifacts, package shims, and org features. Storage and limit modes are selected by CLI flags. |
| Local Apex tests | Works well for the VM subset, with isolated test data, statics, limits, async drain, and JSON/JUnit output. |
| SOQL, DML, triggers, SObjects, and storage | Work well for the checked local data runtime contract. |
| `Database` methods | Supported for the tracked local rows in the stdlib ledger. |
| `String`, dates, time, math, assertions, labels, URLs, and user info | Wide support, with exact rows in the stdlib ledger. |
| `Schema`, describe APIs, JSON, regex, HTTP mocks, email, Visualforce controller/page rendering, and tracked `Test.*` helpers | Supported local rows are complete in the checked ledger. Hosted chrome, exact Visualforce lifecycle timing, byte-for-byte PDF output, delivery, and service behavior have explicit unsupported rows. |
| Visualforce and LWC local preview | Preview features. `glade dev vf` serves useful local `/apex` routes. `glade dev lwc` serves a local Lightning workbench at `/` and `/lwc`, context presets, route previews, selected LDS/UI API and Apex wire paths, base-component shims, packaged SLDS assets, and Visualforce Lightning Out through the shared local runtime. Hosted Lightning Experience, full UI API parity, Flow Builder behavior, live EMP streaming, exact console APIs, and exact base-component behavior stay with Salesforce. |
| Platform services such as approval, quick actions, business hours, sandbox lifecycle, request context, and Trailblazer identity helpers | Deterministic local harness rows are supported where the checked ledger says `supported`. Hosted service execution stays outside the local contract. |
| Local API server, LSP, DAP, watch, and profile tools | Work well for local development. The local API covers REST discovery, SObject CRUD/query, limits and record counts, Tooling executeAnonymous, local Tooling source/schema metadata queries, Composite sObject insert, Composite Batch and Tree, Bulk API v2 simple query jobs, layout/default metadata, metadata job status, fixture resets, and SQLite persistence. |
| Enterprise graph and report tools | Work as conservative local evidence for assessment, cruft review, and refactor proof. |

Drill down from there:

```bash
glade check --project .
glade test --project . --json
glade support
mkdir -p reports
glade report assess --project . --format html --out reports/glade-assessment.html
```

- Public support map: <https://glade.sh/guide/support-map>
- Method-level standard library coverage: [docs/STDLIB_COVERAGE.md](docs/STDLIB_COVERAGE.md)

The rule is simple. A supported row has code and checked coverage.

## Common Workflows

Run one class, one method, or only tests affected by local changes:

```bash
glade test --project . --class RefinementServiceTest --json --no-progress
glade test --project . --class RefinementServiceTest --method testRefinesFileRow --json --no-progress
glade test changed --project . --since origin/main --json --no-progress
glade test failed --project .
glade plugins install @glade/performance
mkdir -p reports
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
Checked generated ledger docs in this repository are published outputs from the
first-party compat plugin; base `glade` does not regenerate them.

First-party plugins are documented in [docs/PLUGINS.md](docs/PLUGINS.md).
Install user-facing plugins as `@glade/performance` and `@glade/orgpackage`.
`@glade/compat` is maintainer-facing support tooling. The short aliases
`performance` and `orgpackage` resolve to those canonical names. Third-party
plugins use the same executable manifest contract.

The default public plugin registry at `https://plugins.glade.sh/index.json`
serves the first-party plugin catalog: `@glade/compat`,
`@glade/orgpackage`, and `@glade/performance`. Direct archives and local links
remain available for offline, private, and development use.

List installable marketplace plugins:

```bash
glade plugins available
```

Run anonymous Apex:

```bash
glade exec "System.debug('hello from glade');"
```

Install the bundled VS Code extension and open the local Glade hub:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

The extension adds Glade Home, Start Here, Local Runs, Data Environments, Local
Org, Exec & SOQL, Debug, and Plugins views without taking over org-backed
Salesforce commands.

Serve the Visualforce preview feature for useful local `/apex` rendering:

```bash
glade dev vf --project . --addr 127.0.0.1:8080
```

Serve the LWC preview shell and open a context preset:

```bash
glade toolchain install
glade dev lwc --project . --open
glade dev lwc --project . --context accountRecord --open
```

Capture installed package contracts when a local project depends on package
APIs without carrying package source:

```bash
glade plugins install @glade/orgpackage
glade package capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet
```

Serve a Salesforce-shaped local API:

```bash
glade server --project . --addr 127.0.0.1:8080
```

Write CI artifacts from the same local run loop:

```bash
mkdir -p reports
glade check --project . --format sarif --output reports/glade-check.sarif
glade dev test --project . --out .glade/runs
glade report github latest --runs-dir .glade/runs
glade report export latest --runs-dir .glade/runs --format html --output reports/glade-report.html
```

Map a large codebase and collect branch-change proof:

```bash
glade inspect graph --project . --json
glade inspect definition --project . --symbol Account.Name
glade inspect references --project . --symbol RefinementService.total --json
glade refactor rename --project . --symbol RefinementService --to FileRefinementService --dry-run --json
mkdir -p reports
glade report assess --project . --format html --out reports/glade-assessment.html
glade report cruft --project . --format html --out reports/glade-cruft.html
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
```

## Docs

- [Tester field guide](docs/TESTER_FIELD_GUIDE.md)
- [Install](docs/INSTALL.md)
- [Project configuration](docs/CONFIG.md)
- [Local Apex testing](docs/LOCAL_TESTING.md)
- [Public workflow guide](https://glade.sh/guide/workflows)
- [Product areas](https://glade.sh/guide/modules)
- [CLI reference](https://glade.sh/reference/cli)
- [Support map](https://glade.sh/guide/support-map)
- [CI artifacts](docs/CI_ARTIFACTS.md)
- [Rich local workflows](docs/RICH_LOCAL_WORKFLOWS.md)
- [Enterprise workflows](docs/ENTERPRISE_WORKFLOWS.md)
- [Security & Trust](docs/SECURITY_TRUST.md)
- [Security policy](SECURITY.md)
- [Test startup cache](docs/TEST_STARTUP_CACHE.md)
- [Plugins](docs/PLUGINS.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Compatibility policy](docs/COMPATIBILITY.md)
- [Standard library coverage](docs/STDLIB_COVERAGE.md)
- [Local LWC support](docs/LWC_SUPPORT.md)
- [Editor and debug setup](docs/EDITOR.md)
