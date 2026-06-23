# All Salesforce Surface Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close every Salesforce docs row that Glade does not currently cover, either as implemented local behavior or as a deliberate shape-only platform surface with an explicit local runtime fence.

**Architecture:** Treat the Surface Ledger as the contract. No hand-picked gap list is sufficient. `glade-tools` owns docs extraction, ledger comparison, packet generation, and corpus classification; `glade` owns product shape, semantic acceptance, runtime/passive behavior, and explicit unsupported fences. A row is closed only when it leaves `gap` and `failure`; local-impossible Salesforce behavior closes as `explicitUnsupported`, `passive`, or `stubNoOp`, not as an open row.

**Tech Stack:** Go, Node.js generator scripts, local Salesforce docs at `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run`, paired `glade` and `glade-tools` worktrees, Surface Ledger JSON, generated Apex platform symbols, generated data reference shape, VM runtime fixtures, public and private corpus checks.

---

## Current Baseline

This baseline was generated against `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage` using a temporary Go workspace so `glade-tools` resolved `github.com/glade-sh/glade` to the worktree, not `/Users/matt/Dev/glade` `main`.

```bash
tmp=$(mktemp -d /tmp/glade-tools-work.XXXXXX)
cd "$tmp"
go work init /Users/matt/Dev/glade-tools /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go work edit -replace github.com/glade-sh/apex-parser=/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/third_party/glade-apex-parser
GOWORK="$tmp/go.work" go run /Users/matt/Dev/glade-tools/cmd/glade-tools surface refresh \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/tooling_system_symbols.json.gz \
  --out /tmp/glade-surface-refresh-codex-salesforce-sema-coverage-worktree
```

Expected baseline before this plan starts:

```text
implemented=125923
passive=43819
stubNoOp=245
explicitUnsupported=957
gap=22162
failure=4174
missingShape=14106
missingEvidence=8056
returnTypeMismatch=4033
parameterMismatch=141
total=197280
```

Source-family backlog from the current ledger:

| source family | open rows |
| --- | ---: |
| connect-rest-api | 7673 |
| apex | 6722 |
| no docs source | 6095 |
| soap-api | 991 |
| metadata-api | 903 |
| ui-api | 856 |
| site-references | 817 |
| apex-guide | 554 |
| rest-api | 374 |
| tooling-api | 352 |
| platform-events | 230 |
| bulk-api | 161 |
| lightning | 151 |
| cli-reference | 105 |
| streaming-api | 92 |
| service-connector-api-reference | 91 |
| soql-sosl | 73 |
| analytics-cli-reference | 68 |
| commerce-cli-reference | 17 |
| limits-reference | 10 |
| REFERENCE_COVERAGE.md | 1 |

Apex and Apex Guide backlog:

```text
open apex/apex-guide rows: 7276
failure: 3918
gap: 3358
property rows: 4000
method rows: 2585
type rows: 691
```

Top Apex clusters:

| cluster | open rows |
| --- | ---: |
| ConnectApi | 3951 |
| ChatterFeeds | 203 |
| String | 110 |
| DescribeSObjectResult | 70 |
| ContentHub | 64 |
| Database | 56 |
| Action | 56 |
| DescribeFieldResult | 52 |
| Limits | 51 |
| CdpQuery | 50 |
| Datetime | 42 |
| Site | 42 |
| CommerceBuyerExperience | 41 |
| System | 38 |
| Test | 34 |
| XmlStreamReader | 32 |
| ChatterMessages | 30 |
| ChatterUsers | 28 |
| Topics | 28 |
| JSONGenerator | 28 |
| Matcher | 27 |
| Math | 27 |
| SingleEmailMessage | 26 |
| XmlNode | 25 |
| Partition | 23 |
| LeadConvert | 23 |
| UserInfo | 23 |
| ManagedContent | 22 |
| Recommendations | 22 |
| StandardSetController | 21 |
| LineItemInput | 21 |
| EnhancedPaymentDataInput | 17 |

## Closure Rules

Every row in `SURFACE_LEDGER.json` must end in one of these buckets:

- `implemented`: shape, behavior, and evidence agree.
- `passive`: the docs row is a DTO, value object, enum-like row, declarative service shell, or read-only shape where local behavior is passive typed storage/defaulting.
- `stubNoOp`: the docs row is a void/no-op local shell and a test proves it returns without cloud side effects.
- `explicitUnsupported`: the docs row describes pure cloud platform behavior that Glade must not mimic locally; sema accepts the shape and runtime returns a stable UnsupportedFeature diagnostic.

These states are not closed:

- `gap`
- `failure`
- `missing-shape`
- `missing-behavior`
- `missing-evidence`
- `return-type-mismatch`
- `parameter-mismatch`
- `stale-glade-shape`
- `docs-org-mismatch`
- `passive-service-risk`
- `parser`

The final gate is:

```bash
glade-tools surface check --ledger /tmp/glade-surface-final/SURFACE_LEDGER.json --strict
```

Expected final output:

```text
surface check: ok
open rows: 0
```

## File Map

`glade-tools` owns the measurement tools:

- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/model.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/report.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/packets.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/glade_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/evidence_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_surface_command.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_surface_command_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/apexdocs/inventory.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/apexdocs/inventory_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/doc_contracts.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/doc_contracts_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go`

`glade` owns product behavior and generated product shape:

- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/scripts/generate-system-stub-symbols.mjs`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/apex_docs_contracts.json`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/system_stub_symbols_generated.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/symbols.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_calls.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_ir.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/type_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_checks.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/platform_passive_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/generated_platform_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/dispatch_static.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/stdlib.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/*_test.go` files matching touched runtime surfaces.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/storage/standard_fields.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/storage/model_test.go`

## Worktree Layout

Use sibling worktrees so `glade-tools/go.mod` can point at the selected `glade` checkout without touching the user's ambient checkout.

```text
/Users/matt/Dev/.worktrees/all-surface-closure/integration/glade
/Users/matt/Dev/.worktrees/all-surface-closure/integration/glade-tools
/Users/matt/Dev/.worktrees/all-surface-closure/ledger-gates/glade
/Users/matt/Dev/.worktrees/all-surface-closure/ledger-gates/glade-tools
/Users/matt/Dev/.worktrees/all-surface-closure/docs-contracts/glade
/Users/matt/Dev/.worktrees/all-surface-closure/docs-contracts/glade-tools
/Users/matt/Dev/.worktrees/all-surface-closure/apex-core/glade
/Users/matt/Dev/.worktrees/all-surface-closure/apex-core/glade-tools
/Users/matt/Dev/.worktrees/all-surface-closure/apex-dto/glade
/Users/matt/Dev/.worktrees/all-surface-closure/apex-dto/glade-tools
/Users/matt/Dev/.worktrees/all-surface-closure/server-shape/glade
/Users/matt/Dev/.worktrees/all-surface-closure/server-shape/glade-tools
/Users/matt/Dev/.worktrees/all-surface-closure/ui-shape/glade
/Users/matt/Dev/.worktrees/all-surface-closure/ui-shape/glade-tools
/Users/matt/Dev/.worktrees/all-surface-closure/corpus-proof/glade
/Users/matt/Dev/.worktrees/all-surface-closure/corpus-proof/glade-tools
```

Before any `glade-tools` command in a squad, create a local `go.work`:

```bash
tmp=$(mktemp -d /tmp/glade-tools-work.XXXXXX)
cd "$tmp"
go work init "$SQUAD_TOOLS" "$SQUAD_GLADE"
go work edit -replace github.com/glade-sh/apex-parser="$SQUAD_GLADE/third_party/glade-apex-parser"
GOWORK="$tmp/go.work" go list -m -json github.com/glade-sh/glade
```

Expected proof:

```text
"Dir": "$SQUAD_GLADE"
```

## Task 1: Add The Strict Zero-Open Surface Gate

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/report.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_surface_command.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_surface_command_test.go`

- [ ] **Step 1: Add a failing strict check test**

Add this test to `compat_surface_command_test.go`:

```go
func TestCompatSurfaceCheckStrictRejectsEveryOpenRow(t *testing.T) {
	root := t.TempDir()
	ledger := filepath.Join(root, "ledger.json")
	data := `{
  "schemaVersion": 1,
  "rows": [
    {"surfaceId":"apex:System.Missing","product":"apex","area":"runtime","kind":"method","docs":"present","gladeShape":"absent","gladeBehavior":"none","evidence":"none","bucket":"gap","gapClass":"missing-shape"},
    {"surfaceId":"apex:System.BadReturn","product":"apex","area":"runtime","kind":"method","docs":"present","gladeShape":"signature-known","gladeBehavior":"supported","evidence":"none","bucket":"failure","gapClass":"return-type-mismatch"}
  ],
  "summary": {
    "implemented": 0,
    "partial": 0,
    "passive": 0,
    "stubNoOp": 0,
    "explicitUnsupported": 0,
    "gaps": {"missing-shape": 1},
    "failures": {"return-type-mismatch": 1},
    "total": 2
  }
}`
	if err := os.WriteFile(ledger, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "check", "--ledger", ledger, "--strict"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("strict check passed with open rows stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "open surface rows=2") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
```

- [ ] **Step 2: Run red**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli -run TestCompatSurfaceCheckStrictRejectsEveryOpenRow -count=1
```

Expected:

```text
FAIL
unknown flag "--strict"
```

- [ ] **Step 3: Add strict check support**

Add this field in `surfaceledger.CheckOptions`:

```go
Strict bool
```

Add this helper in `surfaceledger/report.go`:

```go
func OpenRows(rows []SurfaceLedgerRow) []SurfaceLedgerRow {
	var out []SurfaceLedgerRow
	for _, row := range rows {
		if row.Bucket == BucketGap || row.Bucket == BucketFailure {
			out = append(out, row)
		}
	}
	return out
}
```

Add this block at the start of `CheckLedger`:

```go
if options.Strict {
	open := OpenRows(ledger.Rows)
	if len(open) > 0 {
		first := open[0]
		return fmt.Errorf("open surface rows=%d first=%s gap=%s", len(open), first.SurfaceID, first.GapClass)
	}
}
```

Add this flag in `runCompatSurfaceCheck`:

```go
case "--strict":
	options.Strict = true
```

- [ ] **Step 4: Run green**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger ./internal/toolcli -run 'TestCompatSurfaceCheckStrictRejectsEveryOpenRow|TestCompatSurfaceCheckAcceptsTypeMismatchRatchets' -count=1
```

Expected:

```text
ok
```

- [ ] **Step 5: Commit**

```bash
git add internal/surfaceledger/report.go internal/toolcli/compat_surface_command.go internal/toolcli/compat_surface_command_test.go
git commit -m "test: add strict surface closure gate"
```

## Task 2: Add Packet Manifests That Cover Every Open Row

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/packets.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/packets_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/report.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_surface_command.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_surface_command_test.go`

- [ ] **Step 1: Add a test that every open row receives one packet**

Add this test to `packets_test.go`:

```go
func TestPacketManifestAssignsEveryOpenRow(t *testing.T) {
	ledger := SurfaceLedger{Rows: []SurfaceLedgerRow{
		{SurfaceID: "apex:ConnectApi.Feed.body", Bucket: BucketFailure, Product: ProductApex, Owner: "runtime", DocsSource: "apex/apex_connectapi_output_Feed.md", TypeName: "ConnectApi.Feed"},
		{SurfaceID: "connect-rest:connect_responses_feed.body", Bucket: BucketGap, Product: ProductUnknown, Owner: "runtime", DocsSource: "connect-rest-api/connect_responses_feed.md", TypeName: "Feed"},
		{SurfaceID: "soap:DescribeSObjectResult.fields", Bucket: BucketGap, Product: ProductUnknown, Owner: "data-runtime", DocsSource: "soap-api/sforce_api_calls_describesobjects_describesobjectresult.md", TypeName: "DescribeSObjectResult"},
	}}
	manifest := BuildPacketManifest(ledger)
	if len(manifest.Packets) != 3 {
		t.Fatalf("packet count = %d, want 3: %#v", len(manifest.Packets), manifest.Packets)
	}
	if len(manifest.Unassigned) != 0 {
		t.Fatalf("unassigned rows = %#v", manifest.Unassigned)
	}
	if manifest.TotalOpenRows != 3 {
		t.Fatalf("total open rows = %d, want 3", manifest.TotalOpenRows)
	}
}
```

- [ ] **Step 2: Run red**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger -run TestPacketManifestAssignsEveryOpenRow -count=1
```

Expected:

```text
FAIL
undefined: BuildPacketManifest
```

- [ ] **Step 3: Implement packet manifest types**

Add to `packets.go`:

```go
type PacketManifest struct {
	TotalOpenRows int             `json:"totalOpenRows"`
	Packets       []SurfacePacket `json:"packets"`
	Unassigned    []string        `json:"unassigned"`
}

type SurfacePacket struct {
	ID        string   `json:"id"`
	Owner     string   `json:"owner"`
	Family    string   `json:"family"`
	SourceDir string   `json:"sourceDir"`
	Rows      []string `json:"rows"`
}

func BuildPacketManifest(ledger SurfaceLedger) PacketManifest {
	grouped := map[string]*SurfacePacket{}
	manifest := PacketManifest{}
	for _, row := range ledger.Rows {
		if row.Bucket != BucketGap && row.Bucket != BucketFailure {
			continue
		}
		manifest.TotalOpenRows++
		packetID := packetIDForRow(row)
		if packetID == "" {
			manifest.Unassigned = append(manifest.Unassigned, row.SurfaceID)
			continue
		}
		packet := grouped[packetID]
		if packet == nil {
			packet = &SurfacePacket{
				ID:        packetID,
				Owner:     firstNonEmpty(row.Owner, "runtime"),
				Family:    firstNonEmpty(row.SalesforceSurfaceFamily, row.Product),
				SourceDir: surfaceSourceDir(row.DocsSource),
			}
			grouped[packetID] = packet
		}
		packet.Rows = append(packet.Rows, row.SurfaceID)
	}
	for _, packet := range grouped {
		sort.Strings(packet.Rows)
		manifest.Packets = append(manifest.Packets, *packet)
	}
	sort.Slice(manifest.Packets, func(i, j int) bool { return manifest.Packets[i].ID < manifest.Packets[j].ID })
	sort.Strings(manifest.Unassigned)
	return manifest
}
```

Add helper functions:

```go
func packetIDForRow(row SurfaceLedgerRow) string {
	sourceDir := surfaceSourceDir(row.DocsSource)
	if sourceDir == "" {
		sourceDir = "no-docs-source"
	}
	ns := row.Namespace
	if ns == "" && strings.Contains(row.TypeName, ".") {
		ns = strings.SplitN(row.TypeName, ".", 2)[0]
	}
	if ns == "" {
		ns = row.Product
	}
	return strings.ToLower(sourceDir + "/" + firstNonEmpty(ns, "unknown") + "/" + firstNonEmpty(row.Kind, "row"))
}

func surfaceSourceDir(path string) string {
	if idx := strings.IndexByte(path, '/'); idx > 0 {
		return path[:idx]
	}
	return path
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
```

- [ ] **Step 4: Add packet manifest CLI output**

Extend `runCompatSurfacePacket` with `--manifest <path>` or add `runCompatSurfacePackets`. The command must write JSON with every open row assigned:

```bash
glade-tools surface packet --ledger SURFACE_LEDGER.json --manifest /tmp/surface-packets.json
```

Expected:

```text
surface packet manifest: ok
open rows: 26336
packets: <nonzero>
unassigned: 0
```

- [ ] **Step 5: Commit**

```bash
git add internal/surfaceledger/packets.go internal/surfaceledger/packets_test.go internal/toolcli/compat_surface_command.go internal/toolcli/compat_surface_command_test.go
git commit -m "feat: assign every open surface row to packets"
```

## Task 3: Expand Docs Extraction Across Every Source Family

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/apexdocs/inventory.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/apexdocs/inventory_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/doc_contracts.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/doc_contracts_test.go`

- [ ] **Step 1: Add source-family completeness tests**

Add a table test that expects every current docs family to produce rows or an explicit skip reason:

```go
func TestDocsSnapshotCoversEveryRequiredSourceFamily(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		"analytics-cli-reference", "apex", "apex-guide", "bulk-api", "cli-reference",
		"commerce-cli-reference", "connect-rest-api", "limits-reference", "lightning",
		"metadata-api", "platform-events", "rest-api", "service-connector-api-reference",
		"site-references", "soap-api", "soql-sosl", "streaming-api", "tooling-api", "ui-api",
		"visualforce", "lwc",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "sample.md"), []byte("# Sample\n\n| Name | Type |\n| --- | --- |\n| id | String |\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := DocsSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[surfaceSourceDir(row.DocsSource)] = true
	}
	for _, dir := range []string{"apex", "connect-rest-api", "soap-api", "metadata-api", "ui-api", "rest-api", "tooling-api", "bulk-api", "platform-events", "streaming-api", "lightning", "visualforce", "lwc"} {
		if !seen[dir] {
			t.Fatalf("docs snapshot did not produce rows for %s; seen=%v", dir, seen)
		}
	}
}
```

- [ ] **Step 2: Run red**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger ./internal/apexdocs ./internal/capability -run 'DocsSnapshotCoversEveryRequiredSourceFamily|Inventory' -count=1
```

Expected: FAIL for every source family currently parsed as `unknown` or skipped.

- [ ] **Step 3: Implement source family parsers**

Add extraction paths for these source families:

```text
apex: Apex classes, interfaces, enums, methods, properties, constructors, ConnectApi input/output/static method pages.
apex-guide: language rules, SOQL/SOSL rules, collections, batch interfaces, namespace/type resolution, Flow.Interview prose.
connect-rest-api: resource request/response DTO shapes and endpoint resources; close as server shape unless Glade serves the local endpoint.
rest-api: generic REST resources; close as server shape or explicit unsupported.
tooling-api: Tooling resources and test-runner resources; close as server shape or explicit unsupported.
soap-api: describe/result DTOs and call shapes; close as data/server shape or explicit unsupported.
metadata-api: deploy/retrieve/result DTOs; close as package/server shape or explicit unsupported.
ui-api: object-info, record, layout, picklist, and response DTOs; close as server/data shape or explicit unsupported.
bulk-api: job/batch/result resources; close as server shape or explicit unsupported.
platform-events: event bus resources and event shape; close as Apex/data shape where local runtime supports it, otherwise explicit unsupported.
streaming-api: channel/event resources; close as server shape or explicit unsupported.
service-connector-api-reference: client API objects; close as explicit unsupported unless Glade has a local service connector runtime.
lightning, visualforce, lwc: module/tag/resource shape; close as UI shape, local preview behavior, or explicit unsupported.
site-references, analytics-cli-reference, cli-reference, commerce-cli-reference, limits-reference: classify into product family or explicit non-Glade platform references.
```

- [ ] **Step 4: Verify source-family rows**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger ./internal/apexdocs ./internal/capability -count=1
```

Expected:

```text
ok
```

- [ ] **Step 5: Commit**

```bash
git add internal/apexdocs internal/surfaceledger internal/capability
git commit -m "feat: extract all Salesforce docs source families"
```

## Task 4: Generate Product Shape From Docs Contracts

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/apex_docs_contracts.json`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/scripts/generate-system-stub-symbols.mjs`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/system_stub_symbols_generated.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols_test.go`

- [ ] **Step 1: Replace the one-row contract file with generated contracts**

Generate `apex_docs_contracts.json` from the docs scrape. It must include all Apex docs rows, not only `ConnectApi.ManagedContentVersionCollection`.

```bash
cd /Users/matt/Dev/glade-tools
glade_tools_work=/tmp/glade-tools-work-for-contracts
mkdir -p "$glade_tools_work"
go run ./cmd/glade-tools surface docs \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --output /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/apex_docs_contracts.json
```

Expected:

```text
surface docs: ok
contracts: greater than 1
```

- [ ] **Step 2: Add a guard test that rejects tiny contract files**

Add to `internal/typesys/standard_symbols_test.go`:

```go
func TestApexDocsContractsCoverBroadDocsSurface(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "generated", "apex_docs_contracts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Documents []struct {
			SourcePath string `json:"sourcePath"`
			Members    []struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"members"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Documents) < 500 {
		t.Fatalf("docs contract file has %d documents; want broad Apex docs coverage", len(payload.Documents))
	}
	required := []string{
		"apex/apex_methods_system_string.md",
		"apex/apex_methods_system_database.md",
		"apex/apex_methods_system_sobject_describe.md",
		"apex/apex_ConnectAPI_ChatterFeeds_static_methods.md",
		"apex/apex_class_commercepayments_LineItemInput.md",
	}
	seen := map[string]bool{}
	for _, doc := range payload.Documents {
		seen[doc.SourcePath] = true
	}
	for _, path := range required {
		if !seen[path] {
			t.Fatalf("missing docs contract path %s", path)
		}
	}
}
```

- [ ] **Step 3: Run red or green based on contract generation**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go test ./internal/typesys -run TestApexDocsContractsCoverBroadDocsSurface -count=1
```

Expected before generation:

```text
FAIL
docs contract file has 1 documents
```

- [ ] **Step 4: Regenerate standard symbols**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
node scripts/generate-system-stub-symbols.mjs \
  example-projects/stubs/apex-system-stubs \
  internal/typesys/system_stub_symbols_generated.go \
  testdata/generated/apex_docs_contracts.json
gofmt -w internal/typesys/system_stub_symbols_generated.go
```

Expected:

```text
generated symbols include typed properties and signatures from docs contracts
```

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/typesys -count=1
git diff --check
git add testdata/generated/apex_docs_contracts.json scripts/generate-system-stub-symbols.mjs internal/typesys internal/typesys/system_stub_symbols_generated.go
git commit -m "feat: generate Apex platform shape from docs contracts"
```

## Task 5: Close Apex Core Runtime And Evidence Rows

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/dispatch_static.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/platform_passive_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/generated_platform_runtime.go`
- Modify: matching `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/*_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/evidence_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/evidence_snapshot_test.go`

- [ ] **Step 1: Generate Apex core packets**

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools surface packet \
  --ledger /tmp/glade-surface-refresh-codex-salesforce-sema-coverage-worktree/SURFACE_LEDGER.json \
  --owner runtime \
  --source apex \
  --manifest /tmp/apex-core-packets.json
```

Expected:

```text
unassigned: 0
```

- [ ] **Step 2: Close supported-but-no-evidence rows with fixtures**

For every row where `bucket=gap`, `gladeBehavior=supported`, and `evidence=none`, add a fixture or reclassify to explicit unsupported. Start with these clusters because they have many open rows and users hit them in real Apex:

```text
Approval, Database, Schema, ApexPages, System, String, Datetime, Date, Decimal, Math, Limits, UserInfo, Test, Matcher, Pattern, JSON, JSONGenerator, JSONParser, XmlNode, XmlStreamReader, XmlStreamWriter, Messaging.SingleEmailMessage, Site, Cache.Org, Cache.Session, Cache.Partition.
```

Fixture rule:

```text
If a local deterministic behavior exists, add a VM or sema test and evidence row.
If the behavior depends on Salesforce cloud state, leave sema shape in place and add an UnsupportedFeature runtime test.
```

- [ ] **Step 3: Add a test template for each supported behavior packet**

Use this shape for VM-supported rows:

```go
func TestSurfaceEvidenceDatabaseResultAccessors(t *testing.T) {
	program := `
Account a = new Account(Name = 'Acme');
Database.SaveResult sr = Database.insert(a, false);
System.assert(sr.isSuccess());
System.assertNotEquals(null, sr.getId());
System.assertEquals(0, sr.getErrors().size());
`
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
```

Use this shape for pure Salesforce cloud behavior:

```go
func TestSurfaceUnsupportedChatterFeedsCloudCall(t *testing.T) {
	program := `
ConnectApi.ChatterFeeds.getFeedElementsFromFeed(null, ConnectApi.FeedType.News, 'me');
`
	_, err := Execute(program, nil)
	if err == nil || !strings.Contains(err.Error(), `unsupported call "ConnectApi.ChatterFeeds.getFeedElementsFromFeed"`) {
		t.Fatalf("error = %v, want explicit unsupported", err)
	}
}
```

- [ ] **Step 4: Register evidence**

Each new fixture must produce rows in `EvidenceSnapshot`. Use exact surface IDs from `surface explain`.

```bash
go run ./cmd/glade-tools surface explain \
  --ledger /tmp/glade-surface-refresh-codex-salesforce-sema-coverage-worktree/SURFACE_LEDGER.json \
  --id 'apex:Database.SaveResult.getId()'
```

Expected:

```text
surface id appears with evidence=fixture after refresh
```

- [ ] **Step 5: Verify Apex core packets**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go test ./internal/typesys ./internal/sema ./internal/vm -count=1
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger -count=1
```

- [ ] **Step 6: Commit**

```bash
git add internal/typesys internal/sema internal/vm
git commit -m "feat: close Apex core surface rows"
cd /Users/matt/Dev/glade-tools
git add internal/surfaceledger
git commit -m "test: record Apex core surface evidence"
```

## Task 6: Close Apex DTO And Static Service Shape

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/apex_docs_contracts.json`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/system_stub_symbols_generated.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/platform_passive_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/generated_platform_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/glade_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/glade_snapshot_test.go`

- [ ] **Step 1: Close all ConnectApi DTO property type mismatches**

All `ConnectApi` input/output pages must become generated shape. Do not hand-add one DTO at a time.

Required docs source groups:

```text
apex/apex_connectapi_input_*.md
apex/apex_connectapi_output_*.md
apex/apex_ConnectAPI_*_static_methods.md
connect-rest-api/connect_responses_*.md
```

Expected result after regeneration:

```text
ConnectApi return-type-mismatch: 0
ConnectApi parameter-mismatch: 0
ConnectApi missing-shape: 0 except rows explicitly classified as non-Apex REST server shape
```

- [ ] **Step 2: Close Commerce and commercepayments shapes**

The current known open rows include:

```text
commercepayments.AuthorizationRequest.paymentMethodData
commercepayments.BaseRequest.BaseRequest(String,Map<String,String>)
commercepayments.EnhancedPaymentDataInput
commercepayments.LineItemInput
commercepayments.PaymentGatewayContext.PaymentGatewayContext(commercepayments.PaymentGatewayRequest,String)
commercepayments.PaymentGatewayNotificationRequest.requestBody
commercepayments.PaymentsHttp.send(HttpRequest)
commercepayments.PostAuthApiPaymentMethodRequest.PostAuthApiPaymentMethodRequest(commercepayments.AlternativePaymentMethodRequestPaymentMethodRequest)
commercepayments.PostAuthorizationResponse.setAlternativePaymentMethodResponse(commercepayments.AlternativePaymentMethodResponse)
commercepayments.SaleRequest.enhancedPaymentData
commercepayments.SaleRequest.paymentInitiationSourceId
CommerceBuyGrp.BuyerGroupRequest.guest_uuid_essential_{siteId}
CommerceBuyGrp.BuyerGroupRequest.isGuestUser
CommerceBuyGrp.BuyerGroupRequest.locale
Auth.AuthConfiguration.getLoginRightFrameUrl
```

Add docs-contract rows and sema tests that instantiate the classes, read every property, and call every constructor.

- [ ] **Step 3: Shape-only static service rule**

For static service classes such as `ConnectApi.ChatterFeeds`, `ConnectApi.ContentHub`, `ConnectApi.CdpQuery`, `ConnectApi.CommerceBuyerExperience`, `ConnectApi.ManagedContent`, and `ConnectApi.Recommendations`:

```text
Sema must accept the documented method signature.
Runtime must either return a typed passive value when local deterministic data exists, or throw UnsupportedFeature with the exact method name.
Surface Ledger must classify the row as explicitUnsupported or passive, not gap or failure.
```

- [ ] **Step 4: Verify DTO and static service closure**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go test ./internal/typesys ./internal/sema ./internal/vm -count=1
cd /Users/matt/Dev/glade-tools
GOWORK="$tmp/go.work" go run ./cmd/glade-tools surface refresh \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/tooling_system_symbols.json.gz \
  --out /tmp/glade-surface-after-apex-dto
GOWORK="$tmp/go.work" go run ./cmd/glade-tools surface check --ledger /tmp/glade-surface-after-apex-dto/SURFACE_LEDGER.json \
  --max-return-type-mismatch 0 \
  --max-parameter-mismatch 0
```

Expected:

```text
surface check: ok
```

- [ ] **Step 5: Commit**

```bash
git add testdata/generated/apex_docs_contracts.json internal/typesys internal/sema internal/vm
git commit -m "feat: close generated Apex DTO surface"
```

## Task 7: Close Server API Shape-Only Rows

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/model.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge_test.go`

- [ ] **Step 1: Add policy tests for pure server surfaces**

Add a merge test with one representative row from each source family:

```go
func TestPureServerDocsRowsCloseAsExplicitUnsupportedWhenNoLocalServerExists(t *testing.T) {
	rows := []SurfaceLedgerRow{
		{SurfaceID: "connect-rest:connect_responses_feed.body", Product: ProductUnknown, Area: AreaServer, Kind: KindResource, Docs: SourcePresent, DocsSource: "connect-rest-api/connect_responses_feed.md"},
		{SurfaceID: "soap:DescribeSObjectResult.fields", Product: ProductUnknown, Area: AreaServer, Kind: KindProperty, Docs: SourcePresent, DocsSource: "soap-api/sforce_api_calls_describesobjects_describesobjectresult.md"},
		{SurfaceID: "metadata:DeployResult.details", Product: ProductUnknown, Area: AreaServer, Kind: KindProperty, Docs: SourcePresent, DocsSource: "metadata-api/meta_deployresult.md"},
		{SurfaceID: "ui-api:ObjectInfo.fields", Product: ProductUnknown, Area: AreaServer, Kind: KindProperty, Docs: SourcePresent, DocsSource: "ui-api/ui_api_responses_object_info.md"},
		{SurfaceID: "bulk-api:JobInfo.state", Product: ProductUnknown, Area: AreaServer, Kind: KindProperty, Docs: SourcePresent, DocsSource: "bulk-api/asynch_api_reference_jobinfo.md"},
		{SurfaceID: "tooling:Run.className", Product: ProductTooling, Area: AreaServer, Kind: KindProperty, Docs: SourcePresent, DocsSource: "tooling-api/intro_rest_resources_testing_runner_sync.md"},
	}
	for _, row := range rows {
		classified := classifyServerShapeOnly(row)
		if classified.Bucket != BucketExplicitUnsupported {
			t.Fatalf("%s bucket = %s, want explicitUnsupported", row.SurfaceID, classified.Bucket)
		}
	}
}
```

- [ ] **Step 2: Implement the server shape-only classifier**

In `merge.go`, classify rows from these source directories as explicit unsupported unless a Glade local server evidence row exists:

```text
connect-rest-api
rest-api
tooling-api
soap-api
metadata-api
ui-api
bulk-api
streaming-api
service-connector-api-reference
analytics-cli-reference
cli-reference
commerce-cli-reference
site-references
```

The classifier must keep rows open when a local Glade server route claims support but lacks evidence.

- [ ] **Step 3: Verify server closure**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger -run 'PureServerDocsRows|Merge|DocsSnapshot' -count=1
```

Expected:

```text
ok
```

- [ ] **Step 4: Commit**

```bash
git add internal/surfaceledger
git commit -m "feat: close pure Salesforce server API rows as explicit unsupported"
```

## Task 8: Close Data Reference Rows

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/glade_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/storage/standard_fields.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/storage/model_test.go`

- [ ] **Step 1: Split data reference from server DTOs**

Rows from SOAP, UI API, and object/field references that describe standard object fields must map to `data-reference`. Rows that describe API response fields stay server shape-only.

Test cases:

```text
Account.Name -> data-reference generated field shape
User.ProfileId -> data-reference generated field shape
DescribeSObjectResult.fields -> server DTO shape-only
ObjectInfo.fields -> server DTO shape-only
```

- [ ] **Step 2: Regenerate standard fields if data-reference rows are missing**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go test ./internal/storage -run 'Standard|Field|Model' -count=1
```

Expected:

```text
ok
```

- [ ] **Step 3: Verify data-runtime owner rows are zero open**

```bash
python3 - <<'PY'
import json
rows=json.load(open('/tmp/glade-surface-final/SURFACE_LEDGER.json'))['rows']
open_rows=[r for r in rows if r.get('owner')=='data-runtime' and r.get('bucket') in ('gap','failure')]
if open_rows:
    raise SystemExit(f'data-runtime open rows={len(open_rows)} first={open_rows[0]["surfaceId"]}')
print('data-runtime open rows=0')
PY
```

- [ ] **Step 4: Commit**

```bash
git add internal/storage
git commit -m "feat: close Salesforce data reference rows"
cd /Users/matt/Dev/glade-tools
git add internal/surfaceledger
git commit -m "test: classify data reference rows separately"
```

## Task 9: Close UI, LWC, Aura, Visualforce, And Lightning Rows

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/compat/lwc_corpus_scan.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/compat/lwc_corpus_scan_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/page_render.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/vm/*visualforce*_test.go`

- [ ] **Step 1: Classify UI docs rows**

Use this rule:

```text
LWC modules/components documented as local preview-supported must have shape and fixture evidence.
Visualforce tags/classes supported by local rendering must have behavior fixtures.
Aura/Lightning framework APIs not modeled locally must be explicitUnsupported shape-only.
Site-reference rows that describe external hosted services must be explicitUnsupported.
```

- [ ] **Step 2: Add UI closure tests**

Add one row each for:

```text
lwc module shape supported
visualforce local tag supported
visualforce PageReference.getContent explicit unsupported when rendering is not available
Aura framework API explicit unsupported
Lightning runtime API explicit unsupported
```

- [ ] **Step 3: Verify UI owner rows**

```bash
python3 - <<'PY'
import json
rows=json.load(open('/tmp/glade-surface-final/SURFACE_LEDGER.json'))['rows']
open_rows=[r for r in rows if r.get('owner')=='ui' and r.get('bucket') in ('gap','failure')]
if open_rows:
    raise SystemExit(f'ui open rows={len(open_rows)} first={open_rows[0]["surfaceId"]}')
print('ui open rows=0')
PY
```

- [ ] **Step 4: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/surfaceledger internal/compat
git commit -m "feat: close Salesforce UI docs rows"
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
git add internal/vm
git commit -m "test: add UI surface runtime evidence"
```

## Task 10: Remove No-Docs-Source Open Rows

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/glade_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/report.go`

- [ ] **Step 1: Add a no-source fail test**

Rows with `DocsSource == ""` and `bucket in (gap, failure)` hide coverage loss. Add this check:

```go
func TestStrictCheckRejectsOpenRowsWithoutDocsSource(t *testing.T) {
	ledger := SurfaceLedger{Rows: []SurfaceLedgerRow{{
		SurfaceID: "unknown:blank-source",
		Product: ProductUnknown,
		Area: AreaRuntime,
		Kind: KindType,
		Bucket: BucketGap,
		GapClass: GapMissingShape,
	}}}
	err := CheckLedger(ledger, CheckOptions{Strict: true})
	if err == nil || !strings.Contains(err.Error(), "open surface rows=1") {
		t.Fatalf("err = %v, want strict open row failure", err)
	}
}
```

- [ ] **Step 2: Fix source attribution**

Every row must get one of:

```text
docsSource: real markdown path from docs scrape
docsSource: generated-from-org:<source>
docsSource: generated-from-glade:<source>
docsSource: explicit-local-policy:<policy>
```

Rows must not remain open with a blank source.

- [ ] **Step 3: Verify**

```bash
python3 - <<'PY'
import json
rows=json.load(open('/tmp/glade-surface-final/SURFACE_LEDGER.json'))['rows']
open_blank=[r for r in rows if r.get('bucket') in ('gap','failure') and not r.get('docsSource')]
if open_blank:
    raise SystemExit(f'open rows without docsSource={len(open_blank)} first={open_blank[0]["surfaceId"]}')
print('open rows without docsSource=0')
PY
```

- [ ] **Step 4: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/surfaceledger
git commit -m "test: reject open rows without source attribution"
```

## Task 11: Corpus Closure Must Not Hide Surface Rows

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_command.go`

- [ ] **Step 1: Add corpus allowed-finding gate**

The corpus may end with only:

```text
performance-advisory
project-metadata-missing with named missing metadata evidence
```

It may not end with:

```text
semantic-contract-gap
source-parse-error
project-discovery-duplicate
docs-contract-mismatch
generated-shape-gap
unclassified
```

- [ ] **Step 2: Add tests for disallowed classes**

Use this table:

```go
func TestCorpusClosureDisallowsSurfaceGaps(t *testing.T) {
	report := Report{Counts: map[string]int{
		"performance-advisory":       2,
		"project-metadata-missing":   3,
		"semantic-contract-gap":      1,
		"source-parse-error":         1,
		"project-discovery-duplicate": 1,
		"docs-contract-mismatch":     1,
		"generated-shape-gap":        1,
		"unclassified":              1,
	}}
	got := DisallowedForCheckClosure(report)
	want := map[string]int{
		"semantic-contract-gap":      1,
		"source-parse-error":         1,
		"project-discovery-duplicate": 1,
		"docs-contract-mismatch":     1,
		"generated-shape-gap":        1,
		"unclassified":              1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DisallowedForCheckClosure() = %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 3: Verify corpus closure**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go build -o /tmp/glade-all-surface-closure ./cmd/glade
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/public \
  --glade /tmp/glade-all-surface-closure \
  --out /tmp/glade-public-corpus-all-surface-closure
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/private \
  --glade /tmp/glade-all-surface-closure \
  --out /tmp/glade-private-corpus-all-surface-closure
```

Expected:

```text
semantic-contract-gap=0
source-parse-error=0
project-discovery-duplicate=0
docs-contract-mismatch=0
generated-shape-gap=0
unclassified=0
```

- [ ] **Step 4: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/corpuscheck internal/toolcli
git commit -m "test: forbid corpus closure surface gaps"
```

## Task 12: Final All-Surface Verification

**Files:**
- Modify only files changed by earlier tasks.

- [ ] **Step 1: Rebuild Glade**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go build -o /tmp/glade-all-surface-final ./cmd/glade
```

Expected:

```text
exit 0
```

- [ ] **Step 2: Regenerate the final Surface Ledger from the worktree**

```bash
tmp=$(mktemp -d /tmp/glade-tools-work.XXXXXX)
cd "$tmp"
go work init /Users/matt/Dev/glade-tools /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go work edit -replace github.com/glade-sh/apex-parser=/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/third_party/glade-apex-parser
GOWORK="$tmp/go.work" go run /Users/matt/Dev/glade-tools/cmd/glade-tools surface refresh \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/tooling_system_symbols.json.gz \
  --out /tmp/glade-surface-final
```

Expected:

```text
surface refresh: ok
gaps: missingShape=0 missingBehavior=0 missingEvidence=0
failures: parser=0 docsOrgMismatch=0 staleGlade=0 returnTypeMismatch=0 parameterMismatch=0 passiveServiceRisk=0
```

- [ ] **Step 3: Run strict closure**

```bash
GOWORK="$tmp/go.work" go run /Users/matt/Dev/glade-tools/cmd/glade-tools surface check \
  --ledger /tmp/glade-surface-final/SURFACE_LEDGER.json \
  --strict
```

Expected:

```text
surface check: ok
open rows: 0
```

- [ ] **Step 4: Run product tests**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go test ./internal/typesys ./internal/sema ./internal/vm ./internal/storage ./internal/project -count=1
go test ./...
scripts/smoke.sh
```

Expected:

```text
ok
smoke passes
```

- [ ] **Step 5: Run tools tests**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/apexdocs ./internal/surfaceledger ./internal/capability ./internal/corpuscheck ./internal/toolcli -count=1
```

Expected:

```text
ok
```

- [ ] **Step 6: Run corpus proof**

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/public \
  --glade /tmp/glade-all-surface-final \
  --out /tmp/glade-public-corpus-final
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/private \
  --glade /tmp/glade-all-surface-final \
  --out /tmp/glade-private-corpus-final
```

Expected:

```text
semantic-contract-gap=0
source-parse-error=0
project-discovery-duplicate=0
docs-contract-mismatch=0
generated-shape-gap=0
unclassified=0
```

- [ ] **Step 7: Commit final proof artifacts or handoff note**

Do not check in `/tmp` reports. If a checked artifact is required, write a concise generated summary under `glade-tools/docs/generated/` and keep large ledgers out of `glade`.

```bash
git diff --check
git status --short
```

Expected:

```text
no whitespace errors
only intended files changed
```

## Self-Review Checklist

- [ ] Every source family in the current ledger appears in a task above.
- [ ] Every current open class has a closure path: missing shape, missing evidence, return mismatch, parameter mismatch, blank source, corpus class.
- [ ] Pure Salesforce cloud behavior is not implemented as fake local behavior; it is shaped and fenced with `UnsupportedFeature`.
- [ ] `glade` does not gain docs scrapers, corpus runners, or maintenance dashboards.
- [ ] `glade-tools` does not hide product gaps by classifier wording.
- [ ] The strict surface check fails until open rows are zero.
- [ ] The final corpus gate cannot pass with semantic contract gaps.
- [ ] The final answer includes the exact report path and current counts.
