#!/usr/bin/env sh
set -eu

repo="glade-sh/glade"
install_dir="${GLADE_INSTALL_DIR:-$HOME/.local/bin}"
version="${GLADE_VERSION:-latest}"
github_token="${GLADE_GITHUB_TOKEN:-${GH_TOKEN:-${GITHUB_TOKEN:-}}}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "glade install: missing required command: $1" >&2
    exit 1
  fi
}

need curl
need grep
need install
need tar
need uname
need awk

curl_github() {
  if [ -n "$github_token" ]; then
    curl \
      -H "Authorization: Bearer $github_token" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "$@"
  else
    curl "$@"
  fi
}

os="$(uname -s)"
arch="$(uname -m)"
tmpdir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT INT TERM

case "$os" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *)
    echo "glade install: unsupported OS: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *)
    echo "glade install: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

api_base="https://api.github.com/repos/$repo"
release_json="$tmpdir/release.json"

fetch_release() {
  release_ref="$1"
  curl_github -fsSL "$api_base/releases/$release_ref" -o "$release_json" || {
    echo "glade install: could not resolve release $release_ref" >&2
    echo "glade install: for a private repo, export GLADE_GITHUB_TOKEN with contents:read access" >&2
    exit 1
  }
}

if [ "$version" = "latest" ]; then
  fetch_release latest
  version="$(awk -F '"' '/"tag_name":/ { print $4; exit }' "$release_json")"
  if [ -z "$version" ]; then
    echo "glade install: latest release did not include a tag name" >&2
    exit 1
  fi
else
  fetch_release "tags/$version"
fi

archive="glade_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$repo/releases/download/$version/$archive"

release_asset_api_url() {
  wanted="$1"
  awk -v wanted="$wanted" '
    /"url":/ && /\/releases\/assets\// {
      line = $0
      sub(/^[^"]*"url":[[:space:]]*"/, "", line)
      sub(/".*/, "", line)
      url = line
    }
    /"name":/ {
      line = $0
      sub(/^[^"]*"name":[[:space:]]*"/, "", line)
      sub(/".*/, "", line)
      if (line == wanted && url != "") {
        print url
        found = 1
        exit
      }
    }
    END { if (!found) exit 1 }
  ' "$release_json"
}

download_asset() {
  asset="$1"
  output="$2"
  asset_url="$(release_asset_api_url "$asset")" || {
    echo "glade install: release asset not found: $asset" >&2
    exit 1
  }
  curl_github -fL -H "Accept: application/octet-stream" "$asset_url" -o "$output"
}

mkdir -p "$install_dir"

echo "glade install: downloading $url"
download_asset "$archive" "$tmpdir/$archive" || {
  echo "glade install: archive download failed" >&2
  echo "glade install: for a private repo, export GLADE_GITHUB_TOKEN with contents:read access" >&2
  exit 1
}
download_asset "SHA256SUMS.txt" "$tmpdir/SHA256SUMS.txt" || {
  echo "glade install: checksum download failed" >&2
  echo "glade install: for a private repo, export GLADE_GITHUB_TOKEN with contents:read access" >&2
  exit 1
}

checksum_line="$(grep "  \\./$archive\$" "$tmpdir/SHA256SUMS.txt")" || {
  echo "glade install: checksum not found for $archive" >&2
  exit 1
}

if command -v shasum >/dev/null 2>&1; then
  (cd "$tmpdir" && printf '%s\n' "$checksum_line" | shasum -a 256 -c -)
elif command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmpdir" && printf '%s\n' "$checksum_line" | sha256sum -c -)
else
  echo "glade install: missing required command: shasum or sha256sum" >&2
  exit 1
fi

tar -xzf "$tmpdir/$archive" -C "$tmpdir"
install -m 0755 "$tmpdir/glade" "$install_dir/glade"

share_dir="${GLADE_HOME:-$HOME/.local/share/glade}"
if [ -d "$tmpdir/share/glade" ]; then
  mkdir -p "$share_dir"
  rm -rf "$share_dir/third_party" "$share_dir/lwcruntime"
  cp -R "$tmpdir/share/glade/." "$share_dir/"
  echo "glade LWC toolchain installed to $share_dir"
fi

echo "glade installed to $install_dir/glade"
"$install_dir/glade" version
