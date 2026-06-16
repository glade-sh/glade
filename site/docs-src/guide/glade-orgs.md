# Use Glade as an sf target

`glade org` creates a local Salesforce-style target for tools that speak to
`sf`. It is a local Glade server and SQLite database. It is not a real scratch
org, and Salesforce remains the validation gate.

## Create a local org

Create a named local target from the project root:

```bash
glade org create my-glade-org
```

The database holds local SObject rows. The project supplies Apex, schema, and
metadata shape. By default, Glade writes the database to
`.glade/orgs/my-glade-org.sqlite` and picks the next loopback address starting
at `127.0.0.1:17911`.

Pass `--project` when creating the target from outside the project root:

```bash
glade org create my-glade-org --project /path/to/project
```

Use `--db` or `--addr` only when you want to pin the database path or local
server address:

```bash
glade org create my-glade-org --db .glade/orgs/my-glade-org.sqlite --addr 127.0.0.1:17911
```

## Start the local org

Start the local server from the saved target:

```bash
glade org start my-glade-org --project .
```

The target uses Glade's local Salesforce API routes. Keep it on loopback unless
an authenticating reverse proxy stands in front of it.

## Register it with sf

Write the target into an `sf` config directory:

```bash
glade org auth my-glade-org --project .
```

Use a temporary config when testing scripts:

```bash
SF_CONFIG_DIR="$(mktemp -d)" glade org auth my-glade-org --project .
```

`glade org list`, `status`, `start`, and `auth` read the saved target from the
project's `.glade/orgs` directory. Run them from the project root or pass
`--project <root>`.

## Run sf data commands

Point `sf data` at the Glade alias:

```bash
sf data create record -o my-glade-org -s Account -v "Name='Local' External_Id__c='acct-1'"
sf data query -o my-glade-org -q "SELECT Id, Name FROM Account WHERE External_Id__c = 'acct-1'"
```

Some installed `sf` versions use `-o`; some plugin commands still document
`-u`. Follow the target-org flag used by the installed command.

## Run sf apex

Run anonymous Apex against the local runtime:

```bash
printf "insert new Account(Name = 'Apex Local', External_Id__c = 'apex-1');\n" > scripts/seed.apex
sf apex run -o my-glade-org -f scripts/seed.apex
```

## Run NimbleAMS data import

NimbleAMS-shaped imports can target the same alias when their route needs stay
inside Glade's supported local API set:

```bash
sf nimbleams data import -f ./data/insertOrder.json -u my-glade-org
```

The sample fixture in
`testdata/local-tests/glade-org-nimbleams/insertOrder.json` covers ordered
Account and Contact records, external-id references, and an ApexScript cleaner.

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
