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
product commit: d91c35316941d96aa595996fc5bbeb6c6c356b5e
candidate: 79f52c936880b0bc7715a1f9be83befa899398d178248e047a55b9b64026c475
glade test --project evidence/current-base/messaging-extract-inbound-v10/userprovisioning-v26b/local --no-cache --json
status: passed; total: 1; passed: 1; compileErrors: 0; runtimeErrors: 0
```

## Salesforce evidence

The API-67 probe source and raw receipt are under:

`evidence/current-base/messaging-extract-inbound-v10/userprovisioning-v26b/`

Salesforce compilation succeeded with zero component errors. The hosted test
still fails with `System.ListException: List index out of bounds: 0` in the
single-line probe class. This is retained as an unresolved hosted-runtime
probe issue, not credited as a Salesforce runtime pass. The exact candidate
has local compile/runtime parity; the 68 rows remain pending a narrowed
Salesforce runtime probe or an explicit environment/fixture waiver. No
Salesforce runtime claim is made by this receipt.
