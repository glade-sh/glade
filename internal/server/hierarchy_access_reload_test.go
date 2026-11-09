package server

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestHierarchyAccessReloadTransitions(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["PrivateProbe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "PrivateProbe__c", SharingModel: "Private"}}
	s := New(&org)
	for _, step := range []struct {
		names []string
		want  string
	}{
		{nil, ""},
		{[]string{"PrivateProbe__c"}, "true"},
		{nil, ""},
	} {
		s.ReloadProjectState(SourceMetadata{Project: project.Project{HierarchyAccessObjects: step.names}}, typesys.Index{}, nil, nil)
		if got := org.Objects["PrivateProbe__c"].Definition.Metadata["gladeHierarchyAccess"]; got != step.want {
			t.Fatalf("hierarchy setting = %q, want %q", got, step.want)
		}
	}
}
