# Known Gaps

Generated from `internal/capability`.

The MVP target is `full-featured aer-parity MVP`. This document lists required capabilities that are not yet `supported`.

## Summary

- Required complete: 0/21
- Required incomplete: 21

## Apex front end

### `apex.parser.project-scale`: Parse and index large SFDX projects

- Status: `partial`
- Gap: Parser and symbol baselines exist; method-body model and large-project compatibility fixtures are incomplete.

### `apex.sema.body`: Method-body semantic analysis

- Status: `partial`
- Gap: Current sema checks declarations, member and parameter type references, project namespace-qualified type references, basic visibility conflicts, interface member visibility, and a conservative method-body baseline for local declarations, constructor references, simple assignments, project method calls, and known-receiver overload arity/simple argument type matching. Full expression typing, flow-sensitive scopes, inherited/interface dispatch, and Apex coercion rules remain incomplete.

## Data runtime

### `dml.apex`: Apex DML statements and Database methods

- Status: `partial`
- Gap: Apex insert/update/delete/upsert/undelete syntax and Database.insert/update/delete allOrNone paths now call the DML engine, return SaveResult-like objects, set Ids, and roll back allOrNone failures. Merge, external-id upsert, undelete fidelity, and full error arrays remain incomplete.

### `fixtures.persistence`: Seed/export/reset local fixtures with persistence

- Status: `partial`
- Gap: SQLite-backed org storage now persists object definitions, records, ID sequences, fixture seed/export/reset/inspect, alias and relationship reference resolution, deterministic users/profiles/permissions, server persistence, and fixture/reset endpoints. Large-fixture performance tuning and richer permission semantics remain incomplete.

### `sobject.apex`: Apex-integrated SObject construction and field access

- Status: `partial`
- Gap: Apex now supports schema-backed new Account(Name='Acme'), typed field access, dotted assignment, get/put, Id propagation after DML, parent relationship projection access, and VM/storage record conversion. Typed describe APIs and broader SObject system fields remain incomplete.

### `soql.apex`: Static and dynamic SOQL from Apex

- Status: `partial`
- Gap: Static SOQL literals and Database.query now execute against the in-memory org with simple bind variables, projection, parent relationship fields, COUNT(), single-SObject assignment, equality/inequality filters, order, limit, and offset. Subqueries, broader aggregates, complex predicates, and SQLite planning remain incomplete.

### `triggers.runtime`: Trigger invocation and context

- Status: `partial`
- Gap: Project triggers are compiled and invoked from VM DML for before/after operations with Trigger.new/old/maps/flags/operationType/size basics and rollback on thrown errors. Full bulk ordering semantics, recursive limits, addError, undelete storage state, and relationship side effects remain incomplete.

## Developer experience

### `dap.command`: VS Code debug flow through oaer test/exec --debug

- Status: `partial`
- Gap: DAP content-length transport, setBreakpoints, continue/pause/next, stackTrace, scopes, variables, evaluate, and oaer exec/test --debug snapshot sessions are wired. True live VM suspension, step-in/out semantics, and breakpoint-driven execution control remain incomplete.

### `lsp.command`: oaer lsp core editor features

- Status: `partial`
- Gap: oaer lsp now runs a stdio LSP transport with initialize, diagnostics, document/workspace symbols, hover, and completion. Definition, references, semantic tokens, and incremental document sync remain incomplete.

### `profile.native`: Native trace/profile reports

- Status: `partial`
- Gap: Trace/profile reports aggregate statements, methods, SOQL, DML, source offsets, event categories, and governor-like SOQL/DML deltas. pprof-compatible CPU output and per-statement wall-clock timing remain incomplete.

### `watch.command`: oaer test --watch affected-test loop

- Status: `partial`
- Gap: oaer test --watch now runs a polling watch loop with debounce, JSON event stream, affected-test selection, reruns, and context cancellation. Native OS watcher backends and in-flight VM cancellation remain incomplete.

## Limits

### `limits.core`: Governor counters and strict/permissive enforcement

- Status: `partial`
- Gap: The VM now tracks SOQL queries/rows, DML statements/rows, approximate heap, statement-count CPU, callouts, and async jobs. Limits.* exposes current and max counters, permissive mode records violations, strict mode raises System.LimitException, and oaer exec/test accept --limit-mode. Exact Salesforce accounting and configurable per-test caps remain incomplete.

## Local API server

### `server.local-api`: Salesforce-shaped local API with CRUD/query/executeAnonymous

- Status: `partial`
- Gap: CRUD/query/queryAll, describe/recent, limits, OAuth userinfo/id stubs, Tooling executeAnonymous, composite sObject insert, normal REST JSON payloads, Salesforce-shaped error arrays, SQLite persistence, and fixture/reset endpoints are wired. Full auth, Tooling object coverage, Composite Graph, Bulk API, and broader REST resources remain incomplete.

## Release

### `compat.dashboard`: Generated compatibility dashboard and CI gate

- Status: `partial`
- Gap: The MVP gate, JSON matrix, generated Markdown dashboard, and CI drift check exist. Compatibility fixtures still need expansion before this can be supported.

### `release.packaging`: Installable release binaries, checksums, docs

- Status: `unsupported`
- Gap: Release packaging is not implemented.

## Runtime

### `stdlib.core`: Core System/String/Date/Datetime/JSON/Math APIs

- Status: `partial`
- Gap: Assertions, debug, collections, selected String methods, Limits counters, Date/Datetime/Time basics, Math integer helpers, JSON serialize/deserializeUntyped, EncodingUtil, Crypto SHA-256, Schema global describe basics, UserInfo, FeatureManagement, Messaging, ApexPages, and HttpResponse-shaped callout mocks now exist for the supported VM subset.

### `vm.classes`: Classes, methods, constructors, statics, properties

- Status: `partial`
- Gap: The VM now registers class metadata from project tests, constructs objects with instance fields/properties, invokes property getter/setter bodies, runs constructor bodies, supports this(...) and super(...) constructor chaining, matches overloaded methods/constructors by argument types, executes static and instance initializer blocks, preserves source field order for initialization/reset, resets statics through initializer blocks, stores static fields, dispatches overrides through inheritance, supports super method calls, resolves namespace-qualified class names, and enforces a runtime baseline for private/protected and namespace-global access. Full Apex visibility/test-visible rules, package semantics, field initializer expression ordering, and generic overload/coercion fidelity remain incomplete.

### `vm.control-flow`: Control flow and exceptions

- Status: `partial`
- Gap: Anonymous and test method execution now supports for/enhanced-for/do-while, break/continue, switch-on, throw, try/catch/finally, multi-catch, bare rethrow, exception messages/getMessage, catchable null dereference, and basic exception stack reporting. Complete Apex exception hierarchy semantics remain incomplete.

## Tests

### `async.core`: Queueable/Future/Batch/Scheduled basics

- Status: `partial`
- Gap: System.enqueueJob queues object jobs in test context and Test.stopTest drains Queueable execute methods. Future, Batchable, Schedulable, chained job limits, and durable async job records remain incomplete.

### `tests.runner`: Run real Apex test classes

- Status: `partial`
- Gap: Discovery, method dispatch, @TestSetup execution, static reset, startTest/stopTest, runAs, Queueable stopTest draining, and assertion stack frames now work for the supported VM subset.

### `tests.salesforce-semantics`: @TestSetup, startTest/stopTest, runAs, isolation

- Status: `partial`
- Gap: @TestSetup methods execute before each test with statics reset before the test body; each test gets a fresh cloned org and VM for isolation; startTest/stopTest and runAs are modeled. Exact governor window restoration, profile/permission semantics, and platform auth details remain incomplete.

