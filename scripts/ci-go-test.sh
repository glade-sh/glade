#!/usr/bin/env bash
set -euo pipefail

heartbeat_seconds="${CI_GO_TEST_HEARTBEAT_SECONDS:-60}"
if [[ ! "${heartbeat_seconds}" =~ ^[0-9]+$ ]] || [[ "${heartbeat_seconds}" -lt 1 ]]; then
	heartbeat_seconds=60
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"

testlog_renderer_path=""
testlog_renderer_dir=""
testlog_status_files=()
owned_child_roots=()
package_map_temp=""
package_test_metadata_temp=""
package_lane_rows=""

cleanup_testlog_renderer() {
	local status_file
	for status_file in ${testlog_status_files[@]+"${testlog_status_files[@]}"}; do
		rm -f "${status_file}"
	done
	if [[ -n "${testlog_renderer_dir}" ]]; then
		rm -f "${testlog_renderer_dir}/testlog"
		rmdir "${testlog_renderer_dir}" 2>/dev/null || true
	fi
	if [[ -n "${package_map_temp}" ]]; then
		rm -f "${package_map_temp}"
	fi
	if [[ -n "${package_test_metadata_temp}" ]]; then
		rm -f "${package_test_metadata_temp}"
	fi
}
trap cleanup_testlog_renderer EXIT

register_owned_child() {
	owned_child_roots+=("$1")
}

unregister_owned_child() {
	local completed_pid="$1"
	local pid
	local -a remaining=()
	for pid in ${owned_child_roots[@]+"${owned_child_roots[@]}"}; do
		if [[ "${pid}" != "${completed_pid}" ]]; then
			remaining+=("${pid}")
		fi
	done
	owned_child_roots=(${remaining[@]+"${remaining[@]}"})
}

terminate_owned_tree() {
	local root_pid="$1"
	local child_pid
	local children
	children="$(pgrep -P "${root_pid}" 2>/dev/null || true)"
	for child_pid in ${children}; do
		terminate_owned_tree "${child_pid}"
	done
	kill -KILL "${root_pid}" 2>/dev/null || true
}

terminate_owned_children() {
	local root_pid
	for root_pid in ${owned_child_roots[@]+"${owned_child_roots[@]}"}; do
		terminate_owned_tree "${root_pid}"
	done
	for root_pid in ${owned_child_roots[@]+"${owned_child_roots[@]}"}; do
		wait "${root_pid}" 2>/dev/null || true
	done
	owned_child_roots=()
}

handle_wrapper_signal() {
	local rc="$1"
	trap - INT TERM
	terminate_owned_children
	exit "${rc}"
}

trap 'handle_wrapper_signal 130' INT
trap 'handle_wrapper_signal 143' TERM

run_with_heartbeat() {
	local label="$1"
	shift
	local pid
	local heartbeat_pid
	local rc=0

	echo "::group::${label}"
	printf '[ci] GOMAXPROCS=%s\n' "${GOMAXPROCS:-default}"
	printf '+'
	printf ' %q' "$@"
	printf '\n'

	"$@" &
	pid="$!"
	register_owned_child "${pid}"

	(
		elapsed=0
		sleep_pid=""
		trap 'if [[ -n "${sleep_pid}" ]]; then kill "${sleep_pid}" 2>/dev/null || true; fi; exit 0' TERM INT
		while true; do
			sleep "${heartbeat_seconds}" &
			sleep_pid="$!"
			wait "${sleep_pid}" || exit 0
			sleep_pid=""
			if ! kill -0 "${pid}" 2>/dev/null; then
				exit 0
			fi
			elapsed=$((elapsed + heartbeat_seconds))
			printf '[ci] %s still running after %ss\n' "${label}" "${elapsed}"
		done
	) &
	heartbeat_pid="$!"
	register_owned_child "${heartbeat_pid}"

	wait "${pid}" || rc="$?"
	unregister_owned_child "${pid}"
	kill "${heartbeat_pid}" 2>/dev/null || true
	wait "${heartbeat_pid}" 2>/dev/null || true
	unregister_owned_child "${heartbeat_pid}"
	echo "::endgroup::"
	return "${rc}"
}

prepare_testlog_renderer() {
	if [[ -n "${testlog_renderer_path}" ]]; then
		return
	fi
	if [[ -n "${CI_TESTLOG_RENDERER:-}" ]]; then
		testlog_renderer_path="${CI_TESTLOG_RENDERER}"
		return
	fi
	testlog_renderer_dir="$(mktemp -d "${TMPDIR:-/tmp}/glade-ci-testlog-bin.XXXXXX")"
	testlog_renderer_path="${testlog_renderer_dir}/testlog"
	if ! go build -o "${testlog_renderer_path}" ./scripts/internal/testlog; then
		echo "[ci] testlog renderer build failed; live output unavailable" >&2
		testlog_renderer_path="/usr/bin/false"
		return 1
	fi
}

run_testlog_renderer() {
	local renderer_rc=0
	local -a args=(-output /dev/null)
	if [[ "${CI_VERBOSE:-0}" == "1" ]]; then
		args+=(-verbose)
	fi
	"${testlog_renderer_path}" "${args[@]}" || renderer_rc="$?"
	# A renderer that exits early must not close the test pipeline. Drain the
	# remaining stream so tee can finish the raw artifact and go test can exit.
	cat >/dev/null
	return "${renderer_rc}"
}

run_json_with_heartbeat() {
	local label="$1"
	local artifact="$2"
	shift 2
	local status_file
	local pid
	local heartbeat_pid
	local native_rc
	local tee_rc
	local renderer_rc

	mkdir -p "$(dirname "${artifact}")"
	prepare_testlog_renderer || true
	status_file="$(mktemp "${TMPDIR:-/tmp}/glade-ci-testlog.XXXXXX")"
	testlog_status_files+=("${status_file}")
	echo "::group::${label}"
	printf '[ci] GOMAXPROCS=%s\n' "${GOMAXPROCS:-default}"
	printf '+ go test -json -vet=off -trimpath'
	printf ' %q' "$@"
	printf ' | testlog -output %q\n' "${artifact}"

	(
		set +e
		local -a test_command=(go)
		if [[ "${CI_LOCAL_RELEASE_METRICS:-0}" == "1" && "${label}" == "go test local-release "* ]]; then
			test_command=(/usr/bin/time -l)
			if [[ -n "${testlog_append:-}" ]]; then test_command+=(-a); fi
			test_command+=(-o "${artifact%.json}.time.txt" go)
		fi
		if [[ -n "${CI_TEST_BIN_DIR:-}" ]]; then
			python3 "${script_dir}/ci-test-binary.py" test -json -vet=off -trimpath "$@"
		else
			"${test_command[@]}" test -json -vet=off -trimpath "$@"
		fi | tee ${testlog_append:+-a} "${artifact}" | run_testlog_renderer
		pipeline_status=("${PIPESTATUS[@]}")
		printf '%s %s %s\n' "${pipeline_status[0]}" "${pipeline_status[1]}" "${pipeline_status[2]}" >"${status_file}"
	) &
	pid="$!"
	register_owned_child "${pid}"
	(
		elapsed=0
		sleep_pid=""
		trap 'if [[ -n "${sleep_pid}" ]]; then kill "${sleep_pid}" 2>/dev/null || true; fi; exit 0' TERM INT
		while true; do
			sleep "${heartbeat_seconds}" &
			sleep_pid="$!"
			wait "${sleep_pid}" || exit 0
			sleep_pid=""
			if ! kill -0 "${pid}" 2>/dev/null; then
				exit 0
			fi
			elapsed=$((elapsed + heartbeat_seconds))
			printf '[ci] %s still running after %ss\n' "${label}" "${elapsed}"
		done
	) &
	heartbeat_pid="$!"
	register_owned_child "${heartbeat_pid}"
	wait "${pid}" || true
	unregister_owned_child "${pid}"
	kill "${heartbeat_pid}" 2>/dev/null || true
	wait "${heartbeat_pid}" 2>/dev/null || true
	unregister_owned_child "${heartbeat_pid}"
	echo "::endgroup::"
	if ! read -r native_rc tee_rc renderer_rc <"${status_file}"; then
		rm -f "${status_file}"
		echo "[ci] unable to read go test pipeline status" >&2
		return 1
	fi
	rm -f "${status_file}"
	if [[ "${tee_rc}" -ne 0 ]]; then
		printf '[ci] raw event writer failed with status %s; artifact: %s\n' "${tee_rc}" "${artifact}" >&2
	fi
	if [[ "${renderer_rc}" -ne 0 ]]; then
		printf '[ci] testlog renderer failed with status %s; raw events: %s\n' "${renderer_rc}" "${artifact}" >&2
	fi
	if [[ "${native_rc}" -ne 0 ]]; then
		return "${native_rc}"
	fi
	return "${tee_rc}"
}

testlog_artifact() {
	local kind="$1"
	local pkg="$2"
	local name="${pkg#./}"
	name="${name//\//-}"
	printf '%s/%s-%s.json\n' "${CI_GO_TEST_ARTIFACT_DIR:-ci-artifacts/go-test}" "${kind}" "${name}"
}

load_package_lanes() {
	if [[ -n "${package_lane_rows}" ]]; then
		return
	fi
	local go_command="${CI_GO_COMMAND:-go}"
	package_map_temp="$(mktemp "${TMPDIR:-/tmp}/glade-ci-packages.XXXXXX")"
	package_test_metadata_temp="$(mktemp "${TMPDIR:-/tmp}/glade-ci-package-tests.XXXXXX")"
	(cd "${repo_root}" && "${go_command}" list -f '{{.ImportPath}}{{printf "\t"}}{{if or .TestGoFiles .XTestGoFiles}}has-tests{{else}}no-tests{{end}}' ./...) >"${package_test_metadata_temp}"
	awk -F '\t' 'NF == 2 && ($2 == "has-tests" || $2 == "no-tests") { print $1; next } { exit 1 }' "${package_test_metadata_temp}" >"${package_map_temp}"
	package_lane_rows="$(cd "${repo_root}" && "${go_command}" run ./scripts/internal/cishard --package-manifest "${script_dir}/ci-package-lanes.json" --packages "${package_map_temp}")"
}

package_lane_packages() {
	local lane="$1"
	load_package_lanes
	awk -F '\t' -v lane="${lane}" '$1 == lane { print $2 }' <<<"${package_lane_rows}"
}

run_package_lane() {
	local lane="$1"
	# Full server validation requires L08. Ordinary CI explicitly routes it to
	# the provisioned Browser authority with -skip below.
	if [[ "${lane}" == "server-and-playground" ]]; then
		local -x GLADE_LWC_BROWSER=1
	fi
	local kind="$2"
	local timeout="$3"
	local skip_regex="${4:-}"
	local routing="${5:-all}"
	local -a packages=()
	local -a visualforce_packages=()
	local -a args=()
	load_package_lanes
	if [[ "${routing}" == "ci-remaining" ]]; then
		# Remaining toolchain consumers share the user installation directory.
		args+=(-p=1)
	fi
	while IFS= read -r pkg; do
		if [[ "${routing}" == "ci-remaining" && ( "${pkg}" == "./internal/lwc/compile" || "${pkg}" == "./internal/visualforce" || "${pkg}" == "./internal/vm" ) ]]; then
			continue
		elif [[ "${lane}" == "remaining-go" && "${kind}" == "test" && "${pkg}" == "./internal/visualforce" ]]; then
			visualforce_packages+=("${pkg}")
		else
			packages+=("${pkg}")
		fi
	done < <(awk -F '\t' -v lane="${lane}" '$1 == lane { print $2 }' <<<"${package_lane_rows}")
	if [[ "${#packages[@]}" -eq 0 && "${#visualforce_packages[@]}" -eq 0 ]]; then
		echo "[ci] package lane ${lane} is empty" >&2
		return 1
	fi
	if [[ "${kind}" == "race" ]]; then
		args+=(-race)
	fi
	args+=(-timeout="${timeout}")
	if [[ -n "${skip_regex}" ]]; then
		args+=(-skip "${skip_regex}")
	fi
	local lane_rc=0 visualforce_rc=0
	if [[ "${#packages[@]}" -gt 0 ]]; then
		run_json_with_heartbeat "go test ${lane}" "$(testlog_artifact "${kind}" "${lane}")" "${args[@]}" "${packages[@]}" || lane_rc="$?"
	fi
	if [[ "${#visualforce_packages[@]}" -gt 0 ]]; then
		# Browser conformance makes Visualforce longer than the other packages.
		# Keep its local-release budget without extending every package's timeout.
		run_json_with_heartbeat "go test ${lane} Visualforce" "$(testlog_artifact "${kind}" "${lane}-visualforce")" -timeout=70m "${visualforce_packages[@]}" || visualforce_rc="$?"
	fi
	if [[ "${lane_rc}" -ne 0 ]]; then
		return "${lane_rc}"
	fi
	return "${visualforce_rc}"
}

node_integration_run_regex='^(?:TestBuildCompileConfigAPIVersionMatrix|TestLWCModuleAvailabilityFollowsBundleAPIVersion|TestComplexTemplateExpressionsFollowBundleAPIVersion|TestHTMLDetailsNameFollowsBundleAPIVersion|TestCompilePreservesDeclaredAPI67|TestCompileProjectLWCBundles|TestCompileRewritesTemplateStylesheetImports|TestCompileEmitsSiblingJSModules|TestCompileEmitsUtilityOnlyLWCModules|TestCompileEmitsAdditionalHTMLTemplateModules|TestCompileTransformsCustomRenderComponentWithoutSameNameTemplate|TestCompileEnablesLwcOnDirective|TestSetupBundleIncludesLabelsSibling|TestSetupImportMapIncludesLocalComponents|TestVFPageBootstrapsLightningOut|TestVFPageBootstrapsMultiWidgetLightningOut|TestLightningModulesServesCompiledJS|TestLightningModulesServesSiblingModuleWithoutJSExtension|TestLWCShellComponentRouteServesHTML|TestLWCShellRootRendersHomeWithFormalTabsAndBuilderLink|TestLWCShellBuilderRouteRendersBuilderNavigationLayoutAndSampleRecord|TestLWCShellTabRouteIncludesPreviewRouteCatalog|TestServerRootRendersLWCHomeWhenProjectHasLWCs|TestLWCShellRendersApplicationNavAndConsoleMode|TestLWCShellAppRouteFallsBackToApplicationDefaultTab|TestLWCShellUnsupportedCustomTabReturnsDiagnostic|TestLWCShellMixedPageDiagnosticsStillRendersValidComponents|TestValidateRootFindsRepoCheckout|TestInstallFromCWDSkipsGlobalShareAsSource|TestInstallFromCopiesToolchain|TestEnsureRootHonorsExplicitGladeHomeBeforeUserShare|TestRunDoctorReportsParser|TestRunDoctorJSON|TestRunDoctorShortFlags|TestRunDoctorReportsProjectLocalDataEnvironment)$'

write_node_integration_expected() {
	local output="$1"
	cat >"${output}" <<'EOF'
github.com/glade-sh/glade/internal/gladecli	TestRunDoctorJSON
github.com/glade-sh/glade/internal/gladecli	TestRunDoctorReportsParser
github.com/glade-sh/glade/internal/gladecli	TestRunDoctorReportsProjectLocalDataEnvironment
github.com/glade-sh/glade/internal/gladecli	TestRunDoctorShortFlags
github.com/glade-sh/glade/internal/gladehome	TestEnsureRootHonorsExplicitGladeHomeBeforeUserShare
github.com/glade-sh/glade/internal/gladehome	TestInstallFromCWDSkipsGlobalShareAsSource
github.com/glade-sh/glade/internal/gladehome	TestInstallFromCopiesToolchain
github.com/glade-sh/glade/internal/gladehome	TestValidateRootFindsRepoCheckout
github.com/glade-sh/glade/internal/lwc/compile	TestBuildCompileConfigAPIVersionMatrix
github.com/glade-sh/glade/internal/lwc/compile	TestCompileEmitsAdditionalHTMLTemplateModules
github.com/glade-sh/glade/internal/lwc/compile	TestCompileEmitsSiblingJSModules
github.com/glade-sh/glade/internal/lwc/compile	TestCompileEmitsUtilityOnlyLWCModules
github.com/glade-sh/glade/internal/lwc/compile	TestCompileEnablesLwcOnDirective
github.com/glade-sh/glade/internal/lwc/compile	TestCompilePreservesDeclaredAPI67
github.com/glade-sh/glade/internal/lwc/compile	TestCompileProjectLWCBundles
github.com/glade-sh/glade/internal/lwc/compile	TestCompileRewritesTemplateStylesheetImports
github.com/glade-sh/glade/internal/lwc/compile	TestCompileTransformsCustomRenderComponentWithoutSameNameTemplate
github.com/glade-sh/glade/internal/lwc/compile	TestComplexTemplateExpressionsFollowBundleAPIVersion
github.com/glade-sh/glade/internal/lwc/compile	TestHTMLDetailsNameFollowsBundleAPIVersion
github.com/glade-sh/glade/internal/lwc/compile	TestLWCModuleAvailabilityFollowsBundleAPIVersion
github.com/glade-sh/glade/internal/lwcbrowser	TestSetupBundleIncludesLabelsSibling
github.com/glade-sh/glade/internal/lwcbrowser	TestSetupImportMapIncludesLocalComponents
github.com/glade-sh/glade/internal/server	TestLWCShellAppRouteFallsBackToApplicationDefaultTab
github.com/glade-sh/glade/internal/server	TestLWCShellBuilderRouteRendersBuilderNavigationLayoutAndSampleRecord
github.com/glade-sh/glade/internal/server	TestLWCShellComponentRouteServesHTML
github.com/glade-sh/glade/internal/server	TestLWCShellMixedPageDiagnosticsStillRendersValidComponents
github.com/glade-sh/glade/internal/server	TestLWCShellRendersApplicationNavAndConsoleMode
github.com/glade-sh/glade/internal/server	TestLWCShellRootRendersHomeWithFormalTabsAndBuilderLink
github.com/glade-sh/glade/internal/server	TestLWCShellTabRouteIncludesPreviewRouteCatalog
github.com/glade-sh/glade/internal/server	TestLWCShellUnsupportedCustomTabReturnsDiagnostic
github.com/glade-sh/glade/internal/server	TestLightningModulesServesCompiledJS
github.com/glade-sh/glade/internal/server	TestLightningModulesServesSiblingModuleWithoutJSExtension
github.com/glade-sh/glade/internal/server	TestServerRootRendersLWCHomeWhenProjectHasLWCs
github.com/glade-sh/glade/internal/server	TestVFPageBootstrapsLightningOut
github.com/glade-sh/glade/internal/server	TestVFPageBootstrapsMultiWidgetLightningOut
EOF
}

validate_node_integration_events() {
	local events_path="$1"
	local expected_path="$2"
	local discovery_path="$3"
	local summary_path="$4"
	python3 - "${events_path}" "${expected_path}" "${discovery_path}" "${summary_path}" <<'PY'
import json
import os
import sys

events_path, expected_path, discovery_path, summary_path = sys.argv[1:]
expected = []
expected_count = 35
with open(expected_path, encoding="utf-8") as source:
    for line_number, raw in enumerate(source, 1):
        fields = raw.rstrip("\n").split("\t")
        if len(fields) != 2 or not all(fields):
            raise SystemExit(f"[ci] malformed node integration expected row {line_number}")
        expected.append(tuple(fields))
if len(expected) != expected_count or len(set(expected)) != expected_count or expected != sorted(expected):
    raise SystemExit(f"[ci] node integration expected set must be {expected_count} unique sorted package/name pairs")

expected_set = set(expected)
discovered = []
terminals = []
try:
    with open(events_path, encoding="utf-8") as source:
        for line_number, raw in enumerate(source, 1):
            try:
                event = json.loads(raw)
            except Exception as error:
                raise ValueError(f"malformed JSON event at line {line_number}: {error}")
            if not isinstance(event, dict):
                raise ValueError(f"non-object JSON event at line {line_number}")
            test = event.get("Test")
            package = event.get("Package")
            action = event.get("Action")
            if test and action == "skip" and (package, test.split("/", 1)[0]) in expected_set:
                raise ValueError(f"skip event for selected test {package}.{test}")
            if not test or "/" in test:
                continue
            pair = (package, test)
            if pair not in expected_set:
                raise ValueError(f"unexpected top-level test event {package}.{test}")
            if action == "run":
                discovered.append(pair)
            elif action in {"pass", "skip", "fail"}:
                terminals.append((pair, action))
    if len(discovered) != expected_count or len(set(discovered)) != expected_count or set(discovered) != expected_set:
        raise ValueError("discovery does not contain each expected test exactly once")
    if len(terminals) != expected_count:
        raise ValueError(f"terminal count is {len(terminals)}, want {expected_count}")
    terminal_pairs = [pair for pair, _ in terminals]
    if len(set(terminal_pairs)) != expected_count or set(terminal_pairs) != expected_set:
        raise ValueError("terminal results do not contain each expected test exactly once")
    not_passed = [(pair, action) for pair, action in terminals if action != "pass"]
    if not_passed:
        raise ValueError(f"non-pass terminal results: {not_passed}")
except Exception as error:
    for path in (summary_path,):
        try:
            os.remove(path)
        except FileNotFoundError:
            pass
    print(f"[ci] node integration validation rejected: {error}", file=sys.stderr)
    raise SystemExit(1)

with open(discovery_path, "w", encoding="utf-8") as target:
    for package, test in sorted(discovered):
        target.write(f"{package}\t{test}\n")
with open(summary_path, "w", encoding="utf-8") as target:
    json.dump({"valid": True, "tests": expected_count, "passed": expected_count, "skipped": 0, "failed": 0}, target, sort_keys=True, indent=2)
    target.write("\n")
PY
}

run_node_integration() {
	local artifact_dir="${CI_NODE_INTEGRATION_ARTIFACT_DIR:-ci-artifacts/go-test-node-integration}"
	local events_path="${artifact_dir}/test-node-integration.json"
	local expected_path="${artifact_dir}/expected.txt"
	local discovery_path="${artifact_dir}/discovery.txt"
	local summary_path="${artifact_dir}/validation-summary.json"
	local status_file
	local pid heartbeat_pid
	local native_rc tee_rc renderer_rc validator_rc=0
	mkdir -p "${artifact_dir}"
	rm -f "${summary_path}" "${discovery_path}"
	write_node_integration_expected "${expected_path}"
	prepare_testlog_renderer || true
	status_file="$(mktemp "${TMPDIR:-/tmp}/glade-ci-node-integration.XXXXXX")"
	testlog_status_files+=("${status_file}")
	echo "::group::go test node-integration"
	printf '[ci] GOMAXPROCS=%s\n' "${GOMAXPROCS:-default}"
	# These packages can install into the same user toolchain directory. Run
	# their test binaries serially so installation cannot reset another's files.
	printf '+ go test -json -vet=off -p=1 -count=1 -timeout=30m -trimpath -run %q ./internal/gladecli ./internal/gladehome ./internal/lwc/compile ./internal/lwcbrowser ./internal/server | testlog -output %q\n' "${node_integration_run_regex}" "${events_path}"
	(
		set +e
		go test -json -vet=off -p=1 -count=1 -timeout=30m -trimpath -run "${node_integration_run_regex}" ./internal/gladecli ./internal/gladehome ./internal/lwc/compile ./internal/lwcbrowser ./internal/server | tee "${events_path}" | run_testlog_renderer
		pipeline_status=("${PIPESTATUS[@]}")
		printf '%s %s %s\n' "${pipeline_status[0]}" "${pipeline_status[1]}" "${pipeline_status[2]}" >"${status_file}"
	) &
	pid="$!"
	register_owned_child "${pid}"
	(
		elapsed=0
		sleep_pid=""
		trap 'if [[ -n "${sleep_pid}" ]]; then kill "${sleep_pid}" 2>/dev/null || true; fi; exit 0' TERM INT
		while true; do
			sleep "${heartbeat_seconds}" &
			sleep_pid="$!"
			wait "${sleep_pid}" || exit 0
			sleep_pid=""
			if ! kill -0 "${pid}" 2>/dev/null; then exit 0; fi
			elapsed=$((elapsed + heartbeat_seconds))
			printf '[ci] go test node-integration still running after %ss\n' "${elapsed}"
		done
	) &
	heartbeat_pid="$!"
	register_owned_child "${heartbeat_pid}"
	wait "${pid}" || true
	unregister_owned_child "${pid}"
	kill "${heartbeat_pid}" 2>/dev/null || true
	wait "${heartbeat_pid}" 2>/dev/null || true
	unregister_owned_child "${heartbeat_pid}"
	echo "::endgroup::"
	if ! read -r native_rc tee_rc renderer_rc <"${status_file}"; then
		echo "[ci] unable to read node integration pipeline status" >&2
		return 1
	fi
	if [[ "${native_rc}" -eq 0 && "${tee_rc}" -eq 0 ]]; then
		validate_node_integration_events "${events_path}" "${expected_path}" "${discovery_path}" "${summary_path}" || validator_rc="$?"
	else
		rm -f "${summary_path}" "${discovery_path}"
	fi
	if [[ "${native_rc}" -ne 0 ]]; then
		printf '[ci] native node integration go test failed with status %s\n' "${native_rc}" >&2
		return "${native_rc}"
	fi
	if [[ "${tee_rc}" -ne 0 ]]; then
		printf '[ci] node integration raw event writer failed with status %s\n' "${tee_rc}" >&2
		return "${tee_rc}"
	fi
	if [[ "${renderer_rc}" -ne 0 ]]; then
		printf '[ci] node integration testlog renderer failed with status %s; raw events preserved\n' "${renderer_rc}" >&2
	fi
	return "${validator_rc}"
}

run_core_tests() {
	run_named_package_lane "gladecli"
	run_named_package_lane "sema"
	run_named_package_lane "server-and-playground"
	run_named_package_lane "repoguard"
	run_named_package_lane "remaining-go"
}

run_named_package_lane() {
	local lane="$1"
	case "${lane}" in
		gladecli|sema|server-and-playground|repoguard)
			run_package_lane "${lane}" test 30m
			;;
		remaining-go)
			# VM exceeded 20 minutes on main and PR; 30m provides at least 50% margin over 20m.
			run_package_lane "${lane}" test 30m
			;;
		apextest)
			echo "[ci] apextest must use dedicated apex-shard routing" >&2
			return 2
			;;
		*)
			echo "[ci] unknown package lane: ${lane}" >&2
			return 2
			;;
	esac
}

# These lane-specific skips route tests to provisioned authorities; they are not
# known-failure waivers. The 35 excluded Node tests run in node-integration,
# whose validator requires every selected test to pass without skips. The four
# browser tests run in .github/workflows/browser.yml with Node, LWC dependencies,
# Chromium and GLADE_LWC_BROWSER=1; that workflow rejects Go and Node skip events.
# local-release uses run_local_release_lane instead and retains these tests.
run_ci_package_lane() {
	local lane="$1"
	case "${lane}" in
		workflow-lint)
			run_json_with_heartbeat "go test workflow-lint" "$(testlog_artifact test workflow-lint)" -count=1 -timeout=3m -run '^TestCIActionlint' ./scripts/ci_actionlint_test.go
			;;
		gladecli)
			run_package_lane "${lane}" test 30m '^(?:TestRunDoctorReportsParser|TestRunDoctorJSON|TestRunDoctorShortFlags|TestRunDoctorReportsProjectLocalDataEnvironment)$'
			;;
		server-and-playground)
			run_package_lane "${lane}" test 30m '^(?:TestVFPageBootstrapsLightningOut|TestVFPageBootstrapsMultiWidgetLightningOut|TestLightningModulesServesCompiledJS|TestLightningModulesServesSiblingModuleWithoutJSExtension|TestLWCShellComponentRouteServesHTML|TestLWCShellRootRendersHomeWithFormalTabsAndBuilderLink|TestLWCShellBuilderRouteRendersBuilderNavigationLayoutAndSampleRecord|TestLWCShellTabRouteIncludesPreviewRouteCatalog|TestServerRootRendersLWCHomeWhenProjectHasLWCs|TestLWCShellRendersApplicationNavAndConsoleMode|TestLWCShellAppRouteFallsBackToApplicationDefaultTab|TestLWCShellUnsupportedCustomTabReturnsDiagnostic|TestLWCShellMixedPageDiagnosticsStillRendersValidComponents|TestL08SalesforceConformance|TestProcessBuilder.*Hosts)$'
			;;
		server-process-builder)
			# Large Process Builder host-route tests run in their own job; the
			# server-and-playground lane skips the same pattern.
			run_json_with_heartbeat "go test server-process-builder" "$(testlog_artifact test server-process-builder)" -timeout=45m -run '^TestProcessBuilder.*Hosts$' ./internal/server
			;;
		remaining-go)
			run_package_lane "${lane}" test 30m '^(?:TestBuildCompileConfigAPIVersionMatrix|TestLWCModuleAvailabilityFollowsBundleAPIVersion|TestComplexTemplateExpressionsFollowBundleAPIVersion|TestHTMLDetailsNameFollowsBundleAPIVersion|TestCompilePreservesDeclaredAPI67|TestCompileProjectLWCBundles|TestCompileRewritesTemplateStylesheetImports|TestCompileEmitsSiblingJSModules|TestCompileEmitsUtilityOnlyLWCModules|TestCompileEmitsAdditionalHTMLTemplateModules|TestCompileTransformsCustomRenderComponentWithoutSameNameTemplate|TestCompileEnablesLwcOnDirective|TestSetupBundleIncludesLabelsSibling|TestSetupImportMapIncludesLocalComponents|TestValidateRootFindsRepoCheckout|TestInstallFromCWDSkipsGlobalShareAsSource|TestInstallFromCopiesToolchain|TestEnsureRootHonorsExplicitGladeHomeBeforeUserShare|TestBrowserRuntimeSuite|TestGeneratedPhase3BaseComponentsRunInBrowser|TestLWCAPI67RegistrationRunsInBrowser)$' ci-remaining
			;;
		sema|repoguard)
			run_named_package_lane "${lane}"
			;;
		*)
			run_named_package_lane "${lane}"
			;;
	esac
}

run_full_tests() {
	run_package_lane "apextest" test 30m
	run_core_tests
}

local_release_jobs() {
	local requested="${LOCAL_GO_TEST_JOBS:-auto}"
	case "${requested}" in
		auto)
			# No low-memory overlap has passed the resource matrix yet. Keep the
			# automatic mode serial until that evidence exists; an explicit positive
			# value is an intentional local override.
			printf '1\n'
			;;
		*[!0-9]*|0|'')
			echo "[ci] LOCAL_GO_TEST_JOBS must be auto or a positive integer" >&2
			return 2
			;;
		*)
			printf '%s\n' "${requested}"
			;;
	esac
}

validate_local_release_package_summary() {
	local lane="$1"
	local label="$2"
	local events_path="$3"
	local summary_path="$4"
	local package_metadata_path="$5"
	shift 5
	python3 - "${lane}" "${label}" "${events_path}" "${summary_path}" "${package_metadata_path}" "${repo_root}" "${script_dir}/ci-apex-native-gaps.json" "$@" <<'PY'
import csv
import json
import os
import sys
import tempfile
from pathlib import Path

lane, label, events_path, summary_path, package_metadata_path, repo_root, apex_gaps_path, *arguments = sys.argv[1:]
module = "github.com/glade-sh/glade"
expected = []
for argument in arguments:
    if argument == ".":
        expected.append(module)
    elif argument.startswith("./"):
        expected.append(module + argument[1:])
    else:
        expected.append(argument)
# DataWeave is explicitly deferred by the offline surface scope. Keep only
# these optional installation/engine checks visible as deferred when their
# explicit prerequisites are absent. Configured inputs must execute normally.
import re
engine_inputs = ("GLADE_DATAWEAVE_TEST_CLASSPATH", "GLADE_DATAWEAVE_TEST_JAVA_HOME")
installation_inputs = ("GLADE_DATAWEAVE_TEST_ENGINE", "GLADE_DATAWEAVE_TEST_JAVA_HOME")
deferred_dataweave = {
    (module + "/internal/gladecli", "TestDataWeaveToolchainCLIInstalledJSON"): (installation_inputs, "explicit Java17 JDK and verified DataWeave engine required"),
    (module + "/internal/gladehome", "TestDataWeaveInstallExecuteAndTamper"): (installation_inputs, "explicit Java17 JDK and verified DataWeave engine required"),
    (module + "/internal/dataweave", "TestOfficialEngineSourceAndDenials"): (engine_inputs, "explicit DataWeave engine and Java17 test toolchain required"),
    (module + "/internal/dataweave", "TestOfficialEngineExplicitProjectModules"): (engine_inputs, "explicit DataWeave engine and Java17 test toolchain required"),
    (module + "/internal/dataweave", "TestOfficialEngineTypedApexInputsAndOutput"): (engine_inputs, "explicit DataWeave engine and Java17 test toolchain required"),
    (module + "/internal/dataweave", "TestOfficialEngineUnprovedJavaOutputIsHostError"): (engine_inputs, "explicit DataWeave engine and Java17 test toolchain required"),
    (module + "/internal/dataweave", "TestOfficialEngineQueryRelationshipWriterBoundary"): (engine_inputs, "query relationship contract requires explicit real DataWeave toolchain; configured acceptance must execute this test"),
    (module + "/internal/dataweave", "TestOfficialEngineExtendedTypedValues"): (engine_inputs, "explicit DataWeave engine and Java17 test toolchain required"),
    (module + "/internal/dataweave", "TestVerifyEngineRequiresExactSourceOffers"): (("GLADE_DATAWEAVE_TEST_INSTALLED_ENGINE",), "explicit provisioned engine required"),
    (module + "/internal/dataweave", "TestPinnedEngineInstallation"): (("GLADE_DATAWEAVE_TEST_INSTALL_DIRECTORY",), "explicit engine installation destination required"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceSourceNames"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceChangedSource"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceBuiltinImports"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceRestrictedWordsInLiterals"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceLocalFunctionShadow"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceReadURLDeadBranch"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceEnvironmentDeadBranch"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceReadURLAlias"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceEnvironmentAlias"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveSourceProjectModule"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveTypedAdmitted"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunC5BulkTypedProof"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunC5LegacyAdmitted"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunC5QueryProvenanceAdmitted"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/vm", "TestExecDataWeaveScriptResultCarriers"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/vm", "TestNestedSOQLAPIVersion66"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/vm", "TestNestedSOQLAPIVersion65ThrowsDataWeaveScriptException"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/vm", "TestExecDataWeaveScriptErrorThrowsScriptException"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/vm", "TestExecDataWeaveExcelOutputErrorThrowsScriptException"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/vm", "TestExecDataWeaveMultipleInputsReturnsXMLString"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/vm", "TestExecDataWeaveJsonDateFormatPreservesRecipeFieldOrder"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/vm", "TestExecDataWeaveRecipeConversionsReturnStructuredValues"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
    (module + "/internal/apextest", "TestRunDataWeaveScriptResourceExecutesRuntimeStub"): (("GLADE_DATAWEAVE_APEX_TEST_HOME",), "requires explicitly installed DataWeave toolchain"),
}
# These exact subtests validate captured native rows but deliberately do not
# replay an unsupported bind boundary. See the left_member rows and local_replay
# flag in internal/soql/testdata/soql_safe_navigation_bind/case-table.tsv and
# native/api62/anonymous/org.tsv and native/api67/anonymous/org.tsv.
# Keep the package, complete subtest identity and emitted reason exact; the
# parent test and every other skipped row must still fail validation.
native_boundaries = {
    (module + "/internal/vm", "TestSOQLSafeNavigationBindNativeConformance/" + api + "/left_member"):
        "native boundary is captured but outside this bind replay"
    for api in ("62.0", "67.0")
}
# Push race still reports captured known gaps, rather than treating them as
# passing conformance. Reconstruct the exact expected identity and text from
# the owned native matrix; no other skipped row is excused.
race_known_gaps = {}
if lane == "race" and module + "/internal/sema" in expected:
    fixture = Path(repo_root) / "internal/apextest/testdata/conformance/qualified_receivers"
    with (fixture / "cases.tsv").open(newline="", encoding="utf-8") as source:
        cases = {row["case"]: row for row in csv.DictReader(source, delimiter="\t")}
    with (fixture / "compile.tsv").open(newline="", encoding="utf-8") as source:
        for row in csv.DictReader(source, delimiter="\t"):
            case = cases[row["case"]]
            eligible = (case["kind"] == "known_gap" and row["case"] != "standard_schema_fields_map"
                        or row["route"] == "anonymous" and case["kind"] == "receiver" and row["compiled"] == "false")
            if eligible and row["compiled"] == "false":
                key = "/".join((row["source_api"], row["route"], row["case"]))
                name = "TestAnalyzeQualifiedReceiverNativeControls/KNOWN_GAP/" + key
                reason = f"KNOWN GAP: local analysis accepts {case['expression']}; native compile.tsv row {key} rejects it: {row['problem']}"
                race_known_gaps[(module + "/internal/sema", name)] = reason
if lane == "race":
    with tempfile.TemporaryDirectory() as directory:
        probe = Path(directory) / "caseprobe"
        probe.touch()
        case_sensitive = not (Path(directory) / "CASEPROBE").exists()
else:
    case_sensitive = False
case_reason = "case-variant alias requires a case-insensitive filesystem; this filesystem is case-sensitive"
case_routes = {
    module + "/internal/gladecli": {
        "TestHomeProjectRoutesTwoProcessesPendingInstall": ("Compile", "CompileBatch", "ServerModules", "ServerVendor", "ServerShims", "ServerRuntime", "DevLWC", "DevVF"),
        "TestHomeCLIStatusRoutesTwoProcessesPendingInstall": ("ToolchainStatusCLI", "DoctorCLI"),
    },
    module + "/internal/gladehome": {
        "TestHomeReadersTwoProcessesPendingInstall": ("EnsureRoot", "Root", "LWCToolchainDir", "ShimsDir", "RuntimeAssetDir", "ToolchainStatus"),
    },
}
race_case_skips = {(package, f"{parent}/install_then_{route}/{selector}")
    for package, parents in case_routes.items() for parent, routes in parents.items()
    for route in routes for selector in ("GLADE_HOME_case", "GLADE_ROOT_case")}
readonly_case_routes = {
    module + "/internal/gladehome": {
        "TestHomeReadersCompletedReadOnlyHome": ("EnsureRoot", "Root", "LWCToolchainDir", "ShimsDir", "RuntimeAssetDir", "ToolchainStatus"),
    },
    module + "/internal/gladecli": {
        "TestHomeProjectRoutesCompletedReadOnlyHome": ("Compile", "CompileBatch", "ServerModules", "ServerVendor", "ServerShims", "ServerRuntime", "DevLWC", "DevVF"),
        "TestHomeCLIStatusRoutesCompletedReadOnlyHome": ("ToolchainStatusCLI", "DoctorCLI"),
    },
}
readonly_case_skips = {(package, f"{parent}/{lock}/{route}/{selector}")
    for package, parents in readonly_case_routes.items() for parent, routes in parents.items()
    for lock in ("absent", "read-only", "inaccessible") for route in routes
    for selector in ("GLADE_HOME_case", "GLADE_ROOT_case")}
# Exact leaf identities only: an unexpected skip or different reason remains an error.
with open(apex_gaps_path, encoding="utf-8") as f:
    apex_gaps = json.load(f)
if not isinstance(apex_gaps, list) or len(apex_gaps) != len(set(apex_gaps)) or any(not isinstance(x, str) or "/" not in x for x in apex_gaps):
    raise ValueError("invalid Apex native gap inventory")
native_boundaries.update({(module + "/internal/apextest", name):
    "captured native observation has a known local gap" for name in apex_gaps})
summary = {"version": 1, "lane": lane, "label": label, "valid": False,
           "expected": sorted(expected), "passed": [], "errors": [], "deferred": []}
try:
    if len(expected) != len(set(expected)):
        raise ValueError("expected package list contains duplicates")
    test_files = {}
    with open(package_metadata_path, encoding="utf-8") as source:
        for line_number, line in enumerate(source, 1):
            package, separator, status = line.rstrip("\n").partition("\t")
            if not separator or not package or status not in ("has-tests", "no-tests") or package in test_files:
                raise ValueError(f"invalid package test-file metadata at line {line_number}")
            test_files[package] = status
    metadata_missing = sorted(set(expected) - set(test_files))
    if metadata_missing:
        raise ValueError("package test-file metadata is missing: " + ", ".join(metadata_missing))
    terminal = {}
    selection_path = os.environ.get("CI_SHARD_SELECTION_FOR_SUMMARY")
    expected_terminals = 1
    chunk_tests = []
    chunk_seen = set()
    chunk_index = 0
    if selection_path:
        import importlib.util
        sys.dont_write_bytecode = True
        spec = importlib.util.spec_from_file_location("shard_chunks", os.path.join(repo_root, "scripts", "ci-shard-chunks.py"))
        chunk_module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(chunk_module)
        with open(selection_path, encoding="utf-8") as selection:
            chunk_tests = [set(regex[4:-2].split("|")) for regex in chunk_module.chunks(json.load(selection))]
        expected_terminals = len(chunk_tests)
        if expected_terminals < 1 or len(expected) != 1:
            raise ValueError("invalid shard package selection")
    skipped_tests = []
    test_output = {}
    failed_tests = []
    with open(events_path, encoding="utf-8") as source:
        for line_number, line in enumerate(source, 1):
            if not line.strip():
                continue
            event = json.loads(line)
            if not isinstance(event, dict):
                raise ValueError(f"event {line_number} is not an object")
            package = event.get("Package")
            action = event.get("Action")
            test = event.get("Test")
            if test not in (None, ""):
                if (expected_terminals > 1 and package in expected and isinstance(test, str)
                        and "/" not in test and action in ("pass", "fail", "skip")):
                    chunk_seen.add(test)
                # A package pass only says its process reached a terminal
                # state. It cannot excuse a skipped top-level test or subtest.
                identity = (package, test)
                if action == "output" and isinstance(event.get("Output"), str):
                    test_output.setdefault(identity, []).append(event["Output"])
                if action == "fail":
                    failed_tests.append(f"{package}/{test}")
                if action == "skip":
                    known_reason = race_known_gaps.get(identity)
                    case_skip = lane == "race" and case_sensitive and identity in race_case_skips
                    readonly_skip = lane == "race" and case_sensitive and identity in readonly_case_skips
                    reason = known_reason or (case_reason if case_skip else "case alias requires a case-insensitive filesystem" if readonly_skip else "")
                    source_file = "qualified_receiver_matrix_test.go" if known_reason else "readonly.go" if readonly_skip else "process.go"
                    pattern = r"\s+" + re.escape(source_file) + r":\d+: " + re.escape(reason) + r"\s*$"
                    if reason and any(re.fullmatch(pattern, line)
                            for output in test_output.get(identity, ()) for line in output.splitlines()):
                        summary["deferred"].append({"package": package, "test": test,
                            "skipReason": reason, "reason": "captured known gap" if known_reason else "case-sensitive filesystem"})
                        continue
                    boundary_cause = native_boundaries.get(identity)
                    boundary_pattern = (r"\s+\S+_test.go:\d+: " + re.escape(boundary_cause) + r"\s*$"
                                        if boundary_cause else "")
                    if boundary_cause and any(re.fullmatch(boundary_pattern, line)
                            for output in test_output.get(identity, ()) for line in output.splitlines()):
                        summary["deferred"].append({"package": package, "test": test,
                            "skipReason": boundary_cause,
                            "reason": "captured native boundary; local replay is explicitly excluded by the case table"})
                        continue
                    # These case-alias routes require a case-insensitive filesystem.
                    # Linux runs skip only the case-variant leaf, retaining the
                    # passing parent and all other selector/consumer coverage.
                    project_consumers = r"(?:Compile|CompileBatch|DevLWC|DevVF|ServerModules|ServerRuntime|ServerShims|ServerVendor)"
                    cli_consumers = r"(?:DoctorCLI|ToolchainStatusCLI)"
                    case_leaf = r"GLADE_(?:HOME|ROOT)_case"
                    completed = r"(?:TestHomeProjectRoutesCompletedReadOnlyHome/(?:absent|inaccessible|read-only)/" + project_consumers + r"|TestHomeCLIStatusRoutesCompletedReadOnlyHome/(?:absent|inaccessible|read-only)/" + cli_consumers + ")/" + case_leaf
                    pending = r"(?:TestHomeProjectRoutesTwoProcessesPendingInstall/install_then_" + project_consumers + r"|TestHomeCLIStatusRoutesTwoProcessesPendingInstall/install_then_" + cli_consumers + ")/" + case_leaf
                    reason = None
                    if lane == "gladecli-ci" and package == module + "/internal/gladecli":
                        if re.fullmatch(completed, test):
                            reason = r"\s+readonly\.go:\d+: case alias requires a case-insensitive filesystem\s*$"
                        elif re.fullmatch(pending, test):
                            reason = r"\s+process\.go:\d+: case-variant alias requires a case-insensitive filesystem; this filesystem is case-sensitive\s*$"
                    if reason and any(re.fullmatch(reason, line) for output in test_output.get(identity, ()) for line in output.splitlines()):
                        continue
                    inputs, cause = deferred_dataweave.get(identity, ((), ""))
                    missing = [name for name in inputs if not os.environ.get(name)]
                    reason_pattern = r"\s+\S+_test.go:\d+: " + re.escape(cause) + r"\s*$"
                    observed_cause = bool(cause) and any(re.fullmatch(reason_pattern, line)
                        for output in test_output.get(identity, ()) for line in output.splitlines())
                    if missing and observed_cause:
                        summary["deferred"].append({"package": package, "test": test,
                            "skipReason": cause,
                            "reason": "DataWeave is deferred; missing explicit prerequisites: " + ", ".join(missing)})
                    else:
                        skipped_tests.append(f"{package}/{test}")
                continue
            if not isinstance(package, str) or action not in ("pass", "fail", "skip"):
                continue
            if expected_terminals > 1 and package in expected:
                if chunk_index >= expected_terminals or chunk_seen != chunk_tests[chunk_index]:
                    summary["errors"].append(f"package result does not match shard chunk {chunk_index + 1}")
                chunk_index += 1
                chunk_seen = set()
            terminal.setdefault(package, []).append(action)
    if expected_terminals > 1 and chunk_seen:
        summary["errors"].append("test results after final package result")
    missing = sorted(set(expected) - set(terminal))
    duplicates = sorted(package for package, actions in terminal.items() if len(actions) != expected_terminals)
    skipped = sorted(package for package, actions in terminal.items() if "skip" in actions and test_files.get(package) != "no-tests")
    failed = sorted(package for package, actions in terminal.items() if "fail" in actions)
    extra = sorted(set(terminal) - set(expected))
    if missing:
        summary["errors"].append("missing package results: " + ", ".join(missing))
    if duplicates:
        summary["errors"].append("duplicate package results: " + ", ".join(duplicates))
    if skipped:
        summary["errors"].append("skipped package results: " + ", ".join(skipped))
    if skipped_tests:
        summary["errors"].append("skipped test results: " + ", ".join(sorted(skipped_tests)))
    if failed:
        summary["errors"].append("failed package results: " + ", ".join(failed))
    if failed_tests:
        summary["errors"].append("failed test results: " + ", ".join(sorted(failed_tests)))
    if extra:
        summary["errors"].append("extra package results: " + ", ".join(extra))
    summary["passed"] = sorted(package for package, actions in terminal.items() if len(actions) == expected_terminals and (all(action == "pass" for action in actions) or actions == ["skip"]) and package in expected)
    summary["valid"] = not summary["errors"] and summary["passed"] == sorted(expected)
    if not summary["valid"] and not summary["errors"]:
        summary["errors"].append("package summary is incomplete")
except Exception as error:
    summary["errors"].append(str(error))
os.makedirs(os.path.dirname(summary_path) or ".", exist_ok=True)
with open(summary_path, "w", encoding="utf-8") as target:
    json.dump(summary, target, sort_keys=True, indent=2)
    target.write("\n")
for deferred in summary["deferred"]:
    print(f"[ci] {label} deferred {deferred['package']}/{deferred['test']}: {deferred['reason']}")
if not summary["valid"]:
    for error in summary["errors"]:
        print(f"[ci] {label} package summary rejected: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
}

run_local_release_lane() {
	local lane="$1"
	# Full server validation requires L08. Ordinary CI explicitly routes it to
	# the provisioned Browser authority with -skip below.
	if [[ "${lane}" == "server-and-playground" ]]; then
		local -x GLADE_LWC_BROWSER=1
	fi
	local label="$2"
	local timeout="$3"
	local artifact_root="$4"
	local skip_regex="${5:-}"
	local lane_dir="${artifact_root}/${label}"
	local events_path="${lane_dir}/events.json"
	local summary_path="${lane_dir}/package-summary.json"
	local native_rc validation_rc=0
	local -a packages=()
	# Load in this shell so the authoritative test-file metadata survives the
	# package collection; process substitution would otherwise hide its globals.
	load_package_lanes
	while IFS= read -r package; do
		packages+=("${package}")
	done < <(awk -F '\t' -v lane="${lane}" '$1 == lane { print $2 }' <<<"${package_lane_rows}")
	if [[ "${#packages[@]}" -eq 0 ]]; then
		echo "[ci] local-release lane ${lane} is empty" >&2
		return 1
	fi
	mkdir -p "${lane_dir}"
	python3 - "${lane}" "${label}" "${summary_path}" <<'PY'
import json
import sys

lane, label, summary_path = sys.argv[1:]
with open(summary_path, "w", encoding="utf-8") as target:
    json.dump({
        "version": 1,
        "lane": lane,
        "label": label,
        "valid": False,
        "errors": ["lane did not reach result validation"],
    }, target, sort_keys=True, indent=2)
    target.write("\n")
PY
	set +e
	local -a test_args=(-count=1 -timeout="${timeout}")
	if [[ -n "${skip_regex}" ]]; then
		test_args+=(-skip "${skip_regex}")
	fi
	run_json_with_heartbeat "go test local-release ${label}" "${events_path}" "${test_args[@]}" "${packages[@]}"
	native_rc="$?"
	set -e
	validate_local_release_package_summary "${lane}" "${label}" "${events_path}" "${summary_path}" "${package_test_metadata_temp}" "${packages[@]}" || validation_rc="$?"
	if [[ "${native_rc}" -ne 0 ]]; then
		return "${native_rc}"
	fi
	return "${validation_rc}"
}

wait_local_release_lane() {
	local pid="$1"
	local rc=0
	wait "${pid}" || rc="$?"
	unregister_owned_child "${pid}"
	return "${rc}"
}

# Successful vet results can be shared across checkouts. Cached diagnostics can
# retain the producing checkout's paths, so only failures pay for an untrimmed
# rerun. The explicit false also overrides -trimpath inherited through GOFLAGS.
run_vet() (
	local diagnostics
	diagnostics="$(mktemp "${TMPDIR:-/tmp}/glade-ci-vet.XXXXXX")"
	trap 'rm -f "${diagnostics}"' EXIT
	if go vet -trimpath ./... >"${diagnostics}" 2>&1; then
		cat "${diagnostics}"
	else
		go vet -trimpath=false ./...
	fi
)

run_local_release() {
	local_release_jobs >/dev/null
	local artifact_root="${CI_LOCAL_RELEASE_ARTIFACT_DIR:-ci-artifacts/local-release}"
	mkdir -p "${artifact_root}"
	printf '[ci] local release lanes: jobs=1 artifact_dir=%s\n' "${artifact_root}"
	run_with_heartbeat "go vet ./..." run_vet
	run_local_release_lane "repoguard" "guard" 30m "${artifact_root}"
	run_local_release_lane "gladecli" "cli-ui" 30m "${artifact_root}"
	run_local_release_lane "sema" "sema" 30m "${artifact_root}"
	run_local_release_lane "apextest" "apex" 90m "${artifact_root}"
	run_local_release_lane "server-and-playground" "server-playground" 30m "${artifact_root}"
	run_local_release_lane "remaining-go" "remaining" 70m "${artifact_root}"
}

run_selected_local_release_lane() {
	local lane="$1"
	local artifact_root="${CI_LOCAL_RELEASE_ARTIFACT_DIR:-ci-artifacts/local-release}"
	local_release_jobs >/dev/null
	case "${lane}" in
		vet)
			run_with_heartbeat "go vet ./..." run_vet
			;;
		guard)
			run_local_release_lane "repoguard" "guard" 30m "${artifact_root}"
			;;
		cli-ui)
			run_local_release_lane "gladecli" "cli-ui" 30m "${artifact_root}"
			;;
		sema)
			run_local_release_lane "sema" "sema" 30m "${artifact_root}"
			;;
		apex-0|apex-1)
			CI_APEXTEST_ARTIFACT_DIR="${artifact_root}/${lane}" run_apextest_matrix_shard "${lane#apex-}" local-release
			;;
		server-playground)
			run_local_release_lane "server-and-playground" "server-playground" 30m "${artifact_root}"
			;;
		remaining)
			run_local_release_lane "remaining-go" "remaining" 70m "${artifact_root}"
			;;
		*)
			echo "usage: scripts/ci-go-test.sh local-release-lane {vet|guard|cli-ui|sema|apex-0|apex-1|server-playground|remaining}" >&2
			return 2
			;;
	esac
}

run_race_tests() {
	run_package_lane "apextest" race 60m
	run_package_lane "gladecli" race 60m
	run_package_lane "sema" race 60m
	run_package_lane "server-and-playground" race 60m
	run_package_lane "repoguard" race 60m
	run_package_lane "remaining-go" race 30m
}

validate_package_discovery() {
	local raw_path="$1"
	local discovery_path="$2"
	local package_name="$3"
	local label="$4"
	local strict="${5:-0}"
	python3 - "${raw_path}" "${discovery_path}" "${package_name}" "${label}" "${strict}" <<'PY'
import re
import sys

raw_path, discovery_path, package_name, label, strict_text = sys.argv[1:]
strict = strict_text == "1"
names = []
trailers = 0
with open(raw_path, encoding="utf-8") as source:
    for raw_line in source:
        line = raw_line.rstrip("\n")
        if re.fullmatch(r"Test[A-Za-z0-9_]*", line):
            names.append(line)
        elif re.match(r"^ok\s+" + re.escape(package_name) + r"(?:\s|$)", line):
            trailers += 1
            continue
        elif strict and not line:
            raise SystemExit(f"invalid {label} discovery output: empty line")
        elif line:
            raise SystemExit(f"invalid {label} discovery output: {line!r}")
if not names:
    raise SystemExit(f"no {label} tests discovered")
if len(names) != len(set(names)):
    raise SystemExit(f"duplicate {label} test name")
if strict and trailers != 1:
    raise SystemExit(f"{label} discovery package trailer count is {trailers}, want 1")
names.sort()
with open(discovery_path, "w", encoding="utf-8") as target:
    target.write("\n".join(names) + "\n")
PY
}

validate_discovery() {
	validate_package_discovery "$1" "$2" "github.com/glade-sh/glade/internal/apextest" "Apex"
}

select_and_validate_shard() {
	local discovery_path="$1"
	local plan_path="$2"
	local index="$3"
	local selected_path="$4"
	local package_name="$5"
	local shard_count="${6:-2}"
	python3 - "${discovery_path}" "${plan_path}" "${index}" "${selected_path}" "${package_name}" "${shard_count}" <<'PY'
import json
import re
import sys

discovery_path, plan_path, index_text, selected_path, package_name, shard_count_text = sys.argv[1:]
index = int(index_text)
shard_count = int(shard_count_text)
with open(discovery_path, encoding="utf-8") as source:
    discovered = [line.rstrip("\n") for line in source]
with open(plan_path, encoding="utf-8") as source:
    plan = json.load(source)
if plan.get("version") != 1 or plan.get("package") != package_name:
    raise SystemExit("planner returned wrong schema or package")
shards = plan.get("shards")
if not isinstance(shards, list) or len(shards) != shard_count:
    raise SystemExit(f"planner did not return exactly {shard_count} shards")
seen = []
for expected_index, shard in enumerate(shards):
    if not isinstance(shard, dict) or shard.get("index") != expected_index:
        raise SystemExit("planner returned invalid shard index")
    tests = shard.get("tests")
    if not isinstance(tests, list) or not tests or tests != sorted(tests):
        raise SystemExit("planner returned empty or non-canonical shard")
    if any(not isinstance(name, str) or not re.fullmatch(r"Test[A-Za-z0-9_]*", name) for name in tests):
        raise SystemExit("planner returned invalid test name")
    if not isinstance(shard.get("regex"), str) or not shard["regex"]:
        raise SystemExit("planner returned invalid regex")
    if shard["regex"] != "^(?:" + "|".join(re.escape(name) for name in tests) + ")$":
        raise SystemExit("planner returned non-canonical exact-test regex")
    seen.extend(tests)
if len(seen) != len(set(seen)) or sorted(seen) != sorted(discovered):
    raise SystemExit("planner shard union does not match discovery")
with open(selected_path, "w", encoding="utf-8") as target:
    json.dump(shards[index], target, sort_keys=True, indent=2)
    target.write("\n")
PY
}

render_failure_output() {
	local events_path="$1"
	local label="${2:-Apex}"
	python3 - "${events_path}" "${label}" <<'PY' || true
import json
import sys

try:
    with open(sys.argv[1], encoding="utf-8") as source:
        for line in source:
            event = json.loads(line)
            if isinstance(event.get("Output"), str):
                print(event["Output"], end="")
except Exception as error:
    print(f"unable to render {sys.argv[2]} JSON failure output: {error}", file=sys.stderr)
PY
}

validate_shard_results() {
	local selected_path="$1"
	local events_path="$2"
	local summary_path="$3"
	local package_name="$4"
	local label="$5"
	local package_summary_path="${6:-}"
	python3 - "${selected_path}" "${events_path}" "${summary_path}" "${package_name}" "${label}" "${package_summary_path}" <<'PY'
import json
import sys

selected_path, events_path, summary_path, package_name, label, package_summary_path = sys.argv[1:]
summary = {"valid": False, "expected": [], "passed": [], "errors": []}
try:
    deferred = set()
    if package_summary_path:
        with open(package_summary_path, encoding="utf-8") as source:
            package_summary = json.load(source)
        if not package_summary.get("valid"):
            raise ValueError("package summary is invalid")
        deferred = {row["test"] for row in package_summary["deferred"] if row["package"] == package_name}
    with open(selected_path, encoding="utf-8") as source:
        selected = json.load(source)
    expected = selected["tests"]
    summary["expected"] = expected
    terminal = {}
    with open(events_path, encoding="utf-8") as source:
        for line_number, line in enumerate(source, 1):
            if not line.strip():
                continue
            event = json.loads(line)
            if not isinstance(event, dict):
                raise ValueError(f"event {line_number} is not an object")
            package = event.get("Package")
            if package != package_name:
                raise ValueError(f"event {line_number} has wrong package {package!r}")
            name = event.get("Test")
            action = event.get("Action")
            if not isinstance(name, str) or "/" in name or action not in ("pass", "fail", "skip"):
                continue
            terminal.setdefault(name, []).append(action)
    passed = sorted(name for name, actions in terminal.items() if actions == ["pass"])
    summary["passed"] = passed
    deferred_results = sorted(name for name, actions in terminal.items() if actions == ["skip"] and name in deferred)
    if package_summary_path:
        summary["deferred"] = deferred_results
    if sorted(passed + deferred_results) != sorted(expected):
        summary["errors"].append("top-level passing set does not match selected shard")
    duplicates = sorted(name for name, actions in terminal.items() if len(actions) != 1)
    if duplicates:
        summary["errors"].append("duplicate top-level terminal results: " + ", ".join(duplicates))
    wrong = sorted(name for name, actions in terminal.items() if actions != ["pass"] and name not in deferred_results)
    if wrong:
        summary["errors"].append("non-passing top-level results: " + ", ".join(wrong))
    extras = sorted(set(terminal) - set(expected))
    if extras:
        summary["errors"].append("extra top-level results: " + ", ".join(extras))
    missing = sorted(set(expected) - set(terminal))
    if missing:
        summary["errors"].append("missing top-level results: " + ", ".join(missing))
    summary["valid"] = not summary["errors"]
except Exception as error:
    summary["errors"].append(str(error))
with open(summary_path, "w", encoding="utf-8") as target:
    json.dump(summary, target, sort_keys=True, indent=2)
    target.write("\n")
if not summary["valid"]:
    for error in summary["errors"]:
        print(f"{label} result validation: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
}

filter_node_integration_discovery() {
	local discovery_path="$1"
	local artifact_dir="$2"
	local package_name="$3"
	local authority_path="${artifact_dir}/node-integration-expected.tsv"
	write_node_integration_expected "${authority_path}"
	cp "${discovery_path}" "${artifact_dir}/discovery-full.txt"
	python3 - "${artifact_dir}/discovery-full.txt" "${authority_path}" "${discovery_path}" "${package_name}" <<'PY'
import sys

full_path, authority_path, discovery_path, package_name = sys.argv[1:]
with open(full_path, encoding="utf-8") as source:
    discovered = source.read().splitlines()
with open(authority_path, encoding="utf-8") as source:
    excluded = [name for package, name in (line.rstrip("\n").split("\t") for line in source)
                if package == package_name]
if not excluded or len(excluded) != len(set(excluded)):
    raise SystemExit("invalid Node authority selection for " + package_name)
missing = sorted(set(excluded) - set(discovered))
if missing:
    raise SystemExit("Node authority tests missing from discovery: " + ", ".join(missing))
remaining = sorted(set(discovered) - set(excluded))
if not remaining or sorted(remaining + excluded) != discovered:
    raise SystemExit("package and Node authority union does not match discovery")
with open(discovery_path, "w", encoding="utf-8") as target:
    target.write("\n".join(remaining) + "\n")
PY
}

run_test_matrix_shard() {
	local index="${1:-}"
	local package_name="$2"
	local package_arg="$3"
	local lane="$4"
	local label="$5"
	local artifact_dir="$6"
	local history_path="$7"
	local strict_discovery="$8"
	local mode="${9:-test}"
	local shard_count="${10:-2}"
	local index_pattern='^[01]$'
	local index_description='0 or 1'
	local -a lane_packages=()
	local -a test_args=(-timeout=30m)
	local package_summary_path=""
	if [[ "${mode}" == "local-release" ]]; then
		test_args=(-count=1 -timeout=90m)
	fi
	if [[ "${mode}" == "apex-ci" ]]; then
		test_args=(-count=1 -timeout=90m)
		if [[ "${shard_count}" == "16" ]]; then
			index_pattern='^([0-9]|1[0-5])$'
			index_description='between 0 and 15'
		elif [[ "${shard_count}" == "12" ]]; then
			index_pattern='^([0-9]|1[01])$'
			index_description='between 0 and 11'
		else
			index_pattern='^[0-5]$'
			index_description='between 0 and 5'
		fi
	fi
	if [[ "${mode}" == "lwc-compile" ]]; then
		test_args=(-count=1 -timeout=90m)
		index_pattern='^([0-9]|1[01])$'
		index_description='between 0 and 11'
	fi
	if [[ "${mode}" == "visualforce" ]]; then
		test_args=(-count=1 -timeout=70m)
		index_pattern='^[0-7]$'
		index_description='between 0 and 7'
	fi
	if [[ "${mode}" == "vm-ci" || "${mode}" == "gladecli-ci" ]]; then
		test_args=(-count=1 -timeout=30m)
		# PR workers keep four shards; the push matrix passes a smaller count.
		case "${shard_count}" in
			2) index_pattern='^[01]$'; index_description='0 or 1' ;;
			3) index_pattern='^[0-2]$'; index_description='between 0 and 2' ;;
			*) index_pattern='^[0-3]$'; index_description='between 0 and 3' ;;
		esac
	fi
	if [[ "${mode}" == "local-release" || "${mode}" == "lwc-compile" || "${mode}" == "visualforce" || "${mode}" == "vm-ci" || "${mode}" == "gladecli-ci" || "${lane}" == "apextest" ]]; then
		package_summary_path="${artifact_dir}/package-summary.json"
	fi
	local artifact_suffix="invalid"
	if [[ "${index}" =~ ${index_pattern} ]]; then
		artifact_suffix="${index}"
	fi
	local discovery_raw="${artifact_dir}/discovery-command.txt"
	local discovery_stderr="${artifact_dir}/discovery-stderr.txt"
	local discovery="${artifact_dir}/discovery.txt"
	local plan="${artifact_dir}/plan.json"
	local selected="${artifact_dir}/selected-shard.json"
	local events="${artifact_dir}/events.json"
	local summary="${artifact_dir}/validation-summary.json"
	local regex
	local discovery_rc
	local native_rc
	local validation_rc=0

	mkdir -p "${artifact_dir}"
	: >"${discovery_raw}"
	: >"${discovery}"
	: >"${discovery_stderr}"
	: >"${plan}"
	: >"${selected}"
	: >"${events}"
	printf '{"valid": false, "errors": ["shard did not reach result validation"]}\n' >"${summary}"
	if [[ -n "${package_summary_path}" ]]; then
		printf '{"valid": false, "errors": ["shard did not reach result validation"]}\n' >"${package_summary_path}"
	fi
	if [[ ! "${index}" =~ ${index_pattern} ]]; then
		echo "shard index must be ${index_description}" >&2
		return 2
	fi
	if [[ -z "${CI_SHARD_PLANNER:-}" ]]; then
		while IFS= read -r pkg; do
			lane_packages+=("${pkg}")
		done < <(package_lane_packages "${lane}")
		if [[ "${mode}" == "lwc-compile" || "${mode}" == "visualforce" || "${mode}" == "vm-ci" ]]; then
			if [[ "$(printf '%s\n' "${lane_packages[@]}" | awk -v pkg="${package_arg}" '$0 == pkg { n++ } END { print n+0 }')" -ne 1 ]]; then
				echo "[ci] ${label} must occur exactly once in the remaining-go package lane" >&2
				return 1
			fi
		elif [[ "${#lane_packages[@]}" -ne 1 ]]; then
			echo "[ci] ${lane} package lane must contain exactly one package" >&2
			return 1
		else
			package_arg="${lane_packages[0]}"
		fi
	fi

	set +e
	if [[ -n "${CI_TEST_BIN_DIR:-}" ]]; then
		python3 "${script_dir}/ci-test-binary.py" test -trimpath -list '^Test' "${package_arg}"
	else
		go test -vet=off -trimpath -list '^Test' "${package_arg}"
	fi >"${discovery_raw}" 2>"${discovery_stderr}"
	discovery_rc="$?"
	set -e
	if [[ -s "${discovery_stderr}" ]]; then
		cat "${discovery_stderr}" >&2
	fi
	if [[ "${discovery_rc}" -ne 0 ]]; then
		cat "${discovery_raw}" >&2
		return "${discovery_rc}"
	fi
	validate_package_discovery "${discovery_raw}" "${discovery}" "${package_name}" "${label}" "${strict_discovery}"
	if [[ "${mode}" == "lwc-compile" || "${mode}" == "gladecli-ci" ]]; then
		filter_node_integration_discovery "${discovery}" "${artifact_dir}" "${package_name}"
	fi

	if [[ -n "${CI_SHARD_PLANNER:-}" ]]; then
		planner=("${CI_SHARD_PLANNER}" --package "${package_name}" --shards "${shard_count}" --tests "${discovery}")
	else
		planner=(go run ./scripts/internal/cishard --package "${package_name}" --shards "${shard_count}" --tests "${discovery}")
	fi
	if [[ -s "${history_path}" ]]; then
		planner+=(--history "${history_path}")
	fi
	"${planner[@]}" >"${plan}"
	select_and_validate_shard "${discovery}" "${plan}" "${index}" "${selected}" "${package_name}" "${shard_count}"
	local chunks_file="${artifact_dir}/chunk-regexes.txt"
	python3 "${script_dir}/ci-shard-chunks.py" "${selected}" >"${chunks_file}"
	native_rc=0
	local chunk_rc
	local testlog_append=1
	if [[ "${mode}" == "local-release" && "${CI_LOCAL_RELEASE_METRICS:-0}" == "1" ]]; then
		: >"${events%.json}.time.txt"
	fi
	while IFS= read -r regex; do
		chunk_rc=0
		run_json_with_heartbeat "go test ${label} shard ${index}" "${events}" "${test_args[@]}" -run "${regex}" "${package_arg}" || chunk_rc="$?"
		if [[ "${chunk_rc}" -ne 0 && "${native_rc}" -eq 0 ]]; then native_rc="${chunk_rc}"; fi
	done <"${chunks_file}"
	if [[ -n "${package_summary_path}" ]]; then
		# Successful discovery above establishes that this exact package has tests.
		local shard_metadata="${artifact_dir}/package-test-metadata.tsv"
		printf '%s\thas-tests\n' "${package_name}" >"${shard_metadata}"
		local validation_lane="${lane}"
		[[ "${mode}" == "gladecli-ci" ]] && validation_lane="gladecli-ci"
		CI_SHARD_SELECTION_FOR_SUMMARY="${selected}" validate_local_release_package_summary "${validation_lane}" "${label} shard ${index}" "${events}" "${package_summary_path}" "${shard_metadata}" "${package_arg}" || validation_rc="$?"
	fi
	validate_shard_results "${selected}" "${events}" "${summary}" "${package_name}" "${label}" "${package_summary_path}" || validation_rc="$?"
	if [[ "${native_rc}" -ne 0 || "${validation_rc}" -ne 0 ]]; then
		render_failure_output "${events}" "${label}"
	fi
	if [[ "${native_rc}" -ne 0 ]]; then
		return "${native_rc}"
	fi
	return "${validation_rc}"
}

run_apextest_matrix_shard() {
	local index="${1:-}"
	local mode="${2:-test}"
	local label="Apex"
	local shard_count="${CI_APEXTEST_SHARDS:-2}"
	local index_pattern='^[01]$'
	if [[ "${mode}" == "local-release" ]]; then
		label="local-release Apex"
		shard_count="2"
		index_pattern='^[01]$'
	elif [[ "${shard_count}" == "6" || "${shard_count}" == "12" || "${shard_count}" == "16" ]]; then
		mode="apex-ci"
		if [[ "${shard_count}" == "16" ]]; then
			index_pattern='^([0-9]|1[0-5])$'
		elif [[ "${shard_count}" == "12" ]]; then
			index_pattern='^([0-9]|1[01])$'
		else
			index_pattern='^[0-5]$'
		fi
	elif [[ "${shard_count}" != "2" ]]; then
		echo "CI_APEXTEST_SHARDS must be 2, 6, 12, or 16" >&2
		return 2
	fi
	local artifact_suffix="invalid"
	local history_path=""
	[[ "${index}" =~ ${index_pattern} ]] && artifact_suffix="${index}"
	if [[ -s "${CI_APEXTEST_HISTORY_PATH:-}" ]]; then
		history_path="${CI_APEXTEST_HISTORY_PATH}"
	fi
	# The generic runner executes the legacy Apex operations exactly once:
	# go test -vet=off -trimpath -list '^Test' "${apex_package}"
	# planner+=(--history "${CI_APEXTEST_HISTORY_PATH}")
	# run_json_with_heartbeat "go test Apex shard ${index}" "${events}" -timeout=90m -trimpath -run "${regex}" "${apex_package}"
	# CI opts into six, twelve or sixteen shards; local helpers default to two shards.
	# About 280 measured-plan Apex minutes plus 25% test growth
	# estimates 58 minutes per CI shard, or 87 with a 1.5x margin.
	# Allow 90 minutes for cold history and runner variance.
	# Local release trains retain their established two-shard partition.
	run_test_matrix_shard "${index}" "github.com/glade-sh/glade/internal/apextest" "./internal/apextest" "apextest" "${label}" "${CI_APEXTEST_ARTIFACT_DIR:-ci-artifacts/apextest-${artifact_suffix}}" "${history_path}" "0" "${mode}" "${shard_count}"
}

run_sema_matrix_shard() {
	local index="${1:-}"
	local artifact_suffix="invalid"
	[[ "${index}" =~ ^[01]$ ]] && artifact_suffix="${index}"
	run_test_matrix_shard "${index}" "github.com/glade-sh/glade/internal/sema" "./internal/sema" "sema" "sema" "${CI_SEMA_ARTIFACT_DIR:-ci-artifacts/sema-${artifact_suffix}}" "${CI_SEMA_HISTORY_PATH:-}" "1"
}

run_lwc_compile_matrix_shard() {
	local index="${1:-}"
	local artifact_suffix="invalid"
	[[ "${index}" =~ ^([0-9]|1[01])$ ]] && artifact_suffix="${index}"
	run_test_matrix_shard "${index}" "github.com/glade-sh/glade/internal/lwc/compile" "./internal/lwc/compile" "remaining-go" "LWC compile" "${CI_LWC_COMPILE_ARTIFACT_DIR:-ci-artifacts/lwc-compile-${artifact_suffix}}" "${CI_LWC_COMPILE_HISTORY_PATH:-}" "1" "lwc-compile" "12"
}

run_visualforce_matrix_shard() {
	local index="${1:-}"
	local artifact_suffix="invalid"
	[[ "${index}" =~ ^[0-7]$ ]] && artifact_suffix="${index}"
	run_test_matrix_shard "${index}" "github.com/glade-sh/glade/internal/visualforce" "./internal/visualforce" "remaining-go" "Visualforce" "${CI_VISUALFORCE_ARTIFACT_DIR:-ci-artifacts/visualforce-${artifact_suffix}}" "${CI_VISUALFORCE_HISTORY_PATH:-}" "1" "visualforce" "8"
}

run_vm_matrix_shard() {
	local index="${1:-}"
	local artifact_suffix="invalid"
	local shard_count="${CI_VM_SHARDS:-4}"
	if [[ "${shard_count}" != "3" && "${shard_count}" != "4" ]]; then
		echo "CI_VM_SHARDS must be 3 or 4" >&2
		return 2
	fi
	[[ "${index}" =~ ^[0-3]$ && "${index}" -lt "${shard_count}" ]] && artifact_suffix="${index}"
	run_test_matrix_shard "${index}" "github.com/glade-sh/glade/internal/vm" "./internal/vm" "remaining-go" "VM" "${CI_VM_ARTIFACT_DIR:-ci-artifacts/vm-${artifact_suffix}}" "${CI_VM_HISTORY_PATH:-}" "1" "vm-ci" "${shard_count}"
}

run_gladecli_matrix_shard() {
	local index="${1:-}"
	local artifact_suffix="invalid"
	local shard_count="${CI_GLADECLI_SHARDS:-4}"
	if [[ "${shard_count}" != "2" && "${shard_count}" != "4" ]]; then
		echo "CI_GLADECLI_SHARDS must be 2 or 4" >&2
		return 2
	fi
	[[ "${index}" =~ ^[0-3]$ && "${index}" -lt "${shard_count}" ]] && artifact_suffix="${index}"
	run_test_matrix_shard "${index}" "github.com/glade-sh/glade/internal/gladecli" "./internal/gladecli" "gladecli" "gladecli" "${CI_GLADECLI_ARTIFACT_DIR:-ci-artifacts/gladecli-${artifact_suffix}}" "${CI_GLADECLI_HISTORY_PATH:-}" "1" "gladecli-ci" "${shard_count}"
}

refresh_test_history() {
	local package_name="$1"
	local label="$2"
	local prefix="$3"
	local expected_count="$4"
	local shard_count="$5"
	local output="$6"
	shift 6
	if [[ -z "${output}" || "$#" -ne "${shard_count}" ]]; then
		echo "[ci] ${prefix} history requires ${shard_count} shard directories and an output" >&2
		return 2
	fi
	local shard_dir
	for shard_dir in "$@"; do
		if [[ -z "${shard_dir}" ]]; then
			echo "[ci] ${prefix} history requires nonempty shard directories" >&2
			return 2
		fi
	done
	python3 - "${package_name}" "${label}" "${prefix}" "${expected_count}" "${shard_count}" "${output}" "${script_dir}/ci-go-test.sh" "$@" <<'PY'
import datetime
import json
import math
import os
import re
import sys
import tempfile
import subprocess

PACKAGE = sys.argv[1]
LABEL = sys.argv[2]
PREFIX = sys.argv[3]
EXPECTED_COUNT = int(sys.argv[4])
SHARD_COUNT = int(sys.argv[5])
SCRIPT = sys.argv[7]
TEST_NAME = re.compile(r"Test[A-Za-z0-9_]*\Z")
MAX_MILLIS = (1 << 63) - 1
include_deferred = ((PACKAGE == "github.com/glade-sh/glade/internal/apextest"
                     and os.environ.get("CI_APEXTEST_HISTORY_COMPLETE") == "1")
                    or (PACKAGE == "github.com/glade-sh/glade/internal/vm"
                        and os.environ.get("CI_VM_HISTORY_COMPLETE") == "1"))

shard_dirs = sys.argv[8:]
output_path = sys.argv[6]
profile_path = os.environ.get("CI_APEXTEST_SCHEDULING_PROFILE", "") if PACKAGE == "github.com/glade-sh/glade/internal/apextest" else ""
if profile_path and os.path.abspath(profile_path) == os.path.abspath(output_path):
    raise SystemExit("scheduling profile must be separate from accepted history")
if profile_path:
    try:
        os.unlink(profile_path)
    except FileNotFoundError:
        pass
try:
    os.unlink(output_path)
except FileNotFoundError:
    pass

def reject(message):
    raise ValueError(message)

def no_duplicate_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            reject(f"duplicate JSON key {key!r}")
        result[key] = value
    return result

def load_json(path):
    with open(path, encoding="utf-8") as source:
        return json.load(source, object_pairs_hook=no_duplicate_object)

def require_exact_keys(value, keys, label):
    if not isinstance(value, dict) or set(value) != set(keys):
        reject(f"{label} does not have the exact schema")

def is_integer(value):
    return isinstance(value, int) and not isinstance(value, bool)

# Apex conformance tests do most of their work in parallel subtests, so their
# shards are weighed and balance-checked by wall time. Other packages keep
# Elapsed.
WALL_WEIGHTS = PACKAGE == "github.com/glade-sh/glade/internal/apextest"

def event_seconds(event):
    """Return the event's Time as epoch seconds, or None when unusable."""
    stamp = event.get("Time")
    if not isinstance(stamp, str):
        return None
    match = re.fullmatch(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(\.\d+)?(Z|[+-]\d{2}:\d{2})", stamp)
    if match is None:
        return None
    try:
        base = datetime.datetime.fromisoformat(match.group(1) + ("+00:00" if match.group(3) == "Z" else match.group(3)))
    except ValueError:
        return None
    return base.timestamp() + float(match.group(2) or 0)

def load_discovery(path):
    with open(path, encoding="utf-8") as source:
        raw = source.read()
    names = raw.splitlines()
    if not names:
        reject("discovery contains no tests")
    if EXPECTED_COUNT and len(names) != EXPECTED_COUNT:
        reject(f"discovery contains {len(names)} tests, want {EXPECTED_COUNT}")
    if any(not TEST_NAME.fullmatch(name) for name in names):
        reject("discovery contains an invalid top-level test name")
    if len(set(names)) != len(names):
        reject("discovery contains duplicate tests")
    if names != sorted(names) or raw != "\n".join(names) + "\n":
        reject("discovery is not in exact canonical order and format")
    return names

def validate_shard(shard, index, label):
    require_exact_keys(shard, ("index", "tests", "estimatedDurationMillis", "regex"), label)
    if not is_integer(shard["index"]) or shard["index"] != index:
        reject(f"{label} has an invalid shard index")
    tests = shard["tests"]
    if not isinstance(tests, list) or not tests or tests != sorted(tests):
        reject(f"{label} has an empty or non-canonical test list")
    if any(not isinstance(name, str) or not TEST_NAME.fullmatch(name) for name in tests):
        reject(f"{label} has an invalid top-level test name")
    estimate = shard["estimatedDurationMillis"]
    if not is_integer(estimate) or estimate < 0:
        reject(f"{label} has an invalid estimated duration")
    canonical_regex = "^(?:" + "|".join(re.escape(name) for name in tests) + ")$"
    if not isinstance(shard["regex"], str) or shard["regex"] != canonical_regex:
        reject(f"{label} does not have the canonical exact-test regex")
    return tests

try:
    discoveries = [load_discovery(os.path.join(path, "discovery.txt")) for path in shard_dirs]
    if any(discovery != discoveries[0] for discovery in discoveries[1:]):
        reject("shard discoveries do not match")
    discovery = discoveries[0]

    plans = [load_json(os.path.join(path, "plan.json")) for path in shard_dirs]
    if any(plan != plans[0] for plan in plans[1:]):
        reject("shard plans do not match")
    plan = plans[0]
    require_exact_keys(plan, ("version", "package", "historyUsed", "shards"), "plan")
    if not is_integer(plan["version"]) or plan["version"] != 1 or not isinstance(plan["package"], str) or plan["package"] != PACKAGE:
        reject("plan has wrong schema or package")
    if not isinstance(plan["historyUsed"], bool):
        reject("plan historyUsed is not boolean")
    shards = plan["shards"]
    if not isinstance(shards, list) or len(shards) != SHARD_COUNT:
        reject(f"plan does not contain exactly {SHARD_COUNT} shards")
    planned = []
    for index, shard in enumerate(shards):
        planned.extend(validate_shard(shard, index, f"plan shard {index}"))
    if len(planned) != len(set(planned)) or sorted(planned) != discovery:
        reject("plan union does not exactly match discovery")

    durations = {}
    all_deferred = []
    total_duration_millis = 0
    shard_elapsed = []
    for index, shard_dir in enumerate(shard_dirs):
        selected = load_json(os.path.join(shard_dir, "selected-shard.json"))
        validate_shard(selected, index, f"selected shard {index}")
        if selected != shards[index]:
            reject(f"selected shard {index} does not match the canonical plan")
        selected_tests = selected["tests"]
        summary = load_json(os.path.join(shard_dir, "validation-summary.json"))
        summary_keys = ("valid", "expected", "passed", "errors")
        if PACKAGE in ("github.com/glade-sh/glade/internal/apextest", "github.com/glade-sh/glade/internal/lwc/compile", "github.com/glade-sh/glade/internal/visualforce", "github.com/glade-sh/glade/internal/vm", "github.com/glade-sh/glade/internal/gladecli") and "deferred" in summary:
            summary_keys += ("deferred",)
        require_exact_keys(summary, summary_keys, f"shard {index} validation summary")
        deferred = summary.get("deferred", [])
        if (not isinstance(deferred, list) or any(not isinstance(name, str) for name in deferred)
                or deferred != sorted(set(deferred))):
            reject(f"shard {index} has an invalid deferred test list")
        if deferred and PACKAGE not in ("github.com/glade-sh/glade/internal/apextest", "github.com/glade-sh/glade/internal/vm"):
            reject(f"shard {index} has deferred tests outside the Apex/VM policy")
        if PACKAGE in ("github.com/glade-sh/glade/internal/lwc/compile", "github.com/glade-sh/glade/internal/visualforce", "github.com/glade-sh/glade/internal/vm", "github.com/glade-sh/glade/internal/gladecli"):
            # Recheck package terminals and nested skips/failures from raw
            # events before accepting the package coverage/history union.
            with tempfile.TemporaryDirectory(prefix=f"glade-{PREFIX}-history-") as directory:
                metadata = os.path.join(directory, "metadata.tsv")
                checked = os.path.join(directory, "package-summary.json")
                with open(metadata, "w", encoding="utf-8") as target:
                    target.write(PACKAGE + "\thas-tests\n")
                lane = "gladecli-ci" if PACKAGE == "github.com/glade-sh/glade/internal/gladecli" else "remaining-go"
                result = subprocess.run(["bash", SCRIPT, "package-summary", lane, "history",
                    os.path.join(shard_dir, "events.json"), checked, metadata, PACKAGE],
                    capture_output=True, text=True, env={**os.environ, "CI_SHARD_SELECTION_FOR_SUMMARY": os.path.join(shard_dir, "selected-shard.json")})
                if result.returncode != 0:
                    reject(f"shard {index} package validation failed: {result.stderr.strip()}")
        if deferred:
            # Recheck the existing exact prerequisite/reason policy against raw
            # events, rather than trusting a claimed deferral in the summary.
            with tempfile.TemporaryDirectory(prefix=f"glade-{PREFIX}-history-") as directory:
                metadata = os.path.join(directory, "metadata.tsv")
                checked = os.path.join(directory, "package-summary.json")
                with open(metadata, "w", encoding="utf-8") as target:
                    target.write(PACKAGE + "\thas-tests\n")
                lane = "apextest" if PACKAGE == "github.com/glade-sh/glade/internal/apextest" else "remaining-go"
                result = subprocess.run(["bash", "-c",
                    'source "$1"; validate_local_release_package_summary "$2" history "$3" "$4" "$5" "$6"',
                    "history", SCRIPT, lane, os.path.join(shard_dir, "events.json"), checked, metadata, PACKAGE],
                    capture_output=True, text=True, env={**os.environ, "CI_SHARD_SELECTION_FOR_SUMMARY": os.path.join(shard_dir, "selected-shard.json")})
                if result.returncode != 0:
                    reject(f"shard {index} deferred prerequisite validation failed: {result.stderr.strip()}")
                # The shard summary lists deferred selected top-level tests;
                # captured native-boundary subtests stay inside a passing parent.
                checked_deferred = sorted(row["test"] for row in load_json(checked)["deferred"]
                                          if row["test"] in selected_tests)
                if checked_deferred != deferred:
                    reject(f"shard {index} deferred tests do not match raw events")
            all_deferred.extend(deferred)
        if summary["valid"] is not True:
            reject(f"shard {index} validation summary is not valid")
        if (not isinstance(summary["expected"], list) or
                any(not isinstance(name, str) for name in summary["expected"]) or
                not isinstance(summary["passed"], list) or
                any(not isinstance(name, str) for name in summary["passed"]) or
                not isinstance(summary["errors"], list) or
                any(not isinstance(error, str) for error in summary["errors"]) or
                summary["errors"] != [] or
                summary["expected"] != selected_tests or
                summary["passed"] != sorted(summary["passed"]) or
                sorted(summary["passed"] + deferred) != selected_tests):
            reject(f"shard {index} validation summary does not exactly match its selection")

        terminal = {}
        started = {}
        paused = set()
        with open(os.path.join(shard_dir, "events.json"), encoding="utf-8") as source:
            for line_number, line in enumerate(source, 1):
                if not line.strip():
                    continue
                event = json.loads(line, object_pairs_hook=no_duplicate_object)
                if not isinstance(event, dict):
                    reject(f"shard {index} event {line_number} is not an object")
                if event.get("Package") != PACKAGE:
                    reject(f"shard {index} event {line_number} has the wrong package")
                name = event.get("Test")
                action = event.get("Action")
                if isinstance(name, str) and "/" not in name and action == "run":
                    started.setdefault(name, event_seconds(event))
                if isinstance(name, str) and "/" not in name and action == "pause":
                    paused.add(name)
                if not isinstance(name, str) or "/" in name or action not in ("pass", "fail", "skip"):
                    continue
                terminal.setdefault(name, []).append(event)

        if set(terminal) != set(selected_tests):
            reject(f"shard {index} terminal test set does not match its selection")
        elapsed_total = 0.0
        for name in selected_tests:
            events = terminal[name]
            expected_action = "skip" if name in deferred else "pass"
            if len(events) != 1 or events[0].get("Action") != expected_action:
                reject(f"shard {index} test {name} does not have one {expected_action} terminal result")
            if expected_action == "skip":
                if include_deferred:
                    durations[name] = 0
                continue
            elapsed = events[0].get("Elapsed")
            if isinstance(elapsed, bool) or not isinstance(elapsed, (int, float)) or not math.isfinite(elapsed) or elapsed < 0:
                reject(f"shard {index} test {name} has an invalid duration")
            if WALL_WEIGHTS and name not in paused:
                # A parent's Elapsed excludes the time it waits for its
                # parallel subtests; its run-to-terminal interval does not.
                begin, end = started.get(name), event_seconds(events[0])
                if begin is not None and end is not None and math.isfinite(end - begin) and end - begin > elapsed:
                    elapsed = end - begin
            millis = math.floor(elapsed * 1000 + 0.5)
            if millis > MAX_MILLIS:
                reject(f"shard {index} test {name} duration overflows schema v1")
            if millis > MAX_MILLIS - total_duration_millis:
                reject("duration total overflows schema v1 int64")
            total_duration_millis += millis
            durations[name] = millis
            elapsed_total += elapsed
        if not math.isfinite(elapsed_total):
            reject(f"shard {index} elapsed total is not finite")
        shard_elapsed.append(elapsed_total)

    if include_deferred:
        if sorted(durations) != discovery:
            reject("passing and zero-weight deferred terminal union does not exactly match all discovered tests")
    elif len(durations) + len(all_deferred) != len(discovery) or sorted(list(durations) + all_deferred) != discovery:
        reject("passing and deferred terminal union does not exactly match all discovered tests")
    import statistics
    median = statistics.median(shard_elapsed)
    # PR runner timing is advisory after all evidence checks pass; retain
    # observed durations for scheduling without failing a PR on runner noise.
    # The advisory is a ::warning:: annotation. Apex and sema enforce balance
    # at every shard count outside PRs when duration history drove the plan.
    # Train and release PRs (head ref train/ or release/) enforce it too, so
    # an unbalanced plan fails before the push to main.
    # LWC's indivisible long tests can exceed 1.5x its eight-shard median
    # even with an optimal plan;
    # rejecting that evidence would prevent the next run from balancing it.
    # The same holds for a deterministic fallback plan, which splits by test
    # count rather than duration. Its timing is advisory on every event, so
    # the refresh saves durations and the next plan, built from that history,
    # is enforced again.
    if PACKAGE in ("github.com/glade-sh/glade/internal/apextest", "github.com/glade-sh/glade/internal/sema"):
        for index, elapsed in enumerate(shard_elapsed):
            if elapsed > 1.5 * median:
                message = f"shard {index} elapsed {elapsed:.6f}s exceeds 1.5x {SHARD_COUNT}-shard median {median:.6f}s"
                head_ref = os.environ.get("GITHUB_HEAD_REF", "")
                enforced_pr = head_ref.startswith("train/") or head_ref.startswith("release/")
                if os.environ.get("GITHUB_EVENT_NAME") == "pull_request" and not enforced_pr:
                    print(f"::warning::[ci] {LABEL} duration history timing advisory: {message}", file=sys.stderr)
                elif plan["historyUsed"] is not True:
                    print(f"::warning::[ci] {LABEL} duration history timing advisory: {message}; plan used the deterministic fallback", file=sys.stderr)
                else:
                    if profile_path:
                        # The profile is scheduling input only; it is never the accepted history.
                        profile = {
                            "version": 1, "package": PACKAGE,
                            "complete": not all_deferred or include_deferred,
                            "tests": [{"name": name, "durationMillis": durations[name]} for name in sorted(durations)],
                        }
                        profile_dir = os.path.dirname(profile_path) or "."
                        os.makedirs(profile_dir, exist_ok=True)
                        descriptor, temporary = tempfile.mkstemp(prefix=".apextest-scheduling.", dir=profile_dir, text=True)
                        try:
                            with os.fdopen(descriptor, "w", encoding="utf-8") as target:
                                json.dump(profile, target, sort_keys=True, indent=2, allow_nan=False)
                                target.write("\n")
                            os.replace(temporary, profile_path)
                        except BaseException:
                            try:
                                os.unlink(temporary)
                            except FileNotFoundError:
                                pass
                            raise
                    reject(message)

    history = {
        "version": 1,
        "package": PACKAGE,
        "complete": not all_deferred or include_deferred,
        "tests": [{"name": name, "durationMillis": durations[name]} for name in sorted(durations)],
    }
    output_dir = os.path.dirname(output_path) or "."
    os.makedirs(output_dir, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{PREFIX}-duration-history.", dir=output_dir, text=True)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as target:
            json.dump(history, target, sort_keys=True, indent=2, allow_nan=False)
            target.write("\n")
        os.replace(temporary, output_path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise
    if all_deferred and not include_deferred:
        print(f"[ci] {LABEL} duration history incomplete: deferred={all_deferred}; using deterministic scheduling fallback")
    print(f"[ci] {LABEL} duration history refreshed: tests={len(discovery)} shard_elapsed={shard_elapsed} median={median:.6f}s")
except Exception as error:
    print(f"[ci] {LABEL} duration history refresh rejected: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
}

refresh_apextest_history() {
	# Keep the two-shard local train interface; CI supplies output then all dirs.
	if [[ "$#" -eq 3 ]]; then
		refresh_test_history "github.com/glade-sh/glade/internal/apextest" "Apex" "apextest" "0" "2" "$3" "$1" "$2"
	elif [[ "$#" -eq 7 || "$#" -eq 13 || "$#" -eq 17 ]]; then
		local output="$1"
		local shard_count="6"
		if [[ "$#" -eq 13 ]]; then
			shard_count="12"
		elif [[ "$#" -eq 17 ]]; then
			shard_count="16"
		fi
		shift
		refresh_test_history "github.com/glade-sh/glade/internal/apextest" "Apex" "apextest" "0" "${shard_count}" "${output}" "$@"
	else
		echo "usage: scripts/ci-go-test.sh apex-history-refresh {OUTPUT SHARD_0_DIR ... SHARD_N_DIR | SHARD_0_DIR SHARD_1_DIR OUTPUT}" >&2
		return 2
	fi
}

refresh_sema_history() {
	refresh_test_history "github.com/glade-sh/glade/internal/sema" "sema" "sema" "0" "2" "${3:-}" "${1:-}" "${2:-}"
}

refresh_vm_history() {
	# $@ is OUTPUT then one directory per shard (3 on push, 4 on the PR layout).
	refresh_test_history "github.com/glade-sh/glade/internal/vm" "VM" "vm" "0" "$(( $# - 1 ))" "$@"
}

refresh_gladecli_history() {
	# $@ is OUTPUT then one directory per shard (2 on push, 4 on the PR layout).
	refresh_test_history "github.com/glade-sh/glade/internal/gladecli" "gladecli" "gladecli" "0" "$(( $# - 1 ))" "$@"
}

bootstrap_visualforce_history() {
	# A successful unfiltered predecessor supplies timing evidence only. The
	# current workers still discover and validate their complete test union.
	# Adapt its raw stream to the shared collector instead of adding a second
	# package, nested-result or duration validation policy.
	python3 - "${script_dir}/ci-go-test.sh" "$1" "$2" <<'PY'
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

script, output, events = sys.argv[1:]
package = "github.com/glade-sh/glade/internal/visualforce"
try:
    os.unlink(output)
except FileNotFoundError:
    pass
try:
    with tempfile.TemporaryDirectory(prefix="glade-visualforce-bootstrap-") as directory:
        raw_path = os.path.join(directory, "events.json")
        shutil.copyfile(events, raw_path)
        names = set()
        with open(raw_path, encoding="utf-8") as source:
            for line in source:
                if not line.strip():
                    continue
                event = json.loads(line)
                name = event.get("Test")
                if isinstance(name, str) and name and "/" not in name:
                    # Include nonterminal observations so an interrupted test
                    # cannot disappear from the collector's expected set.
                    names.add(name)
        names = sorted(names)
        with open(os.path.join(directory, "discovery.txt"), "w", encoding="utf-8") as target:
            target.write("\n".join(names) + "\n")
        selected = {"index": 0, "tests": names, "estimatedDurationMillis": 0,
                    "regex": "^(?:" + "|".join(re.escape(name) for name in names) + ")$"}
        artifacts = {
            "selected-shard.json": selected,
            "plan.json": {"version": 1, "package": package, "historyUsed": False, "shards": [selected]},
            "validation-summary.json": {"valid": True, "expected": names, "passed": names, "errors": [], "deferred": []},
        }
        for name, value in artifacts.items():
            with open(os.path.join(directory, name), "w", encoding="utf-8") as target:
                json.dump(value, target)
                target.write("\n")
        result = subprocess.run(["bash", "-c",
            'source "$1"; refresh_test_history "$2" Visualforce visualforce 0 1 "$3" "$4"',
            "visualforce-history-bootstrap", script, package, output, directory])
        if result.returncode:
            raise ValueError("serial predecessor failed shared history validation")
except Exception as error:
    print(f"[ci] Visualforce duration history bootstrap rejected: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
}

run_sema_full() {
	local artifact_dir="${CI_SEMA_FULL_ARTIFACT_DIR:-ci-artifacts/sema-full}"
	local package_arg="./internal/sema"
	local discovery_raw="${artifact_dir}/discovery-command.txt"
	local discovery_stderr="${artifact_dir}/discovery-stderr.txt"
	local discovery="${artifact_dir}/discovery.txt"
	local selected="${artifact_dir}/selected-full.json"
	local events="${artifact_dir}/events.json"
	local summary="${artifact_dir}/validation-summary.json"
	local discovery_rc native_rc validation_rc=0
	local -a packages=()
	mkdir -p "${artifact_dir}"
	: >"${discovery_raw}"; : >"${discovery_stderr}"; : >"${discovery}"; : >"${events}"
	printf '{"valid": false, "errors": ["full oracle did not reach result validation"]}\n' >"${summary}"
	if [[ -z "${CI_SHARD_PLANNER:-}" ]]; then
		while IFS= read -r pkg; do packages+=("${pkg}"); done < <(package_lane_packages "sema")
		if [[ "${#packages[@]}" -ne 1 ]]; then
			echo "[ci] sema package lane must contain exactly one package" >&2
			return 1
		fi
		package_arg="${packages[0]}"
	fi
	set +e
	go test -vet=off -trimpath -list '^Test' "${package_arg}" >"${discovery_raw}" 2>"${discovery_stderr}"
	discovery_rc="$?"
	set -e
	[[ ! -s "${discovery_stderr}" ]] || cat "${discovery_stderr}" >&2
	if [[ "${discovery_rc}" -ne 0 ]]; then cat "${discovery_raw}" >&2; return "${discovery_rc}"; fi
	validate_package_discovery "${discovery_raw}" "${discovery}" "github.com/glade-sh/glade/internal/sema" "sema" "1"
	python3 - "${discovery}" "${selected}" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as source:
    tests = source.read().splitlines()
with open(sys.argv[2], "w", encoding="utf-8") as target:
    json.dump({"tests": tests}, target, sort_keys=True, indent=2)
    target.write("\n")
PY
	set +e
	run_json_with_heartbeat "go test sema full oracle" "${events}" -timeout=30m "${package_arg}"
	native_rc="$?"
	set -e
	validate_shard_results "${selected}" "${events}" "${summary}" "github.com/glade-sh/glade/internal/sema" "sema full" || validation_rc="$?"
	if [[ "${native_rc}" -ne 0 || "${validation_rc}" -ne 0 ]]; then render_failure_output "${events}" "sema"; fi
	if [[ "${native_rc}" -ne 0 ]]; then return "${native_rc}"; fi
	return "${validation_rc}"
}

validate_sema_equivalence() {
	local shard_zero="${1:-}" shard_one="${2:-}" full_dir="${3:-}" output="${4:-}"
	if [[ -z "${shard_zero}" || -z "${shard_one}" || -z "${full_dir}" || -z "${output}" ]]; then
		echo "usage: scripts/ci-go-test.sh sema-equivalence SHARD_0_DIR SHARD_1_DIR FULL_DIR OUTPUT" >&2
		return 2
	fi
	python3 - "${shard_zero}" "${shard_one}" "${full_dir}" "${output}" <<'PY'
import json, os, re, sys
PACKAGE = "github.com/glade-sh/glade/internal/sema"
TEST = re.compile(r"Test[A-Za-z0-9_]*\Z")
shards = sys.argv[1:3]
full_dir, output = sys.argv[3:5]
try:
    os.unlink(output)
except FileNotFoundError:
    pass

def reject(message): raise ValueError(message)
def no_dupes(pairs):
    value = {}
    for key, item in pairs:
        if key in value: reject(f"duplicate JSON key {key!r}")
        value[key] = item
    return value
def load(path):
    with open(path, encoding="utf-8") as source: return json.load(source, object_pairs_hook=no_dupes)
def discovery(path):
    with open(path, encoding="utf-8") as source: raw = source.read()
    names = raw.splitlines()
    if not names or names != sorted(names) or len(names) != len(set(names)) or raw != "\n".join(names) + "\n" or any(not TEST.fullmatch(n) for n in names): reject("non-canonical discovery")
    return names
def terminals(path, expected):
    result = {}
    with open(path, encoding="utf-8") as source:
        for line_number, line in enumerate(source, 1):
            if not line.strip(): continue
            event = json.loads(line, object_pairs_hook=no_dupes)
            if event.get("Package") != PACKAGE: reject(f"event {line_number} has wrong package")
            name, action = event.get("Test"), event.get("Action")
            if not isinstance(name, str) or "/" in name or action not in ("pass", "fail", "skip"): continue
            if name in result: reject(f"duplicate terminal result for {name}")
            result[name] = action
    if set(result) != set(expected): reject("terminal result set does not match expected tests")
    return result

try:
    discoveries = [discovery(os.path.join(path, "discovery.txt")) for path in shards]
    full_discovery = discovery(os.path.join(full_dir, "discovery.txt"))
    if discoveries[0] != discoveries[1] or discoveries[0] != full_discovery: reject("discoveries do not match")
    plans = [load(os.path.join(path, "plan.json")) for path in shards]
    if plans[0] != plans[1]: reject("shard plans do not match")
    plan = plans[0]
    if set(plan) != {"version", "package", "historyUsed", "shards"} or plan["version"] != 1 or plan["package"] != PACKAGE or len(plan["shards"]) != 2: reject("invalid plan")
    union, shard_maps = [], []
    for index, path in enumerate(shards):
        selected = load(os.path.join(path, "selected-shard.json"))
        if selected != plan["shards"][index]: reject(f"selected shard {index} differs from plan")
        tests = selected.get("tests")
        if not tests or tests != sorted(tests): reject(f"invalid selected shard {index}")
        union.extend(tests)
        shard_maps.append(terminals(os.path.join(path, "events.json"), tests))
    if len(union) != len(set(union)) or sorted(union) != full_discovery: reject("shard union does not equal discovery")
    shard_map = shard_maps[0] | shard_maps[1]
    full_map = terminals(os.path.join(full_dir, "events.json"), full_discovery)
    if shard_map != full_map: reject("full terminal map differs from exact shard union")
    os.makedirs(os.path.dirname(output) or ".", exist_ok=True)
    with open(output, "w", encoding="utf-8") as target:
        json.dump({"valid": True, "package": PACKAGE, "tests": len(full_map)}, target, sort_keys=True, indent=2)
        target.write("\n")
except Exception as error:
    print(f"[ci] sema equivalence rejected: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
}

main() {
	case "${1:-test}" in
		test)
			run_full_tests
			;;
		core)
			run_core_tests
			;;
		local-release)
			if [[ "$#" -ne 1 ]]; then echo "usage: scripts/ci-go-test.sh local-release" >&2; return 2; fi
			run_local_release
			;;
		local-release-lane)
			if [[ "$#" -ne 2 ]]; then
				echo "usage: scripts/ci-go-test.sh local-release-lane {vet|guard|cli-ui|sema|apex-0|apex-1|server-playground|remaining}" >&2
				return 2
			fi
			run_selected_local_release_lane "$2"
			;;
		race)
			run_race_tests
			;;
		lane)
			if [[ "$#" -ne 2 ]]; then
				echo "usage: scripts/ci-go-test.sh lane {workflow-lint|gladecli|sema|server-and-playground|server-process-builder|repoguard|remaining-go}" >&2
				return 2
			fi
			run_ci_package_lane "$2"
			;;
		node-integration)
			if [[ "$#" -ne 1 ]]; then echo "usage: scripts/ci-go-test.sh node-integration" >&2; return 2; fi
			run_node_integration
			;;
		apex-shard)
			run_apextest_matrix_shard "${2:-}"
			;;
		apex-history-refresh)
			shift
			refresh_apextest_history "$@"
			;;
		sema-shard)
			if [[ "$#" -ne 2 ]]; then echo "usage: scripts/ci-go-test.sh sema-shard 0|1" >&2; return 2; fi
			run_sema_matrix_shard "${2:-}"
			;;
		lwc-compile-shard)
			if [[ "$#" -ne 2 ]]; then echo "usage: scripts/ci-go-test.sh lwc-compile-shard INDEX (0-11)" >&2; return 2; fi
			run_lwc_compile_matrix_shard "${2:-}"
			;;
		lwc-compile-history-refresh)
			if [[ "$#" -ne 10 && "$#" -ne 14 ]]; then echo "usage: scripts/ci-go-test.sh lwc-compile-history-refresh OUTPUT SHARD_DIR... (8 or 12)" >&2; return 2; fi
			shift
			refresh_test_history "github.com/glade-sh/glade/internal/lwc/compile" "LWC compile" "lwc-compile" "0" "$(( $# - 1 ))" "$@"
			;;
		visualforce-shard)
			if [[ "$#" -ne 2 ]]; then echo "usage: scripts/ci-go-test.sh visualforce-shard INDEX (0-7)" >&2; return 2; fi
			run_visualforce_matrix_shard "${2:-}"
			;;
		visualforce-history-refresh)
			if [[ "$#" -ne 6 && "$#" -ne 10 ]]; then echo "usage: scripts/ci-go-test.sh visualforce-history-refresh OUTPUT SHARD_DIR... (4 or 8)" >&2; return 2; fi
			shift
			refresh_test_history "github.com/glade-sh/glade/internal/visualforce" "Visualforce" "visualforce" "0" "$(( $# - 1 ))" "$@"
			;;
		vm-shard)
			if [[ "$#" -ne 2 ]]; then echo "usage: scripts/ci-go-test.sh vm-shard INDEX (0-3; CI_VM_SHARDS 3 limits to 0-2)" >&2; return 2; fi
			run_vm_matrix_shard "${2:-}"
			;;
		vm-history-refresh)
			if [[ "$#" -ne 6 && "$#" -ne 5 ]]; then echo "usage: scripts/ci-go-test.sh vm-history-refresh OUTPUT SHARD_0_DIR ... SHARD_2_DIR (or SHARD_3_DIR)" >&2; return 2; fi
			shift
			refresh_vm_history "$@"
			;;
		gladecli-shard)
			if [[ "$#" -ne 2 ]]; then echo "usage: scripts/ci-go-test.sh gladecli-shard INDEX (0-3; CI_GLADECLI_SHARDS 2 limits to 0-1)" >&2; return 2; fi
			run_gladecli_matrix_shard "${2:-}"
			;;
		gladecli-history-refresh)
			if [[ "$#" -ne 6 && "$#" -ne 4 ]]; then echo "usage: scripts/ci-go-test.sh gladecli-history-refresh OUTPUT SHARD_0_DIR SHARD_1_DIR (or ... SHARD_3_DIR)" >&2; return 2; fi
			shift
			refresh_gladecli_history "$@"
			;;
		visualforce-history-bootstrap)
			if [[ "$#" -ne 3 || -z "$2" || -z "$3" ]]; then echo "usage: scripts/ci-go-test.sh visualforce-history-bootstrap OUTPUT EVENTS" >&2; return 2; fi
			bootstrap_visualforce_history "$2" "$3"
			;;
		package-summary)
			if [[ "$#" -lt 7 ]]; then echo "usage: scripts/ci-go-test.sh package-summary LANE LABEL EVENTS SUMMARY METADATA PACKAGE..." >&2; return 2; fi
			shift
			validate_local_release_package_summary "$@"
			;;
		sema-history-refresh)
			if [[ "$#" -ne 4 ]]; then echo "usage: scripts/ci-go-test.sh sema-history-refresh SHARD_0_DIR SHARD_1_DIR OUTPUT" >&2; return 2; fi
			refresh_sema_history "${2:-}" "${3:-}" "${4:-}"
			;;
		sema-full)
			if [[ "$#" -ne 1 ]]; then echo "usage: scripts/ci-go-test.sh sema-full" >&2; return 2; fi
			run_sema_full
			;;
		sema-equivalence)
			if [[ "$#" -ne 5 ]]; then echo "usage: scripts/ci-go-test.sh sema-equivalence SHARD_0_DIR SHARD_1_DIR FULL_DIR OUTPUT" >&2; return 2; fi
			validate_sema_equivalence "${2:-}" "${3:-}" "${4:-}" "${5:-}"
			;;
		*)
			echo "usage: scripts/ci-go-test.sh [core|test|race|local-release|local-release-lane NAME|lane NAME|node-integration|apex-shard 0-15|apex-history-refresh OUTPUT SHARD_0_DIR ... SHARD_N_DIR (or SHARD_0_DIR SHARD_1_DIR OUTPUT)|sema-shard 0|1|lwc-compile-shard 0-11|lwc-compile-history-refresh OUTPUT SHARD_DIR...|visualforce-shard 0-7|visualforce-history-refresh OUTPUT SHARD_DIR...|vm-shard 0-3|vm-history-refresh OUTPUT SHARD_0_DIR ... SHARD_2_DIR (or SHARD_3_DIR)|gladecli-shard 0-3|gladecli-history-refresh OUTPUT SHARD_0_DIR SHARD_1_DIR (or ... SHARD_3_DIR)|visualforce-history-bootstrap OUTPUT EVENTS|package-summary LANE LABEL EVENTS SUMMARY METADATA PACKAGE...|sema-history-refresh SHARD_0_DIR SHARD_1_DIR OUTPUT|sema-full|sema-equivalence SHARD_0_DIR SHARD_1_DIR FULL_DIR OUTPUT]" >&2
			return 2
			;;
	esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
	main "$@"
fi
