package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

func TestPlainDeveloperSpreadPreservesInPlaceFieldEdit(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObjectSpread(&org)
	obj := org.Objects["AIFeatureExtractor"]
	field := obj.Definition.Fields["Id"]
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
