# Salesforce Surface Packet: Server.ToolingObjects

- Area: Server.ToolingObjects
- Owner: internal/server
- Ledger row filter: `product=tooling`
- Ratchet target: Server.ToolingObjects missing rows do not increase

## dependsOn

- `source shelf green`
- `Ledger.Identity`

## mayRunInParallelWith

- `Core.Runtime.System.FeatureManagement`
- `External.MarketingCloud.AMPscript`

## sharedFiles

- `docs/generated/salesforce/**`
- `testdata/generated/tooling_system_symbols.json.gz`

## exclusiveFiles

- `internal/server/**`
- `internal/storage/**`

## Allowed files

- `internal/server/**`
- `internal/storage/**`

## Blocked files

- `unrelated external docs shelves`
- `corpus-specific runtime exceptions`

## Required fixtures

- `Tooling Objects focused compatibility fixture or explicit unsupported fixture`

## Focused tests

- `go test ./internal/repoguard`

## Done criteria

- `shape, behavior, evidence, capability/docs, and refresh/check are reported in order`

## Rows To Explain First

- No rows matched this packet in the current ledger.

## Baseline Command

```bash
tmp="$(mktemp -d)"
go run ./cmd/glade compat surface refresh \
  --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"
```

## Area ratchet command

```bash
go run ./cmd/glade compat surface check --ledger "$tmp/SURFACE_LEDGER.json" --max-parser-failures 0
```

## Handoff Format

Report focused tests, fixture command, surface refresh, area ratchet command, before counts, after counts, and next top row.

## Standard Validation Block

- focused tests run:
- fixture command run:
- surface refresh run:
- area ratchet command run:
- before counts:
- after counts:
- next top row:
- go test ./internal/repoguard after code changes:

## Docs Defect Path

If a docs row is missing or malformed, choose one path before runtime work:
- re-scrape docs
- copy improved docs into the external docs source
- patch the docs parser to read existing docs correctly
- add a small checked fixture if public docs are ambiguous

## Reviewer Checklist

- no corpus-specific runtime hacks
- public Salesforce behavior cited by docs or fixture
- shape and behavior are not claimed without evidence
- packet area did not expand during work
- generated docs are updated when capability changes

## Breadth Work Order

- Ledger.Identity
- FeatureManagement
- Database.Batchable
- Schema.Describe
- ApexPages.Controllers
- REST.Resources
- Tooling.Objects
