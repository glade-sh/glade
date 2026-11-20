#!/usr/bin/env python3
"""Select changed Go packages and their transitive reverse-import consumers.

The graph inputs are `go list -json ./...` from the repository root and from
the nested Apex parser module. A changed path maps to its nearest enclosing
package in either module; from there the selector walks reverse production,
internal-test and external-test import edges to a fixed point, so a change
reaches every package that compiles or tests against it directly or
transitively (for example apexversion -> vm -> apextest). A changed
*_test.go file directly in a package directory is an exception: it compiles
only into that package's own test binary, as the internal test package or
the external <P>_test package, so it selects that package alone and never
walks to importers, and a parser module test file selects nothing, because
no root lane executes parser packages. Every other changed path below a
package keeps the package mapping and the reverse walk, including a helper
package's production files, which other packages' tests import, testdata,
and a *_test.go file below a non-package directory such as testdata, which
other packages' tests may read. The nested parser module's packages are
connector nodes: they are never executed by the root module's lanes, but
they connect a parser change to the root packages that import the parser.
A path outside every package, including the files of a deleted, renamed or
removed package, whose former consumers must stay selected, conservatively
selects the full set, as do `go.mod` and `go.sum`. A malformed graph or
lane manifest fails the selection instead of producing an empty successful
one.
"""
import argparse
import json
import os
import sys

PARSER_MODULE = 'github.com/glade-sh/apex-parser'
PARSER_DIR = 'third_party/glade-apex-parser'


def fail(message):
    sys.exit(f'ci-pr-select: {message}')


def load_json(path, what):
    try:
        with open(path, encoding='utf-8') as handle:
            return json.load(handle)
    except (OSError, ValueError) as err:
        fail(f'cannot read the {what} {path}: {err}')


def import_paths(pkg):
    """Production, internal-test and external-test import paths of a package."""
    paths = []
    for key in ('Imports', 'TestImports', 'XTestImports'):
        value = pkg.get(key)
        if value is not None:
            paths.extend(value)
    return paths


def load_packages(path, what, root, restrict_dir=None, module=None):
    """Load one package graph, validated and mapped to repository directories."""
    data = load_json(path, what)
    if not isinstance(data, list):
        fail(f'the {what} is not a JSON list')
    packages = {}
    by_dir = {}
    for entry in data:
        if not isinstance(entry, dict):
            fail(f'a {what} entry is not an object')
        name = entry.get('ImportPath')
        directory = entry.get('Dir')
        if not isinstance(name, str) or not name:
            fail(f'a {what} entry has no ImportPath')
        if not isinstance(directory, str) or not directory:
            fail(f'{what} package {name} has no Dir')
        if name in packages:
            fail(f'the {what} lists {name} twice')
        if module is not None and name != module and not name.startswith(module + '/'):
            fail(f'{what} package {name} is outside the {module} module')
        for key in ('Imports', 'TestImports', 'XTestImports'):
            value = entry.get(key)
            if value is not None and (not isinstance(value, list)
                                      or not all(isinstance(item, str) for item in value)):
                fail(f'{what} package {name} has a malformed {key} list')
        relative = os.path.relpath(directory, root)
        if relative.startswith('..'):
            fail(f'{what} package {name} is outside the repository')
        if restrict_dir is not None and not (relative == restrict_dir
                                             or relative.startswith(restrict_dir + os.sep)):
            fail(f'{what} package {name} is outside {restrict_dir}')
        if relative in by_dir:
            fail(f'the {what} maps directory {relative} twice')
        packages[name] = entry
        by_dir[relative] = name
    if not packages:
        fail(f'the {what} lists no packages')
    return packages, by_dir


def load_manifest(path, packages):
    """Validate the CI lane manifest against the root package graph."""
    data = load_json(path, 'manifest')
    if not isinstance(data, dict) or not isinstance(data.get('lanes'), dict):
        fail('the manifest has no lanes object')
    owned = set()
    for lane, members in data['lanes'].items():
        if not isinstance(members, list) or not all(isinstance(member, str) for member in members):
            fail(f'manifest lane {lane} is not a list of package paths')
        for member in members:
            if member not in packages:
                fail(f'manifest lane {lane} lists unknown package {member}')
            owned.add(member)
    if owned != set(packages):
        fail('package graph does not match the CI lane manifest')


def docs_only(path):
    """Explicit docs-only allowlist: docs/** and Markdown files outside testdata."""
    parts = path.split('/')
    if 'testdata' in parts[:-1]:
        return False
    return parts[0] == 'docs' or path.endswith('.md')


def reverse_consumers(packages, parser_packages, changed, parser_changed, parser_unknown):
    """Packages that import a changed package, directly or transitively.

    Reverse production, internal-test and external-test edges close over both
    modules to a fixed point. The nested parser module's packages are
    connector nodes: they are never returned, but a parser change reaches the
    root packages that import the parser through them.
    """
    if not (changed or parser_changed or parser_unknown):
        return set()
    reverse = {}
    for pkg in list(packages.values()) + list(parser_packages.values()):
        for imported in import_paths(pkg):
            reverse.setdefault(imported, set()).add(pkg['ImportPath'])
    seeds = set(changed) | set(parser_changed)
    if parser_changed or parser_unknown:
        # A parser change also seeds every root package that imports the
        # parser, so an untracked parser input cannot hide a consumer.
        for name, pkg in packages.items():
            if any(imported == PARSER_MODULE or imported.startswith(PARSER_MODULE + '/')
                   for imported in import_paths(pkg)):
                seeds.add(name)
    if parser_unknown:
        seeds |= set(parser_packages)
    seen = set(seeds)
    work = list(seeds)
    while work:
        for consumer in reverse.get(work.pop(), ()):
            if consumer not in seen:
                seen.add(consumer)
                work.append(consumer)
    return seen & set(packages)


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--root', required=True)
    p.add_argument('--graph', required=True)
    p.add_argument('--parser-graph', required=True)
    p.add_argument('--manifest', required=True)
    a = p.parse_args()
    root = os.path.abspath(a.root)
    packages, by_dir = load_packages(a.graph, 'package graph', root)
    parser_packages, parser_by_dir = load_packages(
        a.parser_graph, 'parser graph', root, restrict_dir=PARSER_DIR, module=PARSER_MODULE)
    load_manifest(a.manifest, packages)

    changed = set()
    test_changed = set()
    parser_changed = set()
    parser_unknown = False
    unmapped = []
    full = False
    for path in [line.rstrip('\n') for line in sys.stdin]:
        if not path:
            continue
        if path in ('go.mod', 'go.sum'):
            full = True
            continue
        directory = os.path.dirname(path)
        package_dir = directory in by_dir or directory in parser_by_dir
        mapped = None
        while directory and directory != '.':
            if directory in by_dir:
                mapped = by_dir[directory]
                break
            if directory in parser_by_dir:
                mapped = parser_by_dir[directory]
                break
            directory = os.path.dirname(directory)
        if mapped is not None:
            if package_dir and path.endswith('_test.go'):
                # A *_test.go file directly in a package directory compiles
                # only into that package's own test binary, as the internal
                # test package or the external <P>_test package, so no
                # importer can see the change. A parser module test file
                # reaches no root lane either; the pr-guards worker runs the
                # parser module's own tests whenever any parser path changes.
                if mapped not in parser_packages:
                    test_changed.add(mapped)
                continue
            if mapped in parser_packages:
                parser_changed.add(mapped)
            else:
                changed.add(mapped)
            continue
        if path.startswith(PARSER_DIR + '/'):
            # A parser path outside every parser package (a module manifest or
            # a removed parser package) still reaches every root package that
            # imports the parser.
            parser_unknown = True
            continue
        if path.startswith('.do/') or path.startswith('contrib/vscode-glade/'):
            # These inputs are read by playground and CLI tests respectively.
            owner = ('internal/playground' if path.startswith('.do/')
                     else 'internal/gladecli')
            if owner not in by_dir:
                fail(f'no package for {owner}')
            changed.add(by_dir[owner])
            continue
        if (path.startswith('site/') or path.startswith('.github/ISSUE_TEMPLATE/')
                or path == '.gitattributes' or docs_only(path)):
            continue
        # Safe by default: a path outside every Go package (lwcruntime/**,
        # third_party/lwc/**, root testdata/**, workflows, root files, and the
        # deleted files of a removed or renamed package) may be read by any
        # test through a relative path, and a removed package must not hide
        # its former consumers, so select the full set.
        full = True
        unmapped.append(path)
    if full:
        if unmapped:
            print(f'ci-pr-select: {len(unmapped)} unmapped path(s) select the full set',
                  file=sys.stderr)
        selected = set(packages)
    else:
        selected = reverse_consumers(packages, parser_packages, changed,
                                     parser_changed, parser_unknown)
        # A test-only change still selects the changed package itself, so its
        # own lane runs its tests.
        selected |= test_changed
    print(json.dumps(sorted(selected)))


if __name__ == '__main__':
    main()
