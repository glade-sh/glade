package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

// Each case mutates an installed org's schema through a path that does not call
// ClearRuntimeSchemaStamp and keeps the object count. SetOrg must not reuse the
// stamp it recorded before the mutation: without a trusted runtime template
// hint it recomputes the stamp from the current schema.
func TestSetOrgRecomputesStampAfterCountPreservingSchemaMutation(t *testing.T) {
	thing := func() storage.ObjectState {
		return storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName:     "Thing__c",
			Label:       "Thing",
			PluralLabel: "Things",
			KeyPrefix:   "a01",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r"},
			},
			Relations: []storage.Relationship{{Field: "Parent__c", ParentRelationship: "Parent__r", ChildRelationship: "Things__r", ParentObjects: []string{"Account"}}},
		}}
	}
	cases := []struct {
		name   string
		setup  func(*storage.OrgState)
		mutate func(*testing.T, *storage.OrgState)
	}{
		{
			name: "fixture namespace",
			mutate: func(t *testing.T, org *storage.OrgState) {
				if err := storage.ApplyFixture(org, storage.Fixture{Org: storage.FixtureOrg{Namespace: "fixns"}}); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "custom metadata field overlay",
			setup: func(org *storage.OrgState) {
				org.Objects["Setting__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{
					// The overlay assigns the missing key prefix in place.
					APIName:  "Setting__mdt",
					Metadata: map[string]string{"kind": "customMetadata"},
					Fields:   map[string]storage.Field{"Text__c": {APIName: "Text__c", Type: storage.FieldString}},
				}}
			},
			mutate: func(t *testing.T, org *storage.OrgState) {
				err := storage.ApplyCustomMetadataRecords(org, []schema.CustomMetadataRecord{{
					FullName:      "Setting.Seed",
					ObjectName:    "Setting__mdt",
					DeveloperName: "Seed",
					Label:         "Seed",
					Values:        []schema.CustomMetadataValue{{Field: "Text__c", Value: "seed"}},
				}})
				if err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "key prefix repair",
			setup: func(org *storage.OrgState) {
				other := thing()
				other.Definition.APIName = "Other__c"
				org.Objects["Other__c"] = other
			},
			mutate: func(t *testing.T, org *storage.OrgState) {
				storage.EnsureUniqueKeyPrefixes(org)
				if org.Objects["Thing__c"].Definition.KeyPrefix == org.Objects["Other__c"].Definition.KeyPrefix {
					t.Fatal("key prefix repair did not change a prefix")
				}
			},
		},
		{
			name: "direct field edit",
			mutate: func(t *testing.T, org *storage.OrgState) {
				object := org.Objects["Thing__c"]
				field := object.Definition.Fields["Name"]
				field.Type = storage.FieldBoolean
				object.Definition.Fields["Name"] = field
				org.Objects["Thing__c"] = object
			},
		},
		{
			name: "direct relationship edit",
			mutate: func(t *testing.T, org *storage.OrgState) {
				object := org.Objects["Thing__c"]
				object.Definition.Relations[0].ChildRelationship = "RenamedThings__r"
				org.Objects["Thing__c"] = object
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["Thing__c"] = thing()
			if tc.setup != nil {
				tc.setup(&org)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			before := machine.metadataCacheStamp
			objects := len(org.Objects)

			tc.mutate(t, &org)
			if len(org.Objects) != objects {
				t.Fatalf("mutation changed the object count %d -> %d", objects, len(org.Objects))
			}
			want := schemaCacheStampForOrg(&org)
			if want == before {
				t.Fatal("mutation does not change the schema stamp")
			}
			machine.SetOrg(&org)
			if got := machine.metadataCacheStamp; got != want {
				t.Fatalf("stamp after mutation = %q, want recomputed %q (before %q)", got, want, before)
			}
			again := New(nil)
			again.SetOrg(&org)
			if got := again.metadataCacheStamp; got != want {
				t.Fatalf("fresh install stamp = %q, want %q", got, want)
			}
		})
	}
}

// Mirrors runCase on an isolation journal: every case installs the same org
// with SetRuntimeTemplateOrg from a primed base, and System.runAs ensures an
// existing standard object. That no-op keeps the template hint, so later cases
// keep taking the matching-stamp path and share the base's schema caches.
func TestPrimedRuntimeCasesKeepTemplateStampAcrossNoopEnsure(t *testing.T) {
	template := storage.NewRuntimeTemplate(storage.NewOrgState())
	storage.EnsureStandardObject(&template.Org, "PermissionSetAssignment")
	PrimeRuntimeTemplateSchema(&template)
	base := New(nil)
	journalOrg := template.CloneRuntimeOrg()
	base.PrimeMetadataSchema(&journalOrg)

	for i := range 3 {
		machine := base.CloneRuntime(nil)
		shared := machine.jsonChildRelTypeCache
		machine.SetRuntimeTemplateOrg(&journalOrg)
		if machine.jsonChildRelTypeCache != shared || machine.metadataCacheStamp != template.RuntimeSchemaStamp {
			t.Fatalf("case %d did not take the matching-stamp path", i)
		}
		storage.EnsureStandardObject(&journalOrg, "PermissionSetAssignment")
		if journalOrg.RuntimeSchemaStamp != template.RuntimeSchemaStamp {
			t.Fatalf("case %d: no-op EnsureStandardObject dropped the template stamp", i)
		}
	}

	// A real schema change still drops the hint, and the next case falls back
	// to SetOrg with a recomputed stamp and fresh caches.
	storage.EnsureStandardObject(&journalOrg, "Contact")
	if journalOrg.RuntimeSchemaStamp != "" {
		t.Fatal("adding an object kept the template stamp")
	}
	machine := base.CloneRuntime(nil)
	shared := machine.jsonChildRelTypeCache
	machine.SetRuntimeTemplateOrg(&journalOrg)
	if machine.jsonChildRelTypeCache == shared {
		t.Fatal("changed schema kept the base's schema caches")
	}
	if got, want := machine.metadataCacheStamp, schemaCacheStampForOrg(&journalOrg); got != want {
		t.Fatalf("stamp after schema change = %q, want %q", got, want)
	}
}
