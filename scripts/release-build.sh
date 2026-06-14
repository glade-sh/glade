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
vscode_extension_package="not present"

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

if [[ -f "${ROOT}/contrib/vscode-glade/package.json" ]]; then
	(
		cd "${ROOT}/contrib/vscode-glade"
		if [[ ! -d node_modules ]]; then
			npm ci
		fi
		rm -f dist/vscode-glade-*.vsix
		npm run package
	)
	mkdir -p "${workdir}/share/glade/editor"
	cp "${ROOT}"/contrib/vscode-glade/dist/vscode-glade-*.vsix "${workdir}/share/glade/editor/vscode-glade.vsix"
	vscode_extension_package="present"
fi

mkdir -p "${workdir}/share/glade/lwcruntime/src" "${workdir}/share/glade/third_party"
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
archive_sha256="$(awk '{print $1}' "${DIST_DIR}/${archive}.sha256")"

verifydir="$(mktemp -d "${workdir}/verify.XXXXXX")"
if [[ "${goos}" == "windows" ]]; then
	unzip -q "${DIST_DIR}/${archive}" -d "${verifydir}"
else
	tar -C "${verifydir}" -xzf "${DIST_DIR}/${archive}"
fi
version_output="$("${verifydir}/${binary}" version 2>&1)"
doctor_json="$("${verifydir}/${binary}" doctor --json 2>&1)"
if [[ "${doctor_json}" != *'"parserOK": true'* ]]; then
	echo "ERROR: unpacked binary reports parser unavailable; aborting" >&2
	printf '%s\n' "${doctor_json}" >&2
	exit 1
fi
cat >"${workdir}/ParserSmoke.cls" <<'APEX'
public class ParserSmoke {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
APEX
parser_smoke="$("${verifydir}/${binary}" parse "${workdir}/ParserSmoke.cls" --json 2>&1)"
if [[ "${parser_smoke}" != *'"name": "ParserSmoke"'* ]]; then
	echo "ERROR: unpacked binary parser smoke failed; aborting" >&2
	printf '%s\n' "${parser_smoke}" >&2
	exit 1
fi

export RELEASE_MANIFEST_PATH="${DIST_DIR}/release-manifest.json"
export RELEASE_VERSION="${VERSION}"
export RELEASE_GOOS="${goos}"
export RELEASE_GOARCH="${goarch}"
export RELEASE_ARCHIVE="${archive}"
export RELEASE_ARCHIVE_SHA256="${archive_sha256}"
export RELEASE_VERSION_OUTPUT="${version_output}"
export RELEASE_DOCTOR_JSON="${doctor_json}"
export RELEASE_PARSER_SMOKE="${parser_smoke}"
export RELEASE_VSCODE_EXTENSION_PACKAGE="${vscode_extension_package}"
python3 - <<'PY'
import json
import os

manifest = {
    "schemaVersion": 1,
    "version": os.environ["RELEASE_VERSION"],
    "goos": os.environ["RELEASE_GOOS"],
    "goarch": os.environ["RELEASE_GOARCH"],
    "archive": os.environ["RELEASE_ARCHIVE"],
    "archive_sha256": os.environ["RELEASE_ARCHIVE_SHA256"],
    "version_output": os.environ["RELEASE_VERSION_OUTPUT"],
    "doctor_json": json.loads(os.environ["RELEASE_DOCTOR_JSON"]),
    "parser_smoke": {
        "status": "passed",
        "output": json.loads(os.environ["RELEASE_PARSER_SMOKE"]),
    },
    "vscode_extension_package": os.environ["RELEASE_VSCODE_EXTENSION_PACKAGE"],
}
with open(os.environ["RELEASE_MANIFEST_PATH"], "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2, sort_keys=True)
    f.write("\n")
PY

echo "release artifact written to ${DIST_DIR}/${archive}"
echo "release manifest written to ${DIST_DIR}/release-manifest.json"
