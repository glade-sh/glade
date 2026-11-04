#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 2 ]] || { echo "usage: $0 <candidate-glade-dir> <glade-tools-dir>" >&2; exit 2; }
product="$(cd "$1" && pwd -P)"
tools="$(cd "$2" && pwd -P)"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/glade-lockstep.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
cp "$tools/go.mod" "$tmp/go.mod"
cp "$tools/go.sum" "$tmp/go.sum"
export GOWORK=off
cd "$tools"
go mod edit -modfile="$tmp/go.mod" \
  "-replace=github.com/glade-sh/glade=$product" \
  "-replace=github.com/glade-sh/apex-parser=$product/third_party/glade-apex-parser"
go build -mod=readonly -modfile="$tmp/go.mod" -o "$tmp/glade-tools" ./cmd/glade-tools
