# Full Example Project Support Plan

## Goal

Bring `oaer compat server-examples --json` to full support for the checked-in
enterprise example projects without adding project-specific behavior to the core
runtime or stdlib capability matrix.

Current baseline:

```text
pass=63 fail=0 unsupported=11 missing=0
```

Acceptance target:

```text
fail=0 unsupported=0 missing=0
```

The example projects are compatibility proof code. Project-specific classes such
as `CurrenciesApi`, query factory wrappers, selectors, and managers must execute
as ordinary Apex. They must not be implemented as VM stdlib stubs or capability
entries.

## Guardrails

- Keep core/runtime changes tied to Apex language semantics or public Salesforce
  platform behavior.
- Keep route-specific records, webhook setup data, email templates, and other
  proof fixtures inside `internal/compat/server_examples.go` probe overlays.
- Do not add package-specific method names to `internal/vm` stdlib dispatch.
- When a harness blocker reaches a custom project method, fix the general
  dispatch, parser, SOQL, DML, or VM gap that prevents that method from running.
- Promote capability status only after adding compatibility coverage and updating
  generated docs.

## Current Remaining Blockers

From the latest harness run:

```text
example-projects/sf-cred-pkg-develop POST /services/apexrest/webhookEvents
  RuntimeError: Attempt to de-reference a null object

example-projects/sf-cred-pkg-develop POST /services/apexrest/webhookevent/create
  RuntimeError: ambiguous overload for call "queryFactory.selectFields"

example-projects/src-nmb-nu-develop POST /services/apexrest/selfservice/cart/submit/
  RuntimeError: soql: expected FROM

example-projects/src-nmb-nu-develop POST /services/apexrest/selfservice/email/SocialVerify
  RuntimeError: Email template not found: 00X000000000001AAA

example-projects/src-nmb-nu-develop POST /services/apexrest/selfservice/order/
  unsupported call "CurrenciesApi.v1.syncCurrencyWithRelatedRecord"

example-projects/src-nmb-nu-develop PATCH /services/apexrest/selfservice/sobjects/
  RuntimeError: operator && requires Boolean operands
```

The duplicated webhook and PATCH entries are duplicate REST routes across
example source trees, not separate root causes.

## Phase 1: Diagnostics

Improve runtime diagnostics before broad fixes.

Tasks:

- Add class/method/file/line context to Apex REST runtime errors when available.
- Include expression/member context for null dereferences.
- Include candidate signatures and argument runtime types for overload ambiguity.
- Add optional focused harness tracing for a single probe or route.

Expected result:

- The remaining blockers can be fixed from exact source locations rather than
  inferred symptoms.

## Phase 2: Overload Resolution

Target:

```text
ambiguous overload for call "queryFactory.selectFields"
```

Likely general gap:

- Generic collection element types are not preserved or scored precisely enough
  for overloads such as `Set<String>`, `List<String>`,
  `Set<Schema.SObjectField>`, and `List<Schema.SObjectField>`.

Tasks:

- Preserve generic collection type information through variable declarations,
  casts, literals, method returns, `Map.get`, `Map.values`, and `Map.keySet`.
- Make overload scoring prefer exact collection kind and element type.
- Ensure empty collections do not accidentally match every overload equally.
- Add focused VM tests with fflib-style overload sets.

Expected movement:

- `webhookevent/create` moves past `queryFactory.selectFields`.

## Phase 3: Dynamic SOQL

Target:

```text
selfservice/cart/submit: soql: expected FROM
```

Tasks:

- Capture the exact SOQL string passed to `Database.query` when parsing fails.
- Add a regression test using that generated query.
- Determine whether the bug is in Apex string/query builder execution or in the
  SOQL parser.
- Fix the general VM or SOQL parser behavior.

Likely areas:

- `String.join`
- list/set iteration order
- dynamic string concatenation
- subquery syntax
- relationship fields
- namespaced field resolution

Expected movement:

- Cart submit either passes or reaches the next DML/trigger gap.

## Phase 4: Email Template Rendering

Target:

```text
selfservice/email/SocialVerify: Email template not found: 00X000000000001AAA
```

Correct core scope:

- Local deterministic `Messaging.renderStoredEmailTemplate` behavior is a
  Salesforce platform surface.
- Real delivery transport remains unsupported.

Tasks:

- Implement a local subset of
  `Messaging.renderStoredEmailTemplate(templateId, whoId, whatId)`.
- Look up `EmailTemplate` by Id in org storage.
- Return a `Messaging.SingleEmailMessage` with deterministic subject/html/text
  fields from template fields, with stable empty defaults when template body
  fields are absent.
- Keep `Messaging.sendEmail` as local result-only with no transport.
- Add VM tests with seeded `EmailTemplate`.

Expected movement:

- SocialVerify moves beyond template rendering.

## Phase 5: Boolean Semantics

Target:

```text
selfservice/sobjects PATCH: operator && requires Boolean operands
```

Tasks:

- Use diagnostics to locate the exact expression.
- Support valid Apex casts/coercions from `Object`/SObject field values to
  `Boolean`.
- Verify short-circuit behavior with nullable Boolean expressions.
- Keep invalid operands as explicit runtime diagnostics.

Expected movement:

- PATCH probes pass or reveal a deeper DML/trigger issue.

## Phase 6: Custom Project Method Dispatch

Target:

```text
selfservice/order: unsupported call "CurrenciesApi.v1.syncCurrencyWithRelatedRecord"
```

This must not be stubbed. `CurrenciesApi.v1.syncCurrencyWithRelatedRecord` is a
custom project/package method and must run as ordinary Apex.

Tasks:

- Locate the source declaration and symbol registration for `CurrenciesApi.v1`.
- Determine whether nested/lowercase static class member dispatch is failing.
- Preserve project class names and aliases in the symbol index and VM runtime.
- If the method body contains unsupported language/platform features, report the
  first real unsupported feature inside the method rather than treating the
  custom method call itself as unsupported.
- Add focused tests for nested static class dispatch using lowercase inner class
  names if that is the root cause.

Expected movement:

- The order route moves into ordinary Apex/SOQL/DML execution or passes.

## Phase 7: Webhook Null Dereference

Target:

```text
webhookEvents: Attempt to de-reference a null object
```

Tasks:

- Use Phase 1 diagnostics to identify the exact null expression.
- Classify as one of:
  - missing route overlay data,
  - missing Salesforce platform metadata,
  - Apex runtime gap,
  - SOQL/storage relationship gap,
  - JSON/model hydration gap.
- Add overlay data only when it is endpoint proof data.
- Fix runtime/platform behavior when the null comes from a general Apex or
  Salesforce semantic gap.

Expected movement:

- Webhook events pass or expose the next general runtime gap.

## Phase 8: Harness Ergonomics and Regression Guards

Tasks:

- Add probe filtering for local iteration, for example:
  - `oaer compat server-examples --project <root> --probe webhookEvents`
  - `oaer compat server-examples --route /services/apexrest/...`
- Add concise blocker-only text output.
- Add a guard test that project/package-specific names are not present in stdlib
  or capability tables.
- Keep the existing per-probe overlay isolation tests.

## Parallel Work Plan

Initial squad lanes:

- `diagnostics`: enrich VM/Apex REST runtime errors and add optional harness
  probe filtering/tracing.
- `overload`: fix generic collection-aware overload resolution for query
  factories.
- `email-template`: implement deterministic local
  `Messaging.renderStoredEmailTemplate`.
- `boolean`: fix valid Apex Boolean casts/coercions and `&&` handling.

Follow-up lanes after the first squad reports:

- `dynamic-soql`: cart submit generated query/parser support.
- `custom-dispatch`: `CurrenciesApi.v1` as ordinary custom Apex dispatch.
- `webhook-null`: exact webhook null root cause once diagnostics are available.

## Validation Loop

For every lane:

```bash
go test ./internal/vm ./internal/server ./internal/compat ./internal/storage ./internal/soql
go run ./cmd/oaer compat server-examples --json
```

Before merging back:

```bash
go test ./...
go run ./cmd/oaer compat server-examples --json
```
