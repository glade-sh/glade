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

## Initial Matrix

| Area | Status | Notes |
| --- | --- | --- |
| CLI skeleton | partial | `version`, `help`, and `doctor` exist. |
| Project config | partial | Minimal `oaer.yml` discovery exists. |
| Diagnostics | partial | Shared diagnostic shape exists. |
| Compatibility fixtures | partial | JSON schema model exists and `oaer compat run` executes parse and exec fixtures. |
| Parser | partial | `oaer parse` handles both example projects, including Apex methods named `void`; declaration walking and source utilities exist. |
| Project loader | partial | SFDX package directories and Apex/object/field files are discovered. |
| Schema loader | partial | Custom object and custom field metadata are loaded. |
| Symbol table | partial | Top-level Apex declarations, members, test annotations, triggers, duplicate names, and schema objects are indexed. |
| Semantic analysis | partial | `oaer check` validates declaration/member type references, trigger SObjects, schema lookup references, and test discovery. Method-body type checking is not implemented yet. |
| VM | partial | `oaer exec` runs a small anonymous Apex subset with primitive expressions, variables, collections, expanded String/List/Set/Map methods, Chrome Trace Event instruction traces, `System.assert*`, and `System.debug`. |
| Test runner | partial | `oaer test` discovers `@isTest` and legacy test methods, runs methods whose bodies fit the anonymous VM subset, and emits console, JSON, and JUnit reports. Full class dispatch, setup data, and Salesforce test semantics are not implemented yet. |
| SObject/schema runtime | partial | Runtime SObject values preserve projected fields and explicit nulls; schema describe registry and deterministic key prefixes are available. Apex syntax integration is not complete. |
| SOQL | partial | `internal/soql` parses and executes simple in-memory `SELECT fields FROM Object` queries with equality/inequality filters, `ORDER BY`, `LIMIT`, and `OFFSET`. Relationship queries, binds, aggregates, SQLite planning, and dynamic Apex integration are not implemented yet. |
| DML/transactions/triggers | partial | `internal/dml` supports in-memory insert/update/delete, required/unknown field validation, deterministic IDs, partial result records, and rollback snapshots. Trigger invocation, upsert/undelete/merge, and `Database.*` Apex integration are not implemented yet. |
