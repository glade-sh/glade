#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 && "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
  echo "usage: $0 vMAJOR.MINOR.PATCH" >&2
  exit 2
}
version="$1"
trap 'echo "Distribution is incomplete: check failed at line $LINENO for $version" >&2' ERR
stage="$(mktemp -d "${TMPDIR:-/tmp}/glade-distribution-check.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
cd "$stage"

curl -fsSL --max-time 60 https://downloads.glade.sh/index.json \
  | jq -e --arg version "$version" '.latest == $version' >/dev/null
for channel in latest "$version"; do
  curl -fsSL --max-time 60 "https://downloads.glade.sh/$channel/release-manifest.json" \
    | jq -e --arg version "$version" '.version == $version and (.assets | length) == 4' >/dev/null
done
curl -fsSL --max-time 60 https://glade.sh/site-build.json \
  | jq -e --arg version "$version" '.releaseVersion == $version' >/dev/null
curl -fsSL --max-time 60 https://glade.sh/install.sh -o install.sh

for requested in latest "$version"; do
  export HOME="$stage/$requested"
  export XDG_DATA_HOME="$HOME/.local/share"
  export XDG_CONFIG_HOME="$HOME/.config"
  export GLADE_HOME="$XDG_DATA_HOME/glade"
  export GLADE_INSTALL_DIR="$stage/$requested/bin"
  GLADE_VERSION="$requested" sh "$stage/install.sh"
  test "$("$GLADE_INSTALL_DIR/glade" version)" = "glade $version"
  mkdir -p "$stage/$requested/project"
  cd "$stage/$requested/project"
  "$GLADE_INSTALL_DIR/glade" init --project . --yes
  doctor="$("$GLADE_INSTALL_DIR/glade" doctor)"
  printf '%s\n' "$doctor"
  [[ "$doctor" == *"Ready."* ]] || { echo "installed doctor is not ready" >&2; exit 1; }
done
printf 'Distribution verified: %s, default and pinned installs ready.\n' "$version"
