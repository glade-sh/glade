package storage

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"
)

func TestOrganizationGroupsNativeShapeRoutes(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/setup_object_read/groups.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native group matrix: %v, rows=%d", err, len(rows))
	}
	for _, row := range rows[1:] {
		for _, route := range []string{"shapeBeforePlatform", "shapeAfterPlatform", "shapeOnly", "fixture"} {
			t.Run(row[0]+"/"+row[1]+"/"+route, func(t *testing.T) {
				var groups []struct{ DeveloperName, Type string }
				if err := json.Unmarshal([]byte(row[2]), &groups); err != nil {
					t.Fatal(err)
				}
				var features []string
				if row[1] == "communities" {
					features = []string{"Communities"}
				}
				org := NewOrgState()
				if route == "shapeAfterPlatform" {
					EnsureDeterministicPlatformData(&org)
				}
				ApplyOrgShape(&org, features)
				if route != "shapeOnly" {
					EnsureDeterministicPlatformData(&org)
				}
				if route == "fixture" {
					fixture := FixtureFromOrg(org)
					org = NewOrgState()
					if err := ApplyFixture(&org, fixture); err != nil {
						t.Fatal(err)
					}
				}
				want := make(map[string]string)
				for _, group := range groups {
					want[group.DeveloperName] = group.Type
				}
				found := make(map[string]int)
				for _, group := range org.Objects["Group"].Records {
					name := group.Fields["DeveloperName"].String
					if name != "AllInternalUsers" && name != "AllCustomerPortalUsers" && name != "AllPartnerUsers" {
						continue
					}
					found[name]++
					if group.Fields["Type"].String != want[name] {
						t.Errorf("%s type=%q, native=%q", name, group.Fields["Type"].String, want[name])
					}
				}
				for name := range want {
					if found[name] != 1 {
						t.Errorf("%s count=%d, native=1", name, found[name])
					}
				}
			})
		}
	}
}

func TestOrganizationGroupPlatformSeedAndFixtureRoutes(t *testing.T) {
	for _, route := range []string{"platform", "fixture", "shapeBeforePlatform", "shapeAfterPlatform"} {
		t.Run(route, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "Group")
			// Imported rows can occupy the first generated ID.
			state := org.Objects["Group"]
			state.Records["00G000000000001"] = Record{ID: "00G000000000001", Object: "Group", Fields: map[string]Value{
				"Name": StringValue("Meadow Readers"), "DeveloperName": StringValue("MeadowReaders"), "Type": StringValue("Regular"),
			}}
			org.Objects["Group"] = state
			if route == "shapeBeforePlatform" {
				ApplyOrgShape(&org, []string{"Communities"})
			}
			EnsureDeterministicPlatformData(&org)
			if route == "shapeAfterPlatform" {
				ApplyOrgShape(&org, []string{"Communities"})
			}
			if route == "fixture" {
				fixture := FixtureFromOrg(org)
				org = NewOrgState()
				if err := ApplyFixture(&org, fixture); err != nil {
					t.Fatal(err)
				}
			}
			EnsureDeterministicPlatformData(&org)
			var internal Record
			count := 0
			for _, group := range org.Objects["Group"].Records {
				if group.Fields["DeveloperName"].String == "AllInternalUsers" {
					internal, count = group, count+1
				}
			}
			if count != 1 || internal.Fields["Type"].String != "Organization" {
				t.Fatalf("AllInternalUsers count=%d, type=%q; native plain Developer count=1, type=Organization", count, internal.Fields["Type"].String)
			}
			if org.Objects["Group"].Records["00G000000000001"].Fields["DeveloperName"].String != "MeadowReaders" {
				t.Fatal("platform seed replaced an imported group")
			}
			if org.IDSequences["Group"] < 2 {
				t.Fatal("platform group did not reserve its generated ID")
			}
		})
	}
}

func TestOrganizationGroupsPreserveImportedIdentityRoute(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Group")
	state := org.Objects["Group"]
	id := ID("00G000000000099")
	state.Records[id] = Record{ID: id, Object: "Group", Fields: map[string]Value{
		"DeveloperName": StringValue("AllInternalUsers"), "Type": StringValue("Organization"),
	}}
	org.Objects["Group"] = state
	fixture := FixtureFromOrg(org)
	fixture.Org.Features = []string{"Communities"}
	org = NewOrgState()
	if err := ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	EnsureDeterministicPlatformData(&org)
	ApplyOrgShape(&org, []string{"Communities"})
	count := 0
	for _, record := range org.Objects["Group"].Records {
		if record.Fields["DeveloperName"].String == "AllInternalUsers" {
			count++
			if record.ID != id {
				t.Fatal("platform seed changed the imported organization group identity")
			}
		}
	}
	if count != 1 {
		t.Fatalf("imported organization group duplicated: %d", count)
	}
}
