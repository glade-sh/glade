#!/usr/bin/env bash
set -euo pipefail

site_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${GLADE_SITE_PREVIEW_DIR:-/tmp/glade-site-pages}"
host="${GLADE_SITE_HOST:-127.0.0.1}"
port="${GLADE_SITE_PORT:-65110}"

cd "$site_root"

npm run docs:build

rm -rf "$out_dir"
mkdir -p "$out_dir/docs"
cp index.html "$out_dir/index.html"
cp install.sh "$out_dir/install.sh"
cp CNAME "$out_dir/CNAME"
cp -R .vitepress/dist/. "$out_dir/docs/"

cat <<EOF

Glade site preview is ready.

Home: http://$host:$port/
Docs: http://$host:$port/docs/

EOF

python3 -m http.server "$port" --bind "$host" --directory "$out_dir"
