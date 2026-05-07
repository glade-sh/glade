# E4 Standard Schema Quick Scan

Date: 2026-05-06

Lane: E4 standard schema/SObject token quick scan.

## Scope

This note records the bounded NPSP check for the top six-project local-test
baseline blocker:

- Baseline artifact: `docs/fixtures/local-tests-example-projects.json`
- Project: `example-projects/NPSP-rel-3.237`
- Top blocker: `TDTM_CampaignMember.trigger:30`
- Error: `trigger "TDTM_CampaignMember" references unknown SObject "CampaignMember"`

## Command

```bash
go run ./cmd/oaer compat local-tests --project example-projects/NPSP-rel-3.237 --blockers-only --top-failures 3 --timeout 15000 --json
```

Result:

- `ready=false`
- `durationMs=5916`
- `total=4938`
- `compileGap=4938`
- The first reported outcomes still point at
  `force-app/tdtm/triggers/TDTM_CampaignMember.trigger:30` with unknown
  SObject `CampaignMember`.

## Finding

This is a generated standard-schema coverage gap, not a project metadata
loading/discovery issue.

Evidence:

- `internal/storage/standard_schema_generated.go` includes `Campaign`, but not
  `CampaignMember`, in `standardObjectKeyPrefixData`.
- `internal/storage/standard_schema_generated.go` includes a catalog entry for
  `Campaign`, but no catalog entry for `CampaignMember`.
- `internal/storage/standard_fields.go` only treats generated catalog entries,
  standard key prefixes, and explicit fallback shapes as known standard
  objects. Without a generated or fallback `CampaignMember` shape,
  `IsKnownStandardObject("CampaignMember")` is false.
- `docs/STANDARD_OBJECT_SCHEMA.md` documents the current generated standard
  spread and does not list `CampaignMember`.

## Next Fix

Refresh the public describe-driven standard schema baseline to include
`CampaignMember` rather than adding an NPSP-specific workaround. The minimum
useful generated shape should include the object key prefix and common fields
such as `CampaignId`, `ContactId`, `LeadId`, `Status`, `HasResponded`, and
standard audit/ownership fields, preserving the existing generated-file flow
through `scripts/generate-standard-schema.mjs`.
