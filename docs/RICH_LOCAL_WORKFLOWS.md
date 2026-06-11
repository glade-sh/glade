# Rich Local Workflows

These commands keep local work visible while it runs and easy to repeat after it
finishes.

## Progress

Longer local commands write progress to stderr. Machine-readable command output
stays on stdout.

```bash
glade parse force-app --progress
glade package build --project . --namespace pkg --output .glade/pkg.json --progress
glade db seed --db .glade/org.sqlite --project . --progress fixtures/dev.json
```

Use `--progress-json` when an editor or wrapper wants NDJSON progress events.
Use `--no-progress` for silent scripts. Commands that already had `--quiet`
keep it as an alias.

## DB Seed Wizard

Use the wizard when you know the fixture and database path, but want the full
repeatable command before mutating local data.

```bash
glade db seed --wizard --db .glade/org.sqlite --project . fixtures/dev.json
```

It prints a seed command with progress enabled and an inspect command for the
same database.

## Playground Wizard

Use the playground wizard when you want a ready serve command without starting
the server yet.

```bash
glade playground --wizard --project . --examples
```

It prints a `glade playground` command with the current project, data root,
workspace, examples, public mode, limits, and browser choice reflected in the
flags you provided.

## Package Artifacts

Build an artifact with progress:

```bash
glade package build \
  --project . \
  --namespace pkg \
  --version 1.2.3 \
  --output .glade/pkg-1.2.3.json \
  --progress
```

Inspect it:

```bash
glade package info .glade/pkg-1.2.3.json
glade package info .glade/pkg-1.2.3.json --json
```

Validate it:

```bash
glade package validate .glade/pkg-1.2.3.json
glade package validate .glade/pkg-1.2.3.json --json
```

Compare two artifacts:

```bash
glade package diff .glade/pkg-1.2.2.json .glade/pkg-1.2.3.json
glade package diff .glade/pkg-1.2.2.json .glade/pkg-1.2.3.json --json
```

The diff reports added, removed, and changed global Apex types, custom objects,
and source hash changes.
