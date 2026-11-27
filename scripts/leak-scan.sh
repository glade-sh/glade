#!/usr/bin/env bash
# leak-scan.sh [--range A..B] [--diff FILE]
#
# Scans the added lines of a git diff for material that must never reach the
# public tree: personal absolute paths, host volume paths, private temp paths,
# and every entry of LEAK_SCAN_DENYLIST (newline-separated, case-insensitive
# substrings; comments start with #). Files under a prefix listed in
# .github/leak-scan-allow.txt skip the denylist entries only. The output identifies
# the file by ordinal, line and pattern class, never filenames or matched text.
#
# A binary change cannot be scanned, so it fails, with one narrow exception:
# .github/leak-scan-binary-allow.txt names paths where a human has reviewed the
# binary (for example image files). An entry ending in "/" is a directory
# prefix; any other entry is one exact file. A binary change at a listed path
# passes and is reported as "binary change accepted by allow list: <path>".
# Both allow lists are read from the scanner checkout, which CI takes from the
# base commit, so a change cannot add itself to either list.
#
# A LEAK_SCAN_DENYLIST with no entries (unset, empty, or comments only) fails
# the scan, so a missing secret cannot silently disable the private-name check.
# LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1 opts out deliberately: the scan then prints
# a WARNING that the denylist check was skipped, while the pattern checks
# (personal, volume and private temp paths) still run.
#
# Exit 0 when clean, 1 when there are findings or the denylist is empty without
# the opt-out, 2 on usage or git errors.
set -euo pipefail

range=""
difffile=""
while [ $# -gt 0 ]; do
  case $1 in
    --range) range=${2:?--range needs A..B}; shift 2 ;;
    --diff) difffile=${2:?--diff needs a file}; shift 2 ;;
    *) echo "usage: $0 [--range A..B] [--diff FILE]" >&2; exit 2 ;;
  esac
done

root=$(git rev-parse --show-toplevel) || exit 2
umask 077
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

if [ -n "$difffile" ]; then
  cp "$difffile" "$work/diff"
else
  git -C "$root" diff -U0 --no-color --no-ext-diff --no-textconv "${range:-HEAD~1..HEAD}" -- . >"$work/diff" || exit 2
  helper=$(cd "$(dirname "$0")" && pwd)/leak-scan-expand-gzip.py
  python3 "$helper" "$root" "$work/diff" "${range:-HEAD~1..HEAD}" || exit 2
fi
printf '%s\n' "${LEAK_SCAN_DENYLIST:-}" >"$work/deny"
allowempty=0
if [ "${LEAK_SCAN_ALLOW_EMPTY_DENYLIST:-}" = "1" ]; then allowempty=1; fi
allow="$root/.github/leak-scan-allow.txt"
[ -f "$allow" ] || allow=/dev/null
binallow="$root/.github/leak-scan-binary-allow.txt"
[ -f "$binallow" ] || binallow=/dev/null

awk -v denyfile="$work/deny" -v allowfile="$allow" -v binallowfile="$binallow" -v allowempty="$allowempty" '
BEGIN {
  nd = 0
  while ((getline line < denyfile) > 0) {
    sub(/\r$/, "", line); gsub(/^[ \t]+|[ \t]+$/, "", line)
    if (line == "" || line ~ /^#/) continue
    deny[++nd] = tolower(line)
  }
  close(denyfile)
  na = 0
  while ((getline line < allowfile) > 0) {
    sub(/\r$/, "", line); gsub(/^[ \t]+|[ \t]+$/, "", line)
    if (line == "" || line ~ /^#/) continue
    allow[++na] = line
  }
  close(allowfile)
  nb = 0
  while ((getline line < binallowfile) > 0) {
    sub(/\r$/, "", line); gsub(/^[ \t]+|[ \t]+$/, "", line)
    if (line == "" || line ~ /^#/) continue
    binallow[++nb] = line
  }
  close(binallowfile)
  file = ""; lineno = 0; added = 0; findings = 0; fileid = 0; in_hunk = 0
}
# Hunk counts determine when metadata resumes. Added lines can look exactly
# like headers, so their contents must never change the current file.
in_hunk {
  prefix = substr($0, 1, 1)
  if (prefix == "+") {
    added++
    text = substr($0, 2)
    class = ""
    homecheck = text; personalhome = 0
    while (match(homecheck, /\/home\/[^\/]+\//)) {
      if (substr(homecheck, RSTART, RLENGTH) != "/home/runner/") personalhome = 1
      homecheck = substr(homecheck, RSTART + 1)
    }
    if (text ~ /\/Users\/[^\/]/) class = "personal-path"
    else if (personalhome) class = "personal-path"
    else if (text ~ /\/Volumes\//) class = "volume-path"
    else if (text ~ /\/private\/tmp\//) class = "private-tmp"
    if (class == "" && !allowed) {
      lower = tolower(text)
      for (i = 1; i <= nd; i++) if (index(lower, deny[i]) > 0) { class = "denylist"; break }
    }
    if (class != "") { findings++; files[fileid] = 1; printf "file %d:%d: %s\n", fileid, lineno, class }
    lineno++; newleft--
  } else if (prefix == "-") oldleft--
  else if (prefix == " ") { oldleft--; newleft--; lineno++ }
  if (oldleft <= 0 && newleft <= 0) in_hunk = 0
  next
}
/^diff --git / {
  fileid++; file = ""; allowed = 0; deleted = 0; hdrpath = ""
  # Without a rename the header is "a/P b/P", so P is the middle of the line.
  hdr = substr($0, 12); n = length(hdr)
  if (n > 5 && (n - 5) % 2 == 0) {
    p = (n - 5) / 2
    if (substr(hdr, 1, 2) == "a/" && substr(hdr, p + 3, 1) == " " && substr(hdr, p + 4, 2) == "b/" && substr(hdr, 3, p) == substr(hdr, p + 6, p)) hdrpath = substr(hdr, 3, p)
  }
  next
}
/^deleted file mode / { deleted = 1; next }
/^Binary files .* differ$/ || /^GIT binary patch$/ {
  if (!deleted) {
    bpath = hdrpath
    if ($0 ~ /^Binary files /) {
      line = $0; sub(/ differ$/, "", line); at = 0
      for (i = 1; i <= length(line) - 6; i++) if (substr(line, i, 7) == " and b/") at = i
      if (at > 0) bpath = substr(line, at + 7)
    }
    ok = 0
    # Quoted paths (special characters) and unparsed paths are never accepted.
    if (bpath != "" && bpath !~ /^"/) {
      for (i = 1; i <= nb; i++) {
        if (substr(binallow[i], length(binallow[i])) == "/") { if (index(bpath, binallow[i]) == 1) ok = 1 }
        else if (bpath == binallow[i]) ok = 1
      }
    }
    if (ok) {
      lower = tolower(bpath)
      for (i = 1; i <= nd; i++) if (index(lower, deny[i]) > 0) ok = 0
      if (bpath ~ /\/Users\/[^\/]/ || bpath ~ /\/Volumes\// || bpath ~ /\/private\/tmp\// || bpath ~ /\/home\/[^\/]+\//) ok = 0
    }
    if (ok) printf "binary change accepted by allow list: %s\n", bpath
    else {
      findings++; files[fileid] = 1
      printf "file %d: binary change cannot be scanned\n", fileid
    }
  }
  next
}
/^\+\+\+ / {
  file = substr($0, 5)
  sub(/^b\//, "", file)
  allowed = 0
  for (i = 1; i <= na; i++) if (index(file, allow[i]) == 1) allowed = 1
  next
}
/^@@ / {
  split($0, parts, " ")
  old = parts[2]; plus = parts[3]
  sub(/^-/, "", old); sub(/^\+/, "", plus)
  split(old, oldparts, ","); split(plus, newparts, ",")
  oldleft = index(old, ",") ? oldparts[2] + 0 : 1
  newleft = index(plus, ",") ? newparts[2] + 0 : 1
  lineno = newparts[1] + 0
  in_hunk = (oldleft > 0 || newleft > 0)
  next
}
END {
  nfiles = 0; for (f in files) nfiles++
  # The skip warning must also be disclosed when findings end the scan with
  # exit 1, so it prints before the findings exit, not only on a clean scan.
  if (nd == 0 && allowempty) {
    printf "leak-scan: WARNING denylist check skipped: LEAK_SCAN_DENYLIST is empty (LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1); the pattern checks still ran\n"
  }
  if (findings > 0) { printf "leak-scan: %d finding(s) in %d file(s)\n", findings, nfiles; exit 1 }
  if (nd == 0 && !allowempty) {
    printf "leak-scan: FAIL LEAK_SCAN_DENYLIST is empty; the private-name check cannot run. Set the secret, or pass LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1 to skip it deliberately.\n"
    exit 1
  }
  printf "leak-scan: clean (%d added lines scanned, %d denylist entries)\n", added, nd
}
' "$work/diff"
