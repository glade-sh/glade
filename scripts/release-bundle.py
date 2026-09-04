#!/usr/bin/env python3
"""Create a deterministic bundle, or retain identical uploaded contents on resume."""
import gzip
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import sys
import tarfile
from urllib.parse import quote
import zipfile


def digest(stream):
    result = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        result.update(chunk)
    return result.digest()


def component(name, version, license_name, archive_path):
    item = {
        "type": "library",
        "name": name,
        "version": version,
        "purl": f"pkg:npm/{quote(name, safe='/')}@{quote(version, safe='')}",
        "scope": "required",
        "properties": [{"name": "glade:archive-path", "value": archive_path}],
    }
    if isinstance(license_name, str) and license_name:
        item["licenses"] = [{"license": {"name": license_name}}]
    return item


def archive_members(archive_path):
    members = {}
    with tarfile.open(archive_path, "r:gz") as archive:
        for member in archive:
            name = PurePosixPath(member.name)
            if name.is_absolute() or ".." in name.parts or "\\" in member.name:
                raise SystemExit(f"unsafe archive member: {member.name}")
            if member.isfile():
                if name.as_posix() in members:
                    raise SystemExit(f"duplicate archive member: {member.name}")
                with archive.extractfile(member) as source:
                    members[name.as_posix()] = source.read()
    return members


def is_node_package_manifest(path, prefix):
    if not path.startswith(prefix) or not path.endswith("/package.json"):
        return False
    package_path = path[len(prefix):]
    nested_prefix = "node_modules/"
    if nested_prefix in package_path:
        package_path = package_path[package_path.rfind(nested_prefix) + len(nested_prefix):]
    parts = package_path.split("/")
    if parts[0].startswith("@"):
        return len(parts) == 3
    return len(parts) == 2


def package_path_from_esbuild_input(path):
    marker = "node_modules/"
    index = path.rfind(marker)
    if index < 0:
        raise SystemExit(f"invalid bundled package input: {path}")
    prefix = path[:index + len(marker)]
    parts = path[index + len(marker):].split("/")
    package_parts = parts[:2] if parts[0].startswith("@") else parts[:1]
    if not package_parts or any(not part for part in package_parts):
        raise SystemExit(f"invalid bundled package input: {path}")
    return prefix + "/".join(package_parts)


def add_archive_javascript_components(archive_path, sbom_path):
    members = archive_members(archive_path)
    if "LICENSE" not in members:
        raise SystemExit("archive is missing LICENSE notice")
    if "NOTICE" not in members:
        raise SystemExit("archive is missing NOTICE")

    try:
        document = json.loads(sbom_path.read_text(encoding="utf-8"))
        lwc_root = json.loads(members["share/glade/third_party/lwc/package.json"])
    except KeyError as error:
        raise SystemExit(f"archive is missing packaged LWC metadata: {error}") from error
    except json.JSONDecodeError as error:
        raise SystemExit(f"invalid packaged metadata: {error}") from error
    if not isinstance(document, dict):
        raise SystemExit("CycloneDX document must be an object")
    components = document.setdefault("components", [])
    if not isinstance(components, list):
        raise SystemExit("CycloneDX components must be an array")
    existing_purls = {item.get("purl") for item in components if isinstance(item, dict)}

    added = []
    lwc_prefix = "share/glade/third_party/lwc/node_modules/"
    lwc_components = set()
    for path in sorted(members):
        if not is_node_package_manifest(path, lwc_prefix):
            continue
        try:
            package = json.loads(members[path])
        except json.JSONDecodeError as error:
            raise SystemExit(f"invalid packaged LWC manifest {path}: {error}") from error
        name, version = package.get("name"), package.get("version")
        if not isinstance(name, str) or not isinstance(version, str):
            raise SystemExit(f"packaged LWC manifest lacks name/version: {path}")
        item = component(name, version, package.get("license"), path)
        lwc_components.add(name)
        if item["purl"] not in existing_purls:
            components.append(item)
            existing_purls.add(item["purl"])
            added.append(name)
    dependencies = lwc_root.get("dependencies", {})
    if not isinstance(dependencies, dict) or not set(dependencies).issubset(lwc_components):
        raise SystemExit("packaged LWC direct dependency metadata is incomplete")

    vsix_path = "share/glade/editor/vscode-glade.vsix"
    if vsix_path in members:
        try:
            with zipfile.ZipFile(io.BytesIO(members[vsix_path])) as vsix:
                if "extension/LICENSE.txt" not in vsix.namelist():
                    raise SystemExit("VSIX is missing extension/LICENSE.txt notice")
                if "extension/NOTICE" not in vsix.namelist():
                    raise SystemExit("VSIX is missing extension/NOTICE")
                bundle = vsix.read("extension/out/extension.js")
                meta = json.loads(vsix.read("extension/out/extension.meta.json"))
                evidence = json.loads(vsix.read("extension/out/bundled-dependencies.json"))
                notices = vsix.read("extension/out/THIRD_PARTY_NOTICES.txt").decode("utf-8")
        except KeyError as error:
            raise SystemExit(f"VSIX is missing bundled dependency evidence: {error}") from error
        except (UnicodeDecodeError, json.JSONDecodeError, zipfile.BadZipFile) as error:
            raise SystemExit(f"invalid packaged VSIX metadata: {error}") from error
        bundle_metadata = evidence.get("bundle", {})
        if bundle_metadata.get("path") != "out/extension.js" or bundle_metadata.get("sha256") != hashlib.sha256(bundle).hexdigest():
            raise SystemExit("VSIX bundled dependency evidence does not match extension.js")
        inputs = set()
        for output in meta.get("outputs", {}).values():
            if not isinstance(output, dict):
                continue
            for path, details in output.get("inputs", {}).items():
                if isinstance(details, dict) and details.get("bytesInOutput", 0) > 0 and "node_modules/" in path:
                    inputs.add(package_path_from_esbuild_input(path[path.find("node_modules/"):]))
        if not inputs:
            raise SystemExit("VSIX metafile has no bundled package inputs")
        packages = evidence.get("packages")
        if not isinstance(packages, list):
            raise SystemExit("VSIX bundled dependency evidence lacks packages")
        package_paths = set()
        for package in packages:
            if not isinstance(package, dict):
                raise SystemExit("VSIX bundled dependency evidence has an invalid package")
            name, version, license_name, package_path = (package.get(key) for key in ("name", "version", "license", "packagePath"))
            notice_files = package.get("noticeFiles")
            if not all(isinstance(value, str) and value for value in (name, version, license_name, package_path)):
                raise SystemExit("VSIX bundled dependency evidence lacks package metadata")
            if not isinstance(notice_files, list) or not notice_files or any(not isinstance(file, str) or not file for file in notice_files):
                raise SystemExit(f"VSIX bundled package lacks notice coverage: {package_path}")
            if package_path in package_paths or package_path not in inputs:
                raise SystemExit(f"VSIX bundled dependency evidence has an unexpected package: {package_path}")
            package_paths.add(package_path)
            if f"{name}@{version}" not in notices or any(f"--- {file} ---" not in notices for file in notice_files):
                raise SystemExit(f"VSIX bundled package notices are incomplete: {package_path}")
            item = component(name, version, license_name, vsix_path + "!/extension/out/bundled-dependencies.json")
            if item["purl"] not in existing_purls:
                components.append(item)
                existing_purls.add(item["purl"])
                added.append(name)
        if package_paths != inputs:
            raise SystemExit("VSIX bundled dependency evidence does not cover all metafile inputs")

    sbom_path.write_text(json.dumps(document, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(f"Added {len(added)} archive-resolved JavaScript components to {sbom_path}")


def main():
    if len(sys.argv) == 4 and sys.argv[1] == "sbom":
        add_archive_javascript_components(*(Path(value).resolve() for value in sys.argv[2:]))
        return
    if len(sys.argv) != 3:
        raise SystemExit("usage: release-bundle.py <source-directory> <bundle-outside-source> | release-bundle.py sbom <archive> <sbom>")
    source, output = (Path(value).resolve() for value in sys.argv[1:])
    if not source.is_dir() or output == source or source in output.parents:
        raise SystemExit("bundle output must be outside its source directory")
    paths = sorted(source.rglob("*"))
    if not paths or any(path.is_symlink() or not (path.is_file() or path.is_dir()) for path in paths):
        raise SystemExit("bundle source must contain only regular files and directories")
    files = {path.relative_to(source).as_posix(): path for path in paths if path.is_file()}
    if output.exists():
        seen = set()
        with tarfile.open(output, "r:gz") as archive:
            for member in archive:
                name = PurePosixPath(member.name)
                if name.is_absolute() or ".." in name.parts or "\\" in member.name:
                    raise SystemExit("unsafe existing bundle member")
                key = name.as_posix()
                if member.isdir():
                    if not (source / key).is_dir():
                        raise SystemExit(f"unexpected bundle directory: {key}")
                    continue
                if not member.isfile() or key not in files or key in seen:
                    raise SystemExit(f"unexpected bundle member: {key}")
                seen.add(key)
                with archive.extractfile(member) as stored, files[key].open("rb") as candidate:
                    if member.size != files[key].stat().st_size or digest(stored) != digest(candidate):
                        raise SystemExit(f"existing bundle differs: {key}; refusing replacement")
            if seen != set(files):
                raise SystemExit("existing bundle has an incomplete file set")
        print("Existing bundle contents verified; retaining original bytes")
        return

    def normalize(info):
        info.uid = info.gid = info.mtime = 0
        info.uname = info.gname = ""
        return info

    with output.open("xb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w") as archive:
                for path in paths:
                    archive.add(path, arcname=path.relative_to(source).as_posix(), recursive=False, filter=normalize)


if __name__ == "__main__":
    main()
