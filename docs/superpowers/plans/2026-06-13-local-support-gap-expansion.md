# Local Support Gap Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the Salesforce and Apex surfaces that Glade can support locally, starting with the 9 partial post-MVP capability lanes and the 76 partial standard-library rows, while preserving explicit unsupported fences for hosted Salesforce services.

**Architecture:** Treat local support as three layers: product runtime behavior in `/Users/matt/Dev/glade`, compatibility and surface evidence in `/Users/matt/Dev/glade-tools`, and public support docs in `/Users/matt/Dev/glade`. A row only moves from `partial` to `supported` when runtime behavior, fixture evidence, generated support docs, and public wording all agree.

**Tech Stack:** Go, Glade VM/server/runtime packages, VitePress docs, first-party `glade-tools` compat and surface ledger, JSON fixture files under `glade-tools/docs/fixtures`, Salesforce docs mirror at `/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs`.

---

## Current Checked Baseline

Run these commands first. They are the measuring stick.

```bash
cd /Users/matt/Dev/glade
sed -n '1,80p' docs/KNOWN_GAPS.md
sed -n '1,90p' docs/COMPATIBILITY_DASHBOARD.md
sed -n '55,145p' site/docs-src/guide/support-map.md
awk -F'|' '/\| `partial` \|/ {n++} END {print "stdlib partial rows:", n+0}' docs/STDLIB_COVERAGE.md
awk -F'|' '/\| `unsupported` \|/ {n++} END {print "stdlib unsupported rows:", n+0}' docs/STDLIB_COVERAGE.md
```

Expected current baseline:

| Measure | Current state |
| --- | ---: |
| Required MVP gaps | 0 |
| Required supported capabilities | 21/21 |
| Post-MVP partial capability lanes | 9 |
| Standard-library partial rows | 76 |
| Standard-library unsupported rows | 2 |
| Surface implemented rows | 130266 |
| Surface explicit unsupported rows | 6338 |
| Missing shape/behavior/evidence/failure rows | 0 |

The two exact unsupported standard-library rows are:

- `Answers.findSimilar(Question)` - hosted Answers zone search data.
- `ResetPasswordResult.getPassword()` - hosted identity password output.

Keep `ResetPasswordResult.getPassword()` unsupported. Consider `Answers.findSimilar` only after the local Search/SOSL engine can support deterministic Question/Knowledge matching from local data.

## Priority Order

1. **High value, local, bounded:** standard-library rows with pure local semantics: JSON, Pattern/Matcher, String split, Decimal, EncodingUtil, Type, HTTP DTOs, WebServiceCallout mock routing.
2. **High value, local with metadata:** Schema/describe, BusinessHours, QuickAction, Approval DTO/process result shapes, Messaging template rendering, Search/SOSL, Test helpers.
3. **High value, server-facing:** broader local API server coverage: Composite Batch/Graph, Bulk ingest/query, layout/default-value metadata, local metadata retrieve/deploy simulation.
4. **Useful developer experience:** LSP context completion, DAP launch orchestration, watch/profile summaries, pprof and wall-clock profiling.
5. **Later phase:** exact governor accounting and configurable cap profiles.
6. **Do not pursue locally:** full OAuth/session validation, live org-only Tooling execution, delivered email, real outbound callouts, full Visualforce rendering/PDF generation, hosted identity/admin mutations, `ResetPasswordResult.getPassword`.

## Shared File Map

Product repo: `/Users/matt/Dev/glade`

- `/Users/matt/Dev/glade/internal/vm/dispatch.go` - main stdlib/platform dispatch.
- `/Users/matt/Dev/glade/internal/vm/dispatch_static.go` - static method registration.
- `/Users/matt/Dev/glade/internal/vm/stdlib.go` - Pattern/Matcher, string helpers, scalar stdlib.
- `/Users/matt/Dev/glade/internal/vm/stdlib_string.go` - String behavior.
- `/Users/matt/Dev/glade/internal/vm/stdlib_number.go` - Decimal and number behavior.
- `/Users/matt/Dev/glade/internal/vm/json_runtime.go` - JSON serialize/deserialize dispatch.
- `/Users/matt/Dev/glade/internal/vm/json_parser.go` - JSON parser model.
- `/Users/matt/Dev/glade/internal/vm/json_generator.go` - JSON generator model.
- `/Users/matt/Dev/glade/internal/vm/email_runtime.go` - Messaging and email DTO behavior.
- `/Users/matt/Dev/glade/internal/vm/business_hours_runtime.go` - BusinessHours local calendar model.
- `/Users/matt/Dev/glade/internal/vm/approval_process_runtime.go` - Approval process/result behavior.
- `/Users/matt/Dev/glade/internal/vm/request_runtime.go` - request context and local harness behavior.
- `/Users/matt/Dev/glade/internal/vm/test_support_runtime.go` - Test.* helpers.
- `/Users/matt/Dev/glade/internal/vm/describe_runtime.go` - Schema/describe behavior.
- `/Users/matt/Dev/glade/internal/vm/soql_runtime.go` - SOQL/SOSL runtime surfaces.
- `/Users/matt/Dev/glade/internal/vm/limits.go` - governor counters and cap handling.
- `/Users/matt/Dev/glade/internal/vm/platform_feature_management.go` - FeatureManagement local behavior.
- `/Users/matt/Dev/glade/internal/server/server.go` - route registration.
- `/Users/matt/Dev/glade/internal/server/composite_handlers.go` - Composite handlers.
- `/Users/matt/Dev/glade/internal/server/tooling_metadata_bulk.go` - Tooling metadata and bulk-ish helpers.
- `/Users/matt/Dev/glade/internal/server/source_metadata.go` - local source metadata reads.
- `/Users/matt/Dev/glade/internal/server/describe_payloads.go` - describe and quick action payloads.
- `/Users/matt/Dev/glade/internal/lsp/features.go` - LSP completion, hover, symbol behavior.
- `/Users/matt/Dev/glade/internal/dap/session.go` and `/Users/matt/Dev/glade/internal/dap/handler.go` - DAP session behavior.
- `/Users/matt/Dev/glade/internal/profile/profile.go` - profile report output.
- `/Users/matt/Dev/glade/internal/watch/events.go` and `/Users/matt/Dev/glade/internal/watch/affected.go` - watch events and affected-test logic.
- `/Users/matt/Dev/glade/docs/STDLIB_COVERAGE.md` - generated stdlib coverage.
- `/Users/matt/Dev/glade/docs/COMPATIBILITY_DASHBOARD.md` - generated capability dashboard.
- `/Users/matt/Dev/glade/docs/KNOWN_GAPS.md` - generated required-gap report.
- `/Users/matt/Dev/glade/site/docs-src/guide/support-map.md` - public support map.

Tools repo: `/Users/matt/Dev/glade-tools`

- `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go` - stdlib support rows and notes.
- `/Users/matt/Dev/glade-tools/internal/capability/capability.go` - post-MVP capability rows and notes.
- `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go` - support row guard tests.
- `/Users/matt/Dev/glade-tools/internal/surfaceledger/*` - surface evidence and ledger classification.
- `/Users/matt/Dev/glade-tools/internal/compat/run.go` - fixture runner.
- `/Users/matt/Dev/glade-tools/docs/fixtures/*.json` - evidence fixtures and unsupported fences.

## Phase 0: Baseline, Worktree, and Work Queue

**Purpose:** Give the worker current counts and a sorted work queue. This phase changes no files.

**Files:**
- Read: `/Users/matt/Dev/glade/docs/STDLIB_COVERAGE.md`
- Read: `/Users/matt/Dev/glade/docs/COMPATIBILITY_DASHBOARD.md`
- Read: `/Users/matt/Dev/glade/site/docs-src/guide/support-map.md`
- Read: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`
- Read: `/Users/matt/Dev/glade-tools/internal/capability/capability.go`

- [ ] **Step 0.1: Create paired worktrees**

Run:

```bash
cd /Users/matt/Dev/glade
git status --short --branch
git worktree add .worktrees/local-support-gap-expansion -b codex/local-support-gap-expansion main

cd /Users/matt/Dev/glade-tools
git status --short --branch
git worktree add /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools -b codex/local-support-gap-expansion-tools main
ln -s /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion /Users/matt/Dev/glade/.worktrees/glade
```

Expected:

- Both source repos start from clean local `main`.
- Product worktree is `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion`.
- Tools worktree is `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools`.
- The symlink exists only to satisfy the tools repo `../glade` replace path during checks.

- [ ] **Step 0.2: Refresh support counts**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
tmp="$(mktemp -d)"
go run ./cmd/glade-tools surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"
printf 'SURFACE_TMP=%s\n' "$tmp"
sed -n '1,80p' "$tmp/SURFACE_DASHBOARD.md"
```

Expected:

```text
surface refresh: ok
implemented=130266 partial=1 passive=47494 stubNoOp=262 explicitUnsupported=6338
gaps: missingShape=0 missingBehavior=0 missingEvidence=0
failures: parser=0 docsOrgMismatch=0 staleGlade=0 passiveServiceRisk=0
```

- [ ] **Step 0.3: Extract exact standard-library partial rows**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
awk -F'|' '/\| `partial` \|/ {
  gsub(/^ +| +$/, "", $2)
  gsub(/^ +| +$/, "", $3)
  gsub(/^ +| +$/, "", $5)
  print $2 "\t" $3 "\t" $5
}' docs/STDLIB_COVERAGE.md > /tmp/glade-stdlib-partial.tsv
cat /tmp/glade-stdlib-partial.tsv
```

Expected: 76 rows.

- [ ] **Step 0.4: Split the work queue**

Create `/tmp/glade-local-support-work-queue.md` with these buckets:

```markdown
# Glade Local Support Work Queue

## Pure Local Semantics
- JSON serialize/deserialize rows
- Pattern/Matcher/String.split rows
- Decimal/EncodingUtil/Crypto rows
- Type.forName row
- HTTP DTO rows
- WebServiceCallout mock rows

## Metadata-Backed Local Semantics
- Schema/Describe rows
- BusinessHours rows
- QuickAction rows
- Approval.process rows
- Messaging rows
- Search/SOSL rows
- Test helper rows

## Capability Lanes
- server.rest-breadth
- stdlib.platform-breadth
- lsp.context-completion
- dap.live-ide-orchestration
- profile.pprof-and-timing
- watch.profile-trace-reports
- compat.fixture-expansion
- release.distribution-automation
- limits.exact-accounting
```

Expected: the worker has a list that can be split across subagents with disjoint file ownership.

## Phase 1: Pure Local Standard Library Rows

**Purpose:** Close rows where Salesforce parity is mostly language/runtime semantics, not hosted service behavior. This is the best first implementation phase.

**Candidate rows:**
- `JSON.deserialize`, `JSON.deserializeStrict`, `JSON.deserializeUntyped`, `JSON.serialize`, `JSON.serializePretty`
- `Pattern.compile`, `Pattern.matches`, `Matcher.find`, `Matcher.group`, `Matcher.matches`, `String.split`
- `Decimal.round`, `Decimal.setScale`
- `EncodingUtil.urlDecode`, `EncodingUtil.urlEncode`
- `Crypto.generateDigest`
- `Type.forName`
- `HttpRequest`, `HttpResponse`
- `WebServiceCallout.invoke(...)`

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/json_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/json_parser.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/json_generator.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/stdlib.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/stdlib_string.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/stdlib_number.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/dispatch.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/internal/capability/stdlib.go`
- Create or modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/docs/fixtures/core-runtime-pure-stdlib-closeout.json`

- [ ] **Step 1.1: Write JSON fixture before code**

Create or extend `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/docs/fixtures/core-runtime-pure-stdlib-closeout.json` with one fixture named `core-runtime-pure-stdlib-closeout`. Include evidence rows for the five JSON APIs. Use this Apex source:

```apex
public class PureStdlibJsonDTO {
  public String name;
  public Integer count;
  public List<String> tags;
}

@isTest private class PureStdlibJsonCloseoutTest {
  @isTest static void jsonRoundTripsSupportedShapes() {
    PureStdlibJsonDTO dto = new PureStdlibJsonDTO();
    dto.name = 'local';
    dto.count = 3;
    dto.tags = new List<String>{'a', 'b'};

    String compact = JSON.serialize(dto);
    System.assert(compact.contains('"name"'));
    System.assert(compact.contains('"tags"'));

    String pretty = JSON.serializePretty(dto);
    System.assert(pretty.contains('\n'));

    PureStdlibJsonDTO parsed = (PureStdlibJsonDTO)JSON.deserialize(compact, PureStdlibJsonDTO.class);
    System.assertEquals('local', parsed.name);
    System.assertEquals(3, parsed.count);
    System.assertEquals('b', parsed.tags[1]);

    PureStdlibJsonDTO strictParsed = (PureStdlibJsonDTO)JSON.deserializeStrict(compact, PureStdlibJsonDTO.class);
    System.assertEquals('local', strictParsed.name);

    Map<String, Object> untyped = (Map<String, Object>)JSON.deserializeUntyped('{"ok":true,"items":[1,2],"nested":{"x":"y"}}');
    System.assertEquals(true, untyped.get('ok'));
    System.assertEquals('y', ((Map<String, Object>)untyped.get('nested')).get('x'));
  }
}
```

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go run ./cmd/glade-tools run docs/fixtures/core-runtime-pure-stdlib-closeout.json
```

Expected before implementation: either fixture failure naming the missing edge behavior, or pass if the row already has enough behavior and only needs evidence/status update.

- [ ] **Step 1.2: Implement only failing JSON behavior**

Modify the JSON files listed above. Keep the implementation to the exact failure from Step 1.1. Do not add broad parser rewrites. Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
go test ./internal/vm -run 'JSON|Json' -count=1
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go run ./cmd/glade-tools run docs/fixtures/core-runtime-pure-stdlib-closeout.json
```

Expected: both commands pass.

- [ ] **Step 1.3: Write Pattern/String fixture before code**

Add evidence rows and Apex test source to `core-runtime-pure-stdlib-closeout.json` for:

- `Pattern.compile`
- `Pattern.matches`
- `Matcher.find`
- `Matcher.group`
- `Matcher.matches`
- `String.split`

Use this Apex source:

```apex
@isTest private class PureStdlibRegexCloseoutTest {
  @isTest static void regexRowsUseSupportedJavaSubset() {
    Pattern p = Pattern.compile('(a+)-(b+)', Pattern.CASE_INSENSITIVE);
    Matcher m = p.matcher('AA-bb zz');
    System.assertEquals(true, m.find());
    System.assertEquals('AA-bb', m.group());
    System.assertEquals('AA', m.group(1));
    System.assertEquals('bb', m.group(2));
    System.assertEquals(true, Pattern.matches('\\Q[A-Z]+\\E', '[A-Z]+'));

    List<String> split = 'one,two,three'.split(',');
    System.assertEquals(3, split.size());
    System.assertEquals('two', split[1]);
  }
}
```

Run the fixture. If it fails because the Java subset is deliberately fenced, keep the row partial and record the exact fenced feature in `internal/capability/stdlib.go`.

- [ ] **Step 1.4: Close Decimal, EncodingUtil, Crypto, Type, HTTP DTOs**

Use one small fixture class per family in `core-runtime-pure-stdlib-closeout.json`. Required test behaviors:

```apex
@isTest private class PureStdlibScalarCloseoutTest {
  @isTest static void decimalEncodingCryptoAndTypesAreDeterministic() {
    System.assertEquals(2, Decimal.valueOf('1.5').round());
    System.assertEquals('1.24', String.valueOf(Decimal.valueOf('1.235').setScale(2)));
    System.assertEquals('a b', EncodingUtil.urlDecode('a+b', 'UTF-8'));
    System.assertEquals('a%2Bb', EncodingUtil.urlEncode('a+b', 'UTF-8'));
    System.assertNotEquals(null, Crypto.generateDigest('SHA-512', Blob.valueOf('abc')));
    System.assertEquals('Account', Type.forName('Account').getName());
  }

  @isTest static void httpDtosExposeCommonMutableState() {
    HttpRequest req = new HttpRequest();
    req.setEndpoint('https://example.invalid');
    req.setMethod('POST');
    req.setHeader('X-Test', '1');
    req.setBody('body');
    System.assertEquals('https://example.invalid', req.getEndpoint());
    System.assertEquals('POST', req.getMethod());
    System.assertEquals('1', req.getHeader('X-Test'));
    System.assertEquals('body', req.getBody());

    HttpResponse res = new HttpResponse();
    res.setStatusCode(201);
    res.setStatus('Created');
    res.setHeader('X-Reply', '2');
    res.setBody('ok');
    System.assertEquals(201, res.getStatusCode());
    System.assertEquals('Created', res.getStatus());
    System.assertEquals('2', res.getHeader('X-Reply'));
    System.assertEquals('ok', res.getBody());
  }
}
```

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
go test ./internal/vm -run 'Stdlib|Regex|JSON|Decimal|Encoding|Crypto|Type|Http' -count=1
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go run ./cmd/glade-tools run docs/fixtures/core-runtime-pure-stdlib-closeout.json
```

- [ ] **Step 1.5: Promote only proven rows**

In `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/internal/capability/stdlib.go`, change a row from `StatusPartial` to `StatusSupported` only when all of these are true:

- The fixture passes.
- The notes no longer contain a real boundary such as "not modeled", "no live transport", or "external service".
- The behavior is local and deterministic.
- The surface refresh shows no new missing evidence.

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go test ./internal/capability ./internal/surfaceledger -count=1
go run ./cmd/glade-tools stdlib --output /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/STDLIB_COVERAGE.md
```

Expected: supported row count increases and partial row count falls.

## Phase 2: Metadata-Backed Local Standard Library Rows

**Purpose:** Improve rows that can be modeled from local project metadata and local org data.

**Candidate rows:**
- Schema and Describe rows
- BusinessHours rows
- QuickAction rows
- Approval.process rows
- Messaging rows
- Search/SOSL rows
- Test helper rows

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/describe_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/business_hours_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/approval_process_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/email_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/soql_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/test_support_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/server/describe_payloads.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/internal/capability/stdlib.go`
- Create or modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/docs/fixtures/core-runtime-metadata-backed-stdlib-closeout.json`

- [ ] **Step 2.1: Schema/describe closeout**

Write a fixture with local object metadata for Account custom fields, RecordType, picklist values, and a child relationship. Test:

```apex
@isTest private class MetadataBackedSchemaCloseoutTest {
  @isTest static void localDescribeReadsProjectMetadata() {
    Map<String, Schema.SObjectType> globalMap = Schema.getGlobalDescribe();
    System.assert(globalMap.containsKey('Account'));

    Schema.DescribeSObjectResult accountDescribe = Schema.describeSObjects(new List<String>{'Account'})[0];
    System.assertEquals('Account', accountDescribe.getName());
    System.assert(accountDescribe.fields.getMap().containsKey('Name'));
    System.assert(accountDescribe.getRecordTypeInfos().size() > 0);

    Schema.DescribeFieldResult nameDescribe = Account.Name.getDescribe();
    System.assertEquals('Name', nameDescribe.getName());
    System.assertEquals(true, nameDescribe.isCreateable());
  }
}
```

Add fixture metadata files under the fixture `source` array, not product testdata. Run the fixture first, then implement missing describe behavior.

- [ ] **Step 2.2: BusinessHours closeout**

Add local `BusinessHours` records and test week schedule math. Do not claim holiday/service-calendar parity until local Holiday metadata is modeled. Target support promotion only if holiday and service-calendar limitations are removed. Otherwise strengthen fixture evidence and leave row partial.

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
go test ./internal/vm -run BusinessHours -count=1
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go run ./cmd/glade-tools run docs/fixtures/core-runtime-metadata-backed-stdlib-closeout.json
```

- [ ] **Step 2.3: QuickAction and Approval**

Keep these local and deterministic:

- QuickAction describes read project quickAction metadata.
- QuickAction perform methods return DTOs for supported request shapes.
- Approval.process returns deterministic result DTOs for submit/workitem shapes.
- No live UI action service.
- No live approval engine routing.

If live service wording remains true, keep rows partial. The useful goal is stronger fixture coverage, not false support.

- [ ] **Step 2.4: Messaging and template rendering**

Target local support for:

- `Messaging.SingleEmailMessage` common getters/setters.
- `Messaging.sendEmail` result shape, validation, and limit increments.
- `Messaging.renderStoredEmailTemplate` from local metadata/static resources.

Keep no-delivery and Salesforce content attachment boundaries explicit. Promote only rows whose local contract is complete despite no delivery.

- [ ] **Step 2.5: Search/SOSL**

Build a local search contract over local org records:

- deterministic SOSL `RETURNING` rows
- field projection
- fixed search result IDs
- `Search.find`
- `Search.query`
- `Search.suggest`
- AccessLevel permission behavior where already modeled

Keep ranking, stemming, synonyms, language, and external suggestion services out of scope. Consider `Answers.findSimilar(Question)` only after this search contract is stable.

- [ ] **Step 2.6: Test helper rows**

Close what is local:

- `Test.createStubQueryRow`
- `Test.createStubQueryRows`
- `Test.setCurrentPageReference(Object)` for PageReference-compatible objects
- `Test.getEventBus()` local event delivery
- `Test.getExternalService()` deterministic callback harness
- `Test.loadData` CSV/static-resource behavior
- `Test.setMock` HttpCalloutMock/WebServiceMock registration
- `Test.startTest` and `Test.stopTest` supported counters and async drain

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
go test ./internal/vm ./internal/apextest -run 'Test|Mock|EventBus|ExternalService|LoadData|StartTest|StopTest' -count=1
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go run ./cmd/glade-tools run docs/fixtures/core-runtime-metadata-backed-stdlib-closeout.json
```

## Phase 3: Local API Server Breadth

**Purpose:** Move `server.rest-breadth` from a broad partial lane toward a measured set of locally useful APIs.

**Do first:**
- Composite Batch
- Composite Graph with local request references
- Bulk API v2 ingest into local SQLite
- Bulk API v2 query over local SOQL
- layout/default-value metadata reads from local project metadata
- local metadata retrieve/deploy simulation for project files only

**Do not do:**
- real OAuth/session validation
- live org-only Tooling object execution
- Streaming/PubSub delivery
- GraphQL against Salesforce services

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/server/server.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/server/composite_handlers.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/server/tooling_metadata_bulk.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/server/source_metadata.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/server/describe_payloads.go`
- Test: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/server/server_test.go`
- Create: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/docs/fixtures/server-local-api-breadth-closeout.json`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/internal/capability/capability.go`

- [ ] **Step 3.1: Composite Batch tests**

Add Go tests in `internal/server/server_test.go`:

```go
func TestCompositeBatchRunsLocalSubrequests(t *testing.T) {
    // Create local server with Account metadata and SQLite/in-memory org.
    // POST /services/data/v60.0/composite/batch with one Account insert and one query.
    // Assert hasErrors=false and each result has Salesforce-shaped statusCode/body.
}
```

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
go test ./internal/server -run CompositeBatch -count=1
```

Expected before implementation: fail with missing route or unsupported response.

- [ ] **Step 3.2: Composite Graph tests**

Add tests that insert an Account and reference its ID in a second graph node. Keep graph execution serial and deterministic. Reject cross-graph features that need Salesforce transaction semantics.

- [ ] **Step 3.3: Bulk API v2 local ingest/query**

Implement a narrow local contract:

- create ingest job
- upload CSV
- close job
- inspect successful/failed counts
- query local records through SOQL and return CSV/JSON result chunks

Test with `go test ./internal/server -run Bulk -count=1`.

- [ ] **Step 3.4: Layout/default-value metadata**

Read local layout metadata and default values from project files. Return stable REST/Tooling shapes. Do not infer missing org metadata.

- [ ] **Step 3.5: Update capability row and docs**

Only after passing server fixtures:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go test ./internal/capability ./internal/surfaceledger -count=1
go run ./cmd/glade-tools dashboard --output /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/COMPATIBILITY_DASHBOARD.md

cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
npm --prefix site test
npm --prefix site run build
```

Update `site/docs-src/guide/local-api-server.md` and `site/docs-src/guide/support-map.md` with exact supported routes and exact boundaries.

## Phase 4: Developer Experience Partial Lanes

**Purpose:** Improve useful local workflows without changing the runtime contract.

**Capability lanes:**
- `lsp.context-completion`
- `dap.live-ide-orchestration`
- `profile.pprof-and-timing`
- `watch.profile-trace-reports`

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/lsp/features.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/lsp/handler.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/dap/session.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/dap/handler.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/profile/profile.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/watch/events.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/watch/affected.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/internal/capability/capability.go`

- [ ] **Step 4.1: LSP context completion**

Add tests for completion ranking in `internal/lsp/handler_test.go`:

- inside `SELECT ... FROM Account`, fields rank above Apex keywords.
- after `new List<`, Apex types rank above fields.
- after `Account.`, static members do not swamp SObject fields.

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
go test ./internal/lsp -run Completion -count=1
```

- [ ] **Step 4.2: DAP orchestration**

Add tests for launch/run settings translating into `glade test` or `glade exec` debug sessions. Keep transport and stepping behavior unchanged.

Run:

```bash
go test ./internal/dap ./internal/gladecli -run 'Launch|Debug|DAP' -count=1
```

- [ ] **Step 4.3: Profile outputs**

Add pprof-compatible CPU output and wall-clock statement attribution from trace events. Test output shape, not absolute timings.

Run:

```bash
go test ./internal/profile -count=1
```

- [ ] **Step 4.4: Watch profile summaries**

Add watch events that include changed classes, affected tests, run duration, hot methods, and profile file locations when profiling is enabled.

Run:

```bash
go test ./internal/watch ./internal/testdaemon -count=1
```

## Phase 5: Governor Limits Later Phase

**Purpose:** Move from deterministic local counters toward configurable Salesforce-like cap profiles. This is useful, but it should follow stdlib and server support work.

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/limits.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/vm/runtime_state.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/internal/gladecli/*limit*`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools/internal/capability/capability.go`

- [ ] **Step 5.1: Add cap profile type**

Introduce a small internal type in `internal/vm/limits.go`:

```go
type LimitProfile string

const (
    LimitProfileLocalDeterministic LimitProfile = "local"
    LimitProfileSalesforceSync     LimitProfile = "salesforce-sync"
    LimitProfileSalesforceAsync    LimitProfile = "salesforce-async"
    LimitProfileSalesforceBatch    LimitProfile = "salesforce-batch"
)
```

Add tests that each profile returns explicit caps for SOQL, DML, heap, CPU, callout, email, queueable, future, batch, scheduled, runAs, and savepoints.

- [ ] **Step 5.2: Counter audit**

For each counter, write one test that increments it through real runtime behavior:

- SOQL queries and rows
- DML statements and rows
- projected child relationship rows
- heap mutation
- CPU statement cost
- callout mocks
- email invocation
- queueable/future/batch/scheduled enqueue and drain
- savepoint rollback

Run:

```bash
go test ./internal/vm -run Limits -count=1
```

- [ ] **Step 5.3: CLI/server profile selection**

Expose cap profiles where limit mode already exists. Keep defaults unchanged.

Run:

```bash
go test ./internal/gladecli ./internal/server ./internal/vm -run Limits -count=1
```

## Phase 6: Capability and Documentation Closeout

**Purpose:** Regenerate checked reports, keep the public story honest, and prove the rendered site.

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/STDLIB_COVERAGE.md`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/COMPATIBILITY_DASHBOARD.md`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/KNOWN_GAPS.md`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/site/docs-src/guide/support-map.md`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/site/docs-src/guide/local-api-server.md`
- Modify: `/Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/site/tests/theme.test.mjs`

- [ ] **Step 6.1: Regenerate checked reports**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go run ./cmd/glade-tools dashboard --output /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --output /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/KNOWN_GAPS.md
go run ./cmd/glade-tools stdlib --output /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/STDLIB_COVERAGE.md
```

- [ ] **Step 6.2: Refresh surface dashboard**

Run:

```bash
tmp="$(mktemp -d)"
go run ./cmd/glade-tools surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"
sed -n '1,80p' "$tmp/SURFACE_DASHBOARD.md"
```

Expected:

- `gap | 0`
- `failure | 0`
- no `missingEvidence` rows
- implemented count does not fall

- [ ] **Step 6.3: Update public docs**

Update support wording only from generated facts:

- `site/docs-src/guide/support-map.md`
- `site/docs-src/guide/local-api-server.md`
- `site/docs-src/guide/compatibility-dashboard.md`
- `site/tests/theme.test.mjs`

Keep hosted service boundaries visible. Do not claim:

- delivered email
- live callouts
- live OAuth/session validation
- full Visualforce rendering
- PDF generation
- live org-only Tooling
- hosted identity mutation

- [ ] **Step 6.4: Full verification**

Run product gates:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion
go test ./internal/vm ./internal/server ./internal/lsp ./internal/dap ./internal/profile ./internal/watch ./internal/repoguard -count=1
npm --prefix site test
npm --prefix site run build
git diff --check
```

Run tools gates:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go test ./internal/capability ./internal/surfaceledger ./internal/compat ./internal/toolcli -count=1
git diff --check
```

Run generated-doc checks:

```bash
cd /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion-tools
go run ./cmd/glade-tools dashboard --check /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/KNOWN_GAPS.md
go run ./cmd/glade-tools stdlib --check /Users/matt/Dev/glade/.worktrees/local-support-gap-expansion/docs/STDLIB_COVERAGE.md
```

Expected: all commands exit 0.

## Parallel Subagent Split

Use separate subagents only when their write sets do not overlap.

1. **Pure stdlib worker**
   - Owns: `internal/vm/json_*`, `internal/vm/stdlib*`, `internal/vm/dispatch.go`
   - Owns tools: `docs/fixtures/core-runtime-pure-stdlib-closeout.json`, `internal/capability/stdlib.go`

2. **Metadata-backed stdlib worker**
   - Owns: `describe_runtime.go`, `business_hours_runtime.go`, `approval_process_runtime.go`, `email_runtime.go`, `test_support_runtime.go`
   - Owns tools: `docs/fixtures/core-runtime-metadata-backed-stdlib-closeout.json`, `internal/capability/stdlib.go`
   - Coordinate with pure stdlib worker before editing `stdlib.go`.

3. **Server breadth worker**
   - Owns: `internal/server/*`
   - Owns tools: server fixtures and `internal/capability/capability.go`

4. **Developer experience worker**
   - Owns: `internal/lsp`, `internal/dap`, `internal/profile`, `internal/watch`
   - Owns tools: `internal/capability/capability.go`

5. **Docs and generated report worker**
   - Runs after implementation workers.
   - Owns: generated docs, support map, site tests.

## Completion Rules

- Never move a row to `supported` from notes alone.
- Never broaden a hosted Salesforce service into fake local success.
- Each promoted row needs a passing fixture or product test named in the commit.
- Every phase ends with before counts, after counts, exact commands, and remaining top rows.
- Remove `/Users/matt/Dev/glade/.worktrees/glade` before final cleanup.
- Merge to local `main` only after verification from the merged checkout.
