package storage

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandardProfilesNativeSeedAndResetRoutes(t *testing.T) {
	for _, shape := range []string{"developer", "communities"} {
		file, err := os.Open(filepath.Join("../apextest/testdata/conformance/standard_profiles/native", shape, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(file)
		reader.Comma, reader.LazyQuotes = '\t', true
		rows, err := reader.ReadAll()
		file.Close()
		if err != nil || len(rows) != 13 {
			t.Fatalf("incomplete native matrix: %v", err)
		}
		for _, row := range rows[1:] {
			if row[2] != "anonymous" || row[3] != "profiles" {
				continue
			}
			t.Run(shape+"/"+row[1], func(t *testing.T) {
				var want map[string]string
				if err := json.Unmarshal([]byte(row[4]), &want); err != nil || len(want) != 6 {
					t.Fatalf("invalid profile observation: %v", err)
				}
				org := NewOrgState()
				org.APIVersion = row[1]
				if shape == "communities" {
					ApplyOrgShape(&org, []string{"Communities"})
				}
				for _, route := range []string{"initial", "refresh", "reset"} {
					if route == "reset" {
						ResetNonPlatformData(&org)
					}
					EnsureDeterministicPlatformData(&org)
					t.Run(route, func(t *testing.T) {
						for name, identity := range want {
							t.Run(name, func(t *testing.T) {
								count := 0
								var profile Record
								for _, record := range org.Objects["Profile"].Records {
									if record.Fields["Name"].String == name {
										profile, count = record, count+1
									}
								}
								if identity == "System.QueryException" {
									if count != 0 {
										t.Fatalf("native singleton rejects absent profile; Glade count=%d", count)
									}
									return
								}
								if count != 1 {
									t.Fatalf("native profile count=1; Glade count=%d", count)
								}
								license, ok := org.Objects["UserLicense"].Records[profile.Fields["UserLicenseId"].ID]
								if !ok {
									t.Fatal("profile has no linked license")
								}
								got := strings.Join([]string{profile.Fields["UserType"].String, license.Fields["Name"].String, license.Fields["LicenseDefinitionKey"].String}, "|")
								if got != identity {
									t.Fatalf("native=%s; Glade=%s", identity, got)
								}
							})
						}
					})
				}
			})
		}
	}
}

func TestStandardProfilesImportedIdentityAndCollisionRoutes(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Profile")
	EnsureStandardObject(&org, "UserLicense")
	license := Record{ID: "100000000000099", Object: "UserLicense", Fields: map[string]Value{
		"Name": StringValue("Customer Community Plus"), "LicenseDefinitionKey": StringValue("PID_Customer_Community_Plus"),
		"Status": StringValue("Inactive"), "TotalLicenses": IntegerValue(19),
	}}
	profile := Record{ID: "00e000000000099", Object: "Profile", Fields: map[string]Value{
		"Name": StringValue("Customer Community Plus User"), "UserType": StringValue("PowerCustomerSuccess"),
		"UserLicenseId": IDValue(license.ID), "PermissionsModifyAllData": BooleanValue(false),
	}}
	occupied := Record{ID: "100000000000007", Object: "UserLicense", Fields: map[string]Value{
		"Name": StringValue("Meadow License"), "LicenseDefinitionKey": StringValue("Meadow_License"),
	}}
	occupiedProfile := Record{ID: "00e00000000000D", Object: "Profile", Fields: map[string]Value{
		"Name": StringValue("Meadow Observer"), "UserType": StringValue("Standard"),
	}}
	org.Objects["Profile"].Records[occupiedProfile.ID] = occupiedProfile
	org.Objects["UserLicense"].Records[license.ID] = license
	org.Objects["UserLicense"].Records[occupied.ID] = occupied
	org.Objects["Profile"].Records[profile.ID] = profile
	for range 2 {
		EnsureDeterministicPlatformData(&org)
	}
	for _, want := range []Record{license, profile, occupied, occupiedProfile} {
		got := org.Objects[want.Object].Records[want.ID]
		for field, value := range want.Fields {
			if !ValuesEqual(got.Fields[field], value) {
				t.Errorf("imported %s.%s=%v; want %v", want.Object, field, got.Fields[field], value)
			}
		}
	}
	for _, name := range []string{"Customer Community Plus User", "Partner Community User", "Contract Manager", "Solution Manager"} {
		count := 0
		for _, record := range org.Objects["Profile"].Records {
			if record.Fields["Name"].String == name {
				count++
			}
		}
		if count != 1 {
			t.Errorf("profile %s count=%d; want 1", name, count)
		}
	}
}

func TestStandardProfilesImportedStandardProfileRoute(t *testing.T) {
	org := NewOrgState()
	const profileID ID = "00e000000000099"
	fields := map[string]Value{
		"Name": StringValue("Contract Manager"), "UserType": StringValue("Standard"),
		"UserLicenseId": IDValue("100000000000001"), "PermissionsActivateContract": BooleanValue(true),
	}
	if err := ApplyFixture(&org, Fixture{Version: FixtureVersion, Objects: []FixtureObject{
		{Name: "Profile", Records: []FixtureRecord{{ID: profileID, Fields: fields}}},
	}}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		EnsureDeterministicPlatformData(&org)
	}
	count := 0
	for _, profile := range org.Objects["Profile"].Records {
		if profile.Fields["Name"].String == "Contract Manager" {
			count++
			if profile.ID != profileID {
				t.Errorf("default duplicated imported profile: %s", profile.ID)
			}
		}
	}
	if count != 1 {
		t.Fatalf("imported standard profile count=%d; want 1", count)
	}
	for field, want := range fields {
		if got := org.Objects["Profile"].Records[profileID].Fields[field]; !ValuesEqual(got, want) {
			t.Errorf("imported %s=%v; want %v", field, got, want)
		}
	}
}
