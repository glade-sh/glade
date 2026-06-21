package typesys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestBuildIndex(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Hello.cls")
	triggerPath := filepath.Join(root, "HelloTrigger.trigger")
	writeFile(t, classPath, "public class Hello { public void run() {} }")
	writeFile(t, triggerPath, "trigger HelloTrigger on Account (before insert) {}")

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath, triggerPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	if len(idx.Types) != 1 || idx.Types[0].Name != "Hello" || len(idx.Types[0].Members) != 1 {
		t.Fatalf("types = %#v", idx.Types)
	}
	if len(idx.Triggers) != 1 || idx.Triggers[0].ObjectName != "Account" {
		t.Fatalf("triggers = %#v", idx.Triggers)
	}
}

func TestBuildIndexDuplicateType(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one.cls")
	second := filepath.Join(root, "two.cls")
	writeFile(t, first, "public class Hello {}")
	writeFile(t, second, "public class hello {}")

	idx := Build(project.Project{Root: root, ApexFiles: []string{first, second}}, schema.Schema{})
	if !idx.HasErrors() {
		t.Fatalf("expected duplicate diagnostic: %#v", idx.Diagnostics)
	}
}

func TestBuildIndexAllowsDuplicateTypesAcrossPackageDirectories(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "packages/one/classes/Shared.cls")
	second := filepath.Join(root, "packages/two/classes/Shared.cls")
	writeFile(t, first, "public class Shared { public String one() { return 'one'; } }")
	writeFile(t, second, "public class Shared { public String two() { return 'two'; } }")

	idx := Build(project.Project{
		Root: root,
		PackageDirectories: []project.PackageDirectory{
			{Path: "packages/one"},
			{Path: "packages/two"},
		},
		ApexFiles: []string{first, second},
	}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	if len(idx.Types) != 2 {
		t.Fatalf("types = %#v", idx.Types)
	}
}

func TestBuildLoadsManagedPackageArtifactSymbols(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "pkg.glade-package.json")
	if err := os.WriteFile(artifactPath, []byte(`{
  "namespace": "pkg",
  "version": "1.0",
  "apexTypes": [
    {
      "kind": "class",
      "name": "Address",
      "namespace": "pkg",
      "sourceRoot": "/tmp/pkg",
      "version": "1.0",
      "dependency": true,
      "modifiers": ["global"]
    }
  ],
  "objects": [
    {
      "name": "pkg__CartItemLine__c",
      "fields": [
        {"name": "pkg__Product__c", "type": "reference"}
      ]
    }
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := Build(project.Project{
		Root: root,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:    "pkg",
			ArtifactPath: artifactPath,
			Version:      "1.0",
			Status:       "loaded",
		}},
	}, schema.Schema{})

	if len(idx.Dependencies) != 1 || idx.Dependencies[0].Status != "loaded" || idx.Dependencies[0].ApexTypes != 1 || idx.Dependencies[0].Objects != 1 {
		t.Fatalf("dependencies = %#v", idx.Dependencies)
	}
	if len(idx.Types) != 1 || idx.Types[0].Name != "Address" || idx.Types[0].Namespace != "pkg" || !idx.Types[0].Dependency {
		t.Fatalf("types = %#v", idx.Types)
	}
	if len(idx.Objects) != 1 || idx.Objects[0].Name != "pkg__CartItemLine__c" {
		t.Fatalf("objects = %#v", idx.Objects)
	}
}

func TestBuildLoadsCapturedManagedPackageArtifactMetadata(t *testing.T) {
	root := t.TempDir()
	packagesDir := filepath.Join(root, "packages")
	artifactPath := filepath.Join(packagesDir, "pkg.glade-package.json")
	if err := os.MkdirAll(packagesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact, err := packageartifact.BuildCaptured(packageartifact.BuildCapturedOptions{
		Namespace:           "pkg",
		PackageName:         "Billing Core",
		Version:             "1.2.3",
		LabelNames:          []string{"pkg__Billing_Error"},
		StaticResourceNames: []string{"pkg__BillingAssets"},
		LightningBundles: []packageartifact.LightningBundle{{
			Namespace: "pkg",
			Name:      "billingConsole",
			Type:      "lwc",
			Exposed:   true,
		}},
		Capture: packageartifact.CaptureProvenance{
			Source: "org",
			OrgID:  "00Dxx0000000001",
		},
		ApexTypes: []packageartifact.ApexType{{
			Kind:      apexast.DeclarationClass,
			Name:      "BillingGateway",
			Namespace: "pkg",
			Modifiers: []string{"global"},
		}},
		Objects: []schema.Object{{
			Name: "pkg__Billing_Profile__c",
			Fields: []schema.Field{{
				Name: "pkg__External_Key__c",
				Type: "Text",
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := packageartifact.WriteJSON(artifactPath, artifact); err != nil {
		t.Fatal(err)
	}
	consumerRoot := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json:1.2.3"]
`)

	loaded, err := project.Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	idx := Build(loaded, schema.Schema{})

	if len(idx.Dependencies) != 1 || idx.Dependencies[0].Status != "loaded" {
		t.Fatalf("dependencies = %#v", idx.Dependencies)
	}
	if idx.Dependencies[0].ApexTypes != 1 || idx.Dependencies[0].Objects != 1 || idx.Dependencies[0].Labels != 1 || idx.Dependencies[0].StaticResources != 1 {
		t.Fatalf("dependency summary = %#v", idx.Dependencies[0])
	}
	if idx.Dependencies[0].CaptureSource != "org" || idx.Dependencies[0].CaptureOrgID != "00Dxx0000000001" {
		t.Fatalf("dependency capture = %#v", idx.Dependencies[0])
	}
	if len(idx.Types) != 1 || idx.Types[0].Name != "BillingGateway" || !idx.Types[0].Artifact {
		t.Fatalf("types = %#v", idx.Types)
	}
	if !codeIntelIDPresent(idx.CodeIntelSymbols, "metadata:label:pkg__Billing_Error") {
		t.Fatalf("missing label symbol: %#v", idx.CodeIntelSymbols)
	}
	if !codeIntelIDPresent(idx.CodeIntelSymbols, "metadata:static_resource:pkg__BillingAssets") {
		t.Fatalf("missing static resource symbol: %#v", idx.CodeIntelSymbols)
	}
}

func TestBuildDoesNotDuplicateMissingPackageShimDependency(t *testing.T) {
	idx := Build(project.Project{
		PackageShims: []project.PackageShim{{
			Namespace:  "pkg",
			SourceRoot: "/missing/package-shim",
			Status:     "missing",
		}},
		DependencyDiagnostics: []project.DependencyDiagnostic{{
			Namespace:  "pkg",
			SourceRoot: "/missing/package-shim",
			Status:     "missing",
			Code:       "package_shim_missing",
			Message:    "package shim source root not found",
		}},
	}, schema.Schema{})

	if len(idx.Dependencies) != 1 {
		t.Fatalf("dependencies = %#v", idx.Dependencies)
	}
	if idx.Dependencies[0].Status != "missing" || len(idx.Dependencies[0].Diagnostics) != 1 {
		t.Fatalf("dependency = %#v", idx.Dependencies[0])
	}
}

func TestBuildNamespacesSourceBackedDependencySchema(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	objectPath := filepath.Join(depRoot, "force-app/main/default/objects/Order__c/Order__c.object-meta.xml")
	fieldPath := filepath.Join(depRoot, "force-app/main/default/objects/Order__c/fields/TransactionDate__c.field-meta.xml")
	lookupPath := filepath.Join(depRoot, "force-app/main/default/objects/Order__c/fields/Entity__c.field-meta.xml")
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Order</label></CustomObject>`)
	writeFile(t, fieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>TransactionDate__c</fullName><type>Date</type></CustomField>`)
	writeFile(t, lookupPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Entity__c</fullName><type>Lookup</type><referenceTo>Entity__c</referenceTo><relationshipName>Entity__r</relationshipName></CustomField>`)

	depProject := project.Project{
		Root:        depRoot,
		Namespace:   "pkg",
		ObjectFiles: []string{objectPath},
		FieldFiles:  []string{fieldPath, lookupPath},
	}
	idx := Build(project.Project{
		Root: root,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkg",
			SourceRoot: depRoot,
			Status:     "loaded",
			Project:    &depProject,
		}},
	}, schema.Schema{})

	objects := map[string]schema.Object{}
	for _, object := range idx.Objects {
		objects[object.Name] = object
	}
	order, ok := objects["pkg__Order__c"]
	if !ok {
		t.Fatalf("objects = %#v", idx.Objects)
	}
	fields := map[string]schema.Field{}
	for _, field := range order.Fields {
		fields[field.Name] = field
	}
	if _, ok := fields["pkg__TransactionDate__c"]; !ok {
		t.Fatalf("fields = %#v", order.Fields)
	}
	entity := fields["pkg__Entity__c"]
	if entity.RelationshipName != "pkg__Entity__r" || len(entity.ReferenceTo) != 1 || entity.ReferenceTo[0] != "pkg__Entity__c" {
		t.Fatalf("entity field = %#v", entity)
	}
}

func TestBuildDoesNotSurfaceSourceBackedDependencyDuplicateDiagnostics(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	first := filepath.Join(depRoot, "one.cls")
	second := filepath.Join(depRoot, "two.cls")
	writeFile(t, first, "public class SharedDependency {}")
	writeFile(t, second, "public class sharedDependency {}")

	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkg",
		ApexFiles: []string{first, second},
	}
	idx := Build(project.Project{
		Root: root,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkg",
			SourceRoot: depRoot,
			Status:     "loaded",
			Project:    &depProject,
		}},
	}, schema.Schema{})

	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
}

func TestBuildIndexDiscoversTests(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "HelloTest.cls")
	writeFile(t, classPath, "@IsTest(IsParallel=true) private class HelloTest { private class Helper {} @isTest private static void run() {} }")

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	types := make(map[string]TypeSymbol)
	for _, typ := range idx.Types {
		types[typ.Name] = typ
	}
	runIsTest := false
	for _, member := range types["HelloTest"].Members {
		if member.Name == "run" {
			runIsTest = member.IsTest
		}
	}
	if !types["HelloTest"].IsTest || !types["HelloTest.Helper"].IsTest || !runIsTest {
		t.Fatalf("test flags not set: %#v", idx.Types)
	}
}

func TestBuildIndexKeepsMethodParameters(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Hello.cls")
	writeFile(t, classPath, "public class Hello { public void run(String name, Account account) {} }")

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	params := idx.Types[0].Members[0].Parameters
	if len(params) != 2 || params[0].Name != "name" || params[1].Type != "Account" {
		t.Fatalf("parameters = %#v", params)
	}
}

func TestBuildIndexPreservesQualifiedMethodParameterTypes(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Hello.cls")
	writeFile(t, classPath, `
public class Hello {
  public void run(System.Location location, final List<System.Cookie> cookies) {}
  private class Location {
    public Location(System.Location location) {}
  }
}
`)

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	params := idx.Types[0].Members[0].Parameters
	if len(params) != 2 {
		t.Fatalf("parameters = %#v", params)
	}
	if params[0].Type != "System.Location" || params[1].Type != "List<System.Cookie>" {
		t.Fatalf("parameter types = %#v", params)
	}
	nested := idx.Types[1]
	if nested.Name != "Hello.Location" || len(nested.Members) != 1 || len(nested.Members[0].Parameters) != 1 {
		t.Fatalf("nested type = %#v", nested)
	}
	if nested.Members[0].Parameters[0].Type != "System.Location" {
		t.Fatalf("nested constructor parameters = %#v", nested.Members[0].Parameters)
	}
}

func TestBuildIndexPromotesNestedTypes(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Outer.cls")
	writeFile(t, classPath, `
public class Outer {
  public class Inner {
    public String label() { return 'inner'; }
  }
  public interface Marker {
    void mark();
  }
}
`)

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	types := map[string]TypeSymbol{}
	for _, typ := range idx.Types {
		types[typ.Name] = typ
	}
	for _, name := range []string{"Outer", "Outer.Inner", "Outer.Marker"} {
		if _, ok := types[name]; !ok {
			t.Fatalf("missing nested type %s in %#v", name, idx.Types)
		}
	}
	if len(types["Outer.Inner"].Members) != 1 || types["Outer.Inner"].Members[0].Name != "label" {
		t.Fatalf("inner members = %#v", types["Outer.Inner"].Members)
	}
}

func TestBuildIndexAddsDataWeaveScriptResources(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "force-app/main/default/dw/helloWorld.dwl")
	metaPath := filepath.Join(root, "force-app/main/default/dw/error.dwl-meta.xml")
	writeFile(t, scriptPath, "%dw 2.0\noutput application/json\n---\n{}")
	writeFile(t, metaPath, "<DataWeaveResource/>")

	idx := Build(project.Project{
		Root:           root,
		DataWeaveFiles: []string{scriptPath},
		DataWeaveMetas: []string{metaPath},
	}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	types := map[string]TypeSymbol{}
	for _, typ := range idx.Types {
		types[typ.Name] = typ
	}
	for _, name := range []string{"DataWeaveScriptResource.helloWorld", "DataWeaveScriptResource.error"} {
		typ, ok := types[name]
		if !ok {
			t.Fatalf("missing %s in %#v", name, idx.Types)
		}
		if typ.SuperClass != "DataWeave.Script" || len(typ.Members) != 2 {
			t.Fatalf("type %s = %#v", name, typ)
		}
	}
}

func TestUpdateApexFilesReplacesChangedSymbolsAndDropsDeleted(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First.cls")
	second := filepath.Join(root, "Second.cls")
	trigger := filepath.Join(root, "SecondTrigger.trigger")
	writeFile(t, first, "public class First {}")
	writeFile(t, second, "public class Second { public void oldName() {} }")
	writeFile(t, trigger, "trigger SecondTrigger on Account (before insert) {}")
	idx := Build(project.Project{Root: root, ApexFiles: []string{first, second, trigger}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}

	writeFile(t, second, "public class Second { public void newName() {} }")
	updated := UpdateApexFiles(idx, []string{second}, []string{trigger})
	if updated.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", updated.Diagnostics)
	}
	if len(updated.Triggers) != 0 {
		t.Fatalf("triggers = %#v", updated.Triggers)
	}
	types := map[string]TypeSymbol{}
	for _, typ := range updated.Types {
		types[typ.Name] = typ
	}
	if _, ok := types["First"]; !ok {
		t.Fatalf("missing retained type: %#v", updated.Types)
	}
	secondType := types["Second"]
	if len(secondType.Members) != 1 || secondType.Members[0].Name != "newName" {
		t.Fatalf("second type = %#v", secondType)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func codeIntelIDPresent(symbols []packageartifact.CodeIntelSymbol, id string) bool {
	for _, symbol := range symbols {
		if symbol.ID == id {
			return true
		}
	}
	return false
}
