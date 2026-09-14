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
| CodeQL | GitHub code scanning for Go with the security-extended query suite. Pull-request and full-branch runs apply checked, context-specific exclusions for queries that do not complete within the job limit. |
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

The v0.2.15 binaries embed commit `82b8495972715a27aec8f864bc4ab39e7fa03974`
with `vcs.modified=false`.

**Inventory scope:** the published v0.2.15 per-archive SBOMs inventory 128–129
packaged components: 32–33 Go modules and 96 LWC/Babel or VSIX npm
dependencies. The macOS archives contain 33 Go modules; the Linux archives
contain 32. The extension includes dependency notices and an inventory bound
to its bundle hash. Attestation proves the inventory's origin, not its
completeness.

The archive also packages notice evidence for the Go distribution, linked Go
modules named by the exact binary, and the vendored Apex parser. This supports
review but does not decide the sufficiency of notices for system or CGO
libraries. The earlier v0.2.13 inventory remains Go-only.

Tagged releases publish:

- Platform archives for macOS and Linux.
- `SHA256SUMS.txt`.
- `release-manifest.json`.
- A CycloneDX SBOM for each archive.
- Artifact attestations for the archive and its CycloneDX SBOM.

Tag publication is fail-closed. The tagged commit must already have an exact-SHA
successful `Required CI` push run. Each platform archive's provenance and
CycloneDX attestation must verify before any platform asset is uploaded.

Save this verification-only block as `verify-release-download.sh`, review it,
and run `sh verify-release-download.sh`. It stops at every failed gate, keeps
the downloaded files in a new temporary directory for inspection, and does not
extract, install, or execute Glade.

<!-- release-verifier:start -->
```sh
#!/bin/sh
# Verify one stable macOS/Linux release archive without extracting or running it.
# Dependencies: curl, jq, gh (authenticated when required), awk, mktemp,
# uname, and either shasum or sha256sum. Leaves downloads for inspection.
set -eu

fail() {
	printf 'Glade verification stopped: %s\n' "$*" >&2
	exit 1
}

for tool in curl jq gh awk mktemp uname; do
	command -v "$tool" >/dev/null 2>&1 || fail "required tool not found: $tool"
done

if command -v shasum >/dev/null 2>&1; then
	checksum_tool=shasum
elif command -v sha256sum >/dev/null 2>&1; then
	checksum_tool=sha256sum
else
	fail 'install shasum or sha256sum before continuing'
fi

case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) fail 'this helper supports the published macOS and Linux archives only' ;;
esac

case "$(uname -m)" in
	arm64 | aarch64) arch=arm64 ;;
	x86_64 | amd64) arch=amd64 ;;
	*) fail 'unsupported architecture' ;;
esac

workdir="$(mktemp -d "${TMPDIR:-/tmp}/glade-verify.XXXXXXXX")" || fail 'cannot create an isolated download directory'
printf 'Downloads retained in: %s\n' "$workdir"
cd "$workdir" || fail 'cannot enter download directory'

fetch() {
	curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
		--fail --show-error --silent --location --max-time 60 \
		--output "$2" "$1" || fail "download failed: $1"
}

fetch 'https://downloads.glade.sh/latest/release-manifest.json' manifest.json
version="$(jq -er '.version | select(type == "string") | select(test("^v[0-9]+\\.[0-9]+\\.[0-9]+$"))' manifest.json)" \
	|| fail 'manifest must contain a stable vMAJOR.MINOR.PATCH version'
archive="glade_${version}_${os}_${arch}.tar.gz"
base="https://downloads.glade.sh/${version}"

fetch "${base}/${archive}" "$archive"
fetch "${base}/SHA256SUMS.txt" SHA256SUMS.txt
awk -v wanted="./$archive" '
	$2 == wanted {
		n++
		if (NF != 2 || length($1) != 64 || $1 ~ /[^0-9a-fA-F]/) bad=1
		selected=$0
	}
	END { if (n != 1 || bad) exit 1; print selected }
' SHA256SUMS.txt >selected-checksum.txt \
	|| fail 'expected exactly one valid checksum entry for this archive'

if [ "$checksum_tool" = shasum ]; then
	shasum -a 256 -c selected-checksum.txt \
		|| fail 'checksum mismatch; do not extract or run the archive'
else
	sha256sum -c selected-checksum.txt \
		|| fail 'checksum mismatch; do not extract or run the archive'
fi

gh attestation verify "$archive" -R glade-sh/glade \
	--signer-workflow glade-sh/glade/.github/workflows/release.yml \
	--source-ref "refs/tags/${version}" \
	|| fail 'provenance verification failed; do not extract or run the archive'
gh attestation verify "$archive" -R glade-sh/glade \
	--signer-workflow glade-sh/glade/.github/workflows/release.yml \
	--source-ref "refs/tags/${version}" \
	--predicate-type https://cyclonedx.org/bom \
	|| fail 'CycloneDX attestation verification failed; do not extract or run the archive'

printf '\nDownload verification completed for %s.\nArchive: %s/%s\nNot extracted, installed, or executed.\n' \
	"$version" "$workdir" "$archive"
```
<!-- release-verifier:end -->

If verification stops, do not bypass the failed gate. Keep the exact command
and sanitized diagnostic. Verification alone does not approve an archive for
your organization's use; review its inventory and provenance under your own
policy.

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
