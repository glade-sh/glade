# Full Example Project Support Plan

## Goal

Bring the checked-in enterprise example projects to full local support without
adding project-specific behavior to the VM, stdlib dispatch, or capability
tables.

There are two related targets:

- `server-examples-green`: every probe in `oaer compat server-examples --json`
  is `pass`, with `fail=0`, `unsupported=0`, and `missing=0`.
- `legacy-project-test-ready`: scanner and local test evidence show that the
  example projects' metadata, controller tests, declarative automation, files,
  messaging, and platform test APIs can run locally through general OAER
  behavior.

Current `server-examples` baseline from May 5, 2026:

```text
pass=65 fail=6 unsupported=3 missing=0
```

Acceptance target:

```text
fail=0 unsupported=0 missing=0
```

The example projects are compatibility proof code. Project-specific classes such
as `CurrenciesApi`, query factory wrappers, selectors, managers, and REST
controllers must execute as ordinary Apex. They must not be implemented as VM
stdlib stubs or capability entries.

## Guardrails

- Keep runtime changes tied to Apex language semantics or public Salesforce
  platform behavior.
- Keep route-specific records, webhook setup data, email templates, and other
  proof fixtures inside `internal/compat/server_examples.go` probe overlays.
- Do not add package-specific method names to `internal/vm` stdlib dispatch.
- When a harness blocker reaches a custom project method, fix the general
  dispatch, parser, SOQL, DML, or VM gap that prevents that method from running.
- Promote capability status only after adding compatibility coverage and
  updating generated docs.
- Do not use proprietary AER internals as an implementation source.

## Current Server-Example Blockers

All remaining live blockers are in `lane-2-apex-rest`. Core REST, auth/user,
tooling/metadata, composite, bulk, seed-data, and project presence probes are
passing.

```text
example-projects/sf-cred-pkg-develop POST /services/apexrest/webhookEvents
  500 fail: 95: Invalid field '' for object 'Setup_Data__c'
  duplicated by .claude worktree copies plus active force-app source

example-projects/sf-cred-pkg-develop POST /services/apexrest/webhookevent/create
  500 fail: 715: soql: expected FROM
  duplicated by .claude worktree copies plus active force-app source

example-projects/src-nmb-nu-develop POST /services/apexrest/selfservice/cart/submit/
  501 unsupported: QueryException: soql: expected FROM
  at BatchSelector.selectAutomaticOpen

example-projects/src-nmb-nu-develop POST /services/apexrest/selfservice/email/SocialVerify
  501 unsupported: Email template not found: 00X000000000001AAA
  at EmailTemplates.BuildEmailMessageForEntity

example-projects/src-nmb-nu-develop POST /services/apexrest/selfservice/order/
  501 unsupported: unsupported call "CurrenciesApi.v1.syncCurrencyWithRelatedRecord"
  while initializing di_Binding.bindingImplsByType
```

## Active Wave 1 Squad

The first implementation wave runs from `example-project-support` using
parallel worktrees.

| Lane | Worktree | Branch | Scope |
| --- | --- | --- | --- |
| SOQL builder | `/Users/matt/Dev/oaer-lane-example-soql-builder` | `codex/example-soql-builder` | Capture and fix generated dynamic SOQL failures for webhook create and cart submit. |
| Email template | `/Users/matt/Dev/oaer-lane-example-email-template` | `codex/example-email-template` | Fix the SocialVerify template lookup/rendering path with general `EmailTemplate` behavior or route-proof overlay data. |
| Custom dispatch | `/Users/matt/Dev/oaer-lane-example-custom-dispatch` | `codex/example-custom-dispatch` | Make `CurrenciesApi.v1.syncCurrencyWithRelatedRecord` dispatch as ordinary custom Apex, with no stdlib stub. |
| Apex REST diagnostics | `/Users/matt/Dev/oaer-lane-example-apexrest-diagnostics` | `codex/example-apexrest-diagnostics` | Add focused server-example filters, blocker-oriented output, and richer route/source/runtime diagnostics. |

Merge each lane independently when its focused tests pass and its diff is
general-purpose. After each merge, rerun the server-example harness because the
remaining blockers are likely to shift.

## Wave 0: Harness Ergonomics

This work can merge before or alongside runtime fixes.

Tasks:

- Add `server-examples` filters by project name, path/route substring, probe
  name, family, owner lane, and outcome.
- Add blocker-only output that avoids printing huge successful describe payloads
  during local iteration.
- Ignore hidden nested project worktrees such as
  `example-projects/sf-cred-pkg-develop/.claude/worktrees` by default so copied
  REST resources do not triple failure counts.
- Keep JSON schema backward-compatible. New filters should reduce the included
  projects/probes, not rename existing fields.
- Add tests for filtering, blocker-only output, and hidden-worktree exclusion.

Validation:

```bash
go test ./internal/compat ./internal/oaercli
go run ./cmd/oaer compat server-examples --json
```

## Wave 1: Green Server Examples

### Dynamic SOQL And Query Builders

Target blockers:

- `sf-cred` `/webhookevent/create`: `soql: expected FROM`
- `src-nmb-nu` cart submit: `QueryException: soql: expected FROM`
- `sf-cred` `/webhookEvents`: invalid blank `Setup_Data__c` field, if caused by
  generated field lists

Tasks:

- Capture the exact SOQL string passed to `Database.query` when parsing fails.
- Add focused regression tests using the generated query or the smallest owned
  Apex builder that reproduces it.
- Fix the general VM string/list/map behavior or SOQL parser gap.
- Keep invalid SOQL diagnostics explicit and include the generated query text
  when available.

### Email Template Lookup And Rendering

Target blocker:

- `src-nmb-nu` SocialVerify: `Email template not found: 00X000000000001AAA`

Tasks:

- Determine whether the project lookup fails before `Messaging` rendering or
  whether local `Messaging.renderStoredEmailTemplate` is incomplete.
- If the route needs proof data, add deterministic `EmailTemplate` overlay data
  scoped to the relevant probe.
- If the platform behavior is incomplete, implement the local subset:
  `EmailTemplate` lookup by Id, deterministic subject/html/plain fields, stable
  empty defaults, captured email side effects, and no real transport.
- Add VM and compatibility tests.

### Custom Project Dispatch

Target blocker:

- `CurrenciesApi.v1.syncCurrencyWithRelatedRecord`

Tasks:

- Locate `CurrenciesApi`, nested/static `v1`, and the method declaration.
- Determine whether the root cause is nested/lowercase type dispatch, static
  field initialization, alias resolution, method visibility, or DI binding
  initialization.
- Add a focused VM test for the general dispatch shape.
- Fix the runtime/type-system path so the method body executes or reports the
  first real unsupported feature inside it.

### Apex REST Diagnostics

Tasks:

- Include route class/method, source file, line, VM stack, and generated SOQL
  context where available.
- Keep Salesforce-shaped HTTP error arrays and stable unsupported/error
  classification.
- Add tests for runtime errors inside Apex REST dispatch.

Wave 1 merge gate:

```bash
go test ./internal/vm ./internal/soql ./internal/storage ./internal/server ./internal/compat ./internal/apextest
go run ./cmd/oaer compat server-examples --json
```

## Wave 2: Metadata And Resolution Support

The scanner-scale project evidence says full example support is dominated by
metadata-backed resolution, not only Apex runtime primitives.

Primary blockers:

- `custommetadata.legacy-records`
- `labels.localization`
- `metadata.legacy-source`
- `ui.presentation-metadata`
- `staticresources.urlfor`
- profiles, permission sets, layouts, field sets, global value sets, tabs, named
  credentials, and remote sites when tests resolve them

Squad boundaries:

- `metadata-loader`: legacy `.object`, `.md`, `.labels`, `.layout`, `.profile`,
  `.permissionset`, `.tab`, `.workflow`, `.flow`, and related source formats.
- `labels-and-translations`: `$Label`, translation files, namespace handling,
  and deterministic missing-label diagnostics.
- `resources-and-ui-metadata`: static resource URL generation, content assets,
  layout/list-view/field-set metadata needed by tests.

Validation:

```bash
go run ./cmd/oaer compat examples --json
go run ./cmd/oaer compat post-parity --json
go test ./internal/project ./internal/schema ./internal/examplescan ./internal/projectscan
```

## Wave 3: Visualforce Controller Tests

Target support is controller/test behavior, not full Visualforce rendering.

Tasks:

- Parse and index `.page` and `.component` metadata: controller,
  standardController, extensions, action, component attributes, assign-to
  bindings, and simple merge expressions.
- Resolve `Page.SomePage` references to local metadata.
- Implement `PageReference` URL, redirect, parameters, headers, cookies, and
  request body stubs.
- Isolate `ApexPages.currentPage()` per test and server request.
- Implement `ApexPages.Message`, `Severity`, `addMessage`, `getMessages`, and
  `hasMessages`.
- Construct custom controllers, standard controllers, standard set
  controllers, and controller extensions.

Validation:

```bash
go test ./internal/vm ./internal/apextest ./internal/projectscan
go run ./cmd/oaer inspect gaps --project example-projects/src-nmb-nu-develop --json
```

## Wave 4: Aura And LWC Controller Discovery

Tasks:

- Discover LWC `@salesforce/apex`, `@wire`, `@salesforce/label`,
  `@salesforce/resourceUrl`, `@salesforce/schema`, and local `c/...` imports.
- Discover Aura controller/helper references and `@AuraEnabled` methods.
- Support wrapper serialization shapes used by controller tests.
- Connect UI import evidence to Apex test readiness without implementing a full
  browser lifecycle.

## Wave 5: Platform Test APIs

Primary blockers:

- `System.Callable`
- `System.StubProvider`
- `Test.createStub`
- ApexMocks-style invocation plumbing
- `Auth`, `Site`, `Network`, `Community`, Platform Cache, and narrow
  `ConnectApi` test context
- Apex Metadata deploy DTOs/callbacks when used by tests

Tasks:

- Implement deterministic local test doubles and explicit unsupported edges.
- Keep callout/auth behavior local and fixture-driven.
- Add owned compatibility fixtures modeled after public patterns from the
  example projects.

## Wave 6: Data Side Effects

Tasks:

- Support `Attachment`, `Document`, `ContentVersion`, `ContentDocument`, and
  `ContentDocumentLink` body fields, IDs, relationships, and deterministic URLs.
- Capture `Messaging.sendEmail` side effects and limit accounting.
- Extend email template merge context only where tests need it.

## Wave 7: Declarative Automation

Tasks:

- Execute Workflow Rule criteria, field updates, email alerts, recursive
  save-order behavior, and rollback.
- Execute record-triggered/autolaunched Flow and Process Builder-style metadata
  that mutates records or calls `@InvocableMethod`.
- Emit trace events for workflow/flow decisions and side effects.

## Final Readiness Gate

Before claiming full example project support:

```bash
go test ./...
go run ./cmd/oaer compat server-examples --json
go run ./cmd/oaer compat examples --json
go run ./cmd/oaer compat post-parity --json
go run ./cmd/oaer compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --check docs/KNOWN_GAPS.md
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
```

Release language should stay conservative until the corresponding gate is green:

- `server-examples-green`: REST/server proof routes are green.
- `legacy-project-test-ready`: local test execution across the example-project
  blocker families is green.
- `declarative-automation-test-ready`: Workflow/Flow save-order side effects
  are green.
