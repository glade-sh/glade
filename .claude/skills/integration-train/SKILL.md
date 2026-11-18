---
name: integration-train
description: Use when combining approved glade branches into one train branch and pull request, or when a component branch needs to know what CI it must run.
---

# Integration train

A train combines several approved branches into one branch and one pull
request. The component diffs were already reviewed. The train review covers
merge resolutions only.

## Component branches

Small, one ticket each. The author runs build, vet and the focused tests, then
pushes. No full suite per component. Do not bump `SemanticABI`, `PlatformABI` or
`testRuntimeCacheABI`; name the bump class in the PR body.

## Build the train

1. Branch from `origin/main`.
2. Merge the component heads in dependency order. Resolve conflicts once, and
   write each non-trivial resolution into the PR body.
3. Do not merge `main` into a component branch.
4. Open one pull request. The fast lane runs on the union.
5. Dispatch the full run on the train branch:

       gh workflow run ci.yml --ref <train branch>

6. Merge only when the dispatched run is green on the exact head and the
   resolution review approves.

## Restart rules

A new commit on the train restarts the dispatched run and the review. A new
commit on a component restarts that component's checks and its place in the
train. Do not rewrite history under a running check.

Reuse corpus and real-project results when the product tree is identical to the
one already measured. Details: `docs/WORKFLOW.md` section 3.12.
