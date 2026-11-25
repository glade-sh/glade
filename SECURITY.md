# Security Policy

## Supported versions

Security fixes ship on the current release line. Install the latest release
unless your team has pinned a version for review.

Release archives are built by GitHub Actions from tagged source and published
with `SHA256SUMS.txt`, CycloneDX SBOM files, and GitHub artifact attestations.
The tagged commit must have an exact-SHA successful `Required CI` push run.
Archive provenance and CycloneDX attestations must verify before upload.

## Release assets

Choose `glade_vX.Y.Z_<os>_<cpu>.tar.gz` for your system from
[GitHub Releases](https://github.com/glade-sh/glade/releases) or the
`https://downloads.glade.sh/vX.Y.Z/` mirror path. Use `darwin` for macOS
or `linux` for Linux. Use `arm64` for Apple silicon or other ARM64 systems,
or `amd64` for x86-64.

- `SHA256SUMS.txt` contains the platform archive checksums.
- `<archive>.sbom.json` lists dependencies for that archive. Compare it with
  your allowlist if your review process requires an inventory.
- `<archive>.sigstore.json`, where attached on GitHub Releases, contains
  the signed provenance for that archive. GitHub publishes the CycloneDX
  attestation separately through its attestation API.
- `required-ci-attestation.json` records the successful Required CI run.
- `salesforce-release-authority.json` records the Salesforce correctness
  check for the release.
- `glade-release-artifacts-vX.Y.Z.tar.gz` is the audit bundle. Install the
  platform archive for your OS and CPU.

## Report a vulnerability

Use [private vulnerability reporting](https://github.com/glade-sh/glade/security/advisories/new)
for Glade, or the [Tools private reporting route](https://github.com/glade-sh/glade-tools/security/advisories/new)
for first-party plugins. Private reporting is enabled for both repositories.

If GitHub reporting is unavailable, email [security@glade.sh](mailto:security@glade.sh).
Do not post vulnerability details in a public issue or discussion.

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

These workflows can make network requests:

- The installer fetches the script from `glade.sh` and release files from
  `downloads.glade.sh` or GitHub Releases. `glade update` runs the installer
  embedded in the installed binary. The installer and verification helper
  can fetch public attestations from `api.github.com` and GitHub's bundle
  storage. `gh` can fetch Sigstore trust material and, with a GitHub login,
  attestations. The current standalone installer and verifier disable
  GitHub CLI telemetry and its update check. Older embedded installers use
  your GitHub CLI settings.
- Plugin discovery, search, and registry lookups use
  `https://plugins.glade.sh/index.json` or the registry you configured.
  Remote plugin install and restore fetch the selected archive URL.
  Installed plugins run as separate executables and can contact services
  used by the plugin.
- Salesforce data imports and editor target checks can run Salesforce CLI
  (`sf`) against the org you selected or configured. Linking a local Glade
  org also invokes `sf`. Salesforce CLI has its own usage reporting, enabled
  by default. See [Salesforce CLI telemetry settings](https://developer.salesforce.com/docs/platform/sfdx-setup/guide/sfdx-dev-cli-telemetry.html).
- The local org commands send status and token requests to the configured
  Glade server, which defaults to loopback. Apex HTTP callouts run against
  local mocks or return a diagnostic. Browser preview code and links can
  make requests subject to the preview and browser policies.
- Opening documentation, issue, or release links sends a browser request to
  the link destination. Building from source can fetch Go modules and npm
  dependencies. The installed LWC toolchain is copied from local files.

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

This verification-only helper requires a POSIX shell, `curl`, `jq`, `gh`,
`awk`, `mktemp`, `uname`, and either `shasum` or `sha256sum`. Save the block
as `verify-release-download.sh`, review it, then run
`sh verify-release-download.sh`. A GitHub login is needed only if the helper
cannot fetch a required attestation as a local bundle. In that case,
run `gh auth login` and retry.

It selects the macOS or Linux archive for the stable version in the distribution
manifest. Every failed gate stops the script. It leaves the downloaded files in
a new temporary directory and does not extract, install, or execute Glade.

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

If verification stops, do not bypass the failed gate. Keep the exact command
and sanitized diagnostic. Verification alone does not approve an archive for
your organization's use; review its inventory and provenance under your own
policy.

Use the signed provenance to identify the release workflow, source commit,
and tag. Use the matching SBOM to review the inventory for that platform
archive.

### Historical example

The v0.2.15 binaries embed commit `82b8495972715a27aec8f864bc4ab39e7fa03974`
with `vcs.modified=false`. The per-archive SBOMs inventory 128–129 components,
with 32–33 Go modules and 96 bundled LWC/Babel and VSIX npm packages. The
macOS archives contain 33 Go modules and the Linux archives contain 32.
