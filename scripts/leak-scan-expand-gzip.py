#!/usr/bin/env python3
"""Expand scannable gzip blobs in a git diff, leaving all other binaries intact.

Only regular-file blobs with simple Git paths and valid, bounded UTF-8 text are
expanded. The complete new text is added to the scan, not only changed lines.
An unsafe or unsupported blob retains its binary marker and therefore fails the
caller's existing binary check. A supplied external diff is never expanded.
"""

import gzip
import io
import re
import subprocess
import sys
import zlib
from pathlib import Path

MAX_COMPRESSED = 16 * 1024 * 1024
MAX_EXPANDED = 64 * 1024 * 1024
SIMPLE_PATH = re.compile(rb"[A-Za-z0-9_./-]+\Z")
HEADER = re.compile(rb"diff --git a/([A-Za-z0-9_./-]+) b/\1\n\Z")


def git(root, *args):
    return subprocess.check_output(["git", "-C", root, *args], stderr=subprocess.DEVNULL)


def candidates(root, revision):
    if revision.startswith("-"):
        raise ValueError("invalid revision")
    raw = git(root, "diff", "--raw", "--no-abbrev", "--no-renames", "-z", revision, "--", ".")
    fields = raw.split(b"\0")
    result = {}
    for i in range(0, len(fields) - 1, 2):
        metadata, path = fields[i], fields[i + 1]
        parts = metadata.split()
        if len(parts) != 5 or not SIMPLE_PATH.fullmatch(path):
            continue
        _, new_mode, _, new_oid, status = parts
        if new_mode != b"100644" and new_mode != b"100755":
            continue
        if status[:1] not in (b"A", b"M", b"T"):
            continue
        result[path] = new_oid.decode("ascii")
    return result


def expanded_text(root, oid):
    size = int(git(root, "cat-file", "-s", oid))
    if size > MAX_COMPRESSED:
        return None
    blob = git(root, "cat-file", "blob", oid)
    if not blob.startswith(b"\x1f\x8b"):
        return None
    try:
        with gzip.GzipFile(fileobj=io.BytesIO(blob)) as source:
            data = source.read(MAX_EXPANDED + 1)
        if len(data) > MAX_EXPANDED:
            return None
        data.decode("utf-8")
    except (EOFError, OSError, UnicodeDecodeError, ValueError, zlib.error):
        return None
    if any(byte < 32 and byte not in (9, 10, 13) or byte == 127 for byte in data):
        return None
    return data


def scan_section(header, path, data):
    lines = data.split(b"\n")
    if lines and lines[-1] == b"":
        lines.pop()
    prefix = header + b"--- /dev/null\n+++ b/" + path + b"\n"
    if not lines:
        return prefix
    prefix += b"@@ -0,0 +1," + str(len(lines)).encode("ascii") + b" @@\n"
    return prefix + b"".join(b"+" + line + b"\n" for line in lines)


def main():
    if len(sys.argv) != 4:
        return 2
    root, diff_path, revision = sys.argv[1:]
    try:
        changed = candidates(root, revision)
        diff = Path(diff_path).read_bytes()
        sections = re.split(rb"(?m)(?=^diff --git )", diff)
        for i, section in enumerate(sections):
            header = section.split(b"\n", 1)[0] + b"\n"
            match = HEADER.fullmatch(header)
            if not match or b"\nBinary files " not in section:
                continue
            path = match.group(1)
            oid = changed.get(path)
            if oid is None:
                continue
            data = expanded_text(root, oid)
            if data is not None:
                sections[i] = scan_section(header, path, data)
        Path(diff_path).write_bytes(b"".join(sections))
    except (OSError, ValueError, subprocess.CalledProcessError):
        print("leak-scan: unable to inspect compressed changes", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
