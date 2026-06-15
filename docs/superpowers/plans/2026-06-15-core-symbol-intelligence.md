# Core Symbol Intelligence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build editor-neutral Glade project intelligence: stable symbols, precise references, dependency SDK artifacts, offline schema symbols, refactor-safe queries, cache reuse, and CLI/server access.

**Architecture:** Add a new `internal/codeintel` package as the shared symbol and reference engine. Existing packages keep ownership: `project` loads files, `schema` loads metadata, `typesys` extracts declarations, `sema` checks Apex, `soql` parses queries, `packageartifact` stores dependency contracts, `enterprisegraph` reports project impact, and `gladecli` exposes product commands.

**Tech Stack:** Go, existing Apex parser in `internal/apexast`, existing type index in `internal/typesys`, existing schema model in `internal/schema` and `internal/storage`, existing SOQL parser in `internal/soql`, JSON cache files under `.glade/symbols`, focused Go tests.

---

## Execution Model For GPT-5.5 Medium

This plan is too broad for one context. The controller agent should use GPT-5.5 medium subagents in separate worktrees for independent lanes.

Do not dispatch two implementation agents into the same worktree. They will step on each other. Use one integration branch and one worktree per lane:

```bash
cd /Users/matt/Dev/glade
git status --short
git switch -c codex/core-symbol-intelligence
mkdir -p /Users/matt/Dev/glade-worktrees
```

For each parallel lane, create a branch and worktree from the current integration branch:

```bash
git worktree add /Users/matt/Dev/glade-worktrees/codeintel-model -b codex/codeintel-model
git worktree add /Users/matt/Dev/glade-worktrees/codeintel-schema -b codex/codeintel-schema
git worktree add /Users/matt/Dev/glade-worktrees/codeintel-artifact -b codex/codeintel-artifact
```

When a lane finishes, the controller merges or cherry-picks that lane into `codex/core-symbol-intelligence`, runs the listed tests, and removes the lane worktree only after proof:

```bash
cd /Users/matt/Dev/glade
git merge --no-ff codex/codeintel-model
go test ./internal/codeintel ./internal/typesys ./internal/schema
git worktree remove /Users/matt/Dev/glade-worktrees/codeintel-model
```

The controller should run the review cycle after each merge:

1. Spec review subagent: check only whether the lane met this plan.
2. Code quality review subagent: check design, tests, and regressions.
3. Fix review findings before merging the next lane.

## Out Of Scope

- Do not touch `contrib/vscode-glade`.
- Do not add live Salesforce login, deploy, retrieve, or org auth.
- Do not move compatibility scanners back into public Glade.
- Do not change runtime semantics just to satisfy symbol queries.
- Do not silently rename symbols when references are ambiguous.

## Product Boundaries

These changes are core Glade product work. They improve `check`, `inspect`, local test selection, reports, package artifacts, server metadata APIs, and future editor clients.

Use captured describe JSON and local project metadata for org-shaped symbols. Glade already has `glade schema import describe --input <describe.json>`. Extend that path rather than adding live org access.

## Target File Map

Create:

- `internal/codeintel/model.go`: public data structures for symbols, declarations, uses, references, and graph queries.
- `internal/codeintel/id.go`: stable symbol ID constructors and parsing helpers.
- `internal/codeintel/build.go`: orchestration from `typesys.Index`, schema, source files, and optional cached symbols.
- `internal/codeintel/declarations.go`: declaration extraction from `typesys.Index`.
- `internal/codeintel/apex_uses.go`: Apex lexical and typed use collector.
- `internal/codeintel/schema_uses.go`: schema, SOQL, DML, trigger, and custom metadata use collector.
- `internal/codeintel/query.go`: definition, references, affected, and rename-safe query helpers.
- `internal/codeintel/cache.go`: `.glade/symbols` cache read/write and freshness checks.
- `internal/codeintel/patch.go`: workspace edit and patch planning for refactors.
- `internal/codeintel/testutil_test.go`: test project builder helpers.
- `internal/codeintel/*_test.go`: focused tests for each unit.
- `internal/sosl/parser.go`: small parser for `FIND ... RETURNING ...` symbol extraction.
- `internal/sosl/parser_test.go`: SOSL parser tests.
- `internal/gladecli/inspect_intelligence_test.go`: CLI tests for `inspect definition`, `inspect references`, and symbol cache output.
- `internal/gladecli/refactor_command.go`: CLI entry for safe rename once approved in this plan.
- `internal/gladecli/refactor_command_test.go`: CLI tests for dry-run and write behavior.

Modify:

- `internal/typesys/symbols.go`: add optional stable IDs and source offsets to existing symbols without breaking JSON consumers.
- `internal/packageartifact/artifact.go`: add symbol contract data and versioned artifact schema fields.
- `internal/project/project.go`: include symbol cache input files in project freshness where needed.
- `internal/schema/schema.go`: expose imported describe provenance for cached symbol use.
- `internal/gladecli/cli.go`: route new `inspect` subcommands and `refactor rename`.
- `internal/gladecli/enterprise_report_command.go`: use `codeintel` graph for report/refactor proof once parity is reached.
- `internal/enterprisegraph/build.go`: replace text scan edges with `codeintel` references.
- `internal/watch/refgraph.go`: replace type word scan with `codeintel` dependencies.
- `internal/watch/affected.go`: keep conservative fallbacks but use typed graph first.
- `internal/server/source_metadata.go`: optionally expose codeintel query data through local Tooling-shaped metadata rows.
- `internal/cliui/help.go`: update help text for new product commands.
- `docs/ARCHITECTURE.md`, `docs/EDITOR.md`, `docs/LOCAL_TESTING.md`: document core symbol engine without mentioning VS Code as the driver.

## Shared Types

The implementation should begin with these exact exported names. Later tasks depend on them.

```go
package codeintel

import "github.com/glade-sh/glade/internal/diagnostic"

type SymbolKind string

const (
	SymbolApexType       SymbolKind = "apex_type"
	SymbolApexMember     SymbolKind = "apex_member"
	SymbolApexLocal      SymbolKind = "apex_local"
	SymbolTrigger        SymbolKind = "trigger"
	SymbolSObject        SymbolKind = "sobject"
	SymbolSObjectField   SymbolKind = "sobject_field"
	SymbolCustomMetadata SymbolKind = "custom_metadata"
	SymbolLabel          SymbolKind = "label"
	SymbolStaticResource SymbolKind = "static_resource"
	SymbolUnknown        SymbolKind = "unknown"
)

type UseKind string

const (
	UseDeclaration UseKind = "declaration"
	UseRead        UseKind = "read"
	UseWrite       UseKind = "write"
	UseCall        UseKind = "call"
	UseConstruct   UseKind = "construct"
	UseExtends     UseKind = "extends"
	UseImplements  UseKind = "implements"
	UseQuery       UseKind = "query"
	UseMutate      UseKind = "mutate"
	UseMetadata    UseKind = "metadata"
)

type SymbolID string

type Location struct {
	File  string            `json:"file,omitempty"`
	Range diagnostic.Range  `json:"range"`
}

type Symbol struct {
	ID         SymbolID          `json:"id"`
	Kind       SymbolKind        `json:"kind"`
	Name       string            `json:"name"`
	Container  SymbolID          `json:"container,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	Type       string            `json:"type,omitempty"`
	Signature  string            `json:"signature,omitempty"`
	File       string            `json:"file,omitempty"`
	Range      diagnostic.Range  `json:"range"`
	Dependency bool              `json:"dependency,omitempty"`
	Artifact   bool              `json:"artifact,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type Use struct {
	SymbolID SymbolID          `json:"symbolId,omitempty"`
	Kind     UseKind           `json:"kind"`
	Name     string            `json:"name"`
	File     string            `json:"file"`
	Range    diagnostic.Range  `json:"range"`
	Context  SymbolID          `json:"context,omitempty"`
	Resolved bool              `json:"resolved"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Graph struct {
	ProjectRoot string             `json:"projectRoot"`
	Symbols     map[SymbolID]Symbol `json:"symbols"`
	Uses        []Use               `json:"uses"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}
```

## Task 0: Baseline And Guard Rails

**Files:**
- Read: `AGENTS.md`
- Read: `docs/ARCHITECTURE.md`
- Read: `internal/typesys/symbols.go`
- Read: `internal/sema/sema.go`
- Read: `internal/schema/schema.go`
- Read: `internal/packageartifact/artifact.go`
- Read: `internal/enterprisegraph/build.go`
- Read: `internal/watch/refgraph.go`

- [ ] **Step 1: Confirm clean working scope**

Run:

```bash
cd /Users/matt/Dev/glade
git status --short
```

Expected: note any existing user changes. Do not revert them.

- [ ] **Step 2: Run current focused proof**

Run:

```bash
go test ./internal/lsp ./internal/project ./internal/typesys ./internal/schema ./internal/packageartifact ./internal/enterprisegraph ./internal/watch
```

Expected: all packages pass before starting.

- [ ] **Step 3: Create integration branch**

Run:

```bash
git switch -c codex/core-symbol-intelligence
```

Expected: branch created. If branch exists, inspect it before reusing.

- [ ] **Step 4: Record baseline command output**

Run:

```bash
go test ./internal/lsp ./internal/project ./internal/typesys ./internal/schema ./internal/packageartifact ./internal/enterprisegraph ./internal/watch 2>&1 | tee /tmp/glade-codeintel-baseline.txt
```

Expected: `/tmp/glade-codeintel-baseline.txt` contains all package results.

## Task 1: Core Codeintel Model

**Parallelism:** Serial. Every other lane depends on this.

**Files:**
- Create: `internal/codeintel/model.go`
- Create: `internal/codeintel/id.go`
- Create: `internal/codeintel/model_test.go`
- Create: `internal/codeintel/id_test.go`

- [ ] **Step 1: Write failing ID tests**

Create tests with these cases:

```go
func TestStableSymbolIDs(t *testing.T) {
	tests := []struct {
		name string
		got  codeintel.SymbolID
		want codeintel.SymbolID
	}{
		{"type", codeintel.ApexTypeID("pkg", "InvoiceService"), "apex:type:pkg:InvoiceService"},
		{"member", codeintel.ApexMemberID("pkg", "InvoiceService", "method", "total", "Decimal(Account)"), "apex:member:pkg:InvoiceService:method:total:Decimal(Account)"},
		{"object", codeintel.SObjectID("Account"), "schema:object:Account"},
		{"field", codeintel.SObjectFieldID("Account", "Name"), "schema:field:Account:Name"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s ID = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Write graph merge and sort tests**

Test `Graph.AddSymbol`, `Graph.AddUse`, `Graph.SortedSymbols`, and `Graph.References`.

Expected behavior:

- duplicate symbol IDs merge metadata without losing the first file/range.
- `SortedSymbols` sorts by kind, name, then ID.
- `References(id, true)` includes declarations.
- `References(id, false)` excludes declaration uses.

- [ ] **Step 3: Implement `model.go`**

Use the shared types above. Add methods:

```go
func NewGraph(projectRoot string) Graph
func (g *Graph) AddSymbol(symbol Symbol)
func (g *Graph) AddUse(use Use)
func (g Graph) SortedSymbols() []Symbol
func (g Graph) Definition(id SymbolID) (Symbol, bool)
func (g Graph) References(id SymbolID, includeDeclaration bool) []Use
func (g Graph) UsesByFile(file string) []Use
```

- [ ] **Step 4: Implement `id.go`**

Use colon-separated stable IDs. Escape literal `:` in names as `%3A`; unescape only in parsing helpers.

Required functions:

```go
func ApexTypeID(namespace, name string) SymbolID
func ApexMemberID(namespace, typeName, kind, name, signature string) SymbolID
func ApexLocalID(file string, line, column int, name string) SymbolID
func TriggerID(namespace, name string) SymbolID
func SObjectID(name string) SymbolID
func SObjectFieldID(objectName, fieldName string) SymbolID
func CustomMetadataID(objectName, developerName string) SymbolID
func LabelID(name string) SymbolID
func StaticResourceID(name string) SymbolID
func ParseID(id SymbolID) []string
```

- [ ] **Step 5: Run proof**

Run:

```bash
go test ./internal/codeintel
```

Expected: tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/codeintel
git commit -m "feat: add core code intelligence model"
```

## Task 2: Declaration Extraction From Existing Index

**Parallelism:** Serial after Task 1.

**Files:**
- Create: `internal/codeintel/declarations.go`
- Create: `internal/codeintel/declarations_test.go`
- Create: `internal/codeintel/testutil_test.go`
- Modify: `internal/typesys/symbols.go` only if a needed range or parameter field is absent.

- [ ] **Step 1: Write failing declaration tests**

Build a temporary SFDX project with:

- `InvoiceService.cls` with a field, constructor, overloaded methods, and property.
- `InvoiceTrigger.trigger` on `Invoice__c`.
- `objects/Invoice__c/Invoice__c.object-meta.xml`.
- `objects/Invoice__c/fields/Amount__c.field-meta.xml`.
- one custom metadata record `Feature.Default.md-meta.xml`.

Test that `codeintel.BuildDeclarations(index)` emits:

- class symbol `apex:type::InvoiceService`.
- method symbols with different signatures for overloads.
- trigger symbol.
- object symbol `schema:object:Invoice__c`.
- field symbol `schema:field:Invoice__c:Amount__c`.
- custom metadata symbol.
- declaration uses for each symbol.

- [ ] **Step 2: Implement declaration extraction**

Add:

```go
func BuildDeclarations(index typesys.Index) Graph
func SymbolForType(typ typesys.TypeSymbol) Symbol
func SymbolForMember(typ typesys.TypeSymbol, member typesys.MemberSymbol) Symbol
func SymbolForTrigger(trigger typesys.TriggerSymbol) Symbol
func SymbolForObject(object schema.Object) Symbol
func SymbolForField(object schema.Object, field schema.Field) Symbol
```

Use `typesys.TypeSymbol.Namespace`, `Dependency`, and `Artifact` fields. Preserve `typ.File`, `typ.Range`, `member.Range`, and object/field metadata.

- [ ] **Step 3: Run proof**

```bash
go test ./internal/codeintel ./internal/typesys ./internal/schema
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/codeintel internal/typesys/symbols.go
git commit -m "feat: index declarations for code intelligence"
```

## Parallel Wave 1: Use Collection Lanes

Start these after Task 2 is merged to the integration branch. Use separate worktrees.

### Lane 1A: Apex Type And Member Uses

**Files:**
- Create: `internal/codeintel/apex_uses.go`
- Create: `internal/codeintel/apex_uses_test.go`
- Modify: `internal/codeintel/build.go`

**Subagent prompt:**

```text
Implement Apex type/member use collection for internal/codeintel. Stay inside internal/codeintel unless a test proves a missing exported helper is required. Use the existing typesys.Index and source files. Ignore comments and ordinary string literals. Resolve class references, constructor calls, static member calls, instance member calls when local variable type is visible in the same method, extends, implements, and trigger object references. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Write tests**

Required test cases:

- `new InvoiceService().total(a)` resolves constructor and instance method call.
- `InvoiceService.staticTotal(a)` resolves static method call.
- `public class Child extends Base implements Worker` emits `UseExtends` and `UseImplements`.
- comments and string literals do not create references.
- duplicate method names in different classes do not cross-resolve.

- [ ] **Step 2: Implement lexical scanner**

Add scanner helpers:

```go
type token struct {
	Text  string
	File  string
	Range diagnostic.Range
}

func apexTokens(file, source string) []token
func collectApexUses(index typesys.Index, declarations Graph) []Use
```

The scanner must skip:

- `// line comments`
- `/* block comments */`
- single-quoted Apex strings
- double-quoted XML-ish strings if present in test fixtures

- [ ] **Step 3: Implement simple scope type collection**

Handle local declarations shaped like:

```apex
Account a = new Account(Name = 'Acme');
List<Account> accounts = new List<Account>();
InvoiceService svc = new InvoiceService();
```

Add helper:

```go
func localTypesBefore(tokens []token, offsetToken int) map[string]string
```

Do not try full Apex control-flow in this lane. Ambiguous receivers should produce unresolved uses, not false resolved references.

- [ ] **Step 4: Run proof**

```bash
go test ./internal/codeintel -run 'Apex|Declaration' -count=1
```

Expected: all tests pass.

### Lane 1B: Schema, SOQL, DML, And Trigger Uses

**Files:**
- Create: `internal/codeintel/schema_uses.go`
- Create: `internal/codeintel/schema_uses_test.go`
- Modify: `internal/codeintel/build.go`

**Subagent prompt:**

```text
Implement schema and query use collection for internal/codeintel. Use existing schema.Object data and internal/soql.Parse. Resolve SObject construction, SObject field reads/writes, static SOQL object and field references, child subqueries, relationship fields, DML mutations, trigger object references, and Schema.SObjectType/Object__c.SObjectType token patterns. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Write tests**

Required test cases:

- `Account a = new Account(Name = 'Acme')` resolves `Account` and `Account.Name`.
- `a.Name = 'Other'` emits `UseWrite` for `Account.Name`.
- `[SELECT Id, Name FROM Account WHERE Owner.Name != null]` resolves `Account`, `Account.Id`, `Account.Name`, and relationship path metadata.
- `insert a` emits `UseMutate` for `Account`.
- `trigger AccountTrigger on Account` emits trigger object reference.
- `Schema.SObjectType.Account.fields.Name` resolves object and field.

- [ ] **Step 2: Implement SOQL collector**

Use:

```go
query, err := soql.Parse(queryText)
```

Walk:

- `query.Object`
- `query.Fields`
- `query.Where`
- `query.Order`
- `query.GroupBy`
- `query.Aggregates`
- `query.ChildQueries`
- `query.Typeofs`

When ranges are not available from `internal/soql`, use the enclosing SOQL literal range and put `metadata["rangePrecision"]="query"`.

- [ ] **Step 3: Implement SObject field write collector**

Support these first:

```apex
record.Field__c = value;
record.put('Field__c', value);
new Object__c(Field__c = value);
```

Use local variable type collection from Lane 1A if already merged. If not merged, duplicate a tiny local helper and let controller reconcile during integration.

- [ ] **Step 4: Run proof**

```bash
go test ./internal/codeintel -run 'Schema|SOQL|DML|Trigger' -count=1
```

Expected: all tests pass.

### Lane 1C: SOSL Parser And Uses

**Files:**
- Create: `internal/sosl/parser.go`
- Create: `internal/sosl/parser_test.go`
- Modify: `internal/codeintel/schema_uses.go`
- Modify: `internal/codeintel/schema_uses_test.go`

**Subagent prompt:**

```text
Add a small SOSL parser for symbol extraction only. Support FIND {...} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id). Do not build a runtime executor. Integrate parsed RETURNING objects and fields into codeintel schema uses. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Write parser tests**

Cases:

```text
FIND {Acme} RETURNING Account(Id, Name)
FIND 'Acme' IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id)
FIND :term RETURNING Invoice__c(Id, Amount__c WHERE Amount__c > 0)
```

Expected model:

```go
type Query struct {
	Returning []ReturningObject
}
type ReturningObject struct {
	Object string
	Fields []string
}
```

- [ ] **Step 2: Implement parser**

Keep it deterministic and small. Tokenize identifiers, commas, parentheses, and skip nested `WHERE` clauses after collecting field identifiers.

- [ ] **Step 3: Integrate with codeintel**

Static SOSL in Apex uses square brackets beginning with `FIND`. Add object/field uses for returned objects and fields.

- [ ] **Step 4: Run proof**

```bash
go test ./internal/sosl ./internal/codeintel -run 'SOSL|Schema' -count=1
```

Expected: all tests pass.

## Task 3: Integrate Codeintel Build

**Parallelism:** Serial after Wave 1 lanes are merged.

**Files:**
- Create: `internal/codeintel/build.go`
- Create: `internal/codeintel/build_test.go`
- Modify: `internal/codeintel/declarations.go`
- Modify: `internal/codeintel/apex_uses.go`
- Modify: `internal/codeintel/schema_uses.go`

- [ ] **Step 1: Write full build tests**

Test:

```go
func TestBuildCombinesDeclarationsAndUses(t *testing.T)
func TestBuildKeepsUnresolvedUsesWithDiagnostic(t *testing.T)
func TestBuildSortsDeterministicJSON(t *testing.T)
```

Expected:

- all declarations present.
- resolved uses point to symbol IDs.
- unresolved references include `Resolved=false` and a warning diagnostic only where useful.
- JSON output is deterministic across two builds.

- [ ] **Step 2: Implement build orchestration**

Add:

```go
type Options struct {
	IncludeDependencies bool
	IncludeUnresolved  bool
	UseCache           bool
}

func Build(index typesys.Index, opts Options) (Graph, error)
```

Default behavior:

- include project symbols.
- include dependency declarations.
- include project source uses.
- skip dependency source scans unless `IncludeDependencies=true`.

- [ ] **Step 3: Run proof**

```bash
go test ./internal/codeintel ./internal/typesys ./internal/sema ./internal/schema ./internal/soql ./internal/sosl
```

Expected: all pass.

- [ ] **Step 4: Commit integration**

```bash
git add internal/codeintel internal/sosl
git commit -m "feat: build project code intelligence graph"
```

## Parallel Wave 2: Consumers And Storage

Start these after Task 3 is merged.

### Lane 2A: Dependency Artifact Symbol Contracts

**Files:**
- Modify: `internal/packageartifact/artifact.go`
- Modify: `internal/packageartifact/artifact_test.go`
- Modify: `internal/typesys/symbols.go`
- Create: `internal/codeintel/artifact.go`
- Create: `internal/codeintel/artifact_test.go`

**Subagent prompt:**

```text
Extend Glade package artifacts with code intelligence symbol contracts. Preserve backward compatibility for existing artifact JSON. Add versioned fields for symbols and uses that let consumers resolve public/global dependency types, methods, objects, fields, labels, static resources, and custom metadata records without source. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Add artifact fields**

Add to `packageartifact.Artifact`:

```go
ArtifactSchemaVersion int `json:"artifactSchemaVersion,omitempty"`
Symbols []codeintel.Symbol `json:"symbols,omitempty"`
Uses []codeintel.Use `json:"uses,omitempty"`
```

If import cycles appear, use a narrow `packageartifact.Symbol` mirror and conversion functions in `internal/codeintel/artifact.go`. Do not create a cycle.

- [ ] **Step 2: Test backward compatibility**

Existing JSON without `artifactSchemaVersion`, `symbols`, or `uses` must still load.

- [ ] **Step 3: Test dependency consumption**

Build a consumer project with:

```yaml
project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json:1.0"]
```

Expected: `typesys.Build` and `codeintel.Build` include dependency symbols marked `Dependency=true` and `Artifact=true`.

- [ ] **Step 4: Run proof**

```bash
go test ./internal/packageartifact ./internal/typesys ./internal/codeintel -run 'Artifact|Dependency' -count=1
```

### Lane 2B: Symbol Cache

**Files:**
- Create: `internal/codeintel/cache.go`
- Create: `internal/codeintel/cache_test.go`
- Modify: `internal/project/project.go` only if project freshness needs a shared helper.
- Modify: `docs/LOCAL_TESTING.md`

**Subagent prompt:**

```text
Add editor-neutral symbol cache read/write under .glade/symbols. Cache codeintel.Graph as JSON with project root, source hash, Glade version if available, and input file mtimes/hashes. Do not change test/DAP caches. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Define cache files**

Use:

```text
.glade/symbols/index.json
.glade/symbols/schema.json
.glade/symbols/dependencies.json
```

- [ ] **Step 2: Implement API**

```go
type CacheMetadata struct {
	SchemaVersion int `json:"schemaVersion"`
	ProjectRoot string `json:"projectRoot"`
	SourceHash string `json:"sourceHash"`
	CreatedAt time.Time `json:"createdAt"`
}

func CacheDir(projectRoot string) string
func WriteCache(projectRoot string, graph Graph) error
func ReadCache(projectRoot string) (Graph, CacheMetadata, error)
func CacheFresh(projectRoot string, index typesys.Index) bool
func ClearCache(projectRoot string) error
```

- [ ] **Step 3: Write tests**

Required:

- write then read preserves symbols and uses.
- stale source hash returns not fresh.
- missing cache returns typed `ErrCacheMiss`.
- clear removes only `.glade/symbols`.

- [ ] **Step 4: Run proof**

```bash
go test ./internal/codeintel -run 'Cache' -count=1
```

### Lane 2C: Inspect Definition And References

**Files:**
- Modify: `internal/gladecli/cli.go`
- Create: `internal/gladecli/inspect_intelligence_test.go`
- Modify: `internal/cliui/help.go`
- Modify: `docs/EDITOR.md`

**Subagent prompt:**

```text
Expose codeintel graph through CLI inspect subcommands. Add glade inspect definition and glade inspect references. Keep output useful in text and JSON. Do not touch contrib/vscode-glade. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Add command shapes**

Support:

```bash
glade inspect definition --project . --symbol InvoiceService
glade inspect definition --project . --file force-app/main/default/classes/InvoiceService.cls --line 6 --column 13
glade inspect references --project . --symbol InvoiceService.total --json
glade inspect references --project . --symbol Account.Name --include-declaration
```

- [ ] **Step 2: Text output contract**

Definition text:

```text
Definition
  symbol: InvoiceService.total
  kind: apex_member
  file: force-app/main/default/classes/InvoiceService.cls
  range: 6:17-6:22
```

References text:

```text
References
  symbol: Account.Name
  count: 3
  force-app/main/default/classes/InvoiceService.cls:8:21 read
  force-app/main/default/classes/InvoiceService.cls:9:7 write
```

- [ ] **Step 3: JSON output contract**

Use CLI envelope:

```json
{
  "command": "inspect references",
  "status": "ok",
  "data": {
    "symbol": {},
    "references": []
  }
}
```

- [ ] **Step 4: Run proof**

```bash
go test ./internal/gladecli ./internal/codeintel -run 'Inspect.*Definition|Inspect.*References|CodeIntel' -count=1
```

### Lane 2D: Enterprise Graph And Watch Selection

**Files:**
- Modify: `internal/enterprisegraph/build.go`
- Modify: `internal/enterprisegraph/build_test.go`
- Modify: `internal/watch/refgraph.go`
- Modify: `internal/watch/refgraph_test.go`
- Modify: `internal/watch/affected.go`
- Modify: `internal/watch/affected_test.go`

**Subagent prompt:**

```text
Replace conservative word-scan graph edges with codeintel references where possible. Preserve safe over-selection for tests when codeintel cannot resolve a change. Enterprise graph reports should gain precision, not lose nodes. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Update enterprise graph builder**

Use `codeintel.Build(ctx.Index, codeintel.Options{})`.

Map:

- `SymbolApexType` -> class/interface/enum nodes.
- `SymbolApexMember` -> method/field/test method nodes.
- `SymbolSObject` -> SObject or platform event nodes.
- `UseCall` -> `EdgeKindCalls`.
- `UseRead`, `UseWrite`, `UseConstruct` -> `EdgeKindReferences`.
- `UseQuery` -> `EdgeKindQueries`.
- `UseMutate` -> `EdgeKindMutates`.
- `UseMetadata` -> `EdgeKindMetadataReferences`.

- [ ] **Step 2: Update watch ref graph**

Keep `RefGraph` API stable. Internally build dependencies from codeintel uses. If codeintel returns unresolved uses in a changed file, keep the existing conservative all-test fallback.

- [ ] **Step 3: Run proof**

```bash
go test ./internal/enterprisegraph ./internal/watch ./internal/codeintel -count=1
```

Expected: existing tests pass and new precision tests pass.

## Task 4: Merge Wave 2 And Verify

**Parallelism:** Serial integration.

- [ ] **Step 1: Merge lanes in this order**

```bash
git merge --no-ff codex/codeintel-artifact
go test ./internal/packageartifact ./internal/typesys ./internal/codeintel

git merge --no-ff codex/codeintel-cache
go test ./internal/codeintel

git merge --no-ff codex/codeintel-inspect
go test ./internal/gladecli ./internal/codeintel

git merge --no-ff codex/codeintel-graph-watch
go test ./internal/enterprisegraph ./internal/watch ./internal/codeintel
```

- [ ] **Step 2: Resolve conflicts by keeping shared `codeintel` names stable**

If conflicts appear in `internal/codeintel/build.go`, preserve:

- `Build(index typesys.Index, opts Options) (Graph, error)`
- `BuildDeclarations(index typesys.Index) Graph`
- `collectApexUses`
- `collectSchemaUses`

- [ ] **Step 3: Run combined proof**

```bash
go test ./internal/codeintel ./internal/packageartifact ./internal/typesys ./internal/gladecli ./internal/enterprisegraph ./internal/watch ./internal/schema ./internal/sema
```

Expected: all pass.

- [ ] **Step 4: Commit merge fixes**

```bash
git add internal docs
git commit -m "feat: wire code intelligence into product surfaces"
```

## Parallel Wave 3: Refactor, Offline Symbols, And Server APIs

Start after Task 4.

### Lane 3A: Offline Describe Symbols

**Files:**
- Modify: `internal/gladecli/cli.go` around `runSchemaImportDescribe`
- Modify: `internal/schema/schema.go`
- Modify: `internal/codeintel/cache.go`
- Create: `internal/codeintel/offline_schema_test.go`
- Modify: `internal/gladecli/test_command_selectors_test.go` or create a new schema import test.

**Subagent prompt:**

```text
Extend captured describe import so Glade can turn describe JSON into cached symbol data. Do not add live Salesforce auth. Add --project-cache <root> to glade schema import describe so imported schema can feed .glade/symbols/schema.json. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Add CLI flag**

Command:

```bash
glade schema import describe --input reports/org-describe.json --output schema/local.schema.json --project-cache .
```

Behavior:

- existing `--input` and `--output` behavior stays unchanged.
- `--project-cache <root>` writes schema symbol cache under `<root>/.glade/symbols/schema.json`.
- command fails if `<root>` is not a project root.

- [ ] **Step 2: Add tests**

Use a small describe JSON fixture with Account and a custom object.

Expected:

- output schema JSON still writes.
- cache file writes.
- `codeintel.ReadCache` or schema cache reader can load object and field symbols.

- [ ] **Step 3: Run proof**

```bash
go test ./internal/gladecli ./internal/schema ./internal/codeintel -run 'Schema.*Describe|OfflineSchema|Cache' -count=1
```

### Lane 3B: Safe Rename Refactor

**Files:**
- Create: `internal/refactor/rename.go`
- Create: `internal/refactor/rename_test.go`
- Modify: `internal/codeintel/patch.go`
- Create: `internal/gladecli/refactor_command.go`
- Create: `internal/gladecli/refactor_command_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`

**Subagent prompt:**

```text
Implement safe rename as a core Glade command. Start with symbols that codeintel resolves exactly: Apex type names, Apex member names with unique symbol IDs, SObject fields, and SObjects. Add dry-run JSON and optional write. Refuse ambiguous or unresolved references. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Add rename planner**

API:

```go
type RenameOptions struct {
	ProjectRoot string
	Symbol string
	File string
	Line int
	Column int
	NewName string
	Write bool
}

type RenamePlan struct {
	Symbol codeintel.Symbol `json:"symbol"`
	NewName string `json:"newName"`
	Edits []FileEdit `json:"edits"`
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics,omitempty"`
}

func PlanRename(index typesys.Index, opts RenameOptions) (RenamePlan, error)
func Apply(plan RenamePlan) error
```

- [ ] **Step 2: Add safety rules**

Refuse rename when:

- symbol cannot be resolved.
- any reference is unresolved.
- two edits overlap.
- new name is not a valid Apex identifier for Apex symbols.
- new field/object name lacks required `__c` suffix for custom schema symbols.
- target file changed between planning and write.

- [ ] **Step 3: Add CLI**

Command:

```bash
glade refactor rename --project . --symbol InvoiceService --to BillingService --dry-run --json
glade refactor rename --project . --file force-app/main/default/classes/InvoiceService.cls --line 5 --column 14 --to totalNet --write
```

Default is dry-run. `--write` applies edits.

- [ ] **Step 4: Tests**

Required:

- dry-run returns edits without writing.
- write changes all exact references.
- ambiguous method name fails with diagnostic.
- schema field rename updates Apex references but does not edit metadata XML in this first pass unless that XML location is represented as a declaration.
- overlapping edits fail.

- [ ] **Step 5: Run proof**

```bash
go test ./internal/refactor ./internal/gladecli ./internal/codeintel -run 'Rename|Refactor' -count=1
```

### Lane 3C: SOQL And SOSL Semantic Diagnostics

**Files:**
- Modify: `internal/sema/sema_checks.go`
- Modify: `internal/sema/sema_test.go`
- Modify: `internal/soql/query.go` only if range metadata is needed.
- Modify: `internal/sosl/parser.go`
- Create: `internal/sema/query_semantics_test.go`

**Subagent prompt:**

```text
Use codeintel schema/query resolution to tighten SOQL and SOSL diagnostics in sema. Focus on unknown object, unknown field, relationship field, child query relationship, aggregate alias, TYPEOF branch object, and SOSL RETURNING fields. Do not change runtime query execution in this lane. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Tests**

Add diagnostics with stable codes:

- `GLADESEMA_QUERY_OBJECT`
- `GLADESEMA_QUERY_FIELD`
- `GLADESEMA_QUERY_RELATIONSHIP`
- `GLADESEMA_SOSL_FIELD`

Each test should assert code and range line/column when available.

- [ ] **Step 2: Implement checks**

Use `codeintel.Build(index, codeintel.Options{IncludeUnresolved: true})` or a narrower resolver helper. Emit diagnostics for unresolved query uses with query metadata.

- [ ] **Step 3: Run proof**

```bash
go test ./internal/sema ./internal/codeintel ./internal/soql ./internal/sosl -run 'SOQL|SOSL|Query|Schema' -count=1
```

### Lane 3D: Local Server Symbol Query API

**Files:**
- Modify: `internal/server/source_metadata.go`
- Create: `internal/server/codeintel_test.go`
- Modify: `docs/INSTALL.md`

**Subagent prompt:**

```text
Expose codeintel through local server endpoints for scripts and tools. Add Glade-local endpoints under /services/data/vXX.X/tooling/glade/symbols and /references. Keep Salesforce-shaped existing endpoints unchanged. Return DONE with tests and commit SHA.
```

- [ ] **Step 1: Add endpoints**

Add local-only endpoints:

```text
GET /services/data/v61.0/tooling/glade/symbols
GET /services/data/v61.0/tooling/glade/definition?symbol=Account.Name
GET /services/data/v61.0/tooling/glade/references?symbol=Account.Name
```

Return JSON:

```json
{
  "totalSize": 1,
  "done": true,
  "records": []
}
```

- [ ] **Step 2: Tests**

Use existing server test setup with a small project index. Assert response status and JSON shape.

- [ ] **Step 3: Run proof**

```bash
go test ./internal/server ./internal/codeintel -run 'CodeIntel|Tooling|Symbols' -count=1
```

## Task 5: Merge Wave 3 And Verify

**Parallelism:** Serial integration.

- [ ] **Step 1: Merge lanes in this order**

```bash
git merge --no-ff codex/codeintel-offline-schema
go test ./internal/gladecli ./internal/schema ./internal/codeintel

git merge --no-ff codex/codeintel-refactor
go test ./internal/refactor ./internal/gladecli ./internal/codeintel

git merge --no-ff codex/codeintel-query-sema
go test ./internal/sema ./internal/soql ./internal/sosl ./internal/codeintel

git merge --no-ff codex/codeintel-server-api
go test ./internal/server ./internal/codeintel
```

- [ ] **Step 2: Run product proof**

```bash
go test ./internal/codeintel ./internal/refactor ./internal/sosl ./internal/sema ./internal/schema ./internal/typesys ./internal/project ./internal/packageartifact ./internal/gladecli ./internal/enterprisegraph ./internal/watch ./internal/server
```

Expected: all pass.

- [ ] **Step 3: Commit merge fixes**

```bash
git add internal docs
git commit -m "feat: add offline symbols and safe refactor surfaces"
```

## Task 6: Documentation And CLI Help

**Parallelism:** Can run in parallel with final review after Task 5, but merge after code settles.

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/EDITOR.md`
- Modify: `docs/LOCAL_TESTING.md`
- Modify: `docs/COMPATIBILITY.md`
- Modify: `docs/CI_ARTIFACTS.md`
- Modify: `internal/cliui/help.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Document architecture**

Add a short section:

```markdown
## Code Intelligence

`internal/codeintel` builds editor-neutral project intelligence from the type
index, metadata schema, dependency artifacts, and source files. It powers
symbol inspection, reference queries, conservative refactor planning, enterprise
graph edges, changed-test selection, and future editor clients.
```

- [ ] **Step 2: Document commands**

Add examples:

```bash
glade inspect definition --project . --symbol Account.Name
glade inspect references --project . --symbol InvoiceService.total --json
glade refactor rename --project . --symbol InvoiceService --to BillingService --dry-run
glade schema import describe --input reports/org-describe.json --project-cache .
```

- [ ] **Step 3: Update help tests**

Add assertions that:

- `glade inspect --help` lists `definition` and `references`.
- `glade schema --help` lists `--project-cache`.
- `glade refactor --help` lists `rename`.

- [ ] **Step 4: Run proof**

```bash
go test ./internal/gladecli ./internal/cliui
```

If `internal/cliui` has no package tests, run:

```bash
go test ./internal/gladecli
```

## Task 7: Performance And Cache Benchmarks

**Parallelism:** Can run after Task 5. Keep separate worktree.

**Files:**
- Create: `internal/codeintel/build_benchmark_test.go`
- Modify: `internal/typesys/symbols_benchmark_test.go` only if shared fixture helpers move.
- Modify: `docs/LOCAL_TESTING.md`

- [ ] **Step 1: Add benchmark**

Benchmark:

```go
func BenchmarkBuildCodeIntelSmallProject(b *testing.B)
func BenchmarkBuildCodeIntelCachedProject(b *testing.B)
```

Use generated temp projects with 25 classes and 10 objects. Do not check in giant fixtures.

- [ ] **Step 2: Add cache assertion test**

Test second build with cache avoids source re-read where cache is fresh. Use a counter hook or exported test-only file reader in `internal/codeintel`.

- [ ] **Step 3: Run proof**

```bash
go test ./internal/codeintel -run 'Cache|Build' -bench 'BenchmarkBuildCodeIntel' -benchmem
```

Expected: benchmark runs and reports allocations. Do not assert exact timings.

## Task 8: Full Verification

**Parallelism:** Serial.

- [ ] **Step 1: Run focused suite**

```bash
go test ./internal/codeintel ./internal/refactor ./internal/sosl ./internal/sema ./internal/schema ./internal/typesys ./internal/project ./internal/packageartifact ./internal/gladecli ./internal/enterprisegraph ./internal/watch ./internal/server
```

Expected: all pass.

- [ ] **Step 2: Run broad suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 3: Run smoke if broad suite passes**

```bash
scripts/smoke.sh
```

Expected: smoke passes. If laptop contention is high, run this only after focused tests and tell the user it is the remaining heavy gate.

- [ ] **Step 4: Check docs and generated files**

```bash
git diff --check
```

Expected: no whitespace errors.

- [ ] **Step 5: Inspect public help**

```bash
go run ./cmd/glade inspect --help
go run ./cmd/glade schema --help
go run ./cmd/glade refactor --help
```

Expected: help names only product behavior. No compatibility scanner or maintenance wording appears.

## Stop Rules

Stop and report before continuing if any of these happen:

- The implementation requires live Salesforce login.
- A public command addition conflicts with product direction.
- Rename cannot distinguish same-named members across classes.
- A refactor test passes by word replacement instead of symbol references.
- `go test ./internal/codeintel` is flaky twice.
- A parallel lane needs to edit the same files as another active lane.
- The cache format cannot stay backward compatible.

## Final Review Prompt

Use this prompt for the final review subagent:

```text
Review the core symbol intelligence branch. Focus on bugs, unsafe refactors, cache invalidation holes, public CLI contract drift, dependency artifact compatibility, and maintenance-boundary violations. Do not summarize first. List findings with file/line references. Verify that no VS Code extension files were changed.
```

## Completion Criteria

The branch is complete when:

- `internal/codeintel` builds deterministic symbols and references from project source, schema, dependency artifacts, and cached describe symbols.
- `glade inspect definition` and `glade inspect references` work in text and JSON.
- `glade refactor rename` produces safe dry-run plans and writes only when references are exact.
- dependency artifacts carry enough symbol contract data for consumers without source.
- enterprise graph and changed-test selection use codeintel where possible and keep conservative fallbacks.
- schema import can write offline symbol cache from captured describe JSON.
- SOQL and SOSL semantic diagnostics use schema-aware references.
- focused tests, broad tests, smoke, and `git diff --check` pass.
