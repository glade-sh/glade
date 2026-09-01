package packageartifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestBuildAppliesInstalledNamespaceToCustomSchema(t *testing.T) {
	artifact, err := Build("pkg", "1.0", project.Project{Root: t.TempDir(), SourceAPIVersion: "65.0"}, schema.Schema{Objects: []schema.Object{
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

func TestBuildRequiresValidSourceAPIVersion(t *testing.T) {
	for _, sourceAPIVersion := range []string{"", "68.0", "67.1"} {
		t.Run(sourceAPIVersion, func(t *testing.T) {
			_, err := Build("pkg", "1.0", project.Project{
				Root:             t.TempDir(),
				SourceAPIVersion: sourceAPIVersion,
			}, schema.Schema{}, nil)
			if err == nil {
				t.Fatalf("Build accepted sourceApiVersion %q", sourceAPIVersion)
			}
		})
	}
}

func TestBuildPreservesHistoricalSourceAPIVersion(t *testing.T) {
	for _, sourceAPIVersion := range []string{"43.0", "61.0"} {
		artifact, err := Build("pkg", "1.0", project.Project{Root: t.TempDir(), SourceAPIVersion: sourceAPIVersion}, schema.Schema{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if artifact.SourceAPIVersion != sourceAPIVersion {
			t.Fatalf("sourceApiVersion = %q, want %q", artifact.SourceAPIVersion, sourceAPIVersion)
		}
		if issues := Validate(artifact); len(issues) != 0 {
			t.Fatalf("Validate issues = %#v", issues)
		}
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
		SourceAPIVersion:    "65.0",
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
	if artifact.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", artifact.SchemaVersion, CurrentSchemaVersion)
	}
	if artifact.Labels != 1 || len(artifact.LabelNames) != 1 || artifact.LabelNames[0] != "pkg__Greeting" {
		t.Fatalf("labels = %d %#v", artifact.Labels, artifact.LabelNames)
	}
	if artifact.StaticResources != 1 || len(artifact.StaticResourceNames) != 1 || artifact.StaticResourceNames[0] != "pkg__Site" {
		t.Fatalf("static resources = %d %#v", artifact.StaticResources, artifact.StaticResourceNames)
	}
}

func TestBuildCapturedArtifactPreservesOrgProvenanceAndMetadataNames(t *testing.T) {
	artifact, err := BuildCaptured(BuildCapturedOptions{
		Namespace:        "pkg",
		PackageName:      "Billing Core",
		Version:          "1.2.3.4",
		SourceAPIVersion: "65.0",
		Capture: CaptureProvenance{
			Source:      "org",
			OrgID:       "00Dxx0000000001",
			Username:    "builder@example.com",
			TargetOrg:   "packaging",
			APIVersion:  "65.0",
			CapturedAt:  mustParsePackageArtifactTime(t, "2026-06-19T12:00:00Z"),
			PackageID:   "033xx0000000001",
			InstalledID: "0A3xx0000000001",
		},
		ApexTypes: []ApexType{{
			Kind:      apexast.DeclarationClass,
			Name:      "BillingGateway",
			Namespace: "pkg",
			Modifiers: []string{"global"},
			Members: []ApexMember{{
				Kind:      apexast.DeclarationMethod,
				Name:      "authorize",
				Type:      "Boolean",
				Modifiers: []string{"global", "static"},
				Parameters: []apexast.Parameter{{
					Name: "amount",
					Type: "Decimal",
				}},
			}},
		}},
		Objects: []schema.Object{{
			Name: "pkg__Billing_Profile__c",
			Fields: []schema.Field{{
				Name: "pkg__External_Key__c",
				Type: "Text",
			}},
		}},
		LabelNames:          []string{"pkg__Billing_Error"},
		StaticResourceNames: []string{"pkg__BillingAssets"},
		LightningBundles: []LightningBundle{{
			Namespace: "pkg",
			Name:      "billingConsole",
			Type:      "lwc",
			Exposed:   true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.SchemaVersion != 2 {
		t.Fatalf("schema version = %d, want 2", artifact.SchemaVersion)
	}
	if artifact.Capture.OrgID != "00Dxx0000000001" || artifact.Capture.PackageID != "033xx0000000001" {
		t.Fatalf("capture provenance = %#v", artifact.Capture)
	}
	if artifact.Labels != 1 || len(artifact.LabelNames) != 1 || artifact.LabelNames[0] != "pkg__Billing_Error" {
		t.Fatalf("labels = %d %#v", artifact.Labels, artifact.LabelNames)
	}
	if artifact.StaticResources != 1 || len(artifact.StaticResourceNames) != 1 || artifact.StaticResourceNames[0] != "pkg__BillingAssets" {
		t.Fatalf("static resources = %d %#v", artifact.StaticResources, artifact.StaticResourceNames)
	}
	if len(artifact.LightningBundles) != 1 || artifact.LightningBundles[0].QualifiedName() != "pkg/billingConsole" {
		t.Fatalf("lightning bundles = %#v", artifact.LightningBundles)
	}
	if artifact.SourceHash == "" {
		t.Fatal("sourceHash is empty")
	}
	if !artifactHasCodeIntelSymbol(artifact, "metadata:label:pkg__Billing_Error") {
		t.Fatalf("missing label symbol in %#v", artifact.CodeIntelSymbols)
	}
	if !artifactHasCodeIntelSymbol(artifact, "metadata:static_resource:pkg__BillingAssets") {
		t.Fatalf("missing static resource symbol in %#v", artifact.CodeIntelSymbols)
	}
}

func TestBuildCapturedRequiresValidSourceAPIVersion(t *testing.T) {
	for _, sourceAPIVersion := range []string{"", "68.0", "67.1"} {
		t.Run(sourceAPIVersion, func(t *testing.T) {
			_, err := BuildCaptured(BuildCapturedOptions{
				Namespace:        "pkg",
				SourceAPIVersion: sourceAPIVersion,
			})
			if err == nil {
				t.Fatalf("BuildCaptured accepted sourceApiVersion %q", sourceAPIVersion)
			}
		})
	}
}

func TestBuildCapturedNormalizesSupportedSourceAPIVersion(t *testing.T) {
	artifact, err := BuildCaptured(BuildCapturedOptions{
		Namespace:        "pkg",
		SourceAPIVersion: "v67.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.SourceAPIVersion != "67.0" {
		t.Fatalf("sourceApiVersion = %q, want 67.0", artifact.SourceAPIVersion)
	}
}

func TestValidateRequiresValidSourceAPIVersionForCurrentSchema(t *testing.T) {
	for _, test := range []struct {
		name    string
		version string
		want    string
	}{
		{name: "missing", want: "sourceApiVersion is required"},
		{name: "future", version: "68.0", want: "unsupported source API version"},
	} {
		t.Run(test.name, func(t *testing.T) {
			issues := Validate(Artifact{
				SchemaVersion:    CurrentSchemaVersion,
				Namespace:        "pkg",
				SourceHash:       "abc",
				SourceAPIVersion: test.version,
			})
			if len(issues) != 1 || !strings.Contains(issues[0], test.want) {
				t.Fatalf("issues = %#v, want %q", issues, test.want)
			}
		})
	}
}

func TestValidateRejectsUnsupportedArtifactSchemaVersion(t *testing.T) {
	issues := Validate(Artifact{
		SchemaVersion: 99,
		Namespace:     "pkg",
		SourceHash:    "abc",
		BuiltAt:       mustParsePackageArtifactTime(t, "2026-06-19T12:00:00Z"),
	})
	if len(issues) != 1 || issues[0] != "unsupported artifact schemaVersion 99" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestBuildCapturedArtifactHashIgnoresInputOrder(t *testing.T) {
	first, err := BuildCaptured(BuildCapturedOptions{
		Namespace:        "pkg",
		Version:          "1.0",
		SourceAPIVersion: "65.0",
		Capture: CaptureProvenance{
			Source:     "org",
			CapturedAt: mustParsePackageArtifactTime(t, "2026-06-19T12:00:00Z"),
		},
		ApexTypes: []ApexType{
			capturedArtifactApexType("Zeta"),
			capturedArtifactApexTypeWithMembers("Alpha",
				capturedArtifactMethod("run", "String"),
				capturedArtifactMethod("run", "Decimal"),
			),
		},
		Objects: []schema.Object{
			capturedArtifactObject("pkg__Zeta__c", "Name", "pkg__Code__c"),
			capturedArtifactObject("pkg__Alpha__c", "pkg__Code__c", "Name"),
		},
		LabelNames:          []string{"pkg__Zeta", "pkg__Alpha"},
		StaticResourceNames: []string{"pkg__ZetaAssets", "pkg__AlphaAssets"},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildCaptured(BuildCapturedOptions{
		Namespace:        "pkg",
		Version:          "1.0",
		SourceAPIVersion: "65.0",
		Capture: CaptureProvenance{
			Source:     "org",
			CapturedAt: mustParsePackageArtifactTime(t, "2026-06-19T12:00:00Z"),
		},
		ApexTypes: []ApexType{
			capturedArtifactApexTypeWithMembers("Alpha",
				capturedArtifactMethod("run", "Decimal"),
				capturedArtifactMethod("run", "String"),
			),
			capturedArtifactApexType("Zeta"),
		},
		Objects: []schema.Object{
			capturedArtifactObject("pkg__Alpha__c", "Name", "pkg__Code__c"),
			capturedArtifactObject("pkg__Zeta__c", "pkg__Code__c", "Name"),
		},
		LabelNames:          []string{"pkg__Alpha", "pkg__Zeta"},
		StaticResourceNames: []string{"pkg__AlphaAssets", "pkg__ZetaAssets"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceHash != second.SourceHash {
		t.Fatalf("source hashes differ: %s != %s", first.SourceHash, second.SourceHash)
	}
}

func TestBuildCapturedArtifactHashIgnoresCaptureProvenance(t *testing.T) {
	first, err := BuildCaptured(BuildCapturedOptions{
		Namespace:        "pkg",
		Version:          "1.0",
		SourceAPIVersion: "65.0",
		Capture: CaptureProvenance{
			Source:     "org",
			TargetOrg:  "packaging",
			OrgID:      "00D000000000001",
			CapturedAt: mustParsePackageArtifactTime(t, "2026-06-19T12:00:00Z"),
		},
		ApexTypes: []ApexType{capturedArtifactApexType("Alpha")},
		Objects:   []schema.Object{capturedArtifactObject("pkg__Alpha__c", "Name")},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildCaptured(BuildCapturedOptions{
		Namespace:        "pkg",
		Version:          "1.0",
		SourceAPIVersion: "65.0",
		Capture: CaptureProvenance{
			Source:     "org",
			TargetOrg:  "subscriber",
			OrgID:      "00D000000000002",
			CapturedAt: mustParsePackageArtifactTime(t, "2026-06-19T12:05:00Z"),
		},
		ApexTypes: []ApexType{capturedArtifactApexType("Alpha")},
		Objects:   []schema.Object{capturedArtifactObject("pkg__Alpha__c", "Name")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceHash != second.SourceHash {
		t.Fatalf("source hashes differ: %s != %s", first.SourceHash, second.SourceHash)
	}
}

func TestValidateTreatsOmittedArtifactSchemaVersionAsVersionOne(t *testing.T) {
	issues := Validate(Artifact{
		Namespace:  "pkg",
		SourceHash: "abc",
		BuiltAt:    mustParsePackageArtifactTime(t, "2026-06-19T12:00:00Z"),
	})
	if len(issues) != 0 {
		t.Fatalf("issues = %#v", issues)
	}
}

func artifactHasCodeIntelSymbol(artifact Artifact, id string) bool {
	for _, symbol := range artifact.CodeIntelSymbols {
		if symbol.ID == id {
			return true
		}
	}
	return false
}

func capturedArtifactApexType(name string) ApexType {
	return capturedArtifactApexTypeWithMembers(name,
		capturedArtifactMethod("zeta", "String"),
		capturedArtifactMethod("alpha", "String"),
	)
}

func capturedArtifactApexTypeWithMembers(name string, members ...ApexMember) ApexType {
	return ApexType{
		Kind:      apexast.DeclarationClass,
		Name:      name,
		Namespace: "pkg",
		Modifiers: []string{"global"},
		Members:   members,
	}
}

func capturedArtifactMethod(name string, paramType string) ApexMember {
	method := ApexMember{Kind: apexast.DeclarationMethod, Name: name, Type: "String", Modifiers: []string{"global"}}
	if paramType != "" {
		method.Parameters = []apexast.Parameter{{Name: "value", Type: paramType}}
	}
	return method
}

func capturedArtifactObject(name string, fields ...string) schema.Object {
	object := schema.Object{Name: name}
	for _, field := range fields {
		object.Fields = append(object.Fields, schema.Field{Name: field, Type: "Text"})
	}
	return object
}

func mustParsePackageArtifactTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
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
