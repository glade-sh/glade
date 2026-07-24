# Security Policy

## Supported versions

Security fixes ship on the current release line. Install the latest release
unless your team has pinned a version for review.

Release archives are built by GitHub Actions from tagged source and published
with `SHA256SUMS.txt`, CycloneDX SBOM files, and GitHub artifact attestations.
The tagged commit must have an exact-SHA successful `Required CI` push run.
Archive provenance and CycloneDX attestations must verify before upload.

## Report a vulnerability

Report private security issues through GitHub Security Advisories for the
`glade-sh/glade` repository.

If advisories are unavailable, send the smallest reproduction you can share to
the maintainer through the project contact channel. Do not open a public issue
for a vulnerability until there is a coordinated disclosure path.

Include:

- The Glade version from `glade version`.
- The operating system and CPU architecture.
- The command that exposed the issue.
- Whether the issue requires a crafted Salesforce project, a local server route,
  a plugin archive, or a release installer path.
- Any output that does not include private customer source.

## Local laptop behavior

Glade does not require a Salesforce org login for supported local checks.

Typical local use reads an SFDX project from disk, writes optional `.glade`
state, runs local Apex parsing and execution, and can start localhost servers
for the playground, Visualforce preview, LWC preview, DAP, LSP, and the local
Salesforce-shaped API.

Glade does not send project source to a hosted Glade service.

Network access can happen when you:

- Install or update release archives.
- Install the local LWC toolchain.
- Install or update plugins from a registry or archive URL.
- Run commands that your own Apex or local workflow explicitly points at a
  network service.

Persistent local storage can include:

- `glade.yml` in the project.
- `.glade/` project state and run artifacts.
- SQLite files named by `--db`.
- User-level plugin, editor, and toolchain data under the OS user data
  directory.

## Security gates

The repository runs:

- `govulncheck` for reachable Go vulnerabilities.
- CodeQL with the security-extended query suite.
- gosec SARIF upload for Go source-pattern review.
- `npm audit --omit=dev --audit-level=high` for packaged JavaScript
  production dependencies.
- GitHub Dependency Review on pull requests.
- OpenSSF Scorecard with results ready for a public badge after repository
  publication.
- Release smoke checks, checksums, SBOM generation, and blocking artifact
  attestations.

`gosec` is uploaded to code scanning while the baseline is being triaged. Treat
new high-severity findings as release blockers unless they are documented false
positives.

## Verify a release archive

```bash
curl -L -o glade.tar.gz "$GLADE_RELEASE_URL"
curl -L -o SHA256SUMS.txt "$GLADE_CHECKSUMS_URL"
shasum -a 256 -c SHA256SUMS.txt
gh attestation verify glade.tar.gz -R glade-sh/glade
gh attestation verify glade.tar.gz -R glade-sh/glade \
  --predicate-type https://cyclonedx.org/bom
tar -xzf glade.tar.gz
./glade version
./glade doctor
```

Compare the matching `*.sbom.json` release asset with your internal dependency
allowlist when policy requires an inventory review.
