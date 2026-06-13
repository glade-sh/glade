# Deep Project Review - 2026-06-13

Scope: `/Users/matt/Dev/glade/.worktrees/deep-review-pair/glade` and
`/Users/matt/Dev/glade/.worktrees/deep-review-pair/glade-tools`.

Method: paired worktrees, two explorer agents, focused local inspection, narrow
tests first, then package verification. Full `go test ./...` passed at the
start in `glade` with `-p 2`. Full `glade-tools` testing showed one stale
fixture and a high memory compat package; the fixture was fixed and the broad
compat package later passed.

## Baseline Evidence

- `glade`: `go test ./... -p 2 -count=1` passed before edits.
- `glade-tools`: initial `go test ./... -p 2 -count=1` failed in
  `internal/compat` at
  `core-runtime-messaging-page-search-options`.
- The failing fixture expected zero `Search.suggest` results. Current Glade
  behavior and stdlib coverage both model one deterministic local suggestion.
- `go run ./cmd/glade render --help` exposed a hidden command not listed in
  help, completion, docs, or the product command inventory.
- `glade-tools` shell scripts tried to build `./cmd/glade`, which does not
  exist in the split tools repository.

## Completed Tasks

- [x] Fixed stale `Search.suggest` fixture expectations in
  `glade-tools/docs/fixtures/core-runtime-messaging-page-search-options.json`.
- [x] Removed the hidden product `glade render` command path and kept LWC
  rendering behind `internal/lwc` tests.
- [x] Added a regression test that `render` is not a public command.
- [x] Added `glade test` exact selector flags to shared help and shell
  completion: `--class`, `--method`, and `--class-file`.
- [x] Corrected the public support map count for ApexPages and PageReference.
- [x] Moved standard object regeneration docs and generated headers to the
  sibling `glade-tools` source path.
- [x] Replaced private stub paths in active docs with environment-driven source
  variables.
- [x] Fixed `glade-tools` scripts that built a missing `cmd/glade`; they now
  build `cmd/glade-tools`.
- [x] Made local-test perf scripts require an explicit project root instead of
  silently assuming a private example-project path.
- [x] Made the example-project baseline runner require
  `GLADE_BASELINE_PROJECTS`.
- [x] Redacted private corpus names from the checked local-test baseline.
- [x] Updated `glade-tools` top-level help and plugin manifest to list every
  dispatched command root.
- [x] Made `glade-tools` usage lead with the maintenance binary while still
  documenting the installed plugin form.
- [x] Updated Salesforce coverage next gates and surface packet commands to use
  `glade-tools surface`.
- [x] Removed the private Salesforce docs fallback path from active
  `salesforce-coverage` and `product-namespaces` commands. They now require
  `--source`, `--inventory`, `--catalog`, or `GLADE_SALESFORCE_DOCS_SOURCE`.
- [x] Regenerated `docs/generated/SALESFORCE_COVERAGE_MANIFEST.json` and
  `.md`.
- [x] Fixed `stub-inventory --check` drift hints so rerun commands include the
  required `--source`.
- [x] Neutralized the active stub inventory source label.
- [x] Split heavy `glade-tools/internal/compat` tests into a bounded default
  path and explicit full sweeps. The default package run now validates fixture
  JSON, executes a small documented-fixture smoke set, skips large local-test
  readiness fixtures, disables disk startup caches in unit tests, and admits
  one `RunLocalTests` call at a time.
- [x] Added opt-in full-sweep knobs:
  `GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1` and
  `GLADE_TOOLS_RUN_FULL_LOCAL_TEST_FIXTURES=1`.
- [x] Marked `glade-tools/docs/migrated` as a historical archive so old command
  recipes and private corpus references are not mistaken for current guidance.
- [x] Fixed the SObject overlay generator so `ExternalId` or other
  `Id`-suffixed string fields no longer become references from their names.
- [x] Regenerated `standard_sobject_stub_overlay_generated.go` from the real
  SObject stub source corpus. The final generated overlay removes 94 false
  field references, keeps explicit parent-relationship ID fields such as
  `RunningUserEntityAccessId` as references, and leaves relationship rows
  unchanged at 5,276.

## Remaining Tasks

No remaining tasks from this review.

## Verification Log

- `go test ./internal/gladecli -run '^(TestRenderIsNotPublicCommand|TestRunUnknownCommand|TestRunCompletionBash|TestRunCompletionFish|TestRunTopLevelHelpAlignment)$' -count=1`
- `go test ./internal/lwc -run 'TestRender' -count=1`
- `go test ./internal/gladecli -run '^(TestRunCompletionBash|TestRunCompletionFish|TestRunCommandHelp|TestRunTopLevelHelpAlignment)$' -count=1`
- `npm test -- --runInBand` in `glade/site`
- `go test ./internal/toolcli -count=1`
- `go test ./internal/surfaceledger -count=1`
- `go test ./internal/capability -run 'TestBuildSalesforceCoverageReport' -count=1`
- `go test ./internal/compat -run 'TestRunDocumentedFixtures/core-runtime-messaging-page-search-options' -count=1 -v`
- `go test ./internal/compat -run '^Test(DocumentedFixtureJSONLoadAndValidate|RunDocumentedFixtures)$' -count=1 -timeout=3m`
- `go test ./internal/compat -run '^TestRunLocalTestsNoDiskCacheDoesNotWriteStartupCache$' -count=1`
- `go test ./internal/compat -run '^TestLocalTestFixtureExecutionSelection$' -count=1`
- `GLADE_TOOLS_RUN_FULL_LOCAL_TEST_FIXTURES=1 go test ./internal/compat -run '^TestRunLocalTestsPlatformAPIsFixtureReady$' -count=1`
- `GLADE_TOOLS_RUN_FULL_LOCAL_TEST_FIXTURES=1 go test ./internal/compat -run 'TestRunLocalTests.*FixtureReady|TestCheckLocalTestCorpusFixture' -count=1 -timeout=10m`: 31.74s real, 4.14 GB maximum resident set size, and no `.glade/test/startup.gob` fixture caches recreated.
- `go test ./internal/compat -count=1 -timeout=3m`: 16.92s real, 2.02 GB maximum resident set size after optimization. Earlier default package run was 39.73s real and 6.74 GB RSS; the old documented fixture execution path alone measured 199.27s real and 9.34 GB RSS.
- `go test ./internal/gladecli ./internal/lwc ./internal/storage -count=1`
- `go test ./internal/toolcli ./internal/surfaceledger ./internal/capability -count=1`
- `go test ./internal/toolcli -run 'Test(SalesforceCoverage|ProductNamespaces)RequiresSourceInput' -count=1`
- `go test ./internal/toolcli ./internal/surfaceledger ./internal/capability -count=1`: 29.60s real, 1.75 GB maximum resident set size.
- `bash -n scripts/apex-docs-support-gate.sh scripts/build-glade-for-perf.sh scripts/local-test-perf.sh`
- `node --check scripts/baseline-local-tests-example-projects.mjs`
- `node --test scripts/generate-sobject-stub-overlay.test.mjs`
- `node --check scripts/generate-sobject-stub-overlay.mjs scripts/generate-sobject-stub-overlay.test.mjs`
- `node scripts/generate-sobject-stub-overlay.mjs <real-sobject-stub-source> ../glade/internal/storage/standard_sobject_stub_overlay_generated.go`
- Generated overlay review: `FieldReference` entries changed from 5,130 to
  5,036; `RelationshipName` entries changed from 6,325 to 6,026; relationship
  rows stayed at 5,276.
- `go test ./internal/storage -count=1`: 12.44s real, 2.60 GB maximum resident
  set size.
- JSON parse check for `docs/fixtures/local-tests-example-projects.json` and
  `docs/generated/SALESFORCE_COVERAGE_MANIFEST.json`
- `git diff --check` in both worktrees
- Active `glade-tools` sweep: no matches for old `cmd/glade compat`, private
  paths, private corpus roots, private stub source paths, or old
  `glade surface` recipes outside tests and `docs/migrated`.
