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
mkdir -p reports
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

Use exact class and method selectors for the short inner loop. Then run the wider suite before shipping.

## Limit modes

Glade supports limit modes for local execution. `strict` enforces supported governor limits closer to Salesforce behavior. `permissive` keeps the local loop moving when a project depends on unfinished areas.

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

## LWC dev shell

The LWC local shell is a preview feature. It gives useful local preview routes,
not exact hosted Lightning Experience behavior.

Install the local LWC toolchain first:

```bash
glade toolchain install
```

Serve local LWCs from the project on disk:

```bash
glade dev lwc --project . --open
```

Open a preset from `glade.lwc.json`:

```bash
glade dev lwc --project . --context accountRecord --open
```

The local context endpoint at `/lightning/local/context.json` reports the
active route, PageReference, mounted components, discovered apps, named
contexts, selected context, diagnostics, routes, and service support.

Use a separate context file when presets live outside the project-root
`glade.lwc.json`:

```bash
glade dev lwc --project . --context-file config/lwc-contexts.json --context accountRecord --open
```

Or open one explicit route:

```bash
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
```

Use `--target url-addressable`, `--target record-action`, or `--target
global-action` with `--component`, `--action`, `--object`, and `--record` when
you need those shell contexts without a named preset.

Use `--port 8080` for the common localhost shortcut. Use `--addr` when scripts
need a full bind address.

The `/lwc` workbench lists contexts, routes, mounted components, and
diagnostics. The startup banner still lists discovered routes:

```text
/lwc
/lwc/preview/component/<namespace>/<component>
/lwc/preview/cmp/<namespace>/<component>?c__name=value
/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>
/lwc/preview/app/<Page>
/lwc/preview/home/<Page>
/lwc/preview/tab/<Tab>
/lwc/preview/action/<Object>/<recordId>/<ActionName>
/lwc/preview/action/global/<ActionName>
```

Record, app, home, tab, URL-addressable component, and quick action routes
resolve LWC bundle metadata, FlexiPages, custom applications, custom tabs, and
quick action metadata. Visualforce-backed tabs redirect to `/apex/<Page>`. That
redirect and the shared Lightning Out runtime are the Visualforce boundary in
this LWC loop.

The shell supports local Apex controller imports, `getRecord`, `getRecords`,
`getRecordCreateDefaults`, local DML-backed create/update/delete record helpers,
record-input helper functions, `lightning/uiObjectInfoApi` object info and
picklist wires, `lightning/uiRelatedListApi` child rows, schema tokens, labels,
static resources, content assets, `CurrentPageReference`, basic
`NavigationMixin` behavior, local message service, resource loading, toast
events, practical common base components, and SLDS-shaped local styling. Create
defaults include project layout field sections when available, with a generated
full layout from createable fields as the local fallback. `lightning/uiLayoutApi`
`getLayout` returns the same local Record Layout shape. Glade loads fixture
records from `data/*.json` when they use the Glade storage fixture format.

Tools can read current local shell state at:

```text
/lightning/local/context.json
```

See [Local LWC Shell](/guide/lwc-local-shell) for routes, fixtures, and limits.

## Visualforce dev server

Visualforce local rendering is a preview feature. It serves useful local
`/apex/<PageName>` previews, not exact hosted Visualforce behavior.

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
`PageReference.getContent*` output, or byte-for-byte PDF output.

Form posts carry signed Visualforce view state and a CSRF token. Controller and
extension state comes back across posts, while Apex fields marked `transient`
stay out of the saved state. Lightning Out pages validate `ltng:outApp`
dependencies before creating local LWC modules, so a missing Aura dependency or
component name reports a local rendering diagnostic instead of a browser fetch
failure.

## Warm startup across CLI runs

Large projects rebuild local org state and helper compilation on cold start.
`glade test` writes that harness to `.glade/test/startup.meta.json` plus a
hashed payload after the first cold build and reloads it when fingerprint checks
pass.

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
mkdir -p reports
glade test --project . --junit reports/glade-junit.xml
```

Saved run artifacts and CI annotations are covered in [Add Glade to CI](/guide/ci-artifacts).

## Outcomes

Local test runs separate assertion failures from load errors, compile errors,
unsupported features, and internal errors. That split matters. A failing
assertion means the test ran and failed. An unsupported feature means
the runtime stopped at a known unsupported Salesforce API.

```text
  ✓  AccountServiceTest.testCreatesAccount  42ms
  ✗  AccountServiceTest.testRejectsBlankName  12ms

  AccountServiceTest.testRejectsBlankName
  System.AssertException: expected 1, got 0

  force-app/main/default/classes/AccountServiceTest.cls:42
```

Check [what Glade runs locally](/guide/support-map) before relying on platform
service APIs, exact hosted Visualforce behavior, live side effects, or REST
behavior outside the checked local baseline.

::: tip Try it
Exercise the runtime your tests rely on - DML, triggers, and governor limits - in the local playground:

```bash
glade playground --examples --addr 127.0.0.1:1789 --open
```
:::
