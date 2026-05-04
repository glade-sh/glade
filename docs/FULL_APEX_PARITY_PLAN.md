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
  primitive `List.sort`, snapshot `List<T>`/`Set<T>` iterators with
  `hasNext`/`next`, stable unsupported `Iterator.remove`, common `Set<T>`
  bulk operations, deterministic `Map<K,V>` key/value views, previous-value,
  `putAll`, `containsValue`, `toString`, equality, clear/isEmpty/remove
  behavior, null membership edges, recursive no-argument SObject deepClone
  coverage, typed unsupported diagnostics for preserve-option deepClone and
  object/Comparable sort gaps, and bounded `Map<Id,SObject>`
  construction/`putAll(List<SObject>)` for rows with non-null unique Ids.
- String: runtime support now covers case-insensitive contains/prefix/suffix,
  equality/comparison, capitalization, padding/centering, left/right/mid,
  reverse, substring-before/after variants, remove variants, whitespace
  normalization, repeat overloads, `String.isEmpty`, `String.isNotEmpty`,
  regex-backed `replaceAll`/`replaceFirst`/`split` limit behavior, character
  category predicates, and `containsAny`/`containsOnly`/`containsNone`.
- Numbers: runtime support now covers `Integer.valueOf`, `Long.valueOf`,
  `Decimal.valueOf`, `Double.valueOf` including trimmed signed strings,
  integer/decimal conversion helpers, simple finite numeric `format` with
  explicit unsupported diagnostics for locale/pattern overloads,
  `Integer`/`Long` min/max constants, `Decimal.abs`, finite `Decimal.pow`,
  `Decimal.setScale`/`Decimal.round` for exact-name local `RoundingMode`
  values, parse, integer-conversion, and local finite-overflow errors,
  `Math.E`, `Math.PI`, `Math.abs`, `Math.ceil`, `Math.floor`, `Math.max`,
  `Math.min`, `Math.mod`, finite `Math.pow`, `Math.round`,
  `Math.roundToLong`, `Math.signum`, `Math.sqrt`, and deterministic
  trig/log/exp helpers (`acos`, `asin`, `atan`, `atan2`, `cos`, `sin`, `tan`,
  `exp`, `log`, `log10`) for finite pinned domains.
- Date/Datetime: runtime support now covers deterministic local `Date` calendar
  arithmetic and component getters, month-start/month-end helpers, and
  UTC-modeled `Datetime` date arithmetic plus date/time component getters,
  deterministic VM-clock `Date.today`/`Datetime.now`, GMT construction/parsing
  and component helpers, millisecond arithmetic, deterministic `Datetime`
  `format`/`formatGmt` Java-pattern slices for UTC and fixed offsets, stable
  invalid parse/pattern diagnostics, and explicit unsupported diagnostics for
  named timezone formatting.
- Time/TimeZone: runtime support now covers `Time` construction/parsing,
  component getters including milliseconds, wraparound arithmetic, and a
  deterministic `TimeZone` slice for UTC/GMT plus fixed GMT/UTC offsets through
  the local `+14:00` edge with named-zone, invalid-ID, DST, and locale overloads
  reported as unsupported.
- JSON: runtime support now covers `JSON.createGenerator(Boolean)`,
  `JSON.createParser(String)`, and deterministic `JSONGenerator`/`JSONParser`
  slices. Generator coverage includes object/array boundaries, field names,
  string/number/Boolean/null, Date/Datetime/Time/Id/Blob, Object, and
  validated raw value writers, `getAsString`, `close`, and `isClosed`, with
  explicit errors for invalid write order and invalid raw JSON values. Parser
  coverage includes token navigation, `JSONToken` constants, text/current-name
  accessors, integer/long/decimal/double, Boolean, Date, Datetime, Time, Id,
  and Blob accessors, `nextValue`, `skipChildren`, and `clearCurrentToken`.
  Serialize coverage includes compact/pretty output for nested supported maps,
  lists, and objects, with `suppressApexObjectNulls` limited to Apex object
  fields while map/list nulls are preserved.
  Deserialize coverage includes deterministic untyped primitive/list/map/null
  mapping, typed primitive/platform scalar mapping, `List<T>` and
  `Map<String,T>` via local `Type` tokens, strict unknown-field rejection for
  supported SObject/class targets, stable typed shape errors, and typed
  unsupported diagnostics for unregistered object mapping targets.
- Blob/Encoding/Crypto: runtime support now covers deterministic local
  `Blob.valueOf`, `Blob.toString`, `Blob.size`, Base64 encode/decode,
  hex encode/decode, empty Blob/encoding edge cases, invalid Base64/hex and
  non-UTF-8 Blob string errors, URL encode/decode for the UTF-8/utf8/UTF_8
  charset slice with typed unsupported diagnostics for other charsets, digest
  generation for the documented MD5/SHA1/SHA2/SHA3 slice, HMAC generation for
  documented local algorithms with conservative algorithm normalization,
  `Crypto.areEqualConstantTime`, and explicit unsupported errors for local
  key/certificate/encryption/random key-generation surfaces.
- Type/Id/URL/Object: runtime support now covers `Type` equality, hash and
  string forms, constructor-backed zero-arg `Type.newInstance` for registered
  classes, strict 18-character `Id.valueOf` checksum validation, `to15`,
  `to18`, bounded `Id.getSObjectType` key-prefix tokens with stable unknown
  prefix errors, deterministic URL parsing accessors, context/spec URL
  resolution, malformed URL constructor errors, explicit unsupported
  request-context URL helpers, unbacked namespace/package `Type.newInstance`
  diagnostics, and primitive `Object` equality/hash/string behavior.
- System/exceptions/Type hardening: runtime support now covers deterministic
  `System.today`, `System.now`, `System.currentTimeMillis`, `System.debug`
  LoggingLevel dispatch, built-in `LoggingLevel` enum value helpers, local
  false-valued async context probes (`System.isBatch`, `System.isFuture`,
  `System.isQueueable`, `System.isScheduled`), core exception
  metadata/string/cause helpers, assertion Object-message string conversion,
  deterministic null/exception debug formatting, null/blank/unknown local
  `Type.forName` edges, and local `Type.isAssignableFrom` for registered
  classes/interfaces and built-in exception hierarchy checks.
- String completion: runtime support now covers deterministic CSV, HTML4, XML,
  Java, EcmaScript, Unicode, and single-quote escaping/unescaping helpers;
  `{0}`-style `String.format`; abbreviation; common prefix and difference
  helpers; Levenshtein distance; rune/code-point and char-array helpers; ASCII
  printable checks; split-by-character-type helpers; index-of-any and
  ordinal-index helpers; overlay, rotate, swapCase, strip variants; static
  `stripAll`; additional literal remove/replace edge helpers; and pinned
  deterministic core HTML/XML entity edges with unknown named entities left
  unchanged.
- Fixtures:
  - `docs/fixtures/core-collection-stdlib.json`
  - `docs/fixtures/core-collection-stdlib-sobject-deepclone.json`
  - `docs/fixtures/core-collection-stdlib-unsupported-list-deepclone-options.json`
  - `docs/fixtures/core-collection-stdlib-unsupported-set-deepclone-options.json`
  - `docs/fixtures/core-collection-stdlib-unsupported-map-deepclone-options.json`
  - `docs/fixtures/core-collection-stdlib-unsupported-sort-object.json`
  - `docs/fixtures/core-string-stdlib.json`
  - `docs/fixtures/core-string-more-stdlib.json`
  - `docs/fixtures/core-json-stdlib.json`
  - `docs/fixtures/core-json-generator-invalid-state.json`
  - `docs/fixtures/core-json-unsupported-mapping.json`
  - `docs/fixtures/core-string-completion-stdlib.json`
  - `docs/fixtures/core-string-entity-edge-stdlib.json`
  - `docs/fixtures/core-numeric-stdlib.json`
  - `docs/fixtures/core-numeric-stdlib-unsupported-format.json`
  - `docs/fixtures/core-numeric-stdlib-unsupported-long-format.json`
  - `docs/fixtures/core-numeric-stdlib-unsupported-decimal-format.json`
  - `docs/fixtures/core-numeric-stdlib-unsupported-double-format.json`
  - `docs/fixtures/core-numeric-stdlib-invalid-finite.json`
  - `docs/fixtures/core-numeric-stdlib-invalid-rounding-mode.json`
  - `docs/fixtures/core-datetime-stdlib.json`
  - `docs/fixtures/core-datetime-invalid-date.json`
  - `docs/fixtures/core-datetime-invalid-datetime.json`
  - `docs/fixtures/core-datetime-invalid-time.json`
  - `docs/fixtures/core-datetime-format-unsupported-token.json`
  - `docs/fixtures/core-datetime-format-unsupported-timezone.json`
  - `docs/fixtures/core-timezone-unsupported-named-zone.json`
  - `docs/fixtures/core-timezone-unsupported-display-overload.json`
  - `docs/fixtures/core-json-stdlib.json`
  - `docs/fixtures/core-blob-crypto-stdlib.json`
  - `docs/fixtures/core-blob-crypto-invalid-base64.json`
  - `docs/fixtures/core-blob-crypto-invalid-hex.json`
  - `docs/fixtures/core-blob-crypto-invalid-utf8.json`
  - `docs/fixtures/core-blob-crypto-unsupported-charset.json`
  - `docs/fixtures/core-blob-crypto-unsupported-random.json`
  - `docs/fixtures/core-pattern-matcher-stdlib.json`
  - `docs/fixtures/core-type-id-url-stdlib.json`
  - `docs/fixtures/core-system-exceptions-stdlib.json`
  - `docs/fixtures/core-system-assert-null-message.json`
  - `docs/fixtures/core-system-assert-message-edges.json`
  - `docs/fixtures/core-system-assertnot-message-edge.json`
  - `docs/fixtures/core-system-debug-invalid-overload.json`
  - `docs/fixtures/core-system-async-unsupported.json`

Remaining cuts:

1. Collections hardening
   - Remaining gaps: iterator edge cases, exact platform exception shapes,
      SObject-aware deep-clone options, broader comparable-object sorting, and
      wider SObject-map behavior beyond the covered non-null unique-Id slice.

2. String completion
   - Remaining StringUtils-style methods not yet pinned by owned fixtures:
      additional remove/replace overloads and null-heavy edge cases beyond the
      deterministic literal helper slice now covered.
   - Version-specific XML 1.0/1.1 validity, full HTML3/HTML4 named entity
      coverage beyond core/numeric entity references, full Java/EcmaScript
      parity, and MessageFormat quoting remain partial until public behavior is
      pinned.
   - Locale overloads only after behavior is pinned to public expectations.

3. Numeric classes
   - Complete locale-aware `Integer`, `Long`, `Double`, and `Decimal`
     formatting remains unsupported beyond typed diagnostics for local format
     overload calls; full 32-bit-vs-64-bit Integer/Long overflow parity;
     Decimal rounding behavior beyond the local scale 0-15 subset; and exact
     Decimal scale semantics.
   - Pin remaining edge behavior for numeric NaN/infinity/domain cases before
     widening deterministic `Math` parity beyond the currently covered finite
     slice, finite-overflow checks, and explicit domain errors.

4. Date and time classes
   - Remaining work: full locale-aware formatting beyond the pinned English
     `Datetime` pattern tokens, user-local timezone variants beyond the UTC
     model, named `TimeZone` IDs, timezone database offsets, DST behavior where
     public behavior is known, `Date`/`Time` pattern overloads if public
     behavior is pinned, and unsupported static helpers.
   - Newly pinned edge coverage: strict invalid Date/Datetime/Time parsing,
     year 1-9999 component bounds, fixed-offset TimeZone IDs through `±14:00`,
     unsupported named timezone IDs, unsupported DST/locale display-name
     overloads, stable Datetime format token errors, and unsupported named-zone
     formatting errors.

5. Blob, EncodingUtil, and Crypto
   - Newly covered slice: invalid Base64 and odd/invalid hex errors,
     non-UTF-8 `Blob.toString` rejection, UTF-8 charset aliases for URL
     encode/decode, typed unsupported diagnostics for non-UTF-8 URL charsets,
     conservative digest/HMAC algorithm normalization, and stable unsupported
     diagnostics for random AES key generation.
   - Remaining Base64, hex, and URL encoding edge cases beyond the pinned
     empty, casing, invalid-input, and UTF-8 URL charset slice.
   - Digests, HMAC, signatures, and encryption surfaces where local deterministic
     implementation is safe.
   - Additional explicit unsupported diagnostics for any key-store or
     org-cloud-dependent operations not yet routed through stable errors.

6. JSON
   - Complete remaining `JSONGenerator` overloads and exact Salesforce
     exception shapes; raw writer support is limited to validated single-value
     raw JSON strings.
   - Extend `JSONParser` with full streaming edge behavior, parser recovery
     semantics beyond the deterministic clear-current-token and skip-children
     current-name slices.
   - Extend `JSON.deserialize`/`deserializeStrict` beyond the bounded local
     typed primitive/platform scalar, `List<T>`, `Map<String,T>`, and supported
     class/SObject slices; remaining gaps include exact platform exception
     shapes, full class/SObject mapping parity, polymorphic mappings, and
     unsupported Map key coercions. Unregistered object mapping targets now
     return a typed unsupported diagnostic instead of invented parity.
   - Add any remaining suppress-null overloads and edge parity not covered by
     the current `serialize`/`serializePretty` Boolean overload slice.

7. System, exceptions, Type, and reflection
    - Newly covered slice: deterministic current-time helpers, debug
      LoggingLevel dispatch, built-in LoggingLevel values/name/ordinal/toString,
      local false-valued async context probes, exception
      message/type/line/stack/toString helpers, null/blank/unknown local
      Type.forName edges, known built-in exception type construction and
      assignability, assertion message conversion edges, debug overload
      diagnostics, and explicit unsupported diagnostics for unmodeled local async
      lifecycle controls.
    - Remaining gaps: exact assert failure message parity beyond pinned local
      strings, full logging framework behavior beyond collected debug lines and
      enum values, complete platform exception catalog/stack formatting parity,
      Type namespace/package lookup behavior, generic/reflection edge cases, and
      broader cloud/org-context helpers that need stable unsupported diagnostics
      or a local model.

8. Pattern and Matcher
   - Fixture-backed Go `regexp` slice covers compile/matches/pattern,
     split, find/matches/lookingAt, group/groupCount/start/end, reset, local
     anchoring/transparent bounds flags, bounded region/regionStart/regionEnd,
     usePattern, and basic replacement methods. Region-aware replaceAll and
     replaceFirst preserve text outside the local region, and escaped-dollar
     replacement handling has deterministic unit coverage.
   - Remaining gap: Apex uses Java Pattern syntax; local support is Go
     `regexp`, so broader Java-only constructs, exact bounds-region
     interaction, appendReplacement/appendTail StringBuffer behavior, and full
     Java replacement semantics still need explicit compatibility work or remain
     explicit unsupported diagnostics.

9. Id, URL, and primitive object behavior
   - Landed slice: strict 18-character Id checksum validation, `Id.to18`,
     bounded local/common key-prefix `Id.getSObjectType` with stable unknown
     prefix errors, context/spec URL constructor resolution, malformed URL
     constructor diagnostics, explicit unsupported request-context URL helpers,
     and unbacked namespace/package `Type.newInstance` unsupported diagnostics.
   - Remaining gaps: broader/versioned key-prefix catalog behavior, URL
     request-context/cloud-only helpers beyond explicit unsupported diagnostics,
     exact object `toString` versioned output for user classes, Type
     package lookup edge cases, and broader reflection behavior.

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
