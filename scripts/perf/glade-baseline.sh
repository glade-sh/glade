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

go test -run '^$' -bench=BenchmarkAnalyzeIndex -benchmem -benchtime=1x \
  -cpuprofile "$out_dir/sema.cpu" -memprofile "$out_dir/sema.mem" ./internal/sema \
  | tee "$out_dir/sema.bench"

go test -run '^$' -bench=BenchmarkWorkspaceSymbols -benchmem -benchtime=1x \
  -cpuprofile "$out_dir/lsp.cpu" -memprofile "$out_dir/lsp.mem" ./internal/lsp \
  | tee "$out_dir/lsp.bench"

go tool pprof -top -alloc_space "$out_dir/apextest.mem" >"$out_dir/apextest.alloc.top"
go tool pprof -top -cum "$out_dir/apextest.cpu" >"$out_dir/apextest.cpu.cum"
go tool pprof -top -alloc_space "$out_dir/sema.mem" >"$out_dir/sema.alloc.top"
go tool pprof -top -alloc_space "$out_dir/lsp.mem" >"$out_dir/lsp.alloc.top"

echo "wrote $out_dir"
