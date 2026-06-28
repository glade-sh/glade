# CLI Reference

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Reference</p>
  <p>Find the command you need, then copy a real example. Most project commands accept <code>--project &lt;root&gt;</code> and default to the current directory when a project is discoverable.</p>
  <ul>
    <li>Use setup commands when starting a workspace.</li>
    <li>Use check and test commands in the daily Apex loop.</li>
    <li>Use report, editor, and plugin commands for larger teams.</li>
  </ul>
</div>

All commands are local unless you point Glade at an external path or start a server. Most project commands accept `--project PROJECT_ROOT` and default to the current directory when a project is discoverable.

Human output is terminal text. Use `--json` or `--format` for scripts. See [CLI output modes](/guide/cli-output), [Automation and JSON](/guide/automation), and [Exit codes](/guide/exit-codes) for the stable contract.

<div class="docs-command-filter">
  <label for="cli-command-filter">Filter commands</label>
  <input id="cli-command-filter" data-command-filter=".docs-command-card" type="search" placeholder="Try check, test, report, plugin, playground" autocomplete="off" />
</div>

<div class="docs-command-grid" aria-label="Command groups">
  <a class="docs-command-card" href="#glade-config">
    <strong>Project setup</strong>
    <span><code>glade doctor</code>, <code>glade update</code>, <code>glade init</code>, <code>glade config</code>, and shell completion.</span>
  </a>
  <a class="docs-command-card" href="#glade-check">
    <strong>Check, inspect, and refactor</strong>
    <span><code>glade parse</code>, <code>glade inspect</code>, <code>glade schema</code>, <code>glade refactor</code>, and <code>glade check</code>.</span>
  </a>
  <a class="docs-command-card" href="#glade-test">
    <strong>Test</strong>
    <span>Run all tests, one class, one method, changed tests, daemon mode, and cache controls.</span>
  </a>
  <a class="docs-command-card" href="#glade-report-assess">
    <strong>Reports</strong>
    <span>Assessment, cruft, refactor-proof, saved run reports, and CI artifacts.</span>
  </a>
  <a class="docs-command-card" href="#glade-debug">
    <strong>Editor and debug</strong>
    <span>LSP, DAP, VS Code install, debug-log parsing, and profile analysis.</span>
  </a>
  <a class="docs-command-card" href="#glade-tui">
    <strong>Terminal UI</strong>
    <span>Open project, test, data, and plugin workflows in one terminal board.</span>
  </a>
  <a class="docs-command-card" href="#glade-plugins">
    <strong>Plugins</strong>
    <span>Install, link, inspect, lock, restore, and run first-party plugins.</span>
  </a>
  <a class="docs-command-card" href="#glade-playground">
    <strong>Server and playground</strong>
    <span>Start local Salesforce API routes and browser playground examples.</span>
  </a>
  <a class="docs-command-card" href="#glade-org">
    <strong>sf target</strong>
    <span>Create, start, and register a local Glade org target for supported <code>sf</code> commands.</span>
  </a>
  <a class="docs-command-card" href="#glade-db">
    <strong>Database</strong>
    <span>Seed, import, query, describe, inspect, reset, and export persistent local org state.</span>
  </a>
</div>

## `glade version`

Print build and version information.

```bash
glade version
```

## `glade update`

Update the Glade binary and bundled assets. The default dry run prints the
installer command without running it. A real update requires an explicit shell
opt-in.

```bash
glade update --dry-run
GLADE_UPDATE_ALLOW_SHELL=1 glade update
```

## `glade doctor`

Check the local environment and project discovery basics.

```bash
glade doctor
glade doctor --project .
```

## `glade toolchain`

Install or inspect the global LWC toolchain used by local Lightning Out routes.
Release installs populate it for normal use; source checkouts can refresh it
from the current tree.

```bash
glade toolchain status
glade toolchain status --json
glade toolchain install --from .
```

## `glade completion`

Generate shell completion scripts.

```bash
source <(glade completion bash)
glade completion zsh > ~/.zsh/completions/_glade
glade completion fish > ~/.config/fish/completions/glade.fish
```

## Discovery commands

Use these when you are not sure where to start:

```bash
glade help workflows
glade help commands
glade help exit-codes
glade examples
glade examples --tag limits
glade examples show refinement-service
glade explain GLADESEMA002
glade support
```

`glade` without arguments shows the short workflow doorway. `glade help commands` shows the full command inventory.

## `glade config`

Inspect, validate, and create `glade.yml`.

```bash
glade config show --project .
glade config show --project . --json
glade config validate --project .
glade config init --project . --yes --package-dir force-app
```

`glade init` is a top-level alias for `glade config init`.

## `glade parse`

Parse Apex files and report parser diagnostics.

```bash
glade parse force-app/main/default/classes/RefinementService.cls
glade parse force-app/main/default/classes --progress
glade parse force-app/main/default/classes --json
```

## `glade inspect symbols`

Build the project symbol index and print declarations, members, triggers, and schema objects.

```bash
glade inspect symbols --project .
glade inspect symbols --project . --kind class
glade inspect symbols --project . --full-paths
glade inspect symbols --project . --json
```

## `glade inspect graph`

Build the enterprise graph for Apex, triggers, metadata references, SObjects,
and code-intelligence references with conservative fallbacks.

```bash
glade inspect graph --project . --json
```

## `glade inspect definition`

Resolve a symbol or source location to its definition.

```bash
glade inspect definition --project . --symbol RefinementService
glade inspect definition --project . --file force-app/main/default/classes/RefinementService.cls --line 6 --column 13
```

Use `--json` when an editor task or script needs the structured result.

## `glade inspect references`

Print references for an Apex type, member, SObject, or schema field.

```bash
glade inspect references --project . --symbol RefinementService.total --json
glade inspect references --project . --symbol Account.Name --include-declaration
```

## `glade refactor rename`

Plan or apply a safe rename from the same code-intelligence graph. Dry run is
the default. Use `--write` only after reviewing the planned edits.

```bash
glade refactor rename --project . --symbol RefinementService --to FileRefinementService --dry-run --json
glade refactor rename --project . --file force-app/main/default/classes/RefinementService.cls --line 5 --column 14 --to totalNet --write
```

## `glade schema load`

Load supported Salesforce metadata from an SFDX project.

```bash
glade schema load --project .
glade schema load --project . --json
```

## `glade schema import describe`

Convert captured Salesforce describe JSON into a local Glade schema file.
Live org capture belongs in a plugin. The base command only imports a captured
catalog.

```bash
glade schema import describe --input reports/org-describe.json
glade schema import describe --input reports/org-describe.json --output schema/local.schema.json
glade schema import describe --input reports/org-describe.json --output schema/local.schema.json --project-cache .
```

`--project-cache PROJECT_ROOT` writes schema symbols under `.glade/symbols` for
offline definition, reference, and rename queries.

## `glade check`

Parse and type-check supported Apex and metadata semantics.

```bash
glade check --project .
glade check --project . --json
glade check --project . --format sarif --output glade-check.sarif
glade check --project . --format github
```

## `glade report assess`

Generate an enterprise assessment report.

```bash
glade report assess --project . --format json
mkdir -p reports
glade report assess --project . --format html --out reports/glade-assessment.html --include-metadata --include-tests
```

## `glade report cruft`

Classify conservative delete, deprecate, review, and do-not-delete candidates.

```bash
glade report cruft --project . --format json
mkdir -p reports
glade report cruft --project . --format html --out reports/glade-cruft.html
```

## `glade report refactor-proof`

Collect local evidence for changed Apex and metadata.

```bash
glade report refactor-proof --project . --since origin/main --format json
mkdir -p reports
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
```

## `glade exec`

Run execute-anonymous Apex against local source and storage.
Default output summarizes debug lines and limit counters, then writes the raw log under `.glade/logs`.

```bash
glade exec --project . "System.debug('hello from glade');"
mkdir -p reports
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade exec --project . --debug-log raw "System.debug('hello from glade');"
glade exec --project . --log-out reports/exec.log "System.debug('hello from glade');"
glade exec --project . --limit-mode strict "System.debug(Limits.getDmlStatements());"
```

## `glade test`

Discover and run local Apex tests. Useful flags include `changed --since <ref>`,
`--watch`, `--watch-once`, `--last-failed`, `--wizard`, `--daemon`,
`--connect`, `--no-serve`, `--class`, `--method`, `--json`, `--junit`, and `--limit-mode`.

`glade test serve` keeps the runtime warm across CLI invocations. Later runs
auto-connect through `.glade/test/serve.sock` unless `--no-serve` is set.

Warmed harness state is cached on disk at `.glade/test/startup.meta.json` plus a
hashed payload after a cold build. See
[Test Startup Cache](/guide/test-startup-cache) for freshness rules.

```bash
glade test serve --project .
glade test daemon status --project .
glade test --project .
glade test --project . --class RefinementServiceTest --json
mkdir -p reports
glade test --project . --class RefinementServiceTest --method testRefinesFileRow --junit reports/glade-junit.xml
glade test --project . --connect --class RefinementServiceTest
glade test changed --project . --since origin/main --json --no-progress
glade test failed --project .
glade test --project . --wizard
glade test --project . --watch
glade test --project . --daemon --watch
glade test --project . --limit-mode permissive --json
glade test daemon stop --project .
```

Clear the on-disk startup cache or skip it for one run:

```bash
glade test clear-cache --project .
glade test --project . --no-cache --class RefinementServiceTest
```

Run `glade help test` for the full flag list.

## `glade tui`

Open the terminal UI for project, test, data, and plugin workflows.

```bash
glade tui --project .
glade tui --project . --view tests
glade tui --project . --db .glade/refinement-local.sqlite --view data
glade tui --project . --db .glade/refinement-local.sqlite --view data --target-org devhub --object Account
glade tui --project . --no-ui
```

## `glade dev` and `glade report`

Run the human-focused test loop and save artifacts under `.glade/runs`.

```bash
glade dev --project .
glade dev test --project . --out .glade/runs
glade dev test --project . --failed --out .glade/runs
glade report list --runs-dir .glade/runs
glade report show latest --runs-dir .glade/runs --json
glade report github latest --runs-dir .glade/runs
glade report export latest --runs-dir .glade/runs --format html --output glade-report.html
```

Serve local Visualforce pages with hot reload. This is a preview feature for
useful local `/apex` rendering, not exact hosted Visualforce behavior:

```bash
glade dev vf --project . --port 8080
glade dev vf --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-vf-ready.json
```

The Visualforce server prints `/apex/<PageName>` routes and watches `.page`,
`.component`, `.cls`, Aura, LWC, and static resource changes.

Serve local LWCs in a Lightning-style shell. This is a preview feature for
local routes, not exact hosted Lightning Experience behavior:

```bash
glade toolchain install
glade dev lwc --project . --open
glade dev lwc --project . --context accountRecord --open
glade dev lwc --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
```

The printed base URL opens the local workbench, with `/lwc` available as a
stable link. The workbench includes a filterable LWC catalog and draft page
composer. The shell also prints `/lwc/preview/component/<namespace>/<component>`,
`/lwc/preview/cmp/<namespace>/<component>?c__name=value`,
`/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>`,
`/lwc/preview/app/<Page>`, `/lwc/preview/home/<Page>`,
`/lwc/preview/tab/<Tab>`, `/lwc/preview/utility/<UtilityBar>`,
`/lwc/preview/flow/<FlowApiName>`,
`/lwc/preview/action/<Object>/<recordId>/<Action>`,
`/lwc/preview/action/global/<Action>`,
`/lwc/preview/community/<site>/<page>`, and
`/lwc/preview/community/<site>/cmp/<namespace>/<component>` routes. Community
routes come from named contexts such as `communityAccount`; named contexts such
as `phase3BaseComponents` can also open fixture-specific service probes.
Visualforce-backed tabs redirect to `/apex/<Page>` and share the same Lightning
Out runtime.

## `glade lsp`

Run the Language Server Protocol server over stdio, or emit one diagnostics pass.

```bash
glade lsp --project .
glade lsp --project . --diagnostics-once
```

## `glade profile analyze`

Read a Glade native trace and emit terminal, JSON, Markdown, or pprof-compatible profile output.

```bash
mkdir -p reports
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade profile analyze reports/trace.json
glade profile analyze reports/trace.json --json
glade profile analyze reports/trace.json --format markdown
glade profile analyze reports/trace.json --format pprof > reports/trace.pb.gz
```

## `glade plugins`

Install, link, list, inspect, lock, and restore executable plugins. First-party
plugins provide advisory scanners and org package capture without putting that
work in the base runtime.

```bash
# Requires a live plugin registry, custom registry, direct archive, or linked plugin.
glade plugins available
glade plugins install @glade/performance
glade plugins install @glade/orgpackage
glade plugins list
glade plugins search quality
glade plugins info @glade/performance
glade plugins doctor
glade plugins which performance
glade plugins lock
glade plugins restore
```

The short aliases `performance` and `orgpackage` resolve to `@glade/performance`
and `@glade/orgpackage`. `available` lists configured registry plugins that can
be installed.

Once installed, plugin command roots behave like Glade commands:

```bash
glade performance scan --project . --json
glade package capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet
```

See [Plugins](/guide/plugins) for first-party install and lock-file docs.

## `glade debug`

Parse, profile, explain, or synthesize from Salesforce debug logs.

```bash
glade debug parse --log apex.log --json
glade debug profile --log apex.log
glade debug profile --log apex.log --format markdown
glade debug explain --log apex.log --project . --json
glade debug editor --log apex.log --project . --json
glade debug repro --log apex.log --project . > ReproTest.cls
glade debug replay --log apex.log --project . --json
```

`glade debug profile --log apex.log` profiles a Salesforce debug log. `glade profile analyze reports/trace.json` profiles a Glade native trace.

## `glade editor` and `glade dap`

Install and check editor integrations, or run the Debug Adapter Protocol server.

```bash
glade editor doctor vscode
glade editor install vscode --force
glade dap --project .
```

## `glade package`

Build, inspect, validate, diff, and capture managed package artifacts.

```bash
glade package build --project . --namespace pkg --output pkg.json --progress
glade package info pkg.json --json
glade package validate pkg.json
glade package diff old.json pkg.json --json
glade plugins install @glade/orgpackage
glade package capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet
```

`glade package capture` is a bridge to the `orgpackage` plugin. Base Glade keeps
artifact loading, type checking, and local runtime behavior; the plugin owns
live Salesforce org capture.

## `glade server`

Start the local Salesforce API baseline. Use `--db` for persistence and `--limit-mode` for execute-anonymous limit behavior.

```bash
glade server --addr 127.0.0.1:8080
glade server --project . --db .glade/refinement-local.sqlite --addr 127.0.0.1:8080
glade server --project . --limit-mode strict
```

## `glade org`

Create, start, and register a local Glade org target for supported `sf`
commands. This is a Glade-backed local API target, not a Salesforce scratch org.

```bash
glade org create refinement-local
glade org start refinement-local --project .
glade org auth refinement-local --project .
sf data create record -o refinement-local -s FileRow__c -v "Name='Refine 01'"
sf apex run -o refinement-local -f scripts/refinement.apex
```

See [Use Glade as an sf target](/guide/glade-orgs) for the data import path and
support boundary.

## `glade db`

Seed, reset, export, inspect, query, and describe local org storage fixtures.

```bash
glade db reset --db .glade/refinement-local.sqlite --json
glade db seed --wizard --db .glade/refinement-local.sqlite --project . data/file-rows.json
glade db seed --db .glade/refinement-local.sqlite --project . --progress data/file-rows.json
glade db import sf --target-org devhub --db .glade/refinement-local.sqlite --project . --object Account --fields Id,Name --limit 25 --json
glade db import sf --target-org devhub --list-objects --category custom --json
glade db inspect --db .glade/refinement-local.sqlite --json
glade db query --db .glade/refinement-local.sqlite --project . --json "SELECT Id, Name FROM FileRow__c"
glade db describe --db .glade/refinement-local.sqlite --project . --json FileRow__c
glade db export --db .glade/refinement-local.sqlite > refinement-export.json
```

## `glade playground`

Start the local browser playground for editing classes, running anonymous Apex, and inspecting logs, limits, traces, and org diffs.

```bash
glade playground --db .glade/playground/org.sqlite --addr 127.0.0.1:1789 --open
glade playground --examples --db .glade/playground/org.sqlite
glade playground --example deal-desk-discount-guard --once
glade playground --list-examples
glade playground --no-db --addr 127.0.0.1:1789 --open
glade playground --examples --reset-on-start
glade playground --project . --db .glade/playground/org.sqlite
glade playground --wizard --project . --examples
```

Useful flags:

- `--list-examples` prints built-in example ids, names, file counts, and tags. Add `--project-ref name=path` to include a local project reference.
- `--example <id>` enables built-in examples and starts at `/playground/?example=<id>`. It uses the managed scratch workspace and cannot be combined with `--project` or `--project-ref`.
- `--no-db` keeps org state in memory and writes no `.glade/playground/org.sqlite`.
- `--reset-on-start` clears the scratch workspace and local org state before serving. It refuses `--project <path>` so project source is not deleted.

Built-in examples:

| ID | Name | Command |
| --- | --- | --- |
| `contact-relationship-drill` | Account + Contact Query | `glade playground --example contact-relationship-drill` |
| `refinement-service` | Refinement Service | `glade playground --example refinement-service` |
| `trigger-contact-task` | Before Insert Trigger | `glade playground --example trigger-contact-task` |
| `bulk-trigger-rollup` | Bulk Trigger Rollup | `glade playground --example bulk-trigger-rollup` |
| `collection-selector` | Collection Selector | `glade playground --example collection-selector` |
| `deal-desk-discount-guard` | Deal Desk Discount Guard | `glade playground --example deal-desk-discount-guard` |
| `limit-counter-drill` | Governor Counter Drill | `glade playground --example limit-counter-drill` |
| `governor-limits-strict` | Governor Limits (strict) | `glade playground --example governor-limits-strict` |
| `map-selector-drill` | Map Selector Drill | `glade playground --example map-selector-drill` |
| `org-diff-review-loop` | Org Diff Review Loop | `glade playground --example org-diff-review-loop` |
| `org-diff-dml` | Org Diff after DML | `glade playground --example org-diff-dml` |
| `persist-mode-ledger` | Persist Mode Ledger | `glade playground --example persist-mode-ledger` |
| `renewal-health-scorecard` | Renewal Health Scorecard | `glade playground --example renewal-health-scorecard` |

::: tip Local workbench
Open the local playground with examples:

```bash
glade playground --examples --addr 127.0.0.1:1789 --open
```
:::
