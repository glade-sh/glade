#!/usr/bin/env bash
# Print one `name=true|false` line per shared test binary for $GITHUB_OUTPUT.
# Push, schedule, dispatch, train and release runs build all of them. An
# ordinary pull request builds only the binaries for packages it selects, so a
# docs-only pull request builds nothing.
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
event=${EVENT_NAME:?}
head_ref=${HEAD_REF:-}
packages=(
	"apextest=github.com/glade-sh/glade/internal/apextest"
	"sema=github.com/glade-sh/glade/internal/sema"
	"lwc-compile=github.com/glade-sh/glade/internal/lwc/compile"
	"visualforce=github.com/glade-sh/glade/internal/visualforce"
)
build_all=false
if [[ "$event" != pull_request ]]; then
	build_all=true
else
	case "$head_ref" in train/* | release/*) build_all=true ;; esac
fi
selected=""
if [[ "$build_all" != true ]]; then
	selected=${CI_PR_SELECTED_FILE:-}
	if [[ -z "$selected" ]]; then
		selected=$(mktemp "${RUNNER_TEMP:-/tmp}/ci-pr-binaries.XXXXXX")
		trap 'rm -f "$selected"' EXIT
		(cd "$script_dir/.." && bash scripts/ci-pr-select.sh "$selected")
	fi
fi
for entry in "${packages[@]}"; do
	name=${entry%%=*}
	package=${entry#*=}
	if [[ "$build_all" == true ]] || jq -e --arg pkg "$package" 'index($pkg) != null' "$selected" >/dev/null; then
		echo "$name=true"
	else
		echo "$name=false"
	fi
done
