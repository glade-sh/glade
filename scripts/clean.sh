#!/usr/bin/env bash
# Remove local build/test artifacts that are gitignored but clutter the worktree
# and slow down tooling. Safe to run any time; touches only regenerable files.
set -euo pipefail

cd "$(dirname "$0")/.."

remove() {
	for path in "$@"; do
		if [ -e "$path" ]; then
			rm -rf "$path"
			echo "removed $path"
		fi
	done
}

# Compiled binaries (go build -o / go test -c output).
remove glade
remove ./*.test

# Ad-hoc JSON run result dumps.
remove ./*.localtest.json ./*.compat.json

# Build/output directories and caches.
remove bin dist coverage.out .gocache

# macOS Finder droppings.
find . -name '.DS_Store' -type f -print -delete 2>/dev/null || true

echo "clean: done"
