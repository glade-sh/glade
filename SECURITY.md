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

This verification-only helper requires a POSIX shell, `curl`, `jq`, `gh`,
`awk`, `mktemp`, `uname`, and either `shasum` or `sha256sum`. Configure GitHub
CLI access as required by your environment. Save the block as
`verify-release-download.sh`, review it, then run
`sh verify-release-download.sh`.

It selects the macOS or Linux archive for the stable version in the distribution
manifest. Every failed gate stops the script. It leaves the downloaded files in
a new temporary directory and does not extract, install, or execute Glade.

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

Compare the matching `*.sbom.json` release asset with your internal dependency
allowlist when policy requires an inventory review.

The v0.2.15 binaries embed commit `82b8495972715a27aec8f864bc4ab39e7fa03974`
with `vcs.modified=false`.

The v0.2.15 per-archive SBOMs inventory 128–129 components: 32–33 Go modules
plus 96 bundled LWC/Babel and VSIX npm packages. The macOS archives contain 33
Go modules; the Linux archives contain 32. The earlier v0.2.13 inventory
remains Go-only. Compare the SBOM for the exact archive you install.
