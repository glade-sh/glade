# UserProvisioning batchable lifecycle closure

## Scope

This bounded change covers the 68 evidence rows in
`integration/glade-tools/docs/fixtures/async-userprovisioning-batchable-lifecycle.json`.
The fixture's `Object` display types for several SObject arguments are
normalized to the existing API-67 standard symbols (`SObject`) by the local
compile-shape test and by the Salesforce probe source.

## Change

- Added explicit local construction for `CommittingBatchable`,
  `DeletingBatchable`, `RequestingBatchable`, and `UPASCleaningBatchable`.
  Their valid one-argument forms retain the supplied `uprId` or row list.
- Preserved the existing deterministic zero-argument local defaults used by
  the local UserProvisioning mock tests.
- Bound every concrete batchable surface to its Salesforce
  `Database.Batchable<T>` contract and marked `PluginBatchable` abstract with
  its four overridable hook methods virtual.
- Made platform dispatch walk concrete project subclasses back to their
  UserProvisioning batchable base, so inherited lifecycle methods execute in
  local projects.
- Added a standard-symbol shape test covering every fixture lifecycle method,
  constructor, and `UserProvisioningRequest` class shape.
- Added one consolidated VM test that executes all fixture lifecycle calls and
  five batch submissions in one run.

## Validation

All commands were run in this worktree:

```text
go test ./internal/typesys -run TestUserProvisioning -count=1                         PASS
go test ./internal/vm -run 'TestExecUserProvisioning|TestCompileUserProvisioning' -count=1  PASS
go test ./internal/apextest -run UserProvisioning -count=1                             PASS (no tests to run)
go build ./...                                                                         PASS
git diff --check                                                                       PASS
```

The consolidated test is
`internal/vm.TestExecUserProvisioningBatchableLifecycleFixture`.

The exact candidate replay also passed:

```text
product commit: e274242d762e3322791abf8d26e7f994dfafdde2
candidate: 79f52c936880b0bc7715a1f9be83befa899398d178248e047a55b9b64026c475
glade test --project evidence/current-base/messaging-extract-inbound-v10/userprovisioning-v26b/local --no-cache --json
status: passed; total: 1; passed: 1; compileErrors: 0; runtimeErrors: 0
```

## Salesforce evidence

The API-67 probe source and raw receipts are under:

`evidence/current-base/messaging-extract-inbound-v10/userprovisioning-v26b/`

Salesforce compilation succeeded with zero component errors. The direct
hosted probe still fails with `System.ListException: List index out of bounds: 0`
because the scratch org has no matching `UserProvisioningRequest` data. The
narrowed probe at
`evidence/current-base/messaging-extract-inbound-v10/userprovisioning-v26c/`
records the boundary explicitly: the constructors that query a persisted
request fail with the platform's normal `ListException`/`QueryException`,
while the list-based `RequestingBatchable` constructor passes. This is not
credited as a hosted runtime pass, and it is not a local implementation
mismatch.

The successor packet
`evidence/current-base/messaging-extract-inbound-v10/userprovisioning-v26b/`
closes all 68 fixture rows from the predecessor queue using the exact local
candidate plus Salesforce API-67 compilation and the explicit hosted data
boundary. Terra High review is required before promotion. Until then, the
current-base head remains Cache API-67 v25.
