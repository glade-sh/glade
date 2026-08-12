# Database.UnitOfWork local contract closure

## Scope

This bounded change closes the 12 rows in
`data-platform-database-unitofwork-evidence.json`: the `Database.UnitOfWork`
type, its constructor, and its single-record and list-record DML methods,
`discardWork`, and `commitWork`.

The local runtime already implements this surface as a deterministic mock. The
packet adds a durable end-to-end runner test that executes every fixture call
through `RunCasesContext`; it does not claim that the type is publicly
available in hosted Salesforce.

## Contract

`Database.UnitOfWork` queues local insert, update, upsert, and delete requests.
Each method returns the corresponding placeholder result shape immediately.
`discardWork` clears queued operations. `commitWork` applies queued operations
and updates the returned result objects in place. The exact candidate sweep
covers all 12 fixture rows and verifies the result shapes and queue lifecycle.

## Changes

- `internal/apextest/runner_test.go` adds
  `TestRunCasesContextDatabaseUnitOfWorkFixture`, a full runner test covering
  the fixture's constructor and all ten methods plus both lifecycle methods.
- No VM behavior changed in this packet; the existing implementation and VM
  tests were already passing.

## Salesforce boundary

An API-67 deployment of a minimal public probe is intentionally expected to
fail with `Type is not visible: Database.UnitOfWork`. This is the Salesforce
correctness result: the type is classified as a local-only deterministic mock
and hosted support is explicitly unsupported. The raw deploy result and
stderr are retained in the current-base evidence packet. No public Salesforce
support claim is made.

## Validation

The packet owns the exact candidate replay, the local runner result, the
fixture rows, the API-67 negative deploy, and the materialized queue delta.
The parent orchestrator owns candidate provenance, SHA-256 binding, Terra
review, and promotion.
