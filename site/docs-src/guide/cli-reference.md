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

## `glade parse`

Parse Apex files and report parser diagnostics.

```bash
glade parse force-app/main/default/classes/AccountService.cls
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

## `glade check`

Parse and type-check supported Apex and metadata semantics.

```bash
glade check --project .
glade check --project . --json
```

## `glade exec`

Run execute-anonymous Apex against local source and storage.

```bash
glade exec --project . "System.debug('hello from glade');"
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade exec --project . --limit-mode strict "System.debug(Limits.getDmlStatements());"
```

## `glade test`

Discover and run local Apex tests. Useful flags include `--watch`, `--watch-once`, `--changed-since <ref>`, `--daemon`, `--filter`, `--json`, `--junit`, and `--limit-mode`.

```bash
glade test --project .
glade test --project . --filter AccountServiceTest --json
glade test --project . --filter AccountServiceTest.testCreatesAccount --junit reports/glade-junit.xml
glade test --project . --changed-since origin/main --json
glade test --project . --watch
glade test --project . --daemon --watch
glade test --project . --limit-mode permissive --json
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
glade db seed --db .glade/local-org.sqlite seed.json --json
glade db inspect --db .glade/local-org.sqlite --json
glade db export --db .glade/local-org.sqlite > exported-fixture.json
```

## `glade compat`

Run compatibility fixtures, readiness gates, generated capability reports, and large-project local-test triage.

```bash
glade compat mvp
glade compat mvp --require-ready
glade compat matrix --json
glade compat validate fixtures/example.json
glade compat run fixtures/example.json --json
glade compat local-tests --project . --parallel auto --json
glade compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
glade compat gaps --output docs/KNOWN_GAPS.md
glade compat stdlib --output docs/STDLIB_COVERAGE.md
```

## `glade playground`

Start the local browser playground for editing classes, running anonymous Apex, and inspecting logs, limits, traces, and org diffs.

```bash
glade playground --db .glade/playground/org.sqlite --addr 127.0.0.1:1789 --open
glade playground --examples --db .glade/playground/org.sqlite
glade playground --project . --db .glade/playground/org.sqlite
```

::: tip Try it
Open the hosted playground: [play.glade.sh/playground/](https://play.glade.sh/playground/).
:::
