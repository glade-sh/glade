#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
DIST_DIR="${TMP}/release-dist"

cleanup() {
	rm -rf "${TMP}"
}
trap cleanup EXIT

cd "${ROOT}"
DIST_DIR="${DIST_DIR}" VERSION=smoke "${ROOT}/scripts/release-build.sh" >"${TMP}/release-build.out"

python3 - "${DIST_DIR}" "${TMP}" <<'PY'
import hashlib
import json
import os
from pathlib import Path
from pathlib import PurePosixPath
import posixpath
import re
import stat
import sys
import tarfile
from urllib.parse import unquote, urlparse
import zipfile

dist = Path(sys.argv[1])
metadata = Path(sys.argv[2])

def load(path):
    with path.open(encoding="utf-8") as stream:
        return json.load(stream)

def require(condition, message):
    if not condition:
        raise SystemExit(message)

def digest(path):
    value = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()

def checksum_line(path, archive_name, expected):
    fields = path.read_text(encoding="utf-8").strip().split()
    require(len(fields) == 2, f"invalid checksum file: {path}")
    require(fields[0] == expected, f"checksum digest mismatch: {path}")
    require(fields[1] in (archive_name, f"./{archive_name}"), f"checksum filename mismatch: {path}")
    return fields[1]

def safe_member_name(name):
    require(isinstance(name, str) and name, "unsafe archive member: empty name")
    require("\\" not in name, f"unsafe archive member: {name}")
    path = PurePosixPath(name)
    require(not path.is_absolute(), f"unsafe archive member: {name}")
    require(not re.match(r"^[A-Za-z]:", name), f"unsafe archive member: {name}")
    require(".." not in path.parts, f"unsafe archive member: {name}")

def safe_link_target(member_name, target, relative):
    require(isinstance(target, str) and target, f"unsafe archive member link: {member_name}")
    require("\\" not in target and not target.startswith("/"), f"unsafe archive member link: {member_name}")
    require(not re.match(r"^[A-Za-z]:", target), f"unsafe archive member link: {member_name}")
    base = posixpath.dirname(member_name) if relative else ""
    resolved = posixpath.normpath(posixpath.join(base, target))
    require(resolved != ".." and not resolved.startswith("../"), f"unsafe archive member link: {member_name}")

def validate_archive_members(path, name):
    if name.endswith(".tar.gz"):
        with tarfile.open(path, "r:gz") as archive:
            for member in archive.getmembers():
                safe_member_name(member.name)
                allowed = member.isfile() or member.isdir() or member.issym() or member.islnk()
                require(allowed, f"unsafe archive member: {member.name}")
                if member.issym():
                    safe_link_target(member.name, member.linkname, True)
                elif member.islnk():
                    safe_link_target(member.name, member.linkname, False)
    elif name.endswith(".zip"):
        with zipfile.ZipFile(path) as archive:
            for member in archive.infolist():
                safe_member_name(member.filename)
                mode = member.external_attr >> 16
                file_type = stat.S_IFMT(mode)
                if member.is_dir():
                    require(file_type in (0, stat.S_IFDIR), f"unsafe archive member: {member.filename}")
                elif file_type == stat.S_IFLNK:
                    safe_link_target(member.filename, archive.read(member).decode("utf-8"), True)
                else:
                    require(file_type in (0, stat.S_IFREG), f"unsafe archive member: {member.filename}")
    else:
        raise SystemExit(f"unsupported release archive: {name}")

manifest = load(dist / "release-manifest.json")
require(manifest.get("schemaVersion") == 2, "root manifest schema must be 2")
require(manifest.get("version") == "smoke", "root manifest version must be smoke")
assets = manifest.get("assets")
require(isinstance(assets, list) and len(assets) == 1, "root manifest must contain exactly one asset")
manifest_asset = assets[0]
require(isinstance(manifest_asset, dict), "manifest asset must be an object")

asset_url = manifest_asset.get("url")
require(isinstance(asset_url, str) and asset_url, "asset URL must be nonempty")
parsed = urlparse(asset_url)
raw_name = parsed.path.rsplit("/", 1)[-1]
archive_name = unquote(raw_name)
require(bool(archive_name), "asset URL basename must be nonempty")
require(not os.path.isabs(archive_name), "asset URL basename must not be absolute")
require("/" not in archive_name and "\\" not in archive_name, "asset URL basename must not contain slashes")
require(archive_name not in (".", ".."), "asset URL basename must not be a dot segment")
require(os.path.basename(archive_name) == archive_name, "asset URL basename must be safe")

expected = manifest_asset.get("sha256")
require(isinstance(expected, str) and re.fullmatch(r"[0-9a-fA-F]{64}", expected), "asset SHA-256 must be 64 hex characters")
expected = expected.lower()
asset_os = manifest_asset.get("os")
asset_arch = manifest_asset.get("arch")
require(isinstance(asset_os, str) and asset_os, "asset OS must be nonempty")
require(isinstance(asset_arch, str) and asset_arch, "asset architecture must be nonempty")

verification = manifest.get("verification")
require(isinstance(verification, dict), "manifest verification must be an object")
version_output = verification.get("versionOutput")
require(isinstance(version_output, str) and version_output, "manifest version output must be nonempty")
require(verification.get("doctor") == "passed", "manifest doctor verification must have passed")
require(verification.get("parserSmoke") == "passed", "manifest parser verification must have passed")

for path in (
    dist / f"release-manifest-{asset_os}-{asset_arch}.json",
    dist / "latest" / "release-manifest.json",
    dist / "smoke" / "release-manifest.json",
):
    require(load(path) == manifest, f"manifest copy differs: {path}")

index = load(dist / "index.json")
require(index.get("schemaVersion") == 1, "release index schema must be 1")
require(index.get("latest") == "smoke", "release index latest must be smoke")
versions = index.get("versions")
require(isinstance(versions, list) and len(versions) == 1, "release index must contain exactly one version")
entry = versions[0]
require(isinstance(entry, dict) and entry.get("version") == "smoke", "release index version must be smoke")
reference = entry.get("manifest")
require(reference == "https://downloads.glade.sh/smoke/release-manifest.json", "release index manifest reference is invalid")

archive = dist / archive_name
require(archive.is_file(), "manifest-described archive is missing")
require(digest(archive) == expected, "manifest archive checksum mismatch")
validate_archive_members(archive, archive_name)
archive_checksum_name = checksum_line(dist / f"{archive_name}.sha256", archive_name, expected)
sums_checksum_name = checksum_line(dist / "SHA256SUMS.txt", archive_name, expected)
require(archive_checksum_name == sums_checksum_name, "checksum files name the archive differently")

version_dir = dist / "smoke"
for name in (archive_name, f"{archive_name}.sha256", "SHA256SUMS.txt"):
    root_copy = dist / name
    version_copy = version_dir / name
    require(version_copy.is_file(), f"versioned copy is missing: {name}")
    require(root_copy.read_bytes() == version_copy.read_bytes(), f"versioned copy differs: {name}")

(metadata / "archive-name").write_text(archive_name, encoding="utf-8")
(metadata / "asset-os").write_text(asset_os, encoding="utf-8")
(metadata / "version-output").write_text(version_output, encoding="utf-8")
PY

ARCHIVE_NAME="$(cat "${TMP}/archive-name")"
ASSET_OS="$(cat "${TMP}/asset-os")"
VERSION_OUTPUT="$(cat "${TMP}/version-output")"
EXTRACT_DIR="${TMP}/extracted"
mkdir -p "${EXTRACT_DIR}"

case "${ARCHIVE_NAME}" in
	*.tar.gz)
		tar -C "${EXTRACT_DIR}" -xzf "${DIST_DIR}/${ARCHIVE_NAME}"
		;;
	*.zip)
		unzip -q "${DIST_DIR}/${ARCHIVE_NAME}" -d "${EXTRACT_DIR}"
		;;
	*)
		echo "unsupported release archive: ${ARCHIVE_NAME}" >&2
		exit 1
		;;
esac

if [[ "${ASSET_OS}" == "windows" ]]; then
	GLADE="${EXTRACT_DIR}/glade.exe"
else
	GLADE="${EXTRACT_DIR}/glade"
fi
python3 - "${EXTRACT_DIR}" "${GLADE}" <<'PY'
from pathlib import Path
import stat
import sys

root = Path(sys.argv[1]).resolve(strict=True)
binary = Path(sys.argv[2])
try:
    resolved = binary.resolve(strict=True)
    resolved.relative_to(root)
except (FileNotFoundError, RuntimeError, ValueError):
    raise SystemExit(f"manifest-selected release binary escapes extraction root: {binary}")
mode = binary.lstat().st_mode
if stat.S_ISLNK(mode) or not stat.S_ISREG(mode):
    raise SystemExit(f"manifest-selected release binary is not a regular file: {binary}")
PY
if [[ ! -x "${GLADE}" ]]; then
	echo "manifest-selected release binary is not executable: ${GLADE}" >&2
	exit 1
fi

HASH_BEFORE="$(shasum -a 256 "${GLADE}" | awk '{print $1}')"
ACTUAL_VERSION="$("${GLADE}" version 2>&1)"
if [[ "${ACTUAL_VERSION}" != "${VERSION_OUTPUT}" ]]; then
	echo "release binary version output differs from manifest" >&2
	exit 1
fi
"${ROOT}/scripts/smoke-runtime.sh" "${GLADE}"
HASH_AFTER="$(shasum -a 256 "${GLADE}" | awk '{print $1}')"
if [[ "${HASH_BEFORE}" != "${HASH_AFTER}" ]]; then
	echo "release binary changed during runtime smoke" >&2
	exit 1
fi

echo "distribution smoke: ok"
