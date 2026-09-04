# How Glade works

Glade reads source and metadata from disk, checks it, and executes supported
behavior in its own local runtime. The CLI, editor, and local browser tools
provide different interfaces to that runtime. It is more than a static analyzer,
not a hidden Salesforce org, and not a complete hosted-platform emulator.

Use this overview to choose the Glade subsystem behind a local task. Each
section names the owned behavior, the first command, and the Salesforce
boundary. Open the linked workflow for step-by-step instructions.

| Area | Use it for | Start with |
| --- | --- | --- |
| [Apex runtime](#apex-runtime) | Check Apex, inspect symbols, and run anonymous Apex locally. | `glade check --project .` |
| [Test runner](#test-runner) | Run local Apex tests, affected-test loops, and warm test servers. | `glade test --project .` |
| [Local org and data](#local-org-and-data) | Use named SQLite-backed environments and Salesforce-style local APIs. | `glade org create local-dev` |
| [LWC preview](#lwc-preview) | Open local component and page preview routes. | `glade dev lwc --project . --open` |
| [Visualforce preview](#visualforce-preview) | Serve local pages and controller flows. | `glade dev vf --project . --port 8080` |
| [Debug and profile](#debug-and-profile) | Step through local Apex and analyze debug logs. | `glade dap --project .` |
| [Editor and workbench](#editor-and-workbench) | Use local language, test, data, and debug tools in VS Code. | `glade editor doctor vscode` |
| [Plugins](#plugins) | Run extension executables outside the base product. | `glade plugins list` |

## Apex runtime

The Apex runtime owns the local parser, semantic analyzer, VM, SOQL, DML,
triggers, SObjects, and schema behavior. Use it for local diagnostics, symbol
inspection, or anonymous Apex before a Salesforce validation pass.

```bash
glade check --project .
glade exec --project . "System.debug('local');"
glade inspect symbols --project .
```

Hosted platform services and exact production governor accounting remain
Salesforce validation work.

- [Apex workflow and limits](/guide/enterprise-workflows)
- [Apex language compatibility](/reference/apex-language-compatibility)
- [Apex support map](/reference/apex-support)

## Test runner

The test runner owns local Apex test discovery, execution, isolation, and
result output. Use it for fast full, focused, affected, or warm local loops.

```bash
glade test --project .
glade test changed --project . --since origin/main
glade test serve --project .
```

Run Salesforce tests when a test depends on live org services, installed
behavior that is not represented locally, or final hosted validation.

- [Run Apex tests](/guide/workflows/apex-tests)
- [Test startup cache](/guide/test-startup-cache)
- [CI reports and artifacts](/guide/ci-artifacts)

## Local org and data

Local org and data tools own named SQLite-backed environments, project auth to
those local environments, SObjects, and Salesforce-style REST routes.

```bash
glade org create local-dev
glade org auth local-dev --project .
glade server --project . --db .glade/local-dev.sqlite --addr 127.0.0.1:8080
```

They do not provide live OAuth, org identity, metadata deploy or retrieve,
Streaming, Pub/Sub, GraphQL, or hosted Tooling behavior.

- [Work with local data](/guide/workflows/local-data)
- [Use Glade as an sf target](/guide/glade-orgs)
- [Local API routes](/reference/local-api-routes)

## LWC preview

The LWC shell is a local preview surface, not hosted Lightning Experience. It
owns local component, record, app, tab, action, utility, Flow, and community
preview routes plus bounded data and service shims. The Workbench Console
provides Component Lab and Page Workbench views for those local routes.

```bash
glade toolchain status
glade dev lwc --project . --open
glade dev lwc --project . --context accountRecord --open
```

Use Salesforce for exact Lightning Experience chrome, complete UI API and
GraphQL semantics, hosted permissions, and base-component edge behavior.

- [Preview LWC locally](/guide/workflows/lwc-preview)
- [Local LWC shell](/guide/lwc-local-shell)
- [LWC support matrix](/reference/lwc-support)

## Visualforce preview

The Visualforce server is a local preview surface, not hosted Visualforce. It
owns local page routes, controllers, common component rendering, remoting,
uploads, view state, and preview diagnostics.

```bash
glade dev vf --project . --port 8080
curl http://127.0.0.1:8080/services/data/v65.0/glade/visualforce/support
```

Use Salesforce for hosted chrome, exact lifecycle timing, every component
edge, `PageReference.getContent*`, and byte-for-byte PDF output.

- [Preview Visualforce locally](/guide/workflows/visualforce-preview)
- [Visualforce support matrix](/reference/visualforce-support)

## Debug and profile

Debug and profile tools own local Debug Adapter Protocol sessions and Apex log
analysis. They read local execution traces and saved debug logs.

```bash
glade dap --project .
glade debug profile --log reports/anonymous-output.txt --format markdown
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade profile analyze reports/trace.json --format pprof
```

Use Salesforce replay debugging and hosted logs when a run depends on org
state, hosted services, or production governor accounting.

- [Debug Apex](/guide/workflows/debug-apex)
- [Debug Apex in VS Code](/help/debug-apex-vscode)
- [Profile an Apex debug log](/help/profile-apex-debug-log)

## Editor and workbench

Editor and workbench tools own the Glade VS Code extension, local language
features, Test Explorer integration, CodeLens actions, and DAP wiring.

```bash
glade editor doctor vscode
glade editor install vscode --force
glade lsp --project . --diagnostics-once
```

Use org-backed Salesforce tools for deploy, retrieve, org tests, SOQL Builder,
replay debugging, and org language-server ownership.

- [Use VS Code](/guide/editor)
- [Capability explorer](/guide/workbench)

## Plugins

Plugins are standalone extension executables. The plugin surface owns install,
link, discovery, lock files, and execution while keeping extension workflows
outside the base `glade` runtime.

```bash
glade plugins list
glade plugins available
glade plugins link --exec <plugin-executable>
```

Some plugins may capture org facts or call Salesforce. Review plugin source,
pin lock files in CI, and keep Salesforce validation for hosted behavior.

- [Plugins guide](/guide/plugins)
- [Install and manage plugins](/guide/plugins/install-manage)
- [Plugin lock files and CI](/guide/plugins/lock-ci)
