# Indexed reads for persisted queries

## Current path

`glade db query` opens the database through `openDBStore`. `SQLiteStore.Load`
decodes every record before the command parses and executes SOQL. The `records`
table already has a primary key on `(object_name, id)`, but the SOQL executor
reads an in-memory `OrgState` and cannot use that key to avoid loading rows.
`Save` replaces all four persisted tables in one transaction. `SaveIfChanged`
compares a database snapshot and falls back to a full replacement for changes.

## Proposed boundary

Keep SOQL parsing, validation, evaluation, projections and JSON rendering in
the existing executor. Add a storage-backed read view which supplies candidate
records from SQLite only when a query plan proves that all data the executor
may observe is present. The first plan should handle a single equality on Id,
using the existing SQLite primary key, and otherwise fall back to the current
full load. Derive eligibility from the parsed query and object definition, not
raw query text or object names. Do not assume an Id string is a SQLite key:
prove the existing 15/18-character and case comparison semantics first.

The plan must preserve object resolution, field validation, deleted-row rules,
error order, aggregates and output ordering. Formulas, relationship fields,
subqueries, virtual objects, special read views and any other operation that
can observe additional records must fall back until their dependencies can be
loaded completely. A negative or missing lookup is also subject to the same
validation and error behavior. An incomplete view must never be passed to code
that can write it back.

`openDBStore` also validates project identity and may reconcile schema, Flow
metadata, defaults and sharing. These steps can inspect or write records. A
fast path must either establish from persisted and current project state that
no reconciliation is needed, or do a full load. In particular, a schema
refresh, missing binding, old metadata, project change or failed validation
must not be silently skipped. Existing load errors, including malformed and
orphan records, are observable: design a way to preserve them or explicitly
fall back where the same error cannot be guaranteed. Run validation and the
selected record read in one SQLite snapshot to avoid mixing concurrent commits.

## Migration and writes

The primary key requires no additional Id index or migration. If later plans
need a separate derived index, version and build it in a transaction; backfill
from the authoritative records, reject or rebuild incomplete indexes, and keep
updates and deletes atomic with records. Specify rollback, interrupted migration,
old and newer schema versions, read-only opens and multiple connections before
shipping it. Measure actual database size, including indexes, after migration.

A partial read alone does not make writes incremental. Introduce incremental
writes separately only after tracing every record mutation and deletion, schema
change, sequence update, reset and import. Preserve full-save semantics and
atomic failure behavior, including concurrent writers and stale snapshots.
Never save a partial view as a full org. Benchmark a one-record update to an
existing 100,000-record database against the same operation on the base, and
also measure bulk replacement and database size. Report transaction and setup
boundaries so results are comparable.

## Proof before enabling

* Capture byte-for-byte command output and errors on the base for successful,
  missing, deleted and differently represented Ids, and for invalid fields.
  Include object and field case, limits and `--query-all`.
* Compare the candidate path with the base on an uncaptured neighbor for every
  eligibility condition, including relationship and formula dependencies,
  aggregates, subqueries, special objects, project refresh and corrupted rows.
* Test insert, update, delete, rollback, reset, migration interruption, reopen,
  and concurrent read/write against the same snapshot and on separate stores.
* Measure the complete `glade db query` path, not only the SQLite lookup, with
  100,000 persisted rows. Record wall time, allocated bytes and peak RSS for
  matched outcomes. Alternate base and candidate runs; include single-row
  write, bulk write and on-disk size measurements.

Until these checks pass, use the existing full-load path. The performance
change needs the `testRuntimeCacheABI` bump class at release; this design note
alone changes no runtime cache.
