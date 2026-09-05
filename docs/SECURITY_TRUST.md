# Security & Trust

Glade is a local binary for Salesforce project work. The trust story is a set
of checked facts, not one badge.

## What runs in CI

Report vulnerabilities through [Glade private reporting](https://github.com/glade-sh/glade/security/advisories/new)
or [Tools private reporting](https://github.com/glade-sh/glade-tools/security/advisories/new),
not public issues. If GitHub reporting is unavailable, use
[security@glade.sh](mailto:security@glade.sh).

- `govulncheck` checks reachable vulnerabilities in Go dependencies and the Go
  standard library.
- CodeQL runs the Go security-extended query suite.
- `gosec` uploads SARIF for source-pattern review.
- `npm audit --omit=dev --audit-level=high` checks packaged JavaScript
  production dependencies in the LWC toolchain and VS Code extension.
- GitHub Dependency Review blocks pull requests that add high-severity
  vulnerable dependencies.
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/glade-sh/glade)
  publishes repository-posture results.

## Release proof

The v0.2.14 per-archive SBOMs inventory 128–129 packaged components: 32–33 Go
modules and 96 LWC/Babel or VSIX npm dependencies. The macOS archives contain
33 Go modules; the Linux archives contain 32. The VSIX carries dependency
notices and an inventory bound to its extension bundle hash. An attestation
authenticates the inventory but does not establish completeness. A green gosec
upload likewise does not establish zero findings: the scanner uses `-no-fail`
during triage.

The archive also packages notice evidence for the Go distribution, the linked
Go modules named by the exact binary, and the vendored Apex parser. Those files
support review; they are not a legal-sufficiency opinion about system or CGO
libraries. The earlier v0.2.13 inventory remains Go-only.

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
- `.glade/` run artifacts and project state, including private local caches
  under `.glade/test/` and `.glade/semantic/`.
- SQLite files named by `--db`.
- User-level plugin, editor, and LWC toolchain data under the OS user data
  directory.

## Review packet for architects

Give reviewers:

- [Security policy](../SECURITY.md).
- [Install guide](INSTALL.md).
- [Release policy](RELEASE_POLICY.md).
- Latest GitHub release assets, checksums, SBOMs, and attestations.
- The Security workflow badge, CodeQL/gosec code scanning status, and published
  OpenSSF Scorecard results.
