# Reports and package artifacts

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

## DB seed wizard

Use the wizard when you know the fixture and database path, but want the full
repeatable command before mutating local data.

```bash
glade db seed --wizard --db .glade/org.sqlite --project . fixtures/dev.json
```

It prints a seed command with progress enabled and an inspect command for the
same database.

## Playground wizard

Use the playground wizard when you want a ready serve command without starting
the server yet.

```bash
glade playground --wizard --project . --examples
```

## Package artifacts

```bash
glade package build --project . --namespace pkg --version 1.2.3 --output .glade/pkg-1.2.3.json --progress
glade package info .glade/pkg-1.2.3.json --json
glade package validate .glade/pkg-1.2.3.json
glade package diff .glade/pkg-1.2.2.json .glade/pkg-1.2.3.json --json
```

The diff reports added, removed, and changed global Apex types, custom objects,
and source hash changes.

## Enterprise workflows

Map and prove larger Apex changes with local evidence:

```bash
glade inspect graph --project . --json
glade report assess --project . --format html --out reports/glade-assessment.html
glade report cruft --project . --format html --out reports/glade-cruft.html
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
```

The reports show severity, confidence, evidence, recommendations, and known
limitations. Public and global package surfaces are review or deprecate
candidates, not safe-delete candidates.

See [Enterprise Workflows](/guide/enterprise-workflows) for the report contract,
known limits, and CI proof commands.
