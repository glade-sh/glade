# Glade Docs

This is the reference area for Glade. The public home page is [glade.sh](https://glade.sh/).

Use this page to install Glade, run the first local checks, and find the right section when you need a command, workflow, or support detail.

## Install

For macOS and Linux:

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

Check the binary:

```bash
glade version
glade doctor
```

For more install paths, source builds, and CI setup, read [Installation](/guide/installation).

## First Project Run

From an SFDX project root:

```bash
glade check --project .
glade test --project . --json
```

Run one class, one method, or tests touched by a git ref:

```bash
glade test --project . --filter AccountServiceTest --json
glade test --project . --filter AccountServiceTest.testCreatesAccount --json
glade test --project . --changed-since origin/main --json
```

For the wider testing workflow, read [Local Testing](/guide/local-testing).

## What Glade Runs

Glade is a clean-room local Apex runtime. It parses Apex source, builds a symbol graph, type-checks supported semantics, lowers code into a VM representation, and executes against in-memory or SQLite-backed local org data.

The same runtime sits behind the CLI, local tests, editor support, the local API server, and the playground.

## Find The Right Shelf

| Need | Page |
| --- | --- |
| Install the binary or build from source | [Installation](/guide/installation) |
| Find command flags and examples | [CLI Reference](/guide/cli-reference) |
| Run local Apex tests | [Local Testing](/guide/local-testing) |
| Run only tests affected by a change | [Affected-Test Selection](/guide/affected-tests) |
| Wire editor diagnostics and debug snapshots | [Editor, LSP, and DAP](/guide/editor) |
| Exercise REST flows against local data | [Local API Server](/guide/local-api-server) |
| Try examples in the browser | [Playground](/guide/playground) |
| Check supported surfaces and gaps | [Support Map](/guide/support-map) |
| Read project status and generated ledgers | [Compatibility](/guide/compatibility) |

::: tip Playground
Use the hosted playground when you want to see the runtime before installing:
[play.glade.sh/playground/](https://play.glade.sh/playground/)
:::
