# Playground

The hosted playground runs in the browser at [play.glade.sh/playground/](https://play.glade.sh/playground/). It is the quickest place to try Apex snippets, SOQL, DML, logs, limits, traces, and local org diffs without installing anything.

## Hosted playground

Open the default playground:

```text
https://play.glade.sh/playground/
```

Deep-link to an example by id:

```text
https://play.glade.sh/playground/?example=account-service
https://play.glade.sh/playground/?example=bulk-trigger-rollup
```

Those links are handy in issues, docs, and review comments. A small repro beats a paragraph of hand waving.

## Local playground

Start a local playground server:

```bash
glade playground --addr 127.0.0.1:1789 --open
```

Load built-in examples:

```bash
glade playground --examples --addr 127.0.0.1:1789
```

Persist playground org state to SQLite:

```bash
glade playground --db .glade/playground/org.sqlite --addr 127.0.0.1:1789 --open
```

Start from an SFDX project:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

## What to use it for

- Share runnable examples with a deep link.
- Try a small Apex behavior before adding it to a project.
- Inspect SOQL, DML, limits, traces, and org state in one place.
- Reproduce a compatibility gap before writing a fixture.

::: tip Try it
Open the Account Service example now: [play.glade.sh/playground/?example=account-service](https://play.glade.sh/playground/?example=account-service)
:::
