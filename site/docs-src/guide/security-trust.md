---
pageType: guide
canonicalTask: /guide/security-trust
---

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

## Before you start

You can report a vulnerability or inspect release provenance without creating a
project. Release verification needs a POSIX shell, `curl`, `jq`, `gh`, `awk`,
`mktemp`, `uname`, and either `shasum` or `sha256sum`.

<span id="report-a-vulnerability"></span>

## Report a vulnerability privately

Use [Glade private reporting](https://github.com/glade-sh/glade/security/advisories/new)
or [Tools private reporting](https://github.com/glade-sh/glade-tools/security/advisories/new).
Both repositories enable private reporting. If GitHub is unavailable, email
[security@glade.sh](mailto:security@glade.sh). Include the Glade version, OS and
architecture, exact command, and the smallest reproduction you can share.
Do not put vulnerability details, credentials, customer records, or private
source in public issues. See the [security policy](https://github.com/glade-sh/glade/blob/main/SECURITY.md)
for reporting details.

<span id="ci-gates"></span>

## Interpret CI evidence

| Gate | What it checks |
| --- | --- |
| [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/glade-sh/glade) | Published repository-posture results. |
| govulncheck | Reachable Go vulnerabilities in modules and the Go standard library. |
| CodeQL | Go security-extended scans with reviewed query exclusions. Pull requests run only the fast profile, aggregated by Required CI. Full scans run weekly and on release tags. Analysis is capped at 300 seconds (240 seconds for queries): a timeout warns; errors and unsuppressed alerts fail. |
| gosec | Go source-pattern findings uploaded as SARIF. |
| npm audit | High-severity production dependency findings in packaged JavaScript. |
| Dependency Review | Pull requests that add vulnerable dependencies. |

A completed scan upload is not an absence-of-findings result. An SBOM is a
dependency inventory; an attestation binds provenance, not runtime correctness.

`gosec` reports are uploaded while the existing baseline is triaged. New
high-severity findings should be fixed or documented before release.

The gosec upload uses `-no-fail`: green means the upload completed, not that no
findings exist. A repository-posture score or dependency inventory finding is
not proof of an exploitable runtime vulnerability. Review production scope,
reachability, and the exact candidate before accepting or dismissing a finding.

## Release proof

Open the [GitHub Release](https://github.com/glade-sh/glade/releases) for the
version you plan to use. Pick the platform archive that matches your OS and
CPU. For example, `darwin_arm64` is for Apple silicon and `linux_amd64` is for
64-bit x86 Linux. The audit archive contains release records. Install Glade
from the platform archive.

1. Match the platform archive's SHA-256 to its entry in `SHA256SUMS.txt`.
2. Open that archive's `.sbom.json` file to see the packaged components and
   their versions. Check the binary's `go version -m` output for its embedded
   source revision when reviewing an already trusted binary.
3. Verify the archive's build provenance and CycloneDX attestation against the
   release tag and Glade's release workflow. The build provenance identifies
   the source and workflow. The CycloneDX attestation verifies the origin of
   the inventory. Review the inventory itself for dependency questions.
4. Read `required-ci-attestation.json` and `salesforce-release-authority.json` to
   find the CI and Salesforce checks for that tagged commit.

### Release assets

- Platform archives for macOS and Linux, with per-archive `.sha256` files and
  `SHA256SUMS.txt`.
- `release-manifest.json`, platform manifests, and `index.json`.
- A CycloneDX software bill of materials (SBOM) for each platform archive.
- Build provenance and CycloneDX attestations in GitHub.
- `required-ci-attestation.json` and `salesforce-release-authority.json`.
- The release audit archive, `glade-release-artifacts-vX.Y.Z.tar.gz`.
- A per-archive `.sigstore.json` build provenance bundle where listed under
  Assets. v0.3.15 predates these bundles. The CycloneDX attestation can still be
  checked through GitHub.

The release workflow checks CI for the exact tagged commit and verifies each
platform archive's attestations before publishing it.

### Verify a download

The helper below checks the current stable download. Install the tools listed
above. If GitHub CLI needs to fetch attestations and asks you to sign in, run
`gh auth login` first. The script keeps the downloaded files for inspection and
stops at the first failed check. It leaves the archive untouched.

<<< ../../../scripts/verify-release-download.sh{sh}

### Historical example

The v0.2.15 binaries embed commit `82b8495972715a27aec8f864bc4ab39e7fa03974`
with `vcs.modified=false`.

**Inventory scope:** the published v0.2.15 per-archive SBOMs inventory 128–129
packaged components: 32–33 Go modules and 96 LWC/Babel or VSIX npm
dependencies. The macOS archives contain 33 Go modules; the Linux archives
contain 32. The extension includes dependency notices and an inventory bound
to its bundle hash.
The archive also includes notices for the Go distribution, linked Go modules,
and the vendored Apex parser.

## Laptop behavior

Glade runs on the developer machine. Supported local checks read Salesforce DX
project files, parse Apex, run supported tests, execute supported snippets, and
write optional local run artifacts.

No Salesforce org login is required for supported local checks.

## Network access

Glade uses the network when installing or updating release archives, installing
the local LWC toolchain, or installing plugins from a registry or archive URL.
Org-backed imports and package captures use Salesforce through explicitly
selected workflows. AI assistants have their own provider and source-sharing
settings. Local test isolation is not an OS sandbox.
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
