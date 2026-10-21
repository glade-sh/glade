#!/usr/bin/env python3
"""Build once per workflow; run the same Go tests and JSON coverage guards."""

import hashlib
import json
import os
import shutil
from pathlib import Path
import subprocess
import sys


GO = os.environ.get("CI_TEST_GO_COMMAND", "go")

def output(*args):
    return subprocess.check_output((GO, *args[1:]) if args[0] == "go" else args, text=True).strip()


def identity():
    return {
        "revision": output("git", "rev-parse", "HEAD"),
        "toolchain": json.loads(output("go", "env", "-json", "GOOS", "GOARCH", "GOVERSION", "CGO_ENABLED", "GOFLAGS", "GOEXPERIMENT", "CC", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_LDFLAGS")),
    }


def digest(path):
    with path.open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def build(directory, mode, packages):
    if mode not in ("test", "race") or not packages:
        raise ValueError("build requires test/race and packages")
    directory = Path(directory).resolve()
    directory.mkdir(parents=True, exist_ok=True)
    manifest_path = directory / "manifest.json"
    manifest_path.unlink(missing_ok=True)
    manifest = {"version": 1, **identity(), "mode": mode, "packages": {}}
    if manifest["toolchain"]["CGO_ENABLED"] != "1":
        raise ValueError("shared CI test binaries require CGO_ENABLED=1")
    for index, package in enumerate(packages):
        info = json.loads(output("go", "list", "-json", package))
        path = directory / f"{index}.test"
        command = [GO, "test", "-vet=off", "-c", "-o", str(path)]
        if mode == "race":
            command.append("-race")
        subprocess.run([*command, package], check=True)
        manifest["packages"][info["ImportPath"]] = {"file": path.name, "sha256": digest(path)}
    temporary = directory / "manifest.tmp"
    temporary.write_text(json.dumps(manifest, sort_keys=True) + "\n")
    temporary.replace(manifest_path)


def resolve(directory, package, race):
    directory = Path(directory).resolve()
    manifest = json.loads((directory / "manifest.json").read_text())
    current = identity()
    if manifest["version"] != 1 or any(manifest[k] != v for k, v in current.items()):
        raise ValueError("shared test binary checkout/toolchain mismatch")
    if manifest["mode"] != ("race" if race else "test"):
        raise ValueError("shared test binary race mode mismatch")
    # go list resolves the package working directory without compiling any code.
    info = json.loads(output("go", "list", "-json", package))
    entry = manifest["packages"][info["ImportPath"]]
    path = directory / entry["file"]
    if path.parent != directory or not path.is_file() or digest(path) != entry["sha256"]:
        raise ValueError("shared test binary checksum mismatch")
    path.chmod(path.stat().st_mode | 0o100)
    return path, info


def test(args):
    directory = os.environ.get("CI_TEST_BIN_DIR")
    if not directory:
        os.execvp(GO, [GO, "test", *args])
    # Only the existing single-package shard command contract is admitted.
    package = args[-1]
    json_output = False
    race = False
    flags = []
    index = 0
    while index < len(args) - 1:
        flag = args[index]
        key, sep, value = flag.partition("=")
        if key == "-json":
            json_output = True
        elif key == "-race":
            race = True
        elif key in ("-count", "-timeout", "-run", "-list"):
            if not sep:
                index += 1
                value = args[index]
            flags.append(f"-test.{key[1:]}={value}")
        elif flag in ("-vet=off", "-p=1"):
            pass
        else:
            raise ValueError(f"unsupported shared binary argument: {flag}")
        index += 1
    path, info = resolve(directory, package, race)
    os.chdir(info["Dir"])
    os.environ["PWD"] = info["Dir"]
    if json_output:
        os.execvp(GO, [GO, "tool", "test2json", "-t", "-p", info["ImportPath"], str(path), "-test.v=test2json", "-test.paniconexit0", *flags])
    result = subprocess.run([str(path), *flags])
    if result.returncode == 0:
        print(f"ok\t{info['ImportPath']}")
    raise SystemExit(result.returncode)


def main():
    command, *args = sys.argv[1:]
    if command == "build":
        build(args[0], args[1], args[2:])
    elif command == "copy-race":
        path, _ = resolve(os.environ["CI_TEST_BIN_DIR"], args[0], True)
        shutil.copyfile(path, args[1])
        Path(args[1]).chmod(0o700)
    elif command == "test":
        test(args)
    else:
        raise ValueError("expected build or test")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print(f"[ci] shared test binary: {error}", file=sys.stderr)
        sys.exit(2)
