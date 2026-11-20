#!/usr/bin/env python3
"""Intersect affected packages with a source-derived concurrency candidate set.

This is a pilot heuristic, not a complete concurrency inventory. The nightly
and weekly race workflows remain the coverage backstop.
"""
import argparse
import json
import pathlib
import re
import sys

MARKER = re.compile(r'\b(?:chan|sync|atomic|goroutine|mutex|waitgroup|semaphore)\b'
                    r'|\bgo\s+(?:func\s*\(|[A-Za-z_][\w.]*\s*\()'
                    r'|\.Parallel\s*\(', re.IGNORECASE)


def docs_only(paths, root):
    """Take the fast path only where ci-pr-select cannot map a Go package."""
    if not paths:
        return False
    for name in paths:
        parts = name.split('/')
        if len(parts) == 1 and name.endswith('.md'):
            continue
        if parts[0] != 'docs' or 'testdata' in parts[:-1] or name.endswith('.go'):
            return False
        directory = root.joinpath(*parts[:-1])
        while directory != root:
            if directory.is_dir() and any(directory.glob('*.go')):
                return False
            directory = directory.parent
    return True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--graph')
    parser.add_argument('--docs-only', action='store_true')
    parser.add_argument('--root', default='.')
    args = parser.parse_args()
    if args.docs_only:
        print(str(docs_only(sys.stdin.read().splitlines(), pathlib.Path(args.root).resolve())).lower())
        return
    if not args.graph:
        parser.error('--graph is required')
    with open(args.graph, encoding='utf-8') as handle:
        graph = json.load(handle)
    selected = json.load(sys.stdin)
    if not isinstance(graph, list) or not isinstance(selected, list):
        raise ValueError('graph and selection must be lists')
    packages = {}
    for entry in graph:
        if not isinstance(entry, dict) or not isinstance(entry.get('ImportPath'), str) or not isinstance(entry.get('Dir'), str):
            raise ValueError('malformed package graph')
        name = entry['ImportPath']
        if name in packages:
            raise ValueError('duplicate package in graph')
        packages[name] = pathlib.Path(entry['Dir'])
    if not all(isinstance(name, str) and name in packages for name in selected):
        raise ValueError('selection contains an unknown package')
    result = []
    for name in sorted(set(selected)):
        directory = packages[name]
        if not directory.is_dir():
            raise ValueError('package directory missing: ' + name)
        files = list(directory.glob('*.go'))
        if not files:
            raise ValueError('package has no Go files: ' + name)
        if any(MARKER.search(path.read_text(encoding='utf-8')) for path in files):
            result.append(name)
    json.dump(result, sys.stdout)
    print()


if __name__ == '__main__':
    main()
