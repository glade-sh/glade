# Salesforce Surface Ledger Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` or `superpowers:subagent-driven-development` to implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build one plain ledger that aligns Salesforce docs, an org/API snapshot, Glade's known shape, and Glade's behavior evidence, then reports the actual gaps.

**Architecture:** Four inputs feed one comparable row set: docs snapshot, org/API snapshot, Glade snapshot, and evidence snapshot. The comparator writes a `SurfaceLedger` where every row says what Salesforce exposes, what Glade knows, what Glade can execute, and what proof exists. Probes stay useful, but they are evidence for rows, not the backbone of discovery.

**Tech Stack:** Go 1.26, `internal/capability`, `internal/apexdocs`, `internal/typesys`, `internal/vm`, `internal/server`, `internal/compat`, `internal/oracle`, `testdata/generated/tooling_system_symbols.json.gz`, Salesforce Tooling API completions, REST/Tooling/object describe APIs, docs fixtures.

---

## Why This Is Simpler

Do not start by refactoring the VM. Do not start by making smarter probes.

Start with a ledger. A row. A comparison.

Salesforce gives us three strong boards:

- Public docs: what Salesforce says should exist, and many behavior rules.
- Org/API shape: what a live org exposes through Tooling completions, describes, REST resources, and API version behavior.
- Black-box execution: what a focused owned snippet or test observes.

Glade gives us three more:

- Type shape: `typesys.StandardPlatformSymbols()`, generated stubs, standard objects, schema overlays.
- Behavior claims: `StdlibMatrix`, `stub-behavior`, `capability` rows, server routes, explicit unsupported paths.
- Evidence: compat fixtures, oracle artifacts, local-test corpus outcomes.

Put those in one ledger and sort the gaps. No guessing where the big holes are. No running a thousand probes to learn what a Tooling endpoint already told us.

## Current Evidence

I ran the existing docs chain against the local scrape.

```text
docs inventory: files=3224 members=5177 namespaces=89
catalog entries=8401
reconcile runtime targets=2693
reconcile supported=168 partial=71 typed=2185 unknown=269
doc contracts=197
```

The checked Salesforce coverage manifest already has a Tooling API completions source:

```text
testdata/generated/tooling_system_symbols.json.gz
Tooling API classes=7091
Tooling API members=73326
Runtime APIs found in Tooling API=133/134
Catalog system entries in Tooling API=1985/2693
```

That is a lot of signal. The current reports use it, but not as one joined ledger.

One false join is already visible. `lightning-aura/ref_attr_types_object.md` becomes an Apex-ish `Object` work item because the current inventory classifies too much by title and too little by product folder. Source path must be part of identity.

## The Ledger Row

Create one row type. Keep it boring.

```go
type SurfaceLedgerRow struct {
    SurfaceID string `json:"surfaceId"`
    Product   string `json:"product"`
    Area      string `json:"area"`

    Namespace  string `json:"namespace,omitempty"`
    TypeName   string `json:"typeName,omitempty"`
    MemberName string `json:"memberName,omitempty"`
    Resource   string `json:"resource,omitempty"`
    FieldName  string `json:"fieldName,omitempty"`
    Kind       string `json:"kind"`
    Signature  string `json:"signature,omitempty"`
    ReturnType string `json:"returnType,omitempty"`
    Parameters []string `json:"parameters,omitempty"`

    Docs       SourceState `json:"docs"`
    Org        SourceState `json:"org"`
    GladeShape ShapeState  `json:"gladeShape"`
    GladeBehavior BehaviorState `json:"gladeBehavior"`
    Evidence   EvidenceState `json:"evidence"`

    Owner      string `json:"owner,omitempty"`
    GapClass   string `json:"gapClass,omitempty"`
    Priority   int    `json:"priority,omitempty"`
    Notes      string `json:"notes,omitempty"`
}
```

Use separate state fields:

```text
docs: absent | present | changed | removed | deprecated
org: absent | present | changed | unavailable | not-queried
gladeShape: absent | type-known | signature-known | generated
gladeBehavior: none | passive | unsupported | partial | supported
evidence: none | docs | fixture | oracle | fixture-and-oracle
```

The useful gaps fall out of that:

| Gap class | Meaning |
| --- | --- |
| `missing-shape` | Docs or org exposes it. Glade has no type/member/resource shape. |
| `missing-signature` | Type exists. Member or overload shape does not. |
| `missing-behavior` | Shape exists. No supported, partial, passive, or unsupported behavior decision. |
| `missing-evidence` | Behavior is claimed. No fixture or oracle evidence ties to the surface. |
| `stale-glade-shape` | Glade has a surface absent from docs and org. |
| `docs-org-mismatch` | Docs and live org disagree. Needs release/API-version review. |
| `signature-changed` | Same surface ID. Parameter or return shape changed. |
| `passive-service-risk` | Side-effecting service call has passive/default behavior. |
| `api-version-change` | API availability or retirement text changed. |

## Command Shape

Keep commands direct. Add one high-level command first, then keep the lower-level
commands for debugging.

Daily use:

```bash
glade compat surface refresh \
  --docs "$SALESFORCE_DOCS" \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --out docs/generated/salesforce
```

Live-org refresh:

```bash
glade compat surface refresh \
  --docs "$SALESFORCE_DOCS" \
  --target-org glade-probe-lab \
  --out docs/generated/salesforce
```

Release update:

```bash
glade compat surface refresh \
  --docs "$SALESFORCE_DOCS" \
  --target-org glade-probe-lab \
  --release v66.0 \
  --diff-from docs/generated/salesforce/releases/v65.0/SURFACE_LEDGER.json \
  --out docs/generated/salesforce/releases/v66.0
```

CI check:

```bash
glade compat surface check \
  --ledger docs/generated/salesforce/SURFACE_LEDGER.json \
  --max-missing-shape 269 \
  --max-missing-behavior 0 \
  --max-parser-failures 0
```

The high-level command writes these files:

```text
DOCS_SNAPSHOT.json
ORG_SNAPSHOT.json
GLADE_SNAPSHOT.json
EVIDENCE_SNAPSHOT.json
SURFACE_LEDGER.json
SURFACE_DASHBOARD.md
SURFACE_GAPS.md
SURFACE_FAILURES.md
SURFACE_RELEASE_DIFF.md
```

Terminal output stays small:

```text
surface refresh: ok
inputs: docs=3224 org=tooling_system_symbols.json.gz glade=standard-symbols evidence=fixtures
implemented=168 partial=71 passive=2185 explicitUnsupported=0
gaps: missingShape=269 missingSignature=0 missingBehavior=0 missingEvidence=0
failures: parser=0 docsOrgMismatch=0 staleGlade=0 passiveServiceRisk=0
reports: docs/generated/salesforce/SURFACE_DASHBOARD.md
```

Use lower-level commands when the one command needs inspection:

```bash
glade compat surface docs \
  --source "$SALESFORCE_DOCS" \
  --output docs/generated/salesforce/DOCS_SNAPSHOT.json

glade compat surface org \
  --target-org glade-probe-lab \
  --output docs/generated/salesforce/ORG_SNAPSHOT.json

glade compat surface glade \
  --output docs/generated/salesforce/GLADE_SNAPSHOT.json

glade compat surface evidence \
  docs/fixtures/*.json \
  --oracle docs/fixtures/oracle/fixture-corpus.json \
  --output docs/generated/salesforce/EVIDENCE_SNAPSHOT.json

glade compat surface ledger \
  --docs docs/generated/salesforce/DOCS_SNAPSHOT.json \
  --org docs/generated/salesforce/ORG_SNAPSHOT.json \
  --glade docs/generated/salesforce/GLADE_SNAPSHOT.json \
  --evidence docs/generated/salesforce/EVIDENCE_SNAPSHOT.json \
  --output docs/generated/salesforce/SURFACE_LEDGER.json

glade compat surface gaps \
  --ledger docs/generated/salesforce/SURFACE_LEDGER.json \
  --output docs/generated/salesforce/SURFACE_GAPS.md

glade compat surface explain \
  --ledger docs/generated/salesforce/SURFACE_LEDGER.json \
  --id System.Label.get
```

The live org command can be optional in CI. If no org is configured, use the checked `testdata/generated/tooling_system_symbols.json.gz` snapshot and mark `org` as `not-queried` for sources that need live REST calls.

## Result Buckets

The report must use names a maintainer can act on.

| Bucket | Meaning | Counts as success? |
| --- | --- | --- |
| `implemented` | Glade has shape, modeled behavior, and fixture or oracle evidence. | Yes. |
| `partial` | Glade has useful modeled behavior with documented gaps and evidence. | Yes, but stays visible. |
| `passive` | Glade returns deterministic DTO/default behavior. | Maybe. Safe for DTOs. Suspicious for service calls. |
| `explicitUnsupported` | Glade rejects with a stable unsupported diagnostic and evidence. | Yes, when outside the support claim. |
| `gap` | Missing shape, signature, behavior, or evidence. | No. Work item. |
| `failure` | Parser failure, docs/org mismatch, stale Glade surface, passive service risk, or release regression. | No. Fix or classify. |

`SURFACE_DASHBOARD.md` should lead with these buckets, then show top gaps by owner.
`SURFACE_GAPS.md` should be a work queue. `SURFACE_FAILURES.md` should be short and
red. A failure report that needs scrolling is already losing.

## Implementation Plan

### Task 1: Add The Ledger Package

**Files:**
- Create: `internal/surfaceledger/model.go`
- Create: `internal/surfaceledger/ids.go`
- Create: `internal/surfaceledger/merge.go`
- Create: `internal/surfaceledger/report.go`
- Create: `internal/surfaceledger/model_test.go`
- Create: `internal/surfaceledger/merge_test.go`

- [ ] Define `SurfaceLedgerRow`, source states, shape states, behavior states, and evidence states.
- [ ] Define canonical IDs:
  - Apex type: `apex:System.Label`
  - Apex member: `apex:System.Label.get(String,String)`
  - Tooling object: `tooling:ApexClass`
  - Tooling field: `tooling:ApexClass.Body`
  - REST resource: `rest:composite.post`
  - Visualforce attr: `visualforce:apex:page.showHeader`
  - LWC module: `lwc:@salesforce/apex`
- [ ] Add merge logic that joins rows by `SurfaceID`.
- [ ] Add gap classification logic using only row states.
- [ ] Test product collision: Aura `Object` must never join Apex `System.Object`.
- [ ] Run: `go test ./internal/surfaceledger`.

### Task 1.5: Add The One-Command Runner

**Files:**
- Create: `internal/gladecli/compat_surface_command.go`
- Create: `internal/gladecli/compat_surface_command_test.go`
- Create: `internal/surfaceledger/refresh.go`
- Create: `internal/surfaceledger/refresh_test.go`

- [ ] Add `glade compat surface refresh`.
- [ ] Make `--docs` required unless `GLADE_SALESFORCE_DOCS_SOURCE` is set.
- [ ] Make org input one of `--target-org`, `--tooling-completions`, or the default checked `testdata/generated/tooling_system_symbols.json.gz`.
- [ ] Make `--out` default to `docs/generated/salesforce`.
- [ ] Write all snapshot, ledger, dashboard, gaps, failures, and release-diff files in one run.
- [ ] Print only the compact summary shown above.
- [ ] Add `--json` for machines.
- [ ] Add `--dry-run` to write to a temp dir and print its path.
- [ ] Run: `go test ./internal/gladecli ./internal/surfaceledger`.

### Task 2: Build The Docs Snapshot

**Files:**
- Create: `internal/surfaceledger/docs_snapshot.go`
- Create: `internal/surfaceledger/docs_snapshot_test.go`
- Modify: `internal/apexdocs/inventory.go`
- Modify: `internal/capability/catalog.go`

- [ ] Reuse current Apex docs parsing where it works.
- [ ] Add product-aware classification by source folder before title or namespace.
- [ ] Extract Apex class/member rows, ConnectApi DTO rows, REST resource rows, Tooling object/field rows, Visualforce component attribute rows, Aura JS API rows, and LWC module rows.
- [ ] Preserve docs source paths and titles on every row.
- [ ] Extract available API version text when the prose says `Available in API version`.
- [ ] Keep current `docs-inventory` output stable until callers are moved.
- [ ] Run: `go test ./internal/surfaceledger ./internal/apexdocs ./internal/capability`.

Exit check:

```bash
go run ./cmd/glade compat surface docs \
  --source "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper (1)/salesforce-docs" \
  --json
```

### Task 3: Build The Org/API Snapshot

**Files:**
- Create: `internal/surfaceledger/org_snapshot.go`
- Create: `internal/surfaceledger/org_snapshot_test.go`
- Modify: `internal/capability/tooling_completions.go`
- Modify: `internal/gladecli/compat_command.go`

- [ ] Convert `ToolingCompletions` into ledger rows.
- [ ] Use `testdata/generated/tooling_system_symbols.json.gz` as the default offline org shape.
- [ ] Add optional live org capture through existing `sf` plumbing:
  - Tooling completions: `/services/data/<version>/tooling/completions/?type=apex`
  - SObject global describe and per-object describe.
  - REST resource list when available.
  - Tooling object rows from docs plus live queryable object checks where cheap.
- [ ] Mark rows from offline checked data as `org=present`.
- [ ] Mark rows not covered by the offline data as `org=not-queried`, not absent.
- [ ] Run: `go test ./internal/surfaceledger ./internal/capability ./internal/gladecli`.

Exit check:

```bash
go run ./cmd/glade compat surface org \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --json
```

### Task 4: Build The Glade Snapshot

**Files:**
- Create: `internal/surfaceledger/glade_snapshot.go`
- Create: `internal/surfaceledger/glade_snapshot_test.go`
- Modify: `internal/capability/stdlib.go`
- Modify: `internal/capability/stub_behavior.go`
- Modify: `internal/server/describe_payloads.go` only if a route inventory helper is needed.

- [ ] Convert `typesys.StandardPlatformSymbols()` to `gladeShape` rows.
- [ ] Convert `StdlibMatrix()` to `gladeBehavior` rows.
- [ ] Convert `BuildStubBehaviorReport()` to behavior rows for generated stubs.
- [ ] Add server route rows for REST and Tooling routes that Glade already owns.
- [ ] Keep inferred runtime behavior conservative. If the current code has no explicit claim, write `gladeBehavior=none`.
- [ ] Run: `go test ./internal/surfaceledger ./internal/capability ./internal/server`.

Exit check:

```bash
go run ./cmd/glade compat surface glade --json
```

### Task 5: Build The Evidence Snapshot

**Files:**
- Create: `internal/surfaceledger/evidence_snapshot.go`
- Create: `internal/surfaceledger/evidence_snapshot_test.go`
- Modify: `internal/compat/fixture.go`
- Modify: `internal/compat/evidence.go`
- Modify: `docs/fixtures/*.json` only when adding explicit `surfaceId` fields.

- [ ] Read existing fixture evidence fields.
- [ ] Add optional `surfaceId` to fixture evidence records.
- [ ] Infer old symbol evidence into surface IDs where exact and unambiguous.
- [ ] Read oracle fixture corpus IDs where present.
- [ ] Do not count broad local-test pass as evidence for every touched surface. Use it as corpus signal only.
- [ ] Run: `go test ./internal/surfaceledger ./internal/compat`.

Exit check:

```bash
go run ./cmd/glade compat surface evidence docs/fixtures/*.json --json
```

### Task 6: Add The Comparator And Reports

**Files:**
- Modify: `internal/surfaceledger/merge.go`
- Modify: `internal/surfaceledger/report.go`
- Create: `internal/surfaceledger/priority.go`
- Create: `internal/surfaceledger/priority_test.go`
- Create: `internal/surfaceledger/dashboard.go`
- Create: `internal/surfaceledger/dashboard_test.go`
- Modify: `internal/gladecli/compat_command.go`

- [ ] Join docs, org, Glade, and evidence rows.
- [ ] Classify gap classes.
- [ ] Priority order:
  1. `missing-shape` for runtime Apex core/data/test surfaces.
  2. `signature-changed` for runtime surfaces.
  3. `missing-behavior` for shape-known runtime surfaces.
  4. `passive-service-risk`.
  5. `missing-evidence` for supported/partial behavior.
  6. Product namespace typed gaps.
  7. REST/Tooling server gaps.
  8. Docs-only UI guide rows.
- [ ] Write JSON and Markdown.
- [ ] Write `SURFACE_DASHBOARD.md` with bucket counts first, then owner counts, then top 25 gaps.
- [ ] Write `SURFACE_FAILURES.md` with parser failures, docs/org mismatches, stale Glade rows, passive service risks, and check regressions.
- [ ] Write `SURFACE_RELEASE_DIFF.md` when `--diff-from` is supplied.
- [ ] Add `surface explain` for one row with source facts, gap class, owner, and next command.
- [ ] Add `surface check` with ratchets for each bucket.
- [ ] Run: `go test ./internal/surfaceledger ./internal/gladecli`.

Exit check:

```bash
go run ./cmd/glade compat surface ledger \
  --docs /tmp/docs-snapshot.json \
  --org /tmp/org-snapshot.json \
  --glade /tmp/glade-snapshot.json \
  --evidence /tmp/evidence-snapshot.json \
  --output /tmp/surface-ledger.json

go run ./cmd/glade compat surface gaps \
  --ledger /tmp/surface-ledger.json \
  --output /tmp/surface-gaps.md

go run ./cmd/glade compat surface check \
  --ledger /tmp/surface-ledger.json \
  --max-missing-shape 269 \
  --max-parser-failures 0
```

### Task 7: Use Probes Only Where The Ledger Asks

**Files:**
- Modify: `internal/oracle/orchestrator.go`
- Modify: `internal/oracle/probe_body.go`
- Modify: `internal/gladecli/compat_oracle_command.go`

- [ ] Generate probe inventory from ledger rows, not raw reconcile worklist.
- [ ] Keep Type.forName probes for `missing-shape`.
- [ ] Add behavior probes only for rows with `missing-behavior` and safe input domains.
- [ ] Never probe rows that Tooling or describe already answers unless behavior is ambiguous.
- [ ] Store probe output back as evidence rows keyed by `SurfaceID`.
- [ ] Run: `go test ./internal/oracle ./internal/surfaceledger ./internal/gladecli`.

Exit check:

```bash
go run ./cmd/glade compat oracle inventory \
  --ledger /tmp/surface-ledger.json \
  --gap-class missing-behavior \
  --limit 25 \
  --json
```

## First Slice

Build the ledger with no live org requirement:

1. Docs snapshot from the local Salesforce docs folder.
2. Org snapshot from `testdata/generated/tooling_system_symbols.json.gz`.
3. Glade snapshot from `typesys.StandardPlatformSymbols()`, `StdlibMatrix()`, and `stub-behavior`.
4. Evidence snapshot from `docs/fixtures/*.json`.
5. Ledger and gap report.

This first slice should produce a much better answer to one question:

```text
What does Salesforce expose that Glade does not know, cannot execute, or cannot prove?
```

Validation:

```bash
go test ./internal/surfaceledger ./internal/capability ./internal/apexdocs ./internal/gladecli

src="/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper (1)/salesforce-docs"
tmp="$(mktemp -d)"
go run ./cmd/glade compat surface refresh \
  --docs "$src" \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"

test -s "$tmp/SURFACE_DASHBOARD.md"
test -s "$tmp/SURFACE_LEDGER.json"
test -s "$tmp/SURFACE_GAPS.md"
```

No full example-project local-test run is needed for this slice.

## What This Replaces

This does not throw away current probes. It puts them in their proper place.

Current probes answer:

```text
Can this thing resolve or behave like a tiny owned case?
```

The ledger answers:

```text
What are all the things, where did each fact come from, what does Glade know, what does Glade do, and what proof exists?
```

That is the missing piece.

## Done Criteria

- The ledger can be regenerated from local docs plus checked Tooling data without a live org.
- A live org can refresh the org/API snapshot when available.
- Every gap row has a clear class, source, owner, and next command.
- One command writes the dashboard, gap list, failure list, release diff, and machine-readable ledger.
- The terminal summary shows implemented, partial, passive, explicit unsupported, gaps, and failures without filling the screen.
- A release update is one command with `--release` and `--diff-from`.
- Product docs no longer collide by title.
- `unknown` no longer hides type-known surfaces.
- Supported and partial behavior without evidence is visible.
- Passive behavior on likely service calls is visible.
- Probes are generated only for ledger rows that still need black-box behavior.

Works like a tight door.
