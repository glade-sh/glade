# Run a Local Salesforce-Shaped API

`glade server` starts a local Salesforce-shaped HTTP surface backed by the same runtime used by the CLI. It is meant for local integration tests, tools, and development loops that need REST-shaped org behavior without a live Salesforce org.

## Start the server

Use an ephemeral in-memory org:

```bash
glade server --addr 127.0.0.1:8080
```

Load project metadata and persist data to SQLite:

```bash
glade server --project . --db .glade/local-org.sqlite --addr 127.0.0.1:8080
```

Use strict or permissive limit behavior for supported execute paths:

```bash
glade server --project . --limit-mode strict
```

## Seed local data

Prepare a local database before starting the server:

```bash
glade db reset --db .glade/local-org.sqlite --json
glade db seed --wizard --db .glade/local-org.sqlite --project . seed.json
glade db seed --db .glade/local-org.sqlite --project . --progress seed.json
glade db inspect --db .glade/local-org.sqlite --json
```

## REST surface

The server exposes a Salesforce-shaped baseline for local work: API discovery,
object describe and CRUD-style record operations, SOQL query execution, limits
and record counts, source-backed Tooling metadata reads, virtual schema metadata
queries, Composite sObject inserts, Composite Batch local subrequests,
Bulk API v2 simple scalar query job create/status/whole-result CSV routes,
and execute-anonymous routes where supported by the runtime.

Check the [Support map](/guide/support-map) before relying on full auth,
Composite Graph/Tree, broader Bulk API including locator paging, Streaming, Pub/Sub, GraphQL, metadata
deploy/retrieve jobs, or Tooling surfaces outside the checked local
source/schema metadata baseline.

| Area | Endpoint shape | Status |
| --- | --- | --- |
| API discovery | `/services/data/` | supported |
| Describe | `/services/data/vXX.X/sobjects/<Object>/describe` | supported baseline |
| Query | `/services/data/vXX.X/query?q=...` | supported baseline |
| SObject CRUD | `/services/data/vXX.X/sobjects/<Object>/<Id>` | supported baseline |
| Record counts | `/services/data/vXX.X/limits/recordCount?sObjects=Account,Contact` | supported baseline |
| Execute Anonymous | Tooling executeAnonymous route | supported where runtime supports code |
| Tooling source metadata | Tooling `ApexClass`, `ApexTrigger`, `ApexPage`, `ApexComponent`, `StaticResource`, `CustomObject`, `CustomField`, `Layout`, `CompactLayout`, `RecordType`, and `ValidationRule` query/read paths | supported local baseline |
| Tooling schema metadata | Tooling `EntityDefinition`, `EntityParticle`, `FieldDefinition`, and `RelationshipDomain` query paths | supported local baseline |
| Composite sObject insert | `/services/data/vXX.X/composite/sobjects` | supported baseline |
| Composite Batch | `/services/data/vXX.X/composite/batch` | supported local subrequests |
| Bulk API v2 query | `/services/data/vXX.X/jobs/query` and `/results` | supported simple scalar local query whole-result CSV |
| Glade reset endpoints | `/services/data/vXX.X/glade/reset` and scoped reset routes | supported local-only baseline |

Example request:

```bash
curl -s http://127.0.0.1:8080/services/data/
curl -s http://127.0.0.1:8080/services/data/v60.0/sobjects/Account/describe
curl -s 'http://127.0.0.1:8080/services/data/v60.0/query?q=SELECT+Id,Name+FROM+Account'
curl -s 'http://127.0.0.1:8080/services/data/v60.0/limits/recordCount?sObjects=Account'
```

Example query response:

```json
{
  "totalSize": 1,
  "done": true,
  "records": [
    {
      "attributes": {
        "type": "Account"
      },
      "Name": "Twin Lakes"
    }
  ]
}
```

::: warning Local server only
The local API server does not implement full OAuth or production Salesforce
authentication. Do not expose it to untrusted networks unless an authenticating
reverse proxy stands in front of it.
:::

::: tip Try it
Use the playground when you want the same local org ideas with a browser UI:
[Use the Local Playground](/guide/playground).
:::
