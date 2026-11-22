#!/usr/bin/env python3
"""Select fast CodeQL for non-documentation changes from a NUL-delimited diff."""
import importlib.util
import os
import pathlib
import sys

sys.dont_write_bytecode = True
root = pathlib.Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location('race_select', root / 'scripts/ci-pr-race-select.py')
selector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(selector)
data = sys.stdin.buffer.read()
if data and not data.endswith(b'\0'):
    raise ValueError('incomplete NUL-delimited diff')
paths = [os.fsdecode(path) for path in data.split(b'\0') if path]
print(str(not selector.docs_only(paths, root)).lower())
