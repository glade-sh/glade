# Work with local data

Use a local Glade org when tests, previews, or scripts need records without a
scratch org. Keep Salesforce as the validation gate for live org services.

## Before you start

Run from the project root. Keep seed data in source control when a workflow
depends on it. The default project DB is `.glade/envs/dev.sqlite`; use
`--env <name>` for another project-local DB.

## Steps

Create a local org:

```bash
glade org create refinement-local
```

Start it for the current project:

```bash
glade org start refinement-local --project .
```

Expose it as an sf target:

```bash
glade org auth refinement-local --project .
```

Query through sf:

```bash
sf data query -o refinement-local -q "SELECT Id, Name FROM Account"
```

Seed a local database:

```bash
glade db seed --project . data/file-rows.json
```

Import a small slice from an org already connected through the Salesforce CLI:

```bash
glade db import sf --target-org devhub --project . --object Account --fields Id,Name --limit 25 --json
```

Inspect the database:

```bash
glade db inspect --project .
```

Open the data board when you want inspect, query, seed, reset, and export in
one terminal surface:

```bash
glade db --ui --project .
glade db --ui --project . --target-org devhub --object Account
```

In VS Code, use `Glade: Open Data TUI` for the active local data environment.

## Expected output

The org commands create and start a local target. `sf data query` returns local
records through the Glade target. Seed, import, and inspect commands report the
SQLite tables and rows Glade can use.

## Common wrong turn

Do not use local data as proof of live auth, hosted sharing, external services,
or production data behavior. It is a deterministic local model.

Do not point a project at another project's SQLite file. Glade stores project
schema metadata in the DB and rejects mismatches before local runtime commands
load the wrong object or field shape.

## Deeper reference

- [Local org and data](/guide/modules/local-org-data)
- [Use Glade as an sf target](/guide/glade-orgs)
- [Local API routes](/reference/local-api-routes)
