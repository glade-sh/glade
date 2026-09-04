#!/usr/bin/env python3
"""Materialize notice evidence for the Go modules linked into one release binary."""
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys


PARSER_MODULE = "github.com/glade-sh/apex-parser"
NOTICE_NAME = re.compile(r"^(license|notice|copying)([._-].*)?$", re.IGNORECASE)


def command(*args):
    return subprocess.check_output(args, text=True).strip()


def module_cache_path(module, version):
    def escape(value):
        return "".join("!" + char.lower() if char.isupper() else char for char in value)

    return Path(command("go", "env", "GOMODCACHE")) / (escape(module) + "@" + escape(version))


def go_distribution_license():
    go_root = Path(command("go", "env", "GOROOT"))
    candidates = [go_root / "LICENSE"]
    # Homebrew packages the standard Go tree under libexec and keeps the
    # distribution LICENSE at the formula root.
    if go_root.name == "libexec":
        candidates.append(go_root.parent / "LICENSE")
    for candidate in candidates:
        if candidate.is_file() and not candidate.is_symlink() and candidate.stat().st_size > 0:
            return candidate
    raise SystemExit("Go distribution LICENSE is missing")


def sha256_file(path):
    with path.open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def linked_modules(binary):
    modules = []
    current = None
    for line in command("go", "version", "-m", str(binary)).splitlines():
        fields = line.split()
        if len(fields) >= 3 and fields[0] == "dep":
            current = {"module": fields[1], "version": fields[2], "replacement": None}
            modules.append(current)
        elif len(fields) >= 2 and fields[0] == "=>" and current is not None:
            current["replacement"] = fields[1:]
    if not modules:
        raise SystemExit("built binary has no linked module metadata")
    return sorted(modules, key=lambda item: (item["module"], item["version"]))


def copy_notices(source, destination, required=()):
    if not source.is_dir():
        raise SystemExit(f"linked module source is missing: {source}")
    names = sorted(path.name for path in source.iterdir() if path.is_file() and not path.is_symlink() and NOTICE_NAME.match(path.name))
    if any(name not in names for name in required):
        raise SystemExit(f"linked module source lacks required notice evidence: {source}")
    if not names:
        raise SystemExit(f"linked module source lacks notice evidence: {source}")
    destination.mkdir(parents=True)
    for name in names:
        if (source / name).stat().st_size == 0:
            raise SystemExit(f"linked module source has empty notice evidence: {source / name}")
        shutil.copyfile(source / name, destination / name)
    return names


def safe_component_path(module, version):
    path = PurePosixPath(module)
    if path.is_absolute() or ".." in path.parts or not module or "/" in version or ".." in version:
        raise SystemExit(f"unsafe module metadata: {module}@{version}")
    return Path("modules") / Path(*path.parts) / ("@" + version)


def replacement_source(replacement, source_root):
    if not replacement:
        return None, None
    target = replacement[0]
    if target.startswith(".") or Path(target).is_absolute():
        candidate = (source_root / target).resolve() if not Path(target).is_absolute() else Path(target).resolve()
        if candidate != source_root and source_root not in candidate.parents:
            raise SystemExit(f"replacement escapes source root: {target}")
        return candidate, {"path": candidate.relative_to(source_root).as_posix()}
    if len(replacement) < 2:
        raise SystemExit(f"replacement lacks version: {' '.join(replacement)}")
    return module_cache_path(target, replacement[1]), {"module": target, "version": replacement[1]}


def main(binary_arg, source_root_arg, output_arg):
    binary, source_root, output = (Path(value).resolve() for value in (binary_arg, source_root_arg, output_arg))
    if not binary.is_file() or not source_root.is_dir() or output.exists():
        raise SystemExit("binary and source root must exist; notice destination must be new")
    output.mkdir(parents=True)

    go_license = go_distribution_license()
    go_destination = output / "go"
    go_destination.mkdir()
    shutil.copyfile(go_license, go_destination / "LICENSE")

    linked = linked_modules(binary)
    if not any(item["module"] == PARSER_MODULE for item in linked):
        raise SystemExit("built binary is missing the required vendored parser module")
    components = []
    for linked in linked:
        module, version, replacement = linked["module"], linked["version"], linked["replacement"]
        relative = safe_component_path(module, version)
        source, replacement_detail = replacement_source(replacement, source_root)
        if module == PARSER_MODULE:
            files = copy_notices(source or module_cache_path(module, version), output / relative, required=("LICENSE", "NOTICE.md"))
            kind = "vendored" if replacement_detail and "path" in replacement_detail else ("replacement" if replacement_detail else "module-cache")
        else:
            files = copy_notices(source or module_cache_path(module, version), output / relative)
            kind = "replacement" if replacement_detail else "module-cache"
        component = {"module": module, "version": version, "kind": kind, "noticeFiles": [str(relative / name) for name in files]}
        if replacement_detail:
            component["replacement"] = replacement_detail
        components.append(component)

    manifest = {
        "schemaVersion": 1,
        "binarySHA256": sha256_file(binary),
        "goLicense": "go/LICENSE",
        "components": components,
    }
    (output / "NOTICE-MANIFEST.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")


if __name__ == "__main__":
    if len(sys.argv) != 4:
        raise SystemExit("usage: release-go-notices.py <binary> <source-root> <notice-output>")
    main(*sys.argv[1:])
