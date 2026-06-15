package codeintel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestBuildIncludesDependencyArtifactCodeIntelSymbols(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "pkg.glade-package.json")
	if err := os.WriteFile(artifactPath, []byte(`{
  "namespace": "pkg",
  "version": "1.0",
  "sourceHash": "abc123",
  "builtAt": "2026-06-14T00:00:00Z",
  "codeIntelSymbolsVersion": 1,
  "codeIntelSymbols": [
    {"id": "apex:type:pkg:Address", "kind": "apex_type", "name": "Address", "namespace": "pkg"},
    {"id": "apex:member:pkg:Address:method:format:String()", "kind": "apex_member", "name": "format", "container": "apex:type:pkg:Address", "namespace": "pkg", "type": "String", "signature": "String()"},
    {"id": "schema:object:pkg__Thing__c", "kind": "sobject", "name": "pkg__Thing__c"},
    {"id": "schema:field:pkg__Thing__c:pkg__Amount__c", "kind": "sobject_field", "name": "pkg__Amount__c", "container": "schema:object:pkg__Thing__c", "type": "Number"},
    {"id": "schema:custom_metadata:pkg__Feature__mdt:Default", "kind": "custom_metadata", "name": "pkg__Feature__mdt.Default", "container": "schema:object:pkg__Feature__mdt"},
    {"id": "metadata:label:pkg__Greeting", "kind": "label", "name": "pkg__Greeting"},
    {"id": "metadata:static_resource:pkg__Site", "kind": "static_resource", "name": "pkg__Site"}
  ],
  "codeIntelUsesVersion": 1,
  "codeIntelUses": [
    {"symbolId": "metadata:label:pkg__Greeting", "kind": "declaration", "name": "pkg__Greeting", "resolved": true}
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	index := typesys.Build(project.Project{
		Root: root,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:    "pkg",
			ArtifactPath: artifactPath,
			Version:      "1.0",
			Status:       "loaded",
		}},
	}, schema.Schema{})
	graph := codeintel.Build(index)

	for _, id := range []codeintel.SymbolID{
		codeintel.ApexTypeID("pkg", "Address"),
		codeintel.ApexMemberID("pkg", "Address", "method", "format", "String()"),
		codeintel.SObjectID("pkg__Thing__c"),
		codeintel.SObjectFieldID("pkg__Thing__c", "pkg__Amount__c"),
		codeintel.CustomMetadataID("pkg__Feature__mdt", "Default"),
		codeintel.LabelID("pkg__Greeting"),
		codeintel.StaticResourceID("pkg__Site"),
	} {
		symbol, ok := graph.Definition(id)
		if !ok {
			t.Fatalf("missing symbol %s in %#v", id, graph.SortedSymbols())
		}
		if !symbol.Dependency || !symbol.Artifact {
			t.Fatalf("symbol %s flags = dependency %v artifact %v", id, symbol.Dependency, symbol.Artifact)
		}
	}
	assertBuildUseAt(t, graph.Uses, codeintel.LabelID("pkg__Greeting"), codeintel.UseDeclaration, "pkg__Greeting", 0, 0)
}
