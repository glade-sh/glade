#!/usr/bin/env python3
"""Select changed Go packages and their direct production or test importers."""
import argparse
import json
import os
import sys

p = argparse.ArgumentParser()
p.add_argument('--root', required=True)
p.add_argument('--graph', required=True)
p.add_argument('--manifest', required=True)
a = p.parse_args()
with open(a.graph, encoding='utf-8') as f:
    graph = json.load(f)
with open(a.manifest, encoding='utf-8') as f:
    manifest = json.load(f)
owned = {pkg for lane in manifest['lanes'].values() for pkg in lane}
packages = {pkg['ImportPath']: pkg for pkg in graph}
if set(packages) != owned:
    sys.exit('package graph does not match the CI lane manifest')
root = os.path.abspath(a.root)
by_dir = {os.path.relpath(pkg['Dir'], root): name for name, pkg in packages.items()}
changed = set()
paths = [line.rstrip('\n') for line in sys.stdin]
if any(path in ('go.mod', 'go.sum') for path in paths):
    changed.update(packages)


def docs_only(path):
    """Explicit docs-only allowlist: docs/** and Markdown files outside testdata."""
    parts = path.split('/')
    if 'testdata' in parts[:-1]:
        return False
    return parts[0] == 'docs' or path.endswith('.md')


for path in paths:
    if not path:
        continue
    directory = os.path.dirname(path)
    mapped = False
    while directory and directory != '.':
        if directory in by_dir:
            changed.add(by_dir[directory])
            mapped = True
            break
        directory = os.path.dirname(directory)
    if mapped or path.startswith('third_party/glade-apex-parser/') or docs_only(path):
        continue
    # Safe by default: a path outside every Go package (lwcruntime/**,
    # third_party/lwc/**, root testdata/**, workflows, root files, ...) may be
    # read by any test through a relative path, so select the full set.
    changed.update(packages)
if any(path.startswith("third_party/glade-apex-parser/") for path in paths):
    changed.update(imp for pkg in packages.values()
                   for imp in pkg.get("Imports", []) + pkg.get("TestImports", []) + pkg.get("XTestImports", [])
                   if imp == "github.com/glade-sh/apex-parser" or imp.startswith("github.com/glade-sh/apex-parser/"))
selected = changed.intersection(packages)
for name, pkg in packages.items():
    imports = set(pkg.get('Imports', []) + pkg.get('TestImports', []) + pkg.get('XTestImports', []))
    if changed.intersection(imports):
        selected.add(name)
print(json.dumps(sorted(selected)))
