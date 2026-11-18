---
name: perf-measure
description: Use for a glade performance change or a suspected performance regression - matched before/after outcomes, load-insensitive metrics, and what to put in the pull request.
---

# Measure performance

A performance change keeps outcomes identical. Prove that first, then measure.

## 1. Identical outcomes

Run the same inputs on `origin/main` and on the branch. Compare results, not
timings: pass or fail, outputs, diagnostics. Any difference is a behavior
change and needs its own evidence (`capture-conformance`).

## 2. Targeted benchmark

Add or reuse a Go benchmark that isolates the path. Report `-benchmem`
allocations. An allocation budget in a test is an assertion: do not loosen it
to make a change pass.

## 3. Load-insensitive metrics

Wall time moves with host load. Prefer, in this order:

- instructions retired (`time -l` on macOS, `perf stat` on Linux);
- peak RSS;
- allocation counts and bytes.

Alternate the branch and the baseline in balanced order (A B B A). Repeat the
runs. Check that a quiet baseline reproduces itself before comparing. Profiles
taken on macOS skew; confirm a hot spot with a counter before acting on it.

## 4. Pull request

State the workload, the metric, the baseline and branch numbers, the run count
and the scope: which inputs were measured and which were not. Class:
`performance`. Notes in the release carry only measured numbers. A small, reliable
gain with identical outcomes is acceptable.
