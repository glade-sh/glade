# Salesforce Surface Packet: Ledger.Identity

- Area: Ledger.Identity
- Owner: internal/surfaceledger
- Ledger row filter: `identity joins across docs/org/glade/evidence`
- Ratchet target: paired missing/stale rows decrease for identity examples

## dependsOn

- `source shelf green`

## mayRunInParallelWith

- `External.MarketingCloud.AMPscript`
- `AI.Agentforce`

## sharedFiles

- `docs/generated/salesforce/**`

## exclusiveFiles

- `internal/surfaceledger/**`

## Allowed files

- `internal/surfaceledger/**`
- `internal/gladecli/compat_surface_command.go`
- `docs/plans/**`

## Blocked files

- `internal/vm/**`
- `internal/dml/**`
- `internal/soql/**`

## Required fixtures

- `focused ledger rows for FeatureManagement, Database.Batchable, and ApexPages.Component`

## Focused tests

- `go test ./internal/surfaceledger`

## Done criteria

- `false split rows are joined before feature packets start`

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
go test ./internal/surfaceledger && go run ./cmd/glade compat surface refresh --docs "$GLADE_SALESFORCE_DOCS_SOURCE" --tooling-completions testdata/generated/tooling_system_symbols.json.gz --out "$(mktemp -d)"
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
