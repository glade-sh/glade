# CLI Reference

All commands are local unless you point Glade at an external path or start a server. Most project commands accept `--project <root>` and default to the current directory when a project is discoverable.

## `glade version`

Print build and version information.

```bash
glade version
```

## `glade doctor`

Check the local environment and project discovery basics.

```bash
glade doctor
glade doctor --project .
```

## `glade completion`

Generate shell completion scripts.

```bash
source <(glade completion bash)
glade completion zsh > ~/.zsh/completions/_glade
glade completion fish > ~/.config/fish/completions/glade.fish
```

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
glade parse force-app/main/default/classes/AccountService.cls
glade parse force-app/main/default/classes --progress
glade parse force-app/main/default/classes --json
```

## `glade inspect symbols`

Build the project symbol index and print declarations, members, triggers, and schema objects.

```bash
glade inspect symbols --project .
glade inspect symbols --project . --json
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
```

## `glade check`

Parse and type-check supported Apex and metadata semantics.

```bash
glade check --project .
glade check --project . --json
glade check --project . --format sarif --output glade-check.sarif
glade check --project . --format github
```

## `glade exec`

Run execute-anonymous Apex against local source and storage.

```bash
glade exec --project . "System.debug('hello from glade');"
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade exec --project . --limit-mode strict "System.debug(Limits.getDmlStatements());"
```

## `glade test`

Discover and run local Apex tests. Useful flags include `changed --since <ref>`,
`--watch`, `--watch-once`, `--last-failed`, `--wizard`, `--daemon`,
`--connect`, `--no-serve`, `--filter`, `--json`, `--junit`, and `--limit-mode`.

`glade test serve` keeps the runtime warm across CLI invocations. Later runs
auto-connect through `.glade/test/serve.sock` unless `--no-serve` is set.

Warmed harness state is cached on disk at `.glade/test/startup.gob` after a cold
build. See [Test Startup Cache](/guide/test-startup-cache) for freshness rules.

```bash
glade test serve --project .
glade test daemon status --project .
glade test --project .
glade test --project . --filter AccountServiceTest --json
glade test --project . --filter AccountServiceTest.testCreatesAccount --junit reports/glade-junit.xml
glade test --project . --connect --filter AccountServiceTest
glade test changed --project . --since origin/main --json
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
glade test --project . --no-cache --filter AccountServiceTest
```

Run `glade help test` for the full flag list.

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

## `glade lsp`

Run the Language Server Protocol server over stdio, or emit one diagnostics pass.

```bash
glade lsp --project .
glade lsp --project . --diagnostics-once
```

## `glade profile analyze`

Read a native trace and emit JSON or Markdown profile output.

```bash
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade profile analyze reports/trace.json
glade profile analyze reports/trace.json --json
```

## `glade plugins`

Install, link, list, inspect, lock, and restore executable plugins. First-party
plugins provide compatibility fixtures and advisory scanners without putting
that maintenance code in the base runtime.

```bash
glade plugins install @glade/compat
glade plugins install @glade/performance
glade plugins list
glade plugins doctor
glade plugins which compat
glade plugins lock
glade plugins restore
```

The short aliases `compat` and `performance` resolve to `@glade/compat` and
`@glade/performance`.

Once installed, plugin command roots behave like Glade commands:

```bash
glade compat local-tests --project . --json
glade performance scan --project . --json
```

See [Plugins](/guide/plugins) for the author contract and archive layout.

## `glade debug`

Parse, profile, explain, or synthesize from Salesforce debug logs.

```bash
glade debug parse --log apex.log --json
glade debug profile --log apex.log
glade debug explain --log apex.log --project . --json
```

## `glade editor` and `glade dap`

Install and check editor integrations, or run the Debug Adapter Protocol server.

```bash
glade editor doctor vscode
glade editor install vscode --vsix vscode-glade.vsix --force
glade dap --project .
```

## `glade package`

Build, inspect, validate, and diff managed package artifacts.

```bash
glade package build --project . --namespace pkg --output pkg.json --progress
glade package info pkg.json --json
glade package validate pkg.json
glade package diff old.json pkg.json --json
```

## `glade server`

Start the local Salesforce-shaped REST API baseline. Use `--db` for persistence and `--limit-mode` for execute-anonymous limit behavior.

```bash
glade server --addr 127.0.0.1:8080
glade server --project . --db .glade/local-org.sqlite --addr 127.0.0.1:8080
glade server --project . --limit-mode strict
```

## `glade db`

Seed, reset, export, and inspect local org storage fixtures.

```bash
glade db reset --db .glade/local-org.sqlite --json
glade db seed --wizard --db .glade/local-org.sqlite --project . seed.json
glade db seed --db .glade/local-org.sqlite --project . --progress seed.json
glade db inspect --db .glade/local-org.sqlite --json
glade db export --db .glade/local-org.sqlite > exported-fixture.json
```

## `glade playground`

Start the local browser playground for editing classes, running anonymous Apex, and inspecting logs, limits, traces, and org diffs.

```bash
glade playground --db .glade/playground/org.sqlite --addr 127.0.0.1:1789 --open
glade playground --examples --db .glade/playground/org.sqlite
glade playground --project . --db .glade/playground/org.sqlite
glade playground --wizard --project . --examples
```

::: tip Try it
Open the hosted playground: [play.glade.sh/playground/](https://play.glade.sh/playground/).
:::
