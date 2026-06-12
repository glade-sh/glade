#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(git -C "${ROOT}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
DIST_DIR="${DIST_DIR:-${ROOT}/dist}"
LDFLAGS="-s -w -X github.com/glade-sh/glade/internal/gladecli.Version=${VERSION}"

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
name="glade_${VERSION}_${goos}_${goarch}"
binary="glade"
archive="${name}.tar.gz"
workdir="$(mktemp -d)"

cleanup() {
	rm -rf "${workdir}"
}
trap cleanup EXIT

if [[ "${goos}" == "windows" ]]; then
	binary="glade.exe"
	archive="${name}.zip"
fi

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

(
	cd "${ROOT}/third_party/lwc"
	if [[ ! -d node_modules ]]; then
		npm ci
	fi
)

(
	cd "${ROOT}"
	CGO_ENABLED=1 go build -trimpath -ldflags "${LDFLAGS}" -o "${workdir}/${binary}" ./cmd/glade
)

mkdir -p "${workdir}/share/glade/lwcruntime/src"
cp -R "${ROOT}/third_party/lwc" "${workdir}/share/glade/third_party/lwc"
cp -R "${ROOT}/lwcruntime/src/shims" "${workdir}/share/glade/lwcruntime/src/shims"

doctor_out="$("${workdir}/${binary}" doctor --json 2>&1)"
if [[ "${doctor_out}" != *'"parserOK": true'* ]]; then
	echo "ERROR: built binary reports parser unavailable; aborting" >&2
	printf '%s\n' "${doctor_out}" >&2
	exit 1
fi

cp "${ROOT}/LICENSE" "${workdir}/LICENSE"
if [[ "${goos}" == "windows" ]]; then
	(
		cd "${workdir}"
		zip -q "${DIST_DIR}/${archive}" "${binary}" LICENSE share
	)
else
	tar -C "${workdir}" -czf "${DIST_DIR}/${archive}" "${binary}" LICENSE share
fi

(
	cd "${DIST_DIR}"
	shasum -a 256 "./${archive}" > "${archive}.sha256"
	cp "${archive}.sha256" SHA256SUMS.txt
)

echo "release artifact written to ${DIST_DIR}/${archive}"
