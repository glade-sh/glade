# src-nmb-nu-develop Parallel Squad Plan

Status date: 2026-05-07.

This plan targets `example-projects/src-nmb-nu-develop` after the NUTPL
runtime sentinel went green. The goal is to move this project from a broad
compile-gap frontier to executable local tests without project-specific stubs or
runtime branches.

Current command:

```bash
go run ./cmd/oaer compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --timeout 30000 \
  --top-failures 25 \
  --json
```

Current result:

```text
ready=false
total=11526
compileGap=11526
pass=0 fail=0 unsupported=0 loadError=0 compileError=0 internalError=0
```

The current top outcome blocker is:

```text
ARTransactions.cls:109
constructor "ARTransaction" references unknown variable "ORDER_ITEM_PARAM"
```

That single blocker gates all discovered test methods, but the full diagnostics
show the next layers. Treat these as OAER parity gaps unless a focused
scratch-org probe proves otherwise.

## Gap Families

| Family | Current signal | Likely root |
| --- | --- | --- |
| Nested class access to outer private/test-visible statics | `ARTransactions.ARTransaction` cannot see `ORDER_ITEM_PARAM` / `AMOUNT_PARAM`; same in `ARTransactions1`. | Sema member lookup does not consistently include enclosing type members for nested classes. |
| Inherited fluent methods and base return types | `setField` dominates unknown-method diagnostics (`5405` instances); `toSObject`, `setParent`, `setChildren` follow. | Chained-call typing loses inherited methods and/or loses receiver type after a base-class method returns `SObjectFabricator` / `sfab_FabricatedSObject`. |
| ApexMocks fluent stubbing | `thenReturn`, `when`, matcher calls (`match_anySObjectList`, `match_anySetOfId`, `match_anyObject`) appear after the fabricator frontier. | Sema lacks enough fluent mock/matcher call typing for fflib/ApexMocks patterns already seen in NUTPL but broader here. |
| SOQL parsing inside collection constructors and subqueries | `WHERE`, `AND`, `IN`, `INCLUDES`, and `SELECT` appear as unknown types/methods in `AccountManager.cls`. | Parser/sema treats some static SOQL contexts as Apex expressions, especially `new Map<Id, RecordType>([SELECT ...])`, child subquery `WHERE`, semi-join, and multi-select picklist `INCLUDES`. |
| Generated properties and declaration-order member indexing | `Address` constructors cannot see `CountryCode` / `StateCode`; address constructors reported as missing despite being declared. | Type/member indexing likely misses auto-properties declared after constructors or does not resolve property names case-insensitively throughout constructor validation. |
| Standard Schema/Describe gaps | Missing `Limits.getLimitQueryRows`, `getRecordTypeInfosById`, `Schema.DisplayType.BOOLEAN`, some standard objects such as `RecordType` behavior. | Product namespace declarations and stdlib/schema describe coverage need more generated typed declarations and VM contracts. |
| Private/test-visible access in test contexts | `cannot access private field` is high-volume (`498` diagnostics), often in tests and nested helper patterns. | Access rules likely need Apex-specific treatment for `@TestVisible`, same-top-level nested access, and test-class visibility. |
| Relationship and child-list typing | Unknown fields such as `OrderItem.RecordType.Name`; child relationship `.size()` calls are interpreted as unknown methods. | SObject relationship-path typing and child relationship list typing need to flow through sema, not only runtime SOQL projection. |

Diagnostic counts from the current run:

```text
9893 OAERSEMA008 unknown method
 985 OAERSEMA021 unknown field
 498 OAERSEMA010 inaccessible private member
 318 OAERSEMA006 unknown type
 308 OAERSEMA011 constructor mismatch
 301 OAERSEMA013 unknown variable
 149 OAERSEMA009 no matching overload
 128 OAERSEMA023 unsupported expression/typing edge
 122 OAERSEMA022 ambiguous overload
 101 OAERSEMA018 assignment mismatch
  86 OAERSEMA019 return mismatch
  72 OAERSEMA024 enhanced-for non-collection
```

## Parallel Work Lanes

### Lane A: Nested Class And TestVisible Access

Owner scope: `internal/typesys`, `internal/sema`, focused VM tests only if
runtime access checks are affected.

Representative sources:

- `ARTransactions.cls:5-10`, `ARTransactions.cls:92-110`
- `ARTransactions1.cls:5-10`, `ARTransactions1.cls:73-90`

Tasks:

- Make nested class member lookup include enclosing type static members.
- Ensure `@TestVisible private static final` fields are visible where Apex
  allows them: same top-level type, nested classes, and test classes.
- Keep all Apex name lookup case-insensitive; do not add case-specific
  special cases.
- Add minimal fixtures for nested class access to outer constants and
  `@TestVisible` access from tests.

Validation:

```bash
go test ./internal/typesys ./internal/sema ./internal/vm
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nu-develop --timeout 30000 --top-failures 25 --json
```

Expected movement:

- The top blocker should move past `ARTransactions.ORDER_ITEM_PARAM`.
- `OAERSEMA013` unknown-variable count should drop materially.

### Lane B: Fluent Inheritance And Fabricated SObject Chains

Owner scope: `internal/sema`, `internal/typesys`, `internal/vm` only if
runtime chain execution lacks support after compile moves.

Representative sources:

- `TransactionGenerator2Test.cls:17-23`
- `SObjectFabricator.cls:82-116`, `SObjectFabricator.cls:162-164`
- `sfab_FabricatedSObject` call chains using `.setField(...).toSObject()`

Tasks:

- Fix method lookup for inherited methods on subclasses and chained receiver
  types.
- Preserve the declared return type for fluent calls, while allowing casts from
  base fabricator/SObject helper types where Apex permits them.
- Support `Schema.SObjectField` arguments from field tokens such as
  `Product__c.Id`.
- Add fixtures for:
  - inherited fluent method called on subclass;
  - chained base-return method followed by another inherited method;
  - `Schema.SObjectField` token argument overload resolution.

Validation:

```bash
go test ./internal/typesys ./internal/sema ./internal/vm
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nu-develop --timeout 30000 --top-failures 25 --json
```

Expected movement:

- `setField`, `setParent`, `setChildren`, and `toSObject` unknown-method counts
  should collapse.
- `TransactionGenerator2Test` should stop dominating diagnostics.

### Lane C: Static SOQL Contexts And Child Query Typing

Owner scope: `internal/apexast`, `internal/sema`, `internal/soql`, `internal/vm`
only where static SOQL runtime projection needs a matching fixture.

Representative sources:

- `AccountManager.cls:295-299`: `new Map<Id, RecordType>([SELECT ... WHERE ...])`
- `AccountManager.cls:535-567`: child relationship subquery with `WHERE`
- `AccountManager.cls:577-588`: multiple child relationship subqueries
- `AccountManager.cls:618-622`: semi-join
- `AccountManager.cls:678-679`: multi-select picklist `INCLUDES`

Tasks:

- Ensure static SOQL is recognized as one expression inside collection
  constructors and assignments.
- Prevent SOQL keywords from becoming Apex local types, methods, or variables.
- Add sema support for `new Map<Id, SObjectType>([SOQL])`.
- Type child relationship subquery fields as query-only list properties so
  `.size()` and enhanced-for checks understand them.
- Confirm SOQL parser/runtime support for `INCLUDES`, semi-joins, and child
  subquery filters; add explicit unsupported diagnostics only for genuinely
  unsupported clauses.

Validation:

```bash
go test ./internal/apexast ./internal/sema ./internal/soql ./internal/vm
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nu-develop --timeout 30000 --top-failures 25 --json
```

Expected movement:

- `WHERE`, `AND`, `IN`, `INCLUDES`, and `SELECT` diagnostics should disappear
  from Apex sema.
- Child relationship `.size()` unknown-method diagnostics should drop.

### Lane D: Generated Properties, Constructors, And Case-Insensitive Members

Owner scope: `internal/typesys`, `internal/sema`, generated declaration loading
if product namespace declarations are involved.

Representative sources:

- `Address.cls:21-47`, `Address.cls:52-89`
- `AddressUtil.cls:41-90`, `AddressUtil.cls:195-260`
- Constructor mismatch diagnostics for `Address`, `Payment`, `Order`, and
  response/request DTOs.

Tasks:

- Ensure auto-properties are indexed regardless of declaration order.
- Make property and field lookup case-insensitive across constructor bodies,
  assignment, getter/setter access, overload resolution, and field-path checks.
- Revisit constructor overload matching for classes with global/public
  constructors and property assignments.
- Add fixtures for properties declared after constructors and lower/upper/mixed
  case property access.

Validation:

```bash
go test ./internal/typesys ./internal/sema
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nu-develop --timeout 30000 --top-failures 25 --json
```

Expected movement:

- `Address` constructor and `CountryCode` / `StateCode` field diagnostics
  should disappear.
- Constructor mismatch and ambiguous overload counts should drop enough to
  expose the next frontier.

### Lane E: Stdlib, Schema Describe, And Product Namespace Declarations

Owner scope: `internal/vm`, `internal/sobject`, `internal/storage`,
`internal/schema`, generated product namespace declaration artifacts.

Representative sources:

- `AccountHierarchyBuilder.cls:237`: `Limits.getLimitQueryRows()`
- `AddEditExternalPaymentProfileController.cls:436`:
  `getRecordTypeInfosById()`
- `AccountEvaluator.cls:56`: `Schema.DisplayType.BOOLEAN`
- Permission/Profile/EmailTemplate/RecordType diagnostics in current report.

Tasks:

- Add missing `Limits.*` method declarations and VM behavior for common limit
  getters used by this project.
- Ensure `DescribeSObjectResult.getRecordTypeInfosById()` and related maps are
  declared and executable.
- Complete `Schema.DisplayType` enum values with Apex case-insensitive member
  lookup.
- Add or refresh product namespace generated typed declarations for standard
  objects and stdlib classes touched by this project.

Validation:

```bash
go test ./internal/vm ./internal/sobject ./internal/storage ./internal/schema
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nu-develop --timeout 30000 --top-failures 25 --json
```

Expected movement:

- Missing `Limits.*`, `getRecordTypeInfosById`, and `DisplayType.BOOLEAN`
  diagnostics should clear.
- Some standard-object unknown-field diagnostics should move to either support
  or precise unsupported outcomes.

### Lane F: ApexMocks And Matcher Fluent Typing

Owner scope: `internal/sema`, `internal/vm`, ApexMocks/fflib-focused fixtures.

Representative signals:

- Unknown methods: `thenReturn`, `when`, `match_anySObjectList`,
  `match_anySetOfId`, `match_anyObject`, `then`.
- High test coverage in files using `fflib_ApexMocks` patterns.

Tasks:

- Extend sema call typing for ApexMocks fluent chains and matcher return types.
- Preserve mock receiver type through casts from `mocks.mock(Type.class)`.
- Add focused fixtures that model `mocks.when(mock.method()).thenReturn(value)`
  for object, list, set, and primitive return types.
- Keep runtime behavior deterministic and local; no project-specific mocks.

Validation:

```bash
go test ./internal/sema ./internal/vm
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nu-develop --timeout 30000 --top-failures 25 --json
```

Expected movement:

- `thenReturn`, `when`, matcher, and mock receiver unknown-method diagnostics
  should drop after Lanes A-D expose these paths cleanly.

## Recommended Parallel Order

Start these concurrently:

1. Lane A: nested/test-visible access. Smallest, likely top-blocker unlock.
2. Lane B: fluent inheritance/fabricator chains. Highest diagnostic volume.
3. Lane C: static SOQL contexts. Distinct parser/sema surface.
4. Lane D: properties/constructors/case-insensitive members. Broad but isolated.
5. Lane E: stdlib/schema describe declarations. Can run mostly independently.

Hold Lane F until A/B/D have landed or the worker can create fixtures against
current behavior without duplicating those fixes.

## Integration Protocol

After each lane merge:

```bash
go test ./...
go run ./cmd/oaer compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --timeout 30000 \
  --top-failures 25 \
  --json > /tmp/oaer-src-nmb-nu-after-lane.json
jq '.summary, .topFailures[0:10]' /tmp/oaer-src-nmb-nu-after-lane.json
```

After all initial lanes land, refresh the six-project baseline:

```bash
node scripts/baseline-local-tests-example-projects.mjs
```

Success for this batch is not "all tests pass" yet. Success is a moved frontier:

- `src-nmb-nu-develop` no longer has one compile gap gating every test.
- Top diagnostics move from parser/sema false positives to smaller runtime or
  explicit unsupported families.
- `docs/fixtures/local-tests-example-projects.json` records reduced
  `compileGap` count or first passing subset for `src-nmb-nu-develop`.

