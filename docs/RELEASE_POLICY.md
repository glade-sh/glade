# Release Policy

`glade` releases are tagged as `vMAJOR.MINOR.PATCH`. The release workflow builds
platform archives and `SHA256SUMS.txt` from the tagged source.

## Release Gate

A release can be promoted when:

- The tagged commit has an exact-SHA successful `Required CI` authority from a
  push to `main`; PR and manual CI runs do not qualify.
- The tagged commit has exactly one successful `Salesforce Correctness` check
  from the trusted GitHub App and the check binds the exact `glade-tools` commit.
- `scripts/release-check.sh` passes.
- Release archives pass `glade doctor --json` with `"parserOK": true` and an
  extracted-binary parser smoke. The build requires a clean Git worktree.
- Every platform archive has verified provenance and CycloneDX attestations.
- Public support docs describe the current command surface and unsupported
  boundaries.

The local gate runs site verification, unit tests, and the production build
exactly once, then validates the checked Go package inventory through
memory-safe release lanes. Raw lane events and each validated
`package-summary.json` are written under `ci-artifacts/local-release/`. The
default is serial; do not raise `LOCAL_GO_TEST_JOBS` without host-specific
resource evidence.

The local gate does not include the built-site, rendered-site browser, or preview
smoke checks. Run those as described in [site/README.md](../site/README.md);
they are also part of the CI site job. `Ready.` is a project-level doctor result,
checked separately after installation and `glade init`.

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

Glade uses moving correctness. The checked source window is exactly 65.0,
66.0, and 67.0; the checked endpoint window is 60.0, 65.0, 66.0, and 67.0.
The default for both axes remains 65.0. Well-formed positive whole historical
Apex source versions are preserved for compatibility but remain outside the
checked window and receive no parity or official-correctness credit. Malformed
and future source versions fail explicitly. Glade does not silently choose the
nearest version and does not promise exact behavior for every historical source
version.

Source version, HTTP endpoint version, org profile, and LWC bundle metadata are
independent. An endpoint version never changes source semantics. Each LWC
bundle must declare an exact supported `apiVersion`; module availability follows
that bundle value. Execute Anonymous also remains limited to the checked source
window.

Salesforce release support is generated from the checked Glade Tools release
contract. Each release snapshot carries a checked source receipt for the exact
Atlas tree, per-family hashes, normalized inventory, and current-only LWC
source/filter decision. A promotion must pass generated-file drift checks, the
full Glade Go suite with the real LWC compiler, exact case or product-test
bindings, all source/endpoint/org windows, complete surface and behavior
denominators, and complete release-note routing. Static documentation or a
classification label does not receive execution credit.

For a new Salesforce release, export the versioned documentation and release
notes with the existing Glade Tools exporters, add and classify the adjacent
delta, route every release-note document, regenerate the compact availability
tables, and extend the declared window only after the exact boundary tests pass.
Keep current-only LWC source provenance distinct from versioned Atlas exports.
The correctness gate also requires both candidate binaries to embed the clean
Git revision named by the gate and records their hashes in its provenance
artifact.

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

1. Prepare release changes.
   - Confirm docs reflect the current command names and setup steps.
   - Add the version's release notes before freezing the release commit.
   - Commit the changes and run `scripts/release-check.sh` from a clean checkout.
   - Run the first project check from `INSTALL.md` on at least one SFDX project.

2. Merge the release commit to `main` and wait for its main-push `Required CI`
   job to pass. Run the Salesforce correctness workflow in `glade-tools` for
   that exact Glade commit.
   Then cut and push one annotated tag at that commit. The tag message must
   contain exactly one lowercase full-SHA trailer for the tested Tools commit.
   Reuse `bash scripts/release-preflight.sh "$GLADE_SHA" "$TOOLS_SHA"` before
   tagging. Both SHAs must be frozen, full lowercase commit IDs. Do not create a
   trigger commit or move a pushed tag to repair missing authority.

```bash
git tag -a vX.Y.Z "$GLADE_SHA" -m 'Release vX.Y.Z' -m "Glade-Tools-SHA: $TOOLS_SHA"
git push <remote> vX.Y.Z
```

3. Let the `Release` workflow publish artifacts.
   - The tag gate records the exact successful `Required CI` run and job.
   - The tag gate verifies and records the exact cross-repository Salesforce
     authority before creating the GitHub Release.
   - Artifacts are built to `dist/` with CGO enabled on macOS and Linux runners.
   - `glade doctor --json` must report `"parserOK": true` before an archive is
     written and after extraction; an extracted-binary parse must also pass.
   - Provenance and CycloneDX attestations must verify before platform assets
     are uploaded.
   - `SHA256SUMS.txt`, `index.json`, and `latest/release-manifest.json` are
     published with the release assets.
   - The original Required CI and Salesforce approval JSON records are retained
     with the immutable release and bind the exact product/Tools pair.

4. Verify install from release artifacts.

```bash
curl -L -o glade_vX.Y.Z_linux_amd64.tar.gz "<release-asset-url>"
curl -L -o SHA256SUMS.txt "<checksums-url>"
grep "  \./glade_vX.Y.Z_linux_amd64.tar.gz$" SHA256SUMS.txt | shasum -a 256 -c -
tar -xzf glade_vX.Y.Z_linux_amd64.tar.gz
./glade version
./glade doctor --json
```

5. Update distribution channels.
   - Update the Homebrew tap formula (`glade.rb`) URL and SHA256.
   - Validate `brew install` and `glade version`.

6. Announce the release using the published notes.
   - Call out new supported behavior and remaining unsupported boundaries.
   - Announce distribution complete only after the static channel and site point
     to the version and default plus pinned installs report that version and a
     ready doctor result. A green GitHub Release workflow proves GitHub
     publication, not the later static-host or site publication.

`Required CI` and `Salesforce Correctness` are automated exact-SHA release
authorities. The separate LWC Browser, Race, Security, and local
`scripts/release-check.sh` remain required operator or policy checks; the release
tag does not attest that they ran. A manual Release workflow run on a branch
requires `glade_tools_sha` and verifies the same Salesforce authority, but it
does not publish tag-only assets.

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
