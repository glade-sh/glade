# Run Apex Tests Locally

`glade test` discovers Apex test classes from an SFDX project, compiles supported code, runs tests in the local VM, and reports stable outcomes. It uses the same project loader, parser, semantic analyzer, storage, DML, SOQL, trigger, and limit stack as the rest of the CLI.

## Run all tests

```bash
glade test --project .
```

```text
Glade test

1 selected, 1 passed, 0 failed

Selected: 1
Passed:   1
Failed:   0
Runtime:  420ms

  ✓  AccountServiceTest.testCreatesAccount  42ms

Next:
  glade test --watch
  glade test failed
```

Machine-readable output:

```bash
glade test --project . --json
```

`--json` writes the versioned envelope described in [Automation and JSON](/guide/automation).

JUnit output for CI:

```bash
glade test --project . --junit reports/glade-junit.xml
```

## Filter tests

Run a test class:

```bash
glade test --project . --class AccountServiceTest
```

Run a single method:

```bash
glade test --project . --class AccountServiceTest --method testCreatesAccount
```

Use exact class and method selectors for the short inner loop. Then run the broader suite before shipping.

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

Run tests affected by a git ref without remembering the lower-level flag:

```bash
glade test changed --project . --since HEAD
```

Rerun failures from the last completed run:

```bash
glade test failed --project .
glade test --project . --last-failed
```

Print the next likely loop commands without running tests:

```bash
glade test --project . --wizard
```

## Visualforce dev server

Serve local Visualforce pages from the project on disk:

```bash
glade dev vf --project . --addr 127.0.0.1:8080
```

The startup banner lists `/apex/<PageName>` routes and watches `.page`,
`.component`, `.cls`, Aura, LWC, and static resource changes. Rendering errors
show a local overlay with the page file and expression when Glade can identify
them. Current standard-component support rows are available from the local
server:

```bash
curl http://127.0.0.1:8080/services/data/v61.0/glade/visualforce/support
```

Use `--port 8080` for the common localhost shortcut. Use an ephemeral address
and a ready file for scripts:

```bash
glade dev vf --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-vf-ready.json
```

The local renderer covers common standard components, custom components,
controller actions, page messages, expression and form binding, static
resources, uploads, remoting envelopes, Lightning Out/LWC hosts, AJAX refresh
paths, and local PDF fallback output. It does not promise Salesforce-hosted
chrome, every component edge, exact lifecycle timing, Apex
`PageReference.getContent*` parity, or byte-for-byte PDF output.

## Warm startup across CLI runs

Large projects rebuild local org state and helper compilation on cold start.
`glade test` writes that harness to `.glade/test/startup.gob` after the first
cold build and reloads it when fingerprint checks pass.

**[Test Startup Cache](/guide/test-startup-cache)** explains when the cache is
created, how it stays up to date, when it can be wrong, and how to recover.

```bash
glade test serve --project .
glade test daemon status --project .
glade test --project . --class AccountServiceTest
glade test daemon stop --project .
glade test clear-cache --project .
glade test --project . --no-cache --class AccountServiceTest
```

Clear the cache after `git pull` or Glade upgrades. Use `--no-cache` when
debugging harness issues.

## CI pattern

A small CI gate can check the project, run affected tests, then write JUnit output for test reporting:

```bash
glade check --project . --json
glade test changed --project . --since origin/main --json --no-progress
glade test --project . --junit reports/glade-junit.xml
```

Saved run artifacts and CI annotations are covered in [Add Glade to CI](/guide/ci-artifacts).

## Outcomes

Local test runs separate assertion failures from load errors, compile errors,
unsupported features, and internal errors. That split matters. A failing
assertion means the test ran and failed. An unsupported feature means
the runtime stopped at a known unsupported Salesforce surface.

```text
  ✓  AccountServiceTest.testCreatesAccount  42ms
  ✗  AccountServiceTest.testRejectsBlankName  12ms

  AccountServiceTest.testRejectsBlankName
  System.AssertException: expected 1, got 0

  force-app/main/default/classes/AccountServiceTest.cls:42
```

Check the [Support map](/guide/support-map) before relying on platform
service APIs, exact hosted Visualforce behavior, live side effects, or broad
REST parity.

::: tip Try it
Exercise the runtime your tests rely on - DML, triggers, and governor limits - in the local playground:

```bash
glade playground --examples --addr 127.0.0.1:1789 --open
```
:::
