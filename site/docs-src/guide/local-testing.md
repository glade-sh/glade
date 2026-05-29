# Local Testing

`glade test` discovers Apex test classes from an SFDX project, compiles supported code, runs tests in the local VM, and reports stable outcomes. It uses the same project loader, parser, semantic analyzer, storage, DML, SOQL, trigger, and limit stack as the rest of the CLI.

## Run all tests

```bash
glade test --project .
```

Machine-readable output:

```bash
glade test --project . --json
```

JUnit output for CI:

```bash
glade test --project . --junit reports/glade-junit.xml
```

## Filter tests

Run a test class:

```bash
glade test --project . --filter AccountServiceTest
```

Run a single method:

```bash
glade test --project . --filter AccountServiceTest.testCreatesAccount
```

Use filters for the short inner loop. Then run the broader suite before shipping.

## Limit modes

Glade supports limit modes for local execution. `strict` enforces supported governor limits closer to Salesforce behavior. `permissive` keeps the local loop moving when a project depends on areas Glade has not finished yet.

```bash
glade test --project . --limit-mode strict
glade test --project . --limit-mode permissive --json
```

Use `strict` for gates. Use `permissive` when you are carving into unsupported terrain and want the next useful diagnostic.

## Watch mode

Run tests on file changes:

```bash
glade test --project . --watch
```

Run one affected-test pass and exit:

```bash
glade test --project . --watch-once
```

Run the daemon-backed watch loop:

```bash
glade test --project . --daemon --watch
```

## CI pattern

A small CI gate can check the project, run affected tests, then write JUnit output for test reporting:

```bash
glade check --project . --json
glade test --project . --changed-since origin/main --json
glade test --project . --junit reports/glade-junit.xml
```

## Outcomes

Local test and compatibility gates separate normal test failures from load errors, compile errors, unsupported features, and internal errors. That split matters. A failing assertion and an unsupported platform API leave different tracks.

::: tip Try it
Exercise the runtime your tests rely on — DML, triggers, and governor limits — in the playground: [play.glade.sh/playground/?example=bulk-trigger-rollup](https://play.glade.sh/playground/?example=bulk-trigger-rollup)
:::
