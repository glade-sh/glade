package packageartifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestBuildAppliesInstalledNamespaceToCustomSchema(t *testing.T) {
	artifact, err := Build("pkg", "1.0", project.Project{Root: t.TempDir()}, schema.Schema{Objects: []schema.Object{
		{
			Name: "CartItemLine__c",
			Fields: []schema.Field{
				{Name: "Product__c", Type: "reference", ReferenceTo: []string{"Membership__c"}},
				{Name: "Name", Type: "string"},
			},
		},
		{
			Name:   "Account",
			Fields: []schema.Field{{Name: "Name", Type: "string"}},
		},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Objects[0].Name != "pkg__CartItemLine__c" {
		t.Fatalf("object name = %q", artifact.Objects[0].Name)
	}
	if artifact.Objects[0].Fields[0].Name != "pkg__Product__c" {
		t.Fatalf("field name = %q", artifact.Objects[0].Fields[0].Name)
	}
	if artifact.Objects[0].Fields[0].ReferenceTo[0] != "pkg__Membership__c" {
		t.Fatalf("referenceTo = %#v", artifact.Objects[0].Fields[0].ReferenceTo)
	}
	if artifact.Objects[0].Fields[1].Name != "Name" {
		t.Fatalf("standard field name = %q", artifact.Objects[0].Fields[1].Name)
	}
	if artifact.Objects[1].Name != "Account" {
		t.Fatalf("standard object name = %q", artifact.Objects[1].Name)
	}
}

func TestCompareIgnoresLocalApexFilePaths(t *testing.T) {
	from := Artifact{
		Namespace: "pkg",
		ApexTypes: []ApexType{{
			Name:       "Address",
			Namespace:  "pkg",
			File:       "/tmp/one/force-app/classes/Address.cls",
			SourceRoot: "/tmp/one",
			Members:    []ApexMember{{Name: "format"}},
		}},
	}
	to := Artifact{
		Namespace: "pkg",
		ApexTypes: []ApexType{{
			Name:       "Address",
			Namespace:  "pkg",
			File:       "/tmp/two/force-app/classes/Address.cls",
			SourceRoot: "/tmp/two",
			Members:    []ApexMember{{Name: "format"}},
		}},
	}

	diff := Compare(from, to)
	if diff.ChangedTypes != 0 {
		t.Fatalf("changedTypes = %d, want 0; changed names=%#v", diff.ChangedTypes, diff.ChangedTypeNames)
	}
}

func TestCompareReportsChangedObjects(t *testing.T) {
	from := Artifact{
		Namespace:  "pkg",
		SourceHash: "same",
		Objects: []schema.Object{{
			Name:   "pkg__Thing__c",
			Fields: []schema.Field{{Name: "Name", Type: "string"}},
		}},
	}
	to := Artifact{
		Namespace:  "pkg",
		SourceHash: "same",
		Objects: []schema.Object{{
			Name:   "pkg__Thing__c",
			Fields: []schema.Field{{Name: "Name", Type: "textarea"}},
		}},
	}

	diff := Compare(from, to)
	if !diff.Changed {
		t.Fatal("changed = false, want true")
	}
	if diff.ChangedObjects != 1 || len(diff.ChangedObjectNames) != 1 || diff.ChangedObjectNames[0] != "pkg__Thing__c" {
		t.Fatalf("changed objects = %d %#v", diff.ChangedObjects, diff.ChangedObjectNames)
	}
}

func TestReadJSONLoadsArtifactWithoutCodeIntelFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg.glade-package.json")
	if err := os.WriteFile(path, []byte(`{
  "namespace": "pkg",
  "sourceHash": "abc123",
  "builtAt": "2026-06-14T00:00:00Z",
  "apexTypes": [{"kind": "class", "name": "Address", "namespace": "pkg"}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	artifact, err := ReadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.CodeIntelSymbolsVersion != 0 || len(artifact.CodeIntelSymbols) != 0 || artifact.CodeIntelUsesVersion != 0 || len(artifact.CodeIntelUses) != 0 {
		t.Fatalf("codeintel fields = %d/%d symbols, %d/%d uses", artifact.CodeIntelSymbolsVersion, len(artifact.CodeIntelSymbols), artifact.CodeIntelUsesVersion, len(artifact.CodeIntelUses))
	}
}

func TestBuildEmitsCodeIntelContractSymbols(t *testing.T) {
	root := t.TempDir()
	labelPath := filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels-meta.xml")
	staticPath := filepath.Join(root, "force-app/main/default/staticresources/Site.resource-meta.xml")
	writePackageArtifactTestFile(t, labelPath, `<CustomLabels xmlns="http://soap.sforce.com/2006/04/metadata"><labels><fullName>Greeting</fullName><value>Hello</value></labels></CustomLabels>`)
	writePackageArtifactTestFile(t, staticPath, `<StaticResource xmlns="http://soap.sforce.com/2006/04/metadata"/>`)

	artifact, err := Build("pkg", "1.0", project.Project{
		Root:                root,
		LabelFiles:          []string{labelPath},
		StaticResourceMetas: []string{staticPath},
	}, schema.Schema{
		Objects: []schema.Object{{
			Name: "Thing__c",
			Fields: []schema.Field{
				{Name: "Name", Type: "Text"},
				{Name: "Amount__c", Type: "Number"},
			},
		}},
		CustomMetadataRecords: []schema.CustomMetadataRecord{{
			FullName:      "Feature.Default",
			ObjectName:    "Feature__mdt",
			DeveloperName: "Default",
		}},
	}, []ApexType{{
		Kind:      apexast.DeclarationClass,
		Name:      "Address",
		Namespace: "pkg",
		Modifiers: []string{"global"},
		Members: []ApexMember{{
			Kind:      apexast.DeclarationMethod,
			Name:      "format",
			Type:      "String",
			Modifiers: []string{"global"},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"apex:type:pkg:Address":                            false,
		"apex:member:pkg:Address:method:format:String()":   false,
		"schema:object:pkg__Thing__c":                      false,
		"schema:field:pkg__Thing__c:pkg__Amount__c":        false,
		"schema:custom_metadata:pkg__Feature__mdt:Default": false,
		"metadata:label:pkg__Greeting":                     false,
		"metadata:static_resource:pkg__Site":               false,
	}
	for _, symbol := range artifact.CodeIntelSymbols {
		if _, ok := want[symbol.ID]; ok {
			want[symbol.ID] = true
		}
	}
	for id, found := range want {
		if !found {
			t.Fatalf("missing codeintel symbol %s in %#v", id, artifact.CodeIntelSymbols)
		}
	}
	if artifact.CodeIntelSymbolsVersion != 1 || artifact.CodeIntelUsesVersion != 1 {
		t.Fatalf("versions = symbols %d uses %d", artifact.CodeIntelSymbolsVersion, artifact.CodeIntelUsesVersion)
	}
	if len(artifact.CodeIntelUses) == 0 {
		t.Fatal("missing codeintel declaration uses")
	}
}

func writePackageArtifactTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
