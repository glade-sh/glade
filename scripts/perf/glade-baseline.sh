#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

out_dir="${1:-/tmp/glade-perf-baseline}"
mkdir -p "$out_dir"

bin="$out_dir/glade"
echo "building $bin"
go build -o "$bin" ./cmd/glade

run_time() {
  local name="$1"
  shift
  echo "== $name =="
  /usr/bin/time -l "$@" >"$out_dir/$name.stdout" 2>"$out_dir/$name.time"
  cat "$out_dir/$name.time"
}

run_time version "$bin" version
run_time doctor "$bin" doctor

GODEBUG=inittrace=1 "$bin" version >"$out_dir/version.init.stdout" 2>"$out_dir/version.inittrace"

go test -run '^$' -bench=BenchmarkRunTestSuiteWithClassSetup -benchmem -benchtime=1x \
  -cpuprofile "$out_dir/apextest.cpu" -memprofile "$out_dir/apextest.mem" ./internal/apextest \
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
go test -run '^$' -bench='^BenchmarkAnalyzeIndex$/^size=200$/^mode=warm$' -benchtime=1x -count=1 \
	-cpuprofile "$out_dir/sema.process.cpu" -memprofile "$out_dir/sema.process.mem" ./internal/sema \
	| tee "$out_dir/sema.process.bench"
profile_rows="$(grep -c '^BenchmarkAnalyzeIndex/size=200/mode=warm-' "$out_dir/sema.process.bench" || true)"
if [[ "$profile_rows" != "1" ]]; then
  echo "expected exactly one size-200 sema process benchmark row, got $profile_rows" >&2
  exit 1
fi

go test -run '^$' -bench=BenchmarkWorkspaceSymbols -benchmem -benchtime=1x \
  -cpuprofile "$out_dir/lsp.cpu" -memprofile "$out_dir/lsp.mem" ./internal/lsp \
  | tee "$out_dir/lsp.bench"

go tool pprof -top -alloc_space "$out_dir/apextest.mem" >"$out_dir/apextest.alloc.top"
go tool pprof -top -cum "$out_dir/apextest.cpu" >"$out_dir/apextest.cpu.cum"
go tool pprof -top -alloc_space "$out_dir/sema.process.mem" >"$out_dir/sema.process.alloc.top"
go tool pprof -top -cum "$out_dir/sema.process.cpu" >"$out_dir/sema.process.cpu.cum"
go tool pprof -top -alloc_space "$out_dir/lsp.mem" >"$out_dir/lsp.alloc.top"

echo "wrote $out_dir"
