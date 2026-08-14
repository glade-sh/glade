package typesys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/namespaceremap"
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

func TestBuildRecordsEffectiveSourceAPIVersion(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Hello.cls")
	triggerPath := filepath.Join(root, "Hello.trigger")
	writeFile(t, classPath, "public class Hello {}")
	writeFile(t, classPath+"-meta.xml", `<ApexClass><apiVersion>65.0</apiVersion></ApexClass>`)
	writeFile(t, triggerPath, "trigger Hello on Account (before insert) {}")
	idx := Build(project.Project{Root: root, SourceAPIVersion: "64.0", ApexFiles: []string{classPath, triggerPath}}, schema.Schema{})
	if got := idx.Types[0].EffectiveAPIVersion; got != "65.0" {
		t.Fatalf("type API version = %q", got)
	}
	if got := idx.Triggers[0].EffectiveAPIVersion; got != "64.0" {
		t.Fatalf("trigger API version = %q", got)
	}
}

func TestBuildWithArtifactsCapturesAPIMetadataAndVersionFromOneRead(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "CapturedMetadata.cls")
	metadata := path + "-meta.xml"
	oldMetadata := `<ApexClass><apiVersion>61.0</apiVersion></ApexClass>`
	newMetadata := `<ApexClass><apiVersion>62.0</apiVersion></ApexClass>`
	writeFile(t, path, "public class CapturedMetadata {}")
	writeFile(t, metadata, oldMetadata)
	mutated := false
	t.Cleanup(setApexMetadataCaptureHookForTesting(func(capturedPath string) {
		if capturedPath == path && !mutated {
			mutated = true
			writeFile(t, metadata, newMetadata)
		}
	}))
	idx, artifacts := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	if !mutated {
		t.Fatal("metadata capture hook did not run")
	}
	if !idx.HasErrors() {
		t.Fatal("metadata mutation during capture was accepted")
	}
	_ = artifacts
}

func TestIncrementalUpdateCapturesAPIMetadataAndVersionFromOneRead(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app", "main", "default", "classes", "IncrementalMetadata.cls")
	metadata := path + "-meta.xml"
	writeFile(t, path, "public class IncrementalMetadata { public static Integer value() { return 1; } }")
	writeFile(t, metadata, `<ApexClass><apiVersion>61.0</apiVersion></ApexClass>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	previous := Build(p, schema.Schema{})
	writeFile(t, path, "public class IncrementalMetadata { public static Integer value() { return 2; } }")
	mutated := false
	t.Cleanup(setApexMetadataCaptureHookForTesting(func(capturedPath string) {
		if capturedPath == path && !mutated {
			mutated = true
			writeFile(t, metadata, `<ApexClass><apiVersion>62.0</apiVersion></ApexClass>`)
		}
	}))
	updated, fast := updateApexFilesIncremental(previous, []string{path}, nil)
	if fast {
		t.Fatal("incremental update accepted metadata mutation during capture")
	}
	if !mutated {
		t.Fatal("metadata capture hook did not run")
	}
	_ = updated
}

func TestBuildWithArtifactsRejectsSourceMutationAfterMetadataCapture(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AtomicSource.cls")
	writeFile(t, path, "public class AtomicSource { public static Integer value() { return 1; } }")
	writeFile(t, path+"-meta.xml", `<ApexClass><apiVersion>61.0</apiVersion></ApexClass>`)
	t.Cleanup(setApexMetadataCaptureHookForTesting(func(capturedPath string) {
		if capturedPath == path {
			writeFile(t, path, "public class AtomicSource { public static Integer value() { return 2; } }")
		}
	}))
	idx, _ := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	if !idx.HasErrors() {
		t.Fatalf("source mutation during build was accepted: %#v", idx)
	}
}

func TestIncrementalUpdateRejectsSourceMutationAfterMetadataCapture(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app", "main", "default", "classes", "AtomicIncremental.cls")
	writeFile(t, path, "public class AtomicIncremental { public static Integer value() { return 1; } }")
	writeFile(t, path+"-meta.xml", `<ApexClass><apiVersion>61.0</apiVersion></ApexClass>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	previous := Build(p, schema.Schema{})
	writeFile(t, path, "public class AtomicIncremental { public static Integer value() { return 2; } }")
	t.Cleanup(setApexMetadataCaptureHookForTesting(func(capturedPath string) {
		if capturedPath == path {
			writeFile(t, path, "public class AtomicIncremental { public static Integer value() { return 3; } }")
		}
	}))
	if _, fast := updateApexFilesIncremental(previous, []string{path}, nil); fast {
		t.Fatal("incremental source mutation during metadata capture was accepted")
	}
}

func TestBuildParsesNamespaceTokenApex(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "UsesTokens.cls")
	writeFile(t, path, `public class UsesTokens {
  public void run() {
    %%%NAMESPACE_DOT%%%UTIL_CustomSettings_API.getSettingsForTests(
      new %%%NAMESPACE%%%Hierarchy_Settings__c(%%%NAMESPACE%%%Disable_Preferred_Email_Enforcement__c = false)
    );
  }
}`)

	idx := Build(project.Project{
		Root:      root,
		Namespace: "",
		ApexFiles: []string{path},
	}, schema.Schema{})
	for _, diag := range idx.Diagnostics {
		if diag.Code == "APEXPARSE001" {
			t.Fatalf("unexpected parse diagnostic: %#v", diag)
		}
	}
}

func TestBuildIndexDuplicateType(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one.cls")
	second := filepath.Join(root, "two.cls")
	writeFile(t, first, "public class Hello {}")
	writeFile(t, second, "public class hello {}")

	idx := Build(project.Project{Root: root, ApexFiles: []string{first, second}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("duplicate type should be warning-only: %#v", idx.Diagnostics)
	}
	if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADETYPE001" || idx.Diagnostics[0].Severity != diagnostic.Warning {
		t.Fatalf("expected duplicate warning diagnostic: %#v", idx.Diagnostics)
	}
}

func TestBuildIndexDuplicateTypeSameOwnerIsError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ProbeDuplicateTypeName.cls")
	writeFile(t, path, `public class ProbeDuplicateTypeName {
  class Item {}
  interface Item {}
}`)

	idx := Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	if !idx.HasErrors() {
		t.Fatalf("same-owner duplicate type should be an error: %#v", idx.Diagnostics)
	}
	found := false
	for _, diag := range idx.Diagnostics {
		if diag.Code == "GLADETYPE001" && diag.Severity == diagnostic.Error {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GLADETYPE001 error for same-owner duplicate, got %#v", idx.Diagnostics)
	}
}

func TestTypeSymbolPreservesOwnerAndNesting(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Outer.cls")
	writeFile(t, path, `public class Outer {
  class Mid {
    class Deep {}
  }
}`)

	idx := Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	byName := map[string]TypeSymbol{}
	for _, typ := range idx.Types {
		byName[typ.Name] = typ
	}
	outer, ok := byName["Outer"]
	if !ok {
		t.Fatalf("missing Outer: %#v", idx.Types)
	}
	if outer.LocalName != "Outer" || outer.OwnerName != "" || outer.NestingDepth != 0 {
		t.Fatalf("Outer identity = local=%q owner=%q depth=%d", outer.LocalName, outer.OwnerName, outer.NestingDepth)
	}
	mid, ok := byName["Outer.Mid"]
	if !ok {
		t.Fatalf("missing Outer.Mid: %#v", idx.Types)
	}
	if mid.LocalName != "Mid" || mid.OwnerName != "Outer" || mid.NestingDepth != 1 {
		t.Fatalf("Outer.Mid identity = local=%q owner=%q depth=%d", mid.LocalName, mid.OwnerName, mid.NestingDepth)
	}
	deep, ok := byName["Outer.Mid.Deep"]
	if !ok {
		t.Fatalf("missing Outer.Mid.Deep: %#v", idx.Types)
	}
	if deep.LocalName != "Deep" || deep.OwnerName != "Outer.Mid" || deep.NestingDepth != 2 {
		t.Fatalf("Outer.Mid.Deep identity = local=%q owner=%q depth=%d", deep.LocalName, deep.OwnerName, deep.NestingDepth)
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

func TestBuildRemapsSourceBackedDependencyNamespaceReferences(t *testing.T) {
	root := t.TempDir()
	depRoot := filepath.Join(root, "base-source")
	helper := filepath.Join(depRoot, "force-app/main/classes/Helper.cls")
	gateway := filepath.Join(depRoot, "force-app/main/classes/Gateway.cls")
	trigger := filepath.Join(depRoot, "force-app/main/triggers/BillingTrigger.trigger")
	writeFile(t, helper, "global class Helper { global static String value() { return 'ok'; } }")
	writeFile(t, gateway, "global class Gateway { global static String value() { return BasePkg.Helper.value(); } }")
	writeFile(t, trigger, "trigger BillingTrigger on BasePkg__Billing__c (before insert) { BasePkg.Helper.value(); }")

	idx := Build(project.Project{
		Root:      root,
		Namespace: "consumer",
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace: "stagepkg",
			Status:    "loaded",
			Project: &project.Project{
				Root:      depRoot,
				Namespace: "stagepkg",
				NamespaceRemaps: []namespaceremap.Rule{{
					From: "BasePkg",
					To:   "stagepkg",
				}},
				ApexFiles: []string{helper, gateway, trigger},
			},
		}},
	}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	typeFound := false
	for _, typ := range idx.Types {
		if typ.Name != "Gateway" {
			continue
		}
		typeFound = true
		if typ.Namespace != "stagepkg" {
			t.Fatalf("Gateway namespace = %q, want stagepkg", typ.Namespace)
		}
		if len(typ.SourceNamespaceRemaps) != 1 || typ.SourceNamespaceRemaps[0].From != "BasePkg" || typ.SourceNamespaceRemaps[0].To != "stagepkg" {
			t.Fatalf("source remaps = %#v", typ.SourceNamespaceRemaps)
		}
	}
	if !typeFound {
		t.Fatalf("Gateway dependency type not found: %#v", idx.Types)
	}
	triggerFound := false
	for _, trig := range idx.Triggers {
		if trig.Name != "BillingTrigger" {
			continue
		}
		triggerFound = true
		if trig.Namespace != "stagepkg" || trig.ObjectName != "stagepkg__Billing__c" {
			t.Fatalf("trigger namespace/object = %q/%q", trig.Namespace, trig.ObjectName)
		}
		if len(trig.SourceNamespaceRemaps) != 1 || trig.SourceNamespaceRemaps[0].From != "BasePkg" || trig.SourceNamespaceRemaps[0].To != "stagepkg" {
			t.Fatalf("trigger source remaps = %#v", trig.SourceNamespaceRemaps)
		}
	}
	if !triggerFound {
		t.Fatalf("BillingTrigger dependency trigger not found: %#v", idx.Triggers)
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

func TestBuildIndexPreservesGenericInterfacesWithCommaArgs(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "MapBatch.cls")
	writeFile(t, classPath, `
public class MapBatch implements Database.Batchable<Map<String, Object>>,
    Database.Stateful {
}
`)

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	var found TypeSymbol
	for _, typ := range idx.Types {
		if typ.Name == "MapBatch" {
			found = typ
			break
		}
	}
	if len(found.Interfaces) != 2 ||
		found.Interfaces[0] != "Database.Batchable<Map<String, Object>>" ||
		found.Interfaces[1] != "Database.Stateful" {
		t.Fatalf("interfaces = %#v", found.Interfaces)
	}
}

func TestBuildIndexParsesInheritanceAfterCommentBrace(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "PostInstall.cls")
	writeFile(t, classPath, `
@SuppressWarnings('PMD.AvoidGlobalModifier')
/**
 * @author {@link Example}
 */
global class PostInstall implements InstallHandler {
  global void onInstall(InstallContext context) {}
}
`)

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	var found TypeSymbol
	for _, typ := range idx.Types {
		if typ.Name == "PostInstall" {
			found = typ
			break
		}
	}
	if len(found.Interfaces) != 1 || found.Interfaces[0] != "InstallHandler" {
		t.Fatalf("interfaces = %#v", found.Interfaces)
	}
}

func TestBuildIndexUsesParserInheritanceAndBodyRanges(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Child.cls")
	triggerPath := filepath.Join(root, "Child.trigger")
	classSource := `/* { comment } */
public class Child extends Base implements One, Generic<String> {
  public String run() { return '}'; }
}`
	triggerSource := `trigger ChildTrigger on Account (before insert) {
  System.debug('}');
}`
	writeFile(t, classPath, classSource)
	writeFile(t, triggerPath, triggerSource)

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath, triggerPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	var child TypeSymbol
	for _, typ := range idx.Types {
		if typ.Name == "Child" {
			child = typ
			break
		}
	}
	if child.SuperClass != "Base" || len(child.Interfaces) != 2 || child.Interfaces[0] != "One" || child.Interfaces[1] != "Generic<String>" {
		t.Fatalf("inheritance = %#v", child)
	}
	if len(child.Members) != 1 || child.Members[0].BodyRange == nil {
		t.Fatalf("member body range = %#v", child.Members)
	}
	r := child.Members[0].BodyRange
	if got := classSource[r.Start.Offset:r.End.Offset]; got != "{ return '}'; }" {
		t.Fatalf("member body = %q", got)
	}
	if len(idx.Triggers) != 1 || idx.Triggers[0].BodyRange == nil {
		t.Fatalf("trigger body range = %#v", idx.Triggers)
	}
	r = idx.Triggers[0].BodyRange
	if got := triggerSource[r.Start.Offset:r.End.Offset]; got != "{\n  System.debug('}');\n}" {
		t.Fatalf("trigger body = %q", got)
	}
}

func TestBuildIndexPromotesNestedTypes(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Container.cls")
	writeFile(t, classPath, `
public class Container {
  public class Nested {
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
	for _, name := range []string{"Container", "Container.Nested", "Container.Marker"} {
		if _, ok := types[name]; !ok {
			t.Fatalf("missing nested type %s in %#v", name, idx.Types)
		}
	}
	if len(types["Container.Nested"].Members) != 1 || types["Container.Nested"].Members[0].Name != "label" {
		t.Fatalf("inner members = %#v", types["Container.Nested"].Members)
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

func TestBuildIndexAddsFlowInterviewTypes(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(root, "force-app/main/default/flows/Calculate_discounts.flow-meta.xml")
	writeFile(t, flowPath, "<Flow/>")

	idx := Build(project.Project{
		Root:      root,
		FlowFiles: []string{flowPath},
	}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	types := map[string]TypeSymbol{}
	for _, typ := range idx.Types {
		types[typ.Name] = typ
	}
	typ, ok := types["Flow.Interview.Calculate_discounts"]
	if !ok {
		t.Fatalf("missing flow interview type in %#v", idx.Types)
	}
	if typ.SuperClass != "Flow.Interview" {
		t.Fatalf("type = %#v", typ)
	}
	var found bool
	for _, member := range typ.Members {
		if member.Kind == apexast.DeclarationConstructor &&
			len(member.Parameters) == 1 &&
			member.Parameters[0].Type == "Map<String,Object>" {
			found = true
		}
	}
	if !found {
		t.Fatalf("members = %#v", typ.Members)
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
	if err := os.Remove(trigger); err != nil {
		t.Fatal(err)
	}
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

func TestTypeSymbolPreservesEnumMembersAndHasBody(t *testing.T) {
	root := t.TempDir()
	enumPath := filepath.Join(root, "Color.cls")
	classPath := filepath.Join(root, "Shape.cls")
	writeFile(t, enumPath, `public enum Color { Red, Green }`)
	writeFile(t, classPath, `public abstract class Shape {
  public abstract void draw();
  public void paint() {}
}`)

	idx := Build(project.Project{Root: root, ApexFiles: []string{enumPath, classPath}}, schema.Schema{})
	byName := map[string]TypeSymbol{}
	for _, typ := range idx.Types {
		byName[typ.Name] = typ
	}
	color, ok := byName["Color"]
	if !ok {
		t.Fatalf("missing Color: %#v", idx.Types)
	}
	if len(color.Members) != 2 || color.Members[0].Name != "Red" || color.Members[1].Name != "Green" {
		t.Fatalf("enum members = %#v", color.Members)
	}
	shape, ok := byName["Shape"]
	if !ok {
		t.Fatalf("missing Shape: %#v", idx.Types)
	}
	var draw, paint MemberSymbol
	for _, member := range shape.Members {
		switch member.Name {
		case "draw":
			draw = member
		case "paint":
			paint = member
		}
	}
	if draw.HasBody || !paint.HasBody {
		t.Fatalf("HasBody draw=%v paint=%v members=%#v", draw.HasBody, paint.HasBody, shape.Members)
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

func TestBuildTriggerSymbolRecordsSourceOccurrenceMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ThingTrigger.trigger")
	writeFile(t, path, "trigger ThingTrigger on Thing__c (before insert) {}")

	idx := Build(project.Project{
		Root:             root,
		Namespace:        "runtime",
		SourceAPIVersion: "62.0",
		ApexFiles:        []string{path},
	}, schema.Schema{})
	if len(idx.Triggers) != 1 {
		t.Fatalf("triggers = %#v", idx.Triggers)
	}
	trigger := idx.Triggers[0]
	if trigger.SourceRoot != root {
		t.Fatalf("trigger SourceRoot = %q, want %q", trigger.SourceRoot, root)
	}
	if trigger.Version != "" {
		t.Fatalf("trigger Version = %q, want empty for a non-dependency source", trigger.Version)
	}
}
