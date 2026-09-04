# Security & trust

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Security</p>
  <p>Glade is a local binary. The proof trail is checks, checksums, SBOMs, and verified release provenance.</p>
  <ul>
    <li>Security scans run in GitHub Actions.</li>
    <li>Release archives publish checksums, SBOMs, and blocking attestations.</li>
    <li>Supported local checks do not require a Salesforce org login.</li>
  </ul>
</div>

Security review works best when the facts are close to the installer. Glade
keeps its security proof in the repository, the release workflow, and the
release assets.

## Report a vulnerability privately

Use [Glade private reporting](https://github.com/glade-sh/glade/security/advisories/new)
or [Tools private reporting](https://github.com/glade-sh/glade-tools/security/advisories/new).
Both repositories enable private reporting. If GitHub is unavailable, email
[security@glade.sh](mailto:security@glade.sh). Do not put vulnerability details
in public issues.

## Interpret CI evidence

| Gate | What it checks |
| --- | --- |
| [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/glade-sh/glade) | Published repository-posture results. |
| govulncheck | Reachable Go vulnerabilities in modules and the Go standard library. |
| CodeQL | GitHub code scanning for Go with the security-extended query suite. Pull-request and full-branch runs apply checked, context-specific exclusions for queries that do not complete within the job limit. |
| gosec | Go source-pattern findings uploaded as SARIF. |
| npm audit | High-severity production dependency findings in packaged JavaScript. |
| Dependency Review | Pull requests that add vulnerable dependencies. |

`gosec` reports are uploaded while the existing baseline is triaged. New
high-severity findings should be fixed or documented before release.

The gosec upload uses `-no-fail`: green means the upload completed, not that no
findings exist. A repository-posture score or dependency inventory finding is
not proof of an exploitable runtime vulnerability. Review production scope,
reachability, and the exact candidate before accepting or dismissing a finding.

## Release proof

**Inventory scope:** the published v0.2.13 SBOM covers the Go executable. It
does not inventory all bundled LWC/Babel JavaScript and VSIX dependencies.
Attestation proves the inventory's origin, not its completeness. Keep this
limitation in any supply-chain review of that release.

The unreleased archive builder also inventories packaged LWC/Babel modules
and dependencies present in the bundled VSIX. The extension includes dependency
notices and an inventory bound to its bundle hash. Exact archive and inventory
attestations still require the approved release workflow; local validation does
not amend the published v0.2.13 inventory.

The builder also packages notice evidence for the Go distribution, linked Go
modules named by the exact binary, and the vendored Apex parser. This supports
review but does not decide the sufficiency of notices for system or CGO libraries.

Tagged releases publish:

- Platform archives for macOS and Linux.
- `SHA256SUMS.txt`.
- `release-manifest.json`.
- A CycloneDX SBOM for each archive.
- Artifact attestations for the archive and its CycloneDX SBOM.

Tag publication is fail-closed. The tagged commit must already have an exact-SHA
successful `Required CI` push run. Each platform archive's provenance and
CycloneDX attestation must verify before any platform asset is uploaded.

Verify a downloaded archive:

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

Glade runs on the developer machine. Supported local checks read Salesforce DX
project files, parse Apex, run supported tests, execute supported snippets, and
write optional local run artifacts.

No Salesforce org login is required for supported local checks.

## Network access

Glade uses the network when installing or updating release archives, installing
the local LWC toolchain, or installing plugins from a registry or archive URL.
The local check, test, parse, exec, SOQL, DML, and local API paths do not send
project source to a hosted Glade service.

## Plugins

Plugins are executables. They run as the current OS user with a minimal environment.
Review a plugin's source before installing or linking it, pin plugin versions in CI
with a lock file, and treat `glade plugins link --exec <path>` as immediate trust in
that executable. The registry index is fetched over HTTPS but is not separately signed.

## Local storage

Glade can write project state under `.glade/`, including private local caches
under `.glade/test/` and `.glade/semantic/`, plus `glade.yml`, SQLite databases
named by `--db`, plugin files, editor integration files, and LWC toolchain data
under the OS user data directory.

## Review links

- [Security policy](https://github.com/glade-sh/glade/blob/main/SECURITY.md)
- [Security & Trust repo note](https://github.com/glade-sh/glade/blob/main/docs/SECURITY_TRUST.md)
- [Install guide](/guide/installation)
- [Release runbook](/maintainer/release)
