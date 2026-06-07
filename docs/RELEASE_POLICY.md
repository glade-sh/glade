# Release Policy

`glade` releases are tagged as `vMAJOR.MINOR.PATCH`. The release workflow builds
platform archives and `SHA256SUMS.txt` from the tagged source.

## Compatibility Gate

A release can be promoted as MVP-ready only when:

- `glade compat mvp --require-ready` exits successfully and reports
  `MVP readiness: ready`.
- Every `requiredForMVP` capability in `internal/capability` is `supported`.
- Any feature marked `supported` has compatibility fixtures.
- `docs/COMPATIBILITY_DASHBOARD.md` and `docs/KNOWN_GAPS.md` are regenerated
  from the same capability source and pass CI drift checks.

Until that gate is green, releases must be described as preview builds.

## Release Readiness Labels

Use these labels narrowly. They are claims about checked gates from the current
source tree, not broad promises that every Salesforce behavior is implemented.

- `server-examples-green`: `glade compat server-examples` reports no failing,
  unsupported, or missing probes for the checked server-example corpus.
- `mvp-ready`: `glade compat mvp --require-ready` passes, every MVP-required
  capability is `supported`, and generated compatibility docs are in sync.
- `apex-parity-ready`: the parser, semantic checker, VM, stdlib, storage,
  DML/SOQL, test runner, and local API compatibility fixtures that define the
  Apex parity surface pass from source.
- `legacy-project-test-ready`: `glade compat local-tests --check
  docs/fixtures/local-tests-corpus.json` passes for owned fixtures modeled
  after large legacy projects, and `glade compat post-parity --json` reports no
  test-blocking findings for the checked example-project inventory.
- `declarative-automation-test-ready`: the local-test corpus covers Workflow,
  record-triggered or Process Builder-shaped Flow, invocable Apex actions, DML
  rollback, and trace-visible declarative side effects.
- `visualforce-aura-lwc-controller-test-ready`: owned fixtures cover
  non-rendering Visualforce page/controller/action contracts, Aura Apex
  discovery, LWC Apex import discovery, and VM-level controller action
  dispatch. This label does not imply browser rendering or local UI serving.

## Salesforce API Versions

The current default local API version is `v65.0`. Compatibility is tracked by
the capability matrix, not by a blanket Salesforce API support claim.

When behavior differs by Salesforce API version, add the version to the
capability notes or split the capability into version-scoped entries before
marking it `supported`.

## Upgrade Policy

Patch releases should preserve CLI flags, fixture formats, persistent database
schemas, and documented supported behavior.

Minor releases may add capabilities, commands, fixture fields, or server
resources. They may change behavior for `partial`, `stub`, `unsupported`, or
`unknown` features when the new behavior is more Salesforce-compatible or fails
more explicitly.

Major releases may remove deprecated flags or change persistent formats, but
must document the migration path.

## Release Notes

Each release note should include:

- Compatibility status: output summary from `glade compat mvp`.
- Supported-platform artifacts and checksum verification instructions.
- New capabilities promoted to `supported`, including fixture coverage.
- Known gaps and unsupported-feature diagnostics that changed.
- Upgrade notes for CLI flags, fixture formats, database schemas, and server
  API behavior.

Use [`docs/RELEASE_NOTES.md`](RELEASE_NOTES.md) as the ongoing release log.

## Distribution Workflow

Use this workflow for an easy, repeatable distribution pass.

1. Prepare release branch state.
   - Run the required gates from current source.
   - Confirm docs reflect the current command names and setup steps.
   - Run the installed-binary dogfood checklist on at least one SFDX project.

2. Cut and push a tag.

```bash
git tag vX.Y.Z
git push <remote> vX.Y.Z
```

3. Let the `Release` workflow publish artifacts.
   - Artifacts are built to `dist/` with CGO enabled on macOS and Linux runners.
   - `glade doctor` must report `parser: ok` before an archive is written.
   - `SHA256SUMS.txt` is published with the release assets.

4. Verify install from release artifacts.

```bash
curl -L -o glade_vX.Y.Z_linux_amd64.tar.gz "<release-asset-url>"
curl -L -o SHA256SUMS.txt "<checksums-url>"
grep "  \./glade_vX.Y.Z_linux_amd64.tar.gz$" SHA256SUMS.txt | shasum -a 256 -c -
tar -xzf glade_vX.Y.Z_linux_amd64.tar.gz
./glade version
./glade doctor
```

5. Update distribution channels.
   - Update the Homebrew tap formula (`glade.rb`) URL and SHA256.
   - Validate `brew install` and `glade version`.

6. Publish release notes.
   - Copy compatibility status from `glade compat mvp`.
   - Call out new supported capabilities and remaining known gaps.

For an operator-oriented command checklist, use
[`docs/DISTRIBUTION_WORKFLOW.md`](DISTRIBUTION_WORKFLOW.md).
For a real-project dogfood pass, use
[`docs/DOGFOOD_CHECKLIST.md`](DOGFOOD_CHECKLIST.md).

## Benchmark Checks

Performance-sensitive changes should run the benchmark suite for the touched
areas. A broad one-shot smoke pass is:

```bash
go test -run '^$' -bench . ./internal/apexast ./internal/typesys ./internal/sema ./internal/soql ./internal/dml ./internal/vm ./internal/apextest ./internal/storage ./internal/server ./internal/lsp ./internal/watch
```

Use a fixed `-benchtime` and compare on the same machine when evaluating
regressions.
