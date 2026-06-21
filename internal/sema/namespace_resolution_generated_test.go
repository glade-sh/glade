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

func TestAnalyzeAllowsNamespaceStaticMembersAndEnumValues(t *testing.T) {
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
	for _, name := range typesys.StandardSystemNamespaceTypeNames() {
		t.Run(name, func(t *testing.T) {
			typeName := "System." + name
			result := analyzeSingleGeneratedClass(t, "UsesSystemQualified.cls", namespaceResolutionSourceForType("UsesSystemQualified", typeName))
			if result.HasErrors() {
				t.Fatalf("unexpected diagnostics for %s: %#v", typeName, result.Diagnostics)
			}
		})
	}
}

func TestAnalyzeAllowsEveryDocumentedSchemaImplicitTypeSpelling(t *testing.T) {
	for _, name := range typesys.StandardSchemaNamespaceTypeNames() {
		t.Run(name, func(t *testing.T) {
			result := analyzeSingleGeneratedClass(t, "UsesSchemaImplicit.cls", namespaceResolutionSourceForType("UsesSchemaImplicit", name))
			if result.HasErrors() {
				t.Fatalf("unexpected diagnostics for Schema implicit %s: %#v", name, result.Diagnostics)
			}
		})
	}
}

func namespaceResolutionSourceForType(className, typeName string) string {
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
