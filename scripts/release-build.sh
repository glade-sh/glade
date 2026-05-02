#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(git -C "${ROOT}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
DIST_DIR="${DIST_DIR:-${ROOT}/dist}"
LDFLAGS="-s -w -X github.com/open-aer/oaer/internal/oaercli.Version=${VERSION}"

platforms=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

for platform in "${platforms[@]}"; do
  goos="${platform%%/*}"
  goarch="${platform##*/}"
  name="oaer_${VERSION}_${goos}_${goarch}"
  workdir="$(mktemp -d)"
  binary="oaer"
  archive="${name}.tar.gz"

  if [[ "${goos}" == "windows" ]]; then
    binary="oaer.exe"
    archive="${name}.zip"
  fi

  (
    cd "${ROOT}"
    CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" go build -trimpath -ldflags "${LDFLAGS}" -o "${workdir}/${binary}" ./cmd/oaer
  )
  cp "${ROOT}/LICENSE" "${workdir}/LICENSE"

  if [[ "${goos}" == "windows" ]]; then
    (
      cd "${workdir}"
      zip -q "${DIST_DIR}/${archive}" "${binary}" LICENSE
    )
  else
    tar -C "${workdir}" -czf "${DIST_DIR}/${archive}" "${binary}" LICENSE
  fi

  rm -rf "${workdir}"
done

(
  cd "${DIST_DIR}"
  shasum -a 256 ./* > SHA256SUMS.txt
)

echo "release artifacts written to ${DIST_DIR}"
