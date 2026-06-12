# Use the Local Playground

`glade playground` starts a local browser workbench for Apex snippets, SOQL, DML, logs, limits, traces, and local org diffs. It runs from your machine and can use built-in examples, a scratch workspace, or an SFDX project.

## Start with built-in examples

Load built-in examples:

```bash
glade playground --examples --addr 127.0.0.1:1789
```

Start on one built-in example:

```bash
glade playground --example deal-desk-discount-guard --addr 127.0.0.1:1789 --open
```

`--example` uses the managed scratch workspace. It cannot be combined with `--project` or `--project-ref`.

## Start from an SFDX project

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

Print a ready command without starting the server:

```bash
glade playground --wizard --project . --examples
```

## Persist or reset local state

Persist playground org state to SQLite:

```bash
glade playground --db .glade/playground/org.sqlite --addr 127.0.0.1:1789 --open
```

Clear the scratch workspace and local org state before startup:

```bash
glade playground --examples --reset-on-start --addr 127.0.0.1:1789 --open
```

`--reset-on-start` refuses `--project` so it does not delete project source.

## Use memory-only state

Use memory-only org state:

```bash
glade playground --no-db --addr 127.0.0.1:1789 --open
```

Start a local playground server:

```bash
glade playground --addr 127.0.0.1:1789 --open
```

## List examples

Print local examples without starting a server:

```bash
glade playground --list-examples
```

## Built-in examples

| Group | ID | Name | Command |
| --- | --- | --- | --- |
| Data and SOQL | `contact-relationship-drill` | Account + Contact Query | `glade playground --example contact-relationship-drill` |
| Data and SOQL | `account-service` | Account Factory + Selector | `glade playground --example account-service` |
| Triggers and DML | `trigger-contact-task` | Before Insert Trigger | `glade playground --example trigger-contact-task` |
| Triggers and DML | `bulk-trigger-rollup` | Bulk Trigger Rollup | `glade playground --example bulk-trigger-rollup` |
| Data and SOQL | `collection-selector` | Collection Selector | `glade playground --example collection-selector` |
| Business workflow | `deal-desk-discount-guard` | Deal Desk Discount Guard | `glade playground --example deal-desk-discount-guard` |
| Limits | `limit-counter-drill` | Governor Counter Drill | `glade playground --example limit-counter-drill` |
| Limits | `governor-limits-strict` | Governor Limits (strict) | `glade playground --example governor-limits-strict` |
| Data and SOQL | `map-selector-drill` | Map Selector Drill | `glade playground --example map-selector-drill` |
| Org diff and persistence | `org-diff-review-loop` | Org Diff Review Loop | `glade playground --example org-diff-review-loop` |
| Org diff and persistence | `org-diff-dml` | Org Diff after DML | `glade playground --example org-diff-dml` |
| Org diff and persistence | `persist-mode-ledger` | Persist Mode Ledger | `glade playground --example persist-mode-ledger` |
| Business workflow | `renewal-health-scorecard` | Renewal Health Scorecard | `glade playground --example renewal-health-scorecard` |

## What the playground shows

- Load built-in examples and inspect every source file.
- Try a small Apex behavior before adding it to a project.
- Inspect SOQL, DML, limits, traces, and org state in one place.
- Reproduce a compatibility gap before writing a fixture.

## Troubleshooting

Use `--reset-on-start` with built-in examples when the scratch workspace or
local org state gets stale. Use `--no-db` when a run should leave no state on
disk.

::: tip Try it
Start the local playground with built-in examples:

```bash
glade playground --examples --addr 127.0.0.1:1789 --open
```
:::
