#!/usr/bin/env bash
set -euo pipefail

# macOS archive/copy tools otherwise synthesize AppleDouble `._*` members for
# extended attributes. Those files are not release payload and are not listed
# in the deterministic payload checksum manifest.
export COPYFILE_DISABLE=1

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(git -C "${ROOT}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
DIST_DIR="${DIST_DIR:-${ROOT}/dist}"
DOWNLOAD_BASE_URL="${DOWNLOAD_BASE_URL:-https://downloads.glade.sh}"
INSTALL_SCRIPT_URL="${INSTALL_SCRIPT_URL:-https://glade.sh/install.sh}"
PLUGIN_REGISTRY_URL="${PLUGIN_REGISTRY_URL:-https://plugins.glade.sh/index.json}"
LDFLAGS="-s -w -X github.com/glade-sh/glade/internal/gladecli.Version=${VERSION}"
MODE="${1:-default}"
SHARED_PAYLOAD_NAME="glade-shared-payload.tar.gz"
SHARED_PAYLOAD_CHECKSUM_NAME="${SHARED_PAYLOAD_NAME}.sha256"

if (($# > 1)); then
	echo "usage: $0 [default|shared-payload|platform]" >&2
	exit 2
fi
case "${MODE}" in
	default | shared-payload | platform) ;;
	*)
		echo "unknown release build mode: ${MODE}" >&2
		exit 2
		;;
esac

workdir="$(mktemp -d)"
cleanup() {
	rm -rf "${workdir}"
}
trap cleanup EXIT

write_deterministic_payload_archive() {
	local source_dir="$1"
	local archive_path="$2"
	PAYLOAD_SOURCE_DIR="${source_dir}" PAYLOAD_ARCHIVE_PATH="${archive_path}" python3 - <<'PY'
import gzip
import os
import tarfile

source = os.environ["PAYLOAD_SOURCE_DIR"]
archive = os.environ["PAYLOAD_ARCHIVE_PATH"]

def normalize(info):
    info.uid = 0
    info.gid = 0
    info.uname = ""
    info.gname = ""
    info.mtime = 0
    return info

with open(archive, "wb") as raw:
    with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode="w") as tar:
            for current, dirs, files in os.walk(source):
                dirs.sort()
                files.sort()
                rel_current = os.path.relpath(current, source)
                if rel_current != ".":
                    tar.add(current, arcname=rel_current, recursive=False, filter=normalize)
                for name in files:
                    path = os.path.join(current, name)
                    rel = os.path.relpath(path, source)
                    tar.add(path, arcname=rel, recursive=False, filter=normalize)
PY
}

prepare_shared_payload() {
	local output_dir="$1"
	local payload_root="${workdir}/payload-root"
	local share_root="${payload_root}/share/glade"
	rm -rf "${payload_root}"
	mkdir -p "${share_root}/editor" "${share_root}/lwcruntime/src" "${share_root}/third_party" "${output_dir}"

	(
		cd "${ROOT}/third_party/lwc"
		if [[ ! -d node_modules ]]; then
			npm ci
		fi
	)

	local vscode_extension_package="not present"
	if [[ -f "${ROOT}/contrib/vscode-glade/package.json" ]]; then
		(
			cd "${ROOT}/contrib/vscode-glade"
			if [[ ! -d node_modules ]]; then
				npm ci
			fi
			rm -f dist/vscode-glade-*.vsix
			npm run package
		)
		cp "${ROOT}"/contrib/vscode-glade/dist/vscode-glade-*.vsix "${share_root}/editor/vscode-glade.vsix"
		vscode_extension_package="present"
	fi

	cp -R "${ROOT}/third_party/lwc" "${share_root}/third_party/lwc"
	# npm creates command shims in every node_modules/.bin, including nested
	# dependency trees. They are development/build tooling, not installed runtime
	# files. Remove only those copied .bin directories so every consumer can keep
	# the same strict no-links extraction policy.
	while IFS= read -r -d '' npm_bin_dir; do
		if [[ "$(basename "$(dirname "${npm_bin_dir}")")" == "node_modules" ]]; then
			rm -rf "${npm_bin_dir}"
		fi
	done < <(find "${share_root}/third_party/lwc" -type d -name .bin -print0)
	for runtime_dir in experience lightning shell shims slds; do
		cp -R "${ROOT}/lwcruntime/src/${runtime_dir}" "${share_root}/lwcruntime/src/${runtime_dir}"
	done
	local unsupported_member
	unsupported_member="$(find "${payload_root}/share/glade" ! -type d ! -type f -print -quit)"
	if [[ -n "${unsupported_member}" ]]; then
		echo "ERROR: shared payload producer found unsupported member ${unsupported_member#"${payload_root}/"}" >&2
		exit 1
	fi

	(
		cd "${payload_root}"
		find share/glade -type f ! -name PAYLOAD-SHA256SUMS.txt -print0 \
			| LC_ALL=C sort -z \
			| xargs -0 shasum -a 256 >share/glade/PAYLOAD-SHA256SUMS.txt
	)
	printf '%s\n' "${vscode_extension_package}" >"${share_root}/VSCODE-EXTENSION-STATUS.txt"
	(
		cd "${payload_root}"
		shasum -a 256 share/glade/VSCODE-EXTENSION-STATUS.txt >>share/glade/PAYLOAD-SHA256SUMS.txt
	)

	local payload_path="${output_dir}/${SHARED_PAYLOAD_NAME}"
	write_deterministic_payload_archive "${payload_root}" "${payload_path}"
	(
		cd "${output_dir}"
		shasum -a 256 "${SHARED_PAYLOAD_NAME}" >"${SHARED_PAYLOAD_CHECKSUM_NAME}"
	)
	echo "shared release payload written to ${payload_path}"
}

safe_extract_shared_payload() {
	local payload_archive="$1"
	local destination="$2"
	PAYLOAD_ARCHIVE_PATH="${payload_archive}" PAYLOAD_DESTINATION="${destination}" python3 - <<'PY'
import os
from pathlib import PurePosixPath
import tarfile

archive = os.environ["PAYLOAD_ARCHIVE_PATH"]
destination = os.environ["PAYLOAD_DESTINATION"]

def reject(message):
    raise SystemExit(f"ERROR: unsafe shared payload: {message}")

with tarfile.open(archive, mode="r:gz") as payload:
    members = payload.getmembers()
    seen = set()
    for member in members:
        name = member.name
        path = PurePosixPath(name)
        if not name or name in (".", "./"):
            reject(f"invalid empty root member {name!r}")
        if "\\" in name or path.is_absolute() or ".." in path.parts:
            reject(f"invalid member path {name!r}")
        normalized = path.as_posix().rstrip("/")
        if normalized in seen:
            reject(f"duplicate member {normalized!r}")
        seen.add(normalized)
        if normalized == "share":
            if not member.isdir():
                reject("share root must be a directory")
        elif normalized == "share/glade":
            if not member.isdir():
                reject("share/glade root must be a directory")
        elif not normalized.startswith("share/glade/"):
            reject(f"unexpected root member {name!r}")
        if not member.isdir() and not member.isreg():
            reject(f"unsupported member type for {name!r}")

    os.makedirs(destination, mode=0o755, exist_ok=True)
    for member in members:
        normalized = PurePosixPath(member.name).as_posix().rstrip("/")
        target = os.path.join(destination, *PurePosixPath(normalized).parts)
        if member.isdir():
            os.makedirs(target, mode=member.mode & 0o777, exist_ok=True)
            continue
        os.makedirs(os.path.dirname(target), mode=0o755, exist_ok=True)
        source = payload.extractfile(member)
        if source is None:
            reject(f"could not read regular member {member.name!r}")
        with source, open(target, "xb") as output:
            while True:
                chunk = source.read(1024 * 1024)
                if not chunk:
                    break
                output.write(chunk)
        os.chmod(target, member.mode & 0o777)
PY
}

verify_shared_payload() {
	local payload_root="$1"
	local manifest="${payload_root}/share/glade/PAYLOAD-SHA256SUMS.txt"
	for required in \
		"${manifest}" \
		"${payload_root}/share/glade/VSCODE-EXTENSION-STATUS.txt" \
		"${payload_root}/share/glade/third_party/lwc" \
		"${payload_root}/share/glade/lwcruntime/src/experience" \
		"${payload_root}/share/glade/lwcruntime/src/lightning" \
		"${payload_root}/share/glade/lwcruntime/src/shell" \
		"${payload_root}/share/glade/lwcruntime/src/shims" \
		"${payload_root}/share/glade/lwcruntime/src/slds"; do
		if [[ ! -e "${required}" ]]; then
			echo "ERROR: shared payload missing ${required#"${payload_root}/"}" >&2
			exit 1
		fi
	done
	(
		cd "${payload_root}"
		if ! shasum -a 256 -c share/glade/PAYLOAD-SHA256SUMS.txt; then
			echo "ERROR: shared payload manifest checksum validation failed" >&2
			exit 1
		fi
		find share/glade -type f ! -name PAYLOAD-SHA256SUMS.txt -print \
			| LC_ALL=C sort >"${workdir}/payload-files.actual"
		awk '{sub(/^[^ ]+  /, ""); print}' share/glade/PAYLOAD-SHA256SUMS.txt \
			| LC_ALL=C sort >"${workdir}/payload-files.manifest"
		if ! diff -u "${workdir}/payload-files.manifest" "${workdir}/payload-files.actual"; then
			echo "ERROR: shared payload manifest does not match extracted file set" >&2
			exit 1
		fi
	)
}

consume_platform_payload() {
	local payload_archive="$1"
	local payload_checksum="$2"
	if [[ ! -s "${payload_archive}" || ! -s "${payload_checksum}" ]]; then
		echo "ERROR: platform mode requires a nonempty shared payload and checksum" >&2
		exit 1
	fi
	local expected_payload_sha actual_payload_sha
	expected_payload_sha="$(awk 'NR == 1 {print $1}' "${payload_checksum}")"
	actual_payload_sha="$(shasum -a 256 "${payload_archive}" | awk '{print $1}')"
	if [[ -z "${expected_payload_sha}" || "${expected_payload_sha}" != "${actual_payload_sha}" ]]; then
		echo "ERROR: shared payload checksum mismatch" >&2
		exit 1
	fi

	local goos goarch name binary archive platform_root verifydir vscode_extension_package
	goos="$(go env GOOS)"
	goarch="$(go env GOARCH)"
	name="glade_${VERSION}_${goos}_${goarch}"
	binary="glade"
	archive="${name}.tar.gz"
	if [[ "${goos}" == "windows" ]]; then
		binary="glade.exe"
		archive="${name}.zip"
	fi
	platform_root="${workdir}/platform-root"
	mkdir -p "${platform_root}" "${DIST_DIR}"
	safe_extract_shared_payload "${payload_archive}" "${platform_root}"
	verify_shared_payload "${platform_root}"
	vscode_extension_package="$(cat "${platform_root}/share/glade/VSCODE-EXTENSION-STATUS.txt")"

	(
		cd "${ROOT}"
		CGO_ENABLED=1 go build -trimpath -ldflags "${LDFLAGS}" -o "${platform_root}/${binary}" ./cmd/glade
	)
	doctor_out="$("${platform_root}/${binary}" doctor --json 2>&1 || true)"
	if [[ "${doctor_out}" != *'"parserOK": true'* ]]; then
		echo "ERROR: built binary reports parser unavailable; aborting" >&2
		printf '%s\n' "${doctor_out}" >&2
		exit 1
	fi
	cp "${ROOT}/LICENSE" "${platform_root}/LICENSE"
	if [[ "${goos}" == "windows" ]]; then
		(
			cd "${platform_root}"
			zip -q "${DIST_DIR}/${archive}" "${binary}" LICENSE share
		)
	else
		tar -C "${platform_root}" -czf "${DIST_DIR}/${archive}" "${binary}" LICENSE share
	fi
	(
		cd "${DIST_DIR}"
		shasum -a 256 "./${archive}" >"${archive}.sha256"
		shasum -a 256 -c "${archive}.sha256"
		cp "${archive}.sha256" SHA256SUMS.txt
	)
	local archive_sha256
	archive_sha256="$(awk '{print $1}' "${DIST_DIR}/${archive}.sha256")"

	verifydir="$(mktemp -d "${workdir}/verify.XXXXXX")"
	if [[ "${goos}" == "windows" ]]; then
		unzip -q "${DIST_DIR}/${archive}" -d "${verifydir}"
	else
		tar -C "${verifydir}" -xzf "${DIST_DIR}/${archive}"
	fi
	verify_shared_payload "${verifydir}"
	local version_output doctor_json parser_smoke
	version_output="$("${verifydir}/${binary}" version 2>&1)"
	doctor_json="$("${verifydir}/${binary}" doctor --json 2>&1 || true)"
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
	export RELEASE_PLATFORM_MANIFEST_PATH="${DIST_DIR}/release-manifest-${goos}-${goarch}.json"
	export RELEASE_INDEX_PATH="${DIST_DIR}/index.json"
	export RELEASE_LATEST_MANIFEST_PATH="${DIST_DIR}/latest/release-manifest.json"
	export RELEASE_VERSION_MANIFEST_PATH="${DIST_DIR}/${VERSION}/release-manifest.json"
	export RELEASE_VERSION="${VERSION}"
	export RELEASE_CHANNEL="${RELEASE_CHANNEL:-stable}"
	export RELEASE_GOOS="${goos}"
	export RELEASE_GOARCH="${goarch}"
	export RELEASE_ARCHIVE="${archive}"
	export RELEASE_ARCHIVE_SHA256="${archive_sha256}"
	export RELEASE_SHARED_PAYLOAD_SHA256="${actual_payload_sha}"
	export RELEASE_ASSET_URL="${DOWNLOAD_BASE_URL%/}/${VERSION}/${archive}"
	export RELEASE_INSTALL_SCRIPT="${INSTALL_SCRIPT_URL}"
	export RELEASE_PLUGIN_REGISTRY="${PLUGIN_REGISTRY_URL}"
	export RELEASE_VERSION_OUTPUT="${version_output}"
	export RELEASE_DOCTOR_JSON="${doctor_json}"
	export RELEASE_PARSER_SMOKE="${parser_smoke}"
	export RELEASE_VSCODE_EXTENSION_PACKAGE="${vscode_extension_package}"
	mkdir -p "${DIST_DIR}/latest" "${DIST_DIR}/${VERSION}"
	python3 - <<'PY'
import json
import os

manifest = {
    "schemaVersion": 2,
    "channel": os.environ["RELEASE_CHANNEL"],
    "version": os.environ["RELEASE_VERSION"],
    "assets": [{
        "os": os.environ["RELEASE_GOOS"],
        "arch": os.environ["RELEASE_GOARCH"],
        "url": os.environ["RELEASE_ASSET_URL"],
        "sha256": os.environ["RELEASE_ARCHIVE_SHA256"],
    }],
    "installScript": os.environ["RELEASE_INSTALL_SCRIPT"],
    "pluginRegistry": os.environ["RELEASE_PLUGIN_REGISTRY"],
    "verification": {
        "versionOutput": os.environ["RELEASE_VERSION_OUTPUT"],
        "doctor": "passed",
        "parserSmoke": "passed",
        "vscodeExtensionPackage": os.environ["RELEASE_VSCODE_EXTENSION_PACKAGE"],
        "sharedPayloadSHA256": os.environ["RELEASE_SHARED_PAYLOAD_SHA256"],
    },
}
for path in (os.environ["RELEASE_MANIFEST_PATH"], os.environ["RELEASE_PLATFORM_MANIFEST_PATH"], os.environ["RELEASE_LATEST_MANIFEST_PATH"], os.environ["RELEASE_VERSION_MANIFEST_PATH"]):
    with open(path, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2, sort_keys=True)
        f.write("\n")
index = {
    "schemaVersion": 1,
    "channel": os.environ["RELEASE_CHANNEL"],
    "latest": os.environ["RELEASE_VERSION"],
    "versions": [{
        "version": os.environ["RELEASE_VERSION"],
        "manifest": f"https://downloads.glade.sh/{os.environ['RELEASE_VERSION']}/release-manifest.json",
    }],
}
with open(os.environ["RELEASE_INDEX_PATH"], "w", encoding="utf-8") as f:
    json.dump(index, f, indent=2, sort_keys=True)
    f.write("\n")

for path in (os.environ["RELEASE_MANIFEST_PATH"], os.environ["RELEASE_PLATFORM_MANIFEST_PATH"]):
    with open(path, "r", encoding="utf-8") as f:
        verified = json.load(f)
    if verified.get("verification", {}).get("sharedPayloadSHA256") != os.environ["RELEASE_SHARED_PAYLOAD_SHA256"]:
        raise SystemExit(f"shared payload hash missing from {path}")
    assets = verified.get("assets", [])
    if len(assets) != 1 or assets[0].get("sha256") != os.environ["RELEASE_ARCHIVE_SHA256"]:
        raise SystemExit(f"archive hash missing from {path}")
PY
	cp "${DIST_DIR}/${archive}" "${DIST_DIR}/${VERSION}/${archive}"
	cp "${DIST_DIR}/${archive}.sha256" "${DIST_DIR}/${VERSION}/${archive}.sha256"
	cp "${DIST_DIR}/SHA256SUMS.txt" "${DIST_DIR}/${VERSION}/SHA256SUMS.txt"

	echo "release artifact written to ${DIST_DIR}/${archive}"
	echo "release manifest written to ${DIST_DIR}/release-manifest.json"
}

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"
case "${MODE}" in
	shared-payload)
		prepare_shared_payload "${DIST_DIR}"
		;;
	platform)
		consume_platform_payload \
			"${RELEASE_SHARED_PAYLOAD_ARCHIVE:?RELEASE_SHARED_PAYLOAD_ARCHIVE is required in platform mode}" \
			"${RELEASE_SHARED_PAYLOAD_SHA256:?RELEASE_SHARED_PAYLOAD_SHA256 is required in platform mode}"
		;;
	default)
		private_payload_dir="${workdir}/private-shared"
		mkdir -p "${private_payload_dir}"
		prepare_shared_payload "${private_payload_dir}"
		consume_platform_payload \
			"${private_payload_dir}/${SHARED_PAYLOAD_NAME}" \
			"${private_payload_dir}/${SHARED_PAYLOAD_CHECKSUM_NAME}"
		;;
esac
