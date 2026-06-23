# Private Corpus Sema Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove every private-corpus finding that represents a Glade semantic, symbol, indexing, or classifier gap, while leaving real metadata, duplicate-layout, and performance findings visible.

**Architecture:** Use small Apex fixtures in `internal/sema` and `internal/project` to reproduce each product-shaped failure. Fix general Salesforce and Apex behavior in the analyzer, type system, and project indexer. Use `glade-tools corpus check` only as evidence and classification, never as a source of project-specific exceptions.

**Tech Stack:** Go semantic analyzer and type system, generated Salesforce standard symbols, `glade-tools` corpus classifier, public-style Apex fixtures, private corpus verification with `/tmp/glade-sema-coverage`.

---

## Current Evidence

Fresh private corpus check:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/private \
  --glade /tmp/glade-sema-coverage \
  --out /tmp/glade-corpus-private-check-run
```

Observed output:

```text
corpus check: projects=9 diagnostics=209 unclassified=0 out=/tmp/glade-corpus-private-check-run
```

Buckets:

```text
semantic-contract-gap       184
performance-advisory          9
docs-contract-mismatch        8
project-discovery-duplicate   3
project-metadata-missing      3
generated-shape-gap           2
```

Product-shaped findings to address:

- Method calls on string literals: 54 `hashCode` return mismatches.
- `String.split` return type: 2 `List<String>` assignment failures.
- Static string field token handling: `String.isNotBlank(SomeType.FieldName)`, `SomeType.FieldName.toLowerCase()`, and collection `add(SomeType.FieldName)` failures.
- Static `@TestVisible` map field chained access: `Owner.contentMap.get(key)` failures.
- Fluent builder overload and nested return preservation: `Q.condition(...).isEqualTo(...)` and `.add(...)` failures.
- Namespaced/local source discovery where the referenced Apex source is present but not indexed.
- Classifier bucket precision where missing metadata rows land under `docs-contract-mismatch` or `generated-shape-gap`.

Findings to keep unless separate project/schema evidence proves a product issue:

- Missing managed/custom object metadata such as unknown `__c` object types and unknown relationship paths.
- Duplicate top-level symbols caused by project file layout.
- Performance advisories for SOQL or DML in loops.
- Override and `super(...)` errors caused only by absent base classes.

## File Map

Product repo:

- Modify: `internal/sema/sema_test.go` - add focused regression fixtures for each product-shaped private finding.
- Modify: `internal/sema/body_calls.go` - repair text-call receiver inference and method dispatch ordering.
- Modify: `internal/sema/body_ir.go` - repair compiled IR receiver and return-type inference for the same expressions.
- Modify: `internal/sema/sema.go` - shared helpers for literal receiver calls, static field path resolution, and collection signatures.
- Modify: `internal/sema/type_members.go` - project type member indexing for nested/namespaced source types when needed.
- Modify: `internal/project/project.go` - source-root discovery only if present source files are currently missed.
- Modify: `internal/project/project_test.go` - project discovery regression only if Task 6 proves a missed source-root issue.
- Modify: `internal/typesys/standard_symbols_test.go` - guard standard symbol method return types if symbol merge is implicated.
- Modify: `internal/typesys/standard_symbols.go` or `internal/typesys/system_stub_symbols_generated.go` only if a standard symbol is truly wrong.

Tools repo:

- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go` - tighten classification for missing metadata versus product gaps.
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go` - add classifier fixtures matching the current private report shapes.

## Task 1: Lock The Private Finding Inventory

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go`

- [ ] **Step 1: Add a classifier fixture for current product-shaped messages**

Add this test table to `corpuscheck_test.go` near the existing classifier tests:

```go
func TestClassifyPrivateCorpusProductShapedDiagnostics(t *testing.T) {
	tests := []struct {
		name string
		diag Diagnostic
		want string
	}{
		{
			name: "string literal method return mismatch stays semantic",
			diag: Diagnostic{Code: "GLADESEMA019", Message: `method "hashCode" has invalid return: returns String from Integer method`},
			want: "semantic-contract-gap",
		},
		{
			name: "string split assignment stays semantic",
			diag: Diagnostic{Code: "GLADESEMA018", Message: `method "run" initializes List<String> local "parts" with String`},
			want: "semantic-contract-gap",
		},
		{
			name: "static field fluent string call stays semantic",
			diag: Diagnostic{Code: "GLADESEMA008", Message: `method "run" calls unknown method "FieldNames.State.toLowerCase"`},
			want: "semantic-contract-gap",
		},
		{
			name: "builder overload miss stays semantic",
			diag: Diagnostic{Code: "GLADESEMA009", Message: `method "run" has no matching overload for call "Q.condition" with 1 argument(s)`},
			want: "semantic-contract-gap",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyDiagnostic(tt.diag); got != tt.want {
				t.Fatalf("classifyDiagnostic() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Add a classifier fixture for metadata/project-specific messages**

Add this test in the same file:

```go
func TestClassifyPrivateCorpusMetadataDiagnostics(t *testing.T) {
	tests := []struct {
		name string
		diag Diagnostic
		want string
	}{
		{
			name: "unknown custom object type is metadata",
			diag: Diagnostic{Code: "GLADESEMA002", Message: `field "record" references unknown type "Package__Order__c"`},
			want: "project-metadata-missing",
		},
		{
			name: "unknown namespaced package type is metadata until source is present",
			diag: Diagnostic{Code: "GLADESEMA004", Message: `method "run" parameter "line" references unknown type "pkg.OrderLine"`},
			want: "project-metadata-missing",
		},
		{
			name: "unknown relationship path is metadata",
			diag: Diagnostic{Code: "GLADESEMA_QUERY_RELATIONSHIP", Message: `SOQL query references unknown relationship path "Parent__r.Name" on Child__c`},
			want: "project-metadata-missing",
		},
		{
			name: "duplicate symbol is project discovery duplicate",
			diag: Diagnostic{Code: "GLADETYPE001", Message: `duplicate top-level symbol "DuplicateType"; first seen in /repo/first.cls`},
			want: "project-discovery-duplicate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyDiagnostic(tt.diag); got != tt.want {
				t.Fatalf("classifyDiagnostic() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run the classifier tests and capture the failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpuscheck -run 'TestClassifyPrivateCorpus' -count=1
```

Expected before implementation: at least one metadata case fails if it is still classified as `docs-contract-mismatch` or `generated-shape-gap`.

- [ ] **Step 4: Tighten classifier rules**

In `corpuscheck.go`, update `classifyDiagnostic` so metadata wins before docs/generated buckets:

```go
func classifyDiagnostic(diag Diagnostic) string {
	message := strings.ToLower(diag.Message)
	switch diag.Code {
	case "GLADETYPE001":
		return "project-discovery-duplicate"
	case "APEXPARSE001":
		return "source-parse-error"
	case "GLADEPERF001", "GLADEPERF002":
		return "performance-advisory"
	}
	if diagnosticLooksLikeMissingMetadata(diag.Code, message) {
		return "project-metadata-missing"
	}
	if diagnosticLooksLikeDocsContractMismatch(diag.Code, message) {
		return "docs-contract-mismatch"
	}
	if diagnosticLooksLikeGeneratedShapeGap(diag.Code, message) {
		return "generated-shape-gap"
	}
	if strings.HasPrefix(diag.Code, "GLADESEMA") {
		return "semantic-contract-gap"
	}
	return "unclassified"
}

func diagnosticLooksLikeMissingMetadata(code, message string) bool {
	if code == "GLADESEMA_QUERY_RELATIONSHIP" {
		return true
	}
	if !strings.Contains(message, "unknown type") && !strings.Contains(message, "unknown field") {
		return false
	}
	return strings.Contains(message, "__c") ||
		strings.Contains(message, "__mdt") ||
		strings.Contains(message, "__e") ||
		strings.Contains(message, " type \"pkg.") ||
		strings.Contains(message, " type \"package.") ||
		strings.Contains(message, " type \"ns.")
}
```

Use the local classifier helper names already present in the file if they differ. Keep the rule general; do not name private packages or projects.

- [ ] **Step 5: Run the classifier tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpuscheck -run 'TestClassifyPrivateCorpus|TestCorpusCheck' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the classifier boundary**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/corpuscheck/corpuscheck.go internal/corpuscheck/corpuscheck_test.go
git commit -m "test: classify private corpus finding shapes"
```

## Task 2: Fix String Literal Receiver Calls And `String.split`

**Files:**
- Modify: `internal/sema/sema_test.go`
- Modify: `internal/sema/body_calls.go`
- Modify: `internal/sema/body_ir.go`
- Modify: `internal/sema/sema.go`

- [ ] **Step 1: Add a failing regression test**

Add this test to `internal/sema/sema_test.go`:

```go
func TestAnalyzeStringLiteralMethodAndSplitTypes(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStringLiterals.cls"), `
public class UsesStringLiterals {
  public void run() {
    Integer code = 'Physical'.hashCode();
    List<String> parts = 'a,b,c'.split(',');
    List<String> noMatch = 'hello'.split(',');
    Boolean blank = String.isNotBlank('value');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStringLiterals.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}
```

- [ ] **Step 2: Run the test and verify the current failure**

Run:

```bash
go test ./internal/sema -run TestAnalyzeStringLiteralMethodAndSplitTypes -count=1
```

Expected before implementation: failure showing `hashCode` return type or `String.split` assignment type.

- [ ] **Step 3: Add a shared literal receiver type helper**

Add this helper in `internal/sema/sema.go` near other expression helpers:

```go
func semaLiteralReceiverType(expr string) string {
	expr = strings.TrimSpace(expr)
	if len(expr) >= 2 && strings.HasPrefix(expr, "'") {
		return "String"
	}
	if decimalLiteralPattern.MatchString(expr) {
		return "Decimal"
	}
	if intLiteralPattern.MatchString(expr) {
		return "Integer"
	}
	return semaKeywordLiteralType(expr)
}
```

- [ ] **Step 4: Use the helper in text call receiver inference**

In `internal/sema/body_calls.go`, before falling back to unknown dotted receiver handling in `checkBodyCalls`, ensure `splitSemaMethodPath(callee)` can resolve literal receivers:

```go
if receiverExpr, method, ok := splitSemaMethodPath(callee); ok {
	if receiverType := semaLiteralReceiverType(receiverExpr); receiverType != "" {
		if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "instance"); handled {
			diagnostics = append(diagnostics, platformDiagnostics...)
			continue
		}
	}
}
```

Place this before the existing `inferSemaFieldAccessType(receiverExpr, scope, model)` branch so `'text'.hashCode()` reaches the standard String method table.

- [ ] **Step 5: Use the helper in IR expression type inference**

In `internal/sema/body_ir.go`, inside `inferIRExprType` for `ir.ExprCall`, add literal receiver inference for calls whose callee is a method path:

```go
if receiverExpr, method, ok := splitSemaMethodPath(expr.Callee); ok {
	if receiverType := semaLiteralReceiverType(receiverExpr); receiverType != "" {
		if typ := semaPlatformMethodReturnType(receiverType, method, expr.Args, scope.flat(), model); typ != "" {
			return typ
		}
	}
}
```

If the local helper for platform return inference has a different name, use that helper. Do not add a new parallel method resolver.

- [ ] **Step 6: Run the focused test**

Run:

```bash
go test ./internal/sema -run TestAnalyzeStringLiteralMethodAndSplitTypes -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/sema/sema.go internal/sema/body_calls.go internal/sema/body_ir.go internal/sema/sema_test.go
git commit -m "fix: infer literal string method calls"
```

## Task 3: Preserve Static String Field Types Over SObject Token Guesses

**Files:**
- Modify: `internal/sema/sema_test.go`
- Modify: `internal/sema/body_calls.go`
- Modify: `internal/sema/body_ir.go`

- [ ] **Step 1: Add a regression test for static field strings**

Add:

```go
func TestAnalyzeStaticStringFieldTokensStayStrings(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order {
  public static final String StateFieldName = 'State__c';
  public static final String CartFieldName = 'Cart__c';
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesOrderFieldNames.cls"), `
public class UsesOrderFieldNames {
  public void run() {
    String state = Order.StateFieldName.toLowerCase();
    Boolean present = String.isNotBlank(Order.CartFieldName);
    Set<String> fieldsToQuery = new Set<String>();
    fieldsToQuery.add(Order.StateFieldName);
    fieldsToQuery.add(Order.CartFieldName.toLowerCase());
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Order.cls"),
			filepath.Join(root, "UsesOrderFieldNames.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}
```

- [ ] **Step 2: Run the focused test**

Run:

```bash
go test ./internal/sema -run TestAnalyzeStaticStringFieldTokensStayStrings -count=1
```

Expected before implementation: failure around unknown `toLowerCase`, invalid `add`, or `isNotBlank`.

- [ ] **Step 3: Reuse the guarded static member lookup before standard object token logic**

In both `inferSemaFieldAccessType` and `inferIRExprType`, keep this ordering:

```go
if target, staticOK := semaStaticClassFieldPathMemberInContext(model, scope[semaCurrentTypeScopeKey], parts[0], strings.Join(parts[1:], ".")); staticOK && !hasModifier(target.member.Modifiers, semaSyntheticStandardSObjectFieldModifier) {
	if owner, ok := model[normalizeName(target.owner)]; !ok || !owner.sobject {
		return target.member.Type
	}
}
```

Then run the standard object token logic. This keeps project `Order.StateFieldName` as `String` while preserving real `Event.SObjectType` and `Event.ActivityDateTime`.

- [ ] **Step 4: Make static field method-path receivers use inferred receiver type**

In `checkBodyCalls`, for `Order.StateFieldName.toLowerCase()`, ensure `receiverExpr` is `Order.StateFieldName` and the inferred receiver type `String` flows into `checkSemaPlatformCall`.

Use this shape:

```go
if receiverType := inferSemaFieldAccessType(receiverExpr, scope, model); receiverType != "" {
	receiverMode := "instance"
	if semaTextReceiverExprLooksLikeType(receiverExpr, scope, model) {
		receiverMode = "class"
	}
	if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, receiverMode); handled {
		diagnostics = append(diagnostics, platformDiagnostics...)
		continue
	}
}
```

Keep `receiverMode` as `instance` for field paths. If the helper currently treats any capitalized root as a type, add a guard so `Order.StateFieldName` is not classified as a class receiver after the field path resolves.

- [ ] **Step 5: Run Event and static-field tests together**

Run:

```bash
go test ./internal/sema -run 'TestAnalyzeStaticStringFieldTokensStayStrings|TestAnalyzeEventSObjectTokensPreferStandardSObject|TestAnalyzeNestedEnumOverloadDeclaredLater' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sema/body_calls.go internal/sema/body_ir.go internal/sema/sema_test.go
git commit -m "fix: preserve static string field receiver types"
```

## Task 4: Preserve Static `@TestVisible` Map Field Types Through Chained Calls

**Files:**
- Modify: `internal/sema/sema_test.go`
- Modify: `internal/sema/body_calls.go`
- Modify: `internal/sema/body_ir.go`

- [ ] **Step 1: Add a regression test**

Add:

```go
func TestAnalyzeTestVisibleStaticMapFieldChainedGet(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "EmailContent.cls"), `
public class EmailContent {
  @TestVisible private static Map<String, String> contentMap = new Map<String, String>();
  public static void addContent(String key, String value) {
    contentMap.put(key, value);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "EmailContentTest.cls"), `
@IsTest
private class EmailContentTest {
  private static final String TEST_KEY = 'key';
  private static final String TEST_VALUE = 'value';
  @IsTest
  private static void addContent_validKey_expectStored() {
    EmailContent.addContent(TEST_KEY, TEST_VALUE);
    System.assertEquals(TEST_VALUE, EmailContent.contentMap.get(TEST_KEY));
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "EmailContent.cls"),
			filepath.Join(root, "EmailContentTest.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}
```

- [ ] **Step 2: Run the test**

Run:

```bash
go test ./internal/sema -run TestAnalyzeTestVisibleStaticMapFieldChainedGet -count=1
```

Expected before implementation: unknown method on `EmailContent.contentMap.get` or blocked access.

- [ ] **Step 3: Preserve static field type for chained receiver expressions**

In `inferSemaFieldAccessType`, verify `EmailContent.contentMap` returns `Map<String,String>`. If it does not, add a branch before collection or platform method handling:

```go
if target, staticOK := semaStaticClassFieldPathMemberInContext(model, scope[semaCurrentTypeScopeKey], parts[0], strings.Join(parts[1:], ".")); staticOK {
	if accessOK := semaMemberAccessibleForInference(scope, target, model); accessOK {
		return target.member.Type
	}
}
```

Implement `semaMemberAccessibleForInference` only if existing access checks cannot be reused. It must allow `@TestVisible private` members from `@IsTest` types and keep normal private blocking in diagnostics.

- [ ] **Step 4: Make collection method dispatch accept inferred map receiver types**

Ensure `checkSemaCollectionCall` handles `Map<String,String>.get(String)` when the receiver type came from `inferSemaFieldAccessType`.

If the existing collection signature table lacks `Map.get`, add:

```go
case "map":
	switch normalizeName(method) {
	case "get":
		return semaCollectionSignature{returnType: valueType, params: [][]string{{keyType}}}, true
	case "put":
		return semaCollectionSignature{returnType: valueType, params: [][]string{{keyType, valueType}}}, true
	case "containskey":
		return semaCollectionSignature{returnType: "Boolean", params: [][]string{{keyType}}}, true
	}
```

Use existing `semaCollectionSignature` fields and helper names.

- [ ] **Step 5: Run focused and related tests**

Run:

```bash
go test ./internal/sema -run 'TestAnalyzeTestVisibleStaticMapFieldChainedGet|TestAnalyzeTestVisibleMethodAccess|TestAnalyzeMapConstructorAcceptsChildRelationshipList' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sema/body_calls.go internal/sema/body_ir.go internal/sema/sema_test.go
git commit -m "fix: infer test-visible static map field chains"
```

## Task 5: Fix Fluent Builder Overloads And Collection Add Arguments

**Files:**
- Modify: `internal/sema/sema_test.go`
- Modify: `internal/sema/body_calls.go`
- Modify: `internal/sema/body_ir.go`
- Modify: `internal/sema/sema.go`

- [ ] **Step 1: Add a public-style fluent builder fixture**

Add:

```go
func TestAnalyzeFluentBuilderNestedStaticConditionCalls(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Q.cls"), `
public class Q {
  public Q(Schema.SObjectType objectType) {}
  public Q selectFields(Set<String> fields) { return this; }
  public Q add(Condition condition) { return this; }
  public static Condition condition(String fieldName) { return new Condition(fieldName); }
  public class Condition {
    public Condition(String fieldName) {}
    public Condition isEqualTo(Object value) { return this; }
    public Condition isGreaterThan(Decimal value) { return this; }
    public Condition isNotIn(String bindExpression) { return this; }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order {
  public static final String StateFieldName = 'State__c';
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesQ.cls"), `
public class UsesQ {
  public void run(Id entityId, Set<Id> recordIds) {
    Set<String> fieldsToQuery = new Set<String>();
    fieldsToQuery.add('CurrencyIsoCode');
    fieldsToQuery.add(Order.StateFieldName);
    Q query = new Q(Account.SObjectType)
      .selectFields(fieldsToQuery)
      .add(Q.condition('Entity__c').isEqualTo(entityId))
      .add(Q.condition('Balance__c').isGreaterThan(0))
      .add(Q.condition('Id').isNotIn(':recordIds'));
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Q.cls"),
			filepath.Join(root, "Order.cls"),
			filepath.Join(root, "UsesQ.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}
```

- [ ] **Step 2: Run the test**

Run:

```bash
go test ./internal/sema -run TestAnalyzeFluentBuilderNestedStaticConditionCalls -count=1
```

Expected before implementation: overload miss for `Q.condition`, invalid `add`, or unknown nested condition method.

- [ ] **Step 3: Preserve nested static method return type through text call checking**

In `checkBodyCalls`, when a receiver expression is itself a call such as `Q.condition('Entity__c')`, infer its return type before diagnosing the chained call:

```go
if receiverType := inferSemaArgTypeWithModel(receiverExpr, scope, model); receiverType != "" {
	if platformDiagnostics, handled := checkSemaPlatformCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model, "instance"); handled {
		diagnostics = append(diagnostics, platformDiagnostics...)
		continue
	}
	if collectionDiagnostics, handled := checkSemaCollectionCall(typ, member, receiverType, method, args, bodyOffset+match[2], bodyOffset+match[3], source, scope, model); handled {
		diagnostics = append(diagnostics, collectionDiagnostics...)
		continue
	}
	candidates := resolveMemberMethods(model, receiverType, method)
	diagnostics = append(diagnostics, a.diagnoseMethodCall(typ, member, method, candidates, args, haveArgs, "instance", bodyOffset+match[2], bodyOffset+match[3], source, scope, model)...)
	continue
}
```

Place it only where `receiverExpr` is not a simple scoped variable and after literal/static field receiver handling.

- [ ] **Step 4: Make `inferSemaArgTypeWithModel` resolve static method calls**

Add or extend logic so `Q.condition('Entity__c')` returns `Q.Condition`:

```go
if receiver, method, callArgs, ok := splitSemaStaticCall(arg); ok {
	if classMembers, lookupName, ok := semaClassMembersForReceiver(model, typesys.TypeSymbol{Name: scope[semaCurrentTypeScopeKey]}, receiver); ok {
		_ = classMembers
		argTypes := semaArgTypes(callArgs, scope, model)
		if candidate, matched, _ := bestResolvedMemberByArgTypes(resolveMemberMethods(model, lookupName, method), argTypes, model); matched {
			return candidate.member.Type
		}
	}
}
```

If a parser helper already returns receiver, method, and args, use it. Avoid ad hoc string splitting that breaks nested calls.

- [ ] **Step 5: Run related tests**

Run:

```bash
go test ./internal/sema -run 'TestAnalyzeFluentBuilderNestedStaticConditionCalls|TestAnalyzeStaticStringFieldTokensStayStrings|TestAnalyzeMapConstructorAcceptsChildRelationshipList' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sema/body_calls.go internal/sema/body_ir.go internal/sema/sema.go internal/sema/sema_test.go
git commit -m "fix: infer fluent builder call chains"
```

## Task 6: Separate Present Source Discovery Gaps From Missing Metadata

**Files:**
- Modify: `internal/project/project_test.go`
- Modify: `internal/project/project.go`
- Modify: `internal/sema/sema_test.go`
- Modify: `internal/sema/type_members.go`

- [ ] **Step 1: Prove whether missing namespaced types exist on disk**

Run:

```bash
find /Users/matt/Dev/glade-corpus/private -name 'OrderLine.cls' -o -name 'AgreementSetter.cls' -o -name 'IItemSObjectSupport.cls'
```

Expected:

- If no matching files exist under the checked project roots, keep those diagnostics under `project-metadata-missing`.
- If matching files exist under a package directory that Glade skipped, continue with this task.

- [ ] **Step 2: Add a project discovery regression only for present source**

If Step 1 finds present source, add this test to `internal/project/project_test.go`:

```go
func TestLoadProjectIncludesNestedPackageDirectorySources(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, filepath.Join(root, "sfdx-project.json"), `{
  "packageDirectories": [
    {"path": "core", "default": true},
    {"path": "feature"}
  ],
  "namespace": "pkg"
}`)
	writeProjectFile(t, filepath.Join(root, "core", "main", "default", "classes", "Base.cls"), `public virtual class Base {}`)
	writeProjectFile(t, filepath.Join(root, "feature", "main", "default", "classes", "Child.cls"), `public class Child extends pkg.Base {}`)
	proj, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !containsPathSuffix(proj.ApexFiles, filepath.Join("core", "main", "default", "classes", "Base.cls")) {
		t.Fatalf("missing core source: %#v", proj.ApexFiles)
	}
	if !containsPathSuffix(proj.ApexFiles, filepath.Join("feature", "main", "default", "classes", "Child.cls")) {
		t.Fatalf("missing feature source: %#v", proj.ApexFiles)
	}
}
```

Use existing test helpers if they already exist. Add `containsPathSuffix` only if no equivalent helper exists.

- [ ] **Step 3: Run the project test**

Run:

```bash
go test ./internal/project -run TestLoadProjectIncludesNestedPackageDirectorySources -count=1
```

Expected before implementation: FAIL only if current discovery misses a source root.

- [ ] **Step 4: Fix project source root expansion**

In `internal/project/project.go`, ensure every `packageDirectories[].path` is scanned for Apex classes and triggers. Preserve current exclusions for generated/build folders.

The logic should follow this shape:

```go
for _, dir := range projectFile.PackageDirectories {
	root := filepath.Join(projectRoot, filepath.Clean(dir.Path))
	if !regularDir(root) {
		continue
	}
	apexFiles = append(apexFiles, discoverApexFiles(root)...)
}
```

Do not add private project path names. Do not infer fields or classes from names.

- [ ] **Step 5: Add a namespaced type resolution regression if source is present**

Add to `internal/sema/sema_test.go`:

```go
func TestAnalyzeNamespacedSourceTypeReferenceAcrossPackageDirectories(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `public virtual class Base { public virtual void apply() {} }`)
	writeSemaFile(t, filepath.Join(root, "UsesPkgBase.cls"), `
public class UsesPkgBase extends pkg.Base {
  public override void apply() {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "pkg",
		ApexFiles: []string{filepath.Join(root, "Base.cls"), filepath.Join(root, "UsesPkgBase.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}
```

- [ ] **Step 6: Run project and sema tests**

Run:

```bash
go test ./internal/project -run TestLoadProjectIncludesNestedPackageDirectorySources -count=1
go test ./internal/sema -run TestAnalyzeNamespacedSourceTypeReferenceAcrossPackageDirectories -count=1
```

Expected: PASS if source discovery is a real product gap. If Step 1 proved the source is absent, skip code changes and keep the diagnostics classified as metadata.

- [ ] **Step 7: Commit if code changed**

```bash
git add internal/project/project.go internal/project/project_test.go internal/sema/sema_test.go internal/sema/type_members.go
git commit -m "fix: include present package directory source types"
```

## Task 7: Guard Standard Symbol Merge For Core Object/String Methods

**Files:**
- Modify: `internal/typesys/standard_symbols_test.go`
- Modify: `internal/typesys/standard_symbols.go` only if the test fails

- [ ] **Step 1: Add a merge precedence regression**

Add:

```go
func TestStandardPlatformSymbolsKeepCoreStringMethodTypes(t *testing.T) {
	symbols := typesys.StandardPlatformSymbols()
	stringType := findStandardType(t, symbols, "String")
	requireStandardMethodType(t, stringType, "hashCode", "Integer")
	requireStandardMethodType(t, stringType, "split", "List<String>")
	requireStandardMethodType(t, stringType, "toLowerCase", "String")
	requireStandardMethodType(t, stringType, "isNotBlank", "Boolean")
}
```

Use the existing helper names in `standard_symbols_test.go`. If the file is already in package `typesys`, omit the `typesys.` qualifier.

- [ ] **Step 2: Run the test**

Run:

```bash
go test ./internal/typesys -run TestStandardPlatformSymbolsKeepCoreStringMethodTypes -count=1
```

Expected: PASS. If it fails, fix standard symbol merge so concrete generated return types beat `Object`, empty, or hand-written incomplete rows.

- [ ] **Step 3: Commit if code changed**

```bash
git add internal/typesys/standard_symbols.go internal/typesys/standard_symbols_test.go
git commit -m "test: guard core string symbol return types"
```

## Task 8: Final Private Corpus Ratchet

**Files:**
- Modify: none unless previous tasks reveal a missing focused test.

- [ ] **Step 1: Run focused product tests**

Run:

```bash
go test ./internal/sema -run 'TestAnalyzeStringLiteralMethodAndSplitTypes|TestAnalyzeStaticStringFieldTokensStayStrings|TestAnalyzeTestVisibleStaticMapFieldChainedGet|TestAnalyzeFluentBuilderNestedStaticConditionCalls|TestAnalyzeEventSObjectTokensPreferStandardSObject|TestAnalyzeNestedEnumOverloadDeclaredLater' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run broader product tests**

Run:

```bash
go test ./internal/sema ./internal/project ./internal/typesys -count=1
go build -o /tmp/glade-sema-coverage ./cmd/glade
```

Expected: PASS.

- [ ] **Step 3: Run glade-tools tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpuscheck ./internal/toolcli -count=1
```

Expected: PASS.

- [ ] **Step 4: Re-run private corpus check**

Run:

```bash
cd /Users/matt/Dev/glade-tools
rm -rf /tmp/glade-corpus-private-check-run
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/private \
  --glade /tmp/glade-sema-coverage \
  --out /tmp/glade-corpus-private-check-run
```

Expected:

```text
unclassified=0
```

Expected product-shaped reductions:

- `hashCode` return mismatch count drops to 0.
- `String.split` assignment mismatch count drops to 0.
- `Order.*FieldName.toLowerCase` and `String.isNotBlank(Order.*FieldName)` unknown/collection errors drop to 0.
- Static `@TestVisible` map `.get` unknown-method errors drop to 0.
- `Q.condition` overload and builder `.add` false positives drop to 0.

Expected remaining findings:

- Performance advisories remain.
- Duplicate top-level symbol findings remain.
- Missing managed/custom object metadata remains.
- Unknown relationship paths remain until schema metadata is supplied.
- Override/super errors remain only when the base class is absent from source and metadata.

- [ ] **Step 5: Print the top remaining findings**

Run:

```bash
awk -F'\t' 'NR>1 {print $2}' /tmp/glade-corpus-private-check-run/diagnostics.tsv | sort | uniq -c | sort -nr
awk -F'\t' 'NR>1 {print $3 "\t" $9}' /tmp/glade-corpus-private-check-run/diagnostics.tsv | sort | uniq -c | sort -nr | sed -n '1,40p'
```

Expected: the top table contains only metadata, duplicate-layout, or performance rows plus any newly discovered real product gap with a focused test added before completion.

- [ ] **Step 6: Run whitespace checks**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/codex-salesforce-sema-coverage
git diff --check
cd /Users/matt/Dev/glade-tools
git diff --check
```

Expected: no output and exit code 0.

## Completion Criteria

The work is complete when:

- All focused product tests pass.
- `go test ./internal/sema ./internal/project ./internal/typesys -count=1` passes.
- `go test ./internal/corpuscheck ./internal/toolcli -count=1` passes in `glade-tools`.
- Private corpus check has `unclassified=0`.
- Private corpus no longer reports the product-shaped groups named in Task 8.
- Remaining private findings are explicitly one of:
  - project metadata missing,
  - project discovery duplicate,
  - performance advisory,
  - source parse error,
  - a newly documented real product gap with a focused failing test and follow-up plan.
