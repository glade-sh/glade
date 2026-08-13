# Release Policy

`glade` releases are tagged as `vMAJOR.MINOR.PATCH`. The release workflow builds
platform archives and `SHA256SUMS.txt` from the tagged source.

## Release Gate

A release can be promoted when:

- The tagged commit has an exact-SHA successful `Required CI` authority.
- The tagged commit has exactly one successful `Salesforce Correctness` check
  from the trusted GitHub App and the check binds the exact `glade-tools` commit.
- `scripts/release-check.sh` passes.
- `glade doctor` reports `Ready.` for release archives.
- Every platform archive has verified provenance and CycloneDX attestations.
- Public support docs describe the current command surface and unsupported
  boundaries.

The local gate runs site verification, unit tests, and the production build
exactly once, then validates the checked Go package inventory through
memory-safe release lanes. Raw lane events and each validated
`package-summary.json` are written under `ci-artifacts/local-release/`. The
default is serial; do not raise `LOCAL_GO_TEST_JOBS` without host-specific
resource evidence.

## Release Gate Labels

Use these labels narrowly. They are claims about checked gates from the current
source tree, not broad promises that every Salesforce behavior is implemented.

- `release-ready`: product tests, smoke, install docs, and parser-capable
  release artifacts are current.
- `server-ready`: the local API server product smoke and focused server tests
  pass from source.
- `local-test-ready`: `glade test` product JSON/JUnit output and focused
  runtime tests pass from source.

## Salesforce API Versions

The current default local API version is `v65.0`. Support is tracked by checked
docs and tests, not by a blanket Salesforce API support claim.

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

- Supported-platform artifacts and checksum verification instructions.
- New supported behavior and the tests that cover it.
- Known gaps and unsupported-feature diagnostics that changed.
- Upgrade notes for CLI flags, database schemas, and server API behavior.

Use [`docs/RELEASE_NOTES.md`](RELEASE_NOTES.md) as the ongoing release log.

## Distribution Workflow

Use this workflow for an easy, repeatable distribution pass.

1. Prepare release branch state.
   - Run `scripts/release-check.sh`.
   - Confirm docs reflect the current command names and setup steps.
   - Run the first project check from `INSTALL.md` on at least one SFDX project.

2. Push the release commit and wait for its `Required CI` job to pass. Run the
   Salesforce correctness workflow in `glade-tools` for that exact Glade commit.
   Then cut and push one annotated tag at that commit. The tag message must
   contain exactly one lowercase full-SHA trailer for the tested Tools commit.

```bash
git tag -a vX.Y.Z -m $'Release vX.Y.Z\n\nGlade-Tools-SHA: <lowercase-40-hex-glade-tools-sha>'
git push <remote> vX.Y.Z
```

3. Let the `Release` workflow publish artifacts.
   - The tag gate records the exact successful `Required CI` run and job.
   - The tag gate verifies and records the exact cross-repository Salesforce
     authority before creating the GitHub Release.
   - Artifacts are built to `dist/` with CGO enabled on macOS and Linux runners.
   - `glade doctor` must report `Ready.` before an archive is written.
   - Provenance and CycloneDX attestations must verify before platform assets
     are uploaded.
   - `SHA256SUMS.txt`, `index.json`, and `latest/release-manifest.json` are
     published with the release assets.

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
   - Call out new supported behavior and remaining unsupported boundaries.

`Required CI` and `Salesforce Correctness` are automated exact-SHA release
authorities. Browser, Race, Security, and the local `scripts/release-check.sh`
remain required operator or policy checks; the release tag does not attest that
they ran. A manual Release workflow run requires `glade_tools_sha` and verifies
the same Salesforce authority, but it does not publish tag-only assets.

For a release command checklist, use
[`docs/DISTRIBUTION_WORKFLOW.md`](DISTRIBUTION_WORKFLOW.md).
For a real-project smoke pass, use the first project run in
[`docs/INSTALL.md`](INSTALL.md).

## Benchmark Checks

Performance-sensitive changes should run the benchmark suite for the touched
areas. A broad one-shot smoke pass is:

```bash
go test -run '^$' -bench . ./internal/apexast ./internal/typesys ./internal/sema ./internal/soql ./internal/dml ./internal/vm ./internal/apextest ./internal/storage ./internal/server ./internal/lsp ./internal/watch
```

Use a fixed `-benchtime` and compare on the same machine when evaluating
regressions.

To measure the complete local gate without changing its cache state, wrap it:

```bash
scripts/perf-release-check.sh \
  --label release-check-warm \
  --cache-mode warm \
  --output /tmp/glade-release-check-warm \
  -- scripts/release-check.sh
```

The wrapper records time, maximum RSS, file I/O, toolchain, commit, command,
and caller-declared cache mode in `release-check.json`. It does not clear,
prime, or replace caches, and it is not a release gate. The wrapped
`scripts/release-check.sh` result remains authoritative. Its validated Go lane
evidence remains under `ci-artifacts/local-release/`, including
`package-summary.json` for each lane.
