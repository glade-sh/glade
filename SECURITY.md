# Security Policy

## Supported versions

Security fixes ship on the current release line. Install the latest release
unless your team has pinned a version for review.

Release archives are built by GitHub Actions from tagged source and published
with `SHA256SUMS.txt`, CycloneDX SBOM files, and GitHub artifact attestations.
The tagged commit must have an exact-SHA successful `Required CI` push run.
Archive provenance and CycloneDX attestations must verify before upload.

## Report a vulnerability

Use [private vulnerability reporting](https://github.com/glade-sh/glade/security/advisories/new)
for Glade, or the [Tools private reporting route](https://github.com/glade-sh/glade-tools/security/advisories/new)
for first-party plugins. Private reporting is enabled for both repositories.

An alternate monitored contact is not yet published. If GitHub reporting is
unavailable, retain the report privately and retry the private route; do not
post vulnerability details in a public issue or discussion.

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
- OpenSSF Scorecard with published repository-posture results.
- Release smoke checks, checksums, SBOM generation, and blocking artifact
  attestations.

`gosec` is uploaded to code scanning while the baseline is being triaged. Treat
new high-severity findings as release blockers unless they are documented false
positives.

A green upload job does not mean zero source findings: gosec currently runs
with `-no-fail`. Dependency presence, reachability, and repository-posture
scores are different evidence. Review the underlying reports.

## Verify a release archive

```bash
GLADE_MANIFEST_URL=https://downloads.glade.sh/latest/release-manifest.json
GLADE_VERSION="$(curl -fsSL "$GLADE_MANIFEST_URL" | sed -nE 's/^[[:space:]]*"version": "(v[^"]+)",?$/\1/p')"
[ -n "$GLADE_VERSION" ] || { echo "could not resolve the stable Glade version" >&2; exit 1; }
case "$(uname -s)" in Darwin) GLADE_OS=darwin ;; Linux) GLADE_OS=linux ;; *) echo "unsupported operating system" >&2; exit 1 ;; esac
case "$(uname -m)" in arm64|aarch64) GLADE_ARCH=arm64 ;; x86_64|amd64) GLADE_ARCH=amd64 ;; *) echo "unsupported architecture" >&2; exit 1 ;; esac
GLADE_ARCHIVE="glade_${GLADE_VERSION}_${GLADE_OS}_${GLADE_ARCH}.tar.gz"
GLADE_BASE="https://downloads.glade.sh/${GLADE_VERSION}"
curl -fLO "${GLADE_BASE}/${GLADE_ARCHIVE}"
curl -fLO "${GLADE_BASE}/SHA256SUMS.txt"
GLADE_CHECKSUM_LINE="$(grep "  \./${GLADE_ARCHIVE}$" SHA256SUMS.txt)"
[ -n "$GLADE_CHECKSUM_LINE" ] || { echo "checksum entry not found" >&2; exit 1; }
if command -v shasum >/dev/null 2>&1; then printf '%s\n' "$GLADE_CHECKSUM_LINE" | shasum -a 256 -c -; else printf '%s\n' "$GLADE_CHECKSUM_LINE" | sha256sum -c -; fi
gh attestation verify "$GLADE_ARCHIVE" -R glade-sh/glade
gh attestation verify "$GLADE_ARCHIVE" -R glade-sh/glade \
  --predicate-type https://cyclonedx.org/bom
tar -xzf "$GLADE_ARCHIVE"
./glade version
./glade doctor
```

Compare the matching `*.sbom.json` release asset with your internal dependency
allowlist when policy requires an inventory review.

The v0.2.13 SBOM inventories the Go executable, not the entire archive's bundled
JavaScript/VSIX dependencies. Its attestation authenticates that inventory; it
does not make the inventory complete. Review packaged JavaScript separately
until a release explicitly supplies complete-archive inventory evidence.
