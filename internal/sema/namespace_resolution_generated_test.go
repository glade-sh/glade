package sema

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeAllowsSystemQualifiedTypesEverywhere(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestAnalyzeAllowsNamespaceStaticMembersAndEnumValues(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "NamespaceStaticMembers.cls"), `
public class NamespaceStaticMembers {
  public void run() {
    System.StatusCode systemStatus = System.StatusCode.REQUIRED_FIELD_MISSING;
    StatusCode shortStatus = StatusCode.REQUIRED_FIELD_MISSING;
    Schema.DisplayType schemaDisplay = Schema.DisplayType.STRING;
    DisplayType shortDisplay = DisplayType.PICKLIST;
    System.AccessLevel mode = System.AccessLevel.USER_MODE;
    AccessLevel shortMode = AccessLevel.SYSTEM_MODE;
  }
}
`)
	result := Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "NamespaceStaticMembers.cls")},
	}, schema.Schema{}))
	if result.HasErrors() {
		t.Fatalf("unexpected namespace static member diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsSystemQualifiedDatabaseInnerReturnTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SystemDatabaseInnerReturns.cls"), `
public class SystemDatabaseInnerReturns {
  public void run(Account account) {
    System.Database.SaveResult saveResult = System.Database.insert(account, false);
    Database.SaveResult shortSaveResult = System.Database.update(account, false);
    System.Database.DeleteResult deleteResult = System.Database.delete(account, false);
  }
}
`)
	result := Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "SystemDatabaseInnerReturns.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}}))
	if result.HasErrors() {
		t.Fatalf("unexpected System.Database inner return diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeRestResourceHelperUsesSystemQualifiedRequestAndNestedCast(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "RestResourceHelper.cls"), `
@RestResource(urlMapping='/helper/*')
global with sharing class RestResourceHelper {
  public class Payload {
    public String id;
  }
  @HttpPost
  global static String post(Object raw) {
    System.RestRequest req = System.RestContext.request;
    Payload payload = (Payload)raw;
    String id = ((Payload)raw).id;
    return req.requestURI + ':' + payload.id + ':' + id;
  }
}
`)
	result := Analyze(typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "RestResourceHelper.cls")},
	}, schema.Schema{}))
	if result.HasErrors() {
		t.Fatalf("unexpected RestResource helper diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsEveryDocumentedSystemQualifiedTypeSpelling(t *testing.T) {
	t.Parallel()
	typeNames := make([]string, 0, len(typesys.StandardSystemNamespaceTypeNames()))
	for _, name := range typesys.StandardSystemNamespaceTypeNames() {
		// The generated catalog still contains the stale qualified PushUpgrade
		// alias. Salesforce rejects that spelling; the canonical unqualified
		// type remains covered by the residual contract tests.
		if semaAPI67RejectedPlatformType("System." + name) {
			continue
		}
		typeNames = append(typeNames, "System."+name)
	}
	result := analyzeSingleGeneratedClass(t, "UsesSystemQualified.cls", namespaceResolutionSourceForTypes("UsesSystemQualified", typeNames))
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics for documented System-qualified types: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsEveryDocumentedSchemaImplicitTypeSpelling(t *testing.T) {
	t.Parallel()
	result := analyzeSingleGeneratedClass(t, "UsesSchemaImplicit.cls", namespaceResolutionSourceForTypes("UsesSchemaImplicit", typesys.StandardSchemaNamespaceTypeNames()))
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics for documented Schema implicit types: %#v", result.Diagnostics)
	}
}

func namespaceResolutionSourceForTypes(className string, typeNames []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\npublic class %s {\n", className)
	for i, typeName := range typeNames {
		fmt.Fprintf(&b, "  private %s fieldValue%d;\n", typeName, i)
		fmt.Fprintf(&b, "  public %s roundTrip%d(%s input) {\n", typeName, i, typeName)
		fmt.Fprintf(&b, "    %s localValue%d = input;\n", typeName, i)
		fmt.Fprintf(&b, "    Object obj%d = localValue%d;\n", i, i)
		fmt.Fprintf(&b, "    %s castValue%d = (%s)obj%d;\n", typeName, i, typeName, i)
		fmt.Fprintf(&b, "    List<%s> values%d = new List<%s>();\n", typeName, i, typeName)
		fmt.Fprintf(&b, "    return castValue%d;\n", i)
		fmt.Fprintln(&b, "  }")
	}
	fmt.Fprintln(&b, "}")
	return b.String()
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
