# Storage Schema Design

This document defines the first in-memory storage schema for the post-Phase-4
runtime. It is a contract for future SObject, SOQL, DML, trigger, test
isolation, and fixture work. It is not a database engine design and does not
attempt to implement Salesforce persistence behavior yet.

The matching Go model lives in `internal/storage`. That package should stay
small until the SObject, SOQL, and DML phases need executable behavior.

## Goals

- Represent a Salesforce-like org state in memory with deterministic,
  serializable data structures.
- Keep schema metadata, records, relationships, indexes, transaction frames,
  and fixture data in one coherent model.
- Support deterministic replay: the same schema and fixture input should
  produce the same record IDs and export ordering policy.
- Give later phases a stable place to attach SObject values, SOQL materialized
  results, DML validation, trigger context, and test rollback.

## Non-Goals

- No query planner, predicate evaluator, DML pipeline, trigger runtime, sharing
  engine, validation rule evaluator, or SQLite mapping in this slice.
- No claim of full Salesforce ID checksum compatibility. The first generator
  emits deterministic 15-character IDs and accepts 15- or 18-character IDs.
- No enforcement of field types, required fields, foreign keys, uniqueness, or
  external-ID semantics beyond carrying the metadata required to enforce them
  later.

## Core Model

`OrgState` is the storage root:

- `OrgID`, `APIVersion`, and `Namespace` identify the logical org.
- `Objects` maps object API name to `ObjectState`.
- `IDSequences` stores per-object deterministic ID counters.
- `Transactions` stores active transaction frames and savepoint-oriented
  mutation logs.

`ObjectState` contains:

- `Definition`: object metadata needed by runtime storage.
- `Records`: an ID-keyed map of current records for that object.
- `Indexes`: derived lookup maps. They are cache state, not source of truth.

The canonical key for an object is its API name, such as `Account` or
`Invoice__c`. The canonical key for a record is its Salesforce-like ID string.

## Object And Field Definitions

`ObjectDefinition` carries the storage-facing subset of object metadata:

- API name, labels, sharing model, namespace metadata, and key prefix.
- Field definitions keyed by field API name.
- Relationship definitions derived from lookup/master-detail metadata.
- Index definitions for ID, unique fields, external IDs, and future SOQL
  acceleration.

`Field` intentionally keeps Salesforce field types broad. A later schema pass
can map Metadata API types into this storage enum while retaining raw metadata
outside the storage core. Reference fields carry `ReferenceTo` as a list so
polymorphic fields such as `WhoId` and `WhatId` have a natural shape.

Required, unique, external ID, and case sensitivity flags are stored for future
DML validation. They are not enforced by the model package.

## Records And Values

`Record` stores:

- `ID`: the stable record identifier.
- `Object`: the object API name.
- `Fields`: explicitly assigned field values.
- `ExplicitNulls`: fields assigned to null, distinct from absent fields.
- `System`: system fields such as created/modified users and soft-delete state.

Field absence and explicit null must remain distinct because Apex SObjects,
SOQL projections, JSON fixtures, and DML updates all observe that difference.
For example, a queried SObject may not contain a field at all, while DML can
explicitly set a nullable field to `null`.

`Value` is a JSON-safe tagged union for storage fixtures and snapshots. Decimal,
date, and datetime values are strings so early storage does not inherit Go
floating-point or timezone behavior. Later VM integration can convert between
`storage.Value` and richer Apex heap values at the boundary.

## Relationships

Relationships are represented as metadata, not precomputed object graphs.

- Parent lookups are stored as ID values in reference fields.
- `Relationship.ParentRelationship` names the parent traversal used by SOQL,
  for example `Account`.
- `Relationship.ChildRelationship` names the child subquery collection, for
  example `Contacts`.
- Polymorphic relationships list all possible parent objects.
- Cascade/restricted delete flags are carried for DML, but not enforced here.

SOQL can materialize relationship SObjects and child lists from these metadata
definitions later. The record store should continue to keep the normalized
foreign-key field as the source of truth.

## Indexes

Indexes have two layers:

- `IndexDefinition`: desired index shape for ID, object/type, field, compound
  field, unique, and external-ID lookups.
- `IndexSet`: an in-memory derived map from encoded index key to record IDs.

The first required indexes for future phases are:

- primary ID lookup per object
- object record scan order
- unique field lookup
- external ID lookup
- reference field lookup for child relationship queries

Index sets may be marked dirty. Future SOQL/DML work can choose eager
maintenance for DML operations or lazy rebuild before query execution. The
model package only defines the shape.

## Transactions

Transactions are represented by a stack of `TransactionFrame` values. Each frame
contains:

- an ID/name/depth for transaction or savepoint identity
- ordered `Mutation` entries
- optional before/after record snapshots

This supports the first rollback design without committing to a storage engine:

1. DML opens or reuses the active Apex transaction frame.
2. Each insert, update, delete, undelete, or merge appends a mutation.
3. Savepoints record the current frame depth and mutation index.
4. Rollback restores `Before` snapshots or removes inserted `After` records.
5. Test isolation discards the whole org clone or rolls back to the initial
   frame.

The model includes clone helpers for records, index sets, object state, and org
state so later transaction code can take defensive snapshots.

## Fixture Import And Export

The storage fixture envelope uses `version: "oaer.storage.v1"` and contains:

- `org`: org ID, API version, and namespace.
- `objects`: named object blocks with records.
- `idSequences`: optional deterministic counters for continued ID generation.

Each fixture record may include:

- `id`: explicit ID. If absent, import should allocate one deterministically.
- `alias`: human-readable fixture reference for later relationship resolution.
- `fields`: tagged `Value` entries keyed by field API name.
- `explicitNulls`: field names intentionally set to null.

Import should happen in two passes once implemented:

1. Allocate or validate IDs and collect aliases.
2. Resolve relationship field aliases into IDs and load records.

Export should sort object names, record IDs, field names, and explicit-null
lists to keep compatibility fixtures stable in source control.

## Deterministic IDs

The first ID rule is:

```text
<3-character key prefix><12-character uppercase base36 sequence>
```

Examples:

- `Account` with prefix `001`, sequence `1`: `001000000000001`
- `Custom__c` with prefix `a00`, sequence `1`: `a00000000000001`

Standard object prefixes are seeded for common objects: `Account`, `Contact`,
`User`, `Opportunity`, `RecordType`, and `Profile`. Custom object prefixes are
assigned deterministically from sorted object API names unless metadata supplies
an explicit prefix. The initial custom prefix range starts at `a00`.

The generator is intentionally deterministic, not globally unique. It is scoped
to one fixture/org state and exists to make tests repeatable.

## Future Integration Points

- SObject runtime values should convert to and from `Record` while preserving
  projected-field absence and explicit-null semantics.
- SOQL should read `ObjectDefinition`, `Relationship`, and `IndexDefinition`
  before deciding whether to scan records or use `IndexSet`.
- DML should own field validation, index maintenance, trigger invocation, and
  transaction rollback using `Mutation`.
- Test runner isolation should clone or reset `OrgState` around each test
  method.
- A future SQLite backend can use these types as the logical schema while
  storing records as normalized rows or JSON columns.

## Open Questions

- Whether to compute Salesforce-compatible 18-character checksum IDs or keep
  18-character handling validation-only until compatibility tests require it.
- How much standard-object schema should ship as seed metadata versus fixtures.
- Whether indexes should be maintained eagerly by DML or lazily by SOQL.
- How fixture aliases should represent polymorphic relationships cleanly.
