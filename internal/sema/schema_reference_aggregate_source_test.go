package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaReferenceAggregateMetadataSourceRoutes(t *testing.T) {
	for _, route := range []string{"aggregate_object", "decomposed_override"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "objects/Parcel__c/fields"), 0755); err != nil {
				t.Fatal(err)
			}
			objectPath := filepath.Join(root, "objects/Parcel__c.object")
			writeSemaFile(t, objectPath, `<CustomObject><fields><fullName>Destination__c</fullName><type>Lookup</type><referenceTo>UnavailableDestination</referenceTo></fields></CustomObject>`)
			p := project.Project{Root: root, ObjectFiles: []string{objectPath}}
			want := objectPath
			if route == "decomposed_override" {
				want = filepath.Join(root, "objects/Parcel__c/fields/Destination__c.field-meta.xml")
				writeSemaFile(t, want, `<CustomField><fullName>Destination__c</fullName><type>Lookup</type><referenceTo>UnavailableDestination</referenceTo></CustomField>`)
				p.FieldFiles = []string{want}
			}
			metadata, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, metadata)
			// Disk-cache/artifact encoding must preserve diagnostic provenance.
			raw, err := json.Marshal(index)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &index); err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, d := range Analyze(index).Diagnostics {
				if d.Code != "GLADESEMA003" {
					continue
				}
				count++
				if d.File != want {
					t.Errorf("metadata source=%q want=%q", d.File, want)
				}
			}
			if count == 0 {
				t.Fatal("missing lookup rejection")
			}
		})
	}
}
