# Glade

Glade is an orgless Apex runtime for local development and testing.

Site: <https://glade.sh>

## What Glade Does

- Parses and checks Apex classes and triggers.
- Runs anonymous Apex and local Apex tests.
- Executes SOQL, DML, and trigger paths in a local runtime.
- Serves a Salesforce-shaped local HTTP API for integration tests.
- Tracks behavior coverage with compatibility fixtures and readiness gates.

## Install

For macOS and Linux:

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

Build and run from source:

```bash
git clone https://github.com/glade-sh/glade.git
cd glade
go build -o glade ./cmd/glade
./glade version
./glade doctor
```

During development, you can also run the CLI without installing a binary:

```bash
go run ./cmd/glade version
go run ./cmd/glade check --project path/to/sfdx-project
```

## Quick Start

Prerequisites:

- Glade on your `PATH`
- an SFDX project on disk for local project commands

Smoke-check:

```bash
glade version
glade doctor
```

Run Apex tests locally without a Salesforce org:

```bash
cd path/to/sfdx-project
glade check --project .
glade test --project . --json
```

Run one class, one method, or only tests affected by local changes:

```bash
glade test --project . --filter AccountServiceTest --json
glade test --project . --filter AccountServiceTest.testCreatesAccount --json
glade test --project . --changed-since origin/main --json
```

For large compatibility triage runs, use the compatibility harness and its
class-parallel worker flag:

```bash
glade compat local-tests --project . --parallel auto --json
```

Run anonymous Apex:

```bash
glade exec "System.debug('hello from glade');"
```

## Setup

See [docs/INSTALL.md](docs/INSTALL.md) for:

- one-line install
- release-archive install
- build from source
- CI install patterns
- local Apex testing without an org
- persistent local server setup
- playground setup
- Homebrew tap workflow

## Release and Distribution

See [docs/RELEASE_POLICY.md](docs/RELEASE_POLICY.md) for:

- release gates
- versioning and upgrade policy
- release artifact workflow
- Homebrew tap update workflow

Operator runbook: [docs/DISTRIBUTION_WORKFLOW.md](docs/DISTRIBUTION_WORKFLOW.md)

## Core Docs

- [Start here docs map](docs/README.md)
- [Local Apex testing](docs/LOCAL_TESTING.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Compatibility status](docs/COMPATIBILITY_DASHBOARD.md)
- [Known gaps](docs/KNOWN_GAPS.md)
- [Editor and debug setup](docs/EDITOR.md)
