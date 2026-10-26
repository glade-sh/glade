package storage

import (
	"reflect"
	"testing"
)

func TestOrganizationGroupSeedSnapshotRoutes(t *testing.T) {
	for _, snapshot := range []string{"template", "rollback"} {
		for _, route := range []string{"platform", "shape", "fixture"} {
			t.Run(snapshot+"/"+route, func(t *testing.T) {
				source := NewOrgState()
				EnsureStandardObject(&source, "Group")
				source.Objects["Group"].Records["00G000000000099"] = Record{ID: "00G000000000099", Object: "Group", Fields: map[string]Value{"DeveloperName": StringValue("MeadowReaders"), "Type": StringValue("Regular")}}
				source.IDSequences["Group"] = 99
				source.Features = []string{"Communities"}
				before := source.Objects["Group"].Clone()
				var target OrgState
				if snapshot == "template" {
					target = NewRuntimeTemplate(source).CloneSnapshotOrg()
				} else {
					target = SnapshotRuntimeOrg(&source)
				}
				switch route {
				case "platform":
					EnsureDeterministicPlatformData(&target)
				case "shape":
					ApplyOrgShape(&target, []string{"Communities"})
				case "fixture":
					fixture := NewFixture()
					fixture.Org.Features = []string{"Communities"}
					if err := ApplyFixture(&target, fixture); err != nil {
						t.Fatal(err)
					}
				}
				if !reflect.DeepEqual(before.Records, source.Objects["Group"].Records) || source.IDSequences["Group"] != 99 {
					t.Errorf("%s seed leaked through %s snapshot: base groups=%d, want 1", route, snapshot, len(source.Objects["Group"].Records))
				}
				if len(target.Objects["Group"].Records) != 4 {
					t.Errorf("seeded groups=%d, want regular + 3 native organization groups", len(target.Objects["Group"].Records))
				}
				if sameRecordMap(source.Objects["Group"].Records, target.Objects["Group"].Records) {
					t.Error("seed write retained shared Group records")
				}
			})
		}
	}
}

func TestOrganizationGroupResetRoutes(t *testing.T) {
	for _, shape := range []string{"plain", "communities"} {
		for _, route := range []string{"nonPlatform", "all", "platform"} {
			t.Run(shape+"/"+route, func(t *testing.T) {
				org := NewOrgState()
				features := []string{}
				if shape == "communities" {
					features = []string{"Communities"}
				}
				ApplyOrgShape(&org, features)
				EnsureDeterministicPlatformData(&org)
				original := org.Objects["Group"].Clone()
				org.Objects["Group"].Records["00G000000000099"] = Record{ID: "00G000000000099", Object: "Group", Fields: map[string]Value{"DeveloperName": StringValue("MeadowReaders"), "Type": StringValue("Regular")}}
				org.IDSequences["Group"] = 99
				snapshot := SnapshotRuntimeOrg(&org)
				switch route {
				case "nonPlatform":
					ResetNonPlatformData(&org)
				case "all":
					ResetData(&org)
				case "platform":
					ResetPlatformData(&org)
				}
				want := len(original.Records)
				if route == "platform" {
					want++
				}
				if len(org.Objects["Group"].Records) != want {
					t.Errorf("groups after %s reset=%d, want %d", route, len(org.Objects["Group"].Records), want)
				}
				for id, row := range original.Records {
					if got, ok := org.Objects["Group"].Records[id]; !ok || !reflect.DeepEqual(got.Fields, row.Fields) {
						t.Errorf("reset lost native group %s (%s)", row.Fields["DeveloperName"].String, id)
					}
				}
				if len(snapshot.Objects["Group"].Records) != len(original.Records)+1 {
					t.Error("reset changed rollback snapshot")
				}
			})
		}
	}
}

func TestOrganizationGroupDataResetImportedIdentityRoute(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Group")
	imported := Record{ID: "00G000000000088", Object: "Group", Fields: map[string]Value{"DeveloperName": StringValue("AllInternalUsers"), "Type": StringValue("Organization")}}
	org.Objects["Group"].Records[imported.ID] = imported.Clone()
	org.IDSequences["Group"] = 99
	EnsureDeterministicPlatformData(&org)
	ResetNonPlatformData(&org)
	got, ok := org.Objects["Group"].Records[imported.ID]
	if !ok || !reflect.DeepEqual(got, imported) {
		t.Fatal("data reset discarded imported organization group identity")
	}
	if org.IDSequences["Group"] != 99 {
		t.Fatal("data reset rewound the preserved Group ID sequence")
	}
}

func TestOrganizationGroupSeedNoopSnapshotRoutes(t *testing.T) {
	for _, route := range []string{"platform", "shape", "fixture"} {
		t.Run(route, func(t *testing.T) {
			org := NewOrgState()
			ApplyOrgShape(&org, []string{"Communities"})
			EnsureDeterministicPlatformData(&org)
			target := NewRuntimeTemplate(org).CloneSnapshotOrg()
			sequence := target.IDSequences["Group"]
			switch route {
			case "platform":
				EnsureDeterministicPlatformData(&target)
			case "shape":
				ApplyOrgShape(&target, []string{"Communities"})
			case "fixture":
				fixture := NewFixture()
				fixture.Org.Features = []string{"Communities"}
				if err := ApplyFixture(&target, fixture); err != nil {
					t.Fatal(err)
				}
			}
			if !sameRecordMap(org.Objects["Group"].Records, target.Objects["Group"].Records) {
				t.Fatal("idempotent seed unnecessarily detached Group records")
			}
			if target.IDSequences["Group"] != sequence {
				t.Fatal("idempotent seed advanced Group IDs")
			}
		})
	}
}
