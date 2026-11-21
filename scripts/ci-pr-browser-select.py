#!/usr/bin/env python3
"""Preserve the browser workflow's PR path scope in package selection."""
import fnmatch
import json
from pathlib import Path
import sys
from ci_pr_browser_authorities import browser_paths, expected_tests

selected = json.loads(Path(sys.argv[1]).read_text())
if not isinstance(selected, list) or not all(isinstance(x, str) for x in selected) or len(set(selected)) != len(selected):
    raise SystemExit('invalid selected package list')
paths = [part.decode('utf-8', 'surrogateescape') for part in sys.stdin.buffer.read().split(b'\0')]
patterns = browser_paths()
browser = any(fnmatch.fnmatchcase(path, pattern) for path in paths for pattern in patterns)
if browser:
    selected = sorted(set(selected) | {package for package, _ in expected_tests()})
print(json.dumps({'selected': selected, 'browser': browser}, separators=(',', ':')))
