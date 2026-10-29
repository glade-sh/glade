package dml

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountProfileDefaultsNativeRows(t *testing.T) {
	native, err := os.ReadFile("../../testdata/conformance/person_account_profile_defaults/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(native)), "\n")[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 5 || row[1] != "istest" || !strings.HasSuffix(row[2], "_omitted") {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureDeterministicPlatformData(&org)
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{
				{DeveloperName: "Business_Default", Name: "Business Default", Active: true, Available: true, Default: true},
				{DeveloperName: "Business_Alternate", Name: "Business Alternate", Active: true, Available: true},
				{DeveloperName: "PersonAccount", Name: "Person Account", Active: true, Available: true},
				{DeveloperName: "Provider", Name: "Provider", Active: true, Available: true, PersonAccount: true},
				{DeveloperName: "Patient", Name: "Patient", Active: true, Available: true, PersonAccount: true},
			}
			org.Objects["Account"] = account
			storage.EnsureDeterministicPlatformData(&org)
			// Use the serialized metadata shape so the regression compiles before the
			// runtime understands profile-specific defaults.
			err := json.Unmarshal([]byte(`{"accountRecordTypeDefaults":[{"profileId":"00e000000000001","developerName":"Provider","personAccount":true},{"profileId":"00e000000000001","developerName":"Business_Default"},{"profileId":"00e000000000002","developerName":"Patient","personAccount":true},{"profileId":"00e000000000002","developerName":"Business_Alternate"},{"profileId":"00e000000000003","developerName":"PersonAccount","personAccount":true},{"profileId":"00e000000000003","developerName":"Business_Alternate"}]}`), &org.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			user := org.Objects["User"].Records[storage.DefaultUserID]
			profile := storage.ID("00e000000000001")
			if strings.HasPrefix(row[2], "alternate_") {
				profile = "00e000000000002"
			}
			if strings.HasPrefix(row[2], "generic_") {
				profile = "00e000000000003"
			}
			user.Fields["ProfileId"] = storage.IDValue(profile)
			users := org.Objects["User"]
			users.Records[storage.DefaultUserID] = user
			org.Objects["User"] = users
			fields := map[string]storage.Value{"Name": storage.StringValue("Probe")}
			if strings.Contains(row[2], "_person_") {
				fields = map[string]storage.Value{"LastName": storage.StringValue("Probe")}
			}
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{{Object: "Account", Fields: fields}})
			if !result[0].Success {
				t.Fatal(result)
			}
			stored := org.Objects["Account"].Records[result[0].ID]
			got := ""
			for _, rt := range org.Objects["Account"].Definition.RecordTypes {
				if storage.IDsEqual(rt.ID, idFromStorageValue(stored.Fields["RecordTypeId"])) {
					got = rt.DeveloperName
				}
			}
			if got != row[3] {
				t.Fatalf("native %s; local %s", row[3], got)
			}
		})
	}
	if count != 16 {
		t.Fatalf("native default rows=%d, want 16", count)
	}
}

func TestPersonAccountDefaultPrecedesNamedAlternatives(t *testing.T) {
	native, err := os.ReadFile("../../testdata/conformance/person_account_profile_defaults/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(native)), "\n")[1:] {
		row := strings.Split(line, "\t")
		if row[0] != "62.0" || row[1] != "istest" || !strings.HasSuffix(row[2], "_person_omitted") {
			continue
		}
		t.Run(row[2], func(t *testing.T) {
			types := []storage.RecordTypeInfo{
				{DeveloperName: "Individual_Alternative", Name: "Individual Alternative", Active: true, Available: true, PersonAccount: true},
				{DeveloperName: "PersonAccount", Name: "Person Account", Active: true, Available: true},
				{DeveloperName: "Provider", Name: "Provider", Active: true, Available: true, PersonAccount: true},
				{DeveloperName: "Patient", Name: "Patient", Active: true, Available: true, PersonAccount: true},
			}
			for i := range types {
				types[i].Default = types[i].DeveloperName == row[3]
			}
			org := testOrg()
			storage.EnsureDeterministicPlatformData(&org)
			org.Metadata.AccountRecordTypeDefaults = []storage.AccountRecordTypeDefault{{
				ProfileID: storage.DefaultProfileID, DeveloperName: row[3], PersonAccount: true,
			}}
			selected, ok := AccountDefaultRecordType(&org, storage.DefaultUserID, storage.ObjectDefinition{RecordTypes: types}, storage.Record{
				Object: "Account", Fields: map[string]storage.Value{"LastName": storage.StringValue("Probe")},
			})
			if !ok || selected.DeveloperName != row[3] {
				t.Fatalf("native default %s; local %+v, found=%v", row[3], selected, ok)
			}
		})
	}
}
