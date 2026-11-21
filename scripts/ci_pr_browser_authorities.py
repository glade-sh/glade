#!/usr/bin/env python3
"""Read browser test authorities from the Browser workflow's validator."""
import ast
import json
from pathlib import Path
import re
import sys

WORKFLOW = Path(__file__).resolve().parent.parent / '.github/workflows/browser.yml'

def validator_source():
    match = re.search(r"(?ms)^          python3 - <<'PY'\n(.*?)^          PY$", WORKFLOW.read_text())
    if not match:
        raise ValueError('browser validator missing')
    return '\n'.join(line[10:] if line.startswith('          ') else line for line in match.group(1).splitlines()) + '\n'

def expected_tests():
    tree = ast.parse(validator_source())
    found = [node.value for node in tree.body if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == 'expected' for target in node.targets)]
    if len(found) != 1:
        raise ValueError('browser authority inventory missing')
    expected = ast.literal_eval(found[0])
    if not isinstance(expected, set) or not expected or not all(isinstance(pair, tuple) and len(pair) == 2 and all(isinstance(item, str) and item for item in pair) for pair in expected):
        raise ValueError('invalid browser authority inventory')
    return expected

def browser_paths():
    workflow = WORKFLOW.read_text()
    matches = re.findall(r'(?m)^\s*(\*\.go\|go\.mod\|[^\n]+)\)\s*$', workflow)
    if len(matches) != 1:
        raise ValueError('browser path scope missing')
    return matches[0].split('|')

def main():
    mode, path = sys.argv[1:]
    selected = json.loads(Path(path).read_text())
    packages = {package for package, _ in expected_tests()}
    if mode == '--has-node':
        packages = {package for package in packages if package.endswith('/internal/lwcruntime')}
        if len(packages) != 1:
            raise ValueError('browser Node package is ambiguous')
    elif mode != '--has-target':
        raise ValueError('unknown mode')
    sys.exit(0 if packages.intersection(selected) else 1)

if __name__ == '__main__':
    main()
