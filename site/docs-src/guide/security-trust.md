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

<!-- release-verifier:start -->
```sh
#!/bin/sh
# Verify one stable macOS/Linux release archive without extracting or running it.
# Dependencies: curl, jq, gh, awk, mktemp, uname, and either shasum or
# sha256sum. Leaves downloads for inspection.
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

try_fetch() {
	url="$1" output="$2" deadline="$3"
	shift 3
	curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
		--fail --show-error --silent --location --connect-timeout 10 \
		--max-time "$deadline" "$@" --output "$output" "$url"
}

fetch() {
	try_fetch "$@" || fail "download failed: $1"
}

fetch 'https://downloads.glade.sh/latest/release-manifest.json' manifest.json 60
version="$(jq -er '.version | select(type == "string") | select(test("^v[0-9]+\\.[0-9]+\\.[0-9]+$"))' manifest.json)" \
	|| fail 'manifest must contain a stable vMAJOR.MINOR.PATCH version'
archive="glade_${version}_${os}_${arch}.tar.gz"
base="https://downloads.glade.sh/${version}"

fetch "${base}/${archive}" "$archive" 900 --retry 3
fetch "${base}/SHA256SUMS.txt" SHA256SUMS.txt 60
awk -v wanted="./$archive" '
	$2 == wanted {
		n++
		if (NF != 2 || length($1) != 64 || $1 ~ /[^0-9a-fA-F]/) bad=1
		digest=tolower($1)
	}
	END { if (n != 1 || bad) exit 1; printf "%s  %s\n", digest, wanted }
' SHA256SUMS.txt >selected-checksum.txt \
	|| fail 'expected exactly one valid checksum entry for this archive'

if [ "$checksum_tool" = shasum ]; then
	shasum -a 256 -c selected-checksum.txt \
		|| fail 'checksum mismatch; do not extract or run the archive'
else
	sha256sum -c selected-checksum.txt \
		|| fail 'checksum mismatch; do not extract or run the archive'
fi

# Use the public API for the separate CycloneDX attestation and for
# provenance on releases that predate the bundled provenance asset.
public_attestation() {
	predicate="$1"
	bundle="$2"
	IFS=' ' read -r digest _ < selected-checksum.txt || return 1
	try_fetch "https://api.github.com/repos/glade-sh/glade/attestations/sha256:$digest?predicate_type=$predicate&per_page=100" attestation-response.json 60 \
		-H 'Accept: application/vnd.github+json' -H 'X-GitHub-Api-Version: 2022-11-28' || return 1
	jq -cer '.attestations[] |
		if (.bundle | type) == "object" then .bundle
		elif (.bundle_url | type) == "string" and (.bundle_url | test("^https://[^\\r\\n]+$"))
		then .bundle_url else error("invalid attestation") end' attestation-response.json > bundle-parts.txt || return 1
	: > "$bundle"
	while IFS= read -r part; do
		case "$part" in
			https://*)
				try_fetch "$part" bundle-part.json 60 || return 1
				jq -ce 'objects' bundle-part.json >> "$bundle" || return 1
				;;
			*) printf '%s\n' "$part" >> "$bundle" || return 1 ;;
		esac
	done < bundle-parts.txt
	[ -s "$bundle" ]
}

require_github_login() {
	gh auth status --hostname github.com >/dev/null 2>&1 ||
		fail "could not fetch public $1 attestations; run gh auth login and retry"
}

export GH_HOST=github.com GH_TELEMETRY=false GH_NO_UPDATE_NOTIFIER=1
provenance_bundle="$archive.sigstore.json"
set --
if try_fetch "${base}/${provenance_bundle}" "$provenance_bundle" 60 2>/dev/null ||
	try_fetch "https://github.com/glade-sh/glade/releases/download/${version}/${provenance_bundle}" "$provenance_bundle" 60 2>/dev/null; then
	set -- --bundle "$provenance_bundle"
elif public_attestation 'https%3A%2F%2Fslsa.dev%2Fprovenance%2Fv1' provenance.jsonl; then
	set -- --bundle provenance.jsonl
else
	require_github_login provenance
fi
gh attestation verify "$archive" -R glade-sh/glade "$@" \
	--signer-workflow glade-sh/glade/.github/workflows/release.yml \
	--source-ref "refs/tags/${version}" \
	|| fail 'provenance verification failed; do not extract or run the archive'

set --
if public_attestation 'https%3A%2F%2Fcyclonedx.org%2Fbom' sbom.jsonl; then
	set -- --bundle sbom.jsonl
else
	require_github_login SBOM
fi
gh attestation verify "$archive" -R glade-sh/glade "$@" \
	--signer-workflow glade-sh/glade/.github/workflows/release.yml \
	--source-ref "refs/tags/${version}" \
	--predicate-type https://cyclonedx.org/bom \
	|| fail 'CycloneDX attestation verification failed; do not extract or run the archive'

printf '\nDownload verification completed for %s.\nArchive: %s/%s\nNot extracted, installed, or executed.\n' \
	"$version" "$workdir" "$archive"
```
<!-- release-verifier:end -->

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
