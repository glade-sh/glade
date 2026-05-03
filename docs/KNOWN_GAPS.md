# Known Gaps

Generated from `internal/capability`.

The MVP target is `full-featured aer-parity MVP`. This document lists required capabilities that are not yet `supported`.

## Summary

- Required complete: 4/21
- Required incomplete: 17

## Apex front end

### `apex.parser.project-scale`: Parse and index large SFDX projects

- Status: `partial`
- Gap: Parser and symbol baselines exist, including qualified nested type symbols, stable malformed-parse diagnostics, type-index/sema panic recovery diagnostics, and an enterprise-style multi-class check fixture. Broader real-repository scale and method-body model fidelity are incomplete.

### `apex.sema.body`: Method-body semantic analysis

- Status: `partial`
- Gap: Current sema checks declarations, member and parameter type references, project namespace-qualified and namespace-token schema references, basic visibility conflicts, interface member visibility, override markers, missing concrete interface/abstract method implementations, and a conservative method-body baseline for local declarations, duplicate locals, local initializer and simple assignment type mismatches, simple return type mismatches and missing non-void returns, constructor references, constructor chaining, non-instantiable interface/enum/abstract constructor calls, unknown variable reads in call arguments, project method calls, inherited/interface/super calls, this/super field and return type inference, inherited instance field scope, private/protected method and field visibility through inheritance chains, @TestVisible method access from test classes, known-receiver overload arity/argument matching with exact and narrowest numeric specificity, nearest class/interface specificity, null specificity, and ambiguous overload diagnostics, integer-to-Long/Decimal/Double widening, decimal-literal argument typing, simple binary expression typing, class/interface object assignability, generic collection constructor assignability, known method-call return typing for receiver and chained constructor calls, an IR-backed sema pass for scoped local reads, Boolean conditions, declaration/assignment/return type checks, all-path non-void returns, known user-object field reads/writes, known receiver/same-class method calls, and constructor-call validation across statements/control-flow bodies, and token-level ranges for body diagnostics. Full expression typing and full flow analysis remain incomplete.

## Data runtime

### `dml.apex`: Apex DML statements and Database methods

- Status: `partial`
- Gap: Apex insert/update/delete/upsert/undelete/merge syntax and Database.insert/update/delete/upsert/undelete/merge allOrNone paths now call the DML engine, return SaveResult/UpsertResult/MergeResult objects with single and multi-entry Database.Error lists carrying statusCode, message, and fields arrays, isCreated, merged record IDs, set Ids, stamp common system fields, roll back allOrNone failures, soft-delete and undelete records, match implicit and explicit external-ID upserts, reject ID/object mismatches, enforce unique fields, validate lookup references, reparent lookups on merge, fire supported merge update/delete trigger hooks, and cascade soft-delete children from relationship metadata. Trigger context includes operation flags, size, new/old lists, nullable unavailable contexts, and newMap/oldMap for supported operations. Validation-rule formulas, full merge loser relationship result details, and full Salesforce status-code parity remain incomplete.

### `fixtures.persistence`: Seed/export/reset local fixtures with persistence

- Status: `partial`
- Gap: SQLite-backed org storage now persists object definitions, records, ID sequences, schema migrations/versioning, fixture seed/export/reset/inspect, alias and relationship reference resolution, deterministic users/profiles/permissions, DB lifecycle compatibility coverage, server persistence, and fixture/reset endpoints. Large-fixture performance tuning and richer permission semantics remain incomplete.

### `sobject.apex`: Apex-integrated SObject construction and field access

- Status: `partial`
- Gap: Apex now supports schema-backed new Account(Name='Acme'), typed field access, dotted assignment, get/put with previous-value return, isSet, clear, getPopulatedFieldsAsMap including explicit nulls, common system fields after DML and SOQL projection, field describe basics with picklist values, record type describe maps/lists with deterministic local IDs and common RecordTypeInfo methods, object-level and field-level addError, multi-error hasErrors/getErrors and DML result shaping, Id propagation after DML, parent relationship projection access, and VM/storage record conversion. Permissions, complete typed describe APIs, and broader system field parity remain incomplete.

### `soql.apex`: Static and dynamic SOQL from Apex

- Status: `partial`
- Gap: Static SOQL literals and Database.query now execute against the in-memory org with bind variables beside operators, dotted bind paths, collection binds, projection, FIELDS(ALL/STANDARD/CUSTOM), TYPEOF relationship projection, multi-hop parent relationship fields/filters, child relationship subqueries, semi-joins, anti-joins, COUNT(), COUNT(field), COUNT_DISTINCT, SUM, MIN, MAX, AVG, GROUP BY, ROLLUP, CUBE, HAVING on aggregate expressions, aggregate aliases, GROUPING(field), common date literals, AggregateResult exprN fields, single-SObject assignment, soft-deleted row visibility through ALL ROWS, equality/inequality/comparison filters, AND/OR boolean combinations, IN/NOT IN, LIKE, NOT, parentheses, comma-separated ORDER BY ASC/DESC with NULLS FIRST/LAST, FOR UPDATE parsing, WITH SECURITY_ENFORCED/USER_MODE/SYSTEM_MODE parsing, limit, offset, and QueryException parse errors. SQLite planning, lock contention, security enforcement, and advanced polymorphic relationship behavior remain incomplete.

### `triggers.runtime`: Trigger invocation and context

- Status: `partial`
- Gap: Project triggers are compiled and invoked from VM DML for before/after operations with Trigger.new/old/maps/flags/operationType/size basics, bulk partial-success row alignment, deterministic recursion guard rollback, merge master update hooks, merge duplicate delete hooks, rollback on thrown errors, and object-level/field-level addError shaping single and multiple row SaveResult errors with field lists. Full bulk ordering semantics and relationship side effects remain incomplete.

## Developer experience

### `dap.command`: VS Code debug flow through oaer test/exec --debug

- Status: `partial`
- Gap: DAP content-length transport, setBreakpoints, continue/pause/next, stackTrace with trace-provided line/column positions, scopes, variables, evaluate, and oaer exec/test --debug snapshot sessions are wired. True live VM suspension, step-in/out semantics, and breakpoint-driven execution control remain incomplete.

### `lsp.command`: oaer lsp core editor features

- Status: `partial`
- Gap: oaer lsp now runs a stdio LSP transport with initialize, diagnostics, document/workspace symbols, hover, and completion. Definition, references, semantic tokens, and incremental document sync remain incomplete.

### `profile.native`: Native trace/profile reports

- Status: `partial`
- Gap: Trace/profile reports aggregate statements, methods, SOQL, DML, source offsets, statement line/column ranges, event categories, and governor-like SOQL/DML deltas. pprof-compatible CPU output and per-statement wall-clock timing remain incomplete.

### `watch.command`: oaer test --watch affected-test loop

- Status: `partial`
- Gap: oaer test --watch now runs a polling watch loop with debounce, JSON event stream, affected-test selection, reruns, and context cancellation. Native OS watcher backends and in-flight VM cancellation remain incomplete.

## Limits

### `limits.core`: Governor counters and strict/permissive enforcement

- Status: `partial`
- Gap: The VM now tracks SOQL queries/rows, DML statements/rows, approximate heap, statement-count CPU, callouts, aggregate async jobs, future calls, queueable jobs, batch jobs, scheduled jobs, and email invocations. Limits.* exposes current and max counters for supported SOQL, DML, heap, CPU, callout, aggregate async, future, queueable, and email counters, Test.startTest/Test.stopTest reset and restore test windows, permissive mode records violations, strict mode raises System.LimitException, and oaer exec/test accept --limit-mode. Exact Salesforce accounting and configurable per-test caps remain incomplete.

## Local API server

### `server.local-api`: Salesforce-shaped local API with CRUD/query/executeAnonymous

- Status: `partial`
- Gap: CRUD/query/queryAll, describe/recent, limits, OAuth userinfo/id stubs, Tooling executeAnonymous, composite sObject insert, normal REST JSON payloads, Salesforce-shaped error arrays, SQLite persistence, and fixture/reset endpoints are wired. Full auth, Tooling object coverage, Composite Graph, Bulk API, and broader REST resources remain incomplete.

## Release

### `compat.dashboard`: Generated compatibility dashboard and CI gate

- Status: `partial`
- Gap: The MVP gate, JSON matrix, generated Markdown dashboard, CI drift check, and parse/check/exec/test/DB lifecycle fixture runner exist. Compatibility fixtures still need expansion before this can be supported.

### `release.packaging`: Installable release binaries, checksums, docs

- Status: `partial`
- Gap: Release archives, checksums, GitHub Release upload workflow, install docs, release policy, and smoke coverage exist. Published package-manager distribution, stronger artifact signing, and release promotion automation remain incomplete.

## Runtime

### `stdlib.core`: Core System/String/Date/Datetime/JSON/Math APIs

- Status: `partial`
- Gap: Assertions, debug, collections, selected String methods, Limits counters including future, queueable, and email getters, Date/Datetime/Time basics, Decimal literals/arithmetic/storage conversion, Math integer helpers, JSON serialize/deserializeUntyped, EncodingUtil, Crypto SHA-256, Schema global describe basics, SObject field describe basics with picklist entries, SObject record type describe maps/lists with common RecordTypeInfo methods, UserInfo user/profile identity in test context, FeatureManagement user permission-list checks, Messaging, ApexPages, and HttpResponse-shaped callout mocks now exist for the supported VM subset.

### `vm.control-flow`: Control flow and exceptions

- Status: `partial`
- Gap: Anonymous and test method execution now supports for/enhanced-for/do-while, break/continue, switch-on with switch-local break, throw, ordered catch blocks, pipe-style multi-catch, try/catch/finally including finally-on-return, return override, throw unwinding, finally-preserved and finally-overridden loop signals, bare rethrow with original stack preservation, exception messages/getMessage, getTypeName, getLineNumber, getStackTraceString, System.*Exception name normalization, catchable null dereference, interface-based catch matching, and exception hierarchy matching for common Apex exception types. Remaining gaps are outside control-flow and exception semantics.

