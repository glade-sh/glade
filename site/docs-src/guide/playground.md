# Playground

`glade playground` starts a local browser workbench for Apex snippets, SOQL, DML, logs, limits, traces, and local org diffs. It runs from your machine and can use built-in examples, a scratch workspace, or an SFDX project.

## Local playground

Start a local playground server:

```bash
glade playground --addr 127.0.0.1:1789 --open
```

Load built-in examples:

```bash
glade playground --examples --addr 127.0.0.1:1789
```

Start on one built-in example:

```bash
glade playground --example deal-desk-discount-guard --addr 127.0.0.1:1789 --open
```

`--example` uses the managed scratch workspace. It cannot be combined with `--project` or `--project-ref`.

Print local examples without starting a server:

```bash
glade playground --list-examples
```

Persist playground org state to SQLite:

```bash
glade playground --db .glade/playground/org.sqlite --addr 127.0.0.1:1789 --open
```

Use memory-only org state:

```bash
glade playground --no-db --addr 127.0.0.1:1789 --open
```

Clear the scratch workspace and local org state before startup:

```bash
glade playground --examples --reset-on-start --addr 127.0.0.1:1789 --open
```

Start from an SFDX project:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

`--reset-on-start` refuses `--project` so it does not delete project source.

Print a ready command without starting the server:

```bash
glade playground --wizard --project . --examples
```

## Built-in examples

| ID | Name | Command |
| --- | --- | --- |
| `contact-relationship-drill` | Account + Contact Query | `glade playground --example contact-relationship-drill` |
| `account-service` | Account Factory + Selector | `glade playground --example account-service` |
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

## What to use it for

- Load built-in examples and inspect every source file.
- Try a small Apex behavior before adding it to a project.
- Inspect SOQL, DML, limits, traces, and org state in one place.
- Reproduce a compatibility gap before writing a fixture.

::: tip Try it
Start the local playground with built-in examples:

```bash
glade playground --examples --addr 127.0.0.1:1789 --open
```
:::
