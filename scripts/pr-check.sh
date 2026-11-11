#!/usr/bin/env bash
# pr-check.sh [--base REF] [--head REF] [--body FILE] [--repoguard] [--vet] [--no-leak-scan] [--strict]
#
# Pre-review self-check for a branch that is about to become a pull request. It
# applies the machine-checkable parts of docs/WORKFLOW.md (sections 3.4 to 3.6
# and 4) so a reviewer does not have to bounce the branch for them:
#
#   clean-tree          uncommitted changes are not part of the PR            WARN
#   base-current        the branch is behind its base                         WARN
#   gofmt               changed Go files are gofmt-clean                      FAIL
#   vet                 go vet on changed packages (--vet)                    FAIL
#   leak-scan           scripts/leak-scan.sh on the added lines, when present FAIL
#   org-id              a Salesforce org id (00D...) in an added line         FAIL
#   personal-path       a personal home, host volume or private temp path added FAIL
#   test-first          a product change has a test or fixture change, and the
#                       first test commit is not after the first fix commit   FAIL
#   deleted-assertions  removed lines in existing tests or fixtures           WARN (--strict: FAIL)
#   abi                 SemanticABI, PlatformABI and testRuntimeCacheABI bumps
#                       required by the touched paths (WORKFLOW section 4)    FAIL
#   release-notes       a product change without a RELEASE_NOTES.md line      WARN
#   lanes-manifest      a new Go package missing from ci-package-lanes.json   FAIL
#   native-lines        a product change whose commits carry no NATIVE line   WARN
#   repoguard           go test ./internal/repoguard (--repoguard)            FAIL
#   pr-body             --body FILE has no unfilled <placeholder> or empty
#                       result cell                                           FAIL
#
# Output: one line per check, then "pr-check: PASS (n warnings)" or
# "pr-check: FAIL (f failures, n warnings)". Exit 0 on PASS, 1 on FAIL, 2 on
# usage or git errors. Nothing is modified.
set -u

base=""; head=HEAD; body=""; repoguard=0; vet=0; leak=1; strict=0
while [ $# -gt 0 ]; do
  case $1 in
    --base) base=${2:?--base needs a ref}; shift 2 ;;
    --head) head=${2:?--head needs a ref}; shift 2 ;;
    --body) body=${2:?--body needs a file}; shift 2 ;;
    --repoguard) repoguard=1; shift ;;
    --vet) vet=1; shift ;;
    --no-leak-scan) leak=0; shift ;;
    --strict) strict=1; shift ;;
    *) echo "usage: $0 [--base REF] [--head REF] [--body FILE] [--repoguard] [--vet] [--no-leak-scan] [--strict]" >&2; exit 2 ;;
  esac
done

root=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "pr-check: not a git repository" >&2; exit 2; }
cd "${root}" || exit 2
if [ -z "${base}" ]; then
  if git rev-parse --verify -q origin/main >/dev/null; then base=origin/main
  elif git rev-parse --verify -q main >/dev/null; then base=main
  else echo "pr-check: no origin/main or main; pass --base" >&2; exit 2; fi
fi
git rev-parse --verify -q "${base}^{commit}" >/dev/null || { echo "pr-check: unknown base ${base}" >&2; exit 2; }
git rev-parse --verify -q "${head}^{commit}" >/dev/null || { echo "pr-check: unknown head ${head}" >&2; exit 2; }
mb=$(git merge-base "${base}" "${head}") || exit 2

fails=0; warns=0
pass() { printf 'PASS %s: %s\n' "$1" "$2"; }
warn() { printf 'WARN %s: %s\n' "$1" "$2"; warns=$((warns + 1)); }
fail() { printf 'FAIL %s: %s\n' "$1" "$2"; fails=$((fails + 1)); }
skip() { printf 'SKIP %s: %s\n' "$1" "$2"; }
warn_or_fail() { if [ "${strict}" = 1 ]; then fail "$1" "$2"; else warn "$1" "$2"; fi; }

work=$(mktemp -d); trap 'rm -rf "${work}"' EXIT
git diff --name-status "${mb}" "${head}" > "${work}/status" || exit 2
# changed paths that still exist at head (not deleted)
awk '$1 != "D" {print $NF}' "${work}/status" > "${work}/present"
awk '{print $NF}' "${work}/status" > "${work}/all"
# product: non-test Go files under internal/ or cmd/ at head
grep -E '^(internal|cmd)/.*\.go$' "${work}/present" | grep -v '_test\.go$' > "${work}/product" || true
# tests and fixtures: *_test.go, testdata trees, tsv tables
grep -E '(_test\.go$|/testdata/|\.tsv$)' "${work}/all" > "${work}/tests" || true
nproduct=$(wc -l < "${work}/product" | tr -d ' ')
ntests=$(wc -l < "${work}/tests" | tr -d ' ')
git diff -U0 --no-color --no-ext-diff "${mb}" "${head}" > "${work}/diff" || exit 2
grep -E '^\+' "${work}/diff" | grep -vE '^\+\+\+ ' | sed 's/^+//' > "${work}/added" || true

# clean-tree
if [ "${head}" = HEAD ]; then
  dirty=$(git status --porcelain --untracked-files=no | wc -l | tr -d ' ')
  if [ "${dirty}" = 0 ]; then pass clean-tree "no uncommitted changes"; else warn clean-tree "${dirty} uncommitted change(s) are not part of the PR"; fi
else
  skip clean-tree "head is ${head}, not the working tree"
fi

# base-current
behind=$(git rev-list --count "${head}..${base}")
ahead=$(git rev-list --count "${base}..${head}")
if [ "${ahead}" = 0 ]; then fail base-current "no commits beyond ${base}"
elif [ "${behind}" = 0 ]; then pass base-current "${ahead} commit(s) ahead of ${base}"
else warn base-current "${behind} commit(s) behind ${base}; rebase or merge before opening the PR"; fi

# gofmt
gofmt_bin=$(command -v gofmt || true)
if [ -z "${gofmt_bin}" ] && command -v go >/dev/null 2>&1; then gofmt_bin="$(go env GOROOT)/bin/gofmt"; [ -x "${gofmt_bin}" ] || gofmt_bin=""; fi
if [ -z "${gofmt_bin}" ]; then for c in /usr/local/go/bin/gofmt /opt/homebrew/bin/gofmt; do [ -x "${c}" ] && { gofmt_bin=${c}; break; }; done; fi
gofiles=$(grep -E '\.go$' "${work}/present" || true)
if [ -z "${gofiles}" ]; then skip gofmt "no Go files changed"
elif [ -z "${gofmt_bin}" ]; then skip gofmt "gofmt not found"
elif [ "${head}" != HEAD ]; then skip gofmt "head is ${head}; run on a checkout"
else
  bad=$(printf '%s\n' "${gofiles}" | xargs "${gofmt_bin}" -l 2>/dev/null || true)
  if [ -z "${bad}" ]; then pass gofmt "$(printf '%s\n' "${gofiles}" | wc -l | tr -d ' ') Go file(s) clean"; else fail gofmt "not gofmt-clean: $(printf '%s' "${bad}" | tr '\n' ' ')"; fi
fi

# vet
if [ "${vet}" = 1 ]; then
  if [ "${head}" != HEAD ]; then skip vet "head is ${head}; run on a checkout"
  elif [ -z "${gofiles}" ]; then skip vet "no Go files changed"
  else
    pkgs=$(printf '%s\n' "${gofiles}" | xargs -n1 dirname | sort -u | sed 's#^#./#')
    if go vet ${pkgs} > "${work}/vet" 2>&1; then pass vet "$(printf '%s\n' "${pkgs}" | wc -l | tr -d ' ') package(s)"; else fail vet "$(head -3 "${work}/vet" | tr '\n' ' ')"; fi
  fi
fi

# leak-scan (the script scans added lines of the range; the denylist comes from LEAK_SCAN_DENYLIST)
if [ "${leak}" = 1 ] && [ -x scripts/leak-scan.sh ]; then
  if out=$(bash scripts/leak-scan.sh --range "${mb}..${head}" 2>&1); then pass leak-scan "$(printf '%s' "${out}" | tail -1)"; else fail leak-scan "$(printf '%s' "${out}" | tail -1)"; fi
elif [ "${leak}" = 1 ]; then skip leak-scan "scripts/leak-scan.sh not on this branch; built-in path checks only"
fi

# org-id: 15 or 18 character Salesforce ids with the organization prefix
n=$(grep -cE '(^|[^0-9A-Za-z])00D[0-9A-Za-z]{12}([0-9A-Za-z]{3})?([^0-9A-Za-z]|$)' "${work}/added" || true)
if [ "${n}" = 0 ]; then pass org-id "none in added lines"; else fail org-id "${n} added line(s) carry an organization id"; fi

# personal-path
# The patterns are spelled with brackets so this file never contains the literal paths it rejects.
n=$(grep -cE '/U[s]ers/[^/ ]|/h[o]me/[A-Za-z0-9_-]+/|/V[o]lumes/|/p[r]ivate/tmp/' "${work}/added" || true)
nr=$(grep -cE '/h[o]me/runner/' "${work}/added" || true)
n=$((n - nr)); [ "${n}" -lt 0 ] && n=0
if [ "${n}" = 0 ]; then pass personal-path "none in added lines"; else fail personal-path "${n} added line(s) with a personal, volume or private temp path"; fi

# test-first: commit order of the first test change and the first product change
if [ "${nproduct}" = 0 ]; then
  skip test-first "no product Go change"
else
  first_test=0; first_prod=0; i=0
  for c in $(git rev-list --reverse "${mb}..${head}"); do
    i=$((i + 1))
    files=$(git diff-tree --no-commit-id --name-only -r "${c}")
    if [ "${first_test}" = 0 ] && printf '%s\n' "${files}" | grep -qE '(_test\.go$|/testdata/|\.tsv$)'; then first_test=${i}; fi
    if [ "${first_prod}" = 0 ] && printf '%s\n' "${files}" | grep -E '^(internal|cmd)/.*\.go$' | grep -qv '_test\.go$'; then first_prod=${i}; fi
  done
  if [ "${ntests}" = 0 ]; then fail test-first "${nproduct} product file(s) changed with no test or fixture change (WORKFLOW 3.4)"
  elif [ "${first_test}" -gt "${first_prod}" ]; then fail test-first "first test commit is #${first_test}, after the first product commit #${first_prod}; the test commit must come first"
  elif [ "${first_test}" = "${first_prod}" ]; then warn test-first "test and fix share commit #${first_test}; record a fail-before run from a detached worktree in the PR (WORKFLOW 3.4)"
  else pass test-first "test commit #${first_test} precedes product commit #${first_prod}"
  fi
fi

# deleted-assertions: removed lines in tests or fixtures that still exist at head
git diff -U0 --no-color --no-ext-diff "${mb}" "${head}" -- '*_test.go' '*/testdata/*' '*.tsv' > "${work}/tdiff" 2>/dev/null || true
awk '
/^diff --git / { f=$3; sub(/^a\//,"",f); del=0; next }
/^deleted file mode / { del=1; next }
/^-/ && !/^---/ { if (!del) { c[f]++ } }
END { for (k in c) printf "%d %s\n", c[k], k }' "${work}/tdiff" | sort -rn > "${work}/removed"
if [ -s "${work}/removed" ]; then
  total=$(awk '{s+=$1} END {print s+0}' "${work}/removed")
  warn_or_fail deleted-assertions "${total} line(s) removed from existing tests or fixtures: $(head -5 "${work}/removed" | awk '{printf "%s(%s) ", $2, $1}')cite a native row or local contract for each in the PR"
else
  pass deleted-assertions "no lines removed from existing tests or fixtures"
fi

# abi (WORKFLOW section 4)
sema_touch=$(grep -cE '^internal/(sema|typesys|apexast|apexlang|ir|project|schema|sobject|semanticcache|frontendcache|codeintel)/' "${work}/product" || true)
plat_touch=$(grep -E '^internal/(vm|sema)/' "${work}/product" | grep -cE '/(platform_|builtin_|generated_platform_)[^/]*\.go$|_overlay\.go$' || true)
rt_touch=$(grep -cE '^internal/(vm|apextest|dml|soql|sosl|storage|automation|startupcache|trace|apexlog)/' "${work}/product" || true)
sema_bumped=0; plat_bumped=0; rt_bumped=0
grep -qE '^\+.*\bSemanticABI\s*=' "${work}/diff" && sema_bumped=1
grep -qE '^\+.*\bPlatformABI\s*=' "${work}/diff" && plat_bumped=1
grep -qE '^\+.*\btestRuntimeCacheABI\s*=' "${work}/diff" && rt_bumped=1
abi_missing=""
[ "${sema_touch}" -gt 0 ] && [ "${sema_bumped}" = 0 ] && abi_missing="${abi_missing} SemanticABI(internal/sema/sema.go; ${sema_touch} semantic file(s))"
[ "${plat_touch}" -gt 0 ] && [ "${plat_bumped}" = 0 ] && abi_missing="${abi_missing} PlatformABI(internal/sema/sema.go; ${plat_touch} platform file(s))"
if [ "${rt_touch}" -gt 0 ] || [ "${sema_bumped}" = 1 ] || [ "${sema_touch}" -gt 0 ]; then
  [ "${rt_bumped}" = 0 ] && abi_missing="${abi_missing} testRuntimeCacheABI(internal/apextest/startup_cache.go; ${rt_touch} runtime file(s), sema bump ${sema_bumped})"
fi
if [ "${sema_touch}" = 0 ] && [ "${plat_touch}" = 0 ] && [ "${rt_touch}" = 0 ]; then
  if [ "${sema_bumped}${plat_bumped}${rt_bumped}" = 000 ]; then skip abi "no ABI-bearing path changed"; else pass abi "bumped without a triggering path (allowed)"; fi
elif [ -n "${abi_missing}" ]; then fail abi "bump required but missing:${abi_missing}"
else pass abi "required bumps present (sema ${sema_bumped}, platform ${plat_bumped}, runtime cache ${rt_bumped})"
fi

# release-notes
if [ "${nproduct}" = 0 ]; then skip release-notes "no product change"
elif grep -q '^docs/RELEASE_NOTES.md$' "${work}/all"; then pass release-notes "docs/RELEASE_NOTES.md changed"
else warn release-notes "product change without a docs/RELEASE_NOTES.md line (needed for the behavior and performance classes)"
fi

# lanes-manifest: directories with Go files at head that had none at the merge base
if [ -f scripts/ci-package-lanes.json ] && [ -n "${gofiles}" ]; then
  module=$(sed -n 's/^module //p' go.mod | head -1)
  missing=""
  for d in $(printf '%s\n' "${gofiles}" | xargs -n1 dirname | sort -u); do
    case ${d} in internal/*|cmd/*) ;; *) continue ;; esac
    if [ "$(git ls-tree -r --name-only "${mb}" -- "${d}" | grep -c '\.go$')" = 0 ]; then
      grep -q "\"${module}/${d}\"" scripts/ci-package-lanes.json || missing="${missing} ${d}"
    fi
  done
  if [ -z "${missing}" ]; then pass lanes-manifest "every new Go package is in scripts/ci-package-lanes.json"; else fail lanes-manifest "new package(s) missing from scripts/ci-package-lanes.json:${missing}"; fi
else
  skip lanes-manifest "no manifest or no Go change"
fi

# native-lines
if [ "${nproduct}" = 0 ]; then skip native-lines "no product change"
elif git log --format=%B "${mb}..${head}" | grep -qE '^NATIVE |local contract'; then pass native-lines "commit messages cite NATIVE rows or a local contract"
else warn native-lines "no NATIVE line or local-contract citation in the commit messages (WORKFLOW 3.3 step 4)"
fi

# repoguard
if [ "${repoguard}" = 1 ]; then
  if [ "${head}" != HEAD ]; then skip repoguard "head is ${head}; run on a checkout"
  elif CGO_ENABLED=1 go test ./internal/repoguard > "${work}/rg" 2>&1; then pass repoguard "ok"
  else fail repoguard "$(grep -E '^(--- FAIL|FAIL|.*_test.go:[0-9]+)' "${work}/rg" | head -3 | tr '\n' ' ')"
  fi
fi

# pr-body
if [ -n "${body}" ]; then
  if [ ! -f "${body}" ]; then fail pr-body "no file ${body}"
  else
    # placeholders look like <one to three sentences>; text inside backticks (commands) is not counted
    ph=$(sed 's/`[^`]*`//g' "${body}" | grep -cE '<[a-z][a-z0-9 ,|:/-]*>' || true)
    empty=$(grep -cE '^\|[^|]*\|[^|]*\|[[:space:]]*\|[[:space:]]*$' "${body}" || true)
    unchecked=$(grep -c '^- \[ \]' "${body}" || true)
    msg="placeholders ${ph}, empty result cells ${empty}, unchecked boxes ${unchecked}"
    if [ "${ph}" -gt 0 ] || [ "${empty}" -gt 0 ]; then fail pr-body "${msg}"; elif [ "${unchecked}" -gt 0 ]; then warn pr-body "${msg}"; else pass pr-body "${msg}"; fi
  fi
fi

if [ "${fails}" -gt 0 ]; then printf 'pr-check: FAIL (%d failures, %d warnings)\n' "${fails}" "${warns}"; exit 1; fi
printf 'pr-check: PASS (%d warnings)\n' "${warns}"
