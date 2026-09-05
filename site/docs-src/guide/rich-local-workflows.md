---
pageType: reference
canonicalTask: /guide/rich-local-workflows
---

# Reports and package artifacts

These commands keep local work visible while it runs and easy to repeat after it
finishes.

The git examples assume `origin/main` is your intended base ref and is available
locally. Substitute the correct existing ref for your repository before running
changed-test or refactor commands.

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
glade tui --project . --db .glade/org.sqlite --view data --target-org devhub --object Account
```

The test and data commands can open the same boards:

```bash
glade test --ui --project .
glade db --ui --db .glade/org.sqlite --project .
```

## DB seed wizard

Use the wizard when you know the fixture and database path, but want the full
repeatable command before mutating local data.

```bash
glade db seed --wizard --db .glade/org.sqlite --project . fixtures/dev.json
```

It prints a seed command with progress enabled and an inspect command for the
same database.

## DB import from Salesforce

Use `glade db import sf` when you want a small, repeatable slice of data from an
org already connected through the Salesforce CLI. The command queries `sf`,
converts rows into a Glade fixture, applies it to the SQLite database, and then
prints the normal inspect output.

```bash
glade db import sf --target-org devhub --db .glade/org.sqlite --project . --object Account --fields Id,Name --limit 25 --json
glade db import sf --target-org devhub --list-objects --category custom --json
```

Always confirm the intended org before import. Omitting `--target-org` uses
the Salesforce CLI default target org; use an explicit authorized alias for
repeatable workflows. Generated
object imports default to 25 rows. Use `--query` instead of `--object` when you
need a hand-written SOQL cut.

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
glade plugins install @glade/orgpackage
glade package capture --target-org packaging --namespace pkg --output .glade/packages/pkg.glade-package.json --config-snippet
```

The diff reports added, removed, and changed global Apex types, custom objects,
and source hash changes.

Use `glade package build` when the package source is on disk. Use
`glade package capture` when a local project depends on an installed package
but should not carry the package source. It dispatches to
`glade orgpackage capture` when `@glade/orgpackage` is installed or linked.
The captured artifact supplies global Apex signatures, schema, labels, static
resources, and code-intelligence symbols.

Consume a captured artifact from `glade.yml`:

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

## Enterprise workflows

Map and prove larger Apex changes with local evidence:

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
as editor-facing LSP features. The reports show severity, confidence, evidence,
recommendations, and known limitations. Public and global package APIs are
review or deprecate candidates, not safe-delete candidates.

See [Enterprise Workflows](/guide/enterprise-workflows) for the report contract,
known limits, and CI proof commands.
