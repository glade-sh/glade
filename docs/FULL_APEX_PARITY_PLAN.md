# Full Apex Parity Implementation Plan

This is the living implementation plan for moving `oaer` toward full local Apex
support. Update this file whenever a parity slice lands, a gap is split into a
smaller task, or a public-doc behavior decision changes.

## Ground rules

- Use only public Salesforce documentation, owned fixtures, black-box probes, and
  clean-room reasoning as implementation sources.
- Add fixture evidence before moving a capability to `supported`.
- Prefer executable local behavior for deterministic platform features.
- Use typed stubs plus explicit unsupported diagnostics for cloud-only or
  externally side-effecting operations until a safe local model exists.
- Preserve stable diagnostics, source ranges, trace output, and no-panic behavior
  for malformed Apex, metadata, fixtures, and API requests.
- Keep generated docs in sync after capability status changes.

## Progress ledger

| Phase | Status | Current evidence |
| --- | --- | --- |
| 0. Governance and catalog | Complete | Docs inventory, catalog, evidence reports, docs gate, and product namespace report are implemented. |
| 1. Front-end parity foundation | Complete | Generic collections, enhanced-for typing, expression typing, annotations, static access, and front-end fixtures are implemented in the `front-end-parity` worktree. |
| 2. Core stdlib runtime | In progress | Collection runtime fixture and String runtime fixture are implemented in the `core-stdlib` worktree. |
| 3. Data platform runtime | Not started | Pending. |
| 4. Tests, async, and limits | Not started | Pending; depends on core stdlib and data runtime. |
| 5. Integration, security, UI, events | Not started | Pending; can start stubs after core stdlib advances. |
| 6. Product namespace typed stubs | Complete foundation | Product namespace report covers typed-stub planning for 70 public-doc namespaces. |
| 7. Product namespace local models | Not started | Pending namespace-by-namespace support decisions. |
| 8. MVP/readiness hardening | Not started | Pending broader phase completion. |

## Phase 0: Governance and catalog

Goal: keep parity work measurable and tied to public evidence.

Completed:

- `oaer compat docs-inventory` inventories public Apex docs.
- `oaer compat catalog` builds a hierarchical docs-driven capability catalog.
- `oaer compat evidence` verifies fixture evidence against the catalog.
- `scripts/apex-docs-support-gate.sh` gates docs inventory, catalog, product
  namespace report, and fixture evidence when `OAER_APEX_DOCS_SOURCE` is set.
- Product namespace report separates large product namespaces from core stdlib
  executable-parity work.

Remaining:

- Add coverage threshold reporting once enough fixture categories exist.
- Add stale-doc checks for generated compatibility docs after each capability
  promotion.

## Phase 1: Front-end parity foundation

Goal: make the parser, type model, and semantic checks strong enough that broad
runtime work can trust declarations and diagnostics.

Completed:

- Generic collection method return typing and argument checks.
- Generic enhanced-for typing and direct `Map<K,V>` iteration diagnostics.
- Collection initializer and constructor type checks.
- Ternary, cast, and `instanceof` expression typing.
- Unknown type diagnostics in casts and `instanceof`.
- Common annotation semantic checks.
- Static/instance method access diagnostics.
- Black-box fixtures for generic collections, expression typing, annotations,
  and static access.

Remaining:

- Treat future front-end issues as follow-on hardening bugs unless they block a
  runtime parity slice.
- Continue adding fixtures when new runtime APIs expose parser or sema gaps.

## Phase 2: Core stdlib runtime

Goal: implement deterministic `System` namespace behavior used by ordinary Apex
business logic and tests.

Current progress:

- Collections: runtime support now covers indexed and bulk `List<T>` mutation,
  collection copy constructors, `clone`, no-argument local `deepClone`,
  primitive `List.sort`, common `Set<T>` bulk operations, deterministic
  `Map<K,V>` key/value views, previous-value, `putAll`, `containsValue`,
  `toString`, equality, and clear/isEmpty/remove behavior.
- String: runtime support now covers case-insensitive contains/prefix/suffix,
  equality/comparison, capitalization, padding/centering, left/right/mid,
  reverse, substring-before/after variants, remove variants, whitespace
  normalization, repeat overloads, `String.isEmpty`, `String.isNotEmpty`,
  regex-backed `replaceAll`/`replaceFirst`/`split` limit behavior, character
  category predicates, and `containsAny`/`containsOnly`/`containsNone`.
- Numbers: runtime support now covers `Integer.valueOf`, `Long.valueOf`,
  `Decimal.valueOf`, `Double.valueOf`, integer/decimal conversion helpers,
  simple numeric `format`, `Decimal.abs`, `Decimal.pow`, and `Math.mod`,
  `Math.signum`, and `Math.roundToLong`.
- Date/Datetime: runtime support now covers deterministic local `Date` calendar
  arithmetic and component getters, month-start/month-end helpers, and
  UTC-modeled `Datetime` date arithmetic plus date/time component getters,
  deterministic VM-clock `Date.today`/`Datetime.now`, GMT construction/parsing
  and component helpers, and millisecond arithmetic.
- Time/TimeZone: runtime support now covers `Time` construction/parsing,
  component getters including milliseconds, wraparound arithmetic, and a
  deterministic `TimeZone` slice for UTC/GMT plus fixed GMT/UTC offsets.
- JSON: runtime support now covers `JSON.createGenerator(Boolean)`,
  `JSON.createParser(String)`, and deterministic `JSONGenerator`/`JSONParser`
  slices. Generator coverage includes object/array boundaries, field names,
  string/number/Boolean/null, Date/Datetime/Time/Id/Blob and Object writers,
  `getAsString`, `close`, and `isClosed`, with explicit errors for invalid
  write order. Parser coverage includes token navigation, `JSONToken`
  constants, text/current-name accessors, numeric, Boolean, Date, Datetime,
  Time, Id, and Blob accessors, `nextValue`, and `skipChildren`.
- Blob/Encoding/Crypto: runtime support now covers deterministic local
  `Blob.valueOf`, `Blob.toString`, `Blob.size`, Base64 encode/decode,
  hex encode/decode, digest generation for the documented MD5/SHA1/SHA2/SHA3
  slice, and HMAC generation for documented local algorithms.
- Type/Id/URL/Object: runtime support now covers `Type` equality, hash and
  string forms, constructor-backed zero-arg `Type.newInstance` for registered
  classes, lightweight `Id.valueOf` validation and `to15`, deterministic URL
  parsing accessors, and primitive `Object` equality/hash/string behavior.
- System/exceptions/Type hardening: runtime support now covers deterministic
  `System.today`, `System.now`, `System.currentTimeMillis`, `System.debug`
  LoggingLevel dispatch, core exception metadata/string helpers, and local
  `Type.isAssignableFrom` for registered classes/interfaces and built-in
  exception hierarchy checks.
- String completion: runtime support now covers deterministic CSV, HTML4, XML,
  Java, EcmaScript, Unicode, and single-quote escaping/unescaping helpers;
  `{0}`-style `String.format`; abbreviation; common prefix and difference
  helpers; Levenshtein distance; rune/code-point and char-array helpers; ASCII
  printable checks; and split-by-character-type helpers.
- Fixtures:
  - `docs/fixtures/core-collection-stdlib.json`
  - `docs/fixtures/core-string-stdlib.json`
  - `docs/fixtures/core-string-more-stdlib.json`
  - `docs/fixtures/core-string-completion-stdlib.json`
  - `docs/fixtures/core-numeric-stdlib.json`
  - `docs/fixtures/core-datetime-stdlib.json`
  - `docs/fixtures/core-json-stdlib.json`
  - `docs/fixtures/core-blob-crypto-stdlib.json`
  - `docs/fixtures/core-type-id-url-stdlib.json`
  - `docs/fixtures/core-system-exceptions-stdlib.json`

Remaining cuts:

1. Collections hardening
   - Remaining gaps: iterator edge cases, null-heavy edge behavior, exact
     platform exception shapes, SObject-aware deep-clone options, and broader
     comparable-object sorting.
   - SObject-map constructors and `Map<Id,SObject>` behavior where supported by
     the front end.

2. String completion
   - Remaining StringUtils-style methods not yet pinned by owned fixtures:
     index-of-any/index-of-any-but, overlay, rotate, swapCase, strip variants,
     ordinal index helpers, and additional remove/replace edge cases.
   - Version-specific XML 1.0/1.1 validity, full HTML3/HTML4 named entity
     coverage, full Java/EcmaScript parity, and MessageFormat quoting remain
     partial until public behavior is pinned.
   - Locale overloads only after behavior is pinned to public expectations.

3. Numeric classes
   - Complete `Integer`, `Long`, `Double`, and `Decimal` formatting, min/max
     constants, overflow behavior, rounding modes, and exact scale semantics.
   - Add remaining deterministic `Math` constants and methods.

4. Date and time classes
   - Remaining work: locale-aware formatting, user-local timezone variants
     beyond the UTC model, named `TimeZone` IDs, timezone database offsets, DST
     behavior where public behavior is known, and unsupported static helpers.

5. Blob, EncodingUtil, and Crypto
   - Blob conversion and charset behavior.
   - Base64, hex, URL encoding edge cases.
   - Digests, HMAC, signatures, and encryption surfaces where local deterministic
     implementation is safe.
   - Explicit unsupported diagnostics for key-store or org-cloud-dependent
     operations.

6. JSON
   - Complete remaining `JSONGenerator` methods such as raw writers and exact
     Salesforce exception shapes.
   - Extend `JSONParser` with full streaming edge behavior, remaining accessors,
     parser recovery/clear-current-token semantics, typed deserialize, strict
     deserialize, untyped edge behavior, class/SObject mapping, additional
     suppress-null overloads, and stable error shapes.

7. System, exceptions, Type, and reflection
   - Newly covered slice: deterministic current-time helpers, debug
     LoggingLevel dispatch, exception message/type/line/stack/toString helpers,
     and local Type assignability for class/interface and built-in exception
     hierarchy checks.
   - Remaining gaps: exact assert failure message parity, full logging
     framework behavior, complete exception class matrix and stack formatting,
     Type namespace/package lookup behavior, null/unknown Type.forName edge
     behavior, generic/reflection edge cases, and cloud/org-context helpers that
     need stable unsupported diagnostics or a local model.

8. Pattern and Matcher
   - Fixture-backed Go `regexp` slice covers compile/matches/pattern,
     find/matches/lookingAt, group/groupCount/start/end, and basic replacement
     methods.
   - Remaining gap: Apex uses Java Pattern syntax; local support is Go
     `regexp`, so Java-only constructs and region/bounds/reset APIs still need
     explicit compatibility work.

9. Id, URL, and primitive object behavior
   - Id key prefix and `getSObjectType` behavior, full checksum validation, URL
     request-context/cloud-only helpers, and exact object `toString` versioned
     output for user classes.

Exit criteria:

- Each implemented type/member has catalog evidence and at least one fixture.
- Unsupported local behavior returns a stable diagnostic or runtime error.
- `go test ./...`, all compatibility fixtures, and `go vet ./...` pass.

## Phase 3: Data platform runtime

Goal: make local data behavior close enough for Apex service, trigger, selector,
and domain tests.

Cuts:

1. SObject runtime
   - Dynamic field APIs, typed field APIs, `clone`, `getPopulatedFieldsAsMap`,
     error collection, parent/child relationship access, Id typing, system
     fields, and null/error behavior.

2. Schema describe
   - Object, field, picklist, record type, child relationship, field set, tab,
     data category, and access flag describes.

3. DML and `Database.*`
   - Insert, update, upsert, delete, undelete, merge, empty recycle bin, result
     classes, all-or-none behavior, savepoints, rollback, DMLOptions, duplicate
     and validation results, status codes, and user/system mode flags.

4. SOQL, SOSL, and Search
   - Full grammar coverage, binds, relationship queries, polymorphic fields,
     aggregate functions, date literals, query locators, cursors, lock/security
     clauses, and permission-aware behavior.

5. Custom metadata and settings
   - `getAll`, `getInstance`, fixture seeding, namespace behavior, read-only
     semantics, and test isolation.

Exit criteria:

- Enterprise selector/service/domain fixtures run without unsupported core data
  paths.
- Trigger-heavy fixtures cover ordering, recursion guards, rollback, and limits.

## Phase 4: Tests, async, and limits

Goal: make the Apex test runner the proving ground for local parity.

Cuts:

1. Test runtime
   - `Test.startTest`, `Test.stopTest`, `runAs`-adjacent behavior, setup
     methods, mock lifecycle, fixed search results, page-message clearing,
     permission-set calculation, stubs, and isolation windows.

2. Async
   - Future, Queueable, Batchable, Schedulable, finalizers, AsyncOptions,
     AsyncInfo, flex queue behavior, AsyncApexJob and CronTrigger fields, and
     `stopTest` drain order.

3. Limits
   - All documented getters, separate sync/async/test windows, configurable org
     caps, exact failure types, and deterministic accounting for local runtime
     work.

Exit criteria:

- Async/test/limits fixtures model realistic test classes and pass under strict
  limit mode.
- Limit failures never panic and report stable Apex-shaped errors.

## Phase 5: Integration, security, UI, and events

Goal: provide executable local models where useful and explicit stubs where
external systems dominate.

Cuts:

1. HTTP and callouts
   - HttpRequest, HttpResponse, Http, callout mocks, static-resource mocks,
     Continuation, timeout/error shapes, and callout limits.

2. REST
   - RestContext, RestRequest, RestResponse, annotation dispatch, local server
     integration, request/response property parity, and JSON body behavior.

3. Security and Auth
   - UserInfo, FeatureManagement, permission fixtures, Auth token classes as
     typed stubs or local token models, and permission checks wired into data
     operations.

4. UI and messaging
   - ApexPages, PageReference, QuickAction, Messaging email/push, and
     Visualforce-adjacent state without claiming renderer lifecycle parity.

5. Events
   - EventBus and platform events as local publish/subscribe records with trigger
     dispatch if the runtime model can support it; otherwise typed stubs with
     explicit unsupported diagnostics.

Exit criteria:

- Local server, REST, callout mock, and security fixtures cover common
  integration test shapes.

## Phase 6: Product namespace typed stubs

Goal: let projects compile against broad public product namespaces without
pretending cloud services run locally.

Completed foundation:

- `oaer compat product-namespaces` reports typed-stub targets and namespace
  counts from the public docs catalog.
- Unknown `System` docs stay in core stdlib instead of product namespaces.

Remaining:

- Generate declarations for ConnectApi, Reports, Commerce, DataSource,
  RichMessaging, Datacloud, Process, Cache, Canvas, Functions, Industries,
  UserProvisioning, and other public product namespaces.
- Mark each namespace as one of:
  - typed DTOs only
  - DTOs plus deterministic local read/search behavior
  - executable local model
  - unsupported external operation with stable diagnostic

Exit criteria:

- Product namespace symbols compile with typed declarations.
- External operations return stable unsupported diagnostics unless a local model
  is documented and fixture-backed.

## Phase 7: Product namespace local models

Goal: implement useful local behavior for selected product namespaces after the
typed-stub layer exists.

Candidate order:

1. Cache-like deterministic local stores.
2. Reports read/query DTOs if fixture data can model them.
3. ConnectApi DTO construction and pure validation helpers.
4. Commerce and industry operations only where a local data model is explicit.

Exit criteria:

- Every promoted namespace has a local behavior target, fixtures, and generated
  docs coverage.

## Phase 8: MVP/readiness hardening

Goal: turn parity slices into a reliable release gate.

Cuts:

1. Capability promotion
   - Move only fixture-backed, docs-backed behavior to `supported`.
   - Keep partial/stub/unsupported statuses explicit.

2. Hardening
   - Panic-free malformed input across parse, sema, VM, DML, SOQL, metadata,
     fixtures, LSP, DAP, and server paths.
   - Stable source ranges and diagnostics from parse through runtime traces.

3. Performance and scale
   - Bench parser, symbol index, sema, SOQL, DML, VM, tests, storage, server,
     LSP, and watch mode.
   - Add targeted optimizations only where benchmarks show regressions.

4. Release readiness
   - `oaer compat mvp --require-ready` passes.
   - Generated docs are in sync.
   - Smoke tests cover CLI, DB, LSP, server, profile, compat, and test runner
     surfaces.

## Update protocol

When a parity slice lands:

1. Add or update fixture evidence.
2. Update the relevant phase's "Current progress" or "Completed" section.
3. Move any completed cut out of "Remaining" or annotate it with the supported
   subset.
4. Update generated compatibility docs if capability statuses changed.
5. Record validation commands in the PR or session notes.
6. Leave unresolved behavior in this document with an explicit target:
   executable, typed stub, unsupported diagnostic, or deferred research.
