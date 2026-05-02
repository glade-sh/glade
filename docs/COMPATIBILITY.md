# Compatibility Policy

`oaer` tracks compatibility at feature level. Every language, runtime,
metadata, SOQL, DML, library, tooling, and server feature should be classified
with one of these statuses:

- `supported`: implemented and covered by compatibility tests.
- `partial`: works for common cases with documented gaps.
- `stub`: exists so code can load, but returns a controlled placeholder or
  explicit unsupported result.
- `unsupported`: fails with a stable diagnostic before or during execution.
- `unknown`: not evaluated yet.

Silent wrong behavior is a release blocker for any feature marked `supported`.
Unsupported behavior should fail loudly and predictably.

## MVP Gate

The MVP target is full-featured aer-parity for local Apex development. The
current implementation should not be called MVP-ready while required features
are `partial`, `stub`, `unsupported`, or `unknown`.

Use:

```bash
oaer compat mvp
oaer compat matrix --json
oaer compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
oaer compat gaps --output docs/KNOWN_GAPS.md
```

The source of truth for required MVP capabilities is `internal/capability`.
The generated public dashboard is checked in at
[`docs/COMPATIBILITY_DASHBOARD.md`](COMPATIBILITY_DASHBOARD.md). The generated
known-gaps document is checked in at [`docs/KNOWN_GAPS.md`](KNOWN_GAPS.md). CI
verifies that both generated documents match the capability source.

Release installation and artifact verification are documented in
[`docs/INSTALL.md`](INSTALL.md).
Release promotion, upgrade, and API-version compatibility policy are documented
in [`docs/RELEASE_POLICY.md`](RELEASE_POLICY.md), with ongoing notes in
[`docs/RELEASE_NOTES.md`](RELEASE_NOTES.md).

## Initial Matrix

| Area | Status | Notes |
| --- | --- | --- |
| CLI surface | partial | `version`, `help`, `doctor`, `parse`, `inspect`, `schema`, `check`, `exec`, `test`, `profile analyze`, `server`, `db`, `lsp`, and `compat` exist. Several commands are still partial because their underlying runtime fidelity is partial. |
| Project config | partial | Minimal `oaer.yml` discovery exists. |
| Diagnostics | partial | Shared diagnostic shape exists. |
| Compatibility fixtures | partial | JSON schema model exists; `oaer compat validate/run` executes parse and exec fixtures; `oaer compat matrix` and `oaer compat mvp` expose machine-readable capability status. Broader Salesforce black-box fixtures remain incomplete. |
| Parser | partial | `oaer parse` handles both example projects, including Apex methods named `void`; declaration walking and source utilities exist. |
| Project loader | partial | SFDX package directories and Apex/object/field files are discovered. |
| Schema loader | partial | Custom object and custom field metadata are loaded. |
| Symbol table | partial | Top-level Apex declarations, members, test annotations, triggers, duplicate names, and schema objects are indexed. |
| Semantic analysis | partial | `oaer check` validates declaration/member type references, trigger SObjects, schema lookup references, and test discovery. Method-body type checking is not implemented yet. |
| VM | partial | `oaer exec` runs the supported anonymous Apex subset with variables, expressions, methods/classes, constructors, instance/static fields, inheritance/super dispatch, collections, for/enhanced-for/do-while/switch, try/catch/finally, exception messages, trace events, assertions, and common platform APIs. Full visibility, namespaces, initializer blocks, inner classes, and overload fidelity are not complete. |
| Test runner | partial | `oaer test` discovers `@isTest` and legacy test methods, compiles project helper classes/triggers, runs constructor and instance method bodies, executes `@TestSetup`, resets statics, clones org state per test, supports `startTest`/`stopTest`, `runAs`, Queueable drain, assertion stack frames, and emits console, JSON, and JUnit reports. Full Salesforce auth/profile and async breadth are not complete. |
| SObject/schema runtime | partial | Runtime SObject values preserve projected fields and explicit nulls; schema describe registry and deterministic key prefixes are available. Apex construction, typed/dynamic field access, DML Id propagation, and simple parent relationship projection are wired into the VM. Broader describe/system fields remain incomplete. |
| SOQL | partial | Static SOQL and `Database.query` execute in-memory `SELECT fields FROM Object` queries with binds, equality/inequality filters, `ORDER BY`, `LIMIT`, `OFFSET`, `COUNT()`, single-SObject assignment, and simple parent relationship fields. Subqueries, broad aggregates, complex predicates, SQLite planning, and full relationship query behavior are not complete. |
| DML/transactions/triggers | partial | `internal/dml` and the VM support Apex insert/update/delete/upsert/undelete syntax, `Database.*` allOrNone results, required/unknown field validation, deterministic IDs, rollback snapshots, and before/after trigger invocation with `Trigger.*` basics. Merge, external-id upsert, full undelete fidelity, `addError`, and complete bulk ordering remain incomplete. |
| Storage/fixtures/persistence | partial | SQLite-backed org storage persists object definitions, records, and ID sequences. `oaer db seed/reset/export/inspect` is wired to fixture JSON with alias and relationship-reference resolution, deterministic users/profiles/permissions, and server-backed fixture/reset endpoints. Large-fixture performance tuning and richer permission semantics remain incomplete. |
| DAP | partial | `internal/dap` handles DAP framing plus setBreakpoints, continue/pause/next, stackTrace, scopes, variables, and evaluate against VM snapshots. `oaer exec --debug` and `oaer test --debug` start DAP snapshot sessions. Live VM suspension, breakpoint-driven execution control, and step-in/out fidelity are not complete. |
| LSP | partial | `oaer lsp` runs a stdio LSP transport for initialize/shutdown, diagnostics payloads, symbols, hover, and top-level Apex/SObject completion from the project index. Definition, references, semantic tokens, and incremental document sync are not complete. |
| Watch mode | partial | `oaer test --watch` runs a polling watcher with debounce, JSON event stream, affected-test selection, reruns, and context cancellation. Native OS watcher backends and in-flight VM cancellation are not complete. |
| Native profile analysis | partial | `oaer profile analyze` reads native Chrome Trace Event output and emits ranked JSON or Markdown reports with statement/method/SOQL/DML categories and SOQL/DML row deltas. No external apexrr dependency is used. pprof-compatible CPU output and wall-clock attribution are not complete. |
| Local API server | partial | `oaer server` starts a Salesforce-shaped REST baseline with data version, SObject CRUD, normal REST JSON payload decoding, query/queryAll, describe/recent, limits, OAuth userinfo/id stubs, Tooling `executeAnonymous`, composite sObject insert, optional SQLite persistence through `--db`, OAER fixture/reset endpoints, and Salesforce-shaped error arrays. Full auth, Tooling object coverage, Composite Graph, Bulk API, and broader REST resources are not complete. |
