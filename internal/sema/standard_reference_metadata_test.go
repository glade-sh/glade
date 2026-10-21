package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardReferenceFieldsSupportAssignmentsAndQueryPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "StandardReferenceProbe.cls")
	writeSemaFile(t, path, `
public class StandardReferenceProbe {
  public void run() {
    Visit visit = new Visit();
    AssociatedLocation association = new AssociatedLocation();
    visit.PlaceId = association.LocationId;
    List<AssociatedLocation> rows = [SELECT Id, Location.VisitorAddress.Address FROM AssociatedLocation];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA003" || diag.Code == "GLADESEMA018" || (diag.Code == "GLADESEMA_QUERY_RELATIONSHIP" && strings.Contains(diag.Message, "VisitorAddress")) {
			t.Fatalf("standard reference metadata should type Id fields and relationship paths: %#v", result.Diagnostics)
		}
	}
}

func TestStandardCatalogReferenceMetadataDoesNotRequireUnavailableTargets(t *testing.T) {
	for _, objectName := range []string{"CspTrustedSite", "Product2"} {
		t.Run(objectName, func(t *testing.T) {
			definition, ok := storage.StandardObjectDefinition(objectName)
			if !ok {
				t.Fatalf("missing standard object definition: %s", objectName)
			}
			object := schemaObjectFromStorageDefinition(definition)
			if len(object.Fields) == 0 {
				t.Fatalf("empty standard object projection: %s", objectName)
			}
			root := t.TempDir()
			path := filepath.Join(root, "ReferenceMetadataProbe.cls")
			writeSemaFile(t, path, "public class ReferenceMetadataProbe {}")
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{Objects: []schema.Object{object}})
			result := Analyze(index)
			for _, diag := range result.Diagnostics {
				if diag.Code == "GLADESEMA003" {
					t.Fatalf("catalog projection must not introduce unavailable reference targets: %#v", result.Diagnostics)
				}
			}
		})
	}
}

func TestExplicitLookupMetadataStillRequiresKnownTarget(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ExplicitReferenceProbe.cls")
	writeSemaFile(t, path, "public class ExplicitReferenceProbe {}")
	metadata := schema.Schema{Objects: []schema.Object{{
		Name: "ReferenceEnvelope__c",
		Fields: []schema.Field{{
			Name: "Destination__c", Type: "Lookup", ReferenceTo: []string{"UnavailableDestination__c"},
		}},
	}}}
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, metadata)
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA003" && strings.Contains(diag.Message, "ReferenceEnvelope__c.Destination__c") && strings.Contains(diag.Message, "UnavailableDestination__c") {
			return
		}
	}
	t.Fatalf("explicit missing lookup target must retain its diagnostic: %#v", result.Diagnostics)
}
