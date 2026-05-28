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

Smoke-check:

```bash
glade version
glade doctor
```

Run on an SFDX project:

```bash
glade check --project path/to/project --json
glade test --project path/to/project --json
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
- [Architecture](docs/ARCHITECTURE.md)
- [Compatibility status](docs/COMPATIBILITY_DASHBOARD.md)
- [Known gaps](docs/KNOWN_GAPS.md)
- [Editor and debug setup](docs/EDITOR.md)
