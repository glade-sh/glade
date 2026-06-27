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

## Terminal UI

Use `glade tui` when you want one terminal surface for common local work. It
opens boards for project checks, local tests, persistent data, and plugins.

```bash
glade tui --project .
glade tui --project . --view tests
glade tui --project . --db .glade/org.sqlite --view data
```

The test and data commands can open the same boards:

```bash
glade test --ui --project .
glade db --ui --db .glade/org.sqlite --project .
```

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

Build an artifact with progress when the package source is in the checkout:

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

Capture installed package contracts from an org when the local project depends
on package APIs but should not carry package source:

```bash
glade plugins install @glade/orgpackage
glade package capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet
```

`glade package capture` dispatches to `glade orgpackage capture` when
`@glade/orgpackage` is installed or linked.

Use the captured artifact from `glade.yml`:

```yaml
project:
  managedPackageDependencies: ["pkg:artifact:.glade/packages/pkg.glade-package.json:1.2.3.4"]
```

Captured package methods compile from signatures. They do not run local behavior
unless a shim root supplies source:

```yaml
project:
  packageShims: ["pkg:test-support/package-shims/pkg"]
```

The artifact remains the contract. `packageShims` supplies local source bodies
under the package namespace.

## Enterprise Workflows

Use enterprise reports when a large Apex project needs a map before it needs
edits.

```bash
glade inspect graph --project . --json
glade inspect definition --project . --symbol Account.Name
glade inspect references --project . --symbol RefinementService.total --json
glade refactor rename --project . --symbol RefinementService --to FileRefinementService --dry-run --json
mkdir -p reports
glade report assess --project . --format html --out reports/glade-assessment.html
glade report cruft --project . --format html --out reports/glade-cruft.html
glade report refactor-proof --project . --since origin/main --format html --out reports/glade-refactor-proof.html
```

Definition, reference, and rename commands use the same code-intelligence graph
as editor-facing LSP features. Assessment and cruft reports use static evidence
and confidence levels. Refactor proof records what Glade checked, what it did
not run, and which public or global API surfaces need review.
