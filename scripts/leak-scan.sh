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
# Exit 0 when clean, 1 when there are findings, 2 on usage or git errors.
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
allow="$root/.github/leak-scan-allow.txt"
[ -f "$allow" ] || allow=/dev/null

awk -v denyfile="$work/deny" -v allowfile="$allow" '
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
/^diff --git / { fileid++; file = ""; allowed = 0; deleted = 0; next }
/^deleted file mode / { deleted = 1; next }
/^Binary files .* differ$/ || /^GIT binary patch$/ {
  if (!deleted) {
    findings++; files[fileid] = 1
    printf "file %d: binary change cannot be scanned\n", fileid
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
  if (findings > 0) { printf "leak-scan: %d finding(s) in %d file(s)\n", findings, nfiles; exit 1 }
  printf "leak-scan: clean (%d added lines scanned, %d denylist entries)\n", added, nd
}
' "$work/diff"
