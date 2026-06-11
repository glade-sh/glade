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

Persist playground org state to SQLite:

```bash
glade playground --db .glade/playground/org.sqlite --addr 127.0.0.1:1789 --open
```

Start from an SFDX project:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

Print a ready command without starting the server:

```bash
glade playground --wizard --project . --examples
```

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
