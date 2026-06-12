#!/usr/bin/env sh
set -eu

repo="glade-sh/glade"
install_dir="${GLADE_INSTALL_DIR:-$HOME/.local/bin}"
version="${GLADE_VERSION:-latest}"

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

os="$(uname -s)"
arch="$(uname -m)"

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

if [ "$version" = "latest" ]; then
  latest_url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")"
  version="${latest_url##*/}"
fi

archive="glade_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$repo/releases/download/$version/$archive"
checksums_url="https://github.com/$repo/releases/download/$version/SHA256SUMS.txt"
tmpdir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT INT TERM

mkdir -p "$install_dir"

echo "glade install: downloading $url"
curl -fL "$url" -o "$tmpdir/$archive"
curl -fL "$checksums_url" -o "$tmpdir/SHA256SUMS.txt"

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
