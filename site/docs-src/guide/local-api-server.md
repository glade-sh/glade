# Run local Salesforce API routes

`glade server` starts local Salesforce-style HTTP routes backed by the same runtime used by the CLI. Use it for local integration tests, tools, and development loops that need REST-style org behavior without a live Salesforce org.

## Start the server

Use an ephemeral in-memory org:

```bash
glade server --addr 127.0.0.1:8080
```

Load project metadata and persist data to SQLite:

```bash
glade server --project . --db .glade/refinement-local.sqlite --addr 127.0.0.1:8080
```

Use strict or permissive limit behavior for supported execute paths:

```bash
glade server --project . --limit-mode strict
```

## Seed local data

Prepare a local database before starting the server:

```bash
glade db reset --db .glade/refinement-local.sqlite --json
glade db seed --wizard --db .glade/refinement-local.sqlite --project . data/file-rows.json
glade db seed --db .glade/refinement-local.sqlite --project . --progress data/file-rows.json
glade db inspect --db .glade/refinement-local.sqlite --json
```

## REST routes

The server exposes a Salesforce-style baseline for local work: API discovery,
object describe and CRUD-style record operations, SOQL query execution, limits
and record counts, source-backed Tooling metadata reads, virtual schema metadata
queries, Composite sObject inserts, Composite Batch and Tree local requests,
Bulk API v2 simple scalar query job create/status/whole-result CSV routes,
layout/default-value metadata, metadata job status, and execute-anonymous routes
where supported by the runtime.

Check [what Glade runs locally](/guide/support-map) before relying on live auth,
Composite Graph execution, Bulk API locator paging,
Streaming, Pub/Sub, GraphQL, live metadata deploy/retrieve, or Tooling APIs
outside the checked local source/schema metadata baseline.

| Area | Endpoint | Status |
| --- | --- | --- |
| API discovery | `/services/data/` | supported |
| Describe | `/services/data/vXX.X/sobjects/<Object>/describe` | supported baseline |
| Query | `/services/data/vXX.X/query?q=...` | supported baseline |
| SObject CRUD | `/services/data/vXX.X/sobjects/<Object>/<Id>` | supported baseline |
| Record counts | `/services/data/vXX.X/limits/recordCount?sObjects=Account,Contact` | supported baseline |
| Execute Anonymous | Tooling executeAnonymous route | supported where runtime supports code |
| SOAP Apex executeAnonymous | `/services/Soap/s/vXX.X/<OrgId>` | supported for `sf apex run` |
| Partner SOAP describe/upsert | `/services/Soap/u/vXX.X` | supported baseline for local data import tools |
| Tooling source metadata | Tooling `ApexClass`, `ApexTrigger`, `ApexPage`, `ApexComponent`, `StaticResource`, `CustomObject`, `CustomField`, `Layout`, `CompactLayout`, `RecordType`, and `ValidationRule` query/read paths | supported local baseline |
| Tooling schema metadata | Tooling `EntityDefinition`, `EntityParticle`, `FieldDefinition`, and `RelationshipDomain` query paths | supported local baseline |
| Composite sObject insert | `/services/data/vXX.X/composite/sobjects` | supported baseline |
| Composite Batch | `/services/data/vXX.X/composite/batch` | supported local subrequests |
| Composite Tree | `/services/data/vXX.X/composite/tree/<Object>` | supported local tree requests |
| Bulk API v1 ingest | `/services/async/vXX.X/job...` | supported insert/upsert CSV baseline |
| Bulk API v2 query | `/services/data/vXX.X/jobs/query` and `/results` | supported simple scalar local query whole-result CSV |
| Layout and default metadata | local layout/default-value REST routes | supported local metadata baseline |
| Metadata job status | local metadata job status routes | supported local status baseline |
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
