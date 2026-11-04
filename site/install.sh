#!/usr/bin/env sh
set -eu

repo="glade-sh/glade"
install_dir="${GLADE_INSTALL_DIR:-$HOME/.local/bin}"
version="${GLADE_VERSION:-latest}"
github_token="${GLADE_GITHUB_TOKEN:-${GH_TOKEN:-${GITHUB_TOKEN:-}}}"
download_base="${GLADE_DOWNLOAD_BASE:-https://downloads.glade.sh}"

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
need python3
need mkfifo

curl() {
  command curl --connect-timeout 10 --max-time 120 "$@"
}

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
lock_pid=""
transaction=""
marker_active=0
owner_ready=0

release_install_lock() {
  if [ -n "$lock_pid" ]; then
    if [ "$owner_ready" = 1 ]; then set_install_marker D || true; owner_ready=0; fi
    exec 3>&-
    kill "$lock_pid" 2>/dev/null || true
    wait "$lock_pid" || true
    lock_pid=""
  fi
}

cleanup() {
  if [ -n "$transaction" ]; then
    if recover_install "$transaction"; then
      if [ "$marker_active" = 1 ]; then
        set_install_marker C || { owner_ready=0; echo "glade install: recovery marker remains pending" >&2; }
      fi
    else
      owner_ready=0
      echo "glade install: recovery remains pending in $transaction" >&2
    fi
  fi
  release_install_lock
  rm -rf "$tmpdir"
}
trap cleanup EXIT
trap 'exit 1' INT TERM

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

share_dir="${GLADE_HOME:-${XDG_DATA_HOME:-$HOME/.local/share}/glade}"
if [ -f "$share_dir/go.mod" ]; then
  echo "glade install: selected toolchain directory is a source checkout; select a managed installation directory" >&2
  exit 1
fi
mkdir -p "$share_dir"
share_dir="$(cd "$share_dir" && pwd -P)"
case "$share_dir$install_dir" in *'
'*) echo "glade install: newline in installation path is unsupported" >&2; exit 1 ;; esac
mkdir -p "$install_dir"
install_dir="$(cd "$install_dir" && pwd -P)"
lock_dir="$share_dir"
mkdir -p "$lock_dir"
acquire_install_lock() {
rm -f "$tmpdir/install-lock-fifo" "$tmpdir/install-lock-ready" "$tmpdir/install-marker-ack"
mkfifo "$tmpdir/install-lock-fifo"
python3 -c '
import fcntl, os, sys
pipe = os.open(sys.argv[2], os.O_RDONLY)
lock = os.open(sys.argv[1], os.O_CREAT | os.O_RDWR, 0o600)
fcntl.lockf(lock, fcntl.LOCK_EX)
binary = os.open(sys.argv[7], os.O_CREAT | os.O_RDWR, 0o600)
fcntl.lockf(binary, fcntl.LOCK_EX)
root_owner, binary_owner = sys.argv[8], sys.argv[9]
def check_owner(path, target):
    try:
        with open(path, "rb") as f:
            previous = os.fsdecode(f.read(8192))
    except FileNotFoundError:
        return
    old_target, sep, old_pid = previous.partition("\n")
    if not sep or not old_target or not old_pid.isdigit():
        raise RuntimeError("invalid installer owner fence; manual recovery required")
    try:
        os.kill(int(old_pid), 0)
    except ProcessLookupError:
        pass
    except PermissionError:
        raise RuntimeError("cannot verify the previous installer has exited")
    else:
        raise RuntimeError("previous installer is still active")
    if old_target != target:
        raise RuntimeError("recover the previous installation at its original toolchain directory")
def publish_owner(path, target):
    temp = path + "." + str(os.getpid()) + ".tmp"
    fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        data = os.fsencode(target + "\n" + sys.argv[6])
        if os.write(fd, data) != len(data):
            raise OSError("short owner fence write")
        os.fsync(fd)
    finally:
        os.close(fd)
    os.replace(temp, path)
try:
    check_owner(root_owner, sys.argv[4])
    check_owner(binary_owner, sys.argv[4])
except (RuntimeError, OSError) as err:
    print("glade install: " + str(err), file=sys.stderr)
    sys.exit(1)
existing = os.pread(lock, 8192, 0)
if existing[:1] == b"P":
    print("glade install: run glade toolchain status to recover the source toolchain", file=sys.stderr)
    sys.exit(1)
if existing[:1] == b"S":
    target, _, owner = os.fsdecode(existing[1:]).partition("\n")
    if target != sys.argv[4]:
        print("glade install: recover the previous installation at its original toolchain directory", file=sys.stderr)
        sys.exit(1)
    if owner:
        try:
            os.kill(int(owner), 0)
        except ProcessLookupError:
            pass
        except (PermissionError, ValueError):
            print("glade install: cannot verify the previous installer has exited", file=sys.stderr)
            sys.exit(1)
        else:
            print("glade install: previous installer is still active", file=sys.stderr)
            sys.exit(1)
# Publish both fences before recovery or any replacement can begin.
publish_owner(root_owner, sys.argv[4])
publish_owner(binary_owner, sys.argv[4])
with open(sys.argv[3], "w"):
    pass
while True:
    command = os.read(pipe, 1)
    if not command:
        break
    if command == b"S":
        data = b"S" + os.fsencode(sys.argv[4]) + b"\n" + os.fsencode(sys.argv[6])
        if os.pread(lock, 1, 0) != b"S":
            if os.pwrite(lock, data, 0) != len(data):
                sys.exit(1)
    elif command == b"P":
        os.ftruncate(lock, 0)
        os.pwrite(lock, b"P", 0)
    elif command == b"C":
        # Clear only after recovery is complete; retain the owner fences.
        os.pwrite(lock, b"\0", 0)
        os.ftruncate(lock, 0)
    elif command == b"D":
        for path in (root_owner, binary_owner):
            try:
                os.unlink(path)
            except FileNotFoundError:
                pass
    else:
        sys.exit(1)
    os.fsync(lock)
    with open(sys.argv[5], "w"):
        pass
' "$lock_dir/.install.lock" "$tmpdir/install-lock-fifo" "$tmpdir/install-lock-ready" "$share_dir" "$tmpdir/install-marker-ack" "$$" "$install_dir/.glade-install.lock" "$share_dir/.install-owner" "$install_dir/.glade-install.owner" &
lock_pid=$!
exec 3<>"$tmpdir/install-lock-fifo"
while [ ! -e "$tmpdir/install-lock-ready" ]; do
  if ! kill -0 "$lock_pid" 2>/dev/null; then
    echo "glade install: could not acquire toolchain install lock" >&2
    exit 1
  fi
  sleep 0.1
done
set_install_marker() {
  rm -f "$tmpdir/install-marker-ack"
  printf '%s' "$1" >&3 || return 1
  marker_wait=0
  while [ ! -e "$tmpdir/install-marker-ack" ]; do
    kill -0 "$lock_pid" 2>/dev/null || return 1
    marker_wait=$((marker_wait + 1))
    [ "$marker_wait" -lt 30 ] || return 1
    sleep 0.1
  done
}
owner_ready=1
}
acquire_install_lock
if [ -f "$share_dir/go.mod" ]; then
  echo "glade install: selected toolchain directory is a source checkout; select a managed installation directory" >&2
  exit 1
fi
require_install_lock() {
  set_install_marker S || { echo "glade install: installation lock was lost" >&2; return 1; }
}
# A transaction stores the old assets beside their replacements. The binary
# stages and backups live beside the executable so each rename stays on one FS.
recover_install() {
  txn="$1"
  [ -d "$txn" ] || return 0
  if [ ! -f "$txn/install-dir" ]; then
    rm -rf "$txn"
    return
  fi
  IFS= read -r txn_install_dir < "$txn/install-dir" || return 1
  case "$txn_install_dir" in /*) ;; *) return 1 ;; esac
  txn_install_dir="$(cd "$txn_install_dir" 2>/dev/null && pwd -P)" || return 1
  if [ "$txn_install_dir" != "$install_dir" ]; then
    echo "glade install: recover the previous installation using its original executable directory" >&2
    return 1
  fi
  txn_id="${txn##*/}"
  binary_stage="$txn_install_dir/$txn_id.stage"
  binary_backup="$txn_install_dir/$txn_id.backup"
  if [ -f "$txn/started" ] && [ ! -f "$txn/committed" ]; then
    if [ -f "$txn/names" ]; then
      while IFS= read -r name; do
        [ -n "$name" ] || continue
        case "$name" in *[!A-Za-z0-9_-]*) return 1 ;; esac
        if [ -e "$txn/backup/$name" ] || [ -L "$txn/backup/$name" ]; then
          rm -rf "$share_dir/$name" || return 1
          mv "$txn/backup/$name" "$share_dir/$name" || return 1
        elif [ ! -e "$txn/had-$name" ]; then
          rm -rf "$share_dir/$name" || return 1
        fi
      done < "$txn/names"
    fi
    if [ -e "$binary_backup" ] || [ -L "$binary_backup" ]; then
      rm -f "$txn_install_dir/glade" || return 1
      mv "$binary_backup" "$txn_install_dir/glade" || return 1
    elif [ ! -e "$txn/had-binary" ]; then
      rm -f "$txn_install_dir/glade" || return 1
    fi
  fi
  rm -f "$binary_stage" "$binary_backup" || return 1
  rm -rf "$txn"
}

recover_pending_installs() {
for pending in "$share_dir"/.install-txn.*; do
  [ -d "$pending" ] || continue
  recover_install "$pending" || { owner_ready=0; echo "glade install: could not recover interrupted installation" >&2; exit 1; }
done
set_install_marker C
for stale_source in "$lock_dir"/.toolchain-stage-*; do
  [ -d "$stale_source" ] || continue
  if [ -f "$stale_source/committed" ] || { [ ! -e "$stale_source/old" ] && [ ! -L "$stale_source/old" ]; }; then
    rm -rf "$stale_source"
  else
    set_install_marker P
    echo "glade install: source staging has retained backups without a recovery marker" >&2
    exit 1
  fi
done
}
recover_pending_installs
release_install_lock

archive=""
url=""

manifest_version() {
  python3 - "$1" <<'PY'
import json, re, sys

def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate manifest key")
        result[key] = value
    return result

try:
    with open(sys.argv[1], encoding="utf-8") as source:
        data = json.load(source, object_pairs_hook=unique_object)
    if not isinstance(data, dict):
        raise ValueError("manifest must be an object")
    identities = [data[key] for key in ("version", "tagName") if key in data]
    if not identities or any(not isinstance(value, str) or value != identities[0] for value in identities):
        raise ValueError("invalid manifest identity")
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?(?:\+[A-Za-z0-9.-]+)?", identities[0]):
        raise ValueError("invalid manifest version")
    print(identities[0])
except (OSError, ValueError):
    sys.exit(1)
PY
}

try_static_release() {
  manifest="$tmpdir/release-manifest.json"
  if [ "$version" = "latest" ]; then
    curl -fsSL "$download_base/index.json" -o "$tmpdir/index.json" >/dev/null 2>&1 || true
    manifest_url="$download_base/latest/release-manifest.json"
  else
    manifest_url="$download_base/$version/release-manifest.json"
  fi

  curl -fsSL "$manifest_url" -o "$manifest" >/dev/null 2>&1 || return 1
  resolved_version="$(manifest_version "$manifest" 2>/dev/null || true)"
  if [ -z "$resolved_version" ]; then
    echo "glade install: release manifest has no version" >&2
    exit 1
  fi
  if [ "$version" != "latest" ] && [ "$resolved_version" != "$version" ]; then
    echo "glade install: release manifest version does not match requested version" >&2
    exit 1
  fi

  archive="glade_${resolved_version}_${os}_${arch}.tar.gz"
  url="$download_base/$resolved_version/$archive"
  echo "glade install: downloading $url"
  if ! curl -fsSL "$url" -o "$tmpdir/$archive" >/dev/null 2>&1; then
    url="$download_base/$archive"
    echo "glade install: downloading $url"
    curl -fsSL "$url" -o "$tmpdir/$archive" >/dev/null 2>&1 || return 1
  fi
  if ! curl -fsSL "$download_base/$resolved_version/SHA256SUMS.txt" -o "$tmpdir/SHA256SUMS.txt" >/dev/null 2>&1; then
    curl -fsSL "$download_base/SHA256SUMS.txt" -o "$tmpdir/SHA256SUMS.txt" >/dev/null 2>&1 || return 1
  fi
  version="$resolved_version"
  return 0
}

api_base="https://api.github.com/repos/$repo"
release_json="$tmpdir/release.json"

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

fetch_release() {
  release_ref="$1"
  curl_github -fsSL "$api_base/releases/$release_ref" -o "$release_json" || {
    echo "glade install: could not resolve release $release_ref" >&2
    echo "glade install: for a private repo, export GLADE_GITHUB_TOKEN with contents:read access" >&2
    exit 1
  }
}

if ! try_static_release; then
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
fi

mkdir -p "$install_dir"
install_dir="$(cd "$install_dir" && pwd -P)"

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

need gh
# gh requires authentication to fetch attestations itself, even for public
# repositories. Fetch public bundles through the unauthenticated API instead;
# gh still verifies their signatures and the full release policy below.
attestation_bundle() {
  kind="$1"
  case "$kind" in
    provenance) predicate='https%3A%2F%2Fslsa.dev%2Fprovenance%2Fv1' ;;
    sbom) predicate='https%3A%2F%2Fcyclonedx.org%2Fbom' ;;
    *) return 1 ;;
  esac
  bundle="$tmpdir/$kind.jsonl"
  urls="$tmpdir/$kind.urls"
  digest="$(python3 -c 'import hashlib,sys
h=hashlib.sha256()
with open(sys.argv[1], "rb") as f:
    for chunk in iter(lambda: f.read(1024*1024), b""): h.update(chunk)
print(h.hexdigest())' "$tmpdir/$archive")" || return 1
  curl -fsSL -H 'Accept: application/vnd.github+json' -H 'X-GitHub-Api-Version: 2022-11-28' \
    "$api_base/attestations/sha256:$digest?predicate_type=$predicate&per_page=100" -o "$tmpdir/$kind-response.json" || return 1
  python3 - "$tmpdir/$kind-response.json" "$bundle" "$urls" <<'PY'
import json, sys
try:
    with open(sys.argv[1], encoding="utf-8") as source:
        entries = json.load(source)["attestations"]
    if not isinstance(entries, list) or not entries:
        raise ValueError("no attestations")
    with open(sys.argv[2], "w", encoding="utf-8") as bundles, open(sys.argv[3], "w", encoding="utf-8") as urls:
        for entry in entries:
            if isinstance(entry.get("bundle"), dict):
                print(json.dumps(entry["bundle"], separators=(",", ":")), file=bundles)
            elif isinstance(entry.get("bundle_url"), str) and entry["bundle_url"].startswith("https://") and not any(c in entry["bundle_url"] for c in "\r\n"):
                print(entry["bundle_url"], file=urls)
            else:
                raise ValueError("invalid attestation")
except (OSError, ValueError, TypeError, KeyError, AttributeError):
    sys.exit(1)
PY
  [ "$?" -eq 0 ] || return 1
  while IFS= read -r bundle_url; do
    curl --proto '=https' --proto-redir '=https' -fsSL "$bundle_url" -o "$tmpdir/bundle-part.json" || return 1
    python3 -c 'import json,sys; print(json.dumps(json.load(open(sys.argv[1])), separators=(",", ":")))' "$tmpdir/bundle-part.json" >> "$bundle" || return 1
  done < "$urls"
  [ -s "$bundle" ]
}
bundle_args_provenance=""
bundle_args_sbom=""
if [ -n "$github_token" ]; then
  GH_TOKEN="$github_token"
  export GH_TOKEN
elif ! gh auth status --hostname github.com >/dev/null 2>&1; then
  attestation_bundle provenance || { echo "glade install: could not fetch public provenance attestations" >&2; exit 1; }
  attestation_bundle sbom || { echo "glade install: could not fetch public SBOM attestations" >&2; exit 1; }
  bundle_args_provenance="$tmpdir/provenance.jsonl"
  bundle_args_sbom="$tmpdir/sbom.jsonl"
fi
verify_attestation() {
  bundle_path="$1"; shift
  if [ -n "$bundle_path" ]; then
    gh attestation verify "$tmpdir/$archive" --bundle "$bundle_path" "$@"
  else
    gh attestation verify "$tmpdir/$archive" "$@"
  fi
}
if ! verify_attestation "$bundle_args_provenance" -R "$repo" \
  --signer-workflow "$repo/.github/workflows/release.yml" \
  --source-ref "refs/tags/$version"; then
  echo "glade install: release provenance verification failed" >&2
  exit 1
fi
if ! verify_attestation "$bundle_args_sbom" -R "$repo" \
  --signer-workflow "$repo/.github/workflows/release.yml" \
  --source-ref "refs/tags/$version" --predicate-type https://cyclonedx.org/bom; then
  echo "glade install: release SBOM attestation verification failed" >&2
  exit 1
fi

tar -xzf "$tmpdir/$archive" -C "$tmpdir"
acquire_install_lock
if [ -f "$share_dir/go.mod" ]; then
  echo "glade install: selected toolchain directory is a source checkout; select a managed installation directory" >&2
  exit 1
fi
recover_pending_installs
transaction="$(mktemp -d "$share_dir/.install-txn.XXXXXXXX")"
txn_id="${transaction##*/}"
# Newlines cannot be represented in the recovery record.
case "$install_dir" in *'
'*) echo "glade install: newline in install directory is unsupported" >&2; exit 1 ;; esac
printf '%s\n' "$install_dir" > "$transaction/install-dir"
mkdir "$transaction/stage" "$transaction/backup"
: > "$transaction/names"
binary_stage="$install_dir/$txn_id.stage"
binary_backup="$install_dir/$txn_id.backup"
install -m 0755 "$tmpdir/glade" "$binary_stage"
if ! "$binary_stage" version >/dev/null; then
  echo "glade install: staged executable failed its version check" >&2
  exit 1
fi
if [ -d "$tmpdir/share/glade" ]; then
  for packaged_dir in "$tmpdir/share/glade"/*; do
    [ -d "$packaged_dir" ] || continue
    name="${packaged_dir##*/}"
    case "$name" in
      third_party|lwcruntime|editor) ;;
      *) echo "glade install: unexpected asset directory: $name" >&2; exit 1 ;;
    esac
    printf '%s\n' "$name" >> "$transaction/names"
    cp -R "$packaged_dir" "$transaction/stage/$name"
    if [ -e "$share_dir/$name" ] || [ -L "$share_dir/$name" ]; then
      : > "$transaction/had-$name"
    fi
  done
fi
if [ -e "$install_dir/glade" ] || [ -L "$install_dir/glade" ]; then
  : > "$transaction/had-binary"
fi
: > "$transaction/started"
set_install_marker S
marker_active=1
# Staged replacement of one packaged toolchain directory. The replacement was
# copied to $transaction/stage/$name (the successor of the legacy
# .NAME.install.PID path) and the previous directory moves to
# $transaction/backup/$name (the successor of .NAME.backup.PID). Every rename
# runs under the install lock; recover_install will restore the backup if the
# transaction does not commit.
replace_toolchain_dir() {
  name="$1"
  stage="$transaction/stage/$name"
  backup="$transaction/backup/$name"
  require_install_lock
  if [ -e "$transaction/had-$name" ]; then
    mv "$share_dir/$name" "$backup"
    require_install_lock
  fi
  mv "$stage" "$share_dir/$name"
  require_install_lock
}
while IFS= read -r name; do
  [ -n "$name" ] || continue
  replace_toolchain_dir "$name"
done < "$transaction/names"
if [ -e "$transaction/had-binary" ]; then
  require_install_lock
  mv "$install_dir/glade" "$binary_backup"
  require_install_lock
fi
mv "$binary_stage" "$install_dir/glade"
require_install_lock
: > "$transaction/committed"
recover_install "$transaction"
transaction=""
set_install_marker C
marker_active=0
if [ -d "$tmpdir/share/glade" ]; then
  echo "glade LWC toolchain installed to $share_dir"
fi

echo "glade installed to $install_dir/glade"
release_install_lock
"$install_dir/glade" version
"$install_dir/glade" toolchain status || echo "glade install: active toolchain status unavailable" >&2
