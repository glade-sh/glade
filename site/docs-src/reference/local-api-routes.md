---
pageType: reference
canonicalTask: /guide/workflows/local-data
---

# Local API routes

Use the route map and request examples below for the supported local HTTP
baseline. Endpoint versions are separate from Apex source and LWC API versions.

Use this page when you need the local Salesforce-style HTTP route map.
The detailed guide carries the route groups and request examples.

## Start here

Start `glade server` when a local integration test or tool needs org-shaped HTTP.
Add `--project .` when source metadata matters.
Add `--db <path>` when the local data should persist.

The server covers a checked local baseline.
It does not implement live OAuth, Streaming, Pub/Sub, GraphQL, or hosted metadata deploy and retrieve.
Keep it on a trusted local interface unless a real auth layer stands in front.

## Route map and requests

<!--@include: ../guide/local-api-server.md#local-api-route-reference-->

## Detailed source

[Run local Salesforce API routes](/guide/local-api-server)

## Related workflows

- [Local org and data](/guide/modules#local-org-and-data)
- [Local data workflow](/guide/workflows/local-data)
- [Glade orgs](/guide/glade-orgs)
