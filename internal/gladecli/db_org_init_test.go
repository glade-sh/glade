package gladecli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestOrgStateFromIndexReturnsCustomMetadataApplyError(t *testing.T) {
	_, err := orgStateFromIndex(t.TempDir(), project.Project{}, typesys.Index{
		Objects: []gladeschema.Object{{
			Name:  "Feature__mdt",
			Label: "Feature",
		}},
		CustomMetadataRecords: []gladeschema.CustomMetadataRecord{{
			FullName:      "Missing.Default",
			ObjectName:    "Feature__mdt",
			DeveloperName: "Default",
			Values:        []gladeschema.CustomMetadataValue{{Field: "Missing__c", Value: "x"}},
		}},
	})
	if err == nil {
		t.Fatalf("orgStateFromIndex() error = nil, want custom metadata apply error")
	}
	if !strings.Contains(err.Error(), "Missing__c") {
		t.Fatalf("orgStateFromIndex() error = %v, want metadata object detail", err)
	}
}

func TestProjectBindingKeepsSourceVersionIndependentOfEndpoint(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	org, binding, ok, err := projectOrgAndDBBinding(root)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || org.APIVersion != storage.DefaultRESTAPIVersion || binding.SourceAPIVersion != "67.0" {
		t.Fatalf("ok=%v org endpoint=%q binding source=%q", ok, org.APIVersion, binding.SourceAPIVersion)
	}
}
