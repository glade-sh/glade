# Static record alias instruction measurement

Measured on 2026-10-23 on the local Darwin arm64 laptop, using Go 1.26.10, CGO_ENABLED=1, GOMAXPROCS=2 and nice -n 10. Builds finished before measurement. No Salesforce calls are part of this measurement.

Runtime comparison: base `a recorded revision` versus the implementation at `a recorded revision`. The follow-up adds tests and this evidence only; the runtime files are unchanged. The baseline build uses a Go overlay restoring `internal/vm/value_aliasing.go` and `internal/vm/alias_perf.go` from the base. These are the only product Go files changed by the implementation. Both binaries compile the identical benchmark and fixture from `internal/vm/static_alias_occurrence_test.go` with `-trimpath -p=1 -c`.

The probe uses 200, 800 and 4,000 unique Account records, each held twice in a static list, then replaces Description through the static snapshot path once per unique record. The recorder is disabled. Each process performs one benchmark iteration. Setup, Go package initialization and process startup are included in the retired-instruction count, even though the Go benchmark timer excludes fixture setup. No startup subtraction or allocation adjustment is applied.

For each size, run base then HEAD immediately, repeated three times: B/H, B/H, B/H. The counts below come from `/usr/bin/time -l`, not wall time or VM visit counters.

```sh
/usr/bin/time -l /usr/bin/nice -n 10 "$binary" \
  -test.run='^$' \
  -test.bench="^BenchmarkStaticRecordAliasOccurrence/${records}$" \
  -test.benchtime=1x -test.count=1 -test.cpu=2
```

| Unique records | Base minimum | Base median | HEAD minimum | HEAD median | Median reduction |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 200 | 1,502,377,257 | 1,547,280,270 | 1,424,980,436 | 1,442,581,219 | 6.77% |
| 800 | 2,957,447,311 | 2,972,268,702 | 1,393,411,600 | 1,455,734,580 | 51.02% |
| 4,000 | 39,283,147,859 | 39,580,047,395 | 1,576,428,351 | 1,848,960,016 | 95.33% |

The small 200-record probe shows a modest 6.77% median instruction reduction. Its whole-process cost is dominated by fixed initialization; the HEAD medians at 200 and 800 records are nearly equal, so these measurements do not establish a precise small-size slope. At 800 and 4,000 records the measured reductions are 51.02% and 95.33%. Base whole-process instructions grow 13.32x from 800 to 4,000 records; HEAD grows 1.27x. This supports bounded growth at the large size; three sizes with substantial fixed startup do not independently prove an asymptotic complexity class. The separate visit-count tests pin linear replacement and maintenance work.

Raw retired-instruction counts, in execution order:

| Unique records | Alternation | Tree | Instructions retired |
| ---: | ---: | --- | ---: |
| 200 | 1 | base | 1,580,512,676 |
| 200 | 1 | head | 1,511,602,392 |
| 200 | 2 | base | 1,547,280,270 |
| 200 | 2 | head | 1,442,581,219 |
| 200 | 3 | base | 1,502,377,257 |
| 200 | 3 | head | 1,424,980,436 |
| 800 | 1 | base | 2,979,121,358 |
| 800 | 1 | head | 1,393,411,600 |
| 800 | 2 | base | 2,957,447,311 |
| 800 | 2 | head | 1,455,734,580 |
| 800 | 3 | base | 2,972,268,702 |
| 800 | 3 | head | 1,486,076,588 |
| 4,000 | 1 | base | 39,580,047,395 |
| 4,000 | 1 | head | 1,848,960,016 |
| 4,000 | 2 | base | 39,283,147,859 |
| 4,000 | 2 | head | 1,576,428,351 |
| 4,000 | 3 | base | 39,640,974,274 |
| 4,000 | 3 | head | 1,927,912,105 |

All 18 timed invocations exited 0. Both builds used identical test sources with all follow-up test functions registered, and no source edits occurred between the baseline and HEAD builds. This is a synthetic static snapshot-path measurement, not a claim about every value-mutation caller or real-project speedups. Raw time output is retained locally alongside the two compiled test binaries; binaries and ad-hoc output are not checked in.
