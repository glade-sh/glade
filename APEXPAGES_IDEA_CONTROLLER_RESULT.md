# ApexPages Idea controller fixture closure

## Scope

This bounded change closes the 32 evidence rows in
`ui-apexpages-idea-controller-evidence.json`: nine
`ApexPages.IdeaStandardController` members and 23
`ApexPages.IdeaStandardSetController` members. The fixture is compile-shape
`check` evidence; this packet adds a durable end-to-end runner test that
executes every fixture call through the full local test runner.

## Current state at HEAD

The fixture compile check already passed at HEAD `886e010f`:

```text
glade-tools run docs/fixtures/ui-apexpages-idea-controller-evidence.json
<fixture>: check ok=true
```

The runtime dispatch for both Idea controller types was already present in
`internal/vm/platform_apexpages_formula.go`:

- `ApexPages.IdeaStandardController` delegates the inherited standard
  controller members (`addFields`, `cancel`, `delete`, `edit`, `getId`,
  `getRecord`, `save`, `view`) and adds `getCommentList`.
- `ApexPages.IdeaStandardSetController` delegates the inherited standard set
  controller members (paging, filtering, selection, records, actions) and adds
  `getIdeaList` and `getListViewOptions`.

No product runtime code changed in this packet because the surface was already
implemented and the existing VM value-contract tests already passed.

## Changes

- `internal/apextest/runner_test.go` — added
  `TestRunCasesContextApexPagesIdeaControllerFixture`, which runs the exact
  fixture source (both extension classes) plus a small test class through
  `RunCasesContext`. The test class constructs each extension with the local
  Idea controller instances so every fixture call executes with assertions.
  Direct construction is a local-runtime convenience; Salesforce supplies
  these controllers to extension constructors and does not allow direct
  construction.
- `APEXPAGES_IDEA_CONTROLLER_RESULT.md` — this report.

## Commands run

```text
glade-tools run ui-apexpages-idea-controller-evidence.json      check ok=true
go test ./internal/vm -run 'TestExecApexPagesIdeaStandard|TestExecIdeaStandardSetController' -count=1  PASS (3 tests)
go test ./internal/apextest -run 'TestRunCasesContextApexPagesIdeaControllerFixture' -count=1  PASS
go test ./internal/apextest -count=1                             PASS (309.759s)
go build ./...                                                   PASS
git diff --check                                                 PASS
```

Exact-candidate local replay (binary SHA-256
`79f52c936880b0bc7715a1f9be83befa899398d178248e047a55b9b64026c475`, product
commit `886e010f`):

```text
glade test --project apexpages-idea-controller-v27-local --no-cache --json
status: passed; total: 2; passed: 2; compileErrors: 0; runtimeErrors: 0
```

## Salesforce evidence note

The API-67 witness must use the extension-controller pattern from the fixture
(`IdeaViewExtension(ApexPages.IdeaStandardController controller)` and
`IdeaListExtension(ApexPages.IdeaStandardSetController controller)`), not
direct `new ApexPages.IdeaStandardController()` construction. The fixture
source and pages are the canonical Salesforce compile witness; the local
runner test mirrors those calls.

## Remaining

The parent orchestrator owns the candidate build, Salesforce API-67 deploy and
run receipts, dual-rail materialization, and Terra review. This packet makes no
final release or correctness approval claim.
