# Standard Library Unfenced Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. For the parallel lanes below, the coordinating agent should also use dispatching-parallel-agents and give each implementer a separate worktree.

**Goal:** Move every high-value standard-library row that is not intentionally fenced from `unsupported` to an honest `supported` or `partial` status, with runtime behavior, focused tests, generated docs, and a cleaner support map.

**Architecture:** Split the work into independent runtime families so GPT-5.5 medium subagents can work in parallel without stepping on one another. Each implementation lane owns product code and focused tests in `glade`; one integration lane owns the shared capability catalog and generated docs in `glade-tools` after the product lanes pass. Keep truly live-service rows fenced.

**Tech Stack:** Go 1.26, Glade VM/runtime packages under `internal/vm`, storage model under `internal/storage`, first-party maintenance tooling in sibling `/Users/matt/Dev/glade-tools`, generated compatibility docs under `docs/`.

---

## Scope

Rows in scope are the `unsupported` standard-library rows that either already have local runtime behavior or have a tractable local model with good local-test value.

Do not include these fenced rows in this plan:

- `Answers.findSimilar(Question)`
- `ResetPasswordResult.getPassword()`
- `TrailblazerIdentity.generateUserEmailVerificationToken(String,String,String)`
- `TrailblazerIdentity.getUserOrgInfo(List<String>)`
- `TrailblazerIdentity.splunkLog(String,String)`

These stay fenced because they depend on live Salesforce identity, Trailblazer, Answers, or password reset services. If an agent touches them, reject the patch unless the user explicitly changes scope.

## Parallel Work Rules

Use one worktree per implementation lane:

```bash
git worktree add ../glade-stdlib-quickaction HEAD
git worktree add ../glade-stdlib-context HEAD
git worktree add ../glade-stdlib-testhooks HEAD
git worktree add ../glade-stdlib-async-search HEAD
git worktree add ../glade-stdlib-businesshours HEAD
git worktree add ../glade-stdlib-approval-access HEAD
```

Each subagent must:

- Use GPT-5.5 medium.
- Read only its task and the files named in that task.
- Write failing or strengthening tests first.
- Avoid generated docs and `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go` unless the task says it owns integration.
- Return a short report with changed files, exact rows moved, exact tests run, and remaining gaps.
- Not run broad example-project gates.

The coordinator merges lanes sequentially, resolves conflicts, then runs the integration phase.

## File Map

Product runtime files:

- `internal/vm/dispatch.go` - static platform method dispatch.
- `internal/vm/dispatch_static.go` - known static method allowlist.
- `internal/vm/request_runtime.go` - Request, UIRequest, package license, URL request helpers.
- `internal/vm/test_support_runtime.go` - Test helper runtime hooks.
- `internal/vm/async_job_runtime.go` - Queueable, Batchable, Scheduled Apex, async context.
- `internal/vm/async_platform_runtime.go` - EventBus publish and platform-event delivery.
- `internal/vm/platform_passive_members.go` - passive/member methods on platform DTOs.
- `internal/vm/control_flow.go` - `System.runAs` block handling.
- `internal/vm/construct_runtime.go` - constructors such as `AsyncOptions`.
- `internal/storage/fixture.go` - local org seed data when a model needs seeded records.
- `internal/storage/model.go` and `internal/storage/standard_fields.go` - storage helpers and standard object field adjustments.

Product test files:

- `internal/vm/platform_test.go` - focused VM tests for most stdlib rows.
- `internal/vm/data_test.go` - data, sharing, package license, permission-set behavior.
- `internal/apextest/runner_test.go` - end-to-end local Apex test runner behavior.
- `internal/storage/model_test.go` - storage standard-object behavior.

Maintenance/catalog files:

- `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go` - standard library row source.
- `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go` - status guard tests.
- `docs/STDLIB_COVERAGE.md` - generated standard-library coverage.
- `docs/COMPATIBILITY_DASHBOARD.md` - generated dashboard.
- `docs/KNOWN_GAPS.md` - generated known gaps.
- `site/docs-src/guide/support-map.md` - high-level support map.

## Phase 0: Baseline and Row Inventory

**Files:**

- Read: `docs/STDLIB_COVERAGE.md`
- Read: `site/docs-src/guide/support-map.md`
- Read: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`
- Read: `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`

- [ ] **Step 1: Record the current unsupported rows**

Run from `/Users/matt/Dev/glade`:

```bash
awk -F'|' 'NR>8 && /^\|/ {
  area=$2; api=$3; status=$4; notes=$5;
  gsub(/^[ \t]+|[ \t]+$/,"",area);
  gsub(/^[ \t]+|[ \t]+$/,"",api);
  gsub(/`/,"",api);
  gsub(/`/,"",status);
  gsub(/^[ \t]+|[ \t]+$/,"",status);
  gsub(/^[ \t]+|[ \t]+$/,"",notes);
  if (status=="unsupported") print area "\t" api "\t" notes
}' docs/STDLIB_COVERAGE.md > /tmp/glade-stdlib-unsupported-before.tsv
```

Expected: file contains 58 unsupported rows before this plan.

- [ ] **Step 2: Confirm generated doc baseline**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
```

Expected:

```text
../glade/docs/STDLIB_COVERAGE.md: up to date
```

- [ ] **Step 3: Confirm capability tests fail before any promotion**

Do not edit yet. Run from `/Users/matt/Dev/glade-tools`:

```bash
go test ./internal/capability
```

Expected before implementation: pass. This confirms the current policy still requires these rows to stay unsupported.

- [ ] **Step 4: Dispatch parallel implementation lanes**

Give each subagent one of the phase prompts below. Tell every subagent:

```text
Use GPT-5.5 medium. Work only in your assigned worktree. Write or strengthen focused tests first. Do not edit generated docs. Do not edit glade-tools capability files unless your task explicitly says Integration. Return changed files, exact rows moved, tests run, and blockers.
```

## Phase 1A: QuickAction and Send Email Defaults

**Subagent lane:** `../glade-stdlib-quickaction`

**Rows:**

- `QuickAction.describeAvailableActions`
- `QuickAction.describeAvailableQuickActions(String)`
- `QuickAction.describeQuickActions(List<String>)`
- `QuickAction.retrieveQuickActionTemplate(String,Id)`
- `QuickAction.retrieveQuickActionTemplates(List<String>,Id)`
- `QuickAction.performQuickAction`
- `QuickAction.performQuickAction(QuickAction.QuickActionRequest)`
- `QuickAction.performQuickAction(QuickAction.QuickActionRequest,Boolean)`
- `QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>)`
- `QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>,Boolean)`
- `Test.newSendEmailQuickActionDefaults(Id,Id)`

**Target status:** `partial` for QuickAction rows, because local metadata and DTO behavior work but live UI/action execution is not modeled. `supported` for `Test.newSendEmailQuickActionDefaults(Id,Id)` if the focused test proves deterministic local defaults.

**Files:**

- Modify: `internal/vm/dispatch_static.go`
- Modify: `internal/vm/dispatch.go`
- Modify: `internal/vm/request_runtime.go`
- Modify: `internal/vm/platform_test.go`

- [ ] **Step 1: Strengthen existing QuickAction tests**

Open `internal/vm/platform_test.go`. Extend `TestExecQuickActionDescribeAndTemplateDefaults` to include `describeAvailableActions` as an alias. Add this near the existing available-actions assertions:

```apex
List<QuickAction.DescribeAvailableQuickActionResult> aliasAvailable =
    QuickAction.describeAvailableActions('Account');
System.assertEquals(1, aliasAvailable.size());
System.assertEquals('Account.NewTask', aliasAvailable[0].getName());
```

- [ ] **Step 2: Run the focused test and observe failure**

Run:

```bash
go test ./internal/vm -run TestExecQuickActionDescribeAndTemplateDefaults
```

Expected before implementation: FAIL with unsupported call or unknown static method for `QuickAction.describeAvailableActions`.

- [ ] **Step 3: Add the alias to static dispatch**

In `internal/vm/dispatch_static.go`, add `QuickAction.describeAvailableActions` beside the other QuickAction static methods.

In `internal/vm/request_runtime.go`, update `unsupportedIntegrationSurface` so `QuickAction.describeAvailableActions` is allowed through with the other local QuickAction methods.

In `internal/vm/dispatch.go`, add this case beside `QuickAction.describeAvailableQuickActions`:

```go
case "QuickAction.describeAvailableActions":
	return vm.quickActionDescribeAvailable(args)
```

- [ ] **Step 4: Run the QuickAction focused tests**

Run:

```bash
go test ./internal/vm -run 'TestExecQuickAction(Describe|Perform)|TestExecTestNewSendEmailQuickActionDefaults'
```

Expected: PASS.

- [ ] **Step 5: Check no unsupported QuickAction assertion remains for promoted rows**

Run:

```bash
rg -n 'QuickAction\.(describeAvailableActions|describeAvailableQuickActions|describeQuickActions|retrieveQuickActionTemplate|retrieveQuickActionTemplates|performQuickAction|performQuickActions).*unsupported|local quick action UI surface' internal/vm
```

Expected: either no hits for promoted rows, or only comments in old tests that the subagent removes in the same patch. Do not remove unsupported handling for unrelated QuickAction families outside the listed rows.

## Phase 1B: Request, UIRequest, and UserInfo Package License

**Subagent lane:** `../glade-stdlib-context`

**Rows:**

- `Request.getCurrent()`
- `RequestImpl.getCurrent()`
- `Request.getRequestId()`
- `Request.getQuiddity()`
- `UIRequest.getCurrent()`
- `UIRequest.getRequestHeader(String)`
- `UserInfo.hasPackageLicense(Id)`
- `UserInfo.isCurrentUserLicensedForPackage(Id)`

**Target status:** `supported`, with notes that behavior uses the deterministic local request and org package-license assignments.

**Files:**

- Modify: `internal/vm/request_runtime.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/data_test.go` if package license proof needs more data coverage.

- [ ] **Step 1: Keep or add focused Request/UIRequest tests**

In `internal/vm/platform_test.go`, ensure there is a test equivalent to:

```apex
Request request = Request.getCurrent();
System.assertEquals('glade-request-000000000001', request.getRequestId());
System.assertEquals('RUNTEST_SYNC', request.getQuiddity().name());

Request impl = RequestImpl.getCurrent();
System.assertEquals('glade-request-000000000001', impl.getRequestId());

UIRequest uiRequest = UIRequest.getCurrent();
System.assertEquals('local.glade.example', uiRequest.getRequestHeader('host'));
System.assertEquals('local.glade.example', uiRequest.getRequestHeader('Host'));
System.assertEquals(null, uiRequest.getRequestHeader('x-missing'));
```

If the exact test already exists, do not duplicate it. Add only missing assertions.

- [ ] **Step 2: Keep or add package license test coverage**

In `internal/vm/platform_test.go`, ensure `TestExecUserInfoPackageLicenseUsesOrgAssignments` proves all three calls:

```apex
System.assertEquals(true, UserInfo.hasPackageLicense('050000000000001'));
System.assertEquals(true, UserInfo.isCurrentUserLicensed('pkg'));
System.assertEquals(true, UserInfo.isCurrentUserLicensedForPackage('050000000000001'));
System.assertEquals(false, UserInfo.hasPackageLicense('050000000000002'));
System.assertEquals(false, UserInfo.isCurrentUserLicensed('missing'));
```

This test must seed `PackageLicense` and `UserPackageLicense` in the local org. Use the existing test helper pattern in that file.

- [ ] **Step 3: Run focused tests**

Run:

```bash
go test ./internal/vm -run 'TestExec(RequestGetCurrentBasics|UserInfoPackageLicenseUsesOrgAssignments|UIRequest)'
```

Expected: PASS. If the test list misses the UI request test name, run:

```bash
go test ./internal/vm -run TestExec.*Request
```

Expected: PASS.

- [ ] **Step 4: Leave runtime behavior deterministic**

Do not add real HTTP request state. The local model should stay:

- request ID: `glade-request-000000000001`
- test quiddity: `RUNTEST_SYNC`
- default UI host: derived from `vm.salesforceBaseURL()`
- package license truth: local `PackageLicense` and `UserPackageLicense` records only

## Phase 1C: Test Harness Service Rows

**Subagent lane:** `../glade-stdlib-testhooks`

**Rows:**

- `Test.getEventBus()`
- `Test.enableChangeDataCapture()`
- `Test.getExternalService()`
- `Test.setContinuationResponse(String,HttpResponse)`
- `Test.invokeContinuationMethod(Object,Continuation)`
- `Test.testInstall(InstallHandler,Version)`
- `Test.testInstall(InstallHandler,Version,Boolean)`
- `Test.testUninstall(UninstallHandler)`
- `Test.testNotificationActionHandler(Messaging.NotificationActionHandler,Messaging.ActionableNotification)`
- `Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String)`
- `Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String,Boolean)`
- `SandboxPostCopy.runApexClass(SandboxContext)`
- `SandboxContext.organizationId()`
- `SandboxContext.sandboxId()`
- `SandboxContext.sandboxName()`

**Target status:** `supported` for local test helper execution. `partial` for event bus and external service if the notes name the mock/test-bus boundary.

**Files:**

- Modify: `internal/vm/test_support_runtime.go`
- Modify: `internal/vm/dispatch.go`
- Modify: `internal/vm/async_platform_runtime.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/runtime_state.go` if `Test.enableChangeDataCapture()` needs an observable flag.

- [ ] **Step 1: Keep existing passing proof tests**

Confirm these tests exist and cover the named rows:

```bash
go test ./internal/vm -list 'TestExec(LocalMockHarnessSurfaces|TestInstall|TestUninstall|TestNotification|TestSandboxPostCopy|SafeGeneratedTestHelpers|Continuation)'
```

Expected: output includes:

- `TestExecLocalMockHarnessSurfaces`
- `TestExecTestInstallInvokesInstallHandler`
- `TestExecTestUninstallInvokesUninstallHandler`
- `TestExecTestNotificationActionHandlerInvokesExecuteAction`
- `TestExecTestSandboxPostCopyScriptInvokesRunApexClass`
- `TestExecSafeGeneratedTestHelpers`

- [ ] **Step 2: Add observable CDC behavior for `Test.enableChangeDataCapture()`**

If `Test.enableChangeDataCapture()` only returns `Null`, add an observable test-local flag:

In `internal/vm/runtime_state.go`, add to `TestContext`:

```go
ChangeDataCaptureEnabled bool
```

In `internal/vm/dispatch.go`, inside the `Test.enableChangeDataCapture` case:

```go
vm.testContext.ChangeDataCaptureEnabled = true
return Null, nil
```

In `internal/vm/platform_test.go`, extend `TestExecSafeGeneratedTestHelpers` or add a small test that calls `Test.enableChangeDataCapture()` before a platform-event delivery path and verifies no unsupported error.

- [ ] **Step 3: Run focused test-helper tests**

Run:

```bash
go test ./internal/vm -run 'TestExec(LocalMockHarnessSurfaces|TestInstall|TestUninstall|TestNotification|TestSandboxPostCopy|SafeGeneratedTestHelpers|Continuation)'
```

Expected: PASS.

- [ ] **Step 4: Keep non-test-context errors**

For all `Test.*` helpers in this lane, keep `vm.requireTestContext(...)`. Add or keep a focused negative test where one already exists. Do not make these helpers available outside local test context.

## Phase 1D: Scheduler and Schedulable Context

**Subagent lane:** `../glade-stdlib-async-search`

**Rows:**

- `System.schedule(String,String,Object)`
- `Schedulable.execute(SchedulableContext)`
- `SchedulableContext.getTriggerId()`

**Target status:** `supported` for local Scheduled Apex test execution.

**Files:**

- Modify: `internal/vm/async_job_runtime.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/apextest/runner_test.go` only if runner proof is missing.

- [ ] **Step 1: Verify focused VM proof**

Run:

```bash
go test ./internal/vm -run 'TestExecScheduledApex(ResolvesExecuteBySchedulableContext|CronJobDetailUsesScheduledApexType)'
```

Expected: PASS.

- [ ] **Step 2: Add `getTriggerId` assertion if missing**

In `internal/vm/platform_test.go`, ensure `TestExecScheduledApexResolvesExecuteBySchedulableContext` verifies the schedule ID passed to `execute`:

```apex
String scheduleId = System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
System.assertEquals(scheduleId, ScheduledWorker.triggerId);
```

The registered `execute(SchedulableContext context)` method should include:

```apex
ScheduledWorker.triggerId = context.getTriggerId();
```

- [ ] **Step 3: Run the local Apex runner async tests**

Run:

```bash
go test ./internal/apextest -run 'TestRunner.*(Schedule|Scheduled|Async|Queueable)'
```

Expected: PASS or `[no tests to run]`. If `[no tests to run]`, do not invent a broad runner test here; VM coverage is enough for this stdlib row.

## Phase 2A: PageReference Object Overload and Search Object Overloads

**Subagent lane:** reuse `../glade-stdlib-async-search` after Phase 1D or create `../glade-stdlib-search`

**Rows:**

- `Test.setCurrentPageReference(Object)`
- `Search.find(String,Object)`
- `Search.query(String,Object)`
- `Search.suggest(String,String,Object)`
- `Search.suggest(String,String,Object,Object)`

**Target status:** `partial`. Object overloads work only for locally recognized DTO shapes. Invalid generic objects should continue to receive stable errors.

**Files:**

- Modify: `internal/vm/dispatch.go`
- Modify: `internal/vm/soql_runtime.go`
- Modify: `internal/vm/platform_test.go`

- [ ] **Step 1: Add failing page-reference overload test**

In `internal/vm/platform_test.go`, extend the current page test with:

```apex
Object objectPage = new PageReference('/apex/ObjectOverload');
Test.setCurrentPageReference(objectPage);
System.assertEquals('/apex/ObjectOverload', System.currentPageReference().getUrl());
```

Run:

```bash
go test ./internal/vm -run TestExecApexPagesCurrentPage
```

Expected before implementation: FAIL with `Test.setCurrentPageReference expects PageReference`.

- [ ] **Step 2: Implement PageReference object normalization**

In `internal/vm/dispatch.go`, replace the strict type check for `Test.setCurrentPageReference` with a helper:

```go
case "Test.setCurrentPage", "Test.setCurrentPageReference":
	if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "PageReference") {
		return Null, fmt.Errorf("%s expects PageReference", callee)
	}
	vm.currentPage = cloneValue(args[0])
	return Null, nil
```

The `Object` overload in Apex still arrives as a `ValueObject` with type `PageReference`, so this implementation should pass without weakening the guard for non-page objects.

- [ ] **Step 3: Add failing Search object-overload tests**

In `internal/vm/platform_test.go`, add a test that proves the Object overloads accept typed local option objects:

```apex
Test.setFixedSearchResults(new Id[]{ '001000000000001' });
List<List<SObject>> queryRows = Search.query('FIND {acme} RETURNING Account(Id)', (Object) AccessLevel.SYSTEM_MODE);
System.assertEquals(1, queryRows.size());
List<Search.SearchResult> findRows = Search.find('acme', (Object) AccessLevel.SYSTEM_MODE);
System.assertNotEquals(null, findRows);
Search.SuggestionResults suggestions = Search.suggest('ac', 'Account', (Object) new Search.SuggestionOption());
System.assertNotEquals(null, suggestions);
Search.SuggestionResults secureSuggestions = Search.suggest('ac', 'Account', (Object) new Search.SuggestionOption(), (Object) AccessLevel.USER_MODE);
System.assertNotEquals(null, secureSuggestions);
```

- [ ] **Step 4: Implement recognized Object overload normalization**

In `internal/vm/soql_runtime.go`, normalize the Object argument only when it is a recognized `AccessLevel` or `Search.SuggestionOption` value. Keep arbitrary `Object` values rejected.

Expected behavior:

- `Search.query(String,Object)` accepts `AccessLevel` values and delegates to the existing AccessLevel path.
- `Search.find(String,Object)` accepts `AccessLevel` values and delegates to the existing AccessLevel path.
- `Search.suggest(String,String,Object)` accepts `Search.SuggestionOption`.
- `Search.suggest(String,String,Object,Object)` accepts `Search.SuggestionOption` plus `AccessLevel`.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/vm -run 'TestExec(Search|ApexPagesCurrentPage)'
```

Expected: PASS.

## Phase 2B: Queueable `System.enqueueJob(Object,Object)` and AsyncOptions

**Subagent lane:** `../glade-stdlib-async-search`

**Rows:**

- `System.enqueueJob(Object,Object)`

**Target status:** `partial` until the full `AsyncOptions` surface is modeled. The overload should be promoted only when `maximumQueueableStackDepth` is observable in local queueable chaining.

**Files:**

- Modify: `internal/vm/construct_runtime.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify: `internal/vm/async_job_runtime.go`
- Modify: `internal/vm/platform_test.go`

- [ ] **Step 1: Add failing AsyncOptions accessor test**

In `internal/vm/platform_test.go`, replace the unsupported assertion for `AsyncOptions.getMaximumQueueableStackDepth` with:

```apex
AsyncOptions opts = new AsyncOptions();
System.assertEquals(null, opts.getMaximumQueueableStackDepth());
opts.setMaximumQueueableStackDepth(2);
System.assertEquals(2, opts.getMaximumQueueableStackDepth());
String jobId = System.enqueueJob(new FirstQueue(), opts);
System.assert(jobId.startsWith('707'));
```

Keep `AsyncOptions.setMinimumQueueableDelayInMinutes(1)` unsupported unless this task explicitly implements delay semantics.

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/vm -run 'TestExecAsyncUnsupportedEdgesAreTyped|TestExecQueueable'
```

Expected before implementation: FAIL on the max-depth accessor.

- [ ] **Step 3: Implement max-depth accessors**

In `internal/vm/construct_runtime.go`, ensure `new AsyncOptions()` initializes:

```go
options.Fields["maximumQueueableStackDepth"] = Null
```

In `internal/vm/platform_passive_members.go`, handle:

```go
case "getMaximumQueueableStackDepth":
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("AsyncOptions.getMaximumQueueableStackDepth expects 0 arguments")
	}
	if value, ok := receiver.Fields["maximumQueueableStackDepth"]; ok {
		return value, receiver, false, true, nil
	}
	return Null, receiver, false, true, nil
case "setMaximumQueueableStackDepth":
	if len(args) != 1 || (args[0].Kind != ValueInt && args[0].Kind != ValueNull) {
		return Null, receiver, false, true, fmt.Errorf("AsyncOptions.setMaximumQueueableStackDepth expects Integer")
	}
	receiver.Fields["maximumQueueableStackDepth"] = args[0]
	return receiver, receiver, true, true, nil
```

Keep `setMinimumQueueableDelayInMinutes` unsupported.

- [ ] **Step 4: Run focused async tests**

Run:

```bash
go test ./internal/vm -run 'TestExec(AsyncUnsupportedEdgesAreTyped|Queueable|ScheduledApex)'
```

Expected: PASS.

## Phase 2C: AccessLevel Permission-Set Scope

**Subagent lane:** `../glade-stdlib-approval-access`

**Rows:**

- `AccessLevel.withPermissionSetId(String)`

**Target status:** `partial`. It should create a local AccessLevel value that carries a permission set ID. Enforcement can initially cover object and field permission checks that already read `PermissionSet` and `PermissionSetAssignment`.

**Files:**

- Modify: `internal/vm/dispatch.go`
- Modify: `internal/vm/permissions_runtime.go`
- Modify: `internal/vm/soql_runtime.go` if query enforcement accepts AccessLevel.
- Modify: `internal/vm/dml_runtime.go` if DML enforcement accepts AccessLevel.
- Modify: `internal/vm/data_test.go`
- Modify: `internal/vm/platform_test.go`

- [ ] **Step 1: Add failing construction test**

Replace `TestExecAccessLevelWithPermissionSetIdIsExplicitUnsupported` in `internal/vm/platform_test.go` with:

```apex
AccessLevel scoped = AccessLevel.withPermissionSetId('0PS000000000001');
System.assertEquals('USER_MODE', scoped.name());
```

If AccessLevel values expose no ID accessor, inspect the Go `Value` fields in test helpers rather than adding a new Apex method.

- [ ] **Step 2: Implement scoped AccessLevel value**

In `internal/vm/dispatch.go`, change:

```go
case "AccessLevel.withPermissionSetId":
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("AccessLevel.withPermissionSetId expects String")
	}
	return Null, unsupportedCallError("AccessLevel.withPermissionSetId permission-set-scoped user mode")
```

to return a local AccessLevel object:

```go
case "AccessLevel.withPermissionSetId":
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("AccessLevel.withPermissionSetId expects String")
	}
	value := accessLevelValue("USER_MODE")
	value.Fields["permissionSetId"] = args[0]
	return value, nil
```

Use the existing AccessLevel constructor/helper name in the file. If the helper is named differently, keep the existing helper and add only the `permissionSetId` field.

- [ ] **Step 3: Add enforcement proof**

In `internal/vm/data_test.go`, add a test that:

1. Creates a user.
2. Creates a permission set with one object/field permission.
3. Assigns that permission set to the user.
4. Uses `AccessLevel.withPermissionSetId(ps.Id)` in a SOQL or DML path that already accepts `AccessLevel`.
5. Verifies access differs from regular `USER_MODE` only where the local permission model supports it.

Keep the test narrow. Do not invent full Salesforce permission semantics.

- [ ] **Step 4: Run focused permission tests**

Run:

```bash
go test ./internal/vm -run 'TestExec(AccessLevel|PermissionSet|SecurityStripInaccessible)'
```

Expected: PASS.

## Phase 2D: Package-Version `System.runAs`

**Subagent lane:** `../glade-stdlib-approval-access`

**Rows:**

- `System.runAs(Object,Object)`
- `System.runAs(Package.Version)`

**Target status:** `partial`. The local model should support package-version test context without pretending to install or license packages.

**Files:**

- Modify: `internal/vm/compiler.go`
- Modify: `internal/vm/control_flow.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/vm/platform_test.go`

- [ ] **Step 1: Add failing package-version runAs tests**

In `internal/vm/platform_test.go`, add:

```apex
Package.Version v = new Package.Version(1, 2, 3);
System.runAs(v) {
    System.assertEquals('1.2.3', String.valueOf(v));
}
System.runAs(new User(Id = '005000000000999'), v) {
    System.assertEquals('005000000000999', UserInfo.getUserId());
}
```

Expected before implementation: parser/compiler or runtime rejects the overload.

- [ ] **Step 2: Extend runAs IR only as far as needed**

If `internal/vm/compiler.go` only parses one runAs expression, extend the runAs instruction to carry an optional second expression. If the IR already supports the expression shape, use the existing form.

The local behavior should be:

- `System.runAs(Package.Version)` enters a package-version context and does not change current user.
- `System.runAs(User, Package.Version)` changes current user and enters package-version context.
- The package version is visible only to local package-context checks added by this task or future tasks.

- [ ] **Step 3: Track package version in test context**

In `internal/vm/runtime_state.go`, add to `TestContext`:

```go
CurrentPackageVersion Value
PackageRunAsDepth     int
```

In `internal/vm/control_flow.go`, save and restore these fields around the runAs block.

- [ ] **Step 4: Run focused runAs tests**

Run:

```bash
go test ./internal/vm -run 'TestExec(UserInfo|RunAs|PackageVersion)'
```

Expected: PASS.

## Phase 3A: BusinessHours Local Calendar

**Subagent lane:** `../glade-stdlib-businesshours`

**Rows:**

- `BusinessHours.add(String, Datetime, Long)`
- `BusinessHours.addGmt(String, Datetime, Long)`
- `BusinessHours.diff(String, Datetime, Datetime)`
- `BusinessHours.isWithin(String, Datetime)`
- `BusinessHours.nextStartDate(String, Datetime)`

**Target status:** `partial`. Local record-backed business-hours math works for week schedules and org time zones. Holidays and entitlement-process edge behavior stay out of scope unless already loaded as local metadata.

**Files:**

- Create: `internal/vm/business_hours_runtime.go`
- Modify: `internal/vm/dispatch.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/storage/model_test.go` only if standard BusinessHours fields need storage fixes.

- [ ] **Step 1: Add failing BusinessHours test**

Replace or supplement `TestExecBusinessHoursMetadataBackedMethodsAreUnsupported` in `internal/vm/platform_test.go` with a record-backed test:

```apex
Datetime mondayNine = Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0);
System.assertEquals(true, BusinessHours.isWithin('01m000000000001AAA', mondayNine));
Datetime mondayTen = BusinessHours.add('01m000000000001AAA', mondayNine, 60 * 60 * 1000);
System.assertEquals(Datetime.newInstanceGmt(2026, 6, 15, 17, 0, 0), mondayTen);
System.assertEquals(60 * 60 * 1000, BusinessHours.diff('01m000000000001AAA', mondayNine, mondayTen));
Datetime saturday = Datetime.newInstanceGmt(2026, 6, 20, 16, 0, 0);
System.assertEquals(false, BusinessHours.isWithin('01m000000000001AAA', saturday));
System.assertEquals(Datetime.newInstanceGmt(2026, 6, 22, 16, 0, 0), BusinessHours.nextStartDate('01m000000000001AAA', saturday));
```

Seed the org with a `BusinessHours` record:

- `Id`: `01m000000000001AAA`
- `TimeZoneSidKey`: `America/Los_Angeles`
- `MondayStartTime`: `09:00:00.000Z`
- `MondayEndTime`: `17:00:00.000Z`
- same start/end for Tuesday-Friday
- no Saturday/Sunday hours

- [ ] **Step 2: Implement a small calendar parser**

Create `internal/vm/business_hours_runtime.go` with helpers:

```go
type businessHoursWindow struct {
	start time.Duration
	end   time.Duration
}

type businessHoursCalendar struct {
	id       string
	location *time.Location
	windows  map[time.Weekday]businessHoursWindow
}
```

Implement:

- lookup by ID in local `Org.Objects["BusinessHours"]`
- fallback to the default active `BusinessHours` record only when the argument is blank
- parse day fields named `SundayStartTime`, `SundayEndTime`, through `SaturdayEndTime`
- use `TimeZoneSidKey`, falling back to UTC if missing
- reject missing records with `UnsupportedFeature` or a typed runtime error that names the missing ID

- [ ] **Step 3: Implement methods**

In `internal/vm/dispatch.go`, replace the unsupported cases for BusinessHours:

```go
case "BusinessHours.add", "BusinessHours.addGmt":
	return vm.businessHoursAdd(callee, args)
case "BusinessHours.diff":
	return vm.businessHoursDiff(args)
case "BusinessHours.isWithin":
	return vm.businessHoursIsWithin(args)
case "BusinessHours.nextStartDate":
	return vm.businessHoursNextStartDate(args)
```

Use millisecond arithmetic. Keep leap seconds and Salesforce holiday metadata out of scope.

- [ ] **Step 4: Run focused BusinessHours tests**

Run:

```bash
go test ./internal/vm -run 'TestExecBusinessHours'
```

Expected: PASS.

## Phase 3B: Approval.process Local Model

**Subagent lane:** `../glade-stdlib-approval-access`

**Rows:**

- `Approval.process(Approval.ProcessRequest)`
- `Approval.process(Approval.ProcessRequest, Boolean)`

**Target status:** `partial`. Support deterministic submit/workitem request DTO handling and result shapes. Do not model Salesforce approval engine routing.

**Files:**

- Create: `internal/vm/approval_process_runtime.go`
- Modify: `internal/vm/dispatch.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/data_test.go` only if lock/unlock interaction needs storage proof.

- [ ] **Step 1: Add failing submit request test**

In `internal/vm/platform_test.go`, replace one unsupported process assertion with:

```apex
Account account = new Account(Name = 'Needs Approval');
insert account;
Approval.ProcessSubmitRequest request = new Approval.ProcessSubmitRequest();
request.setObjectId(account.Id);
request.setComments('local submit');
Approval.ProcessResult result = Approval.process(request);
System.assertEquals(true, result.isSuccess());
System.assertEquals(account.Id, result.getEntityId());
System.assertEquals(0, result.getErrors().size());
System.assertNotEquals(null, result.getInstanceId());
System.assertEquals(1, result.getNewWorkitemIds().size());
```

- [ ] **Step 2: Add failing allOrNone test**

Add:

```apex
Approval.ProcessSubmitRequest bad = new Approval.ProcessSubmitRequest();
Approval.ProcessResult badResult = Approval.process(bad, false);
System.assertEquals(false, badResult.isSuccess());
System.assertEquals(1, badResult.getErrors().size());
```

- [ ] **Step 3: Implement deterministic result DTOs**

Create `internal/vm/approval_process_runtime.go`.

Implement:

- `ProcessSubmitRequest` with `ObjectId` produces `Approval.ProcessResult`
- `isSuccess() == true`
- `entityId == request.ObjectId`
- deterministic local `instanceId`, for example prefix `04g`
- deterministic local work item ID, for example prefix `04i`
- `errors` empty on success
- missing object ID returns a failed result when `allOrNone == false`
- missing object ID raises a catchable error when `allOrNone == true` or omitted

Do not implement approval-step metadata or assignment routing.

- [ ] **Step 4: Keep unsupported where no local model exists**

If the request type is not a submit/workitem request the local model understands, return a stable unsupported diagnostic naming the request type. Do not fabricate broad approval behavior.

- [ ] **Step 5: Run focused approval tests**

Run:

```bash
go test ./internal/vm -run 'TestExecApproval'
```

Expected: PASS.

## Phase 4: Capability Catalog Integration

**Subagent lane:** coordinator only, after product lanes merge.

**Files:**

- Modify: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`
- Generate: `docs/STDLIB_COVERAGE.md`
- Generate: `docs/COMPATIBILITY_DASHBOARD.md`
- Generate: `docs/KNOWN_GAPS.md`

- [ ] **Step 1: Remove promoted rows from unsupported policy test**

In `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`, update `TestCoreServiceContextStdlibRowsAreExplicitUnsupported`.

Remove every row promoted by Phases 1-3 except the fenced rows:

```go
"ResetPasswordResult.getPassword()": true,
"TrailblazerIdentity.generateUserEmailVerificationToken(String,String,String)": true,
"TrailblazerIdentity.getUserOrgInfo(List<String>)": true,
"TrailblazerIdentity.splunkLog(String,String)": true,
```

Also leave `Answers.findSimilar(Question)` fenced if it is in a different guard.

- [ ] **Step 2: Add targeted status guard tests**

Add new tests in `capability_test.go`:

```go
func TestQuickActionStdlibRowsAreLocalPartial(t *testing.T) {
	watched := map[string]Status{
		"QuickAction.describeAvailableActions": StatusPartial,
		"QuickAction.describeAvailableQuickActions(String)": StatusPartial,
		"QuickAction.describeQuickActions(List<String>)": StatusPartial,
		"QuickAction.retrieveQuickActionTemplate(String,Id)": StatusPartial,
		"QuickAction.retrieveQuickActionTemplates(List<String>,Id)": StatusPartial,
		"QuickAction.performQuickAction": StatusPartial,
		"QuickAction.performQuickAction(QuickAction.QuickActionRequest)": StatusPartial,
		"QuickAction.performQuickAction(QuickAction.QuickActionRequest,Boolean)": StatusPartial,
		"QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>)": StatusPartial,
		"QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>,Boolean)": StatusPartial,
		"Test.newSendEmailQuickActionDefaults(Id,Id)": StatusSupported,
	}
	assertStdlibStatuses(t, watched)
}

func TestLocalContextStdlibRowsArePromoted(t *testing.T) {
	watched := map[string]Status{
		"Request.getCurrent()": StatusSupported,
		"RequestImpl.getCurrent()": StatusSupported,
		"Request.getRequestId()": StatusSupported,
		"Request.getQuiddity()": StatusSupported,
		"UIRequest.getCurrent()": StatusSupported,
		"UIRequest.getRequestHeader(String)": StatusSupported,
		"UserInfo.hasPackageLicense(Id)": StatusSupported,
		"UserInfo.isCurrentUserLicensedForPackage(Id)": StatusSupported,
	}
	assertStdlibStatuses(t, watched)
}
```

If `assertStdlibStatuses` does not exist, add this helper near the tests:

```go
func assertStdlibStatuses(t *testing.T, watched map[string]Status) {
	t.Helper()
	for _, entry := range StdlibMatrix() {
		want, ok := watched[entry.API]
		if !ok {
			continue
		}
		delete(watched, entry.API)
		if entry.Status != want {
			t.Fatalf("%s = %s, want %s: %s", entry.API, entry.Status, want, entry.Notes)
		}
		if entry.Notes == "" {
			t.Fatalf("%s needs local-model notes", entry.API)
		}
	}
	if len(watched) > 0 {
		t.Fatalf("missing stdlib rows: %#v", watched)
	}
}
```

- [ ] **Step 3: Update `stdlib.go` rows**

In `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`, update each promoted `StdlibEntry` status and notes.

Use these note patterns:

- QuickAction describe/template: `Local QuickAction metadata and deterministic DTO results; no live UI action service execution.`
- QuickAction perform: `Returns deterministic local QuickActionResult DTOs for supported request shapes; no live UI action service execution.`
- Request/UIRequest: `Returns deterministic local request context values for local Apex execution.`
- UserInfo package licenses: `Checks local PackageLicense and UserPackageLicense records for the current runAs/default user.`
- Scheduler: `Queues and drains local Scheduled Apex jobs during Test.stopTest with deterministic CronTrigger IDs.`
- Test harness helpers: `Invokes the local test harness implementation; no live Salesforce service is contacted.`
- BusinessHours: `Local BusinessHours record-backed week schedule math; holiday and service-calendar edge behavior is not modeled.`
- Approval.process: `Deterministic local approval result DTOs for submit/workitem request shapes; no live approval engine routing.`
- AccessLevel.withPermissionSetId: `Carries a local permission-set-scoped user-mode token for supported permission checks.`
- Search Object overloads: `Accepts recognized local AccessLevel or SuggestionOption object values; external search ranking is not modeled.`

- [ ] **Step 4: Run capability tests**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go test ./internal/capability
```

Expected: PASS.

- [ ] **Step 5: Regenerate docs**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go run ./cmd/glade-tools stdlib --output ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --output ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --output ../glade/docs/KNOWN_GAPS.md
```

Expected: each command exits 0 and writes the corresponding file.

- [ ] **Step 6: Check generated docs**

Run:

```bash
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --check ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check ../glade/docs/KNOWN_GAPS.md
```

Expected:

```text
../glade/docs/STDLIB_COVERAGE.md: up to date
../glade/docs/COMPATIBILITY_DASHBOARD.md: up to date
../glade/docs/KNOWN_GAPS.md: up to date
```

## Phase 5: Support Map and Final Validation

**Files:**

- Modify: `site/docs-src/guide/support-map.md`
- Verify: `docs/STDLIB_COVERAGE.md`
- Verify: generated docs

- [ ] **Step 1: Recompute support-map family counts**

Run from `/Users/matt/Dev/glade`:

```bash
awk -F'|' 'NR>8 && /^\|/ {
  area=$2; status=$4;
  gsub(/^[ \t]+|[ \t]+$/,"",area);
  gsub(/`/,"",status);
  gsub(/^[ \t]+|[ \t]+$/,"",status);
  areas[area]++; counts[area SUBSEP status]++; statuses[status]=1
}
END {
  for (area in areas) {
    printf "%s", area;
    for (status in statuses) {
      key=area SUBSEP status;
      if (counts[key]) printf "\t%s=%d", status, counts[key]
    }
    printf "\ttotal=%d\n", areas[area]
  }
}' docs/STDLIB_COVERAGE.md | sort > /tmp/glade-stdlib-family-counts-after.tsv
```

- [ ] **Step 2: Update support map wording**

In `site/docs-src/guide/support-map.md`:

- Rename `Method rows` to `Ledger rows`.
- Update row counts using `/tmp/glade-stdlib-family-counts-after.tsv`.
- Split `Service-only platform APIs` into:
  - `Local test harness and request context` for promoted request/test helper rows.
  - `Fenced live service APIs` for Answers, TrailblazerIdentity, ResetPasswordResult, and any still-fenced service rows.

Keep the status key unchanged.

- [ ] **Step 3: Run focused product tests**

Run from `/Users/matt/Dev/glade`:

```bash
go test ./internal/vm -run 'TestExec(QuickAction|TestNewSendEmail|Request|UserInfoPackageLicense|LocalMockHarness|TestInstall|TestUninstall|TestNotification|TestSandboxPostCopy|SafeGeneratedTestHelpers|Continuation|ScheduledApex|BusinessHours|Approval|AccessLevel|Search|ApexPagesCurrentPage|Async)'
go test ./internal/storage
go test ./internal/apextest -run 'TestRunner.*(Schedule|Scheduled|Async|Queueable)'
```

Expected: PASS. The `internal/apextest` command may report `[no tests to run]`; that is acceptable only if the VM tests above passed.

- [ ] **Step 4: Run maintenance checks**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go test ./internal/capability
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --check ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check ../glade/docs/KNOWN_GAPS.md
```

Expected: PASS and all generated files up to date.

- [ ] **Step 5: Run repo hygiene**

Run from `/Users/matt/Dev/glade`:

```bash
go test ./internal/repoguard
git diff --check
```

Expected: PASS and no whitespace errors.

- [ ] **Step 6: Final unsupported inventory**

Run:

```bash
awk -F'|' 'NR>8 && /^\|/ {
  area=$2; api=$3; status=$4; notes=$5;
  gsub(/^[ \t]+|[ \t]+$/,"",area);
  gsub(/^[ \t]+|[ \t]+$/,"",api);
  gsub(/`/,"",api);
  gsub(/`/,"",status);
  gsub(/^[ \t]+|[ \t]+$/,"",status);
  gsub(/^[ \t]+|[ \t]+$/,"",notes);
  if (status=="unsupported") print area "\t" api "\t" notes
}' docs/STDLIB_COVERAGE.md > /tmp/glade-stdlib-unsupported-after.tsv
diff -u /tmp/glade-stdlib-unsupported-before.tsv /tmp/glade-stdlib-unsupported-after.tsv || true
```

Expected: only intentionally fenced rows and any explicitly documented remaining live-service surfaces stay unsupported.

## Subagent Prompt Pack

Use these prompts as the exact dispatch packet for GPT-5.5 medium workers.

### Prompt: QuickAction Lane

```text
You are implementing Phase 1A of docs/superpowers/plans/2026-06-11-stdlib-unfenced-coverage.md in your own worktree.

Scope:
- QuickAction.describeAvailableActions
- QuickAction.describeAvailableQuickActions(String)
- QuickAction.describeQuickActions(List<String>)
- QuickAction.retrieveQuickActionTemplate(String,Id)
- QuickAction.retrieveQuickActionTemplates(List<String>,Id)
- QuickAction.performQuickAction overloads
- QuickAction.performQuickActions overloads
- Test.newSendEmailQuickActionDefaults(Id,Id)

Touch only:
- internal/vm/dispatch_static.go
- internal/vm/dispatch.go
- internal/vm/request_runtime.go
- internal/vm/platform_test.go

Do not edit generated docs or glade-tools. Write/strengthen tests first. Implement only local metadata/DTO behavior. Do not claim live UI action execution.

Return:
- rows proved
- files changed
- exact tests run
- any row that should remain partial
```

### Prompt: Context Lane

```text
You are implementing Phase 1B of docs/superpowers/plans/2026-06-11-stdlib-unfenced-coverage.md in your own worktree.

Scope:
- Request.getCurrent()
- RequestImpl.getCurrent()
- Request.getRequestId()
- Request.getQuiddity()
- UIRequest.getCurrent()
- UIRequest.getRequestHeader(String)
- UserInfo.hasPackageLicense(Id)
- UserInfo.isCurrentUserLicensedForPackage(Id)

Touch only:
- internal/vm/request_runtime.go
- internal/vm/platform_passive_members.go
- internal/vm/platform_test.go
- internal/vm/data_test.go if package-license proof needs it

Keep behavior deterministic and local. Do not add real HTTP request state.

Return rows proved, files changed, and exact tests run.
```

### Prompt: Test Harness Lane

```text
You are implementing Phase 1C of docs/superpowers/plans/2026-06-11-stdlib-unfenced-coverage.md in your own worktree.

Scope:
- Test.getEventBus()
- Test.enableChangeDataCapture()
- Test.getExternalService()
- Test.setContinuationResponse(String,HttpResponse)
- Test.invokeContinuationMethod(Object,Continuation)
- Test.testInstall overloads
- Test.testUninstall(UninstallHandler)
- Test.testNotificationActionHandler(...)
- Test.testSandboxPostCopyScript overloads
- SandboxPostCopy.runApexClass(SandboxContext)
- SandboxContext organization/sandbox accessors

Touch only:
- internal/vm/test_support_runtime.go
- internal/vm/dispatch.go
- internal/vm/async_platform_runtime.go
- internal/vm/platform_passive_members.go
- internal/vm/runtime_state.go
- internal/vm/platform_test.go

Keep every Test.* helper test-context-only. If Test.enableChangeDataCapture is currently a no-op, add an observable TestContext flag.

Return rows proved, files changed, and exact tests run.
```

### Prompt: Async/Search Lane

```text
You are implementing Phases 1D, 2A, and 2B of docs/superpowers/plans/2026-06-11-stdlib-unfenced-coverage.md in your own worktree.

Scope:
- System.schedule(String,String,Object)
- Schedulable.execute(SchedulableContext)
- SchedulableContext.getTriggerId()
- Test.setCurrentPageReference(Object)
- Search Object overloads
- System.enqueueJob(Object,Object) via AsyncOptions maximumQueueableStackDepth

Touch only:
- internal/vm/async_job_runtime.go
- internal/vm/platform_passive_members.go
- internal/vm/dispatch.go
- internal/vm/soql_runtime.go
- internal/vm/construct_runtime.go
- internal/vm/platform_test.go
- internal/apextest/runner_test.go only if focused runner proof is already present and narrow

Do not implement AsyncOptions delay semantics. Keep invalid generic Object overloads rejected.

Return rows proved, files changed, exact tests run, and rows that must remain partial.
```

### Prompt: BusinessHours Lane

```text
You are implementing Phase 3A of docs/superpowers/plans/2026-06-11-stdlib-unfenced-coverage.md in your own worktree.

Scope:
- BusinessHours.add
- BusinessHours.addGmt
- BusinessHours.diff
- BusinessHours.isWithin
- BusinessHours.nextStartDate

Touch only:
- internal/vm/business_hours_runtime.go
- internal/vm/dispatch.go
- internal/vm/platform_test.go
- internal/storage/model_test.go only if standard BusinessHours field setup blocks tests

Implement local record-backed weekly schedule math. Do not model Salesforce holidays or entitlement process behavior.

Return rows proved, files changed, exact tests run, and unsupported edge cases left explicit.
```

### Prompt: Approval/Access Lane

```text
You are implementing Phases 2C, 2D, and 3B of docs/superpowers/plans/2026-06-11-stdlib-unfenced-coverage.md in your own worktree.

Scope:
- AccessLevel.withPermissionSetId(String)
- System.runAs(Object,Object)
- System.runAs(Package.Version)
- Approval.process overloads

Touch only:
- internal/vm/dispatch.go
- internal/vm/permissions_runtime.go
- internal/vm/soql_runtime.go
- internal/vm/dml_runtime.go
- internal/vm/compiler.go
- internal/vm/control_flow.go
- internal/vm/runtime_state.go
- internal/vm/approval_process_runtime.go
- internal/vm/platform_test.go
- internal/vm/data_test.go

Keep package-version runAs partial. Do not pretend to install packages. Keep Approval.process deterministic and local; no live approval routing.

Return rows proved, files changed, exact tests run, and rows that should stay partial.
```

### Prompt: Catalog Integration Lane

```text
You are implementing Phase 4 and Phase 5 of docs/superpowers/plans/2026-06-11-stdlib-unfenced-coverage.md after all product lanes are merged.

Scope:
- update /Users/matt/Dev/glade-tools/internal/capability/stdlib.go
- update /Users/matt/Dev/glade-tools/internal/capability/capability_test.go
- regenerate docs/STDLIB_COVERAGE.md, docs/COMPATIBILITY_DASHBOARD.md, docs/KNOWN_GAPS.md
- update site/docs-src/guide/support-map.md counts and wording

Do not promote fenced rows:
- Answers.findSimilar(Question)
- ResetPasswordResult.getPassword()
- TrailblazerIdentity.generateUserEmailVerificationToken(String,String,String)
- TrailblazerIdentity.getUserOrgInfo(List<String>)
- TrailblazerIdentity.splunkLog(String,String)

Run the exact check commands in Phase 4 and Phase 5. Return before/after unsupported counts, rows still unsupported, and exact test/check output.
```

## Done Criteria

This plan is complete when:

- Product tests for every promoted row pass.
- `go test ./internal/capability` passes in `/Users/matt/Dev/glade-tools`.
- `docs/STDLIB_COVERAGE.md`, `docs/COMPATIBILITY_DASHBOARD.md`, and `docs/KNOWN_GAPS.md` are regenerated and check-clean.
- `site/docs-src/guide/support-map.md` describes ledger rows, not method rows, and no longer makes already-local rows look like live-service holes.
- The final unsupported inventory contains only fenced live-service rows or rows explicitly kept partial/unsupported with a written reason.
- `go test ./internal/repoguard` and `git diff --check` pass.
