# Salesforce Full Coverage Parallel Squad Plan

## North Star

Build a coverage factory, not a one-off implementation sprint.

The goal is to:

- Inventory Salesforce behavior from local public docs and black-box Tooling/API probes.
- Turn inventory into granular capability IDs in `internal/capability`.
- Generate fixtures first for Apex stdlib, standard objects, Visualforce, metadata, REST/Tooling shapes, and local-test behavior.
- Let parallel squads implement against stable contracts without stepping on each other.
- Promote capabilities only after compatibility coverage exists.

This plan keeps work aligned with `oaer` clean-room rules: public Salesforce docs, black-box compatibility tests, generated docs, explicit unsupported diagnostics, and no proprietary AER internals.

## Current Checkpoint

Verified May 7, 2026:

- Server examples are green: `pass=101 fail=0 unsupported=0 missing=0`.
- The checked post-parity inventory is green for `example-projects`:
  `filesScanned=50457 findings=0 testBlockingFindings=0 surfaces=0`.
- The owned local-test corpus is green:
  `go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-corpus.json --json`.
- Example-project runtime support is partial, not complete:
  `src-nmb-nutpl-develop` is green at `total=761 pass=761`, while the other
  five checked example projects still stop at measured compile-gap frontiers in
  `docs/fixtures/local-tests-example-projects.json`.

That means the next high-leverage squad work should target the remaining
compile frontiers before claiming full example-project support: managed package
dependency artifacts for `znu`, missing standard object/type coverage such as
`CampaignMemberStatus`, static/member resolution gaps, and package/source
layout duplicate-symbol handling.

## Key Inputs

- Local Salesforce docs: `/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper (1)/salesforce-docs`
- OST parser / Tooling API oracle for black-box parse, compile, and API-shape observations.
- Existing `oaer` package boundaries and compatibility infrastructure.
- Existing parity plans:
  - `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`
  - `docs/POST_PARITY_TODO.md`
  - `docs/FEATURE_PARITY_TODO.md`

## Phase 0: Program Setup

### Objective

Create the shared machinery that makes every squad efficient.

### Deliverables

- Capability taxonomy expansion in `internal/capability` for:
  - Standard objects
  - Standard fields
  - Apex stdlib classes and methods
  - Visualforce tags, globals, and controllers
  - Metadata and declarative features
  - REST and Tooling API resources
  - Test-visible platform APIs
- Coverage manifests such as:
  - `docs/generated/SALESFORCE_COVERAGE_MANIFEST.json`
  - `docs/STANDARD_OBJECT_COVERAGE.md`
  - `docs/APEX_STDLIB_COVERAGE.md`
  - `docs/VISUALFORCE_COVERAGE.md`
- Fixture-first implementation rule:
  - No feature moves to `supported` without compatibility coverage.
- Squad branch/worktree convention:
  - One branch or worktree per lane.
  - Shared contracts changed only by an integration captain.

### Baseline Integration Gates

```bash
go test ./...
go run ./cmd/oaer compat mvp --json
go run ./cmd/oaer compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --check docs/KNOWN_GAPS.md
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
```

Add new gates as they land:

```bash
go run ./cmd/oaer compat local-tests --project testdata/local-tests/basic --json
go run ./cmd/oaer compat salesforce-coverage --json
```

## Shared Architecture

### 1. Docs Ingestion Lane

Purpose: convert local Salesforce docs into a structured, queryable source of truth.

Input:

```text
/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper (1)/salesforce-docs
```

Output:

- Apex class and method catalog
- Standard object and field catalog
- Visualforce component, tag, and global variable catalog
- REST and Tooling API endpoint catalog
- Metadata type catalog
- Known examples and snippets extracted from docs

Proposed package/script scope:

- `scripts/`
- `internal/apexdocs`
- Generated JSON under `docs/generated/` or `testdata/generated/`

Agent tasks:

- Build a doc scanner that extracts:
  - Symbol names
  - Method signatures
  - Return types
  - Parameter names and types
  - Object names
  - Field names and types
  - Picklist values where present
  - Example Apex snippets
  - API response examples
- Emit deterministic JSON.

Acceptance criteria:

- Re-running scanner produces stable output.
- Missing or ambiguous docs entries are marked `unknown`, not guessed.
- Generated docs can feed capability reports.

### 2. OST / Tooling API Oracle Lane

Purpose: use the OST parser and Tooling API behavior as a black-box compatibility oracle.

Clean-room boundary:

Use it only for:

- Parse and compile diagnostics
- Public Tooling API result shapes
- Black-box observed behavior
- Fixture generation

Do not copy proprietary internals or undocumented implementation details.

Agent tasks:

- Build a probe runner that takes Apex snippets and records:
  - Whether Salesforce accepts or rejects the snippet
  - Diagnostic line, column, message, and category
  - Inferred parse/compile boundaries
  - `executeAnonymous` response shape
  - Tooling API status and error shape

Output fixtures:

- `docs/fixtures/apex-parser-*.json`
- `docs/fixtures/apex-stdlib-*.json`
- `docs/fixtures/tooling-api-*.json`

Acceptance criteria:

- Probes are reproducible.
- Fixtures are anonymized and deterministic.
- `oaer compat run` can validate against them.

## Squad A: Standard Object & Schema Coverage

Primary packages:

- `internal/schema`
- `internal/storage`
- `internal/sobject`
- `internal/soql`
- `internal/dml`
- `internal/server`

Mission: make standard Salesforce objects first-class in local execution.

Scope:

- Standard object definitions, including:
  - `Account`
  - `Contact`
  - `Opportunity`
  - `Lead`
  - `Case`
  - `User`
  - `Profile`
  - `PermissionSet`
  - `PermissionSetAssignment`
  - `RecordType`
  - `Task`
  - `Event`
  - `Campaign`
  - `Product2`
  - `Pricebook2`
  - `PricebookEntry`
  - `ContentVersion`
  - `ContentDocument`
  - `ContentDocumentLink`
  - `Attachment`
  - `Document`
  - `Group`
  - `QueueSobject`
  - `Organization`
  - `AsyncApexJob`
  - `CronTrigger`
- Standard field metadata:
  - Type
  - Nillability
  - Createable and updateable flags
  - Reference targets
  - Relationship names
  - Picklists
  - External ID and unique flags
- Child relationships.
- Standard record type behavior where applicable.
- Describe parity fixtures.

Fixtures:

- `standard-object-describe-account.json`
- `standard-object-user-permissions.json`
- `standard-object-content-version.json`
- `standard-object-pricebook.json`
- `standard-object-activity-task-event.json`

Acceptance:

```bash
go test ./internal/schema ./internal/storage ./internal/sobject ./internal/soql ./internal/dml
go run ./cmd/oaer compat run docs/fixtures/standard-object-*.json
```

## Squad B: Apex Standard Library Coverage

Primary packages:

- `internal/vm`
- `internal/sema`
- `internal/ir`
- `internal/capability`

Mission: expand Apex stdlib from common subset to broad local-test coverage.

Workstreams:

- System:
  - Assertions
  - Exceptions
  - Limits
  - Test
  - UserInfo
  - Label
  - Type
  - Callable
  - StubProvider
- Core types:
  - `String`
  - `Blob`
  - `Decimal`
  - `Integer`
  - `Long`
  - `Double`
  - `Boolean`
  - `Date`
  - `Datetime`
  - `Time`
  - `Id`
- Collections:
  - `List`
  - `Set`
  - `Map`
  - Iterators
  - Sorting and comparison behavior
- Serialization:
  - `JSON`
  - `JSONGenerator`
  - `JSONParser`
  - `XmlStreamReader`
  - `XmlStreamWriter`
- Security and crypto:
  - `Crypto`
  - `EncodingUtil`
  - `Auth` stubs where test-visible
- Callouts:
  - `Http`
  - `HttpRequest`
  - `HttpResponse`
  - Test mocks only; no real network.

Deliverables:

- Generated Apex stdlib inventory from docs.
- Method-level capability table.
- Runtime implementations.
- Explicit unsupported diagnostics for out-of-scope methods.

Acceptance:

```bash
go test ./internal/vm ./internal/sema
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
```

## Squad C: Parser, Sema, And Type-System Fidelity

Primary packages:

- `internal/apexast`
- `internal/typesys`
- `internal/sema`
- `internal/ir`

Mission: make broad Apex projects parse, index, type-check, and lower reliably.

Scope:

- More expression typing.
- Generics.
- Nested classes, interfaces, and enums.
- Annotations.
- Sharing keywords.
- Access modifiers.
- Property syntax.
- Static initializer behavior.
- Overload resolution.
- Namespaces and managed-package references.
- Schema namespace aliases.
- Partial sema for unsupported runtime features.

OST / Tooling API usage:

Use Tooling API compile responses to build fixtures for:

- Accepted syntax
- Rejected syntax
- Diagnostic ranges
- Method overload behavior
- Namespace references
- Managed package dependency references

Acceptance:

```bash
go test ./internal/apexast ./internal/typesys ./internal/sema ./internal/ir
go run ./cmd/oaer check --project testdata/local-tests/managed-package-dependency --json
```

## Squad D: SOQL, SOSL, DML, And Data Semantics

Primary packages:

- `internal/soql`
- `internal/dml`
- `internal/storage`
- `internal/vm`
- `internal/schema`

Mission: make data behavior credible for real Apex tests.

Scope:

- SOQL:
  - Relationship queries
  - Semi and anti joins
  - Polymorphic references
  - Aggregate queries
  - Date literals
  - `WITH SECURITY_ENFORCED`
  - `USER_MODE` and `SYSTEM_MODE`
  - `FIELDS()`
  - `TYPEOF`
- SOSL:
  - Parse common forms
  - Return deterministic local search results
  - Explicit unsupported diagnostics for unsupported clauses
- DML:
  - Insert, update, upsert, delete, undelete, and merge
  - SaveResult and Error shapes
  - Validation rules
  - Duplicate rules where local-test-visible
  - Lookup validation
  - Cascade delete and undelete basics
  - Mixed DML
  - Transaction rollback

Acceptance:

```bash
go test ./internal/soql ./internal/dml ./internal/storage ./internal/vm
go run ./cmd/oaer compat run docs/fixtures/soql-*.json
go run ./cmd/oaer compat run docs/fixtures/dml-*.json
```

## Squad E: Visualforce And UI Controller Support

Primary packages:

- `internal/visualforce`
- `internal/uicontroller`
- `internal/vm`
- `internal/apextest`
- `internal/sema`

Mission: support Visualforce enough for controller tests and deterministic non-browser rendering.

Current support already includes basics for:

- `PageReference`
- `ApexPages.currentPage()`
- Page parameters
- Messages
- Standard controller basics
- Page reference registration

Scope:

- `.page` and `.component` parsing:
  - `controller`
  - `standardController`
  - `extensions`
  - `action`
  - `recordSetVar`
  - Component attributes
  - Merge expressions
- Controller lifecycle:
  - Custom controller construction
  - Extension construction
  - Standard controller construction
  - Standard set controller behavior
  - Page action invocation
- Merge globals:
  - `$Label`
  - `$ObjectType`
  - `$CurrentPage`
  - `$User`
  - `$Profile`
  - `$Resource`
  - `$Site`
  - `$Component`
- Non-rendering harness:
  - Load page by name
  - Bind parameters
  - Construct controller and extensions
  - Invoke action
  - Inspect messages and redirects
- Later deterministic rendering:
  - `apex:outputText`
  - `apex:repeat`
  - `apex:pageBlock`
  - `apex:form`
  - Custom components

Acceptance:

```bash
go test ./internal/visualforce ./internal/uicontroller ./internal/apextest ./internal/vm
go run ./cmd/oaer compat local-tests --project testdata/local-tests/page-reference --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/ui-controller-contracts --json
```

## Squad F: Declarative Metadata & Automation

Primary packages:

- `internal/automation`
- `internal/dml`
- `internal/storage`
- `internal/schema`
- `internal/projectscan`

Mission: load and execute test-visible declarative behavior.

Scope:

- Custom metadata records
- Custom settings
- Labels and translations
- Static resources
- Named credentials
- Remote site settings
- Workflow rules
- Field updates
- Email alerts
- Flow:
  - Variables
  - Assignments
  - Decisions
  - Loops
  - Record lookup, create, update, and delete
  - Invocable Apex
- Process Builder as Flow-like metadata when possible

Deliverables:

- Metadata loaders.
- Test-visible runtime side effects.
- Transaction rollback integration.
- Trace events for automation decisions.

Acceptance:

```bash
go test ./internal/automation ./internal/dml ./internal/storage ./internal/vm
go run ./cmd/oaer compat run docs/fixtures/automation-*.json
```

## Squad G: Platform APIs For Tests

Primary packages:

- `internal/vm`
- `internal/apextest`
- `internal/storage`
- `internal/resource`

Mission: implement high-value platform APIs that unblock real test classes.

Scope:

- `Test.createStub`
- `System.StubProvider`
- `System.Callable`
- `Site`
- `Network`
- `Auth`
- `ConnectApi.Organization`
- Platform Cache
- `Messaging`
- Email capture
- Files and content side effects
- Static resource URL resolution
- Named credential endpoint resolution
- Callout mocks

Rule:

No real external side effects.

Everything should be:

- Captured
- Deterministic
- Rolled back per test
- Visible through local inspection APIs where useful

Acceptance:

```bash
go test ./internal/vm ./internal/apextest ./internal/resource
go run ./cmd/oaer compat run docs/fixtures/platform-api-*.json
```

## Squad H: REST, Tooling, Composite, And Local Server

Primary packages:

- `internal/server`
- `internal/storage`
- `internal/soql`
- `internal/dml`
- `internal/apextest`

Mission: deepen Salesforce-shaped local API behavior.

Scope:

- REST:
  - SObject CRUD
  - Describe
  - Query and queryAll
  - Recent
  - Limits
  - Composite
  - Batch
  - Tree
  - Collections
- Tooling:
  - `executeAnonymous`
  - Query local Tooling objects
  - ApexClass and ApexTrigger metadata shape
  - AsyncApexJob
  - TraceFlag and ApexLog stubs where useful
- Metadata-adjacent read APIs where test tooling expects them.

Acceptance:

```bash
go test ./internal/server ./internal/storage ./internal/soql ./internal/dml
go run ./cmd/oaer compat server-examples --json
```

## Squad I: Compatibility, Coverage, And Release Gates

Primary packages:

- `internal/compat`
- `internal/capability`
- `internal/projectscan`
- `docs/`
- `scripts/`

Mission: keep all squads honest.

Scope:

- New `compat salesforce-coverage` command.
- New fixture categories:
  - `standard-object`
  - `stdlib`
  - `visualforce`
  - `metadata`
  - `platform-api`
  - `server-api`
  - `tooling-api`
  - `parser-sema`
- Coverage dashboards.
- Gap scanner improvements.
- Top-blocker reports.

Acceptance:

```bash
go run ./cmd/oaer compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --output docs/KNOWN_GAPS.md
go run ./cmd/oaer compat stdlib --output docs/STDLIB_COVERAGE.md
go test ./internal/compat ./internal/capability ./internal/projectscan
```

## Execution Order

### Wave 1: Infrastructure

Run these first because they unblock everyone.

1. Docs ingestion
2. OST/Tooling oracle
3. Capability taxonomy
4. Fixture runner expansion
5. Coverage dashboard skeleton

### Wave 2: High-Value Runtime Coverage

Can run mostly in parallel after Wave 1 contracts are stable.

1. Standard object/schema
2. Apex stdlib
3. Parser/sema fidelity
4. SOQL/DML
5. Visualforce controller harness

### Wave 3: Large-Project Test Unblockers

1. Declarative metadata
2. Platform APIs
3. Files/email/static resources
4. REST/Tooling server depth
5. Managed-package/namespace edge cases

### Wave 4: Full-Coverage Hardening

1. Fuzz and hardening tests
2. Corpus baselines
3. Compatibility docs sync
4. MVP/post-parity gates
5. Performance benchmarks

## Agent Prompt Template

```text
You are working on oaer, a clean-room local Apex runtime.

Goal:
[one-sentence squad goal]

Allowed primary write scope:
[list packages]

Do not modify:
[shared contracts unless requested]

Inputs:
- Local Salesforce docs-derived manifest
- Public Salesforce docs
- Black-box Tooling/API fixtures
- Existing oaer compatibility fixtures

Rules:
- Add compatibility coverage before marking support.
- Use explicit unsupported diagnostics for incomplete behavior.
- Do not use proprietary AER internals.
- Keep behavior deterministic.
- Preserve test isolation and rollback semantics.
- Run package tests and relevant compat gates.

Deliverables:
1. Fixture(s)
2. Implementation
3. Capability status update
4. Generated docs update if status changes
5. Short handoff with remaining gaps
```

## Coordination Rules

### One Integration Captain

The integration captain owns:

- `internal/capability`
- Fixture schema changes
- Generated coverage docs
- Cross-package interfaces
- Final merge sequencing

### Squads Own Behavior, Not Global Contracts

A squad can add features inside its package scope, but changes to shared VM/schema/compat contracts need captain approval.

### Every PR/Handoff Includes

- Capability IDs changed
- Fixtures added
- Packages touched
- Commands run
- Unsupported cases left explicit
- Known divergence from Salesforce

## Recommended Immediate Next Steps

1. [x] Build Salesforce docs ingestion manifest.
2. [x] Build Tooling/API oracle fixture generator.
   - Use `oaer probe tooling-snippet --target-org <alias> --manifest docs/generated/TOOLING_SNIPPET_MANIFEST.json --output <report.json>`.
   - Validate captured reports with `oaer compat tooling-fixtures <report.json>`.
3. [x] Add `compat salesforce-coverage` skeleton.
4. [x] Expand `internal/capability` with granular coverage IDs for Salesforce, standard object, stdlib, and product namespace coverage gates.
5. [ ] Assign squads by primary package ownership for generated typed declarations and runtime implementation lanes.
