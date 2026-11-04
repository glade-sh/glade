#!/usr/bin/env python3
"""Run and check tests in changed Go test files.

Usage: conformance-pairing.py --repo REPO --base BASE --carries FILE
Run the discovered tests with go test -json against the clean committed tree.
Carries is a JSON array of {package, test, count, failures, owner, reason}; failures
are the exact failing leaf test names and count is their number. Every test and @package
record also requires a signature: SHA-256 of compact UTF-8 JSON pairs
[test name, output], in event order. Package output uses an empty test name;
adjacent output from the same name is combined. Old unbound hashes are invalid.
Only an isolated Go result/timing frame corroborated by the immediately following
structured terminal event is omitted. Passing-test output is excluded.
Package signatures cover failed-test and package output; test signatures cover
the failed test and its non-passing subtests.
A failed package needs a count=1 record with test="@package".
"""
import argparse
import fnmatch
import shlex
import ast
import posixpath
import hashlib
import itertools
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

# Remove comments and literals before scanning declarations. Keep newlines and
# token boundaries intact so text inside fixtures cannot create phantom tests.
TEST = re.compile(r'\bfunc\s+((?:Test|Fuzz|Example)\w*)\s*\(', re.UNICODE)


def go_source(source):
    masked = list(source)
    literals = []
    i = 0
    while i < len(source):
        start = i
        if source.startswith('//', i):
            end = source.find('\n', i + 2)
            i = len(source) if end < 0 else end
        elif source.startswith('/*', i):
            end = source.find('*/', i + 2)
            if end < 0:
                raise ValueError('unterminated Go comment')
            i = end + 2
        elif source[i] in ('"', "'", '`'):
            quote = source[i]
            i += 1
            while i < len(source) and source[i] != quote:
                if quote != '`' and source[i] == '\\':
                    i += 1
                i += 1
            if i >= len(source):
                raise ValueError('unterminated Go literal')
            i += 1
            if quote == '`':
                literals.append(source[start + 1:i - 1])
            elif quote == '"':
                try:
                    value = ast.literal_eval(source[start:i])
                    if isinstance(value, str):
                        literals.append(value)
                except (SyntaxError, ValueError):
                    pass
        else:
            i += 1
            continue
        for offset in range(start, i):
            if masked[offset] != '\n':
                masked[offset] = ' '
    return ''.join(masked), literals


def path_join_aliases(source):
    aliases = {'path', 'filepath', 'strings'}
    for match in re.finditer(r'(?:(\w+|\.)\s+)?"(path|path/filepath|strings)"', source):
        alias = match.group(1)
        aliases.add(match.group(2).rsplit('/', 1)[-1] if alias in (None, 'import') else alias)
    return aliases


def package_binding_names(code):
    """Collect names from single and grouped package var/const declarations."""
    names = set(re.findall(r'\b(?:var|const)\s+(\w+)', code))
    for group in re.finditer(r'\b(?:var|const)\s*\(', code):
        # Initializers may contain nested calls or conversions. A closing
        # parenthesis ends the declaration group only at its own depth.
        depth = 1
        end = group.end()
        while end < len(code) and depth:
            depth += (code[end] == '(') - (code[end] == ')')
            end += 1
        if depth:
            raise ValueError('unbalanced package declaration group')
        for declaration in re.finditer(r'(?m)^\s*((?:\w+\s*,\s*)*\w+)(?:\s+[\w.\[\]*]+)?\s*=', code[group.end():end - 1]):
            names.update(re.findall(r'\b\w+\b', declaration.group(1)))
    return names - {'_'}


def embedded_fixture_roots(source, package, fixture):
    roots = set()
    for directive in re.finditer(r'(?m)^\s*//go:embed[ \t]+([^\n]+)', source):
        try:
            patterns = shlex.split(directive.group(1))
        except ValueError:
            raise ValueError('invalid go:embed directive')
        matched = False
        for pattern in patterns:
            pattern = pattern.removeprefix('all:')
            resolved = posixpath.normpath(posixpath.join(package, pattern))
            if fnmatch.fnmatchcase(fixture, resolved) or fixture.startswith(resolved.rstrip('/') + '/'):
                matched = True
                break
        if matched:
            declaration = re.match(r'\s*(?:(?://[^\n]*\n)\s*)*var\s+(\w+)\b', source[directive.end():])
            if not declaration:
                raise ValueError('embedded fixture variable cannot be mapped safely')
            roots.add(declaration.group(1))
    return roots


def fixture_literals(source, join_aliases=None):
    code, literals = go_source(source)
    paths = [value for value in literals if 'testdata/' in value or value.rstrip('/') == 'testdata']
    unknown = []
    suffixes = []
    # Only formatting calls make percent verbs dynamic. Raw strings and ordinary
    # literals may contain scripts, diagnostics, or a literal percent character.
    quoted = r'(?:(?:"(?:\\.|[^"\\])*")|`[^`]*`)'
    aliases = set()
    for imp in re.finditer(r'(?:(\w+|\.)\s+)?"fmt"', source):
        alias = imp.group(1)
        aliases.add('fmt' if alias in (None, 'import') else alias)
    selectors = [r'\b' + re.escape(alias) + r'\s*\.\s*(?:Sprintf|Errorf)\s*\('
                 for alias in aliases if alias != '.']
    if '.' in aliases:
        selectors.append(r'\b(?:Sprintf|Errorf)\s*\(')
    for match in re.finditer('|'.join(selectors) if selectors else r'(?!)', code):
        argument = re.match(r'\s*(' + quoted + r')', source[match.end():])
        if not argument:
            continue
        _, values = go_source(argument.group(1))
        for value in values:
            if ('testdata' in value and re.search(r'%(?:\[[0-9]+\]|[-+#0-9.*])*[A-Za-z]', value.replace('%%', ''))
                    and (not re.search(r'\s', value) or re.match(r'^(?:testdata/|\.{1,2}/|/|%)', value))):
                prefix = value.split('%', 1)[0]
                unknown.append(prefix if 'testdata' in prefix else None)
    # A format may be stored before use (including in a different source
    # file). Track simple bindings to fixed point and fail closed when such a
    # value reaches a formatting call.
    bindings = {}
    quoted_expr = quoted + r'(?:\s*\+\s*' + quoted + r')*'
    assignments = (
        re.compile(r'\b([A-Za-z_]\w*)\s*(?::=|=)\s*(' + quoted_expr + r')'),
        re.compile(r'\b(?:var|const)\s+([A-Za-z_]\w*)\s+[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?\s*=\s*(' + quoted_expr + r')'),
        re.compile(r'(?m)^\s*([A-Za-z_]\w*)\s+[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?\s*=\s*(' + quoted_expr + r')'),
    )
    for assignment in assignments:
        for binding in assignment.finditer(source):
            _, values = go_source(binding.group(2))
            if values:
                bindings.setdefault(binding.group(1), set()).add(''.join(values))
    forwarding = re.compile(r'\b([A-Za-z_]\w*)\s*(?::=|=)\s*([A-Za-z_]\w*)\b')
    while True:
        changed = False
        for binding in forwarding.finditer(code):
            values = bindings.get(binding.group(2), set())
            if values:
                target = bindings.setdefault(binding.group(1), set())
                old = len(target)
                target.update(values)
                changed |= len(target) != old
        if not changed:
            break
    for match in re.finditer('|'.join(selectors) if selectors else r'(?!)', code):
        arg = re.match(r'\s*([A-Za-z_]\w*)\b', code[match.end():])
        for value in bindings.get(arg.group(1), set()) if arg else ():
            if 'testdata' in value and re.search(r'%(?:\[[0-9]+\]|[-+#0-9.*])*[A-Za-z]', value.replace('%%', '')):
                if not re.search(r'\s', value) or re.match(r'^(?:testdata/|\.{1,2}/|/|%)', value):
                    prefix = value.split('%', 1)[0]
                    unknown.append(prefix if 'testdata' in prefix else None)
    # Formatting functions can themselves be bound and forwarded.
    format_functions = set()
    for alias in aliases:
        selector = (r'(?:Sprintf|Errorf)' if alias == '.' else
                    re.escape(alias) + r'\s*\.\s*(?:Sprintf|Errorf)')
        format_functions.update(m.group(1) for m in re.finditer(
            r'\b([A-Za-z_]\w*)\s*(?::=|=)\s*' + selector + r'\b', code))
    while True:
        added = {m.group(1) for m in forwarding.finditer(code) if m.group(2) in format_functions}
        if added <= format_functions:
            break
        format_functions.update(added)
    for name in format_functions:
        for call in re.finditer(r'\b' + re.escape(name) + r'\s*\(', code):
            literal = re.match(r'\s*(' + quoted_expr + r')', source[call.end():])
            if literal:
                _, values = go_source(literal.group(1))
                values = [''.join(values)]
            else:
                arg = re.match(r'\s*([A-Za-z_]\w*)\b', code[call.end():])
                values = bindings.get(arg.group(1), set()) if arg else ()
            for value in values:
                if ('testdata' in value and re.search(r'%(?:\[[0-9]+\]|[-+#0-9.*])*[A-Za-z]', value.replace('%%', ''))
                        and (not re.search(r'\s', value) or re.match(r'^(?:testdata/|\.{1,2}/|/|%)', value))):
                    prefix = value.split('%', 1)[0]
                    unknown.append(prefix if 'testdata' in prefix else None)
    # A path-valued format can place the fixture root in a later argument.
    # Its exact dynamic root cannot generally be recovered, so fail closed.
    call_patterns = selectors + [r'\b' + re.escape(name) + r'\s*\(' for name in format_functions]
    for call in re.finditer('|'.join(call_patterns) if call_patterns else r'(?!)', code):
        stack = 1
        end = call.end()
        while end < len(code) and stack:
            stack += (code[end] == '(') - (code[end] == ')')
            end += 1
        if stack:
            continue
        fragment = source[call.end():end - 1]
        fragment_code, values = go_source(fragment)
        if re.match(r'\s*' + quoted, fragment):
            formats, components = values[:1], values[1:]
            format_name = None
        else:
            name = re.match(r'\s*([A-Za-z_]\w*)\b', fragment_code)
            format_name = name.group(1) if name else None
            formats = bindings.get(format_name, set()) if format_name else ()
            components = values
        possible = list(components)
        for name in re.findall(r'\b[A-Za-z_]\w*\b', fragment_code):
            if name != format_name:
                possible.extend(bindings.get(name, ()))
        for fmt in formats:
            if ('%' not in fmt or re.search(r'\s', fmt) or not ('/' in fmt or fmt.startswith('%'))):
                continue
            arguments = iter(components)
            rendered = re.sub(r'%(?:\[[0-9]+\]|[-+#0-9.*])*[A-Za-z]',
                              lambda _: next(arguments, ''), fmt)
            if any('testdata' in value for value in possible) or ('testdata' in rendered and 'testdata' not in fmt):
                unknown.append(None)
    # Constant prefixes are relative to the package; if a variable precedes
    # testdata, its literal suffix may refer to that root in any package.
    path_aliases = path_join_aliases(source) | (join_aliases or set())
    join_selectors = [r'\b' + re.escape(alias) + r'\s*\.\s*Join\s*\(' for alias in path_aliases if alias != '.']
    if '.' in path_aliases:
        join_selectors.append(r'\bJoin\s*\(')
    for match in re.finditer('|'.join(join_selectors), code):
        # Split only at top-level commas; argument expressions may contain calls,
        # indexes, composite literals and nested Join expressions.
        stack = []
        start = match.end()
        arguments = []
        end = None
        for index in range(start, len(code)):
            char = code[index]
            if char in '([{':
                stack.append(char)
            elif char in ')]}':
                if char == ')' and not stack:
                    arguments.append(source[start:index])
                    end = index
                    break
                if not stack or stack.pop() != {')': '(', ']': '[', '}': '{'}[char]:
                    break
            elif char == ',' and not stack:
                arguments.append(source[start:index])
                start = index + 1
        if end is None:
            continue
        # Nested and variadic joins can hide a composed fixture root. Where
        # the complete path cannot be mapped, refuse to silently omit it.
        variadic_join = any('...' in arg for arg in arguments)
        complex_join = variadic_join or any(re.search(r'\b\w+\s*\.\s*Join\s*\(', arg)
                                            for arg in arguments)
        if complex_join:
            composed = any('+' in m.group() and 'testdata' in ''.join(go_source(m.group())[1])
                           for m in re.finditer(quoted_expr, source))
            if composed or (variadic_join and (any('testdata' in value for value in literals) or
                    any('testdata' in value for values in bindings.values() for value in values))):
                unknown.append(None)
        # Resolve constant or simply bound components before falling back to
        # suffix-based matching for paths with an unknown prefix.
        choices = []
        component = r'(?:' + quoted + r'|[A-Za-z_]\w*)'
        for argument in arguments:
            argument = argument.strip()
            if not re.fullmatch(component + r'(?:\s*\+\s*' + component + r')*', argument):
                choices.append(())
                continue
            values = ['']
            for piece in re.finditer(component, argument):
                value = piece.group()
                if value.startswith(('"', '`')):
                    _, options = go_source(value)
                else:
                    options = bindings.get(value, ())
                if not options:
                    values = []
                    break
                values = [left + right for left, right in itertools.product(values, options)]
                if len(values) > 256:
                    values = []
                    break
            choices.append(values)
        # Preserve the fail-closed rule for a dynamic immediate child of the
        # fixture root: a variable's observed initializer is not proof that it
        # cannot change before this call.
        first = arguments[0].strip() if arguments else ''
        if (re.fullmatch(quoted, first) and go_source(first)[1] == ['testdata']
                and any(not re.fullmatch(quoted, arg.strip()) for arg in arguments[1:])):
            choices = []
        if choices and all(choices):
            count = 1
            for values in choices:
                count *= len(values)
            if count > 256:
                unknown.append(None)
            else:
                paths.extend(value for value in (posixpath.join(*parts) for parts in itertools.product(*choices))
                             if 'testdata' in value)
            continue
        parts = []
        for argument in arguments:
            masked, values = go_source(argument)
            parts.append(values[0] if not masked.strip() and len(values) == 1 else None)
        marker = next((i for i, part in enumerate(parts)
                       if part is not None and 'testdata' in part.split('/')), None)
        if marker is None:
            continue
        prefix = parts[:marker]
        suffix = []
        for part in parts[marker:]:
            if part is None:
                break
            suffix.append(part)
        if any(part is None for part in prefix):
            suffix_parts = posixpath.normpath(posixpath.join(*suffix)).split('/')
            if 'testdata' in suffix_parts:
                suffixes.append('/'.join(suffix_parts[suffix_parts.index('testdata'):]))
            else:
                unknown.append(None)
        else:
            value = posixpath.join(*(prefix + suffix))
            normalized = posixpath.normpath(value)
            if len(suffix) < len(parts) - marker and normalized.split('/')[-1] == 'testdata':
                unknown.append(value)
            else:
                paths.append(value)
    # Resolve literal and bound pieces in concatenations. If a fixture-bearing
    # expression has an unknown piece, fail closed instead of guessing a root.
    token = quoted + r'|[A-Za-z_]\w*'
    expression = r'(?:' + token + r')(?:\s*\+\s*(?:' + token + r'))+'
    for match in re.finditer(expression, source):
        masked = code[match.start():match.end()]
        if '+' not in masked:
            continue
        candidates = ['']
        unresolved = False
        for part in re.finditer(token, match.group()):
            value = part.group()
            if value.startswith(('"', '`')):
                _, literals = go_source(value)
                values = literals
            else:
                values = bindings.get(value, ())
            if not values:
                unresolved = True
                continue
            candidates = [left + right for left, right in itertools.product(candidates, values)]
            if len(candidates) > 256:
                unresolved = True
                break
        if unresolved:
            if 'testdata' in match.group() or any('testdata' in value for value in candidates):
                unknown.append(None)
        else:
            paths.extend(value for value in candidates if 'testdata' in value)

    # strings.Join over literal slices (including simple slice bindings) is a
    # common equivalent to path concatenation.
    slice_expr = r'\[\]\s*string\s*\{([^{}]*)\}'
    slices = {}
    for binding in re.finditer(r'\b(\w+)\s*(?::=|=)\s*' + slice_expr, source):
        slices.setdefault(binding.group(1), []).append(binding.group(2))
    aliases = path_join_aliases(source) | (join_aliases or set())
    calls = [r'\b' + re.escape(alias) + r'\s*\.\s*Join' for alias in aliases if alias != '.']
    if '.' in aliases:
        calls.append(r'\bJoin')
    join = re.compile('(?:' + '|'.join(calls) + r')\s*\(\s*(' + slice_expr + r'|[A-Za-z_]\w*)\s*,\s*(' + quoted + r')\s*\)', re.S)
    for match in join.finditer(source):
        if not code[match.start():match.start() + 1].strip():
            continue
        first = match.group(1)
        literal_slice = re.fullmatch(slice_expr, first, re.S)
        options = [literal_slice.group(1)] if literal_slice else slices.get(first, [])
        _, separators = go_source(match.group(3))
        if not separators:
            continue
        for option in options:
            parts = []
            for item in option.split(','):
                item = item.strip()
                if not item:
                    continue
                masked, literals = go_source(item)
                if not masked.strip() and len(literals) == 1:
                    parts.append(literals)
                elif re.fullmatch(r'[A-Za-z_]\w*', item) and bindings.get(item):
                    parts.append(bindings[item])
                else:
                    parts.append(())
            if not parts or any(not values for values in parts):
                if 'testdata' in option:
                    unknown.append(None)
                continue
            count = 1
            for values in parts:
                count *= len(values)
            if count > 256:
                unknown.append(None)
                continue
            paths.extend(value for value in (separators[0].join(p) for p in itertools.product(*parts)) if 'testdata' in value)
    return paths, unknown, suffixes


def functions(source):
    """Return lexical Go function bodies; identifier edges overapproximate calls."""
    code, _ = go_source(source)
    declaration = re.compile(r'\bfunc\s+(?:\([^)]*\)\s*)?(\w+)\s*\(')
    result = []
    for match in declaration.finditer(code):
        # Skip the parameter list and nested return types. A brace following
        # struct or interface introduces a type, not the function body.
        stack = ['(']
        index = match.end()
        start = None
        while index < len(code):
            char = code[index]
            if char in '([':
                stack.append(char)
            elif char in ')]':
                if stack and stack[-1] == {')': '(', ']': '['}[char]:
                    stack.pop()
            elif char == '{':
                if stack or re.search(r'\b(?:struct|interface)\s*$', code[match.end():index]):
                    stack.append('{')
                else:
                    start = index
                    break
            elif char == '}' and stack and stack[-1] == '{':
                stack.pop()
            index += 1
        if start is None:
            continue
        depth = 1
        end = start + 1
        while end < len(code) and depth:
            depth += (code[end] == '{') - (code[end] == '}')
            end += 1
        if depth:
            raise ValueError('unbalanced function body')
        result.append((match.group(1), source[match.start():end], code[start:end]))
    return result


def imports(source, module, package_name):
    """Return local import aliases and their repository package paths."""
    result = {}
    pattern = re.compile(r'\bimport\s*(?:\(([^)]*)\)|([^\n;]+))', re.S)
    for statement in pattern.finditer(source):
        block = statement.group(1) if statement.group(1) is not None else statement.group(2)
        for item in re.finditer(r'(?:(\w+|\.)\s+)?"([^"\n]+)"', block):
            alias, path = item.group(1), item.group(2)
            if path == module:
                package = '.'
            elif path.startswith(module + '/'):
                package = path[len(module) + 1:]
            else:
                continue
            alias = alias or package_name(package)
            if alias != '_':
                result[alias] = package
    return result


def external_callers(repo, head, roots):
    """Follow references to exported fixture helpers through local imports."""
    match = re.search(r'^module\s+(\S+)', git(repo, 'show', head + ':go.mod').decode(), re.M)
    if not match:
        raise ValueError('cannot identify root module')
    module = match.group(1)
    known = {package: set(names) for package, names in roots.items()}
    cache = {}
    package_names = {}
    package_sources = {}
    method_cache = {}
    dependency_cache = {}
    def sources_in(package):
        if package not in package_sources:
            tree = git(repo, 'ls-tree', '-r', '--name-only', '-z', head, '--', package).decode().split('\0')
            package_sources[package] = [git(repo, 'show', head + ':' + path).decode() for path in tree
                                        if path.endswith('.go') and (posixpath.dirname(path) or '.') == package]
        return package_sources[package]
    def methods(package):
        if package not in method_cache:
            method_cache[package] = set()
            for source in sources_in(package):
                code, _ = go_source(source)
                method_cache[package].update(re.findall(r'\bfunc\s*\([^)]*\)\s*(\w+)\s*\(', code))
        return method_cache[package]
    def package_name(package):
        if package not in package_names:
            tree = git(repo, 'ls-tree', '-r', '--name-only', '-z', head, '--', package).decode().split('\0')
            for path in tree:
                if not path.endswith('.go') or path.endswith('_test.go') or (posixpath.dirname(path) or '.') != package:
                    continue
                source = git(repo, 'show', head + ':' + path).decode()
                code, _ = go_source(source)
                declaration = re.search(r'\bpackage\s+(\w+)', code)
                if declaration:
                    package_names[package] = declaration.group(1)
                    break
            if package not in package_names:
                package_names[package] = package.rsplit('/', 1)[-1]
        return package_names[package]
    selected = set()
    while True:
        search_terms = {name for names in known.values() for name in names if name}
        if not search_terms:
            break
        command = ['git', '-C', str(repo), 'grep', '-l', '-z', '-F']
        for name in sorted(search_terms):
            command += ['-e', name]
        command += [head, '--', '*.go']
        result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode not in (0, 1):
            raise ValueError('fixture caller search failed')
        changed = False
        for entry in result.stdout.decode().split('\0'):
            if not entry:
                continue
            _, path = entry.split(':', 1)
            package = posixpath.dirname(path) or '.'
            if path not in cache:
                source = git(repo, 'show', head + ':' + path).decode()
                cache[path] = (source, functions(source), imports(source, module, package_name),
                               test_names(source) if path.endswith('_test.go') else set())
            source, declarations, aliases, tests = cache[path]
            if package not in dependency_cache:
                dependencies = set()
                for package_source in sources_in(package):
                    dependencies.update(imports(package_source, module, package_name).values())
                dependency_cache[package] = dependencies
            def refers(body):
                identifiers = set(re.findall(r'\b[A-Za-z_]\w*\b', body))
                if identifiers & known.get(package, set()):
                    return True
                for alias, imported in aliases.items():
                    for symbol in known.get(imported, set()):
                        if not symbol or not symbol[0].isupper():
                            continue
                        if alias == '.':
                            if symbol in identifiers:
                                return True
                        elif re.search(r'\b' + re.escape(alias) + r'\s*\.\s*' + re.escape(symbol) + r'\b', body):
                            return True
                # A receiver may have a local name, and its type import may
                # occur in another file of the same Go package. Keep method
                # matching conservative for imported fixture-bearing packages.
                for imported in dependency_cache[package]:
                    for symbol in known.get(imported, set()) & methods(imported):
                        if symbol and symbol[0].isupper() and re.search(r'\.\s*' + re.escape(symbol) + r'\b', body):
                            return True
                return False
            names = known.setdefault(package, set())
            for name, _, body in declarations:
                if name not in names and refers(body):
                    names.add(name)
                    changed = True
                if name in names and name in tests:
                    selected.add((package, name))
            code, _ = go_source(source)
            # If a file refers to a known fixture symbol, multiline or nested
            # initializers may bind it to any declared package name. Follow
            # those names conservatively instead of truncating at newlines.
            if refers(code):
                additions = package_binding_names(code) - names
                if additions:
                    names.update(additions)
                    changed = True
            # Multiple declarations can bind the fixture result to any of
            # their names; conservatively follow every name on the left side.
            multi = (r'\b(?:var|const)\s+((?:\w+\s*,\s*)+\w+)(?:\s+[\w.\[\]*]+)?\s*=([^\n;]+)',
                     r'(?m)^\s*((?:\w+\s*,\s*)+\w+)(?:\s+[\w.\[\]*]+)?\s*=([^\n;]+)')
            for pattern in multi:
                for binding in re.finditer(pattern, code):
                    if refers(binding.group(2)):
                        for name in re.findall(r'\b\w+\b', binding.group(1)):
                            if name not in names and name != '_':
                                names.add(name)
                                changed = True
            bindings = (r'\b(?:var|const)\s+(\w+)(?:\s+[\w.\[\]*]+)?\s*=([^\n;]+)',
                        r'(?m)^\s*(\w+)(?:\s+[\w.\[\]*]+)?\s*=([^\n;]+)')
            for pattern in bindings:
                for binding in re.finditer(pattern, code):
                    if binding.group(1) not in names and refers(binding.group(2)):
                        names.add(binding.group(1))
                        changed = True
        if not changed:
            break
    return selected

def test_names(source):
    code, _ = go_source(source)
    names = set()
    for match in TEST.finditer(code):
        name = match.group(1)
        prefix = 'Example' if name.startswith('Example') else ('Fuzz' if name.startswith('Fuzz') else 'Test')
        if len(name) > len(prefix) and name[len(prefix)].islower():
            continue
        if name == 'TestMain' and re.match(r'\s*(?:\w+\s+)?\*\s*testing\s*\.\s*M\b', code[match.end():]):
            continue
        if prefix == 'Example':
            examples = {n: fragment for n, fragment, _ in functions(source) if n.startswith('Example')}
            if not re.search(r'(?im)^\s*//\s*(?:unordered\s+)?output\s*:', examples.get(name, '')):
                continue
        names.add(name)
    return names


def git(repo, *args, check=True):
    return subprocess.run(['git', '-C', str(repo), *args], check=check,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, required=True)
    parser.add_argument('--base', required=True)
    parser.add_argument('--go', default='go', help='Go executable')
    parser.add_argument('--carries', type=Path, required=True)
    args = parser.parse_args()
    head = git(args.repo, 'rev-parse', 'HEAD').decode().strip()
    base = git(args.repo, 'rev-parse', '--verify', args.base + '^{commit}').decode().strip()
    if git(args.repo, 'status', '--porcelain', '--untracked-files=all'):
        raise ValueError('worktree must be clean')
    git(args.repo, 'merge-base', '--is-ancestor', base, head)
    # --no-renames includes both old and new names, including deletions.
    paths = git(args.repo, 'diff', '--no-renames', '--name-only', '--diff-filter=ACDMRT', '-z', base, head).decode().split('\0')
    selected = set()
    changed_helpers = {}
    setup_packages = set()
    for path in paths:
        if not path.endswith('_test.go'):
            continue
        result = subprocess.run(['git', '-C', str(args.repo), 'show', head + ':' + path],
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode:
            continue  # Removed test files cannot execute at HEAD.
        source = result.stdout.decode()
        package = str(Path(path).parent)
        names = test_names(source)
        funcs = functions(source)
        if any(name in ('TestMain', 'init') for name, _, _ in funcs):
            setup_packages.add(package)
        remaining = source
        for _, fragment, _ in funcs:
            remaining = remaining.replace(fragment, ' ' * len(fragment), 1)
        code, _ = go_source(remaining)
        if re.search(r'\b(?:var|const)\b', code):
            setup_packages.add(package)
        changed_helpers.setdefault(package, set()).update(name for name, _, _ in functions(source) if name not in names)
        for name in names:
            selected.add((str(Path(path).parent), name))
    for package in setup_packages:
        tree = git(args.repo, 'ls-tree', '-r', '--name-only', '-z', head, '--', package).decode().split('\0')
        for candidate in tree:
            if candidate.endswith('_test.go') and (posixpath.dirname(candidate) or '.') == package:
                source = git(args.repo, 'show', head + ':' + candidate).decode()
                selected.update((package, name) for name in test_names(source))
    # A helper in a changed test file can be called by tests in other files.
    # Follow package-local identifier references to a fixed point.
    for package, roots in changed_helpers.items():
        if not roots:
            continue
        tree = git(args.repo, 'ls-tree', '-r', '--name-only', '-z', head, '--', package).decode().split('\0')
        declarations = []
        for candidate in tree:
            if not candidate.endswith('_test.go') or (posixpath.dirname(candidate) or '.') != package:
                continue
            source = git(args.repo, 'show', head + ':' + candidate).decode()
            names = test_names(source)
            declarations.extend((name, body, name in names) for name, _, body in functions(source))
        while True:
            expanded = roots | {name for name, body, _ in declarations
                                if set(re.findall(r'\b[A-Za-z_]\w*\b', body)) & roots}
            if expanded == roots:
                break
            roots = expanded
        selected.update((package, name) for name, _, is_test in declarations if is_test and name in roots)
    # Resolve fixture consumers across packages by explicit relative path or
    # directory literals. Never guess from a test's name. Unknown or dynamic
    # relationships fail closed and require a discoverable fixture reference.
    sources = {}
    package_formats = {}
    package_cross_file = {}
    for path in paths:
        parts = path.split('/')
        if 'testdata' not in parts:
            continue
        idx = parts.index('testdata')
        if idx + 1 >= len(parts):
            raise ValueError(f'{path}: fixture path is not discoverable')
        # Include the operators understood by fixture_literals: every path
        # component may be split, so no individual fixture token is required
        # to occur verbatim in the consuming file.
        needles = set(parts[idx + 1:]) | {'testdata', '+', 'Join', 'Sprintf', 'Errorf'}
        command = ['git', '-C', str(args.repo), 'grep', '-l', '-z', '-F']
        for needle in sorted(needles):
            command += ['-e', needle]
        command += [head, '--', '*.go']
        found = set()
        matched_packages = set()
        aggregate_packages = set()
        result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode not in (0, 1):
            raise ValueError('fixture consumer search failed')
        for entry in result.stdout.decode().split('\0'):
            if not entry:
                continue
            _, candidate = entry.split(':', 1)
            if candidate not in sources:
                source = git(args.repo, 'show', head + ':' + candidate).decode()
                sources[candidate] = (test_names(source) if candidate.endswith('_test.go') else set(), *fixture_literals(source))
            names, literals, unknown, suffixes = sources[candidate]
            package = posixpath.dirname(candidate) or '.'
            source = git(args.repo, 'show', head + ':' + candidate).decode()
            if embedded_fixture_roots(source, package, path):
                matched_packages.add(package)
            if package not in package_formats:
                tree = git(args.repo, 'ls-tree', '-r', '--name-only', '-z', head, '--', package).decode().split('\0')
                files = [git(args.repo, 'show', head + ':' + f).decode() for f in tree
                         if f.endswith('.go') and (posixpath.dirname(f) or '.') == package]
                package_formats[package] = fixture_literals('\n'.join(files))
                codes = [go_source(source)[0] for source in files]
                bindings = [package_binding_names(code) for code in codes]
                package_cross_file[package] = any(
                    re.search(r'\+|\b(?:Join|Sprintf|Errorf)\s*\(', code) and
                    any(re.search(r'\b' + re.escape(name) + r'\b', code)
                        for j, names in enumerate(bindings) if i != j for name in names)
                    for i, code in enumerate(codes))
            package_literals, package_unknown, package_suffixes = package_formats[package]
            unknown = unknown + package_unknown
            for root in unknown:
                resolved_root = posixpath.normpath(posixpath.join(package, root)) if root is not None else None
                if resolved_root is None or path == resolved_root or path.startswith(resolved_root.rstrip('/') + '/'):
                    raise ValueError(f'{candidate}: dynamic fixture root cannot be mapped safely')
            fixture_suffix = '/'.join(parts[idx:])
            for literal in package_literals:
                resolved = posixpath.normpath(posixpath.join(package, literal))
                if 'testdata' in resolved.split('/') and (path == resolved or path.startswith(resolved.rstrip('/') + '/')):
                    aggregate_packages.add(package)
            if any(fixture_suffix == suffix or fixture_suffix.startswith(suffix.rstrip('/') + '/')
                   for suffix in package_suffixes):
                aggregate_packages.add(package)
            for suffix in suffixes:
                if fixture_suffix == suffix or fixture_suffix.startswith(suffix.rstrip('/') + '/'):
                    matched_packages.add(package)
                    found.update((package, name) for name in names)
                    break
            for literal in literals:
                if 'testdata/' not in literal and literal.rstrip('/') != 'testdata' and not literal.rstrip('/').endswith('/testdata'):
                    continue
                resolved = posixpath.normpath(posixpath.join(package, literal))
                resolved_parts = resolved.split('/')
                if 'testdata' not in resolved_parts:
                    continue
                if path == resolved or path.startswith(resolved.rstrip('/') + '/'):
                    matched_packages.add(package)
                    found.update((package, name) for name in names)
                    break
        aggregate_only = (aggregate_packages - matched_packages) | {
            package for package in aggregate_packages if package_cross_file[package]}
        matched_packages.update(aggregate_packages)
        if not matched_packages:
            raise ValueError(f'{path}: changed fixture has no discoverable test consumer')
        # Traverse reverse lexical references across all files in each matched
        # package. Identifier references include helper values and methods, so
        # ambiguous names intentionally select all same-named declarations.
        found = set()
        package_roots = {}
        for package in matched_packages:
            tree = git(args.repo, 'ls-tree', '-r', '--name-only', '-z', head, '--', package).decode().split('\0')
            declarations = []
            roots = set()
            fallback = False
            for candidate in tree:
                if not candidate.endswith('.go') or (posixpath.dirname(candidate) or '.') != package:
                    continue
                source = git(args.repo, 'show', head + ':' + candidate).decode()
                funcs = functions(source)
                roots.update(embedded_fixture_roots(source, package, path))
                aliases = path_join_aliases(source)
                declarations.extend((name, body, name in test_names(source) if candidate.endswith('_test.go') else False) for name, _, body in funcs)
                if candidate not in sources:
                    sources[candidate] = (test_names(source) if candidate.endswith('_test.go') else set(), *fixture_literals(source))
                _, literals, unknown, suffixes = sources[candidate]
                def references_fixture(fragment):
                    literals, unknown, suffixes = fixture_literals(fragment, aliases)
                    for literal in literals:
                        resolved = posixpath.normpath(posixpath.join(package, literal))
                        if path == resolved or path.startswith(resolved.rstrip('/') + '/'):
                            return True
                    fixture_suffix = '/'.join(parts[idx:])
                    return any(fixture_suffix == suffix or fixture_suffix.startswith(suffix.rstrip('/') + '/') for suffix in suffixes)
                if references_fixture(source):
                    local_roots = {name for name, fragment, _ in funcs if references_fixture(fragment)}
                    roots.update(local_roots)
                    remaining = source
                    for _, fragment, _ in funcs:
                        remaining = remaining.replace(fragment, ' ' * len(fragment), 1)
                    # Trace every declared name on a fixture-bearing line,
                    # including grouped, typed and multi-name declarations.
                    code, _ = go_source(remaining)
                    pattern = (r'(?m)(?:\b(?:var|const)\s+(?:\(\s*)?|^\s*)'
                               r'((?:[A-Za-z_]\w*\s*,\s*)*[A-Za-z_]\w*)'
                               r'(?:\s+[\w.\[\]*]+)?\s*=([^\n;]+)')
                    for binding in re.finditer(pattern, code):
                        fragment = remaining[binding.start():binding.end()]
                        try:
                            matches = references_fixture(fragment)
                        except ValueError:
                            continue
                        if matches:
                            roots.update(name for name in re.findall(r'\b\w+\b', binding.group(1)) if name != '_')
                            remaining = remaining[:binding.start()] + ' ' * (binding.end() - binding.start()) + remaining[binding.end():]
                    if references_fixture(remaining):
                        # Unrecognized declarations may export their fixture
                        # value. Conservatively follow every exported identifier.
                        code, _ = go_source(remaining)
                        roots.update(re.findall(r'\b[A-Z]\w*\b', code))
                        fallback = True
            if fallback:
                roots.update(name for name, _, is_test in declarations if is_test)
            if package in aggregate_only:
                # The path is assembled across files, so its exact defining
                # expression cannot be attributed to one file. Conservatively
                # trace all package declarations and run its test entry points.
                roots.update(name for name, _, _ in declarations)
                for candidate in tree:
                    if candidate.endswith('.go') and (posixpath.dirname(candidate) or '.') == package:
                        code, _ = go_source(git(args.repo, 'show', head + ':' + candidate).decode())
                        roots.update(package_binding_names(code))
            changed = True
            while changed:
                changed = False
                for name, body, _ in declarations:
                    identifiers = set(re.findall(r'\b[A-Za-z_]\w*\b', body))
                    if name not in roots and identifiers & roots:
                        roots.add(name)
                        changed = True
            package_roots[package] = roots
            for name, _, is_test in declarations:
                if is_test and name in roots:
                    found.add((package, name))
        found.update(external_callers(args.repo, head, package_roots))
        if not found:
            raise ValueError(f'{path}: fixture consumer package has no discoverable tests')
        selected.update(found)

    module = None
    if any(package == '.' for package, _ in selected):
        match = re.search(r'^module\s+(\S+)', git(args.repo, 'show', head + ':go.mod').decode(), re.M)
        if not match:
            raise ValueError('cannot identify root module')
        module = match.group(1)

    def in_package(event, package):
        value = event.get('Package')
        return isinstance(value, str) and (value == module if package == '.' else value.endswith('/' + package))

    def signature(package, test=None):
        chunks = []
        passing = {event['Test'] for event in events if isinstance(event, dict)
                   and in_package(event, package) and event.get('Action') == 'pass'
                   and isinstance(event.get('Test'), str)}
        for index, event in enumerate(events):
            if not isinstance(event, dict) or event.get('Action') != 'output' or not in_package(event, package):
                continue
            name = event.get('Test')
            if test is not None and (not isinstance(name, str) or not (name == test or name.startswith(test + '/'))):
                continue
            if isinstance(name, str) and name in passing:
                continue
            output = event.get('Output')
            if not isinstance(output, str):
                raise ValueError('invalid output event')
            # cmd/test2json emits the harness result line immediately before
            # its structured terminal event. Do not classify any other line by
            # appearance: test diagnostics may reproduce framing verbatim.
            successor = events[index + 1] if index + 1 < len(events) else None
            if isinstance(successor, dict) and in_package(successor, package):
                action = successor.get('Action')
                if (isinstance(name, str) and successor.get('Test') == name
                        and action in ('pass', 'fail', 'skip')):
                    match = re.fullmatch(r' *--- (PASS|FAIL|SKIP): (\S+) \(([0-9.]+)s\)\r?\n', output)
                    elapsed = successor.get('Elapsed')
                    if (match and match.group(1).lower() == action and match.group(2) == name
                            and type(elapsed) in (int, float) and float(match.group(3)) == elapsed):
                        continue
                if not name and not successor.get('Test') and action in ('pass', 'fail'):
                    prefix = 'FAIL' if action == 'fail' else 'ok'
                    if re.fullmatch(re.escape(prefix + '\t' + event['Package'] + '\t') + r'[0-9.]+s\r?\n', output):
                        continue
            if output:
                owner = name if isinstance(name, str) else ''
                if chunks and chunks[-1][0] == owner:
                    chunks[-1][1] += output
                else:
                    chunks.append([owner, output])
        if not chunks:
            return None
        encoded = json.dumps(chunks, ensure_ascii=False, separators=(',', ':')).encode('utf-8')
        return hashlib.sha256(encoded).hexdigest()

    carries = json.loads(args.carries.read_text())
    if not isinstance(carries, list):
        raise ValueError('carries must be an array')
    records = {}
    for record in carries:
        if not isinstance(record, dict):
            raise ValueError('invalid carry fields')
        basic = {'package', 'test', 'count', 'owner', 'reason'}
        if not basic.issubset(record):
            raise ValueError('invalid carry fields')
        name, count = (record['package'], record['test']), record['count']
        if (not all(isinstance(part, str) and part.strip() for part in name) or name in records or type(count) is not int or count < 1
                or not isinstance(record['owner'], str) or not record['owner'].strip()
                or not isinstance(record['reason'], str) or not record['reason'].strip()):
            raise ValueError('invalid or duplicate carry')
        if name[1] == '@package':
            if 'signature' not in record:
                raise ValueError(f'{name[0]}/@package: carry requires failure signature')
            if set(record) != basic | {'signature'} or not isinstance(record['signature'], str) or not re.fullmatch(r'[0-9a-f]{64}', record['signature']):
                raise ValueError('invalid package carry signature')
            records[name] = (count, None, record['signature'])
            continue
        if 'failures' not in record:
            raise ValueError(f'{name[0]}/{name[1]}: carry requires exact failed case identities')
        if 'signature' not in record:
            raise ValueError(f'{name[0]}/{name[1]}: carry requires failure signature')
        if set(record) != basic | {'failures', 'signature'} or not isinstance(record['signature'], str) or not re.fullmatch(r'[0-9a-f]{64}', record['signature']):
            raise ValueError('invalid carry fields or signature')
        failures = record['failures']
        if (not isinstance(failures, list) or any(not isinstance(case, str) or
                not (case == name[1] or case.startswith(name[1] + '/')) for case in failures)
                or len(set(failures)) != len(failures) or len(failures) != count):
            raise ValueError('invalid carried failure identities or count')
        records[name] = (count, frozenset(failures), record['signature'])
    events = []
    failed_packages = set()
    for package in sorted({package for package, _ in selected}):
        names = sorted(test for pkg, test in selected if pkg == package)
        batches = []
        batch = []
        length = 4
        for name in names:
            escaped = re.escape(name)
            if batch and length + len(escaped) + 1 > 4096:
                batches.append(batch)
                batch, length = [], 4
            batch.append(name)
            length += len(escaped) + 1
        if batch:
            batches.append(batch)
        for batch in batches:
            pattern = '^(' + '|'.join(re.escape(name) for name in batch) + ')$'
            result = subprocess.run([args.go, 'test', '-json', '-count=1', '-p=1', '-timeout=0',
                                     '-run', pattern, './' + package], cwd=args.repo,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            if result.returncode not in (0, 1):
                raise ValueError(f'{package}: test runner failed: {result.stderr.strip()}')
            package_events = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
            package_terminal = [event for event in package_events if isinstance(event, dict)
                                and in_package(event, package) and not event.get('Test')
                                and event.get('Action') in ('pass', 'fail', 'skip')]
            if len(package_terminal) != 1 or package_terminal[0]['Action'] != ('pass' if result.returncode == 0 else 'fail'):
                raise ValueError(f'{package}: test runner exit and package result disagree')
            if result.returncode == 1 and not any(
                    isinstance(event, dict) and event.get('Action') == 'fail'
                    and isinstance(event.get('Test'), str)
                    and event['Test'] in batch
                    and in_package(event, package)
                    for event in package_events):
                raise ValueError(f'{package}: package failed without a failed selected test')
            if result.returncode == 1:
                failed_packages.add(package)
            events.extend(package_events)
    if git(args.repo, 'rev-parse', 'HEAD').decode().strip() != head or git(args.repo, 'status', '--porcelain', '--untracked-files=all'):
        raise ValueError('worktree changed during test execution')
    def approved_skip(event, matches):
        # Delegate to the same identity, reason and prerequisite policy as CI.
        # Feed only this skip and its preceding output so unrelated failures do
        # not change the policy decision or mask a rejected skip.
        package = event.get('Package')
        name = event.get('Test')
        previous = []
        for item in matches:
            if item is event:
                break
            if item.get('Action') == 'output' and item.get('Test') == name:
                previous.append(item)
        policy = Path(__file__).resolve().with_name('ci-go-test.sh')
        with tempfile.TemporaryDirectory(prefix='pairing-skip-') as directory:
            directory = Path(directory)
            event_path, summary, metadata = (directory / n for n in ('events', 'summary', 'metadata'))
            event_path.write_text(''.join(json.dumps(row) + '\n' for row in previous + [event, {'Action': 'pass', 'Package': package}]))
            metadata.write_text(package + '\thas-tests\n')
            result = subprocess.run(['bash', str(policy), 'package-summary', 'gladecli-ci',
                                     'pairing skip policy', str(event_path), str(summary), str(metadata), package],
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            return result.returncode == 0 and summary.is_file() and json.loads(summary.read_text()).get('valid') is True

    errors = []
    for package, test in sorted(selected):
        matches = [e for e in events if isinstance(e, dict)
                   and in_package(e, package)
                   and isinstance(e.get('Test'), str)
                   and (e['Test'] == test or e['Test'].startswith(test + '/'))]
        terminal = [e for e in matches if e.get('Action') in ('pass', 'fail', 'skip')]
        top = [e for e in terminal if e['Test'] == test]
        failures = {e['Test'] for e in terminal if e['Action'] == 'fail'}
        leaves = {name for name in failures if not any(other.startswith(name + '/') for other in failures)}
        package_results = [e for e in events if isinstance(e, dict) and in_package(e, package) and not e.get('Test')
                           and e.get('Action') in ('pass', 'fail', 'skip')]
        if not package_results or any(e['Action'] == 'skip' for e in package_results):
            errors.append(f'{package}/{test}: missing or ambiguous package result')
        elif len(top) != 1 or any(e['Action'] == 'skip' and not approved_skip(e, matches) for e in terminal):
            errors.append(f'{package}/{test}: missing, duplicate, or skipped result')
        elif top[0]['Action'] in ('pass', 'skip') and not failures:
            print(f'PASS {package}/{test}')
        elif top[0]['Action'] == 'fail' and any(e['Action'] == 'fail' for e in package_results) and leaves:
            record = records.get((package, test))
            if record is None or record[:2] != (len(leaves), frozenset(leaves)):
                errors.append(f'{package}/{test}: red without matching counted CARRIED record')
            elif signature(package, test) is None or record[2] != signature(package, test):
                errors.append(f'{package}/{test}: failure signature mismatch or missing diagnostic')
            else:
                print(f'CARRIED {package}/{test} count={len(leaves)}')
        else:
            errors.append(f'{package}/{test}: red without matching counted CARRIED record')
    for package in sorted(failed_packages):
        record = records.get((package, '@package'))
        if record is None or record[:2] != (1, None):
            errors.append(f'{package}: package failure requires counted CARRIED record')
        elif signature(package) is None or record[2] != signature(package):
            errors.append(f'{package}: package failure signature mismatch or missing diagnostic')
        else:
            print(f'CARRIED {package}/@package count=1')
    if errors:
        raise ValueError('\n'.join(errors))
    print(f'checked {len(selected)} tests in changed files at {head}')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError, json.JSONDecodeError) as exc:
        print(f'pairing guard: {exc}', file=sys.stderr)
        sys.exit(1)
