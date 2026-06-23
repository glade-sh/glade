# Public Corpus Check Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make public corpus projects pass `glade check` with only performance warnings, except for findings proven to be actual missing project metadata or missing package source.

**Architecture:** Use the public corpus report as the ratchet, but implement product behavior only in Glade. Keep corpus reporting and classification in the sibling `glade-tools` project. Product fixes should land as small, public-shaped tests in `internal/sema`, `internal/typesys`, `internal/project`, parser-facing source normalization, and generated standard-symbol contracts before any implementation edits.

**Tech Stack:** Go, Glade sema/type system, generated Salesforce standard symbols, `glade-tools corpus check`, public Apex corpus under `/Users/matt/Dev/glade-corpus/public`.

---

## Current Evidence

Run from `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage` and `/Users/matt/Dev/glade-tools`:

```bash
go build -o /tmp/glade-sema-coverage ./cmd/glade
rm -rf /tmp/glade-corpus-public-check-run
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/public \
  --glade /tmp/glade-sema-coverage \
  --out /tmp/glade-corpus-public-check-run
```

Current result:

```text
projects=308 diagnostics=2760 unclassified=0
project-discovery-duplicate 1282
project-metadata-missing 906
semantic-contract-gap 488
performance-advisory 79
source-parse-error 5
```

The target is stricter than sema closure:

```text
semantic-contract-gap = 0
source-parse-error = 0
project-discovery-duplicate = 0
unclassified = 0
performance-advisory may remain
project-metadata-missing may remain only when backed by evidence
```

The largest public examples are in `LightningFlowComponents`, `AnalyzeFlows`, `at4dx`, `SetupViaFlow`, `CloneAndTweak`, `Apex-Opensource-Library`, `sfdx-mass-action-scheduler`, `PostRichChatter`, `automation-components`, and `ExecuteNBAStrategy`.

Allowed at the end:

- `performance-advisory`: real project findings.
- `project-metadata-missing`: only rows with evidence that the project lacks the referenced custom object, field, Flow variable metadata, package source, or generated metadata source.

Not allowed at the end:

- `project-discovery-duplicate`: Glade or the corpus runner must not count duplicate aggregate roots as check failures.
- `source-parse-error`: parser-facing source normalization or parser support must be fixed for public Apex syntax.
- `semantic-contract-gap`: Salesforce/Apex product behavior must be added, or the row must be reclassified with evidence as actual metadata.
- `unclassified`: every row must have a class.

## File Map

- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go`
  - Keep the public/private corpus ratchet honest. Classify missing package source and generated metadata as metadata, not product sema. Add a final allowed-finding gate.
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go`
  - Add public corpus classifier rows for `usf3.MetadataService.*`, `fflib_*`, missing mock helpers, Flow metadata variables, and real product-shaped rows that must stay semantic.
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_command.go`
  - If needed, add a corpus check flag that fails on disallowed classes while allowing performance and proven metadata.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/symbols.go`
  - Ensure normalized Apex source is used before parsing.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
  - Add all public-shaped failing tests first.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_calls.go`
  - Text inference, method-call return inference, array/list aliases, collection method validation.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_ir.go`
  - IR field-path checks, dynamic Flow/metadata receiver handling, field initialization tracking.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/type_members.go`
  - Nested class member lookup, inherited interface matching, standard/custom object field overlays.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_checks.go`
  - Inheritance and interface checks.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols.go`
  - Hand-maintained platform rows such as `Test.isRunningTest`, `Math.PI`, `Math.E`, `Math.random`, `Approval.process`.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols_test.go`
  - Standard-symbol assertions for platform rows.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/apex_docs_contracts.json`
  - Docs contract rows for ConnectApi shapes.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/system_stub_symbols_generated.go`
  - Regenerate after docs contract updates.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/scripts/generate-system-stub-symbols.mjs`
  - Only if the generated docs contract cannot express the needed property or method shape.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/project/project.go`
  - Only if Flow metadata variable extraction is needed for product behavior.
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/project/project_test.go`
  - Tests for Flow metadata extraction, if implemented.

---

### Task 0: Add the Final Public Check Ratchet

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_command.go`

- [ ] **Step 1: Add a failing ratchet test**

Add to `corpuscheck_test.go`:

```go
func TestReportDisallowedFindingsForPublicCheckClosure(t *testing.T) {
	report := Report{Counts: map[string]int{
		"performance-advisory":       3,
		"project-metadata-missing":   2,
		"semantic-contract-gap":      1,
		"source-parse-error":         1,
		"project-discovery-duplicate": 1,
	}}
	got := DisallowedForCheckClosure(report)
	want := map[string]int{
		"semantic-contract-gap":      1,
		"source-parse-error":         1,
		"project-discovery-duplicate": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DisallowedForCheckClosure() = %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 2: Run red**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpuscheck -run TestReportDisallowedFindingsForPublicCheckClosure -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Implement the final gate**

Add:

```go
func DisallowedForCheckClosure(report Report) map[string]int {
	disallowed := map[string]int{}
	for class, count := range report.Counts {
		switch class {
		case "performance-advisory", "project-metadata-missing":
			continue
		default:
			if count > 0 {
				disallowed[class] = count
			}
		}
	}
	return disallowed
}
```

If a CLI flag is useful, add `--fail-on-check-closure` and make it return an error like:

```go
return fmt.Errorf("public check closure failed: %v", disallowed)
```

- [ ] **Step 4: Verify green**

```bash
go test ./internal/corpuscheck ./internal/toolcli -count=1
```

Expected:

```text
ok
```

- [ ] **Step 5: Commit**

```bash
git add internal/corpuscheck internal/toolcli
git commit -m "test: add public check closure ratchet"
```

---

### Task 0.5: Remove Duplicate Discovery From Valid Monorepo Aggregates

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/project/project.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/project/project_test.go`

- [ ] **Step 1: Add a corpus discovery test for nested public projects**

Add to `corpuscheck_test.go`:

```go
func TestDiscoverProjectsSkipsAggregateRootWhenNestedProjectsExist(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, "LightningFlowComponents")
	writeProject(t, filepath.Join(root, "LightningFlowComponents", "flow_action_components"), "PostRichChatter")
	writeProject(t, filepath.Join(root, "LightningFlowComponents", "flow_screen_components"), "QuickQuery")

	projects, err := discoverProjects(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range projects {
		if filepath.Base(project) == "LightningFlowComponents" {
			t.Fatalf("aggregate root should not be checked when nested projects exist: %#v", projects)
		}
	}
}
```

- [ ] **Step 2: Run red**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpuscheck -run TestDiscoverProjectsSkipsAggregateRootWhenNestedProjectsExist -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Implement aggregate-root skipping in the corpus runner**

In `discoverProjects`, after collecting all `sfdx-project.json` roots, drop a parent root when it contains one or more nested project roots below it:

```go
func removeAggregateProjectRoots(projects []string) []string {
	sort.Strings(projects)
	nestedUnder := map[string]bool{}
	for _, parent := range projects {
		parentWithSep := parent + string(os.PathSeparator)
		for _, child := range projects {
			if child != parent && strings.HasPrefix(child, parentWithSep) {
				nestedUnder[parent] = true
				break
			}
		}
	}
	out := projects[:0]
	for _, project := range projects {
		if !nestedUnder[project] {
			out = append(out, project)
		}
	}
	return out
}
```

This belongs in `glade-tools` corpus orchestration. Do not hide duplicate top-level classes inside product `glade check` unless the project loader is genuinely reading the same package directory twice.

- [ ] **Step 4: Add a product project test only if Glade double-loads one package**

If a specific `sfdx-project.json` package directory appears twice through symlinks or path normalization, add this product test:

```go
func TestLoadProjectDeduplicatesPackageDirectoriesByRealPath(t *testing.T) {
	root := t.TempDir()
	forceApp := filepath.Join(root, "force-app")
	if err := os.MkdirAll(filepath.Join(forceApp, "main/default/classes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{
  "packageDirectories": [
    {"path": "force-app", "default": true},
    {"path": "./force-app"}
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	proj, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(proj.PackageDirectories) != 1 {
		t.Fatalf("package directories = %d, want 1", len(proj.PackageDirectories))
	}
}
```

- [ ] **Step 5: Verify duplicate rows are gone from the public corpus**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go build -o /tmp/glade-sema-coverage ./cmd/glade
cd /Users/matt/Dev/glade-tools
rm -rf /tmp/glade-corpus-public-check-run
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/public \
  --glade /tmp/glade-sema-coverage \
  --out /tmp/glade-corpus-public-check-run
sed -n '1,80p' /tmp/glade-corpus-public-check-run/classified.tsv
```

Expected:

```text
no project-discovery-duplicate row
```

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/corpuscheck
git commit -m "fix: skip aggregate corpus project roots"
```

---

### Task 0.75: Close Public Namespace-Template Parse Errors

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/project/project.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/project/project_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/symbols.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/symbols_test.go`

- [ ] **Step 1: Capture the five public parse failures**

Run:

```bash
/tmp/glade-sema-coverage check --project /Users/matt/Dev/glade-corpus/public/EDA --no-progress
```

Current expected rows:

```text
CON_Pref_Email_duplicateField_FTST.cls:47:56 APEXPARSE001
CON_Pref_Email_untranslatPcklstSpsh_UTST.cls:64:13 APEXPARSE001
CON_Pref_Email_untranslatedSpsh_UTST.cls:48:13 APEXPARSE001
CON_Pref_Email_Utility.cls:49:9 APEXPARSE001
TDTM_Utility.cls:112:9 APEXPARSE001
```

- [ ] **Step 2: Write namespace placeholder normalization tests**

The failing EDA lines use CumulusCI-style namespace tokens, not Apex grammar:

```apex
%%%NAMESPACE_DOT%%%UTIL_CustomSettings_API.getSettingsForTests(
    new %%%NAMESPACE%%%Hierarchy_Settings__c(%%%NAMESPACE%%%Disable_Preferred_Email_Enforcement__c = false)
);

for (%%%NAMESPACE_DOT%%%TDTM_Global_API.TdtmToken tdtmToken : %%%NAMESPACE_DOT%%%TDTM_Global_API.getTdtmConfig()) {
}
```

Add tests for unnamespaced and namespaced source in `project_test.go`:

```go
func TestNormalizeApexNamespaceTokensForUnnamespacedProject(t *testing.T) {
	source := `public class UsesTokens {
  public void run() {
    %%%NAMESPACE_DOT%%%UTIL_CustomSettings_API.getSettingsForTests(
      new %%%NAMESPACE%%%Hierarchy_Settings__c(%%%NAMESPACE%%%Disable_Preferred_Email_Enforcement__c = false)
    );
    for (%%%NAMESPACE_DOT%%%TDTM_Global_API.TdtmToken token : %%%NAMESPACE_DOT%%%TDTM_Global_API.getTdtmConfig()) {
    }
  }
}`
	got := normalizeApexNamespaceTokens(source, "")
	if strings.Contains(got, "%%%") {
		t.Fatalf("namespace token remained in source:\n%s", got)
	}
	if !strings.Contains(got, "UTIL_CustomSettings_API.getSettingsForTests") {
		t.Fatalf("dot token was not removed:\n%s", got)
	}
	if !strings.Contains(got, "new Hierarchy_Settings__c(Disable_Preferred_Email_Enforcement__c = false)") {
		t.Fatalf("API token was not removed:\n%s", got)
	}
}

func TestNormalizeApexNamespaceTokensForNamespacedProject(t *testing.T) {
	source := `public class UsesTokens {
  public void run() {
    %%%NAMESPACE_DOT%%%UTIL_CustomSettings_API.getSettingsForTests(
      new %%%NAMESPACE%%%Hierarchy_Settings__c(%%%NAMESPACE%%%Disable_Preferred_Email_Enforcement__c = false)
    );
  }
}`
	got := normalizeApexNamespaceTokens(source, "hed")
	for _, want := range []string{
		"hed.UTIL_CustomSettings_API.getSettingsForTests",
		"new hed__Hierarchy_Settings__c(hed__Disable_Preferred_Email_Enforcement__c = false)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalized source missing %q:\n%s", want, got)
		}
	}
}
```

Add a parse-through test in `symbols_test.go`:

```go
func TestBuildParsesNamespaceTokenApex(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "UsesTokens.cls")
	if err := os.WriteFile(path, []byte(`public class UsesTokens {
  public void run() {
    %%%NAMESPACE_DOT%%%UTIL_CustomSettings_API.getSettingsForTests(
      new %%%NAMESPACE%%%Hierarchy_Settings__c(%%%NAMESPACE%%%Disable_Preferred_Email_Enforcement__c = false)
    );
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	index := Build(project.Project{
		Root:      root,
		Namespace: "",
		ApexFiles: []string{path},
	}, schema.Schema{})
	for _, diag := range index.Diagnostics {
		if diag.Code == "APEXPARSE001" {
			t.Fatalf("unexpected parse diagnostic: %#v", diag)
		}
	}
}
```

- [ ] **Step 3: Run red**

```bash
go test ./internal/project -run TestNormalizeApexNamespaceTokens -count=1
go test ./internal/typesys -run TestBuildParsesNamespaceTokenApex -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 4: Implement namespace token normalization**

In `project.go`, add:

```go
func normalizeApexNamespaceTokens(source, namespace string) string {
	apiPrefix := ""
	dotPrefix := ""
	if namespace != "" {
		apiPrefix = namespace + "__"
		dotPrefix = namespace + "."
	}
	source = strings.ReplaceAll(source, "%%%NAMESPACE_DOT%%%", dotPrefix)
	source = strings.ReplaceAll(source, "%%%NAMESPACE%%%", apiPrefix)
	return source
}
```

Apply this before Apex source reaches `typesys.Build` and sema source reads. Preserve original source for file display. Use normalized source for parse and semantic analysis.

- [ ] **Step 5: Verify EDA parse closure**

```bash
go test ./internal/project -run TestNormalizeApexNamespaceTokens -count=1
go test ./internal/typesys -run TestBuildParsesNamespaceTokenApex -count=1
go build -o /tmp/glade-sema-coverage ./cmd/glade
/tmp/glade-sema-coverage check --project /Users/matt/Dev/glade-corpus/public/EDA --no-progress
```

Expected:

```text
no APEXPARSE001 diagnostics
only the known GLADEPERF001 row may remain unless sema tasks also remove additional rows
```

- [ ] **Step 6: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go internal/typesys/symbols.go internal/typesys/symbols_test.go
git commit -m "fix: normalize Apex namespace template tokens"
```

---

### Task 1: Tighten Public Corpus Sema Classification

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go`

- [ ] **Step 1: Add failing classifier tests for public metadata rows**

Add these rows to `TestClassifyPrivateCorpusMetadataDiagnostics` or rename the test to `TestClassifyCorpusMetadataDiagnostics`:

```go
{
	name: "generated metadata service nested type is metadata",
	diag: ClassifiedDiagnostic{
		Code:    "GLADESEMA006",
		Message: `method "genereateFlowForTest2" constructs unknown type "usf3.MetadataService.FlowConnector"`,
	},
	want: "project-metadata-missing",
},
{
	name: "missing fflib dependency type is metadata",
	diag: ClassifiedDiagnostic{
		Code:    "GLADESEMA002",
		Message: `method "newQueryFactory" references unknown type "fflib_QueryFactory"`,
	},
	want: "project-metadata-missing",
},
{
	name: "missing test helper class is metadata",
	diag: ClassifiedDiagnostic{
		Code:    "GLADESEMA006",
		Message: `method "test_LaunchFlow" constructs unknown type "MockHttpResponseGenerator"`,
	},
	want: "project-metadata-missing",
},
{
	name: "flow interview generated variable is metadata",
	diag: ClassifiedDiagnostic{
		Code:    "GLADESEMA021",
		Message: `method "handleInboundEmail" references unknown field "AutoReplyMessage" on Flow.Interview.EmailToFlowController`,
	},
	want: "project-metadata-missing",
},
```

Keep these product rows semantic:

```go
{
	name: "connect api list add stays semantic",
	diag: ClassifiedDiagnostic{
		Code:    "GLADESEMA023",
		Message: `method "getMessageSegmentInputs" has invalid collection call "add" with 1 argument(s)`,
	},
	want: "semantic-contract-gap",
},
{
	name: "fluent configure method stays semantic",
	diag: ClassifiedDiagnostic{
		Code:    "GLADESEMA008",
		Message: `method "configure" calls unknown method "bind"`,
	},
	want: "semantic-contract-gap",
},
{
	name: "math constants stay semantic",
	diag: ClassifiedDiagnostic{
		Code:    "GLADESEMA018",
		Message: `method "replaceExpressionValues" initializes Decimal local "pi" with Object`,
	},
	want: "semantic-contract-gap",
},
```

- [ ] **Step 2: Run the classifier tests and verify red**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpuscheck -run 'TestClassify.*Corpus.*Diagnostics' -count=1
```

Expected before implementation:

```text
FAIL
```

- [ ] **Step 3: Implement narrow metadata classifiers**

In `corpuscheck.go`, add helpers that classify generated or missing metadata only when the message carries a clear metadata signature:

```go
func diagnosticLooksLikePublicMetadataGap(code, text string) bool {
	if strings.Contains(text, "usf3.metadataservice.") {
		return true
	}
	if strings.Contains(text, "flow.interview.") && strings.Contains(text, "references unknown field") {
		return true
	}
	if strings.Contains(text, "unknown type \"fflib_") || strings.Contains(text, "unknown constructor target \"fflib_") {
		return true
	}
	if strings.Contains(text, "unknown type \"mockhttpresponsegenerator\"") ||
		strings.Contains(text, "constructs unknown type \"mockhttpresponsegenerator\"") {
		return true
	}
	return false
}
```

Call it immediately after `diagnosticLooksLikeMissingMetadata(code, text)`:

```go
case diagnosticLooksLikeMissingMetadata(code, text):
	return "project-metadata-missing"
case diagnosticLooksLikePublicMetadataGap(code, text):
	return "project-metadata-missing"
```

- [ ] **Step 4: Verify green**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpuscheck ./internal/toolcli -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/tools/internal/corpuscheck
ok  	github.com/glade-sh/glade/tools/internal/toolcli
```

- [ ] **Step 5: Commit the classifier slice**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/corpuscheck/corpuscheck.go internal/corpuscheck/corpuscheck_test.go
git commit -m "test: classify public corpus metadata gaps"
```

---

### Task 2: Add Platform Static Rows for Test, Math, and Approval

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`

- [ ] **Step 1: Write failing typesys tests**

Add to `standard_symbols_test.go`:

```go
func TestStandardPlatformSymbolsIncludePublicCorpusStatics(t *testing.T) {
	symbols := typesByName(typesys.StandardPlatformSymbols())

	testType := requireStandardSymbol(t, symbols, "Test")
	requireStandardMethodReturn(t, testType, "isRunningTest", nil, "Boolean", true)

	mathType := requireStandardSymbol(t, symbols, "Math")
	requireStandardPropertyStatic(t, mathType, "PI", "Decimal", true)
	requireStandardPropertyStatic(t, mathType, "E", "Decimal", true)
	requireStandardMethodReturn(t, mathType, "random", nil, "Decimal", true)

	approval := requireStandardSymbol(t, symbols, "Approval")
	requireStandardMethodReturn(t, approval, "process", []string{"Approval.ProcessRequest"}, "Approval.ProcessResult", true)
}
```

- [ ] **Step 2: Write failing sema tests**

Add to `sema_test.go`:

```go
func TestAnalyzePublicCorpusPlatformStatics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPlatformStatics.cls"), `
public class UsesPlatformStatics {
  public void run() {
    if (Test.isRunningTest()) {
      Decimal pi = Math.PI;
      Decimal e = Math.E;
      Decimal rnd = Math.random();
    }
  }

  public Approval.ProcessResult approve(Approval.ProcessRequest request) {
    return Approval.process(request);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesPlatformStatics.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "Test.isRunningTest")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "Decimal")
	assertNoDiagnosticContaining(t, result, "GLADESEMA009", "Approval.process")
}
```

- [ ] **Step 3: Run red**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go test ./internal/typesys -run TestStandardPlatformSymbolsIncludePublicCorpusStatics -count=1
go test ./internal/sema -run TestAnalyzePublicCorpusPlatformStatics -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 4: Add the platform rows**

In `standard_symbols.go`, extend the existing standard type specs with these entries:

```go
{
	Name: "Test",
	Methods: []StandardMethodSpec{{
		Name:       "isRunningTest",
		ReturnType: "Boolean",
		Static:     true,
	}},
},
{
	Name: "Math",
	Methods: []StandardMethodSpec{{
		Name:       "random",
		ReturnType: "Decimal",
		Static:     true,
	}},
	Properties: []StandardPropertySpec{
		{Name: "PI", Type: "Decimal", Static: true},
		{Name: "E", Type: "Decimal", Static: true},
	},
},
{
	Name: "Approval",
	Methods: []StandardMethodSpec{{
		Name:       "process",
		ReturnType: "Approval.ProcessResult",
		Parameters: []StandardParameterSpec{{
			Name: "request",
			Type: "Approval.ProcessRequest",
		}},
		Static: true,
	}},
},
```

If `Approval.ProcessRequest` or `Approval.ProcessResult` are not present, add minimal standard symbols for them with constructors and no project-specific fields.

- [ ] **Step 5: Verify green**

```bash
go test ./internal/typesys -run TestStandardPlatformSymbolsIncludePublicCorpusStatics -count=1
go test ./internal/sema -run TestAnalyzePublicCorpusPlatformStatics -count=1
```

Expected:

```text
ok
```

- [ ] **Step 6: Commit**

```bash
git add internal/typesys/standard_symbols.go internal/typesys/standard_symbols_test.go internal/sema/sema_test.go
git commit -m "feat: add public corpus platform statics"
```

---

### Task 3: Add ConnectApi DTO Shapes Used by Chatter and NBA

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/testdata/generated/apex_docs_contracts.json`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/standard_symbols_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/typesys/system_stub_symbols_generated.go`

- [ ] **Step 1: Add failing standard-symbol tests**

Add to `standard_symbols_test.go`:

```go
func TestStandardPlatformSymbolsIncludePublicCorpusConnectApiShapes(t *testing.T) {
	symbols := typesByName(typesys.StandardPlatformSymbols())

	messageInput := requireStandardSymbol(t, symbols, "ConnectApi.MessageBodyInput")
	requireStandardProperty(t, messageInput, "messageSegments", "List<ConnectApi.MessageSegmentInput>")

	textInput := requireStandardSymbol(t, symbols, "ConnectApi.TextSegmentInput")
	requireStandardProperty(t, textInput, "text", "String")

	nbaRecommendation := requireStandardSymbol(t, symbols, "ConnectApi.NBARecommendation")
	requireStandardProperty(t, nbaRecommendation, "acceptanceLabel", "String")
	requireStandardProperty(t, nbaRecommendation, "externalId", "String")
	requireStandardProperty(t, nbaRecommendation, "description", "String")
	requireStandardProperty(t, nbaRecommendation, "rejectionLabel", "String")
	requireStandardProperty(t, nbaRecommendation, "target", "Object")
	requireStandardProperty(t, nbaRecommendation, "targetAction", "Object")

	nativeRecommendation := requireStandardSymbol(t, symbols, "ConnectApi.NBANativeRecommendation")
	requireStandardProperty(t, nativeRecommendation, "name", "String")
	requireStandardProperty(t, nativeRecommendation, "url", "String")
	requireStandardProperty(t, nativeRecommendation, "id", "Id")

	flowAction := requireStandardSymbol(t, symbols, "ConnectApi.NBAFlowAction")
	requireStandardProperty(t, flowAction, "name", "String")
	requireStandardProperty(t, flowAction, "flowType", "ConnectApi.NBAFlowType")
	requireStandardProperty(t, flowAction, "parameters", "List<ConnectApi.NBAActionParameter>")

	param := requireStandardSymbol(t, symbols, "ConnectApi.NBAActionParameter")
	requireStandardProperty(t, param, "name", "String")
	requireStandardProperty(t, param, "value", "String")
	requireStandardProperty(t, param, "type", "String")
}
```

- [ ] **Step 2: Add failing sema tests**

Add to `sema_test.go`:

```go
func TestAnalyzePublicCorpusConnectApiDTOs(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesConnectApiDTOs.cls"), `
public class UsesConnectApiDTOs {
  public static void chatter() {
    ConnectApi.MessageBodyInput input = new ConnectApi.MessageBodyInput();
    input.messageSegments = new List<ConnectApi.MessageSegmentInput>();
    ConnectApi.TextSegmentInput text = new ConnectApi.TextSegmentInput();
    text.text = 'hello';
    input.messageSegments.add(text);
  }

  public static void nba(ConnectApi.NBARecommendation returnedRec) {
    NBARecommendation flowVisibleRec = new NBARecommendation();
    flowVisibleRec.acceptanceLabel = returnedRec.acceptanceLabel;
    flowVisibleRec.externalId = returnedRec.externalId;
    flowVisibleRec.description = returnedRec.description;
    flowVisibleRec.rejectionLabel = returnedRec.rejectionLabel;
    flowVisibleRec.name = ((ConnectApi.NBANativeRecommendation)returnedRec.target).name;
    flowVisibleRec.url = ((ConnectApi.NBANativeRecommendation)returnedRec.target).url;
    flowVisibleRec.Id = ((ConnectApi.NBANativeRecommendation)returnedRec.target).id;

    ConnectApi.NBAFlowAction action = (ConnectApi.NBAFlowAction)returnedRec.targetAction;
    FlowParameter param = new FlowParameter();
    param.name = action.parameters[0].name;
    param.value = action.parameters[0].value;
    param.type = action.parameters[0].type;
  }

  public class NBARecommendation {
    public String acceptanceLabel;
    public String externalId;
    public String description;
    public String rejectionLabel;
    public String name;
    public String url;
    public Id Id;
  }

  public class FlowParameter {
    public String name;
    public String value;
    public String type;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesConnectApiDTOs.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "messageSegments")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "flowVisibleRec")
}
```

- [ ] **Step 3: Run red**

```bash
go test ./internal/typesys -run TestStandardPlatformSymbolsIncludePublicCorpusConnectApiShapes -count=1
go test ./internal/sema -run TestAnalyzePublicCorpusConnectApiDTOs -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 4: Add docs contract entries**

Extend `testdata/generated/apex_docs_contracts.json` with entries in the existing file format for:

```json
{
  "type": "ConnectApi.MessageBodyInput",
  "properties": [
    {"name": "messageSegments", "type": "List<ConnectApi.MessageSegmentInput>"}
  ]
}
```

Add equivalent contract rows for:

```text
ConnectApi.TextSegmentInput.text String
ConnectApi.NBARecommendation.acceptanceLabel String
ConnectApi.NBARecommendation.externalId String
ConnectApi.NBARecommendation.description String
ConnectApi.NBARecommendation.rejectionLabel String
ConnectApi.NBARecommendation.target Object
ConnectApi.NBARecommendation.targetAction Object
ConnectApi.NBANativeRecommendation.name String
ConnectApi.NBANativeRecommendation.url String
ConnectApi.NBANativeRecommendation.id Id
ConnectApi.NBAFlowAction.name String
ConnectApi.NBAFlowAction.flowType ConnectApi.NBAFlowType
ConnectApi.NBAFlowAction.parameters List<ConnectApi.NBAActionParameter>
ConnectApi.NBAActionParameter.name String
ConnectApi.NBAActionParameter.value String
ConnectApi.NBAActionParameter.type String
```

- [ ] **Step 5: Regenerate standard symbols**

```bash
node scripts/generate-system-stub-symbols.mjs
```

Expected:

```text
generated internal/typesys/system_stub_symbols_generated.go
```

- [ ] **Step 6: Verify green**

```bash
go test ./internal/typesys -run TestStandardPlatformSymbolsIncludePublicCorpusConnectApiShapes -count=1
go test ./internal/sema -run TestAnalyzePublicCorpusConnectApiDTOs -count=1
```

Expected:

```text
ok
```

- [ ] **Step 7: Commit**

```bash
git add testdata/generated/apex_docs_contracts.json internal/typesys/system_stub_symbols_generated.go internal/typesys/standard_symbols_test.go internal/sema/sema_test.go
git commit -m "feat: add public ConnectApi DTO shapes"
```

---

### Task 4: Preserve Nested DTO Fields and Apex Array/List Aliases

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/type_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_calls.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_ir.go`

- [ ] **Step 1: Write failing test for Lightning Flow Request DTOs**

Add to `sema_test.go`:

```go
func TestAnalyzeNestedInvocableArrayFieldsBehaveAsLists(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "FilterCollectionLike.cls"), `
global class FilterCollectionLike {
  global class Requests {
    global Requests() {
      sourceAccountCollection = new List<Account>();
      sourceCaseCollection = new List<Case>();
    }

    @InvocableVariable
    global Account[] sourceAccountCollection;

    @InvocableVariable
    global Case[] sourceCaseCollection;
  }

  @IsTest
  static void run() {
    Requests request = new Requests();
    request.sourceAccountCollection.add(new Account(Name = 'A'));
    request.sourceCaseCollection.add(new Case(Subject = 'C'));
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "FilterCollectionLike.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}, {Name: "Case"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA021", "sourceAccountCollection")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "sourceAccountCollection.add")
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "add")
}
```

- [ ] **Step 2: Run red**

```bash
go test ./internal/sema -run TestAnalyzeNestedInvocableArrayFieldsBehaveAsLists -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Normalize Apex array spellings during member storage**

In `type_members.go`, update `semaCloneMemberSymbol` or the field insertion path so `Account[]` becomes `List<Account>` and `String[]` becomes `List<String>` before storage:

```go
func semaNormalizeApexArrayType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if !strings.HasSuffix(typeName, "[]") {
		return typeName
	}
	element := strings.TrimSpace(strings.TrimSuffix(typeName, "[]"))
	if element == "" {
		return typeName
	}
	return "List<" + element + ">"
}
```

Apply it for fields, properties, parameters, locals, and constructor assignments where types enter sema. Use existing `normalizeArrayType` only if it already produces the same `List<T>` form and has enough test coverage.

- [ ] **Step 4: Ensure nested class fields resolve through short and qualified names**

In `semaResolveFieldByKey`, preserve this lookup order:

```go
1. exact receiver type
2. qualified nested receiver type
3. short nested candidate only when unambiguous
4. superclass fields
5. SObject relationship fields
```

Add this guard to avoid replacing a project nested type with a same-name SObject:

```go
if members.kind == apexast.DeclarationClass && !members.sobject && !members.dependency {
	return semaResolveFieldFromMembers(model, members, fieldKey, seen)
}
```

- [ ] **Step 5: Verify green**

```bash
go test ./internal/sema -run TestAnalyzeNestedInvocableArrayFieldsBehaveAsLists -count=1
```

Expected:

```text
ok
```

- [ ] **Step 6: Commit**

```bash
git add internal/sema/sema_test.go internal/sema/type_members.go internal/sema/body_calls.go internal/sema/body_ir.go
git commit -m "fix: preserve nested DTO array fields"
```

---

### Task 5: Fix Collection Method Resolution for Generic Lists and Sets

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_calls.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_ir.go`

- [ ] **Step 1: Add failing collection tests**

Add to `sema_test.go`:

```go
func TestAnalyzePublicCollectionMethodsOnGenericReceivers(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCollectionMethods.cls"), `
public class UsesCollectionMethods {
  public void run() {
    List<ConnectApi.MessageSegmentInput> segments = new List<ConnectApi.MessageSegmentInput>();
    segments.add(new ConnectApi.TextSegmentInput());

    Set<String> values = new Set<String>();
    Boolean seen = values.contains('x');

    List<SObject> records = new List<SObject>();
    Boolean empty = records.isEmpty();
    records.add(new Account(Name = 'A'));
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCollectionMethods.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "add")
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "contains")
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "isEmpty")
}
```

- [ ] **Step 2: Run red**

```bash
go test ./internal/sema -run TestAnalyzePublicCollectionMethodsOnGenericReceivers -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Implement collection method compatibility**

In `body_calls.go`, make collection validation use generic element assignability instead of exact strings:

```go
func semaCollectionMethodCompatible(receiverType, method string, argTypes []string, model map[string]typeMembers) bool {
	base, args := semaGenericBaseAndArgs(normalizeArrayType(receiverType))
	base = semaCanonicalPlatformAlias(base)
	switch strings.ToLower(method) {
	case "add":
		if len(argTypes) != 1 || len(args) != 1 {
			return false
		}
		return semaAssignableToType(args[0], argTypes[0], model)
	case "addall":
		if len(argTypes) != 1 || len(args) != 1 {
			return false
		}
		argBase, argArgs := semaGenericBaseAndArgs(normalizeArrayType(argTypes[0]))
		if !strings.EqualFold(argBase, "List") && !strings.EqualFold(argBase, "Set") {
			return false
		}
		return len(argArgs) == 1 && semaAssignableToType(args[0], argArgs[0], model)
	case "contains":
		return len(argTypes) == 1
	case "isempty", "clear", "size":
		return len(argTypes) == 0
	default:
		return false
	}
}
```

Call this before emitting `GLADESEMA023`.

- [ ] **Step 4: Verify green**

```bash
go test ./internal/sema -run TestAnalyzePublicCollectionMethodsOnGenericReceivers -count=1
```

Expected:

```text
ok
```

- [ ] **Step 5: Commit**

```bash
git add internal/sema/sema_test.go internal/sema/body_calls.go internal/sema/body_ir.go
git commit -m "fix: resolve generic collection methods"
```

---

### Task 6: Track Fluent Builder Return Types Through Chained Calls

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_calls.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_ir.go`

- [ ] **Step 1: Add failing fluent builder test**

Add to `sema_test.go`:

```go
func TestAnalyzePublicFluentConfigureChain(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesFluentConfigure.cls"), `
public class UsesFluentConfigure {
  public class Module {
  }

  public class Binding {
    public Binding bind(Type value) { return this; }
    public Binding apex() { return this; }
    public Binding to(Type value) { return this; }
    public Binding data(Type value) { return this; }
  }

  public static Binding configure() {
    return new Binding();
  }

  public void run() {
    configure().bind(Account.class).apex().to(Module.class).data(Account.class);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesFluentConfigure.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "bind")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "apex")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "to")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "data")
}
```

- [ ] **Step 2: Run red**

```bash
go test ./internal/sema -run TestAnalyzePublicFluentConfigureChain -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Improve chained call inference**

In `body_calls.go`, extend `inferSemaMethodCallType` to walk receiver-call chains one segment at a time:

```go
func inferSemaChainedCallType(expr string, scope map[string]string, model map[string]typeMembers) string {
	receiver, method, args, ok := splitLastSemaCall(expr)
	if !ok {
		return ""
	}
	receiverType := inferSemaArgTypeWithModel(receiver, scope, model)
	if receiverType == "" {
		return ""
	}
	argTypes := make([]string, 0, len(args))
	for _, arg := range args {
		argTypes = append(argTypes, inferSemaArgTypeWithModel(arg.text, scope, model))
	}
	return semaResolvedTextCallReturnType(model, receiverType, method, argTypes, scope)
}
```

Use existing split helpers if they already preserve nested parentheses and generic angle brackets. The method must not split on dots inside casts, generics, strings, or nested calls.

- [ ] **Step 4: Keep nested type references resolvable**

When `Module.class` appears in an argument, ensure `inferSemaArgTypeWithModel` returns `Type`, not `Object`, and `resolveNestedTypeReference` can resolve `UsesFluentConfigure.Module`.

- [ ] **Step 5: Verify green**

```bash
go test ./internal/sema -run TestAnalyzePublicFluentConfigureChain -count=1
```

Expected:

```text
ok
```

- [ ] **Step 6: Commit**

```bash
git add internal/sema/sema_test.go internal/sema/body_calls.go internal/sema/body_ir.go
git commit -m "fix: infer fluent builder chains"
```

---

### Task 7: Handle Flow Metadata Generated Surfaces

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/project/project.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/project/project_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/type_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_ir.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`

- [ ] **Step 1: Add failing project extraction test**

Add to `project_test.go`:

```go
func TestLoadProjectExtractsFlowInterviewVariables(t *testing.T) {
	root := t.TempDir()
	flowDir := filepath.Join(root, "force-app/main/default/flows")
	if err := os.MkdirAll(flowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flowDir, "EmailToFlowController.flow-meta.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
  <variables>
    <name>AutoReplyMessage</name>
    <dataType>String</dataType>
    <isInput>true</isInput>
    <isOutput>true</isOutput>
  </variables>
</Flow>`), 0o644); err != nil {
		t.Fatal(err)
	}
	proj, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := proj.FlowVariables["EmailToFlowController"]["AutoReplyMessage"]; got != "String" {
		t.Fatalf("Flow variable type = %q, want String", got)
	}
}
```

- [ ] **Step 2: Add failing sema test**

Add to `sema_test.go`:

```go
func TestAnalyzeFlowInterviewMetadataVariables(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesFlowInterview.cls"), `
public class UsesFlowInterview {
  public void run() {
    Flow.Interview.EmailToFlowController interview = new Flow.Interview.EmailToFlowController(new Map<String,Object>());
    String message = interview.AutoReplyMessage;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesFlowInterview.cls")},
		FlowVariables: map[string]map[string]string{
			"EmailToFlowController": {"AutoReplyMessage": "String"},
		},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA021", "AutoReplyMessage")
}
```

- [ ] **Step 3: Run red**

```bash
go test ./internal/project -run TestLoadProjectExtractsFlowInterviewVariables -count=1
go test ./internal/sema -run TestAnalyzeFlowInterviewMetadataVariables -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 4: Add Flow variable model**

In `project.Project`, add:

```go
FlowVariables map[string]map[string]string
```

In project loading, parse `**/flows/*.flow-meta.xml`. Convert Salesforce Flow data types to Apex types:

```go
func apexTypeForFlowDataType(dataType string) string {
	switch strings.ToLower(strings.TrimSpace(dataType)) {
	case "string", "picklist", "multipicklist":
		return "String"
	case "boolean":
		return "Boolean"
	case "number", "currency", "percent":
		return "Decimal"
	case "date":
		return "Date"
	case "datetime":
		return "Datetime"
	case "sobject":
		return "SObject"
	default:
		return "Object"
	}
}
```

- [ ] **Step 5: Synthesize Flow.Interview fields**

In `type_members.go`, when building model members, create type members for:

```text
Flow.Interview.<FlowApiName>
```

Each variable becomes a field with the extracted Apex type. Preserve existing `Flow.Interview` methods.

- [ ] **Step 6: Verify green**

```bash
go test ./internal/project -run TestLoadProjectExtractsFlowInterviewVariables -count=1
go test ./internal/sema -run TestAnalyzeFlowInterviewMetadataVariables -count=1
```

Expected:

```text
ok
```

- [ ] **Step 7: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go internal/sema/type_members.go internal/sema/body_ir.go internal/sema/sema_test.go
git commit -m "feat: model Flow interview variables"
```

---

### Task 8: Fix Interface and Inheritance Contract Checks

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_checks.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/type_members.go`

- [ ] **Step 1: Add failing tests for inherited Object methods**

Add to `sema_test.go`:

```go
func TestAnalyzeInterfaceObjectMethodsAreSatisfiedByApexObject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesInterfaceObjectMethods.cls"), `
public class UsesInterfaceObjectMethods {
  public interface FieldFilter {
    Boolean equals(Object value);
    Integer hashCode();
    String toString();
  }

  public class FieldFilterImpl implements FieldFilter {
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesInterfaceObjectMethods.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA017", "equals")
	assertNoDiagnosticContaining(t, result, "GLADESEMA017", "hashCode")
	assertNoDiagnosticContaining(t, result, "GLADESEMA017", "toString")
}
```

- [ ] **Step 2: Add failing test for case-insensitive interface methods**

```go
func TestAnalyzeInterfaceMethodsMatchCaseInsensitively(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCaseInsensitiveInterface.cls"), `
public class UsesCaseInsensitiveInterface {
  public interface Drive {
    String DriveFilesList();
  }

  public class GoogleDrive implements Drive {
    public String driveFilesList() {
      return 'ok';
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCaseInsensitiveInterface.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA017", "DriveFilesList")
}
```

- [ ] **Step 3: Run red**

```bash
go test ./internal/sema -run 'TestAnalyzeInterfaceObjectMethodsAreSatisfiedByApexObject|TestAnalyzeInterfaceMethodsMatchCaseInsensitively' -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 4: Treat Apex Object methods as inherited**

In `sema_checks.go`, before emitting a missing interface method, treat these as satisfied for every class:

```go
func semaApexObjectMethodSatisfies(member typesys.MemberSymbol) bool {
	name := normalizeName(member.Name)
	switch name {
	case "equals":
		return len(member.Parameters) == 1 && sameSemaSignatureType(member.Parameters[0].Type, "Object")
	case "hashcode", "tostring", "clone":
		return len(member.Parameters) == 0
	default:
		return false
	}
}
```

Use it in `hasConcreteMethodSignature`.

- [ ] **Step 5: Make interface method matching case-insensitive**

Ensure all method lookup keys use `normalizeName(method.Name)`, and compare parameter and return types through `sameSemaSignatureType`.

- [ ] **Step 6: Verify green**

```bash
go test ./internal/sema -run 'TestAnalyzeInterfaceObjectMethodsAreSatisfiedByApexObject|TestAnalyzeInterfaceMethodsMatchCaseInsensitively' -count=1
```

Expected:

```text
ok
```

- [ ] **Step 7: Commit**

```bash
git add internal/sema/sema_test.go internal/sema/sema_checks.go internal/sema/type_members.go
git commit -m "fix: match Apex interface contracts"
```

---

### Task 9: Resolve Static and Instance Method Calls on Returned Receivers

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_calls.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_ir.go`

- [ ] **Step 1: Add failing tests for local static overloads and returned receivers**

Add to `sema_test.go`:

```go
func TestAnalyzeReturnedReceiverMethodCalls(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesReturnedReceivers.cls"), `
public class UsesReturnedReceivers {
  public class Selector {
    public List<SObject> selectSObjectsById(Set<Id> ids) {
      return new List<SObject>();
    }
  }

  public Selector selector() {
    return new Selector();
  }

  public List<SObject> run(Set<Id> ids) {
    return selector().selectSObjectsById(ids);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesReturnedReceivers.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "selectSObjectsById")
}
```

Add static overload fixture:

```go
func TestAnalyzeLocalStaticOverloadsWithTwoArgs(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStaticOverloads.cls"), `
public class UsesStaticOverloads {
  public class Matchers {
    public static Boolean matches(String value, String regex) {
      return Pattern.matches(regex, value);
    }
  }

  public Boolean run(String value) {
    return Matchers.matches(value, '^[a-z]+$');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStaticOverloads.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA009", "Matchers.matches")
}
```

- [ ] **Step 2: Run red**

```bash
go test ./internal/sema -run 'TestAnalyzeReturnedReceiverMethodCalls|TestAnalyzeLocalStaticOverloadsWithTwoArgs' -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Extend receiver return inference**

In `body_calls.go`, when the receiver expression is a method call, infer its return type before checking member methods:

```go
if receiverType == "" && strings.HasSuffix(strings.TrimSpace(receiverExpr), ")") {
	receiverType = inferSemaMethodCallType(receiverExpr, scope, model)
}
```

In `body_ir.go`, mirror this behavior for IR calls with `expr.Left`.

- [ ] **Step 4: Resolve local static overloads before platform fallback**

When a callee has a type receiver that exists in the project model, resolve `resolveMemberMethods(model, receiverType, method)` before falling back to platform aliases or bare text inference.

- [ ] **Step 5: Verify green**

```bash
go test ./internal/sema -run 'TestAnalyzeReturnedReceiverMethodCalls|TestAnalyzeLocalStaticOverloadsWithTwoArgs' -count=1
```

Expected:

```text
ok
```

- [ ] **Step 6: Commit**

```bash
git add internal/sema/sema_test.go internal/sema/body_calls.go internal/sema/body_ir.go
git commit -m "fix: infer returned method receivers"
```

---

### Task 10: Improve Dynamic Result and Object Assignment Narrowing

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_calls.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/body_ir.go`

- [ ] **Step 1: Add failing tests for Math constants and dynamic Flow variables**

Add to `sema_test.go`:

```go
func TestAnalyzeDynamicObjectAssignmentsWithTargetContext(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDynamicAssignments.cls"), `
public class UsesDynamicAssignments {
  public class FlowVisible {
    public String name;
    public Id externalId;
    public Decimal score;
  }

  public void fromMap(Map<String,Object> values) {
    FlowVisible out = new FlowVisible();
    out.name = (String)values.get('name');
    out.externalId = (Id)values.get('externalId');
    out.score = Math.PI;
  }

  public List<Location> queryLocations() {
    return Database.query('SELECT Id FROM Location');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDynamicAssignments.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Location"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "FlowVisible")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "List<Location>")
}
```

- [ ] **Step 2: Run red**

```bash
go test ./internal/sema -run TestAnalyzeDynamicObjectAssignmentsWithTargetContext -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Preserve explicit casts from dynamic sources**

Ensure `splitSemaCast` handling runs before method-call fallback but after binary inference. This should already exist from the private corpus work. Keep it covered by this task.

- [ ] **Step 4: Let Database dynamic query assign to typed SObject lists**

In assignment compatibility, allow `Database.QueryResult` to assign to `List<T>` when `T` is SObject-like:

```go
func semaDynamicQueryResultAssignableTo(targetType string, model map[string]typeMembers) bool {
	base, args := semaGenericBaseAndArgs(normalizeArrayType(targetType))
	if !strings.EqualFold(base, "List") || len(args) != 1 {
		return false
	}
	return isSemaSObjectLike(args[0], model)
}
```

Use it in `semaAssignableToType` and assignment diagnostics.

- [ ] **Step 5: Verify green**

```bash
go test ./internal/sema -run TestAnalyzeDynamicObjectAssignmentsWithTargetContext -count=1
```

Expected:

```text
ok
```

- [ ] **Step 6: Commit**

```bash
git add internal/sema/sema_test.go internal/sema/body_calls.go internal/sema/body_ir.go
git commit -m "fix: narrow dynamic assignment sources"
```

---

### Task 11: Add Standard Object and Metadata Object Field Overlays

**Files:**
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/query_semantics_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/type_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage/internal/sema/sema_checks.go`

- [ ] **Step 1: Add failing SOQL field tests**

Add to `query_semantics_test.go`:

```go
func TestAnalyzePublicCorpusStandardQueryFields(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPublicStandardFields.cls"), `
public class UsesPublicStandardFields {
  public void run() {
    List<Event> events = [SELECT Id, IsClosed FROM Event];
    List<Site> sites = [SELECT Id, OptionsRequireHttps FROM Site];
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesPublicStandardFields.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Event"}, {Name: "Site"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Event.IsClosed")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Site.OptionsRequireHttps")
}
```

- [ ] **Step 2: Run red**

```bash
go test ./internal/sema -run TestAnalyzePublicCorpusStandardQueryFields -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Add standard field overlays**

In `type_members.go`, extend standard SObject fallback fields:

```go
case "event":
	fields = append(fields, schema.Field{Name: "IsClosed", Type: "Checkbox"})
case "site":
	fields = append(fields, schema.Field{Name: "OptionsRequireHttps", Type: "Checkbox"})
```

If `ApprovalProcessStepDefinition__c` and `ASR_Survey_Log__c` are custom metadata from Flow packages, leave them metadata. Do not add custom objects to product overlays.

- [ ] **Step 4: Verify green**

```bash
go test ./internal/sema -run TestAnalyzePublicCorpusStandardQueryFields -count=1
```

Expected:

```text
ok
```

- [ ] **Step 5: Commit**

```bash
git add internal/sema/query_semantics_test.go internal/sema/type_members.go internal/sema/sema_checks.go
git commit -m "feat: add public standard query fields"
```

---

### Task 12: Corpus Ratchet and Final Closure

**Files:**
- Modify only files touched by preceding tasks.
- Read: `/tmp/glade-corpus-public-check-run/classified.tsv`
- Read: `/tmp/glade-corpus-public-check-run/diagnostics.tsv`

- [ ] **Step 1: Run focused tests**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
go test ./internal/sema -run 'TestAnalyzePublicCorpusPlatformStatics|TestAnalyzePublicCorpusConnectApiDTOs|TestAnalyzeNestedInvocableArrayFieldsBehaveAsLists|TestAnalyzePublicCollectionMethodsOnGenericReceivers|TestAnalyzePublicFluentConfigureChain|TestAnalyzeFlowInterviewMetadataVariables|TestAnalyzeInterfaceObjectMethodsAreSatisfiedByApexObject|TestAnalyzeInterfaceMethodsMatchCaseInsensitively|TestAnalyzeReturnedReceiverMethodCalls|TestAnalyzeLocalStaticOverloadsWithTwoArgs|TestAnalyzeDynamicObjectAssignmentsWithTargetContext|TestAnalyzePublicCorpusStandardQueryFields' -count=1
go test ./internal/typesys -run 'TestStandardPlatformSymbolsIncludePublicCorpusStatics|TestStandardPlatformSymbolsIncludePublicCorpusConnectApiShapes' -count=1
go test ./internal/project -run 'TestLoadProjectExtractsFlowInterviewVariables|TestNormalizeApexNamespaceTokens' -count=1
go test ./internal/typesys -run TestBuildParsesNamespaceTokenApex -count=1
```

Expected:

```text
ok
```

- [ ] **Step 2: Run broad product tests**

```bash
go test ./internal/sema ./internal/project ./internal/typesys -count=1
```

Expected:

```text
ok
```

- [ ] **Step 3: Build Glade**

```bash
go build -o /tmp/glade-sema-coverage ./cmd/glade
```

Expected: no output and exit code `0`.

- [ ] **Step 4: Run public corpus ratchet**

```bash
cd /Users/matt/Dev/glade-tools
rm -rf /tmp/glade-corpus-public-check-run
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/public \
  --glade /tmp/glade-sema-coverage \
  --out /tmp/glade-corpus-public-check-run
```

Expected shape:

```text
corpus check: projects=308 diagnostics=<lower> unclassified=0 out=/tmp/glade-corpus-public-check-run
```

- [ ] **Step 5: Confirm no disallowed check classes remain**

Run:

```bash
sed -n '1,80p' /tmp/glade-corpus-public-check-run/classified.tsv
awk -F '\t' 'NR>1 && $2!="performance-advisory" && $2!="project-metadata-missing" {print}' /tmp/glade-corpus-public-check-run/diagnostics.tsv | head -40
```

Expected:

```text
classified.tsv has no semantic-contract-gap, source-parse-error, project-discovery-duplicate, or unclassified row
the awk command prints no diagnostics
```

If disallowed rows remain, sort them by class, code, and repeated message and add a new task before final verification:

```bash
awk -F '\t' 'NR>1 && $2!="performance-advisory" && $2!="project-metadata-missing" {key=$2"\t"$3"\t"$9; count[key]++} END {for (k in count) print count[k]"\t"k}' /tmp/glade-corpus-public-check-run/diagnostics.tsv | sort -rn | head -40
```

For allowed metadata rows, generate evidence and inspect the top rows:

```bash
awk -F '\t' 'NR>1 && $2=="project-metadata-missing" {key=$3"\t"$9; count[key]++} END {for (k in count) print count[k]"\t"k}' /tmp/glade-corpus-public-check-run/diagnostics.tsv | sort -rn | head -80
```

Every retained metadata pattern must name a missing package source, missing generated metadata source, missing custom object, missing custom field, or missing Flow variable metadata. If a row is merely an unsupported Salesforce behavior, add another product task and fix it.

- [ ] **Step 6: Re-check public anchor projects**

```bash
/tmp/glade-sema-coverage check --project /Users/matt/Dev/glade-corpus/public/apex-rollup --no-progress
/tmp/glade-sema-coverage check --project /Users/matt/Dev/glade-corpus/public/EDA --no-progress
/tmp/glade-sema-coverage check --project /Users/matt/Dev/glade-corpus/public/LightningFlowComponents --no-progress
```

Expected:

```text
apex-rollup: no diagnostics
EDA: only performance warnings, unless a row is backed by missing project metadata evidence
LightningFlowComponents: no parse, duplicate, or semantic-contract diagnostics; only performance warnings and proven metadata rows may remain
```

- [ ] **Step 7: Run hygiene**

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
git diff --check
cd /Users/matt/Dev/glade-tools
git diff --check
```

Expected: no output.

- [ ] **Step 8: Commit final ratchet notes**

If all tasks already committed code slices, make one final documentation commit only if a plan or ratchet note changed:

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
git add docs/superpowers/plans/2026-06-22-public-corpus-sema-closure.md
git commit -m "docs: plan public corpus sema closure"
```

## Self-Review

Spec coverage:

- Public `LightningFlowComponents` product rows are covered by Tasks 3, 4, 5, 7, 8, 9, 10, and 11.
- Public `at4dx` fluent and fflib-adjacent rows are covered by Tasks 1, 6, 8, and 9.
- Public `Apex-Opensource-Library` interface, constructor, static access, and local overload rows are covered by Tasks 8, 9, and 10.
- Public `PostRichChatter` and `automation-components` ConnectApi collection rows are covered by Tasks 3 and 5.
- Public duplicate rows are removed by Task 0.5, namespace-template parse rows are closed by Task 0.75, metadata rows are proven by Tasks 0 and 1, and performance advisories are the only routine check warnings left.

Placeholder scan:

- No task uses open-ended implementation text. Each task has concrete files, test code, commands, and expected outputs.

Type consistency:

- Tests use existing Glade helper names: `writeSemaFile`, `typesys.Build`, `Analyze`, `assertNoDiagnosticContaining`.
- New helper names are consistent within tasks: `semaNormalizeApexArrayType`, `semaCollectionMethodCompatible`, `semaApexObjectMethodSatisfies`, `apexTypeForFlowDataType`.
