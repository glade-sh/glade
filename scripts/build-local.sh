#!/usr/bin/env bash
# Build a working glade binary for the host platform and stage it for handing
# off to a few machines before a public release.
#
# IMPORTANT: glade's Apex parser is a generated tree-sitter grammar and REQUIRES
# CGO. This script builds with CGO enabled, so it produces binaries only for the
# current OS/arch. To distribute to a different OS/arch, run this script on a
# machine of that platform (or build from source there).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(git -C "${ROOT}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
DIST_DIR="${DIST_DIR:-${ROOT}/dist}"
LDFLAGS="-s -w -X github.com/glade-sh/glade/internal/gladecli.Version=${VERSION}"

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
name="glade_${VERSION}_${goos}_${goarch}"
binary="glade"
[[ "${goos}" == "windows" ]] && binary="glade.exe"

mkdir -p "${DIST_DIR}"

echo "building ${name} (CGO enabled) ..."
(
  cd "${ROOT}"
  CGO_ENABLED=1 go build -trimpath -ldflags "${LDFLAGS}" -o "${DIST_DIR}/${binary}" ./cmd/glade
)

# Verify the parser is actually wired up (catches accidental no-CGO builds).
doctor_out="$("${DIST_DIR}/${binary}" doctor --json 2>&1)"
if [[ "${doctor_out}" != *'"parserOK": true'* ]]; then
  echo "ERROR: built binary reports parser unavailable; aborting" >&2
  printf '%s\n' "${doctor_out}" >&2
  exit 1
fi

archive="${name}.tar.gz"
[[ "${goos}" == "windows" ]] && archive="${name}.zip"

(
  cd "${DIST_DIR}"
  cp "${ROOT}/LICENSE" LICENSE
  if [[ "${goos}" == "windows" ]]; then
    zip -q "${archive}" "${binary}" LICENSE
  else
    tar -czf "${archive}" "${binary}" LICENSE
  fi
  shasum -a 256 "${archive}" > "${archive}.sha256"
  rm -f LICENSE
)

echo "wrote ${DIST_DIR}/${archive} (+ .sha256)"
echo "verify on the target with: glade version && glade doctor"
