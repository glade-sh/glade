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
