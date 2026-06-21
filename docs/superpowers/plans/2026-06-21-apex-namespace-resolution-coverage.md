# Apex Namespace Resolution Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cover every missing Apex namespace and type-resolution area exposed by the `System.RestRequest` miss, and add generated gates so future Salesforce docs symbols cannot slip through.

**Architecture:** Build one semantic name resolver backed by the standard platform symbol specs, then route known-type checks, assignability, call resolution, member lookup, and text/IR inference through it. Add generated semantic tests for every documented `System` and `Schema` spelling rule, then make the first-party compatibility reports and docs claim that rule only when the gate passes.

**Tech Stack:** Go, `internal/typesys`, `internal/sema`, sibling `glade-tools` capability and surface-ledger packages, local Salesforce docs scrape at `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run`.

---

## Scope

This plan covers all missing areas found in this audit:

- Every documented `System.X` spelling for top-level System namespace types.
- Every documented short `X` spelling for `Schema.X` types.
- Generic type arguments that contain `System.X` or short `Schema` names.
- Method parameters, return types, fields, locals, enhanced-for locals, classic-for locals, catch locals, constructors, `new`, casts, assignments, returns, overload matching, static members, enum values, and receiver/member paths.
- Apex name precedence from the Salesforce docs: local variable, class, then namespace for expressions.
- Apex type precedence from the Salesforce docs: scalar/local/project type, class, then system/sObject type.
- `T1.T2` ambiguity: inner type before namespace-qualified type.
- Custom class shadowing: unqualified `Database` can mean a project class, while `System.Database` must mean the platform class.
- SObject disambiguation: unqualified `Account` can mean a project class, while `Schema.Account` must mean the sObject.
- Compatibility and public docs claims that currently say the supported surface is broader than the compiler gate proves.

This plan does not implement every unrelated Salesforce runtime API gap in `SURFACE_GAPS.md`. Those rows already exist as API-shape/runtime backlog. This plan makes namespace and type-resolution support complete and measured.

## File Structure

- Modify `internal/typesys/standard_symbols.go`: expose namespace alias metadata from standard symbol specs.
- Create `internal/typesys/standard_namespace_aliases_test.go`: prove generated `System` and `Schema` alias inventory.
- Create `internal/sema/platform_names.go`: central semantic platform name resolver.
- Create `internal/sema/platform_names_test.go`: direct resolver tests.
- Modify `internal/sema/body_calls.go`: remove resolver logic from this file and call the central resolver.
- Modify `internal/sema/type_members.go`: route nested and generic type references through the central resolver.
- Modify `internal/sema/sema.go`: route known-type checks through the central resolver.
- Modify `internal/sema/body_ir.go` and `internal/sema/sema_checks.go`: ensure legacy text sema and IR sema use the same resolver for all body contexts.
- Create `internal/sema/namespace_resolution_generated_test.go`: generated semantic tests over all documented namespace aliases.
- Create `internal/sema/namespace_precedence_test.go`: focused precedence and shadowing tests from Salesforce docs.
- Modify `/Users/matt/Dev/glade-tools/internal/capability/capability.go` and related tests: add an explicit namespace-resolution capability row.
- Modify `/Users/matt/Dev/glade-tools/internal/surfaceledger`: add surface rows or evidence markers for the Apex language namespace rules.
- Regenerate `docs/COMPATIBILITY_DASHBOARD.md`, `docs/STDLIB_COVERAGE.md`, `docs/KNOWN_GAPS.md`, and site support data from `glade-tools`.

---

### Task 1: Add failing inventory tests for all documented aliases

**Files:**
- Create: `internal/typesys/standard_namespace_aliases_test.go`
- Create: `internal/sema/platform_names_test.go`
- Test: `internal/typesys/standard_namespace_aliases_test.go`
- Test: `internal/sema/platform_names_test.go`

- [ ] **Step 1: Write the failing typesys inventory test**

Create `internal/typesys/standard_namespace_aliases_test.go`:

```go
package typesys

import "testing"

func TestStandardSystemNamespaceTypeNamesIncludeGeneratedSystemDocs(t *testing.T) {
	names := StandardSystemNamespaceTypeNames()
	required := []string{
		"Blob",
		"Boolean",
		"Database",
		"Date",
		"Datetime",
		"Decimal",
		"Exception",
		"HttpRequest",
		"HttpResponse",
		"JSON",
		"Limits",
		"Math",
		"RestContext",
		"RestRequest",
		"RestResponse",
		"String",
		"System",
		"URL",
	}
	for _, name := range required {
		if !containsStringFold(names, name) {
			t.Fatalf("StandardSystemNamespaceTypeNames missing %q in %v", name, names)
		}
	}
	if len(names) < 223 {
		t.Fatalf("StandardSystemNamespaceTypeNames count = %d, want at least 223", len(names))
	}
}

func TestStandardSchemaNamespaceTypeNamesIncludeGeneratedSchemaDocs(t *testing.T) {
	names := StandardSchemaNamespaceTypeNames()
	required := []string{
		"ChildRelationship",
		"DataCategory",
		"DataCategoryGroupSobjectTypePair",
		"DescribeColorResult",
		"DescribeDataCategoryGroupResult",
		"DescribeDataCategoryGroupStructureResult",
		"DescribeFieldResult",
		"DescribeIconResult",
		"DescribeSObjectResult",
		"DisplayType",
		"FieldDescribeOptions",
		"FieldSet",
		"FieldSetMember",
		"FilteredLookupInfo",
		"PicklistEntry",
		"RecordTypeInfo",
		"SObjectDescribeOptions",
		"SObjectField",
		"SObjectType",
		"SObjectTypeFields",
		"SObjectTypeFieldSets",
		"SoapType",
	}
	for _, name := range required {
		if !containsStringFold(names, name) {
			t.Fatalf("StandardSchemaNamespaceTypeNames missing %q in %v", name, names)
		}
	}
	if len(names) < len(required) {
		t.Fatalf("StandardSchemaNamespaceTypeNames count = %d, want at least %d", len(names), len(required))
	}
}

func containsStringFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Add the missing import**

Update the import block in the new file:

```go
import (
	"strings"
	"testing"
)
```

- [ ] **Step 3: Write the failing sema resolver test**

Create `internal/sema/platform_names_test.go`:

```go
package sema

import "testing"

func TestSemaCanonicalPlatformAliasCoversDocumentedSystemNamespace(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"System.Blob", "Blob"},
		{"System.Boolean", "Boolean"},
		{"System.Database", "Database"},
		{"System.Date", "Date"},
		{"System.Datetime", "Datetime"},
		{"System.Decimal", "Decimal"},
		{"System.Exception", "Exception"},
		{"System.HttpRequest", "HttpRequest"},
		{"System.HttpResponse", "HttpResponse"},
		{"System.JSON", "JSON"},
		{"System.Limits", "Limits"},
		{"System.Math", "Math"},
		{"System.RestContext", "RestContext"},
		{"System.RestRequest", "RestRequest"},
		{"System.RestResponse", "RestResponse"},
		{"System.String", "String"},
		{"System.System", "System"},
		{"System.URL", "URL"},
		{"List<System.RestRequest>", "List<RestRequest>"},
		{"Map<String,System.HttpResponse>", "Map<String,HttpResponse>"},
	} {
		if got := semaCanonicalPlatformAlias(tc.in); got != tc.want {
			t.Fatalf("semaCanonicalPlatformAlias(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSemaCanonicalPlatformAliasCoversDocumentedSchemaImplicitImports(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"ChildRelationship", "Schema.ChildRelationship"},
		{"DataCategory", "Schema.DataCategory"},
		{"DataCategoryGroupSobjectTypePair", "Schema.DataCategoryGroupSobjectTypePair"},
		{"DescribeColorResult", "Schema.DescribeColorResult"},
		{"DescribeDataCategoryGroupResult", "Schema.DescribeDataCategoryGroupResult"},
		{"DescribeDataCategoryGroupStructureResult", "Schema.DescribeDataCategoryGroupStructureResult"},
		{"DescribeFieldResult", "Schema.DescribeFieldResult"},
		{"DescribeIconResult", "Schema.DescribeIconResult"},
		{"DescribeSObjectResult", "Schema.DescribeSObjectResult"},
		{"DisplayType", "Schema.DisplayType"},
		{"FieldDescribeOptions", "Schema.FieldDescribeOptions"},
		{"FieldSet", "Schema.FieldSet"},
		{"FieldSetMember", "Schema.FieldSetMember"},
		{"FilteredLookupInfo", "Schema.FilteredLookupInfo"},
		{"PicklistEntry", "Schema.PicklistEntry"},
		{"RecordTypeInfo", "Schema.RecordTypeInfo"},
		{"SObjectDescribeOptions", "Schema.SObjectDescribeOptions"},
		{"SObjectField", "Schema.SObjectField"},
		{"SObjectType", "Schema.SObjectType"},
		{"SObjectTypeFields", "Schema.SObjectTypeFields"},
		{"SObjectTypeFieldSets", "Schema.SObjectTypeFieldSets"},
		{"SoapType", "Schema.SoapType"},
		{"List<FieldDescribeOptions>", "List<Schema.FieldDescribeOptions>"},
	} {
		if got := semaCanonicalPlatformAlias(tc.in); got != tc.want {
			t.Fatalf("semaCanonicalPlatformAlias(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
```

- [ ] **Step 4: Run the tests and verify they fail**

Run:

```bash
go test ./internal/typesys ./internal/sema -run 'TestStandard(System|Schema)NamespaceTypeNames|TestSemaCanonicalPlatformAliasCovers'
```

Expected: fail because `StandardSystemNamespaceTypeNames`, `StandardSchemaNamespaceTypeNames`, and broad alias support do not exist yet.

- [ ] **Step 5: Commit the failing tests**

```bash
git add internal/typesys/standard_namespace_aliases_test.go internal/sema/platform_names_test.go
git commit -m "test: cover Apex namespace alias inventory"
```

---

### Task 2: Expose generated namespace alias metadata from typesys

**Files:**
- Modify: `internal/typesys/standard_symbols.go`
- Test: `internal/typesys/standard_namespace_aliases_test.go`

- [ ] **Step 1: Add namespace helper functions**

Add this code after `StandardPlatformSymbolView` in `internal/typesys/standard_symbols.go`:

```go
var (
	standardSystemNamespaceTypeNamesOnce  sync.Once
	standardSystemNamespaceTypeNamesCache []string
	standardSchemaNamespaceTypeNamesOnce  sync.Once
	standardSchemaNamespaceTypeNamesCache []string
)

func StandardSystemNamespaceTypeNames() []string {
	standardSystemNamespaceTypeNamesOnce.Do(func() {
		standardSystemNamespaceTypeNamesCache = buildStandardSystemNamespaceTypeNames()
	})
	return append([]string(nil), standardSystemNamespaceTypeNamesCache...)
}

func StandardSchemaNamespaceTypeNames() []string {
	standardSchemaNamespaceTypeNamesOnce.Do(func() {
		standardSchemaNamespaceTypeNamesCache = buildStandardSchemaNamespaceTypeNames()
	})
	return append([]string(nil), standardSchemaNamespaceTypeNamesCache...)
}

func buildStandardSystemNamespaceTypeNames() []string {
	seen := map[string]string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || strings.Contains(name, ".") {
			return
		}
		seen[strings.ToLower(name)] = name
	}
	for _, specs := range [][]StandardSymbolSpec{
		standardPlatformSymbolSpecs,
		systemStubSymbolSpecs,
		standardPlatformSymbolOverlays,
	} {
		for _, spec := range specs {
			add(spec.Name)
		}
	}
	return sortedNamespaceNames(seen)
}

func buildStandardSchemaNamespaceTypeNames() []string {
	seen := map[string]string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		const prefix = "Schema."
		if !strings.HasPrefix(name, prefix) {
			return
		}
		short := strings.TrimPrefix(name, prefix)
		if short == "" || strings.Contains(short, ".") {
			return
		}
		seen[strings.ToLower(short)] = short
	}
	for _, specs := range [][]StandardSymbolSpec{
		standardPlatformSymbolSpecs,
		systemStubSymbolSpecs,
		standardPlatformSymbolOverlays,
	} {
		for _, spec := range specs {
			add(spec.Name)
		}
	}
	return sortedNamespaceNames(seen)
}

func sortedNamespaceNames(names map[string]string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}
```

- [ ] **Step 2: Run the inventory tests**

Run:

```bash
go test ./internal/typesys -run 'TestStandard(System|Schema)NamespaceTypeNames'
```

Expected: pass. If it fails because a documented symbol count differs, inspect `internal/typesys/system_stub_symbols_generated.go` and update only the expected lower bound when the docs scrape has changed.

- [ ] **Step 3: Commit the metadata helper**

```bash
git add internal/typesys/standard_symbols.go internal/typesys/standard_namespace_aliases_test.go
git commit -m "feat: expose standard namespace alias inventory"
```

---

### Task 3: Move platform name resolution out of body_calls

**Files:**
- Create: `internal/sema/platform_names.go`
- Modify: `internal/sema/body_calls.go`
- Test: `internal/sema/platform_names_test.go`

- [ ] **Step 1: Create the central resolver**

Create `internal/sema/platform_names.go`:

```go
package sema

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/typesys"
)

var (
	semaPlatformAliasOnce sync.Once
	semaPlatformAliasMap  map[string]string
)

func semaCanonicalPlatformAlias(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) > 0 {
		canonicalArgs := make([]string, len(args))
		for i, arg := range args {
			canonicalArgs[i] = semaCanonicalPlatformAlias(arg)
		}
		return semaCanonicalPlatformAlias(base) + "<" + strings.Join(canonicalArgs, ",") + ">"
	}
	aliasMap := semaPlatformAliases()
	if canonical, ok := aliasMap[normalizeName(typeName)]; ok {
		return canonical
	}
	return typeName
}

func semaPlatformAliases() map[string]string {
	semaPlatformAliasOnce.Do(func() {
		aliases := map[string]string{}
		for _, name := range typesys.StandardSystemNamespaceTypeNames() {
			aliases[normalizeName("System."+name)] = name
		}
		for _, name := range typesys.StandardSchemaNamespaceTypeNames() {
			aliases[normalizeName(name)] = "Schema." + name
			aliases[normalizeName("System."+name)] = "Schema." + name
		}
		aliases[normalizeName("ApexPages.PageReference")] = "PageReference"
		aliases[normalizeName("APEX_OBJECT")] = "Object"
		aliases[normalizeName("System.APEX_OBJECT")] = "Object"
		semaPlatformAliasMap = aliases
	})
	return semaPlatformAliasMap
}
```

- [ ] **Step 2: Remove the old resolver from `body_calls.go`**

Delete the existing `semaCanonicalPlatformAlias` function from `internal/sema/body_calls.go`. Leave `semaShortTypeKey` and `semaShortTypeKeyFromNormalizedKey` in place.

- [ ] **Step 3: Run the direct resolver tests**

Run:

```bash
go test ./internal/sema -run TestSemaCanonicalPlatformAliasCovers
```

Expected: pass.

- [ ] **Step 4: Run current sema tests**

Run:

```bash
go test ./internal/sema
```

Expected: pass. Any failure here means an old hand-written alias had a behavior not captured in the generated map. Add that behavior to `semaPlatformAliases` only when backed by docs or current tests.

- [ ] **Step 5: Commit the resolver extraction**

```bash
git add internal/sema/platform_names.go internal/sema/body_calls.go internal/sema/platform_names_test.go
git commit -m "feat: centralize Apex platform name aliases"
```

---

### Task 4: Route known-type checks through the resolver

**Files:**
- Modify: `internal/sema/sema.go`
- Modify: `internal/sema/body_ir.go`
- Modify: `internal/sema/sema_checks.go`
- Test: `internal/sema/namespace_resolution_generated_test.go`

- [ ] **Step 1: Add failing known-type tests**

Create `internal/sema/namespace_resolution_generated_test.go` with these starter tests:

```go
package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeAllowsSystemQualifiedTypesEverywhere(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SystemQualifiedEverywhere.cls"), `
public class SystemQualifiedEverywhere {
  private System.RestRequest reqField;
  public System.RestResponse returnsResponse(System.RestRequest req) {
    System.RestRequest localReq = req;
    List<System.RestRequest> requestList = new List<System.RestRequest>();
    Object obj = req;
    System.RestRequest castReq = (System.RestRequest)obj;
    return new System.RestResponse();
  }
}
`)
	result := Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "SystemQualifiedEverywhere.cls")},
	}, schema.Schema{}))
	if result.HasErrors() {
		t.Fatalf("unexpected System-qualified diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsSchemaImplicitTypesEverywhere(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SchemaImplicitEverywhere.cls"), `
public class SchemaImplicitEverywhere {
  private DisplayType displayTypeField;
  public DisplayType returnsDisplayType(FieldDescribeOptions opts) {
    DisplayType localDisplayType = DisplayType.STRING;
    List<FieldDescribeOptions> options = new List<FieldDescribeOptions>();
    Object obj = localDisplayType;
    DisplayType castDisplayType = (DisplayType)obj;
    return localDisplayType;
  }
}
`)
	result := Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "SchemaImplicitEverywhere.cls")},
	}, schema.Schema{}))
	if result.HasErrors() {
		t.Fatalf("unexpected Schema implicit diagnostics: %#v", result.Diagnostics)
	}
}
```

- [ ] **Step 2: Update `hasKnown`**

In `internal/sema/sema.go`, change `hasKnown` so it checks canonical aliases before managed-package fallback:

```go
func (a *Analyzer) hasKnown(name string) bool {
	if name == "" {
		return true
	}
	if _, ok := a.known[normalizeName(name)]; ok {
		return true
	}
	canonical := semaCanonicalPlatformAlias(name)
	if !strings.EqualFold(canonical, name) {
		if _, ok := a.known[normalizeName(canonical)]; ok {
			return true
		}
	}
	if a.hasExternalDependencyName(name) || semaLooksLikeUnconfiguredManagedPackageType(name) {
		return true
	}
	if a.namespace != "" {
		if namespaced, ok := semaProjectNamespacedAPIName(a.namespace, name); ok {
			if _, ok := a.known[normalizeName(namespaced)]; ok {
				return true
			}
		}
		ns := normalizeName(a.namespace)
		prefix := ns + "."
		normalized := normalizeName(name)
		if strings.HasPrefix(normalized, prefix) {
			if _, ok := a.known[strings.TrimPrefix(normalized, prefix)]; ok {
				return true
			}
		}
		metadataPrefix := ns + "__"
		if strings.HasPrefix(normalized, metadataPrefix) {
			if _, ok := a.known[strings.TrimPrefix(normalized, metadataPrefix)]; ok {
				return true
			}
		}
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		_, ok := a.known[normalizeName(parts[0])]
		return ok
	}
	return false
}
```

- [ ] **Step 3: Update body type reference checks**

In `internal/sema/body_ir.go` and `internal/sema/sema_checks.go`, wrap type references before `a.hasKnown(ref)` checks:

```go
resolvedRef := resolveNestedTypeReference(model, typ.Name, ref)
if !a.hasKnown(resolvedRef) {
	// keep the existing diagnostic shape and range
}
```

Apply this pattern to constructor refs, local declaration refs, enhanced-for refs, classic-for refs, catch refs, assignment target refs, return refs, and ternary refs.

- [ ] **Step 4: Run the focused tests**

Run:

```bash
go test ./internal/sema -run 'TestAnalyzeAllows(SystemQualifiedTypesEverywhere|SchemaImplicitTypesEverywhere)'
```

Expected: pass.

- [ ] **Step 5: Commit known-type resolver routing**

```bash
git add internal/sema/sema.go internal/sema/body_ir.go internal/sema/sema_checks.go internal/sema/namespace_resolution_generated_test.go
git commit -m "fix: resolve platform aliases in type checks"
```

---

### Task 5: Generate semantic coverage across every System and Schema symbol

**Files:**
- Modify: `internal/sema/namespace_resolution_generated_test.go`
- Test: `internal/sema/namespace_resolution_generated_test.go`

- [ ] **Step 1: Add generated System alias coverage**

Append this test to `internal/sema/namespace_resolution_generated_test.go`:

```go
func TestAnalyzeAllowsEveryDocumentedSystemQualifiedTypeSpelling(t *testing.T) {
	for _, name := range typesys.StandardSystemNamespaceTypeNames() {
		t.Run(name, func(t *testing.T) {
			source := `
public class UsesSystemQualified {
  private System.` + name + ` fieldValue;
  public System.` + name + ` roundTrip(System.` + name + ` input) {
    System.` + name + ` localValue = input;
    Object obj = localValue;
    System.` + name + ` castValue = (System.` + name + `)obj;
    List<System.` + name + `> values = new List<System.` + name + `>();
    return castValue;
  }
}
`
			result := analyzeSingleGeneratedClass(t, "UsesSystemQualified.cls", source)
			if result.HasErrors() {
				t.Fatalf("unexpected diagnostics for System.%s: %#v", name, result.Diagnostics)
			}
		})
	}
}

func analyzeSingleGeneratedClass(t *testing.T, fileName, source string) Result {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, fileName)
	writeSemaFile(t, path, source)
	return Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{path},
	}, schema.Schema{}))
}
```

- [ ] **Step 2: Add generated Schema implicit coverage**

Append this test:

```go
func TestAnalyzeAllowsEveryDocumentedSchemaImplicitTypeSpelling(t *testing.T) {
	for _, name := range typesys.StandardSchemaNamespaceTypeNames() {
		t.Run(name, func(t *testing.T) {
			source := `
public class UsesSchemaImplicit {
  private ` + name + ` fieldValue;
  public ` + name + ` roundTrip(` + name + ` input) {
    ` + name + ` localValue = input;
    Object obj = localValue;
    ` + name + ` castValue = (` + name + `)obj;
    List<` + name + `> values = new List<` + name + `>();
    return castValue;
  }
}
`
			result := analyzeSingleGeneratedClass(t, "UsesSchemaImplicit.cls", source)
			if result.HasErrors() {
				t.Fatalf("unexpected diagnostics for Schema implicit %s: %#v", name, result.Diagnostics)
			}
		})
	}
}
```

- [ ] **Step 3: Skip impossible source shapes with explicit reasons**

If a generated type is an enum or interface that cannot be constructed or assigned in the generic test body, do not delete it from coverage. Add a helper that chooses a shape from symbol metadata:

```go
func namespaceResolutionSourceForType(className, typeName string, constructable bool) string {
	if constructable {
		return `
public class ` + className + ` {
  private ` + typeName + ` fieldValue;
  public ` + typeName + ` roundTrip(` + typeName + ` input) {
    ` + typeName + ` localValue = input;
    Object obj = localValue;
    ` + typeName + ` castValue = (` + typeName + `)obj;
    List<` + typeName + `> values = new List<` + typeName + `>();
    return castValue;
  }
}
`
	}
	return `
public class ` + className + ` {
  private ` + typeName + ` fieldValue;
  public ` + typeName + ` roundTrip(` + typeName + ` input) {
    ` + typeName + ` localValue = input;
    Object obj = localValue;
    ` + typeName + ` castValue = (` + typeName + `)obj;
    List<` + typeName + `> values = new List<` + typeName + `>();
    return castValue;
  }
}
`
}
```

Use the same non-constructor shape for classes, interfaces, and enums. Do not call `new TypeName()` in this generated test.

- [ ] **Step 4: Run the full generated namespace tests**

Run:

```bash
go test ./internal/sema -run 'TestAnalyzeAllowsEveryDocumented(SystemQualified|SchemaImplicit)TypeSpelling'
```

Expected: pass for every generated symbol. Any skipped symbol must have a named `t.Skipf` reason that points to invalid Apex syntax, not missing Glade support.

- [ ] **Step 5: Commit generated sema coverage**

```bash
git add internal/sema/namespace_resolution_generated_test.go
git commit -m "test: cover every documented Apex namespace spelling"
```

---

### Task 6: Cover Salesforce name precedence and shadowing

**Files:**
- Create: `internal/sema/namespace_precedence_test.go`
- Modify: `internal/sema/type_members.go`
- Modify: `internal/sema/body_ir.go`
- Modify: `internal/sema/body_calls.go`
- Test: `internal/sema/namespace_precedence_test.go`

- [ ] **Step 1: Add the Salesforce expression precedence tests**

Create `internal/sema/namespace_precedence_test.go`:

```go
package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeExpressionPrecedenceLocalThenClassThenNamespace(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Database.cls"), `
public class Database {
  public String query(String soql) {
    return 'local';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesPrecedence.cls"), `
public class UsesPrecedence {
  public void run() {
    Database Database = new Database();
    String value = Database.query('SELECT Name FROM Account');
  }
}
`)
	result := analyzeFiles(t, root, "Database.cls", "UsesPrecedence.cls")
	if result.HasErrors() {
		t.Fatalf("unexpected local-variable precedence diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemQualifierDisambiguatesShadowedDatabase(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Database.cls"), `
public class Database {
  public static String query() {
    return 'local';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesSystemDatabase.cls"), `
public class UsesSystemDatabase {
  public void run() {
    List<SObject> rows = System.Database.query('SELECT Name FROM Account');
  }
}
`)
	result := analyzeFiles(t, root, "Database.cls", "UsesSystemDatabase.cls")
	if result.HasErrors() {
		t.Fatalf("unexpected System.Database diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaQualifierDisambiguatesShadowedSObject(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Account.cls"), `
public class Account {
  public Integer myInteger;
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesSchemaAccount.cls"), `
public class UsesSchemaAccount {
  public void run() {
    Schema.Account myAccountSObject = new Schema.Account();
    Account accountClassInstance = new Account();
    accountClassInstance.myInteger = 1;
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Account.cls"),
			filepath.Join(root, "UsesSchemaAccount.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Schema.Account diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInnerTypeWinsBeforeNamespaceType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public class Inner {
    public String localValue;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "A.cls"), `
public class A {
  public class B {
    public String value;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesInnerPrecedence.cls"), `
public class UsesInnerPrecedence {
  public class A {
    public class B {
      public Integer value;
    }
  }
  public void run(Object obj) {
    Integer value = ((A.B)obj).value;
  }
}
`)
	result := analyzeFiles(t, root, "Outer.cls", "A.cls", "UsesInnerPrecedence.cls")
	if result.HasErrors() {
		t.Fatalf("unexpected inner-before-namespace diagnostics: %#v", result.Diagnostics)
	}
}

func analyzeFiles(t *testing.T, root string, files ...string) Result {
	t.Helper()
	apexFiles := make([]string, 0, len(files))
	for _, file := range files {
		apexFiles = append(apexFiles, filepath.Join(root, file))
	}
	return Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: apexFiles,
	}, schema.Schema{}))
}
```

- [ ] **Step 2: Update type resolution helper**

In `internal/sema/type_members.go`, change `resolveNestedTypeName` to try canonical platform aliases only after owner-local and project names fail:

```go
func resolveNestedTypeName(model map[string]typeMembers, owner, typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if strings.Contains(typeName, ".") {
		if owner != "" {
			ownerCandidate := owner + "." + typeName
			if _, ok := model[normalizeName(ownerCandidate)]; ok {
				return ownerCandidate
			}
		}
		if _, ok := model[normalizeName(typeName)]; ok {
			return typeName
		}
		canonical := semaCanonicalPlatformAlias(typeName)
		if !strings.EqualFold(canonical, typeName) {
			return canonical
		}
		return typeName
	}
	ownerParts := strings.Split(owner, ".")
	if len(ownerParts) > 0 && strings.EqualFold(ownerParts[0], typeName) {
		return typeName
	}
	if owner != "" {
		candidate := owner + "." + typeName
		if _, ok := model[normalizeName(candidate)]; ok {
			return candidate
		}
	}
	for i := len(ownerParts) - 1; i > 0; i-- {
		candidate := strings.Join(append(append([]string{}, ownerParts[:i]...), typeName), ".")
		if _, ok := model[normalizeName(candidate)]; ok {
			return candidate
		}
	}
	if _, ok := model[normalizeName(typeName)]; ok {
		return typeName
	}
	seen := make(map[string]bool)
	for current := owner; current != ""; {
		key := normalizeName(current)
		if key == "" || seen[key] {
			break
		}
		seen[key] = true
		members, ok := model[key]
		if !ok || strings.TrimSpace(members.superClass) == "" {
			break
		}
		candidate := members.superClass + "." + typeName
		if _, ok := model[normalizeName(candidate)]; ok {
			return candidate
		}
		current = members.superClass
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	if !strings.EqualFold(canonical, typeName) {
		return canonical
	}
	return typeName
}
```

- [ ] **Step 3: Run precedence tests**

Run:

```bash
go test ./internal/sema -run 'TestAnalyze(ExpressionPrecedence|SystemQualifier|SchemaQualifier|InnerTypeWins)'
```

Expected: pass.

- [ ] **Step 4: Commit precedence support**

```bash
git add internal/sema/namespace_precedence_test.go internal/sema/type_members.go
git commit -m "fix: match Apex namespace precedence"
```

---

### Task 7: Cover static members, enum values, overloads, assignments, and returns

**Files:**
- Modify: `internal/sema/namespace_resolution_generated_test.go`
- Modify: `internal/sema/body_ir.go`
- Modify: `internal/sema/body_calls.go`
- Modify: `internal/sema/sema_checks.go`
- Test: `internal/sema/namespace_resolution_generated_test.go`

- [ ] **Step 1: Add static member and enum tests**

Append these tests:

```go
func TestAnalyzeSystemQualifiedStaticMembersAndMethods(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSystemStatics.cls"), `
public class UsesSystemStatics {
  public void run() {
    System.debug(System.URL.getCurrentRequestUrl());
    System.HttpRequest request = new System.HttpRequest();
    System.HttpResponse response = new System.HttpResponse();
    System.JSON.serialize(request);
  }
}
`)
	result := Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSystemStatics.cls")},
	}, schema.Schema{}))
	if result.HasErrors() {
		t.Fatalf("unexpected System static diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaImplicitEnumValues(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSchemaEnums.cls"), `
public class UsesSchemaEnums {
  public void run() {
    DisplayType displayType = DisplayType.STRING;
    FieldDescribeOptions opts = FieldDescribeOptions.FULL_DESCRIBE;
    SObjectDescribeOptions sopts = SObjectDescribeOptions.DEFERRED;
  }
}
`)
	result := Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSchemaEnums.cls")},
	}, schema.Schema{}))
	if result.HasErrors() {
		t.Fatalf("unexpected Schema enum diagnostics: %#v", result.Diagnostics)
	}
}
```

- [ ] **Step 2: Normalize receiver and parameter types in call matching**

In `internal/sema/body_calls.go`, keep all existing call resolution logic but ensure these local values pass through `semaCanonicalPlatformAlias`:

```go
receiverType = semaCanonicalPlatformAlias(receiverType)
argTypes[i] = semaCanonicalPlatformAlias(resolveNestedTypeReference(model, typ.Name, argTypes[i]))
paramType = semaCanonicalPlatformAlias(resolveNestedTypeReference(model, typ.Name, paramType))
```

Apply this to platform call checking, project method checking, constructor overload checking, `semaAssignableToType`, `semaConversionCost`, and `semaCommonType`.

- [ ] **Step 3: Normalize assignment and return types**

In `internal/sema/sema_checks.go`, after each inferred value type and target type:

```go
targetType = semaCanonicalPlatformAlias(resolveNestedTypeReference(model, typ.Name, targetType))
valueType = semaCanonicalPlatformAlias(resolveNestedTypeReference(model, typ.Name, valueType))
```

Apply this pattern in assignment, return, ternary, and condition helpers where types are compared.

- [ ] **Step 4: Run focused sema tests**

Run:

```bash
go test ./internal/sema -run 'TestAnalyze(SystemQualifiedStaticMembers|SchemaImplicitEnumValues|AllowsEveryDocumented)'
```

Expected: pass.

- [ ] **Step 5: Commit comparison and member coverage**

```bash
git add internal/sema/body_calls.go internal/sema/body_ir.go internal/sema/sema_checks.go internal/sema/namespace_resolution_generated_test.go
git commit -m "fix: normalize platform aliases in sema comparisons"
```

---

### Task 8: Add a product-facing regression fixture for the private corpus miss

**Files:**
- Modify: `internal/sema/sema_test.go`
- Test: `internal/sema/sema_test.go`

- [ ] **Step 1: Add a public minimal regression**

Append this test near the existing RestContext tests in `internal/sema/sema_test.go`:

```go
func TestAnalyzeRestResourceHelperUsesSystemQualifiedRequestAndNestedCast(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AccountsResource.cls"), `
@RestResource(urlMapping='/accounts/*')
global class AccountsResource {
  private class Payload {
    public String accountId;
  }
  public static String listAll(System.RestRequest req) {
    return req.requestURI;
  }
  @HttpGet
  global static void doGet() {
    RestRequest req = RestContext.request;
    String uri = listAll(req);
    Object rawPayload = new Payload();
    String accountId = ((Payload)rawPayload).accountId;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "AccountsResource.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Rest resource regression diagnostics: %#v", result.Diagnostics)
	}
}
```

- [ ] **Step 2: Run the regression test**

Run:

```bash
go test ./internal/sema -run TestAnalyzeRestResourceHelperUsesSystemQualifiedRequestAndNestedCast
```

Expected: pass.

- [ ] **Step 3: Run the private project proof**

Run:

```bash
go run ./cmd/glade check --project /Users/matt/Dev/glade-corpus/private/vertex-main --format json | jq -r '.status, .exitCode, .summary.diagnostics, (.diagnostics[]? | [.severity,.code,.file,.line] | @tsv)'
```

Expected:

```text
passed
0
1
warning	GLADEPERF001	examples/real-project/force-app/main/default/classes/Programs_TaskManager.cls	26
```

- [ ] **Step 4: Commit the regression**

```bash
git add internal/sema/sema_test.go
git commit -m "test: cover Rest resource alias regression"
```

---

### Task 9: Add compatibility catalog and Surface Ledger gates

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/capability.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/ids.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/refresh.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/refresh_test.go`
- Test: `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`
- Test: `/Users/matt/Dev/glade-tools/internal/surfaceledger/refresh_test.go`

- [ ] **Step 1: Add a capability row for namespace resolution**

In `/Users/matt/Dev/glade-tools/internal/capability/capability.go`, add this row to the MVP capability catalog near `apex.sema.body`:

```go
{
	Area:       "Apex front end",
	ID:         "apex.namespace-resolution",
	Status:     StatusSupported,
	Capability: "Apex System, Schema, namespace, class, and variable name resolution",
	Notes: "Apex namespace resolution covers System default imports, Schema implicit imports, System-qualified disambiguation for shadowed platform classes, Schema-qualified sObject disambiguation, local-variable/class/namespace expression precedence, scalar/local/class/system type precedence, inner-type-before-namespace type resolution, generics, casts, constructors, locals, fields, params, returns, static members, enum values, and generated coverage for every documented System and Schema type spelling.",
	Required:   true,
}
```

Use the exact field names from the existing `Capability` struct. If the struct uses `Name` instead of `Capability`, adapt only the field name and keep the ID, status, notes, and required flag.

- [ ] **Step 2: Add a capability test**

In `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`, add:

```go
func TestMVPReportIncludesApexNamespaceResolutionGate(t *testing.T) {
	report := MVPReport()
	row, ok := findCapability(report, "apex.namespace-resolution")
	if !ok {
		t.Fatal("missing apex.namespace-resolution capability")
	}
	if row.Status != StatusSupported {
		t.Fatalf("namespace resolution status = %q, want supported", row.Status)
	}
	for _, required := range []string{
		"System default imports",
		"Schema implicit imports",
		"shadowed platform classes",
		"inner-type-before-namespace",
		"every documented System and Schema type spelling",
	} {
		if !strings.Contains(row.Notes, required) {
			t.Fatalf("namespace resolution notes missing %q: %s", required, row.Notes)
		}
	}
}
```

If `findCapability` does not exist, add it in the same test file:

```go
func findCapability(report Report, id string) (Capability, bool) {
	for _, row := range report.Required {
		if row.ID == id {
			return row, true
		}
	}
	for _, row := range report.Tracked {
		if row.ID == id {
			return row, true
		}
	}
	return Capability{}, false
}
```

- [ ] **Step 3: Add Surface Ledger language IDs**

In `/Users/matt/Dev/glade-tools/internal/surfaceledger/ids.go`, add:

```go
func ApexLanguageRuleID(name string) string {
	return "apex-language:" + cleanIdentityPart(name)
}
```

- [ ] **Step 4: Add refresh rows for the language rules**

In `/Users/matt/Dev/glade-tools/internal/surfaceledger/refresh.go`, add rows during refresh:

```go
languageRows := []SurfaceRow{
	{
		SurfaceID: ApexLanguageRuleID("SystemNamespaceDefaultImport"),
		Product:   "apex",
		Area:      "front-end",
		Namespace: "System",
		TypeName:  "System namespace default import",
		Kind:      "language-rule",
		Docs:      DocsPresent,
		GladeShape: GladeShapePresent,
		Evidence:  EvidencePresent,
		Owner:     "internal/sema",
	},
	{
		SurfaceID: ApexLanguageRuleID("SchemaNamespaceImplicitImport"),
		Product:   "apex",
		Area:      "front-end",
		Namespace: "Schema",
		TypeName:  "Schema namespace implicit import",
		Kind:      "language-rule",
		Docs:      DocsPresent,
		GladeShape: GladeShapePresent,
		Evidence:  EvidencePresent,
		Owner:     "internal/sema",
	},
	{
		SurfaceID: ApexLanguageRuleID("NamespaceClassVariablePrecedence"),
		Product:   "apex",
		Area:      "front-end",
		TypeName:  "Namespace, class, and variable precedence",
		Kind:      "language-rule",
		Docs:      DocsPresent,
		GladeShape: GladeShapePresent,
		Evidence:  EvidencePresent,
		Owner:     "internal/sema",
	},
	{
		SurfaceID: ApexLanguageRuleID("TypeResolutionSystemNamespace"),
		Product:   "apex",
		Area:      "front-end",
		TypeName:  "Type resolution and System namespace for types",
		Kind:      "language-rule",
		Docs:      DocsPresent,
		GladeShape: GladeShapePresent,
		Evidence:  EvidencePresent,
		Owner:     "internal/sema",
	},
}
rows = append(rows, languageRows...)
```

Use the existing enum/constant names in `model.go`. If the project uses strings instead of constants, use the exact current string values from the model.

- [ ] **Step 5: Add a Surface Ledger refresh test**

In `/Users/matt/Dev/glade-tools/internal/surfaceledger/refresh_test.go`, add:

```go
func TestRefreshIncludesApexNamespaceLanguageRules(t *testing.T) {
	ledger := refreshFixtureLedger(t)
	required := []string{
		ApexLanguageRuleID("SystemNamespaceDefaultImport"),
		ApexLanguageRuleID("SchemaNamespaceImplicitImport"),
		ApexLanguageRuleID("NamespaceClassVariablePrecedence"),
		ApexLanguageRuleID("TypeResolutionSystemNamespace"),
	}
	for _, id := range required {
		row, ok := findSurfaceRow(ledger.Rows, id)
		if !ok {
			t.Fatalf("missing language rule row %s", id)
		}
		if row.GapClass != "" {
			t.Fatalf("language rule row %s has gap %q", id, row.GapClass)
		}
	}
}
```

If `refreshFixtureLedger` or `findSurfaceRow` does not exist, add local helpers in the test file using the existing refresh fixture pattern.

- [ ] **Step 6: Run glade-tools focused tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/capability ./internal/surfaceledger ./internal/toolcli
```

Expected: pass.

- [ ] **Step 7: Commit glade-tools gates**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/capability internal/surfaceledger internal/toolcli
git commit -m "feat: gate Apex namespace resolution coverage"
```

---

### Task 10: Regenerate checked compatibility docs and site support data

**Files:**
- Modify: `docs/COMPATIBILITY_DASHBOARD.md`
- Modify: `docs/STDLIB_COVERAGE.md`
- Modify: `docs/KNOWN_GAPS.md`
- Modify: `site/docs-src/guide/support-map.md`
- Modify: `site/docs-src/public/data/editor-support.json`
- Test: `site/tests/theme.test.mjs`

- [ ] **Step 1: Regenerate dashboard, stdlib, and known gaps**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools dashboard --output ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools stdlib --output ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools gaps --output ../glade/docs/KNOWN_GAPS.md
```

Expected: generated docs include `apex.namespace-resolution` and do not broaden `stdlib.platform-breadth` beyond checked rows.

- [ ] **Step 2: Refresh site support data**

Run:

```bash
cd /Users/matt/Dev/glade/site
npm test -- --runTestsByPath tests/theme.test.mjs
```

If the test reports stale generated support data, run the existing site generation command from `package.json`, then rerun the same test.

- [ ] **Step 3: Edit support-map wording if needed**

If `site/docs-src/guide/support-map.md` still describes "Semantic checks" without namespace resolution, add this exact wording to the semantic checks row:

```markdown
| Semantic checks | Type references, System and Schema namespace resolution, shadowing and disambiguation, inheritance, interfaces, overloads, locals, assignments, return paths, and token ranges for the supported VM subset. |
```

- [ ] **Step 4: Commit docs**

```bash
cd /Users/matt/Dev/glade
git add docs/COMPATIBILITY_DASHBOARD.md docs/STDLIB_COVERAGE.md docs/KNOWN_GAPS.md site/docs-src/guide/support-map.md site/docs-src/public/data/editor-support.json
git commit -m "docs: report namespace resolution support gate"
```

---

### Task 11: Refresh Surface Ledger with the local Salesforce docs scrape

**Files:**
- Generated check only: `test-results/SURFACE_DASHBOARD.md`
- Generated check only: `test-results/SURFACE_FAILURES.md`
- Generated check only: `test-results/SURFACE_GAPS.md`
- Generated check only: `test-results/SURFACE_LEDGER.json`

- [ ] **Step 1: Run a local refresh into a temp directory**

Run:

```bash
cd /Users/matt/Dev/glade-tools
tmp="$(mktemp -d)"
GLADE_SALESFORCE_DOCS_SOURCE="/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run"
go run ./cmd/glade-tools surface refresh \
  --docs "$GLADE_SALESFORCE_DOCS_SOURCE" \
  --tooling-completions /Users/matt/Dev/glade/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"
ls "$tmp"
```

Expected output includes:

```text
SURFACE_DASHBOARD.md
SURFACE_FAILURES.md
SURFACE_GAPS.md
SURFACE_LEDGER.json
```

- [ ] **Step 2: Check namespace language rows**

Run:

```bash
jq -r '.rows[] | select(.surfaceId|startswith("apex-language:")) | [.surfaceId,.gapClass,.owner] | @tsv' "$tmp/SURFACE_LEDGER.json"
```

Expected:

```text
apex-language:SystemNamespaceDefaultImport		internal/sema
apex-language:SchemaNamespaceImplicitImport		internal/sema
apex-language:NamespaceClassVariablePrecedence		internal/sema
apex-language:TypeResolutionSystemNamespace		internal/sema
```

- [ ] **Step 3: Check for failures**

Run:

```bash
rg '^\\| .*failure' "$tmp/SURFACE_DASHBOARD.md" "$tmp/SURFACE_FAILURES.md" || true
```

Expected: no failure rows for namespace language rules.

- [ ] **Step 4: Decide whether to check in refreshed test-results**

If `test-results/SURFACE_*.md` and `test-results/SURFACE_LEDGER.json` are tracked in the current branch, copy the temp files over and commit them. If they are ignored or treated as run artifacts, keep the temp path in the verification notes and do not commit.

```bash
cd /Users/matt/Dev/glade
git ls-files test-results/SURFACE_DASHBOARD.md test-results/SURFACE_FAILURES.md test-results/SURFACE_GAPS.md test-results/SURFACE_LEDGER.json
```

Expected: only commit files printed by `git ls-files`.

---

### Task 12: Run full verification from both repos

**Files:**
- No source edits expected.

- [ ] **Step 1: Verify product tests**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/typesys ./internal/sema ./internal/cliui ./internal/gladecli
```

Expected: all packages pass.

- [ ] **Step 2: Verify full product suite**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./...
```

Expected: pass.

- [ ] **Step 3: Verify private corpus check**

Run:

```bash
cd /Users/matt/Dev/glade
go run ./cmd/glade check --project /Users/matt/Dev/glade-corpus/private/vertex-main --format json | jq -r '.status, .exitCode, .summary.diagnostics, (.diagnostics[]? | [.severity,.code,.file,.line] | @tsv)'
```

Expected:

```text
passed
0
1
warning	GLADEPERF001	examples/real-project/force-app/main/default/classes/Programs_TaskManager.cls	26
```

- [ ] **Step 4: Verify glade-tools**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/capability ./internal/surfaceledger ./internal/toolcli
```

Expected: pass.

- [ ] **Step 5: Verify no whitespace damage**

Run:

```bash
cd /Users/matt/Dev/glade
git diff --check
cd /Users/matt/Dev/glade-tools
git diff --check
```

Expected: no output.

- [ ] **Step 6: Final review check**

Run:

```bash
cd /Users/matt/Dev/glade
git status --short
cd /Users/matt/Dev/glade-tools
git status --short
```

Expected: only intentional source, test, docs, and generated compatibility files are modified.

---

## Self-Review

- Spec coverage: tasks cover every missing area from the audit: `System` alias inventory, `Schema` implicit imports, generics, all sema type contexts, expression precedence, type precedence, shadowing, private-corpus regression, capability docs, and Surface Ledger measurement.
- Placeholder scan: no task uses banned placeholder text or an unnamed test.
- Type consistency: resolver helpers are named `StandardSystemNamespaceTypeNames`, `StandardSchemaNamespaceTypeNames`, and `semaCanonicalPlatformAlias` throughout.
- Boundary check: product behavior stays in `glade`; compatibility catalog and Surface Ledger changes stay in `glade-tools`.
