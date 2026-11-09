#!/usr/bin/env python3
"""Generate the names-only standard describe catalog lookup."""
import gzip
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1]
storage = root / 'internal/storage'
with gzip.open(storage / 'standard_describe_catalog.json.gz', 'rt') as f:
    names = sorted(json.load(f)['describes'])
out = ['// Code generated from standard_describe_catalog.json.gz for names-only lookup; DO NOT EDIT.', '', 'package storage', '', 'var standardDescribeCatalogObjectNames = []string{']
out += [f'\t"{name}",' for name in names]
out += ['}', '']
(storage / 'standard_describe_catalog_names_generated.go').write_text('\n'.join(out))
