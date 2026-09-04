#!/usr/bin/env python3
"""Create a deterministic bundle, or retain identical uploaded contents on resume."""
import gzip
import hashlib
from pathlib import Path, PurePosixPath
import sys
import tarfile


def digest(stream):
    result = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        result.update(chunk)
    return result.digest()


def main():
    if len(sys.argv) != 3:
        raise SystemExit("usage: release-bundle.py <source-directory> <bundle-outside-source>")
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
