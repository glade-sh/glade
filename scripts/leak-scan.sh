#!/usr/bin/env bash
# leak-scan.sh [--range A..B] [--diff FILE]
#
# Scans the added lines of a git diff for material that must never reach the
# public tree: personal absolute paths, host volume paths, private temp paths,
# and every entry of LEAK_SCAN_DENYLIST (newline-separated, case-insensitive
# substrings; comments start with #). Files under a prefix listed in
# .github/leak-scan-allow.txt skip the denylist entries only. The output names
# the file, line and pattern class, never the matched text.
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
  git -C "$root" diff -U0 --no-color --no-ext-diff "${range:-HEAD~1..HEAD}" -- . >"$work/diff" || exit 2
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
  file = ""; lineno = 0; added = 0; findings = 0
}
/^\+\+\+ / {
  file = substr($0, 5)
  sub(/^b\//, "", file)
  if (file == "/dev/null") file = ""
  allowed = 0
  for (i = 1; i <= na; i++) if (index(file, allow[i]) == 1) allowed = 1
  next
}
/^@@ / {
  split($0, parts, " ")
  plus = parts[3]; sub(/^\+/, "", plus); sub(/,.*/, "", plus)
  lineno = plus + 0
  next
}
/^\+/ {
  if (file == "") next
  added++
  text = substr($0, 2)
  class = ""
  if (text ~ /\/Users\/[A-Za-z0-9_]/) class = "personal-path"
  else if (text ~ /\/home\/[a-z][a-z0-9_-]*\// && text !~ /\/home\/runner\//) class = "personal-path"
  else if (text ~ /\/Volumes\//) class = "volume-path"
  else if (text ~ /\/private\/tmp\//) class = "private-tmp"
  if (class == "" && !allowed) {
    lower = tolower(text)
    for (i = 1; i <= nd; i++) if (index(lower, deny[i]) > 0) { class = "denylist"; break }
  }
  if (class != "") { findings++; files[file] = 1; printf "%s:%d: %s\n", file, lineno, class }
  lineno++
  next
}
END {
  nfiles = 0; for (f in files) nfiles++
  if (findings > 0) { printf "leak-scan: %d finding(s) in %d file(s)\n", findings, nfiles; exit 1 }
  printf "leak-scan: clean (%d added lines scanned, %d denylist entries)\n", added, nd
}
' "$work/diff"
