#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
manifest="${CI_RACE_PACKAGE_MANIFEST:-${script_dir}/ci-package-lanes.json}"
git_command="${CI_RACE_GIT_COMMAND:-git}"
go_command="${CI_RACE_GO_COMMAND:-go}"

emit_full() {
	python3 - "${manifest}" <<'PY'
import json
import sys

module = "github.com/glade-sh/glade"
with open(sys.argv[1], encoding="utf-8") as source:
    manifest = json.load(source)
if manifest.get("version") != 1 or not isinstance(manifest.get("lanes"), dict):
    raise SystemExit("race classifier rejected invalid package manifest")
packages = []
for lane in manifest["lanes"].values():
    if not isinstance(lane, list):
        raise SystemExit("race classifier rejected invalid package lane")
    packages.extend(lane)
expected = len(packages)
if expected == 0 or len(set(packages)) != expected or any(not value.startswith(module + "/") for value in packages):
    raise SystemExit(f"race classifier requires {expected} unique in-module packages")
print(json.dumps(sorted("." + value[len(module):] for value in packages), separators=(",", ":")))
PY
}

case "${1:-}" in
	partition)
		if [[ "$#" -ne 2 ]]; then
			echo "usage: scripts/ci-race-packages.sh partition <full-packages-json>" >&2
			exit 2
		fi
		python3 - "${manifest}" "$2" <<'PY'
import json
import sys

module = "github.com/glade-sh/glade"
with open(sys.argv[1], encoding="utf-8") as source:
    manifest = json.load(source)
if manifest.get("version") != 1 or not isinstance(manifest.get("lanes"), dict):
    raise SystemExit("race classifier rejected invalid package manifest")
expected = []
for lane in manifest["lanes"].values():
    if not isinstance(lane, list):
        raise SystemExit("race classifier rejected invalid package lane")
    expected.extend("." + value[len(module):] for value in lane)
packages = json.loads(sys.argv[2])
if not isinstance(packages, list) or sorted(packages) != sorted(expected) or len(packages) != len(set(packages)):
	raise SystemExit("race partition requires the exact unique full-manifest packages")
early = ["./internal/gladecli", "./internal/repoguard"]
apextest = "./internal/apextest"
excluded = set(early + [apextest])
generic = [package for package in packages if package not in excluded]
combined = early + generic + [apextest]
if sorted(combined) != sorted(packages) or len(combined) != len(set(combined)):
    raise SystemExit("race package partition does not exactly cover the full manifest")
print(json.dumps(early, separators=(",", ":")))
print(json.dumps(generic, separators=(",", ":")))
PY
		exit 0
		;;
	high-risk)
		printf '%s\n' '["./internal/apextest","./internal/gladecli","./internal/playground","./internal/sema","./internal/semanticcache","./internal/server","./internal/startupcache","./internal/storage"]'
		exit 0
		;;
	full)
		emit_full
		exit 0
		;;
	changed)
		if [[ "$#" -ne 3 ]]; then
			echo "usage: scripts/ci-race-packages.sh changed <base-sha> <head-sha>" >&2
			exit 2
		fi
		base_sha="$2"
		head_sha="$3"
		;;
	*)
		echo "usage: scripts/ci-race-packages.sh <partition FULL_JSON|high-risk|full|changed BASE HEAD>" >&2
		exit 2
		;;
esac

cd "${repo_root}"
diff_rows="$(mktemp "${TMPDIR:-/tmp}/glade-race-diff.XXXXXX")"
if ! "${git_command}" diff --name-status --no-renames --diff-filter=ACMRD "${base_sha}" "${head_sha}" -- >"${diff_rows}"; then
	rm -f "${diff_rows}"
	echo "[ci] git diff failed; selecting full race manifest" >&2
	emit_full
	exit 0
fi
mapfile -t changed_rows <"${diff_rows}"
rm -f "${diff_rows}"

declare -A changed_dirs=()
for row in "${changed_rows[@]}"; do
	IFS=$'\t' read -r change_status file extra <<<"${row}"
	if [[ -z "${change_status}" || -z "${file}" || -n "${extra:-}" ]]; then
		echo "[ci] malformed git diff row; selecting full race manifest" >&2
		emit_full
		exit 0
	fi
	dependency_file="${file##*/}"
	if [[ "${dependency_file}" == "go.mod" || "${dependency_file}" == "go.sum" ]]; then
		emit_full
		exit 0
	fi
	if [[ "${file}" != *.go ]]; then
		continue
	fi
	if [[ "${change_status}" == "D" ]]; then
		echo "[ci] deleted Go file; selecting full race manifest" >&2
		emit_full
		exit 0
	fi
	dir="$(dirname "${file}")"
	changed_dirs["${dir}"]=1
done
if [[ "${#changed_dirs[@]}" -eq 0 ]]; then
	printf '%s\n' '[]'
	exit 0
fi

changed_imports="$(mktemp "${TMPDIR:-/tmp}/glade-race-changed.XXXXXX")"
package_rows="$(mktemp "${TMPDIR:-/tmp}/glade-race-packages.XXXXXX")"
cleanup() { rm -f "${changed_imports}" "${package_rows}"; }
trap cleanup EXIT

for dir in "${!changed_dirs[@]}"; do
	if ! "${go_command}" list -f '{{.ImportPath}}' "./${dir}" >>"${changed_imports}"; then
		printf '[ci] changed Go directory %s no longer resolves; selecting full race manifest\n' "${dir}" >&2
		emit_full
		exit 0
	fi
done
if ! "${go_command}" list -f '{{.ImportPath}}{{"\t"}}{{join .Deps " "}}{{"\t"}}{{join .TestImports " "}}{{"\t"}}{{join .XTestImports " "}}' ./... >"${package_rows}"; then
	echo "[ci] dependency graph failed; selecting full race manifest" >&2
	emit_full
	exit 0
fi

python3 - "${changed_imports}" "${package_rows}" <<'PY'
import json
import sys

module = "github.com/glade-sh/glade"
with open(sys.argv[1], encoding="utf-8") as source:
    changed = {line.strip() for line in source if line.strip()}
if not changed or any(not value.startswith(module + "/") for value in changed):
    raise SystemExit("race classifier rejected changed package outside module")
rows = {}
with open(sys.argv[2], encoding="utf-8") as source:
    for line_number, raw in enumerate(source, 1):
        fields = raw.rstrip("\n").split("\t")
        if len(fields) != 4:
            raise SystemExit(f"race classifier rejected package row {line_number}")
        package = fields[0]
        if not package.startswith(module + "/"):
            continue
        rows[package] = (
            set(fields[1].split()),
            set(fields[2].split()),
            set(fields[3].split()),
        )
selected = set()
for package, (normal_dependencies, test_imports, xtest_imports) in rows.items():
    dependencies = set(normal_dependencies)
    for test_dependency in test_imports | xtest_imports:
        dependencies.add(test_dependency)
        if test_dependency in rows:
            dependencies.update(rows[test_dependency][0])
    if package in changed or dependencies.intersection(changed):
        selected.add("." + package[len(module):])
print(json.dumps(sorted(selected), separators=(",", ":")))
PY
