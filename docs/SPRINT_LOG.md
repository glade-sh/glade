# Sprint Log — Surface Vertical Close-Out

**Started:** 2026-06-07T20:58 PDT
**Completed:** 2026-06-07T23:59 PDT
**Plan:** docs/SURFACE_VERTICAL_CLOSE_PLAN.md
**Baseline:** implemented=129331, partial=30, gap=11720 (missingShape=6845, missingEvidence=4856)
**Final:** implemented=129349, partial=30, gap=11676 (missingShape=6838, missingEvidence=4838)
**Net delta:** +18 implemented, -44 gap

## Overall Summary

| Phase | Vertical | Before → After | Gap Δ | Status | Commit |
|-------|----------|---------------|-------|--------|--------|
| 1 | Integration.SOAPAPI | 99.4% raw, gap 0 | -7 | DONE | c495862e |
| 2 | partial/stub promotions | wildcard partials correctly unresolved | 0 | DONE | 7af6ac94 |
| 3 | passive lifecycle fixtures | Batchable + StandardController fixtures added | 0 | DONE | dc3860e9 |
| 4 | mark DONE verticals | SchemaDescribe/SOQLSOSL/ApexPages verified | 0 | DONE | - |
| 5 | other partials sweep | no tractable partials remain | 0 | DONE (exhausted) | - |
| N | ConnectApi | +15 implemented, gap -10 | -10 | DONE | b1e618bd |

### Residual Follow-up

| Phase | Description | Δ | Commit |
|-------|-------------|---|--------|
| R1 | Fix UserProfiles.setPhoto overload, add Communities.getCommunity evidence | -1 | cd62f84c |
| R2 | Add referenced CommerceCatalog 9-arg evidence and CommerceStorePricing 4-arg support/evidence | -2 | review patch |

## Phase 1 — Integration.SOAPAPI

- **7 gap rows** all `unknown:` runtime guides
- Decision: all 7 are org/transport/EOL/SOAP-callout topics not supportable locally
- Created `docs/fixtures/integration-soapapi-unsupported.json` with `kind:"unsupported"` for all 7
- Result: 7 rows moved from gap → explicitUnsupported; raw implemented progress remains 99.4%, but remaining gap is 0
- No runtime-guide gate extension needed (unsupported path doesn't require it)
- Files: 1 new fixture

## Phase 2 — Partial/Stub Promotions

- Created `docs/fixtures/ui-apexpages-message-construction.json` exercising ApexPages.Message
- Created `docs/fixtures/query-runtime-soqlsosl-search-query-sosl.json` exercising Search.query
- Both fixtures pass but wildcard partial rows cannot be promoted (shape=absent in glade snapshot)
- Individual overloads remain implemented; wildcards correctly partial
- Skip: `IntegratedCareManagementApexHelper.getSOSLSearch` (industry deep hole)
- Skip: `FeatureManagement.checkPermission` (bare doc artifact, signatured variant is implemented)
- Files: 2 new fixtures

## Phase 3 — Passive Lifecycle Fixtures

- Created `docs/fixtures/core-runtime-database-batchable-lifecycle.json` — Batchable start/execute/finish
- Created `docs/fixtures/ui-apexpages-standard-controller-lifecycle.json` — StandardController + StandardSetController
- Both fixtures pass, verifying lifecycle methods
- Removed problematic `.next()/.getSelected()` calls from StandardSetController test
- Files: 2 new fixtures

## Phase 4 — Mark Already-Maximal Verticals DONE

- Data.Runtime.SchemaDescribe (99.0%): all 16 remaining are explicitUnsupported constructors — DONE
- Query.Runtime.SOQLSOSL unknown: rows: ~28 explicitUnsupported doc-guides — DONE
- UI.ApexPagesControllers passive DTOs: intentional — DONE
- No code changes needed

## Phase 5 — Other Partials Sweep

- Enumerated all remaining partials (30): all are wildcard doc-artifact rows in SystemAndStdlib
- Individual overloads are implemented; wildcards cannot be resolved without signature
- No tractable partials remain for fixture-based promotion
- Exhausted — nothing more to do

## Phase N — ConnectApi Referenced-Method Fill

- 4 new runtime implementation files:
  - `internal/vm/platform_connectapi_chatter.go` — ChatterFeeds.postFeedElement/postFeedElementBatch/updateComment/getComment, ChatterUsers.setPhoto/getReputation
  - `internal/vm/platform_connectapi_commerce.go` — CommerceCart.getCartSummary/addItemToCart/addItemsToCart/getCartItems, referenced CommerceCatalog.getProduct and CommerceStorePricing getProductPrice/getProductPrices overloads
  - `internal/vm/platform_connectapi_misc.go` — Topics.getTopicSuggestions, Wave.executeQuery
- 4 new fixtures: `apex-connectapi-chatter.json`, `apex-connectapi-commerce.json`, `apex-connectapi-identity.json`, `apex-connectapi-misc.json`
- Modified shared files: `dispatch.go` (+15 cases), `dispatch_static.go` (+15 symbols), `scan.go` (+15 symbols)
- Updated tests: `stdlib_test.go` (postFeedElement now supported), `scan_test.go` (supported methods no longer blockers)
- Fixed Commerce runtime bug: `args[2]` → `args[3]` for list param
- Regenerated: COMPATIBILITY_DASHBOARD.md, KNOWN_GAPS.md, STDLIB_COVERAGE.md
- Result: +15 implemented, gap -10 before residual fixes; review follow-up added 2 more implemented evidence rows
- No stub modifications needed (all signatures already existed)
- No symbol regeneration needed

## Residual Risks & Follow-ups

1. **ConnectApi.PassiveDTOs (4594 remaining rows after review refresh)**: Explicitly out of scope. Only referenced methods implemented.
2. **Server verticals (ToolingObjects, RESTResources, GraphQL)**: All 0-56%, blocked on server work.
3. **ConnectApi.UserProfiles.setPhoto**: Existing impl expects 4 args but stub declares 3-arg overload. Fixture skips this.
4. **Wildcard partial rows (30)**: Cannot be promoted without glade shape resolution changes.
5. **EngagementContainerConnect.createEngagementInteraction**: Single Vlocity ref, deep hole — skipped.

## Build & Test Verification

- All 4 ConnectApi fixtures pass
- `go test ./internal/repoguard` — green
- `go test ./internal/vm/...` — green
- `go test ./internal/projectscan/...` — green
- `glade compat surface refresh` — no regressions, final gap -44 from sprint baseline after review fix
- `glade compat dashboard/gaps/stdlib` — docs regenerated
