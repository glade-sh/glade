#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

out_dir="${1:-/tmp/glade-perf-baseline}"
mkdir -p "$out_dir"

if [[ -n "${GLADE_PPROF_BIN:-}" ]]; then
  pprof_cmd=("${GLADE_PPROF_BIN}")
elif [[ -n "${GLADE_PPROF_ROOT:-}" ]]; then
  pprof_cmd=(go run "${GLADE_PPROF_ROOT}")
elif command -v pprof >/dev/null 2>&1; then
  pprof_cmd=(pprof)
else
  # Go 1.26 no longer ships `go tool pprof`; use the maintained upstream
  # command when a standalone pprof binary is unavailable. Pinning the
  # version avoids a network-only `@latest` index lookup on warm/offline hosts.
  pprof_version="${GLADE_PPROF_VERSION:-v0.0.0-20260802141513-ef3492d7dac3}"
  pprof_cmd=(go run "github.com/google/pprof@${pprof_version}")
fi

bin="$out_dir/glade"
echo "building $bin"
go build -o "$bin" ./cmd/glade

run_time() {
  local name="$1"
  shift
  echo "== $name =="
  # `time -l` invokes restricted sysctl calls on some macOS sandboxes and can
  # fail after the command itself succeeds. POSIX timing keeps this workflow
  # portable; pprof below supplies the allocation data for profiled targets.
  /usr/bin/time -p "$@" >"$out_dir/$name.stdout" 2>"$out_dir/$name.time"
  cat "$out_dir/$name.time"
}

run_time version "$bin" version
run_time doctor "$bin" doctor

GODEBUG=inittrace=1 "$bin" version >"$out_dir/version.init.stdout" 2>"$out_dir/version.inittrace"

# Build outside the checkout before profiling. `go test -cpuprofile` asks the
# toolchain to copy its temporary test binary back into the package directory,
# which is blocked by some macOS sandboxed checkouts and makes profiling fail
# before the benchmark starts.
apextest_bin="$out_dir/apextest.test"
go test -c -o "$apextest_bin" ./internal/apextest
"$apextest_bin" \
  -test.run '^$' \
  -test.bench '^BenchmarkRunTestSuiteWithClassSetup$' \
  -test.benchmem \
  -test.benchtime 1x \
  -test.cpuprofile "$out_dir/apextest.cpu" \
  -test.memprofile "$out_dir/apextest.mem" \
  | tee "$out_dir/apextest.bench"

sema_benchmarks=(
  BenchmarkAnalyzeIndex
  BenchmarkCheckMethodBodies
  BenchmarkBuildTypeMembers
  BenchmarkCheckInheritance
  BenchmarkCheckQuerySemantics
  BenchmarkStandardCatalogHydration
)

: >"$out_dir/sema.cold.bench"
: >"$out_dir/sema.warm.bench"
for benchmark in "${sema_benchmarks[@]}"; do
  for size in 200 2000; do
    leaf="$benchmark/size=$size/mode=cold"
    GLADE_SEMA_BENCH_COLD_LEAF="$leaf" \
      go test -run '^$' -bench="^${benchmark}$/^size=${size}$/^mode=cold$" \
        -benchmem -benchtime=1x -count=1 ./internal/sema \
      | tee -a "$out_dir/sema.cold.bench"

    go test -run '^$' -bench="^${benchmark}$/^size=${size}$/^mode=warm$" \
      -benchmem -benchtime=5x -count=1 ./internal/sema \
      | tee -a "$out_dir/sema.warm.bench"
  done
done

echo "note: sema.process profiles include fixture setup, warm-up, and result oracle work; they are not benchmark timing evidence"
sema_bin="$out_dir/sema.test"
go test -c -o "$sema_bin" ./internal/sema
"$sema_bin" \
  -test.run '^$' \
  -test.bench '^BenchmarkAnalyzeIndex$/^size=200$/^mode=warm$' \
  -test.benchtime 1x \
  -test.count 1 \
  -test.cpuprofile "$out_dir/sema.process.cpu" \
  -test.memprofile "$out_dir/sema.process.mem" \
  | tee "$out_dir/sema.process.bench"
profile_rows="$(grep -c '^BenchmarkAnalyzeIndex/size=200/mode=warm-' "$out_dir/sema.process.bench" || true)"
if [[ "$profile_rows" != "1" ]]; then
  echo "expected exactly one size-200 sema process benchmark row, got $profile_rows" >&2
  exit 1
fi

lsp_bin="$out_dir/lsp.test"
go test -c -o "$lsp_bin" ./internal/lsp
"$lsp_bin" \
  -test.run '^$' \
  -test.bench '^BenchmarkWorkspaceSymbols$' \
  -test.benchmem \
  -test.benchtime 1x \
  -test.cpuprofile "$out_dir/lsp.cpu" \
  -test.memprofile "$out_dir/lsp.mem" \
  | tee "$out_dir/lsp.bench"

pprof_available=true
render_pprof() {
  local output="$1"
  shift
  if [[ "${pprof_available}" != true ]]; then
    return 0
  fi
  if ! GO111MODULE=on "${pprof_cmd[@]}" "$@" >"$out_dir/$output"; then
    pprof_available=false
    echo "pprof report generation unavailable; raw profiles remain under $out_dir" >&2
    printf '%s\n' "pprof command failed: ${pprof_cmd[*]}" >"$out_dir/pprof.unavailable"
  fi
}

render_pprof apextest.alloc.top -top -alloc_space "$out_dir/apextest.mem"
render_pprof apextest.cpu.cum -top -cum "$out_dir/apextest.cpu"
render_pprof sema.process.alloc.top -top -alloc_space "$out_dir/sema.process.mem"
render_pprof sema.process.cpu.cum -top -cum "$out_dir/sema.process.cpu"
render_pprof lsp.alloc.top -top -alloc_space "$out_dir/lsp.mem"

echo "wrote $out_dir"
