#!/usr/bin/env bash
# leak-scan-commits.sh A..B
#
# Per-commit release scan over a git range: scripts/leak-scan.sh runs once per
# commit of `git rev-list --reverse A..B`, on that commit alone (first parent
# to commit; the empty tree for a root), so a finding names the commit that
# introduced it. The denylist and the empty-denylist rules are leak-scan.sh's
# own: a LEAK_SCAN_DENYLIST with no entries fails unless
# LEAK_SCAN_ALLOW_EMPTY_DENYLIST=1 opts out, and the skip warning is reported
# once for the range.
#
# Exit 0 when every commit is clean, 1 when a commit has a finding or the
# denylist is empty without the opt-out, 2 on usage or git errors. A range that
# names no commit fails: an empty scan must not report a clean result.
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: $0 A..B (a git rev-list range)" >&2
  exit 2
fi
range=$1

root=$(git rev-parse --show-toplevel) || exit 2
script=$(cd "$(dirname "$0")" && pwd)/leak-scan.sh
[ -f "$script" ] || { echo "leak-scan-commits: leak-scan.sh not found next to this script" >&2; exit 2; }

# Fail on a bad range before scanning anything.
if ! git -C "$root" rev-list --reverse "$range" >/dev/null 2>&1; then
  echo "leak-scan-commits: bad range ${range}" >&2
  exit 2
fi

empty_tree=$(git -C "$root" hash-object -t tree /dev/null) || exit 2
total=0; bad=0; warned=""
while read -r c; do
  [ -n "$c" ] || continue
  total=$((total + 1))
  short=$(git -C "$root" rev-parse --short "$c")
  if git -C "$root" rev-parse -q --verify "${c}^" >/dev/null; then
    commit_range="${c}^..${c}"
  else
    commit_range="${empty_tree}..${c}"   # a root commit scans against the empty tree
  fi
  if out=$(bash "$script" --range "$commit_range" 2>&1); then
    if printf '%s\n' "$out" | grep -q '^leak-scan: reviewed '; then
      printf 'leak-scan-commits: commit %s\n' "$short"
      printf '%s\n' "$out" | grep '^leak-scan: reviewed '
    fi
    # The skip warning is identical for every commit; report it once.
    if [ -z "$warned" ] && printf '%s\n' "$out" | grep -q '^leak-scan: WARNING '; then
      printf '%s\n' "$out" | grep '^leak-scan: WARNING '
      warned=1
    fi
    continue
  fi
  bad=$((bad + 1))
  printf 'leak-scan-commits: commit %s\n%s\n' "$short" "$out"
  # The skip WARNING names the empty denylist too; only the FAIL line means
  # the environment itself must be fixed before scanning can continue.
  if printf '%s\n' "$out" | grep -q '^leak-scan: FAIL LEAK_SCAN_DENYLIST is empty'; then
    # The empty denylist fails every remaining commit identically.
    printf 'leak-scan-commits: LEAK_SCAN_DENYLIST is empty for every commit; fix the environment and rerun\n'
    exit 1
  fi
done < <(git -C "$root" rev-list --reverse "$range")

if [ "$total" -eq 0 ]; then
  echo "leak-scan-commits: no commits in ${range}" >&2
  exit 2
fi
if [ "$bad" -gt 0 ]; then
  printf 'leak-scan-commits: %d of %d commit(s) have findings\n' "$bad" "$total"
  exit 1
fi
printf 'leak-scan-commits: %d commit(s) clean\n' "$total"
