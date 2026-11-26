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
