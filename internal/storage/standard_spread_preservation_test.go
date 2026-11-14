package storage

import (
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestPlainDeveloperSpreadPreservesInPlaceFieldEdit(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObjectSpread(&org)
	obj := org.Objects["AIFeatureExtractor"]
	field := obj.Definition.Fields["Id"]
	original := field
	defer func() { obj.Definition.Fields["Id"] = original }()
	field.Label = "changed"
	obj.Definition.Fields["Id"] = field
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	if got, ok := org.Objects["AIFeatureExtractor"]; !ok || got.Definition.Fields["Id"].Label != "changed" {
		t.Fatal("in-place field edit lost")
	}
}

func TestPlainDeveloperSpreadPreservesExplicitInstall(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "AIFeatureExtractor")
	EnsureStandardObjectSpread(&org)
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	if _, ok := org.Objects["AIFeatureExtractor"]; !ok {
		t.Fatal("explicit install lost")
	}
}

func TestPlainDeveloperSpreadPreservesUncapturedNeighbor(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObjectSpread(&org)
	if _, ok := org.Objects["CareProgram"]; !ok {
		t.Fatal("neighbor missing before shape")
	}
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	if _, ok := org.Objects["CareProgram"]; !ok {
		t.Fatal("uncaptured neighbor lost")
	}
}

func TestSpreadDefinitionSnapshotDetectsInPlaceEdits(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ObjectDefinition)
	}{
		{"field", func(d *ObjectDefinition) { d.Fields["Id"] = Field{APIName: "Id", Label: "changed"} }},
		{"metadata", func(d *ObjectDefinition) { d.Metadata["key"] = "changed" }},
		{"slice", func(d *ObjectDefinition) { d.RecordTypes[0].Name = "changed" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := ObjectDefinition{Fields: map[string]Field{"Id": {APIName: "Id"}}, Metadata: map[string]string{"key": "value"}, RecordTypes: []RecordTypeInfo{{Name: "original"}}}
			snapshot := recordInstalledSpreadDefinition(d)
			tc.mutate(&d)
			if sameDefinitionInstance(d, snapshot) {
				t.Fatal("in-place edit was not detected")
			}
		})
	}
}

func TestSpreadFingerprintDistinguishesEmptyCollections(t *testing.T) {
	d := ObjectDefinition{APIName: "Example"}
	snapshot := recordInstalledSpreadDefinition(d)
	d.Metadata = map[string]string{}
	if spreadDefinitionFingerprint(d) == snapshot.fingerprint {
		t.Fatal("empty map edit was not detected")
	}
}

func TestPlainDeveloperSpreadPreservesInPlaceEditWithoutSnapshot(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObjectSpread(&org)
	org.spreadInstalled = nil
	obj := org.Objects["AIFeatureExtractor"]
	original := obj.Definition.Fields["Id"]
	defer func() { obj.Definition.Fields["Id"] = original }()
	edited := original
	edited.Label = "edited"
	obj.Definition.Fields["Id"] = edited
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	if got, ok := org.Objects["AIFeatureExtractor"]; !ok || got.Definition.Fields["Id"].Label != "edited" {
		t.Fatal("edited definition removed without snapshot")
	}
}

func TestPlainDeveloperSpreadPreservesNestedEditWithoutSnapshot(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObjectSpread(&org)
	org.spreadInstalled = nil
	obj := org.Objects["AIFeatureExtractor"]
	field := obj.Definition.Fields["CreatedById"]
	if len(field.ReferenceTo) == 0 {
		t.Fatal("expected reference target")
	}
	original := field.ReferenceTo[0]
	defer func() { field.ReferenceTo[0] = original }()
	field.ReferenceTo[0] = "EditedTarget"
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	if got, ok := org.Objects["AIFeatureExtractor"]; !ok || !slices.Contains(got.Definition.Fields["CreatedById"].ReferenceTo, "EditedTarget") {
		t.Fatal("nested definition edit removed without snapshot")
	}
}
