# Security & Trust

Glade is a local binary for Salesforce project work. The trust story is a set
of checked facts, not one badge.

## What runs in CI

- `govulncheck` checks reachable vulnerabilities in Go dependencies and the Go
  standard library.
- CodeQL runs the Go security-extended query suite.
- `gosec` uploads SARIF for source-pattern review.
- `npm audit --omit=dev --audit-level=high` checks packaged JavaScript
  production dependencies in the LWC toolchain and VS Code extension.
- GitHub Dependency Review blocks pull requests that add high-severity
  vulnerable dependencies.
- OpenSSF Scorecard records repository posture. Its public badge becomes useful
  after the repository is public.

## Release proof

Each release archive is built from tagged source by GitHub Actions. The release
workflow publishes:

- Platform archives for macOS and Linux.
- `SHA256SUMS.txt`.
- `release-manifest.json`.
- A CycloneDX SBOM for each platform archive.
- GitHub artifact attestations for the archive and its CycloneDX SBOM.

Tag publication is fail-closed. The tagged commit must already have an exact-SHA
successful `Required CI` authority. Each platform archive's provenance and
CycloneDX attestation must then verify before any platform asset is uploaded.

Verify a release archive:

```bash
curl -L -o glade.tar.gz "$GLADE_RELEASE_URL"
curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
shasum -a 256 -c SHA256SUMS.txt
gh attestation verify glade.tar.gz -R glade-sh/glade
gh attestation verify glade.tar.gz -R glade-sh/glade \
  --predicate-type https://cyclonedx.org/bom
tar -xzf glade.tar.gz
./glade doctor
```

## Laptop behavior

Supported local checks do not require a Salesforce org login. Glade reads a
Salesforce DX project from disk, writes optional local state, and runs the
local Apex runtime on the machine where the command starts.

## Network access

Glade uses the network for install, update, plugin registry/archive downloads,
and optional toolchain install paths. Local check, test, parse, exec, SOQL, DML,
and local API work do not call a hosted Glade service.

## Plugins

Plugins are local executables installed or linked by the user. Trust labels in
the registry are informational except where CI blocks community or unlisted
installs without an explicit approval flag. Review community and unlisted plugin
source before installing them.

`glade plugins link --exec <path>` trusts that executable immediately. Use it
only for plugin binaries you built or reviewed.

Plugin subprocesses run as the current OS user. Glade passes a minimal
environment to plugin commands: basic process variables such as `PATH`, `HOME`,
and temp directories, plus `GLADE_*` variables. It does not pass common ambient
Salesforce, GitHub, AWS, or other shell tokens by default.

The plugin registry index is a JSON document fetched over HTTPS. The index is
not separately signed. Archive integrity comes from the SHA256 listed in the
index or supplied on the command line.

## Local storage

Glade can write:

- `glade.yml` project config.
- `.glade/` run artifacts and project state.
- SQLite files named by `--db`.
- User-level plugin, editor, and LWC toolchain data under the OS user data
  directory.

## Review packet for architects

Give reviewers:

- [Security policy](../SECURITY.md).
- [Install guide](INSTALL.md).
- [Release policy](RELEASE_POLICY.md).
- Latest GitHub release assets, checksums, SBOMs, and attestations.
- The Security workflow badge, CodeQL/gosec code scanning status, and OpenSSF
  Scorecard results for public display after repository publication.
