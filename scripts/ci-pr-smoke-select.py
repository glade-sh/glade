#!/usr/bin/env python3
"""Select distribution and runtime smoke for packaging-related PR paths."""
import sys

paths = sys.stdin.buffer.read().split(b'\0')

def packaging(path):
    return (path.startswith((b'site/', b'contrib/vscode-glade/', b'third_party/lwc/',
                             b'scripts/release', b'scripts/smoke-', b'scripts/sync-update-installer',
                             b'.github/workflows/release', b'scripts/verify-release'))
            or path in (b'Dockerfile', b'docs/INSTALL.md', b'scripts/prepare-lwc-shared.mjs',
                        b'.github/workflows/ci.yml', b'.github/release-authorities.json',
                        b'scripts/ci-pr-smoke-select.py'))

print('true' if any(packaging(path) for path in paths) else 'false')
