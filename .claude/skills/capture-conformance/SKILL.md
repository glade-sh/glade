---
name: capture-conformance
description: Use when a change alters Salesforce-observable behavior, or a ticket says "capture needed". Covers writing cases.tsv, capturing org.tsv and native-run.md from a scratch org, writing the data-driven conformance test, and citing NATIVE lines. Not for Glade-only behavior (CLI, server, tooling).
---

# Capture and conformance

Salesforce behavior comes from captured rows. Rows come before code.
Glade-only behavior needs a stated local contract and a regression test instead;
stop here and follow docs/WORKFLOW.md section 3.3, first paragraph.

## 1. Find existing rows first

    git grep -n "<type or method>" -- 'internal/*/testdata/**/org.tsv'

A row already in the tree backs a fix when its route and API match your test.
Do not capture again what is already captured.

## 2. Write the case table

Directory: `internal/<pkg>/testdata/<family>/`. `<pkg>` is the package whose
test reads it: `apextest` (under `testdata/conformance/`) for runtime behavior,
`sema` for compile-time diagnostics, `visualforce`, `lwc`, `dml` for theirs.
Name `<family>` for the behavior, never for a ticket or review round.

`cases.tsv`: one row per case with `id`, `api`, `route`, the source, and the
expected result once captured.

For every rule you intend to implement, write three rows:

- one that accepts;
- one that rejects, with the exception type and message expected;
- one at the boundary.

Capture each case at the current supported API (68.0 for Apex, 67.0 for LWC
and Visualforce) and at the floor (62.0 for Apex, 59.0 for LWC and
Visualforce). Add 67.0 for Apex when the change touches it. Add each route that can reach the code:
anonymous Apex, an `@IsTest` class, the test runner, the server, the
Visualforce or LWC host. Apex quirks get their own rows: default access
modifiers, case-insensitive names, each overload.

## 3. Capture

Use only an org the ticket or maintainer authorized. Create a fresh scratch
org, deploy only synthetic metadata you wrote, run every case at every API,
then delete the org and verify the deletion.

No org access: stop and ask (WORKFLOW.md section 5, item 1). Do not write code
from inference.

Two observations of the same input differ: keep both rows and assert only what
both share. Rows conflict: add a probe that isolates the one variable, capture
it at both APIs with repeats, and let the new rows decide. Never edit a
captured row; add a superseding row and cite both.

## 4. Write the outputs

- `org.tsv`: one row per observed outcome. Columns vary by family; copy the
  header of an `org.tsv` in the same package and keep the case ids of
  `cases.tsv`.
- `native-run.md`: API versions and timestamps. The name `ORG-RUN.txt` is
  rejected by `internal/repoguard`.

## 5. Scrub before the first commit

History is public. A later "normalize" commit does not remove a leak.

- Replace every 15 or 18 character record or org id (`001…`, `005…`, `00D…`).
- Remove usernames, org aliases, instance hosts, namespaces you do not own.
- Remove personal absolute paths and temp paths.
- Then: `git diff --cached | grep -nE '00D[A-Za-z0-9]{12}|/(Users|home)/'`
  prints nothing, and `CGO_ENABLED=1 go test ./internal/repoguard` passes.

## 6. Encode the rows

Copy the nearest `*_conformance_test.go` in that package: one data-driven Go
test that reads `cases.tsv` and `org.tsv`. CI runs it without credentials.

Commit fixture, `org.tsv`, `native-run.md` and the test together, without the
fix. Run the test: it must fail. Then commit the fix.

Before the fix commit, write the table of what the fix admits or rejects and
match each line to a row:

    NATIVE <behavior> <path>/org.tsv:<line> <route> <api>

One line per row, in the commit message and the PR. A shape with no row is not
implemented: leave it at current behavior and list it in the PR as unproven.
Do not key a rule on a type name, field name or literal when the rows prove
only specific cases.

## Done when

- Every behavior change has a NATIVE line at a matching route and API.
- Each rule has accept, reject and boundary rows at two APIs.
- The test fails at the test commit and passes at HEAD.
- `go test ./internal/repoguard` passes.
