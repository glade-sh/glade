---
pageType: guide
canonicalTask: /guide/glade-orgs
---

# Use Glade as an sf target

`glade org` creates a local Salesforce-style target for tools that speak to
`sf`. It is a local Glade server and SQLite database. It is not a real scratch
org, and Salesforce remains the validation gate.

## Before you start

Use a Salesforce DX project for source and schema. Install the Salesforce CLI
only for the sf integration steps. Choose a new local target alias; creating
a Glade target does not provision a Salesforce org.

## Create a local org

Create a named local target from the project root:

```bash
glade org create refinement-local
```

The database holds local SObject rows. The project supplies Apex, schema, and
metadata shape. By default, Glade writes the database to
`.glade/orgs/refinement-local.sqlite` and picks the next loopback address starting
at `127.0.0.1:17911`.

Pass `--project` when creating the target from outside the project root:

```bash
glade org create refinement-local --project /path/to/apex-project
```

Use `--db` or `--addr` only when you want to pin the database path or local
server address:

```bash
glade org create refinement-local --db .glade/orgs/refinement-local.sqlite --addr 127.0.0.1:17911
```

## Start the local org

Start the local server from the saved target:

```bash
glade org start refinement-local --project .
```

The target uses Glade's local Salesforce API routes. Keep it on loopback unless
an authenticating reverse proxy stands in front of it.

## Register it with sf

Write the target into an `sf` config directory:

```bash
glade org auth refinement-local --project .
```

Use a temporary config when testing scripts:

```bash
SF_CONFIG_DIR="$(mktemp -d)" glade org auth refinement-local --project .
```

`glade org list`, `status`, `start`, and `auth` read the saved target from the
project's `.glade/orgs` directory. Run them from the project root or pass
`--project <root>`.

## Run sf data commands

Point `sf data` at the Glade alias:

```bash
sf data create record -o refinement-local -s FileRow__c -v "Name='Refine 01' External_Id__c='file-1'"
sf data query -o refinement-local -q "SELECT Id, Name FROM FileRow__c WHERE External_Id__c = 'file-1'"
```

Some installed `sf` versions use `-o`; some plugin commands still document
`-u`. Follow the target-org flag used by the installed command.

## Run sf apex

Run anonymous Apex against the local runtime:

```bash
printf "RefinementService.run();\n" > scripts/refinement.apex
sf apex run -o refinement-local -f scripts/refinement.apex
```

## Run manifest-based data imports

Manifest-based imports can target the same alias when their route needs stay
inside Glade's supported local API set:

```bash
sf data import tree -p ./data/fileRowsImport.json -o refinement-local
```

The checked import fixture under `testdata/local-tests/glade-org-data-import`
covers ordered records, external-id references, and an ApexScript cleaner.

## Supported locally

- REST query and SObject create, read, update, delete, and external-id upsert.
- Tooling executeAnonymous for supported Apex runtime paths.
- SOAP Apex executeAnonymous for `sf apex run`.
- Partner SOAP describe and upsert routes used by local data import tools.
- Bulk API v1 CSV insert and upsert baseline when the local route set includes it.
- SQLite-backed local data that can be queried with `glade db query`.

## Not supported locally

- A Glade org is not a Salesforce scratch org.
- It does not deploy, retrieve, package, or provision Salesforce metadata.
- It does not provide full OAuth, hosted services, Streaming, Pub/Sub, GraphQL,
  or exact production governor accounting.
- It does not replace a final run in Salesforce before release.
