# Local org and data

Local org and data tools own named local environments and SQLite-backed
project state. They serve Salesforce-style local API routes for development.
They do not own live Salesforce auth or hosted org services.

## Use it when

Use local org and data tools when you need named local state, repeatable data
sets, Salesforce-style REST routes, or a project DB for local execution.

## Entry commands

```bash
glade org create refinement-local
glade org auth refinement-local --project .
glade server --project . --db .glade/refinement-local.sqlite --addr 127.0.0.1:8080
```

## What this module owns

- Named local org records and project auth to a local environment.
- SQLite-backed SObjects, SOQL, DML, triggers, and local Tooling routes.
- Local API server routes for development clients and test harnesses.

## Requires Salesforce

Salesforce remains the validation gate for hosted platform behavior.

Use Salesforce for live OAuth, org identity, metadata deploy or retrieve,
Streaming, Pub/Sub, GraphQL, and hosted Tooling routes outside the local model.

## Related workflows

- [Local data workflow](/guide/workflows/local-data)
- [Glade orgs](/guide/glade-orgs)

## Reference

- [Local API routes](/reference/local-api-routes)
