#!/usr/bin/env bash
set -euo pipefail

# Linux amd64 CI archive. Update the version and digest together; see
# docs/WORKFLOW_VALIDATION.md. Never execute a download before verification.
version=1.7.12
sha256=8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8
if [[ $# -ne 1 || -z $1 ]]; then
  echo 'usage: scripts/install-actionlint.sh DESTINATION (Linux amd64)' >&2
  exit 2
fi
destination=$1
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
archive="$tmp_dir/actionlint.tar.gz"
curl -fsSL --retry 3 "https://github.com/rhysd/actionlint/releases/download/v${version}/actionlint_${version}_linux_amd64.tar.gz" -o "$archive"
if command -v sha256sum >/dev/null 2>&1; then
  printf '%s  %s\n' "$sha256" "$archive" | sha256sum -c -
else
  printf '%s  %s\n' "$sha256" "$archive" | shasum -a 256 -c -
fi
mkdir -p "$destination"
tar -xzf "$archive" -C "$destination" actionlint
